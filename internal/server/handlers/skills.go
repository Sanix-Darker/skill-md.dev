package handlers

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sanixdarker/skillforge/internal/app"
	"github.com/sanixdarker/skillforge/internal/server/middleware"
	"github.com/sanixdarker/skillforge/internal/sources"
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
	source := r.URL.Query().Get("source")

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Build search options
	opts := sources.SearchOptions{
		Query:   query,
		Page:    page,
		PerPage: 20,
	}
	if tag != "" {
		opts.Tags = []string{tag}
	}

	var data map[string]interface{}

	// Determine which sources to search
	var sourcesToSearch []sources.SourceType
	if source != "" {
		sourcesToSearch = []sources.SourceType{sources.SourceType(source)}
	}

	// Perform federated search
	result, err := h.app.FederatedSource.SearchSources(ctx, opts, sourcesToSearch)
	if err != nil {
		h.app.Logger.Error("federated search failed", "error", err)
	}

	// Get all tags from local registry
	tags, _ := h.app.RegistryService.GetAllTags()

	total := 0
	var skills []*sources.ExternalSkill
	bySource := make(map[string]int)
	var searchTime string

	if result != nil {
		skills = result.Skills
		total = result.Total
		for src, count := range result.BySource {
			bySource[string(src)] = count
		}
		searchTime = result.SearchTime.Round(time.Millisecond).String()
	}

	data = map[string]interface{}{
		"Title":      "Browse - Skill Forge",
		"Skills":     skills,
		"Total":      total,
		"Page":       page,
		"Query":      query,
		"Tag":        tag,
		"Tags":       tags,
		"Source":     source,
		"BySource":   bySource,
		"SearchTime": searchTime,
		"HasNext":    total > page*20,
		"HasPrev":    page > 1,
		"NextPage":   page + 1,
		"PrevPage":   page - 1,
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
	source := r.URL.Query().Get("source")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	opts := sources.SearchOptions{
		Query:   query,
		Page:    page,
		PerPage: 20,
	}

	var sourcesToSearch []sources.SourceType
	if source != "" {
		sourcesToSearch = []sources.SourceType{sources.SourceType(source)}
	}

	result, err := h.app.FederatedSource.SearchSources(ctx, opts, sourcesToSearch)
	if err != nil {
		h.app.Logger.Error("federated search failed", "error", err)
	}

	total := 0
	var skills []*sources.ExternalSkill
	bySource := make(map[string]int)
	var searchTime string

	if result != nil {
		skills = result.Skills
		total = result.Total
		for src, count := range result.BySource {
			bySource[string(src)] = count
		}
		searchTime = result.SearchTime.Round(time.Millisecond).String()
	}

	data := map[string]interface{}{
		"Skills":     skills,
		"Total":      total,
		"Page":       page,
		"Query":      query,
		"Source":     source,
		"BySource":   bySource,
		"SearchTime": searchTime,
		"HasNext":    total > page*20,
		"HasPrev":    page > 1,
		"NextPage":   page + 1,
		"PrevPage":   page - 1,
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

	// Sanitize filename to prevent header injection
	safeFilename := middleware.SanitizeFilename(slug)

	w.Header().Set("Content-Type", "text/markdown")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+safeFilename+".md\"")
	w.Write([]byte(skill.Content))
}

// ViewExternal renders an external skill page.
func (h *SkillsHandler) ViewExternal(w http.ResponseWriter, r *http.Request) {
	sourceType := chi.URLParam(r, "source")
	id := chi.URLParam(r, "*")

	// URL decode the ID
	decodedID, err := url.PathUnescape(id)
	if err != nil {
		decodedID = id
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	skill, err := h.app.FederatedSource.GetSkill(ctx, sources.SourceType(sourceType), decodedID)
	if err != nil {
		h.app.Logger.Error("failed to get external skill", "source", sourceType, "id", id, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if skill == nil {
		http.NotFound(w, r)
		return
	}

	// Try to fetch content if not loaded
	if skill.Content == "" {
		content, err := h.app.FederatedSource.GetContent(ctx, skill)
		if err == nil && content != "" {
			skill.Content = content
		}
	}

	data := map[string]interface{}{
		"Title": skill.Name + " - Skill Forge",
		"Skill": skill,
	}

	if err := web.RenderPage(w, "skill-external.html", data); err != nil {
		h.app.Logger.Error("failed to render external skill page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// ImportExternal imports an external skill to the local registry.
func (h *SkillsHandler) ImportExternal(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	sourceType := r.FormValue("source")
	id := r.FormValue("id")

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	skill, err := h.app.FederatedSource.GetSkill(ctx, sources.SourceType(sourceType), id)
	if err != nil {
		h.app.Logger.Error("failed to get external skill for import", "error", err)
		http.Error(w, "Failed to get skill", http.StatusInternalServerError)
		return
	}

	if skill == nil {
		http.Error(w, "Skill not found", http.StatusNotFound)
		return
	}

	// Get content if not loaded
	if skill.Content == "" {
		content, err := h.app.FederatedSource.GetContent(ctx, skill)
		if err != nil {
			h.app.Logger.Error("failed to get skill content", "error", err)
			http.Error(w, "Failed to get skill content", http.StatusInternalServerError)
			return
		}
		skill.Content = content
	}

	if skill.Content == "" {
		http.Error(w, "Skill has no content", http.StatusBadRequest)
		return
	}

	// Import to local registry
	stored, err := h.app.RegistryService.ImportSkill(skill.Content)
	if err != nil {
		h.app.Logger.Error("failed to import skill", "error", err)
		http.Error(w, "Failed to import skill: "+err.Error(), http.StatusBadRequest)
		return
	}

	if middleware.IsHTMXRequest(r) {
		w.Header().Set("HX-Redirect", "/skill/"+stored.Slug)
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, "/skill/"+stored.Slug, http.StatusSeeOther)
	}
}

// GetExternalContent fetches content for an external skill via HTMX.
func (h *SkillsHandler) GetExternalContent(w http.ResponseWriter, r *http.Request) {
	sourceType := chi.URLParam(r, "source")
	id := chi.URLParam(r, "*")

	decodedID, err := url.PathUnescape(id)
	if err != nil {
		decodedID = id
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	skill, err := h.app.FederatedSource.GetSkill(ctx, sources.SourceType(sourceType), decodedID)
	if err != nil {
		h.app.Logger.Error("failed to get external skill", "error", err)
		http.Error(w, "Failed to get skill", http.StatusInternalServerError)
		return
	}

	if skill == nil {
		http.Error(w, "Skill not found", http.StatusNotFound)
		return
	}

	content, err := h.app.FederatedSource.GetContent(ctx, skill)
	if err != nil {
		h.app.Logger.Error("failed to get skill content", "error", err)
		http.Error(w, "Failed to get content", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<pre class="text-sm whitespace-pre-wrap overflow-auto max-h-[600px] bg-terminal-bg p-4 border border-terminal-border">` + content + `</pre>`))
}
