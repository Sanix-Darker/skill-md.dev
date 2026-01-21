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

const codebergAPIURL = "https://codeberg.org/api/v1"

// CodebergSource provides access to SKILL.md files on Codeberg (Gitea-based).
type CodebergSource struct {
	client  *http.Client
	token   string
	enabled bool
}

// codebergSearchResponse represents the Codeberg search response.
type codebergSearchResponse struct {
	OK   bool            `json:"ok"`
	Data []codebergRepo  `json:"data"`
}

type codebergRepo struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	FullName        string `json:"full_name"`
	Description     string `json:"description"`
	HTMLURL         string `json:"html_url"`
	StarsCount      int    `json:"stars_count"`
	DefaultBranch   string `json:"default_branch"`
	Owner           codebergOwner `json:"owner"`
}

type codebergOwner struct {
	Login string `json:"login"`
}

type codebergContentResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	HTMLURL  string `json:"html_url"`
}

// NewCodebergSource creates a new Codeberg source.
func NewCodebergSource(token string) *CodebergSource {
	return &CodebergSource{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		token:   token,
		enabled: true,
	}
}

// Name returns the source type.
func (s *CodebergSource) Name() SourceType {
	return SourceTypeCodeberg
}

// Enabled returns whether this source is enabled.
func (s *CodebergSource) Enabled() bool {
	return s.enabled
}

// SetEnabled sets whether this source is enabled.
func (s *CodebergSource) SetEnabled(enabled bool) {
	s.enabled = enabled
}

// Search finds repositories with SKILL.md files on Codeberg.
func (s *CodebergSource) Search(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	if opts.PerPage == 0 {
		opts.PerPage = 20
	}
	if opts.Page == 0 {
		opts.Page = 1
	}

	// Codeberg repo search (Gitea API)
	query := opts.Query
	if query == "" {
		query = "skill"
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("page", fmt.Sprintf("%d", opts.Page))
	params.Set("limit", fmt.Sprintf("%d", opts.PerPage))

	reqURL := fmt.Sprintf("%s/repos/search?%s", codebergAPIURL, params.Encode())
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
			Source:  SourceTypeCodeberg,
			Page:    opts.Page,
			PerPage: opts.PerPage,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return &SearchResult{
			Skills:  []*ExternalSkill{},
			Source:  SourceTypeCodeberg,
			Page:    opts.Page,
			PerPage: opts.PerPage,
		}, nil
	}

	var searchResp codebergSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check each repo for SKILL.md
	var skills []*ExternalSkill
	for _, repo := range searchResp.Data {
		hasSkill := s.checkForSkillMD(ctx, repo.Owner.Login, repo.Name, repo.DefaultBranch)
		if hasSkill {
			skills = append(skills, s.convertToExternal(&repo))
		}
	}

	return &SearchResult{
		Skills:  skills,
		Total:   len(skills),
		Page:    opts.Page,
		PerPage: opts.PerPage,
		Source:  SourceTypeCodeberg,
	}, nil
}

// GetSkill retrieves a specific skill from Codeberg.
func (s *CodebergSource) GetSkill(ctx context.Context, id string) (*ExternalSkill, error) {
	// ID format: owner/repo
	parts := strings.SplitN(id, "/", 2)
	if len(parts) < 2 {
		return nil, nil
	}

	owner, repo := parts[0], parts[1]

	// Get repo info first
	repoInfo, err := s.getRepoInfo(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	if repoInfo == nil {
		return nil, nil
	}

	// Get SKILL.md content
	content, err := s.getFileContent(ctx, owner, repo, "SKILL.md", repoInfo.DefaultBranch)
	if err != nil {
		return nil, err
	}

	return &ExternalSkill{
		ID:          id,
		Slug:        fmt.Sprintf("%s-%s", owner, repo),
		Name:        repoInfo.FullName,
		Description: repoInfo.Description,
		Content:     content,
		Source:      SourceTypeCodeberg,
		SourceURL:   fmt.Sprintf("https://codeberg.org/%s/%s/src/branch/%s/SKILL.md", owner, repo, repoInfo.DefaultBranch),
		RepoOwner:   owner,
		RepoName:    repo,
		Stars:       repoInfo.StarsCount,
	}, nil
}

// GetContent fetches the full content for a Codeberg skill.
func (s *CodebergSource) GetContent(ctx context.Context, skill *ExternalSkill) (string, error) {
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

// checkForSkillMD checks if a repo has a SKILL.md file.
func (s *CodebergSource) checkForSkillMD(ctx context.Context, owner, repo, branch string) bool {
	if branch == "" {
		branch = "main"
	}

	reqURL := fmt.Sprintf("%s/repos/%s/%s/contents/SKILL.md?ref=%s", codebergAPIURL, owner, repo, branch)
	req, err := http.NewRequestWithContext(ctx, "HEAD", reqURL, nil)
	if err != nil {
		return false
	}

	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// getRepoInfo fetches repository information.
func (s *CodebergSource) getRepoInfo(ctx context.Context, owner, repo string) (*codebergRepo, error) {
	reqURL := fmt.Sprintf("%s/repos/%s/%s", codebergAPIURL, owner, repo)
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

	var repoInfo codebergRepo
	if err := json.NewDecoder(resp.Body).Decode(&repoInfo); err != nil {
		return nil, err
	}

	return &repoInfo, nil
}

// getFileContent fetches file content from a repo.
func (s *CodebergSource) getFileContent(ctx context.Context, owner, repo, path, branch string) (string, error) {
	if branch == "" {
		branch = "main"
	}

	reqURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", codebergAPIURL, owner, repo, url.PathEscape(path), branch)
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

	var contentResp codebergContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&contentResp); err != nil {
		return "", err
	}

	if contentResp.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(contentResp.Content)
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	}

	return contentResp.Content, nil
}

// setHeaders sets common headers for Codeberg API requests.
func (s *CodebergSource) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "SkillMD/1.0")
	if s.token != "" {
		req.Header.Set("Authorization", "token "+s.token)
	}
}

// convertToExternal converts a Codeberg repo to ExternalSkill.
func (s *CodebergSource) convertToExternal(repo *codebergRepo) *ExternalSkill {
	branch := repo.DefaultBranch
	if branch == "" {
		branch = "main"
	}

	return &ExternalSkill{
		ID:          repo.FullName,
		Slug:        strings.ReplaceAll(repo.FullName, "/", "-"),
		Name:        repo.FullName,
		Description: repo.Description,
		Source:      SourceTypeCodeberg,
		SourceURL:   fmt.Sprintf("https://codeberg.org/%s/src/branch/%s/SKILL.md", repo.FullName, branch),
		RepoOwner:   repo.Owner.Login,
		RepoName:    repo.Name,
		Stars:       repo.StarsCount,
	}
}
