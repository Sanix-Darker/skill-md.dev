package cli

import (
	"github.com/spf13/cobra"
)

var (
	lintSeverity      string
	lintJSON          bool
	lintMCP           bool
	lintStrict        bool
	lintFailOnWarning bool
	lintFormat        string
)

var lintCmd = &cobra.Command{
	Use:     "lint [file]",
	Aliases: []string{"check"},
	Short:   "Lint a SKILL.md file for quality and consistency",
	Long: `Run validation-oriented checks on a SKILL.md file and show actionable issues.

The linter is opinionated by design: by default, non-warning errors fail
the command. Add --fail-on-warning when warnings should be treated as hard
failures.

Examples:
  skillmd lint SKILL.md
  skillmd lint SKILL.md --fail-on-warning
  skillmd lint SKILL.md --mcp --strict --format json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runValidateCommand(args[0], validationCommandOptions{
			severity:      lintSeverity,
			outputJSON:    lintJSON,
			mcpCompatible: lintMCP,
			strictMode:    lintStrict,
			failOnWarning: lintFailOnWarning,
			format:        lintFormat,
		})
	},
}

func init() {
	lintCmd.Flags().StringVarP(&lintSeverity, "severity", "s", "", "Minimum severity level (info, warning, error, critical)")
	lintCmd.Flags().BoolVar(&lintJSON, "json", false, "Output validation result as JSON")
	lintCmd.Flags().StringVar(&lintFormat, "format", "text", "Output format: text or json")
	lintCmd.Flags().BoolVar(&lintMCP, "mcp", false, "Enable MCP compatibility validation")
	lintCmd.Flags().BoolVar(&lintStrict, "strict", false, "Enable strict mode with additional quality checks")
	lintCmd.Flags().BoolVar(&lintFailOnWarning, "fail-on-warning", false, "Exit non-zero when warnings are present")

	rootCmd.AddCommand(lintCmd)
}
