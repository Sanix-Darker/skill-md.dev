package sources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const githubAPIURL = "https://api.github.com"

// GitHubSource provides access to SKILL.md files on GitHub.
type GitHubSource struct {
	client  *http.Client
	token   string
	enabled bool
}

// githubSearchResponse represents the GitHub code search response.
type githubSearchResponse struct {
	TotalCount int          `json:"total_count"`
	Items      []githubItem `json:"items"`
}

type githubItem struct {
	Name       string         `json:"name"`
	Path       string         `json:"path"`
	HTMLURL    string         `json:"html_url"`
	Repository githubRepo     `json:"repository"`
	SHA        string         `json:"sha"`
	URL        string         `json:"url"`
}

type githubRepo struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	FullName        string `json:"full_name"`
	Description     string `json:"description"`
	HTMLURL         string `json:"html_url"`
	StargazersCount int    `json:"stargazers_count"`
	Owner           struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type githubContentResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

// NewGitHubSource creates a new GitHub source.
func NewGitHubSource(token string) *GitHubSource {
	return &GitHubSource{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		token:   token,
		enabled: true,
	}
}

// Name returns the source type.
func (s *GitHubSource) Name() SourceType {
	return SourceTypeGitHub
}

// Enabled returns whether this source is enabled.
func (s *GitHubSource) Enabled() bool {
	return s.enabled
}

// SetEnabled sets whether this source is enabled.
func (s *GitHubSource) SetEnabled(enabled bool) {
	s.enabled = enabled
}

// Search finds SKILL.md files on GitHub.
func (s *GitHubSource) Search(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	if opts.PerPage == 0 {
		opts.PerPage = 20
	}
	if opts.Page == 0 {
		opts.Page = 1
	}

	// GitHub code search query
	query := "filename:SKILL.md"
	if opts.Query != "" {
		query = fmt.Sprintf("%s %s", opts.Query, query)
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("page", fmt.Sprintf("%d", opts.Page))
	params.Set("per_page", fmt.Sprintf("%d", opts.PerPage))

	reqURL := fmt.Sprintf("%s/search/code?%s", githubAPIURL, params.Encode())
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
		// Rate limited or unauthorized - return empty results
		return &SearchResult{
			Skills:  []*ExternalSkill{},
			Source:  SourceTypeGitHub,
			Page:    opts.Page,
			PerPage: opts.PerPage,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return &SearchResult{
			Skills:  []*ExternalSkill{},
			Source:  SourceTypeGitHub,
			Page:    opts.Page,
			PerPage: opts.PerPage,
		}, nil
	}

	var searchResp githubSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	skills := make([]*ExternalSkill, len(searchResp.Items))
	for i, item := range searchResp.Items {
		skills[i] = s.convertToExternal(&item)
	}

	return &SearchResult{
		Skills:  skills,
		Total:   searchResp.TotalCount,
		Page:    opts.Page,
		PerPage: opts.PerPage,
		Source:  SourceTypeGitHub,
	}, nil
}

// GetSkill retrieves a specific skill from GitHub.
func (s *GitHubSource) GetSkill(ctx context.Context, id string) (*ExternalSkill, error) {
	// ID format: owner/repo/path
	parts := strings.SplitN(id, "/", 3)
	if len(parts) < 2 {
		return nil, nil
	}

	owner, repo := parts[0], parts[1]
	path := "SKILL.md"
	if len(parts) == 3 {
		path = parts[2]
	}

	reqURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s", githubAPIURL, owner, repo, path)
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

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var contentResp githubContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&contentResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	content, err := base64.StdEncoding.DecodeString(contentResp.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to decode content: %w", err)
	}

	// Get repo info for stars
	repoResp, err := s.getRepoInfo(ctx, owner, repo)
	if err != nil {
		repoResp = &githubRepo{
			Name:     repo,
			FullName: fmt.Sprintf("%s/%s", owner, repo),
		}
	}

	return &ExternalSkill{
		ID:          id,
		Slug:        fmt.Sprintf("%s-%s", owner, repo),
		Name:        fmt.Sprintf("%s/%s", owner, repo),
		Description: repoResp.Description,
		Content:     string(content),
		Source:      SourceTypeGitHub,
		SourceURL:   fmt.Sprintf("https://github.com/%s/%s/blob/main/%s", owner, repo, path),
		RepoOwner:   owner,
		RepoName:    repo,
		Stars:       repoResp.StargazersCount,
	}, nil
}

// GetContent fetches the full content for a GitHub skill.
func (s *GitHubSource) GetContent(ctx context.Context, skill *ExternalSkill) (string, error) {
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

// getRepoInfo fetches repository information.
func (s *GitHubSource) getRepoInfo(ctx context.Context, owner, repo string) (*githubRepo, error) {
	reqURL := fmt.Sprintf("%s/repos/%s/%s", githubAPIURL, owner, repo)
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %d", resp.StatusCode)
	}

	var repoResp githubRepo
	if err := json.NewDecoder(resp.Body).Decode(&repoResp); err != nil {
		return nil, err
	}

	return &repoResp, nil
}

// setHeaders sets common headers for GitHub API requests.
func (s *GitHubSource) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "SkillMD/1.0")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
}

// convertToExternal converts a GitHub item to ExternalSkill.
func (s *GitHubSource) convertToExternal(item *githubItem) *ExternalSkill {
	id := fmt.Sprintf("%s/%s", item.Repository.FullName, item.Path)

	return &ExternalSkill{
		ID:          id,
		Slug:        strings.ReplaceAll(item.Repository.FullName, "/", "-"),
		Name:        item.Repository.FullName,
		Description: item.Repository.Description,
		Source:      SourceTypeGitHub,
		SourceURL:   item.HTMLURL,
		ContentURL:  item.URL,
		RepoOwner:   item.Repository.Owner.Login,
		RepoName:    item.Repository.Name,
		Stars:       item.Repository.StargazersCount,
	}
}
