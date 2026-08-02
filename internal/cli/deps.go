package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sanixdarker/skillf/pkg/deps"
	"github.com/sanixdarker/skillf/pkg/semver"
	"github.com/sanixdarker/skillf/pkg/skill"
	"github.com/spf13/cobra"
)

var depsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Manage SKILL.md dependencies",
	Long: `Commands for managing SKILL.md dependencies.

Examples:
	  skillf deps resolve skill.md
	  skillf deps tree skill.md
	  skillf deps check skill.md`,
}

var depsResolveCmd = &cobra.Command{
	Use:   "resolve [file]",
	Short: "Resolve dependencies and generate skill.lock",
	Long: `Resolve all dependencies in a SKILL.md file to specific versions
and generate a skill.lock file.

The lock file ensures reproducible builds by pinning exact versions.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		s, err := skill.Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse skill: %w", err)
		}

		dependencies := extractDependencies(s)
		if len(dependencies) == 0 {
			fmt.Println("No dependencies found in skill")
			return nil
		}

		// Build dependency graph
		graph := deps.NewDependencyGraph()
		for _, dep := range dependencies {
			constraint, err := semver.ParseConstraint(dep.Version)
			if err != nil {
				return fmt.Errorf("invalid constraint for %s: %w", dep.Name, err)
			}
			graph.AddDependency(dep.Name, constraint, nil)
		}

		// Check for cycles
		if cycle, hasCycle := graph.DetectCycles(); hasCycle {
			return fmt.Errorf("circular dependency detected: %v", cycle)
		}

		// Create resolver with mock version provider
		// In real usage, this would query a registry
		resolver := deps.NewResolver(mockVersionProvider)
		result, err := resolver.Resolve(graph)
		if err != nil {
			return fmt.Errorf("resolution failed: %w", err)
		}

		// Check for errors
		if len(result.Errors) > 0 {
			fmt.Println("Resolution errors:")
			for _, e := range result.Errors {
				fmt.Printf("  - %s (%s): %s\n", e.Dependency, e.Constraint, e.Message)
			}
		}

		// Generate lockfile
		lockPath := filepath.Join(filepath.Dir(filePath), "skill.lock")
		lf := deps.NewLockfile(s.Frontmatter.Name, string(content), result)

		if err := lf.Write(lockPath); err != nil {
			return fmt.Errorf("failed to write lockfile: %w", err)
		}

		fmt.Printf("Resolved %d dependencies\n", len(result.Resolved))
		fmt.Printf("Lockfile written to: %s\n", lockPath)

		return nil
	},
}

var depsTreeCmd = &cobra.Command{
	Use:   "tree [file]",
	Short: "Display dependency tree",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		s, err := skill.Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse skill: %w", err)
		}

		dependencies := extractDependencies(s)
		if len(dependencies) == 0 {
			fmt.Println("No dependencies found")
			return nil
		}

		fmt.Printf("%s@%s\n", s.Frontmatter.Name, s.Frontmatter.Version)
		for i, dep := range dependencies {
			isLast := i == len(dependencies)-1
			prefix := "├── "
			if isLast {
				prefix = "└── "
			}
			fmt.Printf("%s%s (%s)\n", prefix, dep.Name, dep.Version)
		}

		return nil
	},
}

var depsCheckCmd = &cobra.Command{
	Use:   "check [file]",
	Short: "Check for dependency conflicts",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		s, err := skill.Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse skill: %w", err)
		}

		dependencies := extractDependencies(s)
		if len(dependencies) == 0 {
			fmt.Println("✓ No dependencies to check")
			return nil
		}

		// Build dependency graph
		graph := deps.NewDependencyGraph()
		hasErrors := false

		for _, dep := range dependencies {
			constraint, err := semver.ParseConstraint(dep.Version)
			if err != nil {
				fmt.Printf("✗ Invalid constraint for %s: %s\n", dep.Name, dep.Version)
				hasErrors = true
				continue
			}
			graph.AddDependency(dep.Name, constraint, nil)
		}

		// Check for cycles
		if cycle, hasCycle := graph.DetectCycles(); hasCycle {
			fmt.Printf("✗ Circular dependency: %v\n", cycle)
			hasErrors = true
		}

		// Check lockfile
		lockPath := filepath.Join(filepath.Dir(filePath), "skill.lock")
		lf, err := deps.ReadLockfile(lockPath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("⚠ No lockfile found. Run 'skillf deps resolve' to generate one.")
			} else {
				fmt.Printf("✗ Failed to read lockfile: %v\n", err)
				hasErrors = true
			}
		} else {
			if lf.IsStale(string(content)) {
				fmt.Println("⚠ Lockfile is stale. Run 'skillf deps resolve' to update.")
			} else {
				fmt.Println("✓ Lockfile is up to date")
			}
		}

		if !hasErrors {
			fmt.Printf("✓ All %d dependencies valid\n", len(dependencies))
		}

		return nil
	},
}

// SimpleDependency represents a simple dependency for extraction
type SimpleDependency struct {
	Name    string
	Version string
}

func extractDependencies(s *skill.Skill) []SimpleDependency {
	var dependencies []SimpleDependency

	// First, check native frontmatter dependencies
	for _, dep := range s.Frontmatter.Dependencies {
		if dep.Name != "" {
			dependencies = append(dependencies, SimpleDependency{
				Name:    dep.Name,
				Version: dep.Version,
			})
		}
	}

	// If no frontmatter dependencies, look in content sections
	if len(dependencies) == 0 {
		for _, section := range s.Sections {
			if strings.EqualFold(section.Title, "dependencies") ||
				strings.EqualFold(section.Title, "requirements") {
				// Parse dependency list from section content
				lines := strings.Split(section.Content, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					// Parse patterns like "- skill-name: ^1.0.0" or "- skill-name@^1.0.0"
					if strings.HasPrefix(line, "- ") {
						dep := parseDependencyLine(strings.TrimPrefix(line, "- "))
						if dep.Name != "" {
							dependencies = append(dependencies, dep)
						}
					}
				}
			}
		}
	}

	return dependencies
}

func parseDependencyLine(line string) SimpleDependency {
	// Handle "name: version" format
	if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
		return SimpleDependency{
			Name:    strings.TrimSpace(parts[0]),
			Version: strings.TrimSpace(parts[1]),
		}
	}

	// Handle "name@version" format
	if parts := strings.SplitN(line, "@", 2); len(parts) == 2 {
		return SimpleDependency{
			Name:    strings.TrimSpace(parts[0]),
			Version: strings.TrimSpace(parts[1]),
		}
	}

	// Just name, assume any version
	if line != "" && !strings.ContainsAny(line, " \t") {
		return SimpleDependency{
			Name:    line,
			Version: "*",
		}
	}

	return SimpleDependency{}
}

// mockVersionProvider is a placeholder for testing
// In production, this would query a skill registry
func mockVersionProvider(name string) ([]*semver.Version, error) {
	// Return some mock versions for testing
	versions := []*semver.Version{
		semver.MustParse("1.0.0"),
		semver.MustParse("1.1.0"),
		semver.MustParse("1.2.0"),
		semver.MustParse("2.0.0"),
	}
	return versions, nil
}

func init() {
	depsCmd.AddCommand(depsResolveCmd)
	depsCmd.AddCommand(depsTreeCmd)
	depsCmd.AddCommand(depsCheckCmd)
	rootCmd.AddCommand(depsCmd)
}
