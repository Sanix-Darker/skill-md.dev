package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/sanixdarker/skill-md/internal/app"
	"github.com/sanixdarker/skill-md/internal/converter"
	"github.com/sanixdarker/skill-md/internal/server/middleware"
	"github.com/sanixdarker/skill-md/pkg/skill"
	"github.com/sanixdarker/skill-md/web"
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
		"Title":   "Convert - Skill MD",
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
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB max
		h.renderError(w, r, "Failed to parse form: "+err.Error())
		return
	}

	// Get file, URL, or text content
	var content []byte
	var filename string
	var format string

	// Check for URL first
	urlInput := r.FormValue("url")
	if urlInput != "" {
		// URL conversion
		if !strings.HasPrefix(urlInput, "http://") && !strings.HasPrefix(urlInput, "https://") {
			urlInput = "https://" + urlInput
		}
		content = []byte(urlInput)
		filename = urlInput
		format = "url"
	} else {
		// Try file upload
		file, header, err := r.FormFile("file")
		if err == nil {
			defer file.Close()
			content, err = io.ReadAll(file)
			if err != nil {
				h.renderError(w, r, "Failed to read file: "+err.Error())
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
				content = []byte(text)
				filename = text
				format = "url"
			} else {
				content = []byte(text)
				filename = "input.txt"
			}
		}
	}

	// Get format (auto-detect if not specified)
	if format == "" {
		format = r.FormValue("format")
		if format == "" || format == "auto" {
			format = h.app.ConverterManager.DetectFormat(filename, content)
		}
	}

	// Get optional name
	name := r.FormValue("name")

	// Convert
	result, err := h.app.ConverterManager.Convert(format, content, &converter.Options{
		Name:       name,
		SourcePath: filename,
	})
	if err != nil {
		h.renderError(w, r, "Conversion failed: "+err.Error())
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
		URL  string `json:"url"`
		Name string `json:"name,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Ensure URL has scheme
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		req.URL = "https://" + req.URL
	}

	// Convert
	result, err := h.app.ConverterManager.Convert("url", []byte(req.URL), &converter.Options{
		Name:       req.Name,
		SourcePath: req.URL,
	})
	if err != nil {
		http.Error(w, "Conversion failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Render output
	output := skill.Render(result)

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"content": output,
		"name":    result.Frontmatter.Name,
		"format":  "url",
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
		contentStr := r.FormValue("content")
		// Check if it's a URL
		if strings.HasPrefix(contentStr, "http://") || strings.HasPrefix(contentStr, "https://") {
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
