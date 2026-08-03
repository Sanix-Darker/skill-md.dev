package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/sanixdarker/skillf/internal/app"
	"github.com/sanixdarker/skillf/internal/converter"
	"github.com/sanixdarker/skillf/internal/server/middleware"
	"github.com/sanixdarker/skillf/pkg/skill"
	"github.com/sanixdarker/skillf/web"
)

// Upload limits for convert handler
const (
	maxConvertUploadSize = 10 << 20 // 10MB total
	maxConvertFileSize   = 5 << 20  // 5MB per file
)

// ConvertHandler handles conversion requests.
type ConvertHandler struct {
	app *app.App
}

// NewConvertHandler creates a new ConvertHandler.
func NewConvertHandler(application *app.App) *ConvertHandler {
	return &ConvertHandler{app: application}
}

// Index renders the convert page.
func (h *ConvertHandler) Index(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":   "Convert - Skillf",
		"Formats": h.app.ConverterManager.SupportedFormats(),
	}

	if err := web.RenderPage(w, "convert.html", data); err != nil {
		h.app.Logger.Error("failed to render convert page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Convert handles file conversion.
func (h *ConvertHandler) Convert(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	if err := r.ParseMultipartForm(maxConvertUploadSize); err != nil {
		h.app.Logger.Error("failed to parse convert form", "error", err)
		h.renderError(w, r, "Failed to parse form. Please try again.")
		return
	}

	// Get file, URL, or text content
	var content []byte
	var filename string
	format := r.FormValue("format")

	// Check for URL first
	urlInput := strings.TrimSpace(r.FormValue("url"))
	if urlInput != "" {
		remoteInput, err := converter.ResolveRemoteInput(h.app.ConverterManager, urlInput, format)
		if err != nil {
			h.app.Logger.Error("failed to resolve remote input", "url", urlInput, "format", format, "error", err)
			h.renderError(w, r, "Failed to fetch the provided URL. Please check it and try again.")
			return
		}
		content = remoteInput.Content
		filename = remoteInput.SourcePath
		format = remoteInput.Format
	} else {
		// Try file upload
		file, header, err := r.FormFile("file")
		if err == nil {
			// Validate per-file size
			if header.Size > maxConvertFileSize {
				h.renderError(w, r, "File too large (max 5MB)")
				return
			}
			defer file.Close()
			content, err = io.ReadAll(file)
			if err != nil {
				h.app.Logger.Error("failed to read file", "error", err)
				h.renderError(w, r, "Failed to read file. Please try again.")
				return
			}
			filename = header.Filename
		} else {
			// Try text input
			text := r.FormValue("content")
			if text == "" {
				h.renderError(w, r, "No file, URL, or content provided")
				return
			}

			// Check if text is a URL
			text = strings.TrimSpace(text)
			if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
				remoteInput, err := converter.ResolveRemoteInput(h.app.ConverterManager, text, format)
				if err != nil {
					h.app.Logger.Error("failed to resolve remote input from content", "url", text, "format", format, "error", err)
					h.renderError(w, r, "Failed to fetch the provided URL. Please check it and try again.")
					return
				}
				content = remoteInput.Content
				filename = remoteInput.SourcePath
				format = remoteInput.Format
			} else {
				content = []byte(text)
				filename = "input.txt"
			}
		}
	}

	// Get format (auto-detect if not specified)
	if format == "" || format == "auto" {
		format = h.app.ConverterManager.DetectFormat(filename, content)
	}

	// Get optional name
	name := r.FormValue("name")

	// Convert
	result, err := h.app.ConverterManager.Convert(format, content, &converter.Options{
		Name:       name,
		SourcePath: filename,
	})
	if err != nil {
		h.app.Logger.Error("conversion failed", "format", format, "error", err)
		h.renderError(w, r, "Conversion failed. Please check the input format and try again.")
		return
	}

	// Render output
	output := skill.Render(result)

	// Return result
	if middleware.IsHTMXRequest(r) {
		data := map[string]interface{}{
			"Content":  output,
			"Name":     result.Frontmatter.Name,
			"Format":   format,
			"Filename": filename,
		}
		if err := web.RenderPartial(w, "code-preview.html", data); err != nil {
			h.app.Logger.Error("failed to render preview", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	} else {
		w.Header().Set("Content-Type", "text/markdown")
		w.Write([]byte(output))
	}
}

// ConvertURL handles URL conversion via JSON API.
func (h *ConvertHandler) ConvertURL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL    string `json:"url"`
		Name   string `json:"name,omitempty"`
		Format string `json:"format,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.app.Logger.Error("invalid JSON in convert URL request", "error", err)
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	req.URL = converter.NormalizeRemoteURL(req.URL)

	remoteInput, err := converter.ResolveRemoteInput(h.app.ConverterManager, req.URL, req.Format)
	if err != nil {
		h.app.Logger.Error("URL conversion failed", "url", req.URL, "format", req.Format, "error", err)
		http.Error(w, "Conversion failed. Please check the URL and try again.", http.StatusInternalServerError)
		return
	}

	result, err := h.app.ConverterManager.Convert(remoteInput.Format, remoteInput.Content, &converter.Options{
		Name:       req.Name,
		SourcePath: remoteInput.SourcePath,
	})
	if err != nil {
		h.app.Logger.Error("URL conversion failed", "url", req.URL, "format", remoteInput.Format, "error", err)
		http.Error(w, "Conversion failed. Please check the URL and try again.", http.StatusInternalServerError)
		return
	}

	// Render output
	output := skill.Render(result)

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"content": output,
		"name":    result.Frontmatter.Name,
		"format":  remoteInput.Format,
		"url":     req.URL,
	})
}

// DetectFormat detects the format of uploaded content.
func (h *ConvertHandler) DetectFormat(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"format": "text"})
		return
	}

	var content []byte
	var filename string

	file, header, err := r.FormFile("file")
	if err == nil {
		defer file.Close()
		content, _ = io.ReadAll(file)
		filename = header.Filename
	} else {
		contentStr := strings.TrimSpace(r.FormValue("content"))
		urlStr := strings.TrimSpace(r.FormValue("url"))
		isURLInput := urlStr != ""
		if !isURLInput {
			urlStr = contentStr
			isURLInput = strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://")
		}

		if isURLInput {
			remoteInput, err := converter.ResolveRemoteInput(h.app.ConverterManager, urlStr, "auto")
			if err == nil {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"format": remoteInput.Format})
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"format": "url"})
			return
		}
		content = []byte(contentStr)
		filename = "input" + filepath.Ext(r.FormValue("filename"))
	}

	format := h.app.ConverterManager.DetectFormat(filename, content)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"format": format})
}

func (h *ConvertHandler) renderError(w http.ResponseWriter, r *http.Request, msg string) {
	if middleware.IsHTMXRequest(r) {
		data := map[string]interface{}{
			"Error": msg,
		}
		web.RenderPartial(w, "error.html", data)
	} else {
		http.Error(w, msg, http.StatusBadRequest)
	}
}
