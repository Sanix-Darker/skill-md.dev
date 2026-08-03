package validation

import (
	"net/url"
	"regexp"
	"time"

	"github.com/sanixdarker/skillf/pkg/skill"
)

// semverRegex matches semantic version strings.
var semverRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

func (v *Validator) validateFrontmatter(s *skill.Skill, result *ValidationResult) {
	fm := s.Frontmatter

	// Required: name
	if fm.Name == "" {
		v.addIssue(result, ValidationIssue{
			Code:       CodeMissingName,
			Message:    "Skill name is required in frontmatter",
			Severity:   SeverityCritical,
			Field:      "frontmatter.name",
			Suggestion: "Add 'name: \"Your Skill Name\"' to frontmatter",
		})
	}

	// Required: version
	if fm.Version == "" {
		v.addIssue(result, ValidationIssue{
			Code:       CodeMissingVersion,
			Message:    "Skill version is required in frontmatter",
			Severity:   SeverityCritical,
			Field:      "frontmatter.version",
			Suggestion: "Add 'version: \"1.0.0\"' to frontmatter using semantic versioning",
		})
	} else if !isValidSemver(fm.Version) {
		v.addIssue(result, ValidationIssue{
			Code:       CodeInvalidVersion,
			Message:    "Version must follow semantic versioning (e.g., 1.0.0)",
			Severity:   SeverityError,
			Field:      "frontmatter.version",
			Suggestion: "Use format: MAJOR.MINOR.PATCH (e.g., 1.0.0, 2.1.3)",
		})
	}

	// Recommended: description
	if fm.Description == "" {
		v.addIssue(result, ValidationIssue{
			Code:       CodeMissingDescription,
			Message:    "Skill description is recommended for discoverability",
			Severity:   SeverityWarning,
			Field:      "frontmatter.description",
			Suggestion: "Add a brief description of what this skill does",
		})
	}

	// Optional: tags validation
	if len(fm.Tags) > 0 {
		for i, tag := range fm.Tags {
			if tag == "" {
				v.addIssue(result, ValidationIssue{
					Code:       CodeEmptyTags,
					Message:    "Empty tag found in tags array",
					Severity:   SeverityWarning,
					Field:      "frontmatter.tags",
					Suggestion: "Remove empty tags from the tags array",
					Line:       i,
				})
			}
		}
	}

	// Optional: date validation
	if fm.CreatedAt != "" && !isValidDate(fm.CreatedAt) {
		v.addIssue(result, ValidationIssue{
			Code:       CodeInvalidDate,
			Message:    "created_at must be a valid RFC3339 date",
			Severity:   SeverityWarning,
			Field:      "frontmatter.created_at",
			Suggestion: "Use RFC3339 format: 2006-01-02T15:04:05Z07:00",
		})
	}

	if fm.UpdatedAt != "" && !isValidDate(fm.UpdatedAt) {
		v.addIssue(result, ValidationIssue{
			Code:       CodeInvalidDate,
			Message:    "updated_at must be a valid RFC3339 date",
			Severity:   SeverityWarning,
			Field:      "frontmatter.updated_at",
			Suggestion: "Use RFC3339 format: 2006-01-02T15:04:05Z07:00",
		})
	}

	// Optional: base_url validation
	if fm.BaseURL != "" && !isValidURL(fm.BaseURL) {
		v.addIssue(result, ValidationIssue{
			Code:       CodeInvalidURL,
			Message:    "base_url must be a valid URL",
			Severity:   SeverityError,
			Field:      "frontmatter.base_url",
			Suggestion: "Use a valid URL format: https://api.example.com",
		})
	}
}

func isValidSemver(version string) bool {
	return semverRegex.MatchString(version)
}

func isValidDate(date string) bool {
	// Try RFC3339
	if _, err := time.Parse(time.RFC3339, date); err == nil {
		return true
	}
	// Try date only
	if _, err := time.Parse("2006-01-02", date); err == nil {
		return true
	}
	return false
}

func isValidURL(urlStr string) bool {
	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	return u.Scheme != "" && u.Host != ""
}
