package merger

import (
	"strings"

	"github.com/sanixdarker/skillforge/pkg/skill"
)

// ConflictStrategy defines how to resolve conflicts.
type ConflictStrategy int

const (
	// KeepFirst keeps the first value when conflicts occur.
	KeepFirst ConflictStrategy = iota
	// KeepLast keeps the last value when conflicts occur.
	KeepLast
	// KeepLonger keeps the longer content.
	KeepLonger
	// Combine combines all values.
	Combine
)

// ConflictResolver handles merge conflicts.
type ConflictResolver struct {
	strategy ConflictStrategy
}

// NewConflictResolver creates a new ConflictResolver.
func NewConflictResolver(strategy ConflictStrategy) *ConflictResolver {
	return &ConflictResolver{strategy: strategy}
}

// ResolveString resolves conflicts between string values.
func (r *ConflictResolver) ResolveString(values []string) string {
	if len(values) == 0 {
		return ""
	}

	// Filter empty values
	nonEmpty := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			nonEmpty = append(nonEmpty, v)
		}
	}

	if len(nonEmpty) == 0 {
		return ""
	}

	if len(nonEmpty) == 1 {
		return nonEmpty[0]
	}

	switch r.strategy {
	case KeepFirst:
		return nonEmpty[0]
	case KeepLast:
		return nonEmpty[len(nonEmpty)-1]
	case KeepLonger:
		longest := nonEmpty[0]
		for _, v := range nonEmpty[1:] {
			if len(v) > len(longest) {
				longest = v
			}
		}
		return longest
	case Combine:
		return strings.Join(nonEmpty, "\n\n")
	default:
		return nonEmpty[0]
	}
}

// ResolveSections resolves conflicts between sections.
func (r *ConflictResolver) ResolveSections(sections []skill.Section) skill.Section {
	if len(sections) == 0 {
		return skill.Section{}
	}

	if len(sections) == 1 {
		return sections[0]
	}

	// Use first section's metadata
	result := skill.Section{
		Title: sections[0].Title,
		Level: sections[0].Level,
	}

	// Collect contents
	contents := make([]string, 0, len(sections))
	for _, sec := range sections {
		if sec.Content != "" {
			contents = append(contents, sec.Content)
		}
	}

	result.Content = r.ResolveString(contents)
	return result
}

// Conflict represents a merge conflict.
type Conflict struct {
	Field    string
	Values   []string
	Resolved string
}

// DetectConflicts detects potential conflicts between skills.
func DetectConflicts(skills []*skill.Skill) []Conflict {
	var conflicts []Conflict

	// Check name conflicts
	names := make(map[string]int)
	for _, s := range skills {
		if s.Frontmatter.Name != "" {
			names[s.Frontmatter.Name]++
		}
	}
	if len(names) > 1 {
		values := make([]string, 0, len(names))
		for name := range names {
			values = append(values, name)
		}
		conflicts = append(conflicts, Conflict{
			Field:  "name",
			Values: values,
		})
	}

	// Check version conflicts
	versions := make(map[string]int)
	for _, s := range skills {
		if s.Frontmatter.Version != "" {
			versions[s.Frontmatter.Version]++
		}
	}
	if len(versions) > 1 {
		values := make([]string, 0, len(versions))
		for ver := range versions {
			values = append(values, ver)
		}
		conflicts = append(conflicts, Conflict{
			Field:  "version",
			Values: values,
		})
	}

	// Check section title conflicts (same title, different content)
	sectionsByTitle := make(map[string][]string)
	for _, s := range skills {
		for _, sec := range s.Sections {
			key := strings.ToLower(sec.Title)
			sectionsByTitle[key] = append(sectionsByTitle[key], sec.Content)
		}
	}

	for title, contents := range sectionsByTitle {
		if len(contents) <= 1 {
			continue
		}

		// Check if contents differ
		unique := make(map[string]bool)
		for _, c := range contents {
			unique[c] = true
		}
		if len(unique) > 1 {
			conflicts = append(conflicts, Conflict{
				Field:  "section:" + title,
				Values: contents,
			})
		}
	}

	return conflicts
}
