package skill

import (
	"fmt"
	"strings"
)

// Render generates the SKILL.md content from a Skill struct.
func Render(s *Skill) string {
	var b strings.Builder

	// Write frontmatter
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("name: %q\n", s.Frontmatter.Name))
	b.WriteString(fmt.Sprintf("version: %q\n", s.Frontmatter.Version))
	if s.Frontmatter.Description != "" {
		b.WriteString(fmt.Sprintf("description: %q\n", s.Frontmatter.Description))
	}
	if s.Frontmatter.Author != "" {
		b.WriteString(fmt.Sprintf("author: %q\n", s.Frontmatter.Author))
	}
	if len(s.Frontmatter.Tags) > 0 {
		b.WriteString("tags:\n")
		for _, tag := range s.Frontmatter.Tags {
			b.WriteString(fmt.Sprintf("  - %q\n", tag))
		}
	}
	if s.Frontmatter.Source != "" {
		b.WriteString(fmt.Sprintf("source: %q\n", s.Frontmatter.Source))
	}
	if s.Frontmatter.SourceType != "" {
		b.WriteString(fmt.Sprintf("source_type: %q\n", s.Frontmatter.SourceType))
	}
	if s.Frontmatter.CreatedAt != "" {
		b.WriteString(fmt.Sprintf("created_at: %q\n", s.Frontmatter.CreatedAt))
	}
	if s.Frontmatter.UpdatedAt != "" {
		b.WriteString(fmt.Sprintf("updated_at: %q\n", s.Frontmatter.UpdatedAt))
	}
	// Enhanced metadata fields
	if s.Frontmatter.Difficulty != "" {
		b.WriteString(fmt.Sprintf("difficulty: %q\n", s.Frontmatter.Difficulty))
	}
	if s.Frontmatter.EndpointCount > 0 {
		b.WriteString(fmt.Sprintf("endpoint_count: %d\n", s.Frontmatter.EndpointCount))
	}
	if len(s.Frontmatter.AuthMethods) > 0 {
		b.WriteString("auth_methods:\n")
		for _, method := range s.Frontmatter.AuthMethods {
			b.WriteString(fmt.Sprintf("  - %q\n", method))
		}
	}
	if s.Frontmatter.BaseURL != "" {
		b.WriteString(fmt.Sprintf("base_url: %q\n", s.Frontmatter.BaseURL))
	}
	if s.Frontmatter.HasExamples {
		b.WriteString("has_examples: true\n")
	}
	b.WriteString("---\n\n")

	// Write sections
	for _, section := range s.Sections {
		b.WriteString(strings.Repeat("#", section.Level))
		b.WriteString(" ")
		b.WriteString(section.Title)
		b.WriteString("\n\n")
		if section.Content != "" {
			b.WriteString(section.Content)
			b.WriteString("\n\n")
		}
	}

	return strings.TrimSuffix(b.String(), "\n")
}

// RenderMinimal generates a minimal SKILL.md without frontmatter.
func RenderMinimal(s *Skill) string {
	var b strings.Builder

	b.WriteString("# ")
	b.WriteString(s.Frontmatter.Name)
	b.WriteString("\n\n")

	if s.Frontmatter.Description != "" {
		b.WriteString(s.Frontmatter.Description)
		b.WriteString("\n\n")
	}

	for _, section := range s.Sections {
		b.WriteString(strings.Repeat("#", section.Level))
		b.WriteString(" ")
		b.WriteString(section.Title)
		b.WriteString("\n\n")
		if section.Content != "" {
			b.WriteString(section.Content)
			b.WriteString("\n\n")
		}
	}

	return strings.TrimSuffix(b.String(), "\n")
}
