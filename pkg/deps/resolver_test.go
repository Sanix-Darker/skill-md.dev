package deps

import (
	"testing"

	"github.com/sanixdarker/skillf/pkg/semver"
)

func TestDependencyGraphCycleDetection(t *testing.T) {
	tests := []struct {
		name     string
		deps     map[string][]string
		hasCycle bool
	}{
		{
			name: "no cycle",
			deps: map[string][]string{
				"a": {"b", "c"},
				"b": {"c"},
				"c": {},
			},
			hasCycle: false,
		},
		{
			name: "simple cycle",
			deps: map[string][]string{
				"a": {"b"},
				"b": {"a"},
			},
			hasCycle: true,
		},
		{
			name: "indirect cycle",
			deps: map[string][]string{
				"a": {"b"},
				"b": {"c"},
				"c": {"a"},
			},
			hasCycle: true,
		},
		{
			name: "self cycle",
			deps: map[string][]string{
				"a": {"a"},
			},
			hasCycle: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewDependencyGraph()
			for name, deps := range tt.deps {
				c, _ := semver.ParseConstraint("*")
				g.AddDependency(name, c, deps)
			}

			_, hasCycle := g.DetectCycles()
			if hasCycle != tt.hasCycle {
				t.Errorf("DetectCycles() = %v, want %v", hasCycle, tt.hasCycle)
			}
		})
	}
}

func TestTopologicalSort(t *testing.T) {
	g := NewDependencyGraph()

	// c depends on nothing
	// b depends on c
	// a depends on b and c
	c, _ := semver.ParseConstraint("*")
	g.AddDependency("c", c, nil)
	g.AddDependency("b", c, []string{"c"})
	g.AddDependency("a", c, []string{"b", "c"})

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort() error = %v", err)
	}

	// c must come before b
	// b must come before a
	cIdx, bIdx, aIdx := -1, -1, -1
	for i, n := range order {
		switch n {
		case "a":
			aIdx = i
		case "b":
			bIdx = i
		case "c":
			cIdx = i
		}
	}

	if cIdx > bIdx {
		t.Error("c should come before b")
	}
	if bIdx > aIdx {
		t.Error("b should come before a")
	}
}

func TestResolver(t *testing.T) {
	// Mock version provider
	provider := func(name string) ([]*semver.Version, error) {
		versions := map[string][]*semver.Version{
			"lib-a": {
				semver.MustParse("1.0.0"),
				semver.MustParse("1.1.0"),
				semver.MustParse("2.0.0"),
			},
			"lib-b": {
				semver.MustParse("1.0.0"),
				semver.MustParse("1.5.0"),
			},
		}
		return versions[name], nil
	}

	g := NewDependencyGraph()
	cA, _ := semver.ParseConstraint("^1.0.0")
	cB, _ := semver.ParseConstraint(">=1.0.0")
	g.AddDependency("lib-a", cA, nil)
	g.AddDependency("lib-b", cB, nil)

	resolver := NewResolver(provider)
	result, err := resolver.Resolve(g)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(result.Errors) > 0 {
		t.Errorf("Resolve() had errors: %v", result.Errors)
	}

	// lib-a should resolve to 1.1.0 (highest matching ^1.0.0)
	// lib-b should resolve to 1.5.0 (highest matching >=1.0.0)
	for _, dep := range result.Resolved {
		switch dep.Name {
		case "lib-a":
			if dep.Version.String() != "1.1.0" {
				t.Errorf("lib-a resolved to %s, want 1.1.0", dep.Version)
			}
		case "lib-b":
			if dep.Version.String() != "1.5.0" {
				t.Errorf("lib-b resolved to %s, want 1.5.0", dep.Version)
			}
		}
	}
}

func TestBuildTree(t *testing.T) {
	g := NewDependencyGraph()
	c, _ := semver.ParseConstraint("^1.0.0")

	g.AddDependency("root", c, []string{"child1", "child2"})
	g.AddDependency("child1", c, []string{"grandchild"})
	g.AddDependency("child2", c, nil)
	g.AddDependency("grandchild", c, nil)

	tree := g.BuildTree("root")

	if tree.Name != "root" {
		t.Errorf("Root name = %s, want root", tree.Name)
	}
	if len(tree.Dependencies) != 2 {
		t.Errorf("Root has %d children, want 2", len(tree.Dependencies))
	}

	// Check child1 has grandchild
	for _, child := range tree.Dependencies {
		if child.Name == "child1" {
			if len(child.Dependencies) != 1 {
				t.Errorf("child1 has %d children, want 1", len(child.Dependencies))
			}
			if child.Dependencies[0].Name != "grandchild" {
				t.Errorf("child1's child is %s, want grandchild", child.Dependencies[0].Name)
			}
		}
	}
}

func TestFormatTree(t *testing.T) {
	tree := &DependencyTree{
		Name:    "root",
		Version: "1.0.0",
		Dependencies: []*DependencyTree{
			{
				Name:    "child1",
				Version: "2.0.0",
				Dependencies: []*DependencyTree{
					{Name: "grandchild", Version: "3.0.0"},
				},
			},
			{Name: "child2", Version: "1.5.0"},
		},
	}

	output := FormatTree(tree, "", true)

	// Just check it doesn't panic and produces output
	if len(output) == 0 {
		t.Error("FormatTree returned empty string")
	}

	// Should contain all names
	for _, name := range []string{"root", "child1", "child2", "grandchild"} {
		if !containsString(output, name) {
			t.Errorf("Output missing %s", name)
		}
	}
}

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
