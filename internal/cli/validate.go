package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sanixdarker/skillf/pkg/skill"
	"github.com/sanixdarker/skillf/pkg/validation"
	"github.com/spf13/cobra"
)

var (
	validateSeverity string
	validateJSON     bool
	validateMCP      bool
	validateStrict   bool
)

type validationCommandOptions struct {
	severity      string
	outputJSON    bool
	mcpCompatible bool
	strictMode    bool
	failOnWarning bool
	format        string
}

func runValidateCommand(inputPath string, opts validationCommandOptions) error {
	format := opts.format
	if opts.outputJSON {
		format = "json"
	}
	if format == "" {
		format = "text"
	}
	if format != "text" && format != "json" {
		return fmt.Errorf("invalid validation format: %s", format)
	}

	content, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	s, err := skill.Parse(string(content))
	if err != nil {
		if format == "json" {
			result := validation.ValidationResult{
				Valid: false,
				Issues: []validation.ValidationIssue{
					{
						Code:       "PARSE_ERROR",
						Message:    err.Error(),
						Severity:   validation.SeverityCritical,
						Suggestion: "Ensure the file has valid YAML frontmatter and markdown content",
					},
				},
			}
			return outputValidationResult(result)
		}
		return fmt.Errorf("parse error: %w", err)
	}

	validationOpts := validation.DefaultOptions()
	validationOpts.MCPCompat = opts.mcpCompatible
	validationOpts.StrictMode = opts.strictMode

	if opts.severity != "" {
		sev, err := validation.ParseSeverity(opts.severity)
		if err != nil {
			return fmt.Errorf("invalid severity: %w", err)
		}
		validationOpts.MinSeverity = sev
	}

	validator := validation.NewValidator(validationOpts)
	result := validator.Validate(s)

	if format == "json" {
		if err := outputValidationResult(result); err != nil {
			return err
		}
		if !result.Valid || (opts.failOnWarning && result.HasWarnings()) {
			return fmt.Errorf("validation failed")
		}
		return nil
	}

	if err := outputValidationHuman(result, inputPath); err != nil {
		return err
	}

	if !result.Valid || (opts.failOnWarning && result.HasWarnings()) {
		return fmt.Errorf("validation failed")
	}

	return nil
}

var validateCmd = &cobra.Command{
	Use:   "validate [file]",
	Short: "Validate a SKILL.md file",
	Long: `Validate a SKILL.md file for correctness and completeness.

Checks performed:
  - Valid YAML frontmatter
  - Required fields (name, version with semver format)
  - Valid markdown structure
  - Section hierarchy
  - MCP compatibility (optional)

Severity levels:
  - info: Informational notes
  - warning: Should be fixed but not critical
  - error: May cause problems for AI agents
  - critical: Makes the skill unusable

Examples:
  skillf validate skill.md
  skillf validate skill.md --severity warning
  skillf validate skill.md --json
  skillf validate skill.md --mcp --strict`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runValidateCommand(args[0], validationCommandOptions{
			severity:      validateSeverity,
			outputJSON:    validateJSON,
			mcpCompatible: validateMCP,
			strictMode:    validateStrict,
			failOnWarning: false,
			format:        "",
		})
	},
}

func outputValidationResult(result validation.ValidationResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func outputValidationHuman(result validation.ValidationResult, path string) error {
	if result.Valid {
		fmt.Printf("✓ Valid SKILL.md: %s\n", result.SkillName)
		fmt.Printf("  Version: %s\n", result.Version)
		fmt.Printf("  Sections: %d\n", result.Statistics.TotalSections)
		fmt.Printf("  Words: %d\n", result.Statistics.TotalWords)
		fmt.Printf("  Code blocks: %d\n", result.Statistics.TotalCodeBlocks)

		if len(result.Issues) > 0 {
			fmt.Printf("\nSuggestions (%d):\n", len(result.Issues))
			for _, issue := range result.Issues {
				fmt.Printf("  [%s] %s\n", issue.Severity, issue.Message)
				if issue.Suggestion != "" {
					fmt.Printf("          → %s\n", issue.Suggestion)
				}
			}
		}
		return nil
	}

	fmt.Printf("✗ Invalid SKILL.md: %s\n\n", path)

	// Group issues by severity
	critical := result.IssuesBySeverity(validation.SeverityCritical)
	errors := filterBySeverity(result.Issues, validation.SeverityError)
	warnings := filterBySeverity(result.Issues, validation.SeverityWarning)
	info := filterBySeverity(result.Issues, validation.SeverityInfo)

	if len(critical) > 0 {
		fmt.Println("Critical:")
		for _, issue := range critical {
			printIssue(issue)
		}
	}

	if len(errors) > 0 {
		fmt.Println("\nErrors:")
		for _, issue := range errors {
			printIssue(issue)
		}
	}

	if len(warnings) > 0 {
		fmt.Println("\nWarnings:")
		for _, issue := range warnings {
			printIssue(issue)
		}
	}

	if len(info) > 0 {
		fmt.Println("\nInfo:")
		for _, issue := range info {
			printIssue(issue)
		}
	}

	fmt.Printf("\nSummary: %d critical, %d errors, %d warnings, %d info\n",
		result.Statistics.CriticalCount,
		result.Statistics.ErrorCount,
		result.Statistics.WarningCount,
		result.Statistics.InfoCount)

	return fmt.Errorf("validation failed")
}

func filterBySeverity(issues []validation.ValidationIssue, severity validation.Severity) []validation.ValidationIssue {
	var filtered []validation.ValidationIssue
	for _, issue := range issues {
		if issue.Severity == severity {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

func printIssue(issue validation.ValidationIssue) {
	field := ""
	if issue.Field != "" {
		field = fmt.Sprintf(" [%s]", issue.Field)
	}
	fmt.Printf("  • %s%s: %s\n", issue.Code, field, issue.Message)
	if issue.Suggestion != "" {
		fmt.Printf("    → %s\n", issue.Suggestion)
	}
}

func init() {
	validateCmd.Flags().StringVarP(&validateSeverity, "severity", "s", "", "Minimum severity level (info, warning, error, critical)")
	validateCmd.Flags().BoolVar(&validateJSON, "json", false, "Output validation result as JSON")
	validateCmd.Flags().BoolVar(&validateMCP, "mcp", false, "Enable MCP compatibility validation")
	validateCmd.Flags().BoolVar(&validateStrict, "strict", false, "Enable strict mode with additional quality checks")
	rootCmd.AddCommand(validateCmd)
}
