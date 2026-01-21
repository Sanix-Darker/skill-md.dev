package merger

import (
	"testing"

	"github.com/sanixdarker/skill-md/pkg/skill"
)

func TestMerger_Merge_EmptySkillsArray(t *testing.T) {
	m := New()
	result, err := m.Merge([]*skill.Skill{}, nil)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for empty skills array, got %v", result)
	}
}

func TestMerger_Merge_NilSkillsArray(t *testing.T) {
	m := New()
	result, err := m.Merge(nil, nil)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for nil skills array, got %v", result)
	}
}

func TestMerger_Merge_SingleSkill(t *testing.T) {
	m := New()
	input := skill.NewSkill("Test Skill", "A test description")
	input.Sections = []skill.Section{
		{Title: "Overview", Level: 2, Content: "This is an overview."},
	}

	result, err := m.Merge([]*skill.Skill{input}, nil)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != input {
		t.Errorf("expected same skill instance for single skill, got different instance")
	}
}

func TestMerger_Merge_TwoSkills(t *testing.T) {
	m := New()

	skill1 := skill.NewSkill("User API", "User management")
	skill1.Frontmatter.Tags = []string{"users", "api"}
	skill1.Sections = []skill.Section{
		{Title: "Overview", Level: 2, Content: "User API overview"},
		{Title: "Endpoints", Level: 2, Content: "GET /users"},
	}

	skill2 := skill.NewSkill("Product API", "Product catalog")
	skill2.Frontmatter.Tags = []string{"products", "api"}
	skill2.Sections = []skill.Section{
		{Title: "Overview", Level: 2, Content: "Product API overview"},
		{Title: "Endpoints", Level: 2, Content: "GET /products"},
	}

	result, err := m.Merge([]*skill.Skill{skill1, skill2}, nil)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Check name generation
	if result.Frontmatter.Name != "User API + Product API" {
		t.Errorf("expected name 'User API + Product API', got %q", result.Frontmatter.Name)
	}

	// Check description merging
	expectedDesc := "User management Product catalog"
	if result.Frontmatter.Description != expectedDesc {
		t.Errorf("expected description %q, got %q", expectedDesc, result.Frontmatter.Description)
	}

	// Check that sections are merged
	if len(result.Sections) != 2 {
		t.Errorf("expected 2 merged sections (Overview, Endpoints), got %d", len(result.Sections))
	}
}

func TestMerger_Merge_SectionGroupingCaseInsensitive(t *testing.T) {
	m := New()

	skill1 := skill.NewSkill("API 1", "")
	skill1.Sections = []skill.Section{
		{Title: "OVERVIEW", Level: 2, Content: "First overview"},
	}

	skill2 := skill.NewSkill("API 2", "")
	skill2.Sections = []skill.Section{
		{Title: "overview", Level: 2, Content: "Second overview"},
	}

	result, err := m.Merge([]*skill.Skill{skill1, skill2}, nil)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Sections with same title (case insensitive) should be merged
	if len(result.Sections) != 1 {
		t.Errorf("expected 1 merged section, got %d", len(result.Sections))
	}
}

func TestMerger_Merge_TagDeduplication(t *testing.T) {
	m := New()

	skill1 := skill.NewSkill("API 1", "")
	skill1.Frontmatter.Tags = []string{"api", "users", "common"}

	skill2 := skill.NewSkill("API 2", "")
	skill2.Frontmatter.Tags = []string{"api", "products", "common"}

	result, err := m.Merge([]*skill.Skill{skill1, skill2}, nil)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Tags should be deduplicated
	tagMap := make(map[string]int)
	for _, tag := range result.Frontmatter.Tags {
		tagMap[tag]++
	}

	for tag, count := range tagMap {
		if count > 1 {
			t.Errorf("tag %q appears %d times, expected 1", tag, count)
		}
	}

	// Should have exactly 4 unique tags: api, users, products, common
	if len(result.Frontmatter.Tags) != 4 {
		t.Errorf("expected 4 unique tags, got %d", len(result.Frontmatter.Tags))
	}
}

func TestMerger_Merge_NameGeneration(t *testing.T) {
	m := New()

	tests := []struct {
		name     string
		skills   []*skill.Skill
		opts     *Options
		expected string
	}{
		{
			name: "custom name from options",
			skills: []*skill.Skill{
				skill.NewSkill("API 1", ""),
				skill.NewSkill("API 2", ""),
			},
			opts:     &Options{Name: "Custom Merged API"},
			expected: "Custom Merged API",
		},
		{
			name: "two skills auto-named",
			skills: []*skill.Skill{
				skill.NewSkill("User API", ""),
				skill.NewSkill("Product API", ""),
			},
			opts:     nil,
			expected: "User API + Product API",
		},
		{
			name: "three skills auto-named",
			skills: []*skill.Skill{
				skill.NewSkill("API 1", ""),
				skill.NewSkill("API 2", ""),
				skill.NewSkill("API 3", ""),
			},
			opts:     nil,
			expected: "API 1 + API 2 + API 3",
		},
		{
			name: "four or more skills truncated",
			skills: []*skill.Skill{
				skill.NewSkill("API 1", ""),
				skill.NewSkill("API 2", ""),
				skill.NewSkill("API 3", ""),
				skill.NewSkill("API 4", ""),
			},
			opts:     nil,
			expected: "API 1 + API 2 + more",
		},
		{
			name: "skills without names",
			skills: []*skill.Skill{
				{Frontmatter: skill.Frontmatter{Name: ""}},
				{Frontmatter: skill.Frontmatter{Name: ""}},
			},
			opts:     nil,
			expected: "Merged Skill",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := m.Merge(tt.skills, tt.opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Frontmatter.Name != tt.expected {
				t.Errorf("expected name %q, got %q", tt.expected, result.Frontmatter.Name)
			}
		})
	}
}

func TestMerger_Merge_WithDeduplication(t *testing.T) {
	m := New()

	skill1 := skill.NewSkill("API 1", "")
	skill1.Sections = []skill.Section{
		{Title: "Overview", Level: 2, Content: "This is the API overview with some detailed information about the service."},
	}

	skill2 := skill.NewSkill("API 2", "")
	skill2.Sections = []skill.Section{
		{Title: "Overview", Level: 2, Content: "This is the API overview with some detailed information about the service."},
	}

	opts := &Options{Deduplicate: true}
	result, err := m.Merge([]*skill.Skill{skill1, skill2}, opts)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// With deduplication, identical content should be removed
	if result.Sections[0].Content == "" {
		t.Error("expected non-empty content after deduplication")
	}
}

func TestMerger_mergeSections_EmptyContent(t *testing.T) {
	m := New()

	sections := []skill.Section{
		{Title: "Test", Level: 2, Content: ""},
		{Title: "Test", Level: 2, Content: "  "},
		{Title: "Test", Level: 2, Content: "Actual content"},
	}

	result := m.mergeSections(sections)

	// Empty/whitespace content should be skipped
	if result.Content != "Actual content" {
		t.Errorf("expected 'Actual content', got %q", result.Content)
	}
}

func TestMerger_mergeSections_ExactDuplicates(t *testing.T) {
	m := New()

	sections := []skill.Section{
		{Title: "Test", Level: 2, Content: "Same content"},
		{Title: "Test", Level: 2, Content: "Same content"},
		{Title: "Test", Level: 2, Content: "Different content"},
	}

	result := m.mergeSections(sections)

	// Exact duplicates should be removed
	expected := "Same content\n\nDifferent content"
	if result.Content != expected {
		t.Errorf("expected %q, got %q", expected, result.Content)
	}
}

func TestMerger_mergeSections_SingleSection(t *testing.T) {
	m := New()

	section := skill.Section{Title: "Test", Level: 2, Content: "Content"}
	result := m.mergeSections([]skill.Section{section})

	if result.Title != section.Title || result.Content != section.Content {
		t.Error("single section should be returned unchanged")
	}
}

func TestMerger_Merge_PreservesFirstSectionLevel(t *testing.T) {
	m := New()

	skill1 := skill.NewSkill("API 1", "")
	skill1.Sections = []skill.Section{
		{Title: "Overview", Level: 3, Content: "First"},
	}

	skill2 := skill.NewSkill("API 2", "")
	skill2.Sections = []skill.Section{
		{Title: "Overview", Level: 2, Content: "Second"},
	}

	result, err := m.Merge([]*skill.Skill{skill1, skill2}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use the first section's level
	if result.Sections[0].Level != 3 {
		t.Errorf("expected level 3, got %d", result.Sections[0].Level)
	}
}

func TestMerger_Merge_CustomDescription(t *testing.T) {
	m := New()

	skill1 := skill.NewSkill("API 1", "First description")
	skill2 := skill.NewSkill("API 2", "Second description")

	opts := &Options{Description: "Custom combined description"}
	result, err := m.Merge([]*skill.Skill{skill1, skill2}, opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Frontmatter.Description != "Custom combined description" {
		t.Errorf("expected custom description, got %q", result.Frontmatter.Description)
	}
}

func TestMerger_Merge_SetsCreatedAt(t *testing.T) {
	m := New()

	skill1 := skill.NewSkill("API 1", "")
	skill2 := skill.NewSkill("API 2", "")

	result, err := m.Merge([]*skill.Skill{skill1, skill2}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Frontmatter.CreatedAt == "" {
		t.Error("expected CreatedAt to be set")
	}
}

func TestMerger_Merge_SectionOrderPreserved(t *testing.T) {
	m := New()

	skill1 := skill.NewSkill("API 1", "")
	skill1.Sections = []skill.Section{
		{Title: "Overview", Level: 2, Content: "Overview 1"},
		{Title: "Authentication", Level: 2, Content: "Auth 1"},
		{Title: "Endpoints", Level: 2, Content: "Endpoints 1"},
	}

	skill2 := skill.NewSkill("API 2", "")
	skill2.Sections = []skill.Section{
		{Title: "Endpoints", Level: 2, Content: "Endpoints 2"},
		{Title: "Authentication", Level: 2, Content: "Auth 2"},
	}

	result, err := m.Merge([]*skill.Skill{skill1, skill2}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Section order should be preserved based on first appearance
	expected := []string{"Overview", "Authentication", "Endpoints"}
	for i, section := range result.Sections {
		if section.Title != expected[i] {
			t.Errorf("section %d: expected %q, got %q", i, expected[i], section.Title)
		}
	}
}
