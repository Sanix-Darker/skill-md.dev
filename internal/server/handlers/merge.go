package handlers

import (
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/sanixdarker/skill-md/internal/app"
	"github.com/sanixdarker/skill-md/internal/merger"
	"github.com/sanixdarker/skill-md/internal/server/middleware"
	"github.com/sanixdarker/skill-md/internal/sources"
	"github.com/sanixdarker/skill-md/pkg/skill"
	"github.com/sanixdarker/skill-md/web"
)

// Upload limits for merge handler
const (
	maxMergeUploadSize = 10 << 20 // 10MB total
	maxMergeFileSize   = 5 << 20  // 5MB per file
)

// SkillRef represents a reference to a skill for merging.
type SkillRef struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Name   string `json:"name"`
}

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
		"Title": "Merge - Skill MD",
	}

	if err := web.RenderPage(w, "merge.html", data); err != nil {
		h.app.Logger.Error("failed to render merge page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Merge handles skill merging from files or skill references.
func (h *MergeHandler) Merge(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxMergeUploadSize); err != nil {
		h.app.Logger.Error("failed to parse merge form", "error", err)
		h.renderError(w, r, "Failed to parse form. Please try again.")
		return
	}

	var skills []*skill.Skill

	// Check for skill references first (from search/browse)
	skillRefsJSON := r.FormValue("skill_refs")
	if skillRefsJSON != "" {
		var skillRefs []SkillRef
		if err := json.Unmarshal([]byte(skillRefsJSON), &skillRefs); err != nil {
			h.app.Logger.Error("invalid skill references", "error", err)
			h.renderError(w, r, "Invalid skill references. Please try again.")
			return
		}

		if len(skillRefs) < 2 {
			h.renderError(w, r, "At least 2 skills are required for merging")
			return
		}

		// Fetch content for each skill reference
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()

		for _, ref := range skillRefs {
			content, err := h.fetchSkillContent(ctx, ref)
			if err != nil {
				h.app.Logger.Error("failed to fetch skill", "name", ref.Name, "error", err)
				h.renderError(w, r, "Failed to fetch skill. Please try again.")
				return
			}

			s, err := skill.Parse(content)
			if err != nil {
				h.app.Logger.Error("failed to parse skill", "name", ref.Name, "error", err)
				h.renderError(w, r, "Failed to parse skill. Please check the skill format.")
				return
			}

			skills = append(skills, s)
		}
	} else {
		// Fall back to file uploads
		files := r.MultipartForm.File["files"]
		if len(files) < 2 {
			h.renderError(w, r, "At least 2 files are required for merging")
			return
		}

		// Parse all skills from files
		for _, fileHeader := range files {
			// Validate per-file size
			if fileHeader.Size > maxMergeFileSize {
				h.renderError(w, r, "File too large (max 5MB per file)")
				return
			}

			file, err := fileHeader.Open()
			if err != nil {
				h.app.Logger.Error("failed to open file", "filename", fileHeader.Filename, "error", err)
				h.renderError(w, r, "Failed to open file. Please try again.")
				return
			}

			content, err := io.ReadAll(file)
			file.Close()
			if err != nil {
				h.app.Logger.Error("failed to read file", "filename", fileHeader.Filename, "error", err)
				h.renderError(w, r, "Failed to read file. Please try again.")
				return
			}

			s, err := skill.Parse(string(content))
			if err != nil {
				h.app.Logger.Error("failed to parse file", "filename", fileHeader.Filename, "error", err)
				h.renderError(w, r, "Failed to parse file. Please check the skill format.")
				return
			}

			skills = append(skills, s)
		}
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
		h.app.Logger.Error("merge failed", "error", err)
		h.renderError(w, r, "Merge failed. Please try again.")
		return
	}

	// Check for nil result (empty skills array)
	if result == nil {
		h.renderError(w, r, "No skills to merge")
		return
	}

	// Render output
	output := skill.Render(result)

	// Return result
	if middleware.IsHTMXRequest(r) {
		data := map[string]interface{}{
			"Content":    output,
			"Name":       result.Frontmatter.Name,
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

// fetchSkillContent fetches the content for a skill reference.
func (h *MergeHandler) fetchSkillContent(ctx context.Context, ref SkillRef) (string, error) {
	sourceType := sources.SourceType(ref.Source)

	// For local skills, get from registry
	if sourceType == sources.SourceTypeLocal {
		stored, err := h.app.RegistryService.GetSkill(ref.ID)
		if err != nil {
			return "", err
		}
		if stored == nil {
			return "", nil
		}
		return stored.Content, nil
	}

	// For external skills, use federated source
	skill, err := h.app.FederatedSource.GetSkill(ctx, sourceType, ref.ID)
	if err != nil {
		return "", err
	}

	if skill == nil {
		return "", nil
	}

	// Get content if not loaded
	if skill.Content == "" {
		content, err := h.app.FederatedSource.GetContent(ctx, skill)
		if err != nil {
			return "", err
		}
		return content, nil
	}

	return skill.Content, nil
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
