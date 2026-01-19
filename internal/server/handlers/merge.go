package handlers

import (
	"io"
	"mime/multipart"
	"net/http"

	"github.com/sanixdarker/skillforge/internal/app"
	"github.com/sanixdarker/skillforge/internal/merger"
	"github.com/sanixdarker/skillforge/internal/server/middleware"
	"github.com/sanixdarker/skillforge/pkg/skill"
	"github.com/sanixdarker/skillforge/web"
)

// MergeHandler handles merge requests.
type MergeHandler struct {
	app *app.App
}

// NewMergeHandler creates a new MergeHandler.
func NewMergeHandler(application *app.App) *MergeHandler {
	return &MergeHandler{app: application}
}

// Index renders the merge page.
func (h *MergeHandler) Index(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Merge - Skill Forge",
	}

	if err := web.RenderPage(w, "merge.html", data); err != nil {
		h.app.Logger.Error("failed to render merge page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Merge handles skill merging.
func (h *MergeHandler) Merge(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(50 << 20); err != nil { // 50MB max
		h.renderError(w, r, "Failed to parse form: "+err.Error())
		return
	}

	// Get uploaded files
	files := r.MultipartForm.File["files"]
	if len(files) < 2 {
		h.renderError(w, r, "At least 2 files are required for merging")
		return
	}

	// Parse all skills
	var skills []*skill.Skill
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			h.renderError(w, r, "Failed to open file: "+err.Error())
			return
		}

		content, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			h.renderError(w, r, "Failed to read file: "+err.Error())
			return
		}

		s, err := skill.Parse(string(content))
		if err != nil {
			h.renderError(w, r, "Failed to parse "+fileHeader.Filename+": "+err.Error())
			return
		}

		skills = append(skills, s)
	}

	// Get options
	name := r.FormValue("name")
	dedupe := r.FormValue("dedupe") == "true" || r.FormValue("dedupe") == "on"

	// Merge
	result, err := h.app.Merger.Merge(skills, &merger.Options{
		Name:        name,
		Deduplicate: dedupe,
	})
	if err != nil {
		h.renderError(w, r, "Merge failed: "+err.Error())
		return
	}

	// Render output
	output := skill.Render(result)

	// Return result
	if middleware.IsHTMXRequest(r) {
		data := map[string]interface{}{
			"Content":    output,
			"Name":       result.Frontmatter.Name,
			"FileCount":  len(files),
			"SkillCount": len(skills),
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

// parseSkillFromFile parses a skill from a multipart file.
func (h *MergeHandler) parseSkillFromFile(fh *multipart.FileHeader) (*skill.Skill, error) {
	file, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return skill.Parse(string(content))
}

func (h *MergeHandler) renderError(w http.ResponseWriter, r *http.Request, msg string) {
	if middleware.IsHTMXRequest(r) {
		data := map[string]interface{}{
			"Error": msg,
		}
		web.RenderPartial(w, "error.html", data)
	} else {
		http.Error(w, msg, http.StatusBadRequest)
	}
}
