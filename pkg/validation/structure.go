package validation

import (
	"strings"

	"github.com/sanixdarker/skillf/pkg/skill"
)

// Important section titles that are recommended in a skill.
var recommendedSections = map[string]bool{
	"quick start":     true,
	"quickstart":      true,
	"overview":        true,
	"getting started": true,
	"introduction":    true,
}

func (v *Validator) validateStructure(s *skill.Skill, result *ValidationResult) {
	// Check for content
	if len(s.Sections) == 0 && s.Content == "" {
		v.addIssue(result, ValidationIssue{
			Code:       CodeNoContent,
			Message:    "Skill has no content or sections",
			Severity:   SeverityCritical,
			Suggestion: "Add markdown content with proper section headers",
		})
		return
	}

	// Count words and code blocks for statistics
	totalWords := 0
	totalCodeBlocks := 0

	for _, section := range s.Sections {
		totalWords += countWords(section.Content)
		totalCodeBlocks += countCodeBlocks(section.Content)
	}
	if s.Content != "" {
		totalWords += countWords(s.Content)
		totalCodeBlocks += countCodeBlocks(s.Content)
	}

	result.Statistics.TotalWords = totalWords
	result.Statistics.TotalCodeBlocks = totalCodeBlocks

	// Check section hierarchy
	v.validateSectionHierarchy(s, result)

	// Check for recommended sections
	v.validateRecommendedSections(s, result)

	// Check for empty sections
	v.validateEmptySections(s, result)

	// Check for duplicate sections
	v.validateDuplicateSections(s, result)

	// Check for long sections
	v.validateSectionLength(s, result)

	// Check for code examples
	if totalCodeBlocks == 0 && v.options.StrictMode {
		v.addIssue(result, ValidationIssue{
			Code:       CodeNoCodeExamples,
			Message:    "No code examples found in skill",
			Severity:   SeverityWarning,
			Suggestion: "Add code examples using fenced code blocks with language specifiers",
		})
	}

	// Check for broken code blocks
	v.validateCodeBlocks(s, result)
}

func (v *Validator) validateSectionHierarchy(s *skill.Skill, result *ValidationResult) {
	var prevLevel int
	for i, section := range s.Sections {
		if i == 0 {
			prevLevel = section.Level
			continue
		}
		// Sections should not skip levels (e.g., h1 -> h3)
		if section.Level > prevLevel+1 {
			v.addIssue(result, ValidationIssue{
				Code:       CodeSkippedHeading,
				Message:    "Section '" + section.Title + "' skips heading level (h" + itoa(section.Level) + " after h" + itoa(prevLevel) + ")",
				Severity:   SeverityWarning,
				Field:      "sections[" + itoa(i) + "]",
				Suggestion: "Use proper heading hierarchy: h1 -> h2 -> h3",
			})
		}
		prevLevel = section.Level
	}
}

func (v *Validator) validateRecommendedSections(s *skill.Skill, result *ValidationResult) {
	hasQuickStart := false
	hasOverview := false

	for _, section := range s.Sections {
		titleLower := strings.ToLower(section.Title)
		if recommendedSections[titleLower] {
			if strings.Contains(titleLower, "quick") || strings.Contains(titleLower, "start") {
				hasQuickStart = true
			}
			if strings.Contains(titleLower, "overview") || strings.Contains(titleLower, "introduction") {
				hasOverview = true
			}
		}
	}

	if !hasQuickStart && v.options.StrictMode {
		v.addIssue(result, ValidationIssue{
			Code:       CodeMissingQuickStart,
			Message:    "No 'Quick Start' or 'Getting Started' section found",
			Severity:   SeverityInfo,
			Suggestion: "Add a Quick Start section to help users get started quickly",
		})
	}

	if !hasOverview && v.options.StrictMode {
		v.addIssue(result, ValidationIssue{
			Code:       CodeMissingOverview,
			Message:    "No 'Overview' or 'Introduction' section found",
			Severity:   SeverityInfo,
			Suggestion: "Add an Overview section to explain the skill's purpose",
		})
	}
}

func (v *Validator) validateEmptySections(s *skill.Skill, result *ValidationResult) {
	for i, section := range s.Sections {
		content := strings.TrimSpace(section.Content)
		if content == "" {
			v.addIssue(result, ValidationIssue{
				Code:       CodeEmptySection,
				Message:    "Section '" + section.Title + "' has no content",
				Severity:   SeverityWarning,
				Field:      "sections[" + itoa(i) + "]",
				Suggestion: "Add content to the section or remove it",
			})
		}
	}
}

func (v *Validator) validateDuplicateSections(s *skill.Skill, result *ValidationResult) {
	seen := make(map[string]int)
	for i, section := range s.Sections {
		titleLower := strings.ToLower(section.Title)
		if prevIdx, exists := seen[titleLower]; exists {
			v.addIssue(result, ValidationIssue{
				Code:       CodeDuplicateSection,
				Message:    "Duplicate section title '" + section.Title + "' (first at index " + itoa(prevIdx) + ")",
				Severity:   SeverityWarning,
				Field:      "sections[" + itoa(i) + "]",
				Suggestion: "Rename or merge duplicate sections",
			})
		} else {
			seen[titleLower] = i
		}
	}
}

func (v *Validator) validateSectionLength(s *skill.Skill, result *ValidationResult) {
	maxLen := v.options.MaxSectionLength
	if maxLen <= 0 {
		maxLen = 2000
	}

	for i, section := range s.Sections {
		wordCount := countWords(section.Content)
		if wordCount > maxLen {
			v.addIssue(result, ValidationIssue{
				Code:       CodeLongSection,
				Message:    "Section '" + section.Title + "' is very long (" + itoa(wordCount) + " words)",
				Severity:   SeverityInfo,
				Field:      "sections[" + itoa(i) + "]",
				Suggestion: "Consider breaking into smaller subsections for readability",
			})
		}
	}
}

func (v *Validator) validateCodeBlocks(s *skill.Skill, result *ValidationResult) {
	// Check for unclosed code blocks in each section
	for i, section := range s.Sections {
		if hasUnclosedCodeBlock(section.Content) {
			v.addIssue(result, ValidationIssue{
				Code:       CodeBrokenCodeBlock,
				Message:    "Section '" + section.Title + "' has unclosed code block",
				Severity:   SeverityError,
				Field:      "sections[" + itoa(i) + "]",
				Suggestion: "Ensure all ``` code blocks are properly closed",
			})
		}
	}

	// Check main content too
	if s.Content != "" && hasUnclosedCodeBlock(s.Content) {
		v.addIssue(result, ValidationIssue{
			Code:       CodeBrokenCodeBlock,
			Message:    "Main content has unclosed code block",
			Severity:   SeverityError,
			Field:      "content",
			Suggestion: "Ensure all ``` code blocks are properly closed",
		})
	}
}

func hasUnclosedCodeBlock(content string) bool {
	count := 0
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			count++
		}
	}
	return count%2 != 0
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
