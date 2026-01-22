package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const bitbucketAPIURL = "https://api.bitbucket.org/2.0"

// BitbucketSource provides access to SKILL.md files on Bitbucket.
type BitbucketSource struct {
	client   *http.Client
	username string
	password string
	enabled  bool
}

// bitbucketSearchResponse represents the Bitbucket search response.
type bitbucketSearchResponse struct {
	Size    int              `json:"size"`
	Page    int              `json:"page"`
	Pagelen int              `json:"pagelen"`
	Values  []bitbucketValue `json:"values"`
}

type bitbucketValue struct {
	Type         string           `json:"type"`
	ContentMatch bool             `json:"content_match"`
	PathMatches  []bitbucketMatch `json:"path_matches"`
	File         bitbucketFile    `json:"file"`
}

type bitbucketMatch struct {
	Text  string `json:"text"`
	Match bool   `json:"match"`
}

type bitbucketFile struct {
	Path   string             `json:"path"`
	Type   string             `json:"type"`
	Links  bitbucketFileLinks `json:"links"`
	Commit bitbucketCommit    `json:"commit"`
}

type bitbucketFileLinks struct {
	Self bitbucketLink `json:"self"`
}

type bitbucketLink struct {
	Href string `json:"href"`
}

type bitbucketCommit struct {
	Repository bitbucketRepo `json:"repository"`
}

type bitbucketRepo struct {
	Name     string         `json:"name"`
	FullName string         `json:"full_name"`
	UUID     string         `json:"uuid"`
	Links    bitbucketLinks `json:"links"`
}

type bitbucketLinks struct {
	HTML bitbucketLink `json:"html"`
}

// NewBitbucketSource creates a new Bitbucket source.
func NewBitbucketSource(username, password string) *BitbucketSource {
	return &BitbucketSource{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		username: username,
		password: password,
		enabled:  true,
	}
}

// Name returns the source type.
func (s *BitbucketSource) Name() SourceType {
	return SourceTypeBitbucket
}

// Enabled returns whether this source is enabled.
func (s *BitbucketSource) Enabled() bool {
	return s.enabled
}

// SetEnabled sets whether this source is enabled.
func (s *BitbucketSource) SetEnabled(enabled bool) {
	s.enabled = enabled
}

// Search finds SKILL.md files on Bitbucket.
func (s *BitbucketSource) Search(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	if opts.PerPage == 0 {
		opts.PerPage = 20
	}
	if opts.Page == 0 {
		opts.Page = 1
	}

	// Bitbucket code search query
	query := "SKILL.md"
	if opts.Query != "" {
		query = fmt.Sprintf("%s SKILL.md", opts.Query)
	}

	params := url.Values{}
	params.Set("search_query", query)
	params.Set("page", fmt.Sprintf("%d", opts.Page))
	params.Set("pagelen", fmt.Sprintf("%d", opts.PerPage))

	reqURL := fmt.Sprintf("%s/search/code?%s", bitbucketAPIURL, params.Encode())
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
			Source:  SourceTypeBitbucket,
			Page:    opts.Page,
			PerPage: opts.PerPage,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return &SearchResult{
			Skills:  []*ExternalSkill{},
			Source:  SourceTypeBitbucket,
			Page:    opts.Page,
			PerPage: opts.PerPage,
		}, nil
	}

	var searchResp bitbucketSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Filter and convert
	var skills []*ExternalSkill
	seen := make(map[string]bool)
	for _, item := range searchResp.Values {
		if !strings.HasSuffix(item.File.Path, "SKILL.md") {
			continue
		}
		repoKey := item.File.Commit.Repository.FullName
		if seen[repoKey] {
			continue
		}
		seen[repoKey] = true
		skills = append(skills, s.convertToExternal(&item))
	}

	return &SearchResult{
		Skills:  skills,
		Total:   searchResp.Size,
		Page:    opts.Page,
		PerPage: opts.PerPage,
		Source:  SourceTypeBitbucket,
	}, nil
}

// GetSkill retrieves a specific skill from Bitbucket.
func (s *BitbucketSource) GetSkill(ctx context.Context, id string) (*ExternalSkill, error) {
	// ID format: workspace/repo/path
	parts := strings.SplitN(id, "/", 3)
	if len(parts) < 2 {
		return nil, nil
	}

	workspace, repo := parts[0], parts[1]
	path := "SKILL.md"
	if len(parts) == 3 {
		path = parts[2]
	}

	// Get file content
	reqURL := fmt.Sprintf("%s/repositories/%s/%s/src/main/%s", bitbucketAPIURL, workspace, repo, path)
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

	content, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	return &ExternalSkill{
		ID:        id,
		Slug:      fmt.Sprintf("%s-%s", workspace, repo),
		Name:      fmt.Sprintf("%s/%s", workspace, repo),
		Content:   string(content),
		Source:    SourceTypeBitbucket,
		SourceURL: fmt.Sprintf("https://bitbucket.org/%s/%s/src/main/%s", workspace, repo, path),
		RepoOwner: workspace,
		RepoName:  repo,
	}, nil
}

// GetContent fetches the full content for a Bitbucket skill.
func (s *BitbucketSource) GetContent(ctx context.Context, skill *ExternalSkill) (string, error) {
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

// setHeaders sets common headers for Bitbucket API requests.
func (s *BitbucketSource) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "SkillMD/1.0")
	if s.username != "" && s.password != "" {
		req.SetBasicAuth(s.username, s.password)
	}
}

// convertToExternal converts a Bitbucket search result to ExternalSkill.
func (s *BitbucketSource) convertToExternal(item *bitbucketValue) *ExternalSkill {
	repo := item.File.Commit.Repository
	fullName := repo.FullName
	if fullName == "" {
		fullName = repo.Name
	}

	id := fmt.Sprintf("%s/%s", fullName, item.File.Path)

	return &ExternalSkill{
		ID:        id,
		Slug:      strings.ReplaceAll(fullName, "/", "-"),
		Name:      fullName,
		Source:    SourceTypeBitbucket,
		SourceURL: repo.Links.HTML.Href + "/src/main/" + item.File.Path,
		RepoName:  repo.Name,
	}
}
