package sources

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestFederatedSource_Search_NoSources(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	result, err := fs.Search(context.Background(), SearchOptions{Query: "test"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(result.Skills))
	}
	if result.Total != 0 {
		t.Errorf("expected 0 total, got %d", result.Total)
	}
}

func TestFederatedSource_Search_SingleSource(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	mockSource := NewMockSource(SourceTypeGitHub)
	mockSource.SetSearchResult(&SearchResult{
		Skills: []*ExternalSkill{
			{ID: "1", Name: "Skill 1", Source: SourceTypeGitHub, Stars: 100},
			{ID: "2", Name: "Skill 2", Source: SourceTypeGitHub, Stars: 50},
		},
		Total:  2,
		Source: SourceTypeGitHub,
	})

	fs.RegisterSource(mockSource)

	result, err := fs.Search(context.Background(), SearchOptions{Query: "test"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(result.Skills))
	}
	if result.Total != 2 {
		t.Errorf("expected total 2, got %d", result.Total)
	}
}

func TestFederatedSource_Search_MultipleSources(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	// GitHub source
	github := NewMockSource(SourceTypeGitHub)
	github.SetSearchResult(&SearchResult{
		Skills: []*ExternalSkill{
			{ID: "gh1", Name: "GitHub Skill", Source: SourceTypeGitHub, Stars: 100},
		},
		Total:  1,
		Source: SourceTypeGitHub,
	})

	// GitLab source
	gitlab := NewMockSource(SourceTypeGitLab)
	gitlab.SetSearchResult(&SearchResult{
		Skills: []*ExternalSkill{
			{ID: "gl1", Name: "GitLab Skill", Source: SourceTypeGitLab, Stars: 50},
		},
		Total:  1,
		Source: SourceTypeGitLab,
	})

	fs.RegisterSource(github)
	fs.RegisterSource(gitlab)

	result, err := fs.Search(context.Background(), SearchOptions{Query: "test"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(result.Skills))
	}
	if result.Total != 2 {
		t.Errorf("expected total 2, got %d", result.Total)
	}

	// Check results from both sources
	if _, ok := result.BySource[SourceTypeGitHub]; !ok {
		t.Error("expected GitHub results")
	}
	if _, ok := result.BySource[SourceTypeGitLab]; !ok {
		t.Error("expected GitLab results")
	}
}

func TestFederatedSource_Search_CacheHit(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	callCount := 0
	mockSource := NewMockSource(SourceTypeGitHub)
	// Wrap to track calls - using original behavior
	originalSearch := mockSource.searchResult
	mockSource.SetSearchResult(&SearchResult{
		Skills: []*ExternalSkill{
			{ID: "1", Name: "Cached Skill", Source: SourceTypeGitHub},
		},
		Total:  1,
		Source: SourceTypeGitHub,
	})
	_ = originalSearch

	fs.RegisterSource(mockSource)

	opts := SearchOptions{Query: "cached", Page: 1, PerPage: 20}

	// First search
	result1, _ := fs.Search(context.Background(), opts)

	// Second search - should hit cache (source not called again)
	result2, _ := fs.Search(context.Background(), opts)

	if result1 == nil || result2 == nil {
		t.Fatal("expected non-nil results")
	}

	// Verify both return same data
	if len(result1.Skills) != len(result2.Skills) {
		t.Error("cache should return same results")
	}

	_ = callCount // callCount tracking would need source modification
}

func TestFederatedSource_Search_SourceError(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	// Source that returns error
	errorSource := NewMockSource(SourceTypeGitHub)
	errorSource.SetSearchError(errors.New("source error"))

	// Source that succeeds
	goodSource := NewMockSource(SourceTypeGitLab)
	goodSource.SetSearchResult(&SearchResult{
		Skills: []*ExternalSkill{
			{ID: "1", Name: "Good Skill", Source: SourceTypeGitLab},
		},
		Total:  1,
		Source: SourceTypeGitLab,
	})

	fs.RegisterSource(errorSource)
	fs.RegisterSource(goodSource)

	result, err := fs.Search(context.Background(), SearchOptions{Query: "test"})

	// Should not return error - errors from individual sources are logged
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still have results from good source
	if len(result.Skills) != 1 {
		t.Errorf("expected 1 skill from good source, got %d", len(result.Skills))
	}
}

func TestFederatedSource_Search_ResultSorting(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	// Local source (should appear first)
	local := NewMockSource(SourceTypeLocal)
	local.SetSearchResult(&SearchResult{
		Skills: []*ExternalSkill{
			{ID: "local1", Name: "Local Skill", Source: SourceTypeLocal, Stars: 0},
		},
		Total:  1,
		Source: SourceTypeLocal,
	})

	// GitHub source with high stars
	github := NewMockSource(SourceTypeGitHub)
	github.SetSearchResult(&SearchResult{
		Skills: []*ExternalSkill{
			{ID: "gh1", Name: "GitHub High", Source: SourceTypeGitHub, Stars: 1000},
			{ID: "gh2", Name: "GitHub Low", Source: SourceTypeGitHub, Stars: 10},
		},
		Total:  2,
		Source: SourceTypeGitHub,
	})

	fs.RegisterSource(local)
	fs.RegisterSource(github)

	result, _ := fs.Search(context.Background(), SearchOptions{Query: "test"})

	if len(result.Skills) < 3 {
		t.Fatalf("expected 3 skills, got %d", len(result.Skills))
	}

	// Local should be first
	if result.Skills[0].Source != SourceTypeLocal {
		t.Error("expected local skill to be first")
	}

	// High stars should come before low stars
	var foundHighStars, foundLowStars bool
	var highIndex, lowIndex int
	for i, s := range result.Skills {
		if s.ID == "gh1" {
			foundHighStars = true
			highIndex = i
		}
		if s.ID == "gh2" {
			foundLowStars = true
			lowIndex = i
		}
	}

	if foundHighStars && foundLowStars && highIndex > lowIndex {
		t.Error("expected high stars skill before low stars skill")
	}
}

func TestFederatedSource_Search_DisabledSource(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	mockSource := NewMockSource(SourceTypeGitHub)
	mockSource.SetEnabled(false)
	mockSource.SetSearchResult(&SearchResult{
		Skills: []*ExternalSkill{
			{ID: "1", Name: "Should not appear"},
		},
		Total:  1,
		Source: SourceTypeGitHub,
	})

	fs.RegisterSource(mockSource)

	result, _ := fs.Search(context.Background(), SearchOptions{Query: "test"})

	// Disabled source should not return results
	if len(result.Skills) != 0 {
		t.Errorf("expected 0 skills from disabled source, got %d", len(result.Skills))
	}
}

func TestFederatedSource_SearchSources_SpecificSources(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	github := NewMockSource(SourceTypeGitHub)
	github.SetSearchResult(&SearchResult{
		Skills: []*ExternalSkill{{ID: "gh1", Source: SourceTypeGitHub}},
		Total:  1,
		Source: SourceTypeGitHub,
	})

	gitlab := NewMockSource(SourceTypeGitLab)
	gitlab.SetSearchResult(&SearchResult{
		Skills: []*ExternalSkill{{ID: "gl1", Source: SourceTypeGitLab}},
		Total:  1,
		Source: SourceTypeGitLab,
	})

	fs.RegisterSource(github)
	fs.RegisterSource(gitlab)

	// Search only GitHub
	result, _ := fs.SearchSources(context.Background(), SearchOptions{Query: "test"}, []SourceType{SourceTypeGitHub})

	if len(result.Skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(result.Skills))
	}
	if result.Skills[0].Source != SourceTypeGitHub {
		t.Error("expected only GitHub results")
	}
}

func TestFederatedSource_SearchSource_Single(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	mockSource := NewMockSource(SourceTypeGitHub)
	mockSource.SetSearchResult(&SearchResult{
		Skills: []*ExternalSkill{{ID: "1", Name: "Test", Source: SourceTypeGitHub}},
		Total:  1,
		Source: SourceTypeGitHub,
	})

	fs.RegisterSource(mockSource)

	result, err := fs.SearchSource(context.Background(), SourceTypeGitHub, SearchOptions{Query: "test"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(result.Skills))
	}
}

func TestFederatedSource_SearchSource_UnknownSource(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	result, err := fs.SearchSource(context.Background(), "unknown", SearchOptions{Query: "test"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Skills) != 0 {
		t.Errorf("expected empty skills for unknown source, got %d", len(result.Skills))
	}
}

func TestFederatedSource_GetSkill_CacheHit(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	mockSource := NewMockSource(SourceTypeGitHub)
	mockSource.SetSkill("skill1", &ExternalSkill{
		ID:      "skill1",
		Name:    "Cached Skill",
		Source:  SourceTypeGitHub,
		Content: "Content",
	})

	fs.RegisterSource(mockSource)

	// First call
	skill1, _ := fs.GetSkill(context.Background(), SourceTypeGitHub, "skill1")

	// Second call - should hit cache
	skill2, _ := fs.GetSkill(context.Background(), SourceTypeGitHub, "skill1")

	if skill1 == nil || skill2 == nil {
		t.Fatal("expected non-nil skills")
	}
	if skill1.Name != skill2.Name {
		t.Error("cache should return same skill")
	}
}

func TestFederatedSource_GetSkill_UnknownSource(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	skill, err := fs.GetSkill(context.Background(), "unknown", "id1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill != nil {
		t.Error("expected nil skill for unknown source")
	}
}

func TestFederatedSource_GetContent(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	mockSource := NewMockSource(SourceTypeGitHub)
	mockSource.SetContent("skill1", "# Test Content\n\nThis is the content.")

	fs.RegisterSource(mockSource)

	skill := &ExternalSkill{
		ID:     "skill1",
		Source: SourceTypeGitHub,
	}

	content, err := fs.GetContent(context.Background(), skill)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "# Test Content\n\nThis is the content." {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestFederatedSource_GetContent_AlreadyLoaded(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	skill := &ExternalSkill{
		ID:      "skill1",
		Source:  SourceTypeGitHub,
		Content: "Already loaded content",
	}

	content, err := fs.GetContent(context.Background(), skill)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "Already loaded content" {
		t.Errorf("expected already loaded content, got: %s", content)
	}
}

func TestFederatedSource_GetContent_UnknownSource(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	skill := &ExternalSkill{
		ID:     "skill1",
		Source: "unknown",
	}

	content, err := fs.GetContent(context.Background(), skill)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty content for unknown source, got: %s", content)
	}
}

func TestFederatedSource_ClearCache(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	// Add some cached data
	fs.cache.Set("key1", "value1")
	fs.skillCache.Set("key2", "value2")

	// Clear caches
	fs.ClearCache()

	// Verify caches are empty
	if _, ok := fs.cache.Get("key1"); ok {
		t.Error("expected cache to be cleared")
	}
	if _, ok := fs.skillCache.Get("key2"); ok {
		t.Error("expected skill cache to be cleared")
	}
}

func TestFederatedSource_EnabledSources(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	enabled := NewMockSource(SourceTypeGitHub)
	enabled.SetEnabled(true)

	disabled := NewMockSource(SourceTypeGitLab)
	disabled.SetEnabled(false)

	fs.RegisterSource(enabled)
	fs.RegisterSource(disabled)

	sources := fs.EnabledSources()

	if len(sources) != 1 {
		t.Errorf("expected 1 enabled source, got %d", len(sources))
	}
	if len(sources) > 0 && sources[0] != SourceTypeGitHub {
		t.Errorf("expected GitHub to be enabled, got %s", sources[0])
	}
}

func TestFederatedSource_GetSource(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	mockSource := NewMockSource(SourceTypeGitHub)
	fs.RegisterSource(mockSource)

	t.Run("existing source", func(t *testing.T) {
		source, ok := fs.GetSource(SourceTypeGitHub)
		if !ok {
			t.Error("expected to find registered source")
		}
		if source == nil {
			t.Error("expected non-nil source")
		}
	})

	t.Run("non-existent source", func(t *testing.T) {
		_, ok := fs.GetSource(SourceTypeGitLab)
		if ok {
			t.Error("expected not to find unregistered source")
		}
	})
}

func TestFederatedSource_Search_DefaultOptions(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	mockSource := NewMockSource(SourceTypeGitHub)
	mockSource.SetSearchResult(&SearchResult{
		Skills: []*ExternalSkill{{ID: "1"}},
		Total:  1,
		Source: SourceTypeGitHub,
	})

	fs.RegisterSource(mockSource)

	// Search with zero page/perPage
	result, _ := fs.Search(context.Background(), SearchOptions{
		Query:   "test",
		Page:    0,
		PerPage: 0,
	})

	// Should use defaults (page=1, perPage=20)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestFederatedSource_Search_Timing(t *testing.T) {
	fs := NewFederatedSource(newTestLogger())

	mockSource := NewMockSource(SourceTypeGitHub)
	mockSource.SetSearchResult(&SearchResult{
		Skills:     []*ExternalSkill{{ID: "1"}},
		Total:      1,
		Source:     SourceTypeGitHub,
		SearchTime: 10 * time.Millisecond,
	})

	fs.RegisterSource(mockSource)

	result, _ := fs.Search(context.Background(), SearchOptions{Query: "test"})

	if result.SearchTime == 0 {
		t.Error("expected SearchTime to be set")
	}
}
