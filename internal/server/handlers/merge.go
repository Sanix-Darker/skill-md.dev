package handlers

import (
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/sanixdarker/skillforge/internal/app"
	"github.com/sanixdarker/skillforge/internal/merger"
	"github.com/sanixdarker/skillforge/internal/server/middleware"
	"github.com/sanixdarker/skillforge/internal/sources"
	"github.com/sanixdarker/skillforge/pkg/skill"
	"github.com/sanixdarker/skillforge/web"
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
		"Title": "Merge - Skill Forge",
	}

	if err := web.RenderPage(w, "merge.html", data); err != nil {
		h.app.Logger.Error("failed to render merge page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Merge handles skill merging from files or skill references.
func (h *MergeHandler) Merge(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(50 << 20); err != nil { // 50MB max
		h.renderError(w, r, "Failed to parse form: "+err.Error())
		return
	}

	var skills []*skill.Skill

	// Check for skill references first (from search/browse)
	skillRefsJSON := r.FormValue("skill_refs")
	if skillRefsJSON != "" {
		var skillRefs []SkillRef
		if err := json.Unmarshal([]byte(skillRefsJSON), &skillRefs); err != nil {
			h.renderError(w, r, "Invalid skill references: "+err.Error())
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
				h.renderError(w, r, "Failed to fetch skill '"+ref.Name+"': "+err.Error())
				return
			}

			s, err := skill.Parse(content)
			if err != nil {
				h.renderError(w, r, "Failed to parse skill '"+ref.Name+"': "+err.Error())
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
