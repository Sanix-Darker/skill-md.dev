// Package cli provides the command-line interface.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version is set at build time.
	Version = "dev"
	// Commit is set at build time.
	Commit = "none"
)

var rootCmd = &cobra.Command{
	Use:   "skillmd",
	Short: "Convert technical specs to SKILL.md format for AI agents",
	Long: `skill-md.dev is a tool for converting technical specifications
(OpenAPI, GraphQL, Postman, etc.) into SKILL.md format that AI agents
can understand and use.

Features:
  - Convert various spec formats to SKILL.md
  - Merge multiple skills into one
  - Browse and search skill registry
  - Web UI and CLI interfaces`,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("skillmd version %s (commit: %s)\n", Version, Commit)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
