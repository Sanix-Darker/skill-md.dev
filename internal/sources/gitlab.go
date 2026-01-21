package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const gitlabAPIURL = "https://gitlab.com/api/v4"

// GitLabSource provides access to SKILL.md files on GitLab.
type GitLabSource struct {
	client  *http.Client
	token   string
	enabled bool
}

// gitlabSearchResponse represents the GitLab search response.
type gitlabSearchResponse struct {
	Data       string `json:"data"`
	Filename   string `json:"filename"`
	Ref        string `json:"ref"`
	Startline  int    `json:"startline"`
	ProjectID  int    `json:"project_id"`
	Path       string `json:"path"`
}

type gitlabProject struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	PathWithNS      string `json:"path_with_namespace"`
	WebURL          string `json:"web_url"`
	StarCount       int    `json:"star_count"`
	DefaultBranch   string `json:"default_branch"`
}

// NewGitLabSource creates a new GitLab source.
func NewGitLabSource(token string) *GitLabSource {
	return &GitLabSource{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		token:   token,
		enabled: true,
	}
}

// Name returns the source type.
func (s *GitLabSource) Name() SourceType {
	return SourceTypeGitLab
}

// Enabled returns whether this source is enabled.
func (s *GitLabSource) Enabled() bool {
	return s.enabled
}

// SetEnabled sets whether this source is enabled.
func (s *GitLabSource) SetEnabled(enabled bool) {
	s.enabled = enabled
}

// Search finds SKILL.md files on GitLab.
func (s *GitLabSource) Search(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	if opts.PerPage == 0 {
		opts.PerPage = 20
	}
	if opts.Page == 0 {
		opts.Page = 1
	}

	// GitLab blob search query
	query := "SKILL.md"
	if opts.Query != "" {
		query = fmt.Sprintf("%s SKILL.md", opts.Query)
	}

	params := url.Values{}
	params.Set("scope", "blobs")
	params.Set("search", query)
	params.Set("page", fmt.Sprintf("%d", opts.Page))
	params.Set("per_page", fmt.Sprintf("%d", opts.PerPage))

	reqURL := fmt.Sprintf("%s/search?%s", gitlabAPIURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return &SearchResult{
			Skills:  []*ExternalSkill{},
			Source:  SourceTypeGitLab,
			Page:    opts.Page,
			PerPage: opts.PerPage,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return &SearchResult{
			Skills:  []*ExternalSkill{},
			Source:  SourceTypeGitLab,
			Page:    opts.Page,
			PerPage: opts.PerPage,
		}, nil
	}

	var searchResp []gitlabSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Filter to only SKILL.md files and convert
	var skills []*ExternalSkill
	seen := make(map[int]bool)
	for _, item := range searchResp {
		if !strings.HasSuffix(item.Filename, "SKILL.md") {
			continue
		}
		if seen[item.ProjectID] {
			continue
		}
		seen[item.ProjectID] = true

		skill, err := s.itemToSkill(ctx, &item)
		if err != nil {
			continue
		}
		skills = append(skills, skill)
	}

	return &SearchResult{
		Skills:  skills,
		Total:   len(skills),
		Page:    opts.Page,
		PerPage: opts.PerPage,
		Source:  SourceTypeGitLab,
	}, nil
}

// GetSkill retrieves a specific skill from GitLab.
func (s *GitLabSource) GetSkill(ctx context.Context, id string) (*ExternalSkill, error) {
	// ID format: projectID/path
	parts := strings.SplitN(id, "/", 2)
	if len(parts) < 1 {
		return nil, nil
	}

	projectID := parts[0]
	path := "SKILL.md"
	if len(parts) == 2 {
		path = parts[1]
	}

	// Get project info first
	project, err := s.getProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, nil
	}

	// Get file content
	content, err := s.getFileContent(ctx, projectID, path, project.DefaultBranch)
	if err != nil {
		return nil, err
	}

	return &ExternalSkill{
		ID:          id,
		Slug:        strings.ReplaceAll(project.PathWithNS, "/", "-"),
		Name:        project.PathWithNS,
		Description: project.Description,
		Content:     content,
		Source:      SourceTypeGitLab,
		SourceURL:   fmt.Sprintf("%s/-/blob/%s/%s", project.WebURL, project.DefaultBranch, path),
		Stars:       project.StarCount,
	}, nil
}

// GetContent fetches the full content for a GitLab skill.
func (s *GitLabSource) GetContent(ctx context.Context, skill *ExternalSkill) (string, error) {
	if skill.Content != "" {
		return skill.Content, nil
	}

	fullSkill, err := s.GetSkill(ctx, skill.ID)
	if err != nil {
		return "", err
	}
	if fullSkill != nil {
		return fullSkill.Content, nil
	}
	return "", nil
}

// getProject fetches project information.
func (s *GitLabSource) getProject(ctx context.Context, projectID string) (*gitlabProject, error) {
	reqURL := fmt.Sprintf("%s/projects/%s", gitlabAPIURL, url.PathEscape(projectID))
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %d", resp.StatusCode)
	}

	var project gitlabProject
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, err
	}

	return &project, nil
}

// getFileContent fetches file content from a project.
func (s *GitLabSource) getFileContent(ctx context.Context, projectID, path, ref string) (string, error) {
	if ref == "" {
		ref = "main"
	}

	reqURL := fmt.Sprintf("%s/projects/%s/repository/files/%s/raw?ref=%s",
		gitlabAPIURL, url.PathEscape(projectID), url.PathEscape(path), url.QueryEscape(ref))
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return "", err
	}

	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status: %d", resp.StatusCode)
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
		if len(content) > 1024*1024 {
			break
		}
	}

	return string(content), nil
}

// setHeaders sets common headers for GitLab API requests.
func (s *GitLabSource) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "SkillMD/1.0")
	if s.token != "" {
		req.Header.Set("PRIVATE-TOKEN", s.token)
	}
}

// itemToSkill converts a GitLab search result to ExternalSkill.
func (s *GitLabSource) itemToSkill(ctx context.Context, item *gitlabSearchResponse) (*ExternalSkill, error) {
	project, err := s.getProject(ctx, fmt.Sprintf("%d", item.ProjectID))
	if err != nil || project == nil {
		return &ExternalSkill{
			ID:        fmt.Sprintf("%d/%s", item.ProjectID, item.Path),
			Slug:      fmt.Sprintf("gitlab-%d", item.ProjectID),
			Name:      item.Filename,
			Source:    SourceTypeGitLab,
			SourceURL: fmt.Sprintf("https://gitlab.com/projects/%d", item.ProjectID),
		}, nil
	}

	return &ExternalSkill{
		ID:          fmt.Sprintf("%d/%s", item.ProjectID, item.Path),
		Slug:        strings.ReplaceAll(project.PathWithNS, "/", "-"),
		Name:        project.PathWithNS,
		Description: project.Description,
		Source:      SourceTypeGitLab,
		SourceURL:   fmt.Sprintf("%s/-/blob/%s/%s", project.WebURL, item.Ref, item.Path),
		Stars:       project.StarCount,
	}, nil
}
