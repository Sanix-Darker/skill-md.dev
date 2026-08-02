package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/sanixdarker/skillf/pkg/semver"
	"github.com/sanixdarker/skillf/pkg/skill"
	"github.com/spf13/cobra"
)

var skillVersionCmd = &cobra.Command{
	Use:   "skill-version",
	Short: "Manage SKILL.md versions",
	Long: `Commands for managing SKILL.md versions.

Examples:
  skillf skill-version get skill.md
  skillf skill-version bump patch skill.md
  skillf skill-version check skill.md`,
}

var versionGetCmd = &cobra.Command{
	Use:   "get [file]",
	Short: "Get the current version of a SKILL.md file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		content, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		s, err := skill.Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse skill: %w", err)
		}

		if s.Frontmatter.Version == "" {
			return fmt.Errorf("no version found in frontmatter")
		}

		fmt.Println(s.Frontmatter.Version)
		return nil
	},
}

var versionBumpCmd = &cobra.Command{
	Use:   "bump [major|minor|patch] [file]",
	Short: "Bump the version of a SKILL.md file",
	Long: `Bump the version of a SKILL.md file by major, minor, or patch.

Examples:
  skillf skill-version bump patch skill.md   # 1.0.0 -> 1.0.1
  skillf skill-version bump minor skill.md   # 1.0.0 -> 1.1.0
  skillf skill-version bump major skill.md   # 1.0.0 -> 2.0.0`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		bumpTypeStr := args[0]
		filePath := args[1]

		var bumpType semver.BumpType
		switch bumpTypeStr {
		case "major":
			bumpType = semver.BumpMajor
		case "minor":
			bumpType = semver.BumpMinor
		case "patch":
			bumpType = semver.BumpPatch
		default:
			return fmt.Errorf("invalid bump type: %s (use major, minor, or patch)", bumpTypeStr)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		s, err := skill.Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse skill: %w", err)
		}

		oldVersion := s.Frontmatter.Version
		if oldVersion == "" {
			oldVersion = "0.0.0"
		}

		v, err := semver.Parse(oldVersion)
		if err != nil {
			return fmt.Errorf("invalid current version: %w", err)
		}

		newVersion := v.Bump(bumpType)

		// Update the file
		contentStr := string(content)
		if oldVersion == "0.0.0" {
			// Add version field if missing
			contentStr = addVersionToFrontmatter(contentStr, newVersion.String())
		} else {
			// Replace existing version
			contentStr = replaceVersion(contentStr, oldVersion, newVersion.String())
		}

		if err := os.WriteFile(filePath, []byte(contentStr), 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		fmt.Printf("Version bumped: %s -> %s\n", oldVersion, newVersion.String())
		return nil
	},
}

var versionCheckCmd = &cobra.Command{
	Use:   "check [file]",
	Short: "Check if version is valid semantic version",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		content, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		s, err := skill.Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse skill: %w", err)
		}

		if s.Frontmatter.Version == "" {
			fmt.Println("✗ No version found in frontmatter")
			return fmt.Errorf("missing version")
		}

		v, err := semver.Parse(s.Frontmatter.Version)
		if err != nil {
			fmt.Printf("✗ Invalid version: %s\n", s.Frontmatter.Version)
			return fmt.Errorf("invalid version: %w", err)
		}

		fmt.Printf("✓ Valid semantic version: %s\n", v.String())
		fmt.Printf("  Major: %d\n", v.Major)
		fmt.Printf("  Minor: %d\n", v.Minor)
		fmt.Printf("  Patch: %d\n", v.Patch)
		if v.Prerelease != "" {
			fmt.Printf("  Prerelease: %s\n", v.Prerelease)
		}
		if v.Build != "" {
			fmt.Printf("  Build: %s\n", v.Build)
		}

		return nil
	},
}

func addVersionToFrontmatter(content, version string) string {
	// Find the frontmatter section and add version after name
	lines := strings.Split(content, "\n")
	var result []string
	inFrontmatter := false
	addedVersion := false

	for _, line := range lines {
		result = append(result, line)
		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
			} else {
				inFrontmatter = false
			}
		}
		if inFrontmatter && strings.HasPrefix(line, "name:") && !addedVersion {
			result = append(result, fmt.Sprintf("version: \"%s\"", version))
			addedVersion = true
		}
	}

	return strings.Join(result, "\n")
}

func replaceVersion(content, oldVersion, newVersion string) string {
	// Replace version in frontmatter
	oldPattern := fmt.Sprintf("version: \"%s\"", oldVersion)
	newPattern := fmt.Sprintf("version: \"%s\"", newVersion)
	if strings.Contains(content, oldPattern) {
		return strings.Replace(content, oldPattern, newPattern, 1)
	}

	// Try without quotes
	oldPattern = fmt.Sprintf("version: %s", oldVersion)
	newPattern = fmt.Sprintf("version: \"%s\"", newVersion)
	if strings.Contains(content, oldPattern) {
		return strings.Replace(content, oldPattern, newPattern, 1)
	}

	// Try with single quotes
	oldPattern = fmt.Sprintf("version: '%s'", oldVersion)
	newPattern = fmt.Sprintf("version: \"%s\"", newVersion)
	return strings.Replace(content, oldPattern, newPattern, 1)
}

func init() {
	skillVersionCmd.AddCommand(versionGetCmd)
	skillVersionCmd.AddCommand(versionBumpCmd)
	skillVersionCmd.AddCommand(versionCheckCmd)
	rootCmd.AddCommand(skillVersionCmd)
}
