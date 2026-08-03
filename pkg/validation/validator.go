package validation

import (
	"strings"

	"github.com/sanixdarker/skillf/pkg/skill"
)

// Validator provides SKILL.md validation functionality.
type Validator struct {
	options ValidationOptions
}

// NewValidator creates a new Validator with the given options.
func NewValidator(opts ValidationOptions) *Validator {
	return &Validator{options: opts}
}

// NewDefaultValidator creates a new Validator with default options.
func NewDefaultValidator() *Validator {
	return NewValidator(DefaultOptions())
}

// Validate performs full validation on a skill.
func (v *Validator) Validate(s *skill.Skill) ValidationResult {
	result := ValidationResult{
		Valid:  true,
		Issues: []ValidationIssue{},
		Statistics: ValidationStats{
			TotalSections: len(s.Sections),
		},
	}

	if s.Frontmatter.Name != "" {
		result.SkillName = s.Frontmatter.Name
	}
	if s.Frontmatter.Version != "" {
		result.Version = s.Frontmatter.Version
	}

	// Run all validators
	v.validateFrontmatter(s, &result)
	v.validateStructure(s, &result)
	if v.options.MCPCompat {
		v.validateMCP(s, &result)
	}
	if v.options.StrictMode {
		v.validateQuality(s, &result)
	}

	// Update statistics
	v.calculateStats(&result)

	// Determine overall validity
	result.Valid = result.Statistics.CriticalCount == 0 && result.Statistics.ErrorCount == 0

	return result
}

// ValidateContent validates raw SKILL.md content.
func (v *Validator) ValidateContent(content string) (ValidationResult, error) {
	s, err := skill.Parse(content)
	if err != nil {
		return ValidationResult{
			Valid: false,
			Issues: []ValidationIssue{
				{
					Code:       "PARSE_ERROR",
					Message:    err.Error(),
					Severity:   SeverityCritical,
					Suggestion: "Ensure the file has valid YAML frontmatter and markdown content",
				},
			},
		}, nil
	}
	return v.Validate(s), nil
}

func (v *Validator) calculateStats(result *ValidationResult) {
	for _, issue := range result.Issues {
		switch issue.Severity {
		case SeverityInfo:
			result.Statistics.InfoCount++
		case SeverityWarning:
			result.Statistics.WarningCount++
		case SeverityError:
			result.Statistics.ErrorCount++
		case SeverityCritical:
			result.Statistics.CriticalCount++
		}
	}
}

func (v *Validator) addIssue(result *ValidationResult, issue ValidationIssue) {
	if issue.Severity >= v.options.MinSeverity {
		result.Issues = append(result.Issues, issue)
	}
}

// countWords counts words in a string.
func countWords(s string) int {
	return len(strings.Fields(s))
}

// countCodeBlocks counts fenced code blocks in content.
func countCodeBlocks(content string) int {
	count := 0
	lines := strings.Split(content, "\n")
	inCodeBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inCodeBlock {
				count++
			}
			inCodeBlock = !inCodeBlock
		}
	}
	return count
}
