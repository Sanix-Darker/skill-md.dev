package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/sanixdarker/skillf/internal/app"
	"github.com/sanixdarker/skillf/internal/merger"
	"github.com/sanixdarker/skillf/internal/server/middleware"
	"github.com/sanixdarker/skillf/internal/sources"
	"github.com/sanixdarker/skillf/pkg/skill"
	"github.com/sanixdarker/skillf/web"
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

type mergeInputMetadata struct {
	Index      int
	Name       string
	Source     string
	Type       string
	Identifier string
	Version    string
}

type mergeConflictValueMetadata struct {
	Source     string
	Name       string
	Version    string
	SourceType string
	Value      string
}

type mergeConflictMetadata struct {
	Field    string
	Values   []mergeConflictValueMetadata
	Resolved string
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
		"Title": "Merge - Skillf",
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
	var inputMetadata []*mergeInputMetadata

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
			s = applySourceMetadata(s, ref.Source, ref.Name)
			inputMetadata = append(inputMetadata, &mergeInputMetadata{
				Index:      len(inputMetadata) + 1,
				Name:       pickMergeInputName(s, ref.Name),
				Source:     skillSourceLabel(s, len(inputMetadata)),
				Type:       mergeSourceTypeFromRef(ref.Source),
				Identifier: ref.ID,
				Version:    strings.TrimSpace(s.Frontmatter.Version),
			})

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
			s = applySourceMetadata(s, "upload", fileHeader.Filename)
			inputMetadata = append(inputMetadata, &mergeInputMetadata{
				Index:      len(inputMetadata) + 1,
				Name:       pickMergeInputName(s, fileHeader.Filename),
				Source:     skillSourceLabel(s, len(inputMetadata)),
				Type:       "upload",
				Identifier: fileHeader.Filename,
				Version:    strings.TrimSpace(s.Frontmatter.Version),
			})

			skills = append(skills, s)
		}
	}

	// Get options
	name := r.FormValue("name")
	description := r.FormValue("description")
	dedupe := r.FormValue("dedupe") == "true" || r.FormValue("dedupe") == "on"
	strategy, err := merger.ParseConflictStrategy(r.FormValue("strategy"))
	if err != nil {
		h.app.Logger.Error("invalid conflict strategy", "value", r.FormValue("strategy"), "error", err)
		h.renderError(w, r, "Invalid merge strategy. Use keep_first, keep_last, keep_longer, or combine.")
		return
	}

	// Merge
	result, err := h.app.Merger.Merge(skills, &merger.Options{
		Name:             name,
		Description:      description,
		Deduplicate:      dedupe,
		ConflictStrategy: strategy,
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
	conflicts := merger.DetectConflicts(skills, strategy)
	metadataConflicts := buildMergeConflictMetadata(conflicts, inputMetadata)
	sourcesSummary := summarizeMergeSources(inputMetadata)

	mergeInputs := make([]mergeInputMetadata, 0, len(inputMetadata))
	for _, input := range inputMetadata {
		if input != nil {
			mergeInputs = append(mergeInputs, *input)
		}
	}

	// Return result
	if middleware.IsHTMXRequest(r) {
		data := map[string]interface{}{
			"Content":            output,
			"Name":               result.Frontmatter.Name,
			"SkillCount":         len(skills),
			"Conflicts":          conflicts,
			"MergeConflicts":     metadataConflicts,
			"ConflictCount":      len(conflicts),
			"ConflictStrategy":   strategy.String(),
			"MergeInputCount":    len(mergeInputs),
			"MergeInputs":        mergeInputs,
			"MergeSourceSummary": sourcesSummary,
			"MergeStrategyLabel": mergeStrategyDisplayName(strategy),
			"MergeDedupe":        dedupe,
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

func buildMergeConflictMetadata(conflicts []merger.Conflict, inputMetadata []*mergeInputMetadata) []mergeConflictMetadata {
	metadataBySource := map[string]mergeInputMetadata{}
	for _, input := range inputMetadata {
		if input == nil {
			continue
		}
		metadataBySource[input.Source] = *input
	}

	mergedConflicts := make([]mergeConflictMetadata, 0, len(conflicts))
	for _, conflict := range conflicts {
		values := make([]mergeConflictValueMetadata, 0, len(conflict.Values))
		for _, value := range conflict.Values {
			item := mergeConflictValueMetadata{
				Source: value.Source,
				Value:  value.Value,
			}
			if meta, ok := metadataBySource[value.Source]; ok {
				item.Name = meta.Name
				item.Version = meta.Version
				item.SourceType = meta.Type
			}
			values = append(values, item)
		}
		mergedConflicts = append(mergedConflicts, mergeConflictMetadata{
			Field:    conflict.Field,
			Values:   values,
			Resolved: conflict.Resolved,
		})
	}

	return mergedConflicts
}

func summarizeMergeSources(inputs []*mergeInputMetadata) []string {
	counts := make(map[string]int)
	for _, input := range inputs {
		if input == nil {
			continue
		}
		counts[humanizeMergeSource(input.Type)]++
	}

	keys := make([]string, 0, len(counts))
	for source := range counts {
		keys = append(keys, source)
	}
	sort.Strings(keys)

	summary := make([]string, 0, len(keys))
	for _, source := range keys {
		summary = append(summary, fmt.Sprintf("%s %d", source, counts[source]))
	}

	return summary
}

func mergeSourceTypeFromRef(source string) string {
	source = strings.TrimSpace(strings.ToLower(source))
	if source == "" {
		return "upload"
	}
	return source
}

func mergeStrategyDisplayName(strategy merger.ConflictStrategy) string {
	switch strategy {
	case merger.KeepFirst:
		return "keep_first (safe default)"
	case merger.KeepLast:
		return "keep_last"
	case merger.KeepLonger:
		return "keep_longer"
	case merger.Combine:
		return "combine"
	default:
		return strategy.String()
	}
}

func pickMergeInputName(s *skill.Skill, fallback string) string {
	name := strings.TrimSpace(s.Frontmatter.Name)
	if name == "" {
		name = strings.TrimSpace(fallback)
	}
	if name == "" {
		name = "(unnamed)"
	}
	return name
}

func skillSourceLabel(s *skill.Skill, index int) string {
	source := strings.TrimSpace(s.Frontmatter.Source)
	if source == "" {
		source = "uploaded"
	}
	return fmt.Sprintf("%s #%d", source, index+1)
}

func humanizeMergeSource(source string) string {
	switch source {
	case "local":
		return "Local"
	case "skills.sh":
		return "SKILLS.sh"
	case "github":
		return "GitHub"
	case "gitlab":
		return "GitLab"
	case "bitbucket":
		return "Bitbucket"
	case "codeberg":
		return "Codeberg"
	case "upload":
		return "Upload"
	default:
		if source == "" {
			return "Upload"
		}
		return source
	}
}

// Strategies returns the supported merge strategies.
func (h *MergeHandler) Strategies(w http.ResponseWriter, r *http.Request) {
	type strategyPayload struct {
		Value       string `json:"value"`
		Label       string `json:"label"`
		Description string `json:"description"`
	}

	strategies := merger.SupportedStrategies()
	payload := make([]strategyPayload, 0, len(strategies))

	for _, strategy := range strategies {
		payload = append(payload, strategyPayload{
			Value:       strategy.Value,
			Label:       strategy.Label,
			Description: strategy.Description,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		h.app.Logger.Error("failed to render merge strategies", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
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

func applySourceMetadata(s *skill.Skill, source string, hint string) *skill.Skill {
	if s == nil {
		return s
	}
	if s.Frontmatter.Source == "" {
		s.Frontmatter.Source = source
	}
	if s.Frontmatter.SourceType == "" {
		s.Frontmatter.SourceType = source
	}
	if s.Frontmatter.Name == "" && strings.TrimSpace(hint) != "" {
		s.Frontmatter.Name = strings.TrimSpace(hint)
	}
	return s
}

func (h *MergeHandler) renderError(w http.ResponseWriter, r *http.Request, msg string) {
	if middleware.IsHTMXRequest(r) {
		w.Header().Set("X-Skillf-Error", "true")
		data := map[string]interface{}{
			"Error": msg,
		}
		web.RenderPartial(w, "error.html", data)
	} else {
		http.Error(w, msg, http.StatusBadRequest)
	}
}
