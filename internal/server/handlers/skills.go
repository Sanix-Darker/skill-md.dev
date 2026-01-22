package handlers

import (
	"context"
	"html"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sanixdarker/skill-md/internal/app"
	"github.com/sanixdarker/skill-md/internal/server/middleware"
	"github.com/sanixdarker/skill-md/internal/sources"
	"github.com/sanixdarker/skill-md/web"
)

// Input validation constants
const (
	MaxQueryLength     = 500
	MaxSlugLength      = 100
	MaxSkillContentLen = 5 * 1024 * 1024 // 5MB max skill content
)

// validSources is a whitelist of allowed source types
var validSources = map[string]bool{
	"local":     true,
	"skills.sh": true,
	"github":    true,
	"gitlab":    true,
	"bitbucket": true,
	"codeberg":  true,
}

// containsPathTraversal checks if a string contains path traversal sequences
func containsPathTraversal(s string) bool {
	// Check for common path traversal patterns
	dangerous := []string{
		"..",
		"./",
		".\\",
		"%2e%2e",
		"%252e%252e",
		"..%2f",
		"..%5c",
	}
	lower := strings.ToLower(s)
	for _, pattern := range dangerous {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

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
		PerPage: 10,
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

	// If no specific query, source, or tag, fetch from all configured sources
	if query == "" && len(sourcesToSearch) == 0 && tag == "" {
		sourcesToSearch = []sources.SourceType{
			sources.SourceTypeGitHub,
			sources.SourceTypeSkillsSH,
		}
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
	sourceErrors := make(map[string]string)
	var searchTime string

	if result != nil {
		skills = result.Skills
		total = result.Total
		for src, count := range result.BySource {
			bySource[string(src)] = count
		}
		for src, errMsg := range result.SourceErrors {
			sourceErrors[string(src)] = errMsg
		}
		searchTime = result.SearchTime.Round(time.Millisecond).String()
	}

	data = map[string]interface{}{
		"Title":        "Browse - Skill MD",
		"Skills":       skills,
		"Total":        total,
		"Page":         page,
		"Query":        query,
		"Tag":          tag,
		"Tags":         tags,
		"Source":       source,
		"BySource":     bySource,
		"SourceErrors": sourceErrors,
		"SearchTime":   searchTime,
		"HasNext":      total > page*10,
		"HasPrev":      page > 1,
		"NextPage":     page + 1,
		"PrevPage":     page - 1,
	}

	htmxReq := middleware.GetHTMX(r)
	if htmxReq.IsHTMX && !htmxReq.IsBoosted {
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
		"Title": skill.Name + " - Skill MD",
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
	file, header, err := r.FormFile("file")
	if err == nil {
		// Validate file size
		if header.Size > MaxSkillContentLen {
			http.Error(w, "File too large (max 5MB)", http.StatusRequestEntityTooLarge)
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, MaxSkillContentLen+1))
		if err != nil {
			http.Error(w, "Failed to read file", http.StatusBadRequest)
			return
		}
		if len(data) > MaxSkillContentLen {
			http.Error(w, "File too large (max 5MB)", http.StatusRequestEntityTooLarge)
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

	// Validate content length
	if len(content) > MaxSkillContentLen {
		http.Error(w, "Content too large (max 5MB)", http.StatusRequestEntityTooLarge)
		return
	}

	stored, err := h.app.RegistryService.ImportSkill(content)
	if err != nil {
		h.app.Logger.Error("failed to create skill", "error", err)
		http.Error(w, "Failed to create skill. Please check the skill format.", http.StatusBadRequest)
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

	skills, total, err := h.app.RegistryService.ListSkills(page, 10)
	if err != nil {
		h.app.Logger.Error("failed to list skills", "error", err)
	}

	data := map[string]interface{}{
		"Skills":   skills,
		"Total":    total,
		"Page":     page,
		"HasNext":  total > page*10,
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
	mergeMode := r.URL.Query().Get("merge_mode") == "true"

	// Validate input length
	if len(query) > MaxQueryLength {
		http.Error(w, "Query too long", http.StatusBadRequest)
		return
	}

	// Validate source type if provided
	if source != "" && !validSources[source] {
		http.Error(w, "Invalid source type", http.StatusBadRequest)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	opts := sources.SearchOptions{
		Query:   query,
		Page:    page,
		PerPage: 10,
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
	sourceErrors := make(map[string]string)
	var searchTime string

	if result != nil {
		skills = result.Skills
		total = result.Total
		for src, count := range result.BySource {
			bySource[string(src)] = count
		}
		for src, errMsg := range result.SourceErrors {
			sourceErrors[string(src)] = errMsg
		}
		searchTime = result.SearchTime.Round(time.Millisecond).String()
	}

	data := map[string]interface{}{
		"Skills":       skills,
		"Total":        total,
		"Page":         page,
		"Query":        query,
		"Source":       source,
		"BySource":     bySource,
		"SourceErrors": sourceErrors,
		"SearchTime":   searchTime,
		"HasNext":      total > page*10,
		"HasPrev":      page > 1,
		"NextPage":     page + 1,
		"PrevPage":     page - 1,
		"MergeMode":    mergeMode,
	}

	// Use merge-specific template when in merge mode
	templateName := "skill-list.html"
	if mergeMode {
		templateName = "merge-skill-list.html"
	}

	if err := web.RenderPartial(w, templateName, data); err != nil {
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
		w.Header().Set("HX-Redirect", "/merge")
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, "/merge", http.StatusSeeOther)
	}
}

// Download returns the skill content as a downloadable file.
func (h *SkillsHandler) Download(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	// Validate slug length and format
	if len(slug) > MaxSlugLength || len(slug) == 0 {
		http.Error(w, "Invalid slug", http.StatusBadRequest)
		return
	}

	// Check for path traversal in slug
	if containsPathTraversal(slug) {
		http.Error(w, "Invalid slug", http.StatusBadRequest)
		return
	}

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

	// Validate source type against whitelist
	if !validSources[sourceType] {
		http.Error(w, "Invalid source type", http.StatusBadRequest)
		return
	}

	// URL decode the ID
	decodedID, err := url.PathUnescape(id)
	if err != nil {
		decodedID = id
	}

	// Validate ID to prevent path traversal
	if containsPathTraversal(decodedID) {
		http.Error(w, "Invalid skill ID", http.StatusBadRequest)
		return
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
		"Title": skill.Name + " - Skill MD",
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

	// Validate source type against whitelist
	if !validSources[sourceType] {
		http.Error(w, "Invalid source type", http.StatusBadRequest)
		return
	}

	// Validate id parameter
	if id == "" || len(id) > 500 {
		http.Error(w, "Invalid skill ID", http.StatusBadRequest)
		return
	}

	// Check for path traversal in id
	if containsPathTraversal(id) {
		http.Error(w, "Invalid skill ID", http.StatusBadRequest)
		return
	}

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
		http.Error(w, "Failed to import skill. Please try again.", http.StatusBadRequest)
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

	// Validate source type against whitelist
	if !validSources[sourceType] {
		http.Error(w, "Invalid source type", http.StatusBadRequest)
		return
	}

	decodedID, err := url.PathUnescape(id)
	if err != nil {
		decodedID = id
	}

	// Validate ID to prevent path traversal
	if containsPathTraversal(decodedID) {
		http.Error(w, "Invalid skill ID", http.StatusBadRequest)
		return
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
	w.Write([]byte(`<pre class="text-sm whitespace-pre-wrap overflow-auto max-h-[600px] bg-terminal-bg p-4 border border-terminal-border">` + html.EscapeString(content) + `</pre>`))
}
