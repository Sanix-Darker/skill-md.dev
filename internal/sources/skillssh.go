package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const skillsshBaseURL = "https://skills.sh/api"

// SkillsSHSource provides access to the SKILLS.sh registry.
type SkillsSHSource struct {
	client  *http.Client
	baseURL string
	enabled bool
}

// skillsshSearchResponse represents the API response.
type skillsshSearchResponse struct {
	Skills []skillsshSkill `json:"skills"`
	Total  int             `json:"total"`
	Page   int             `json:"page"`
}

type skillsshSkill struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Version     string   `json:"version"`
	ContentURL  string   `json:"content_url"`
	URL         string   `json:"url"`
	UpdatedAt   string   `json:"updated_at"`
}

// NewSkillsSHSource creates a new SKILLS.sh source.
func NewSkillsSHSource() *SkillsSHSource {
	return &SkillsSHSource{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: skillsshBaseURL,
		enabled: true,
	}
}

// Name returns the source type.
func (s *SkillsSHSource) Name() SourceType {
	return SourceTypeSkillsSH
}

// Enabled returns whether this source is enabled.
func (s *SkillsSHSource) Enabled() bool {
	return s.enabled
}

// SetEnabled sets whether this source is enabled.
func (s *SkillsSHSource) SetEnabled(enabled bool) {
	s.enabled = enabled
}

// Search finds skills on SKILLS.sh.
func (s *SkillsSHSource) Search(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	if opts.PerPage == 0 {
		opts.PerPage = 20
	}
	if opts.Page == 0 {
		opts.Page = 1
	}

	params := url.Values{}
	params.Set("q", opts.Query)
	params.Set("page", fmt.Sprintf("%d", opts.Page))
	params.Set("per_page", fmt.Sprintf("%d", opts.PerPage))

	reqURL := fmt.Sprintf("%s/skills/search?%s", s.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "SkillForge/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &SearchResult{
			Skills:  []*ExternalSkill{},
			Source:  SourceTypeSkillsSH,
			Page:    opts.Page,
			PerPage: opts.PerPage,
		}, nil
	}

	var searchResp skillsshSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	skills := make([]*ExternalSkill, len(searchResp.Skills))
	for i, sk := range searchResp.Skills {
		skills[i] = s.convertToExternal(&sk)
	}

	return &SearchResult{
		Skills:  skills,
		Total:   searchResp.Total,
		Page:    opts.Page,
		PerPage: opts.PerPage,
		Source:  SourceTypeSkillsSH,
	}, nil
}

// GetSkill retrieves a specific skill from SKILLS.sh.
func (s *SkillsSHSource) GetSkill(ctx context.Context, id string) (*ExternalSkill, error) {
	reqURL := fmt.Sprintf("%s/skills/%s", s.baseURL, url.PathEscape(id))
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "SkillForge/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var sk skillsshSkill
	if err := json.NewDecoder(resp.Body).Decode(&sk); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return s.convertToExternal(&sk), nil
}

// GetContent fetches the full content for a SKILLS.sh skill.
func (s *SkillsSHSource) GetContent(ctx context.Context, skill *ExternalSkill) (string, error) {
	if skill.ContentURL == "" {
		return "", nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", skill.ContentURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "text/plain")
	req.Header.Set("User-Agent", "SkillForge/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var content []byte
	content = make([]byte, 0, 64*1024)
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			content = append(content, buf[:n]...)
		}
		if err != nil {
			break
		}
		if len(content) > 1024*1024 { // 1MB limit
			break
		}
	}

	return string(content), nil
}

// convertToExternal converts a SKILLS.sh skill to ExternalSkill.
func (s *SkillsSHSource) convertToExternal(sk *skillsshSkill) *ExternalSkill {
	var updatedAt time.Time
	if sk.UpdatedAt != "" {
		updatedAt, _ = time.Parse(time.RFC3339, sk.UpdatedAt)
	}

	sourceURL := sk.URL
	if sourceURL == "" {
		sourceURL = fmt.Sprintf("https://skills.sh/skill/%s", sk.Slug)
	}

	return &ExternalSkill{
		ID:          sk.ID,
		Slug:        sk.Slug,
		Name:        sk.Name,
		Description: sk.Description,
		Tags:        sk.Tags,
		Source:      SourceTypeSkillsSH,
		SourceURL:   sourceURL,
		ContentURL:  sk.ContentURL,
		Version:     sk.Version,
		UpdatedAt:   updatedAt,
	}
}
