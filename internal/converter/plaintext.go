package converter

import (
	"path/filepath"
	"strings"

	"github.com/sanixdarker/skillforge/pkg/skill"
)

// PlainTextConverter converts plain text to SKILL.md.
type PlainTextConverter struct{}

func (c *PlainTextConverter) Name() string {
	return "text"
}

func (c *PlainTextConverter) CanHandle(filename string, content []byte) bool {
	// Plain text is the fallback, accepts everything
	ext := getExtension(filename)
	return ext == ".txt" || ext == ".md" || ext == ""
}

func (c *PlainTextConverter) Convert(content []byte, opts *Options) (*skill.Skill, error) {
	name := "Skill"
	if opts != nil && opts.Name != "" {
		name = opts.Name
	} else if opts != nil && opts.SourcePath != "" {
		// Use filename without extension as name
		base := filepath.Base(opts.SourcePath)
		name = strings.TrimSuffix(base, filepath.Ext(base))
		name = strings.Title(strings.ReplaceAll(name, "-", " "))
		name = strings.Title(strings.ReplaceAll(name, "_", " "))
	}

	s := skill.NewSkill(name, "")
	s.Frontmatter.SourceType = "text"
	if opts != nil && opts.SourcePath != "" {
		s.Frontmatter.Source = opts.SourcePath
	}

	// Parse the content looking for structure
	text := string(content)

	// Check if already has markdown headers
	if strings.Contains(text, "\n# ") || strings.HasPrefix(text, "# ") {
		// Parse existing markdown structure
		existingSkill, err := skill.Parse(text)
		if err == nil && len(existingSkill.Sections) > 0 {
			s.Sections = existingSkill.Sections
			if existingSkill.Frontmatter.Name != "" {
				s.Frontmatter.Name = existingSkill.Frontmatter.Name
			}
			if existingSkill.Frontmatter.Description != "" {
				s.Frontmatter.Description = existingSkill.Frontmatter.Description
			}
			return s, nil
		}
	}

	// Simple text - create a single content section
	s.AddSection("Description", 2, strings.TrimSpace(text))

	return s, nil
}
