package cli

import (
	"fmt"
	"os"

	"github.com/sanixdarker/skillforge/pkg/skill"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate [file]",
	Short: "Validate a SKILL.md file",
	Long: `Validate a SKILL.md file for correctness and completeness.

Checks performed:
  - Valid YAML frontmatter
  - Required fields (name, version)
  - Valid markdown structure
  - Section hierarchy

Examples:
  skillforge validate skill.md`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath := args[0]

		// Read input file
		content, err := os.ReadFile(inputPath)
		if err != nil {
			return fmt.Errorf("failed to read input file: %w", err)
		}

		// Parse the skill
		s, err := skill.Parse(string(content))
		if err != nil {
			return fmt.Errorf("parse error: %w", err)
		}

		// Validate
		errors := validate(s)
		if len(errors) > 0 {
			fmt.Println("Validation errors:")
			for _, e := range errors {
				fmt.Printf("  - %s\n", e)
			}
			return fmt.Errorf("validation failed with %d errors", len(errors))
		}

		fmt.Printf("Valid SKILL.md: %s\n", s.Frontmatter.Name)
		fmt.Printf("  Version: %s\n", s.Frontmatter.Version)
		fmt.Printf("  Sections: %d\n", len(s.Sections))
		if len(s.Frontmatter.Tags) > 0 {
			fmt.Printf("  Tags: %v\n", s.Frontmatter.Tags)
		}

		return nil
	},
}

func validate(s *skill.Skill) []string {
	var errors []string

	// Check required fields
	if s.Frontmatter.Name == "" {
		errors = append(errors, "missing required field: name")
	}
	if s.Frontmatter.Version == "" {
		errors = append(errors, "missing required field: version")
	}

	// Check for content
	if len(s.Sections) == 0 && s.Content == "" {
		errors = append(errors, "skill has no content or sections")
	}

	// Check section hierarchy
	var prevLevel int
	for i, section := range s.Sections {
		if i == 0 {
			prevLevel = section.Level
			continue
		}
		// Sections should not skip levels (e.g., h1 -> h3)
		if section.Level > prevLevel+1 {
			errors = append(errors, fmt.Sprintf("section '%s' skips heading level (h%d after h%d)", section.Title, section.Level, prevLevel))
		}
		prevLevel = section.Level
	}

	return errors
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
