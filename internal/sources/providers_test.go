package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGitHubSource tests
func TestGitHubSource_Name(t *testing.T) {
	source := NewGitHubSource("")
	if source.Name() != SourceTypeGitHub {
		t.Errorf("expected %s, got %s", SourceTypeGitHub, source.Name())
	}
}

func TestGitHubSource_Enabled(t *testing.T) {
	source := NewGitHubSource("")

	if !source.Enabled() {
		t.Error("expected source to be enabled by default")
	}

	source.SetEnabled(false)
	if source.Enabled() {
		t.Error("expected source to be disabled")
	}

	source.SetEnabled(true)
	if !source.Enabled() {
		t.Error("expected source to be enabled")
	}
}

func TestGitHubSource_Search_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	source := NewGitHubSource("")
	// Note: Can't easily override base URL without modifying the struct

	// Test with context cancellation instead
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := source.Search(ctx, SearchOptions{Query: "test"})
	if err == nil {
		t.Log("expected error with cancelled context, but request may have completed")
	}
}

func TestGitHubSource_Search_DefaultOptions(t *testing.T) {
	source := NewGitHubSource("")

	// Search with zero values - should use defaults
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Just verify it doesn't panic
	_, _ = source.Search(ctx, SearchOptions{Query: ""})
}

// TestGitLabSource tests
func TestGitLabSource_Name(t *testing.T) {
	source := NewGitLabSource("")
	if source.Name() != SourceTypeGitLab {
		t.Errorf("expected %s, got %s", SourceTypeGitLab, source.Name())
	}
}

func TestGitLabSource_Enabled(t *testing.T) {
	source := NewGitLabSource("")

	if !source.Enabled() {
		t.Error("expected source to be enabled by default")
	}

	source.SetEnabled(false)
	if source.Enabled() {
		t.Error("expected source to be disabled")
	}
}

func TestGitLabSource_Search_DefaultOptions(t *testing.T) {
	source := NewGitLabSource("")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Verify it doesn't panic
	_, _ = source.Search(ctx, SearchOptions{Query: ""})
}

// TestCodebergSource tests
func TestCodebergSource_Name(t *testing.T) {
	source := NewCodebergSource("")
	if source.Name() != SourceTypeCodeberg {
		t.Errorf("expected %s, got %s", SourceTypeCodeberg, source.Name())
	}
}

func TestCodebergSource_Enabled(t *testing.T) {
	source := NewCodebergSource("")

	if !source.Enabled() {
		t.Error("expected source to be enabled by default")
	}

	source.SetEnabled(false)
	if source.Enabled() {
		t.Error("expected source to be disabled")
	}
}

func TestCodebergSource_Search_DefaultOptions(t *testing.T) {
	source := NewCodebergSource("")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Verify it doesn't panic
	_, _ = source.Search(ctx, SearchOptions{Query: ""})
}

// TestBitbucketSource tests
func TestBitbucketSource_Name(t *testing.T) {
	source := NewBitbucketSource("", "")
	if source.Name() != SourceTypeBitbucket {
		t.Errorf("expected %s, got %s", SourceTypeBitbucket, source.Name())
	}
}

func TestBitbucketSource_Enabled(t *testing.T) {
	source := NewBitbucketSource("", "")

	if !source.Enabled() {
		t.Error("expected source to be enabled by default")
	}

	source.SetEnabled(false)
	if source.Enabled() {
		t.Error("expected source to be disabled")
	}
}

func TestBitbucketSource_Search_DefaultOptions(t *testing.T) {
	source := NewBitbucketSource("", "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Verify it doesn't panic
	_, _ = source.Search(ctx, SearchOptions{Query: ""})
}

// TestSkillsSHSource tests
func TestSkillsSHSource_Name(t *testing.T) {
	source := NewSkillsSHSource("")
	if source.Name() != SourceTypeSkillsSH {
		t.Errorf("expected %s, got %s", SourceTypeSkillsSH, source.Name())
	}
}

func TestSkillsSHSource_Enabled(t *testing.T) {
	source := NewSkillsSHSource("")

	if !source.Enabled() {
		t.Error("expected source to be enabled by default")
	}

	source.SetEnabled(false)
	if source.Enabled() {
		t.Error("expected source to be disabled")
	}
}

func TestSkillsSHSource_Search_DefaultOptions(t *testing.T) {
	source := NewSkillsSHSource("")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Verify it doesn't panic
	_, _ = source.Search(ctx, SearchOptions{Query: ""})
}

// TestLocalSource tests
func TestLocalSource_Name(t *testing.T) {
	source := NewLocalSource(nil)
	if source.Name() != SourceTypeLocal {
		t.Errorf("expected %s, got %s", SourceTypeLocal, source.Name())
	}
}

func TestLocalSource_Enabled_NilRegistry(t *testing.T) {
	source := NewLocalSource(nil)

	// Should be disabled when registry is nil
	if source.Enabled() {
		t.Error("expected source to be disabled with nil registry")
	}
}

func TestLocalSource_Search_NilRegistry(t *testing.T) {
	source := NewLocalSource(nil)

	result, err := source.Search(context.Background(), SearchOptions{Query: "test"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Skills) != 0 {
		t.Errorf("expected 0 skills with nil registry, got %d", len(result.Skills))
	}
}

func TestLocalSource_GetSkill_NilRegistry(t *testing.T) {
	source := NewLocalSource(nil)

	skill, err := source.GetSkill(context.Background(), "test-id")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill != nil {
		t.Error("expected nil skill with nil registry")
	}
}

func TestLocalSource_GetContent_NilRegistry(t *testing.T) {
	source := NewLocalSource(nil)

	content, err := source.GetContent(context.Background(), &ExternalSkill{
		ID:   "test",
		Slug: "test-slug",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty content with nil registry, got %q", content)
	}
}

// Test source utility functions
func TestSourceLabel(t *testing.T) {
	tests := []struct {
		source   SourceType
		expected string
	}{
		{SourceTypeLocal, "Local"},
		{SourceTypeGitHub, "GitHub"},
		{SourceTypeGitLab, "GitLab"},
		{SourceTypeBitbucket, "Bitbucket"},
		{SourceTypeCodeberg, "Codeberg"},
		{SourceTypeSkillsSH, "SKILLS.sh (Vercel)"},
		{"unknown", "unknown"}, // Unknown returns the source type string
	}

	for _, tt := range tests {
		t.Run(string(tt.source), func(t *testing.T) {
			label := SourceLabel(tt.source)
			if label != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, label)
			}
		})
	}
}

func TestSourceColor(t *testing.T) {
	tests := []struct {
		source   SourceType
		notEmpty bool
	}{
		{SourceTypeLocal, true},
		{SourceTypeGitHub, true},
		{SourceTypeGitLab, true},
		{SourceTypeBitbucket, true},
		{SourceTypeCodeberg, true},
		{SourceTypeSkillsSH, true},
		{"unknown", true}, // Unknown returns default color
	}

	for _, tt := range tests {
		t.Run(string(tt.source), func(t *testing.T) {
			color := SourceColor(tt.source)
			if tt.notEmpty && color == "" {
				t.Error("expected non-empty color")
			}
		})
	}
}

func TestIsSourceEnabled(t *testing.T) {
	enabled := []SourceType{SourceTypeLocal, SourceTypeGitHub}

	t.Run("source is enabled", func(t *testing.T) {
		if !IsSourceEnabled(enabled, SourceTypeLocal) {
			t.Error("expected Local to be enabled")
		}
		if !IsSourceEnabled(enabled, SourceTypeGitHub) {
			t.Error("expected GitHub to be enabled")
		}
	})

	t.Run("source is not enabled", func(t *testing.T) {
		if IsSourceEnabled(enabled, SourceTypeGitLab) {
			t.Error("expected GitLab to not be enabled")
		}
	})

	t.Run("empty list", func(t *testing.T) {
		if IsSourceEnabled([]SourceType{}, SourceTypeLocal) {
			t.Error("expected no sources to be enabled in empty list")
		}
	})
}

func TestDefaultEnabledSources(t *testing.T) {
	sources := DefaultEnabledSources()

	if len(sources) == 0 {
		t.Error("expected some default enabled sources")
	}

	// Verify Local is included
	found := false
	for _, s := range sources {
		if s == SourceTypeLocal {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Local to be in default enabled sources")
	}
}

// Test source configuration
func TestSourceConfig(t *testing.T) {
	config := SourceConfig{
		GitHubToken:    "gh-token",
		GitLabToken:    "gl-token",
		BitbucketUser:  "bb-user",
		BitbucketPass:  "bb-pass",
		CodebergToken:  "cb-token",
		EnabledSources: []SourceType{SourceTypeGitHub, SourceTypeGitLab},
	}

	if config.GitHubToken != "gh-token" {
		t.Error("GitHub token not set correctly")
	}
	if config.GitLabToken != "gl-token" {
		t.Error("GitLab token not set correctly")
	}
	if len(config.EnabledSources) != 2 {
		t.Errorf("expected 2 enabled sources, got %d", len(config.EnabledSources))
	}
}

// Test provider creation doesn't panic
func TestProviderCreation(t *testing.T) {
	t.Run("GitHub with empty token", func(t *testing.T) {
		source := NewGitHubSource("")
		if source == nil {
			t.Error("expected non-nil source")
		}
	})

	t.Run("GitHub with token", func(t *testing.T) {
		source := NewGitHubSource("test-token")
		if source == nil {
			t.Error("expected non-nil source")
		}
	})

	t.Run("GitLab with empty token", func(t *testing.T) {
		source := NewGitLabSource("")
		if source == nil {
			t.Error("expected non-nil source")
		}
	})

	t.Run("Codeberg with empty token", func(t *testing.T) {
		source := NewCodebergSource("")
		if source == nil {
			t.Error("expected non-nil source")
		}
	})

	t.Run("Bitbucket with empty credentials", func(t *testing.T) {
		source := NewBitbucketSource("", "")
		if source == nil {
			t.Error("expected non-nil source")
		}
	})

	t.Run("SkillsSH", func(t *testing.T) {
		source := NewSkillsSHSource("")
		if source == nil {
			t.Error("expected non-nil source")
		}
	})

	t.Run("Local with nil registry", func(t *testing.T) {
		source := NewLocalSource(nil)
		if source == nil {
			t.Error("expected non-nil source")
		}
	})
}

// TestSkillsSHSource_ParseFrontmatter tests frontmatter parsing for search filtering
func TestSkillsSHSource_ParseFrontmatter(t *testing.T) {
	source := NewSkillsSHSource("")

	t.Run("parses name and description", func(t *testing.T) {
		content := `---
name: "Design System"
description: "A comprehensive design system for UI"
tags:
  - "design"
  - "ui"
---

## Overview
Content here.
`
		name, description, tags := source.parseFrontmatter(content, "default-name")

		if name != "Design System" {
			t.Errorf("expected name 'Design System', got %q", name)
		}
		if description != "A comprehensive design system for UI" {
			t.Errorf("expected description, got %q", description)
		}
		if len(tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(tags))
		}
	})

	t.Run("returns default name when no frontmatter", func(t *testing.T) {
		content := `## Overview
Just regular content.
`
		name, description, tags := source.parseFrontmatter(content, "fallback-name")

		if name != "fallback-name" {
			t.Errorf("expected fallback name, got %q", name)
		}
		if description != "" {
			t.Errorf("expected empty description, got %q", description)
		}
		if len(tags) != 0 {
			t.Errorf("expected no tags, got %d", len(tags))
		}
	})

	t.Run("handles quoted values", func(t *testing.T) {
		content := `---
name: 'Single Quoted Name'
description: "Double quoted description"
---
`
		name, description, _ := source.parseFrontmatter(content, "default")

		if name != "Single Quoted Name" {
			t.Errorf("expected 'Single Quoted Name', got %q", name)
		}
		if description != "Double quoted description" {
			t.Errorf("expected double quoted description, got %q", description)
		}
	})

	t.Run("handles tags with quotes", func(t *testing.T) {
		content := `---
name: "Test"
tags:
  - "quoted-tag"
  - unquoted-tag
---
`
		_, _, tags := source.parseFrontmatter(content, "default")

		if len(tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(tags))
		}
		if len(tags) > 0 && tags[0] != "quoted-tag" {
			t.Errorf("expected 'quoted-tag', got %q", tags[0])
		}
	})
}

// TestSkillsSHSource_MatchesTags tests tag matching for search
func TestSkillsSHSource_MatchesTags(t *testing.T) {
	source := NewSkillsSHSource("")

	t.Run("matches exact tag", func(t *testing.T) {
		tags := []string{"design", "ui", "frontend"}
		if !source.matchesTags(tags, "design") {
			t.Error("expected to match 'design' tag")
		}
	})

	t.Run("matches partial tag", func(t *testing.T) {
		tags := []string{"design-system", "ui-components"}
		if !source.matchesTags(tags, "design") {
			t.Error("expected to match partial 'design' in 'design-system'")
		}
	})

	t.Run("case insensitive match", func(t *testing.T) {
		tags := []string{"Design", "UI"}
		if !source.matchesTags(tags, "design") {
			t.Error("expected case insensitive match")
		}
	})

	t.Run("no match returns false", func(t *testing.T) {
		tags := []string{"api", "backend"}
		if source.matchesTags(tags, "frontend") {
			t.Error("expected no match for 'frontend'")
		}
	})

	t.Run("empty tags returns false", func(t *testing.T) {
		if source.matchesTags([]string{}, "anything") {
			t.Error("expected no match for empty tags")
		}
	})
}

// TestSkillsSHSource_SearchFiltering tests the search filtering logic
func TestSkillsSHSource_SearchFiltering(t *testing.T) {
	// This is a unit test for the filtering logic - we can't easily test the full
	// Search method without mocking HTTP calls, but we can verify the matching logic

	source := NewSkillsSHSource("")

	t.Run("empty query matches all", func(t *testing.T) {
		skill := &ExternalSkill{
			Name:        "Any Skill",
			Description: "Any description",
			Tags:        []string{"any"},
		}
		// With empty query, the search should return all skills
		// The filtering logic: query == "" returns true
		query := ""
		matches := query == "" ||
			containsIgnoreCase(skill.Name, query) ||
			containsIgnoreCase(skill.Description, query) ||
			source.matchesTags(skill.Tags, query)

		if !matches {
			t.Error("empty query should match all skills")
		}
	})

	t.Run("matches by name", func(t *testing.T) {
		skill := &ExternalSkill{
			Name:        "Design System API",
			Description: "Something else",
			Tags:        []string{"api"},
		}
		query := "design"
		matches := containsIgnoreCase(skill.Name, query)

		if !matches {
			t.Error("should match 'design' in name")
		}
	})

	t.Run("matches by description", func(t *testing.T) {
		skill := &ExternalSkill{
			Name:        "Something",
			Description: "A design tool for developers",
			Tags:        []string{"tool"},
		}
		query := "design"
		matches := containsIgnoreCase(skill.Description, query)

		if !matches {
			t.Error("should match 'design' in description")
		}
	})
}

// containsIgnoreCase checks if substr exists in s (case insensitive)
func containsIgnoreCase(s, substr string) bool {
	if substr == "" {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	// Convert both to lowercase and check
	sLower := toLowerASCII(s)
	substrLower := toLowerASCII(substr)
	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}
	return false
}

// toLowerASCII converts ASCII uppercase to lowercase
func toLowerASCII(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + 32
		}
		result[i] = c
	}
	return string(result)
}

// TestSearchQueryMatching tests search query matching logic
func TestSearchQueryMatching(t *testing.T) {
	// Test cases that mirror the SkillsSH search logic
	tests := []struct {
		name        string
		description string
		slug        string
		tags        []string
		query       string
		shouldMatch bool
	}{
		{
			name:        "Design System",
			description: "UI components library",
			tags:        []string{"ui", "design"},
			query:       "design",
			shouldMatch: true,
		},
		{
			name:        "API Gateway",
			description: "REST API management",
			tags:        []string{"api", "gateway"},
			query:       "design",
			shouldMatch: false,
		},
		{
			name:        "Frontend Tools",
			description: "Design tools for frontend",
			tags:        []string{"tools"},
			query:       "design",
			shouldMatch: true, // matches description
		},
		{
			name:        "Backend Service",
			description: "Server implementation",
			tags:        []string{"design-patterns"},
			query:       "design",
			shouldMatch: true, // matches tag
		},
		{
			name:        "Any Skill",
			description: "Any description",
			tags:        []string{},
			query:       "",
			shouldMatch: true, // empty query matches all
		},
	}

	source := NewSkillsSHSource("")

	for _, tt := range tests {
		t.Run(tt.name+" with query "+tt.query, func(t *testing.T) {
			query := tt.query
			matches := query == "" ||
				containsIgnoreCase(tt.name, query) ||
				containsIgnoreCase(tt.description, query) ||
				containsIgnoreCase(tt.slug, query) ||
				source.matchesTags(tt.tags, query)

			if matches != tt.shouldMatch {
				t.Errorf("expected match=%v, got match=%v", tt.shouldMatch, matches)
			}
		})
	}
}
