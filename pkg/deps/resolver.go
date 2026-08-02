// Package deps provides dependency resolution for SKILL.md files.
package deps

import (
	"fmt"
	"sort"

	"github.com/sanixdarker/skillf/pkg/semver"
)

// Dependency represents a skill dependency.
type Dependency struct {
	Name       string `yaml:"name" json:"name"`
	Version    string `yaml:"version" json:"version"`
	Constraint *semver.Constraint
}

// ParseDependency creates a Dependency from name and version constraint.
func ParseDependency(name, version string) (*Dependency, error) {
	constraint, err := semver.ParseConstraint(version)
	if err != nil {
		return nil, fmt.Errorf("invalid version constraint for %s: %w", name, err)
	}
	return &Dependency{
		Name:       name,
		Version:    version,
		Constraint: constraint,
	}, nil
}

// ResolvedDependency represents a resolved dependency with a specific version.
type ResolvedDependency struct {
	Name         string          `json:"name"`
	Version      *semver.Version `json:"version"`
	ContentHash  string          `json:"content_hash,omitempty"`
	Source       string          `json:"source,omitempty"`
	Dependencies []string        `json:"dependencies,omitempty"`
}

// DependencyGraph represents a graph of skill dependencies.
type DependencyGraph struct {
	nodes map[string]*graphNode
}

type graphNode struct {
	name         string
	constraint   *semver.Constraint
	dependencies []string
	resolved     *semver.Version
}

// NewDependencyGraph creates a new dependency graph.
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		nodes: make(map[string]*graphNode),
	}
}

// AddDependency adds a dependency to the graph.
func (g *DependencyGraph) AddDependency(name string, constraint *semver.Constraint, deps []string) {
	g.nodes[name] = &graphNode{
		name:         name,
		constraint:   constraint,
		dependencies: deps,
	}
}

// DetectCycles checks for circular dependencies using DFS.
func (g *DependencyGraph) DetectCycles() ([]string, bool) {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	var cyclePath []string

	var dfs func(node string) bool
	dfs = func(node string) bool {
		visited[node] = true
		recStack[node] = true
		cyclePath = append(cyclePath, node)

		n, exists := g.nodes[node]
		if !exists {
			cyclePath = cyclePath[:len(cyclePath)-1]
			recStack[node] = false
			return false
		}

		for _, dep := range n.dependencies {
			if !visited[dep] {
				if dfs(dep) {
					return true
				}
			} else if recStack[dep] {
				// Found cycle, trim path to show cycle
				for i, p := range cyclePath {
					if p == dep {
						cyclePath = append(cyclePath[i:], dep)
						break
					}
				}
				return true
			}
		}

		cyclePath = cyclePath[:len(cyclePath)-1]
		recStack[node] = false
		return false
	}

	for node := range g.nodes {
		if !visited[node] {
			if dfs(node) {
				return cyclePath, true
			}
		}
	}

	return nil, false
}

// TopologicalSort returns nodes in dependency order (dependencies first).
func (g *DependencyGraph) TopologicalSort() ([]string, error) {
	if cycle, hasCycle := g.DetectCycles(); hasCycle {
		return nil, fmt.Errorf("circular dependency detected: %v", cycle)
	}

	visited := make(map[string]bool)
	var result []string

	var visit func(node string)
	visit = func(node string) {
		if visited[node] {
			return
		}
		visited[node] = true

		if n, exists := g.nodes[node]; exists {
			for _, dep := range n.dependencies {
				visit(dep)
			}
		}
		result = append(result, node)
	}

	// Sort nodes for deterministic output
	nodes := make([]string, 0, len(g.nodes))
	for node := range g.nodes {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)

	for _, node := range nodes {
		visit(node)
	}

	return result, nil
}

// Resolver resolves skill dependencies to specific versions.
type Resolver struct {
	// VersionProvider returns available versions for a skill name.
	VersionProvider func(name string) ([]*semver.Version, error)
}

// NewResolver creates a new dependency resolver.
func NewResolver(provider func(string) ([]*semver.Version, error)) *Resolver {
	return &Resolver{VersionProvider: provider}
}

// ResolutionResult contains the result of dependency resolution.
type ResolutionResult struct {
	Resolved []ResolvedDependency `json:"resolved"`
	Errors   []ResolutionError    `json:"errors,omitempty"`
}

// ResolutionError represents an error during resolution.
type ResolutionError struct {
	Dependency string `json:"dependency"`
	Constraint string `json:"constraint"`
	Message    string `json:"message"`
}

// Resolve resolves all dependencies in the graph to specific versions.
func (r *Resolver) Resolve(graph *DependencyGraph) (*ResolutionResult, error) {
	result := &ResolutionResult{}

	// Get topological order
	order, err := graph.TopologicalSort()
	if err != nil {
		return nil, err
	}

	// Resolve each dependency in order
	for _, name := range order {
		node := graph.nodes[name]
		if node == nil {
			continue
		}

		versions, err := r.VersionProvider(name)
		if err != nil {
			result.Errors = append(result.Errors, ResolutionError{
				Dependency: name,
				Constraint: node.constraint.String(),
				Message:    fmt.Sprintf("failed to fetch versions: %v", err),
			})
			continue
		}

		if len(versions) == 0 {
			result.Errors = append(result.Errors, ResolutionError{
				Dependency: name,
				Constraint: node.constraint.String(),
				Message:    "no versions available",
			})
			continue
		}

		// Find best matching version
		bestMatch := node.constraint.FindBestMatch(versions)
		if bestMatch == nil {
			result.Errors = append(result.Errors, ResolutionError{
				Dependency: name,
				Constraint: node.constraint.String(),
				Message:    fmt.Sprintf("no version satisfies constraint %s", node.constraint),
			})
			continue
		}

		node.resolved = bestMatch
		result.Resolved = append(result.Resolved, ResolvedDependency{
			Name:         name,
			Version:      bestMatch,
			Dependencies: node.dependencies,
		})
	}

	return result, nil
}

// DependencyTree represents a tree view of dependencies.
type DependencyTree struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Constraint   string            `json:"constraint,omitempty"`
	Dependencies []*DependencyTree `json:"dependencies,omitempty"`
}

// BuildTree builds a dependency tree for visualization.
func (g *DependencyGraph) BuildTree(root string) *DependencyTree {
	visited := make(map[string]bool)

	var build func(name string) *DependencyTree
	build = func(name string) *DependencyTree {
		if visited[name] {
			return &DependencyTree{Name: name, Version: "(circular)"}
		}
		visited[name] = true
		defer func() { visited[name] = false }()

		node := g.nodes[name]
		tree := &DependencyTree{Name: name}

		if node != nil {
			if node.resolved != nil {
				tree.Version = node.resolved.String()
			}
			if node.constraint != nil {
				tree.Constraint = node.constraint.String()
			}
			for _, dep := range node.dependencies {
				tree.Dependencies = append(tree.Dependencies, build(dep))
			}
		}

		return tree
	}

	return build(root)
}

// FormatTree formats a dependency tree as a string.
func FormatTree(tree *DependencyTree, prefix string, isLast bool) string {
	var result string

	// Determine the prefix for this line
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	// Format version info
	version := tree.Version
	if tree.Constraint != "" && tree.Version != "" {
		version = fmt.Sprintf("%s (%s)", tree.Version, tree.Constraint)
	}

	result += prefix + connector + tree.Name
	if version != "" {
		result += " @ " + version
	}
	result += "\n"

	// Determine prefix for children
	childPrefix := prefix
	if isLast {
		childPrefix += "    "
	} else {
		childPrefix += "│   "
	}

	for i, child := range tree.Dependencies {
		isChildLast := i == len(tree.Dependencies)-1
		result += FormatTree(child, childPrefix, isChildLast)
	}

	return result
}
