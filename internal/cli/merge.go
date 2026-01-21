package cli

import (
	"fmt"
	"os"

	"github.com/sanixdarker/skill-md/internal/merger"
	"github.com/sanixdarker/skill-md/pkg/skill"
	"github.com/spf13/cobra"
)

var (
	mergeOutput string
	mergeName   string
	mergeDedupe bool
)

var mergeCmd = &cobra.Command{
	Use:   "merge [files...]",
	Short: "Merge multiple SKILL.md files into one",
	Long: `Merge multiple SKILL.md files into a single combined skill.

The merge process:
  1. Parses all input SKILL.md files
  2. Combines sections intelligently
  3. Optionally deduplicates similar content
  4. Resolves any conflicts

Examples:
  skillforge merge api1.md api2.md -o combined.md
  skillforge merge *.md -n "Combined API Skills" --dedupe`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var skills []*skill.Skill

		// Parse all input files
		for _, path := range args {
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", path, err)
			}

			s, err := skill.Parse(string(content))
			if err != nil {
				return fmt.Errorf("failed to parse %s: %w", path, err)
			}

			skills = append(skills, s)
		}

		// Merge skills
		m := merger.New()
		result, err := m.Merge(skills, &merger.Options{
			Name:        mergeName,
			Deduplicate: mergeDedupe,
		})
		if err != nil {
			return fmt.Errorf("merge failed: %w", err)
		}

		// Render output
		output := skill.Render(result)

		// Write output
		if mergeOutput != "" {
			if err := os.WriteFile(mergeOutput, []byte(output), 0644); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}
			fmt.Printf("Merged SKILL.md written to %s\n", mergeOutput)
		} else {
			fmt.Println(output)
		}

		return nil
	},
}

func init() {
	mergeCmd.Flags().StringVarP(&mergeOutput, "output", "o", "", "Output file path")
	mergeCmd.Flags().StringVarP(&mergeName, "name", "n", "", "Name for the merged skill")
	mergeCmd.Flags().BoolVar(&mergeDedupe, "dedupe", false, "Deduplicate similar content")

	rootCmd.AddCommand(mergeCmd)
}
