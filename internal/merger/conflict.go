package merger

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sanixdarker/skillf/pkg/skill"
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

// String returns a display name for the strategy.
func (s ConflictStrategy) String() string {
	switch s {
	case KeepFirst:
		return "keep_first"
	case KeepLast:
		return "keep_last"
	case KeepLonger:
		return "keep_longer"
	case Combine:
		return "combine"
	default:
		return fmt.Sprintf("unknown:%d", s)
	}
}

// ParseConflictStrategy parses a user-provided strategy value into a supported
// conflict strategy.
func ParseConflictStrategy(value string) (ConflictStrategy, error) {
	switch strings.TrimSpace(strings.ToLower(strings.ReplaceAll(value, "-", "_"))) {
	case "", "keep_first", "first":
		return KeepFirst, nil
	case "keep_last", "last":
		return KeepLast, nil
	case "keep_longer", "longer":
		return KeepLonger, nil
	case "combine":
		return Combine, nil
	default:
		return KeepFirst, fmt.Errorf("unknown conflict strategy: %s", value)
	}
}

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

// ConflictValue maps a conflicting value back to its source.
type ConflictValue struct {
	Source string
	Value  string
}

// Conflict represents a merge conflict.
type Conflict struct {
	Field    string
	Values   []ConflictValue
	Resolved string
}

// StrategyInfo describes a supported conflict strategy.
type StrategyInfo struct {
	Value       string
	Label       string
	Description string
}

// SupportedStrategies returns metadata for every supported conflict strategy.
func SupportedStrategies() []StrategyInfo {
	return []StrategyInfo{
		{
			Value:       KeepFirst.String(),
			Label:       "keep_first (safe default)",
			Description: "Keeps the first non-empty value encountered.",
		},
		{
			Value:       KeepLast.String(),
			Label:       "keep_last",
			Description: "Keeps the last non-empty value encountered.",
		},
		{
			Value:       KeepLonger.String(),
			Label:       "keep_longer",
			Description: "Keeps the longest non-empty value.",
		},
		{
			Value:       Combine.String(),
			Label:       "combine",
			Description: "Concatenates values with blank lines.",
		},
	}
}

// DetectConflicts detects potential conflicts between skills and resolves the
// conflict strategy for each field.
func DetectConflicts(skills []*skill.Skill, strategy ConflictStrategy) []Conflict {
	var conflicts []Conflict

	appendIfConflict := func(field string, values []ConflictValue) {
		if len(values) < 2 {
			return
		}
		uniq := uniqueConflictValues(values)
		if len(uniq) <= 1 {
			return
		}
		resolved := resolveConflictValues(uniq, strategy)
		if resolved == "" {
			return
		}

		conflicts = append(conflicts, Conflict{
			Field:    field,
			Values:   uniq,
			Resolved: resolved,
		})
	}

	// Name conflicts.
	nameValues := make([]ConflictValue, 0, len(skills))
	for i, s := range skills {
		if s == nil {
			continue
		}
		name := strings.TrimSpace(s.Frontmatter.Name)
		if name == "" {
			continue
		}
		nameValues = append(nameValues, ConflictValue{
			Source: skillSourceLabel(s, i),
			Value:  name,
		})
	}
	appendIfConflict("name", nameValues)

	// Version conflicts.
	versionValues := make([]ConflictValue, 0, len(skills))
	for i, s := range skills {
		if s == nil {
			continue
		}
		version := strings.TrimSpace(s.Frontmatter.Version)
		if version == "" {
			continue
		}
		versionValues = append(versionValues, ConflictValue{
			Source: skillSourceLabel(s, i),
			Value:  version,
		})
	}
	appendIfConflict("version", versionValues)

	// Section title conflicts (same title, different content).
	sectionsByTitle := make(map[string][]ConflictValue)
	sectionOrder := make([]string, 0)
	for i, s := range skills {
		if s == nil {
			continue
		}
		source := skillSourceLabel(s, i)
		for _, sec := range s.Sections {
			title := strings.TrimSpace(sec.Title)
			if title == "" {
				continue
			}
			content := strings.TrimSpace(sec.Content)
			if content == "" {
				continue
			}

			key := strings.ToLower(title)
			if _, ok := sectionsByTitle[key]; !ok {
				sectionOrder = append(sectionOrder, key)
			}
			sectionsByTitle[key] = append(sectionsByTitle[key], ConflictValue{
				Source: source,
				Value:  content,
			})
		}
	}

	for _, key := range sectionOrder {
		appendIfConflict("section:"+key, sectionsByTitle[key])
	}

	sort.Slice(conflicts, func(i, j int) bool {
		return conflicts[i].Field < conflicts[j].Field
	})

	return conflicts
}

// DetectConflictsLegacy keeps backward compatibility for existing internal callers.
//
// Deprecated: use DetectConflicts(skills, strategy) instead.
func DetectConflictsLegacy(skills []*skill.Skill) []Conflict {
	return DetectConflicts(skills, KeepFirst)
}

func resolveConflictValues(values []ConflictValue, strategy ConflictStrategy) string {
	vals := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value.Value); trimmed != "" {
			vals = append(vals, trimmed)
		}
	}
	return NewConflictResolver(strategy).ResolveString(vals)
}

func uniqueConflictValues(values []ConflictValue) []ConflictValue {
	seen := make(map[string]bool, len(values))
	unique := make([]ConflictValue, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value.Value)
		if trimmed == "" {
			continue
		}
		if seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		value.Value = trimmed
		unique = append(unique, value)
	}
	return unique
}

func skillSourceLabel(s *skill.Skill, index int) string {
	source := strings.TrimSpace(s.Frontmatter.Source)
	if source == "" {
		source = "uploaded"
	}
	return fmt.Sprintf("%s #%d", source, index+1)
}
