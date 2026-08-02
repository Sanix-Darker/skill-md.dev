// Package validation provides SKILL.md validation with severity levels and spec compliance.
package validation

import (
	"encoding/json"
	"fmt"
)

// Severity represents the importance level of a validation issue.
type Severity int

const (
	// SeverityInfo indicates informational notes that don't affect validity.
	SeverityInfo Severity = iota
	// SeverityWarning indicates issues that should be fixed but don't break functionality.
	SeverityWarning
	// SeverityError indicates issues that may cause problems for AI agents.
	SeverityError
	// SeverityCritical indicates issues that make the skill unusable.
	SeverityCritical
)

// String returns the string representation of a Severity.
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// MarshalJSON implements json.Marshaler.
func (s Severity) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// ParseSeverity converts a string to a Severity level.
func ParseSeverity(s string) (Severity, error) {
	switch s {
	case "info":
		return SeverityInfo, nil
	case "warning":
		return SeverityWarning, nil
	case "error":
		return SeverityError, nil
	case "critical":
		return SeverityCritical, nil
	default:
		return SeverityInfo, fmt.Errorf("unknown severity: %s", s)
	}
}

// ValidationCode represents a specific validation issue type.
type ValidationCode string

// Validation codes for different issue types.
const (
	// Frontmatter validation codes
	CodeMissingName        ValidationCode = "MISSING_NAME"
	CodeMissingVersion     ValidationCode = "MISSING_VERSION"
	CodeInvalidVersion     ValidationCode = "INVALID_VERSION"
	CodeMissingDescription ValidationCode = "MISSING_DESCRIPTION"
	CodeEmptyTags          ValidationCode = "EMPTY_TAGS"
	CodeInvalidDate        ValidationCode = "INVALID_DATE"

	// Structure validation codes
	CodeNoContent         ValidationCode = "NO_CONTENT"
	CodeSkippedHeading    ValidationCode = "SKIPPED_HEADING"
	CodeMissingQuickStart ValidationCode = "MISSING_QUICK_START"
	CodeMissingOverview   ValidationCode = "MISSING_OVERVIEW"
	CodeEmptySection      ValidationCode = "EMPTY_SECTION"
	CodeDuplicateSection  ValidationCode = "DUPLICATE_SECTION"
	CodeLongSection       ValidationCode = "LONG_SECTION"
	CodeNoCodeExamples    ValidationCode = "NO_CODE_EXAMPLES"
	CodeBrokenCodeBlock   ValidationCode = "BROKEN_CODE_BLOCK"

	// MCP compatibility codes
	CodeMCPNoTools           ValidationCode = "MCP_NO_TOOLS"
	CodeMCPInvalidToolSchema ValidationCode = "MCP_INVALID_TOOL_SCHEMA"
	CodeMCPMissingToolName   ValidationCode = "MCP_MISSING_TOOL_NAME"
	CodeMCPMissingToolDesc   ValidationCode = "MCP_MISSING_TOOL_DESC"
	CodeMCPNoRateLimits      ValidationCode = "MCP_NO_RATE_LIMITS"
	CodeMCPNoRetryStrategy   ValidationCode = "MCP_NO_RETRY_STRATEGY"

	// Content quality codes
	CodeInconsistentAuth ValidationCode = "INCONSISTENT_AUTH"
	CodeMissingBaseURL   ValidationCode = "MISSING_BASE_URL"
	CodeInvalidURL       ValidationCode = "INVALID_URL"
)

// ValidationIssue represents a single validation problem.
type ValidationIssue struct {
	Code       ValidationCode `json:"code"`
	Message    string         `json:"message"`
	Severity   Severity       `json:"severity"`
	Field      string         `json:"field,omitempty"`
	Line       int            `json:"line,omitempty"`
	Suggestion string         `json:"suggestion,omitempty"`
}

// String returns a human-readable representation of the issue.
func (vi ValidationIssue) String() string {
	prefix := fmt.Sprintf("[%s] %s", vi.Severity, vi.Code)
	if vi.Field != "" {
		prefix += fmt.Sprintf(" in %s", vi.Field)
	}
	if vi.Line > 0 {
		prefix += fmt.Sprintf(" (line %d)", vi.Line)
	}
	return fmt.Sprintf("%s: %s", prefix, vi.Message)
}

// ValidationStats contains statistics about the validation.
type ValidationStats struct {
	TotalSections   int `json:"total_sections"`
	TotalCodeBlocks int `json:"total_code_blocks"`
	TotalWords      int `json:"total_words"`
	InfoCount       int `json:"info_count"`
	WarningCount    int `json:"warning_count"`
	ErrorCount      int `json:"error_count"`
	CriticalCount   int `json:"critical_count"`
}

// ValidationResult contains the complete validation output.
type ValidationResult struct {
	Valid      bool              `json:"valid"`
	SkillName  string            `json:"skill_name,omitempty"`
	Version    string            `json:"version,omitempty"`
	Issues     []ValidationIssue `json:"issues"`
	Statistics ValidationStats   `json:"statistics"`
}

// IssuesBySeverity returns issues filtered by minimum severity level.
func (vr ValidationResult) IssuesBySeverity(minSeverity Severity) []ValidationIssue {
	var filtered []ValidationIssue
	for _, issue := range vr.Issues {
		if issue.Severity >= minSeverity {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

// HasErrors returns true if there are any error or critical issues.
func (vr ValidationResult) HasErrors() bool {
	return vr.Statistics.ErrorCount > 0 || vr.Statistics.CriticalCount > 0
}

// HasWarnings returns true if there are any warning or higher issues.
func (vr ValidationResult) HasWarnings() bool {
	return vr.Statistics.WarningCount > 0 || vr.HasErrors()
}

// ValidationOptions configures validation behavior.
type ValidationOptions struct {
	// MinSeverity filters issues below this level
	MinSeverity Severity
	// MCPCompat enables MCP compatibility checks
	MCPCompat bool
	// StrictMode enables additional quality checks
	StrictMode bool
	// MaxSectionLength is the maximum recommended section length in words
	MaxSectionLength int
}

// DefaultOptions returns sensible default validation options.
func DefaultOptions() ValidationOptions {
	return ValidationOptions{
		MinSeverity:      SeverityInfo,
		MCPCompat:        true,
		StrictMode:       false,
		MaxSectionLength: 2000,
	}
}
