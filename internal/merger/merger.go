// Package merger provides skill merging functionality.
package merger

import (
	"strings"
	"time"

	"github.com/sanixdarker/skillforge/pkg/skill"
)

// Options holds merge options.
type Options struct {
	Name        string
	Description string
	Deduplicate bool
}

// Merger merges multiple skills into one.
type Merger struct {
	dedup *Deduplicator
}

// New creates a new Merger.
func New() *Merger {
	return &Merger{
		dedup: NewDeduplicator(),
	}
}

// Merge combines multiple skills into a single skill.
func (m *Merger) Merge(skills []*skill.Skill, opts *Options) (*skill.Skill, error) {
	if len(skills) == 0 {
		return nil, nil
	}

	if len(skills) == 1 {
		return skills[0], nil
	}

	// Determine name
	name := "Merged Skill"
	if opts != nil && opts.Name != "" {
		name = opts.Name
	} else {
		// Combine first skill names
		names := make([]string, 0, len(skills))
		for _, s := range skills {
			if s.Frontmatter.Name != "" {
				names = append(names, s.Frontmatter.Name)
			}
		}
		if len(names) > 0 {
			if len(names) <= 3 {
				name = strings.Join(names, " + ")
			} else {
				name = strings.Join(names[:2], " + ") + " + more"
			}
		}
	}

	// Create merged skill
	result := skill.NewSkill(name, "")
	result.Frontmatter.CreatedAt = time.Now().Format(time.RFC3339)

	// Collect descriptions
	var descriptions []string
	for _, s := range skills {
		if s.Frontmatter.Description != "" {
			descriptions = append(descriptions, s.Frontmatter.Description)
		}
	}
	if opts != nil && opts.Description != "" {
		result.Frontmatter.Description = opts.Description
	} else if len(descriptions) > 0 {
		result.Frontmatter.Description = strings.Join(descriptions, " ")
	}

	// Merge tags
	tagSet := make(map[string]bool)
	for _, s := range skills {
		for _, tag := range s.Frontmatter.Tags {
			tagSet[tag] = true
		}
	}
	for tag := range tagSet {
		result.Frontmatter.Tags = append(result.Frontmatter.Tags, tag)
	}

	// Merge sections
	allSections := make([]skill.Section, 0)
	for _, s := range skills {
		allSections = append(allSections, s.Sections...)
	}

	// Deduplicate if requested
	if opts != nil && opts.Deduplicate {
		allSections = m.dedup.DeduplicateSections(allSections)
	}

	// Group sections by title
	sectionGroups := make(map[string][]skill.Section)
	sectionOrder := make([]string, 0)

	for _, sec := range allSections {
		key := strings.ToLower(sec.Title)
		if _, exists := sectionGroups[key]; !exists {
			sectionOrder = append(sectionOrder, key)
		}
		sectionGroups[key] = append(sectionGroups[key], sec)
	}

	// Merge grouped sections
	for _, key := range sectionOrder {
		sections := sectionGroups[key]
		merged := m.mergeSections(sections)
		result.Sections = append(result.Sections, merged)
	}

	return result, nil
}

// mergeSections merges sections with the same title.
func (m *Merger) mergeSections(sections []skill.Section) skill.Section {
	if len(sections) == 1 {
		return sections[0]
	}

	// Use the first section as base
	result := skill.Section{
		Title: sections[0].Title,
		Level: sections[0].Level,
	}

	// Combine content
	var contents []string
	seen := make(map[string]bool)

	for _, sec := range sections {
		content := strings.TrimSpace(sec.Content)
		if content == "" {
			continue
		}

		// Simple dedup by exact match
		if !seen[content] {
			contents = append(contents, content)
			seen[content] = true
		}
	}

	result.Content = strings.Join(contents, "\n\n")
	return result
}
