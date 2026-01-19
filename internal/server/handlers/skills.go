package handlers

import (
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/sanixdarker/skillforge/internal/app"
	"github.com/sanixdarker/skillforge/internal/server/middleware"
	"github.com/sanixdarker/skillforge/web"
)

// SkillsHandler handles skill registry requests.
type SkillsHandler struct {
	app *app.App
}

// NewSkillsHandler creates a new SkillsHandler.
func NewSkillsHandler(application *app.App) *SkillsHandler {
	return &SkillsHandler{app: application}
}

// Browse renders the browse page.
func (h *SkillsHandler) Browse(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	tag := r.URL.Query().Get("tag")
	query := r.URL.Query().Get("q")

	var skills interface{}
	var total int
	var err error

	if query != "" {
		skills, total, err = h.app.RegistryService.SearchSkills(query, page, 20)
	} else if tag != "" {
		skills, total, err = h.app.RegistryService.ListSkillsByTag(tag, page, 20)
	} else {
		skills, total, err = h.app.RegistryService.ListSkills(page, 20)
	}

	if err != nil {
		h.app.Logger.Error("failed to list skills", "error", err)
	}

	// Get all tags
	tags, _ := h.app.RegistryService.GetAllTags()

	data := map[string]interface{}{
		"Title":       "Browse - Skill Forge",
		"Skills":      skills,
		"Total":       total,
		"Page":        page,
		"Query":       query,
		"Tag":         tag,
		"Tags":        tags,
		"HasNext":     total > page*20,
		"HasPrev":     page > 1,
		"NextPage":    page + 1,
		"PrevPage":    page - 1,
	}

	if middleware.IsHTMXRequest(r) {
		if err := web.RenderPartial(w, "skill-list.html", data); err != nil {
			h.app.Logger.Error("failed to render skill list", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	} else {
		if err := web.RenderPage(w, "browse.html", data); err != nil {
			h.app.Logger.Error("failed to render browse page", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// View renders a single skill page.
func (h *SkillsHandler) View(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	skill, err := h.app.RegistryService.ViewSkill(slug)
	if err != nil {
		h.app.Logger.Error("failed to get skill", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if skill == nil {
		http.NotFound(w, r)
		return
	}

	data := map[string]interface{}{
		"Title": skill.Name + " - Skill Forge",
		"Skill": skill,
	}

	if err := web.RenderPage(w, "skill.html", data); err != nil {
		h.app.Logger.Error("failed to render skill page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Create handles skill creation.
func (h *SkillsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	var content string
	file, _, err := r.FormFile("file")
	if err == nil {
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "Failed to read file", http.StatusBadRequest)
			return
		}
		content = string(data)
	} else {
		content = r.FormValue("content")
	}

	if content == "" {
		http.Error(w, "No content provided", http.StatusBadRequest)
		return
	}

	stored, err := h.app.RegistryService.ImportSkill(content)
	if err != nil {
		http.Error(w, "Failed to create skill: "+err.Error(), http.StatusBadRequest)
		return
	}

	if middleware.IsHTMXRequest(r) {
		w.Header().Set("HX-Redirect", "/skill/"+stored.Slug)
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, "/skill/"+stored.Slug, http.StatusSeeOther)
	}
}

// List returns a list of skills as JSON or HTML.
func (h *SkillsHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	skills, total, err := h.app.RegistryService.ListSkills(page, 20)
	if err != nil {
		h.app.Logger.Error("failed to list skills", "error", err)
	}

	data := map[string]interface{}{
		"Skills":   skills,
		"Total":    total,
		"Page":     page,
		"HasNext":  total > page*20,
		"HasPrev":  page > 1,
		"NextPage": page + 1,
		"PrevPage": page - 1,
	}

	if err := web.RenderPartial(w, "skill-list.html", data); err != nil {
		h.app.Logger.Error("failed to render skill list", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Search handles skill search.
func (h *SkillsHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	if query == "" {
		h.List(w, r)
		return
	}

	skills, total, err := h.app.RegistryService.SearchSkills(query, page, 20)
	if err != nil {
		h.app.Logger.Error("failed to search skills", "error", err)
	}

	data := map[string]interface{}{
		"Skills":   skills,
		"Total":    total,
		"Page":     page,
		"Query":    query,
		"HasNext":  total > page*20,
		"HasPrev":  page > 1,
		"NextPage": page + 1,
		"PrevPage": page - 1,
	}

	if err := web.RenderPartial(w, "skill-list.html", data); err != nil {
		h.app.Logger.Error("failed to render skill list", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Get returns a single skill.
func (h *SkillsHandler) Get(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	skill, err := h.app.RegistryService.GetSkill(slug)
	if err != nil {
		h.app.Logger.Error("failed to get skill", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if skill == nil {
		http.NotFound(w, r)
		return
	}

	data := map[string]interface{}{
		"Skill": skill,
	}

	if err := web.RenderPartial(w, "skill-detail.html", data); err != nil {
		h.app.Logger.Error("failed to render skill detail", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Delete removes a skill.
func (h *SkillsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.app.RegistryService.DeleteSkill(id); err != nil {
		h.app.Logger.Error("failed to delete skill", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if middleware.IsHTMXRequest(r) {
		w.Header().Set("HX-Redirect", "/browse")
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, "/browse", http.StatusSeeOther)
	}
}

// Download returns the skill content as a downloadable file.
func (h *SkillsHandler) Download(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	skill, err := h.app.RegistryService.GetSkill(slug)
	if err != nil {
		h.app.Logger.Error("failed to get skill", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if skill == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/markdown")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+slug+".md\"")
	w.Write([]byte(skill.Content))
}
