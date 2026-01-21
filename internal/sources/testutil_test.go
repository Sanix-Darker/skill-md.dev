package sources

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
)

// MockResponse represents a mock HTTP response.
type MockResponse struct {
	StatusCode int
	Body       string
	Headers    map[string]string
	Error      error
}

// MockRoundTripper implements http.RoundTripper for testing.
type MockRoundTripper struct {
	mu        sync.Mutex
	Responses map[string]*MockResponse
	Requests  []*http.Request
	// DefaultResponse is returned when no specific response is configured
	DefaultResponse *MockResponse
}

// NewMockRoundTripper creates a new MockRoundTripper.
func NewMockRoundTripper() *MockRoundTripper {
	return &MockRoundTripper{
		Responses: make(map[string]*MockResponse),
		Requests:  make([]*http.Request, 0),
		DefaultResponse: &MockResponse{
			StatusCode: http.StatusOK,
			Body:       "{}",
		},
	}
}

// RoundTrip implements http.RoundTripper.
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	m.Requests = append(m.Requests, req)
	m.mu.Unlock()

	// Check for configured response by URL
	url := req.URL.String()

	m.mu.Lock()
	resp, ok := m.Responses[url]
	m.mu.Unlock()

	if !ok {
		// Try matching by path only
		m.mu.Lock()
		resp, ok = m.Responses[req.URL.Path]
		m.mu.Unlock()
	}

	if !ok {
		// Try matching by host + path
		m.mu.Lock()
		resp, ok = m.Responses[req.URL.Host+req.URL.Path]
		m.mu.Unlock()
	}

	if !ok {
		resp = m.DefaultResponse
	}

	if resp.Error != nil {
		return nil, resp.Error
	}

	httpResp := &http.Response{
		StatusCode: resp.StatusCode,
		Body:       io.NopCloser(strings.NewReader(resp.Body)),
		Header:     make(http.Header),
		Request:    req,
	}

	for k, v := range resp.Headers {
		httpResp.Header.Set(k, v)
	}

	return httpResp, nil
}

// AddResponse adds a mock response for a specific URL.
func (m *MockRoundTripper) AddResponse(url string, resp *MockResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Responses[url] = resp
}

// GetRequests returns all recorded requests.
func (m *MockRoundTripper) GetRequests() []*http.Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*http.Request{}, m.Requests...)
}

// ClearRequests clears all recorded requests.
func (m *MockRoundTripper) ClearRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Requests = make([]*http.Request, 0)
}

// NewMockHTTPClient creates an http.Client with the mock transport.
func NewMockHTTPClient(responses map[string]*MockResponse) *http.Client {
	rt := NewMockRoundTripper()
	for url, resp := range responses {
		rt.AddResponse(url, resp)
	}
	return &http.Client{Transport: rt}
}

// mockGitHubSearchResponse returns a mock GitHub search response.
func mockGitHubSearchResponse() string {
	return `{
		"total_count": 2,
		"incomplete_results": false,
		"items": [
			{
				"id": 12345,
				"name": "skill-api",
				"full_name": "testuser/skill-api",
				"description": "Test skill for API operations",
				"html_url": "https://github.com/testuser/skill-api",
				"owner": {
					"login": "testuser"
				},
				"stargazers_count": 100,
				"updated_at": "2024-01-15T10:00:00Z"
			},
			{
				"id": 12346,
				"name": "skill-web",
				"full_name": "testuser/skill-web",
				"description": "Test skill for web operations",
				"html_url": "https://github.com/testuser/skill-web",
				"owner": {
					"login": "testuser"
				},
				"stargazers_count": 50,
				"updated_at": "2024-01-14T10:00:00Z"
			}
		]
	}`
}

// mockGitLabSearchResponse returns a mock GitLab search response.
func mockGitLabSearchResponse() string {
	return `[
		{
			"id": 12345,
			"name": "skill-api",
			"path_with_namespace": "testuser/skill-api",
			"description": "Test skill for API operations",
			"web_url": "https://gitlab.com/testuser/skill-api",
			"namespace": {
				"path": "testuser"
			},
			"star_count": 100,
			"last_activity_at": "2024-01-15T10:00:00Z"
		},
		{
			"id": 12346,
			"name": "skill-web",
			"path_with_namespace": "testuser/skill-web",
			"description": "Test skill for web operations",
			"web_url": "https://gitlab.com/testuser/skill-web",
			"namespace": {
				"path": "testuser"
			},
			"star_count": 50,
			"last_activity_at": "2024-01-14T10:00:00Z"
		}
	]`
}

// mockCodebergSearchResponse returns a mock Codeberg search response.
func mockCodebergSearchResponse() string {
	return `{
		"ok": true,
		"data": [
			{
				"id": 12345,
				"name": "skill-api",
				"full_name": "testuser/skill-api",
				"description": "Test skill for API operations",
				"html_url": "https://codeberg.org/testuser/skill-api",
				"owner": {
					"login": "testuser"
				},
				"stars_count": 100,
				"updated_at": "2024-01-15T10:00:00Z"
			}
		]
	}`
}

// mockSkillMDContent returns mock SKILL.md content.
func mockSkillMDContent() string {
	return `---
name: "Test API"
version: "1.0.0"
description: "A test API skill"
tags:
  - "api"
  - "test"
---

## Overview

This is a test API skill.

## Endpoints

### GET /test

Returns test data.
`
}

// mockGitHubContentResponse returns a mock GitHub content API response.
func mockGitHubContentResponse() string {
	return `{
		"name": "SKILL.md",
		"path": "SKILL.md",
		"sha": "abc123",
		"size": 200,
		"url": "https://api.github.com/repos/testuser/skill-api/contents/SKILL.md",
		"html_url": "https://github.com/testuser/skill-api/blob/main/SKILL.md",
		"git_url": "https://api.github.com/repos/testuser/skill-api/git/blobs/abc123",
		"download_url": "https://raw.githubusercontent.com/testuser/skill-api/main/SKILL.md",
		"type": "file",
		"content": "LS0tCm5hbWU6ICJUZXN0IEFQSSIKdmVyc2lvbjogIjEuMC4wIgpkZXNjcmlwdGlvbjogIkEgdGVzdCBBUEkgc2tpbGwiCnRhZ3M6CiAgLSAiYXBpIgogIC0gInRlc3QiCi0tLQoKIyMgT3ZlcnZpZXcKClRoaXMgaXMgYSB0ZXN0IEFQSSBza2lsbC4K",
		"encoding": "base64"
	}`
}

// mockGitLabFileResponse returns a mock GitLab file API response.
func mockGitLabFileResponse() string {
	return `{
		"file_name": "SKILL.md",
		"file_path": "SKILL.md",
		"size": 200,
		"encoding": "base64",
		"content": "LS0tCm5hbWU6ICJUZXN0IEFQSSIKdmVyc2lvbjogIjEuMC4wIgpkZXNjcmlwdGlvbjogIkEgdGVzdCBBUEkgc2tpbGwiCnRhZ3M6CiAgLSAiYXBpIgogIC0gInRlc3QiCi0tLQoKIyMgT3ZlcnZpZXcKClRoaXMgaXMgYSB0ZXN0IEFQSSBza2lsbC4K",
		"ref": "main"
	}`
}

// MockSource implements Source interface for testing.
type MockSource struct {
	name         SourceType
	enabled      bool
	searchResult *SearchResult
	searchErr    error
	skills       map[string]*ExternalSkill
	getSkillErr  error
	contentMap   map[string]string
	contentErr   error
}

// NewMockSource creates a new MockSource.
func NewMockSource(name SourceType) *MockSource {
	return &MockSource{
		name:       name,
		enabled:    true,
		skills:     make(map[string]*ExternalSkill),
		contentMap: make(map[string]string),
	}
}

// Name returns the source type.
func (m *MockSource) Name() SourceType {
	return m.name
}

// Enabled returns whether the source is enabled.
func (m *MockSource) Enabled() bool {
	return m.enabled
}

// SetEnabled sets the enabled state.
func (m *MockSource) SetEnabled(enabled bool) {
	m.enabled = enabled
}

// Search returns mock search results.
func (m *MockSource) Search(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	if m.searchResult != nil {
		return m.searchResult, nil
	}
	return &SearchResult{
		Skills:  []*ExternalSkill{},
		Total:   0,
		Page:    opts.Page,
		PerPage: opts.PerPage,
		Source:  m.name,
	}, nil
}

// SetSearchResult sets the mock search result.
func (m *MockSource) SetSearchResult(result *SearchResult) {
	m.searchResult = result
}

// SetSearchError sets the mock search error.
func (m *MockSource) SetSearchError(err error) {
	m.searchErr = err
}

// GetSkill returns a mock skill.
func (m *MockSource) GetSkill(ctx context.Context, id string) (*ExternalSkill, error) {
	if m.getSkillErr != nil {
		return nil, m.getSkillErr
	}
	return m.skills[id], nil
}

// SetSkill sets a mock skill.
func (m *MockSource) SetSkill(id string, skill *ExternalSkill) {
	m.skills[id] = skill
}

// SetGetSkillError sets the mock GetSkill error.
func (m *MockSource) SetGetSkillError(err error) {
	m.getSkillErr = err
}

// GetContent returns mock content.
func (m *MockSource) GetContent(ctx context.Context, skill *ExternalSkill) (string, error) {
	if m.contentErr != nil {
		return "", m.contentErr
	}
	if content, ok := m.contentMap[skill.ID]; ok {
		return content, nil
	}
	return skill.Content, nil
}

// SetContent sets mock content.
func (m *MockSource) SetContent(id, content string) {
	m.contentMap[id] = content
}

// SetContentError sets the mock content error.
func (m *MockSource) SetContentError(err error) {
	m.contentErr = err
}
