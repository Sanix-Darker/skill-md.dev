// Package sources provides external skill registry integrations.
package sources

import (
	"context"
	"time"
)

// SourceType identifies the origin of a skill.
type SourceType string

const (
	SourceTypeLocal     SourceType = "local"
	SourceTypeSkillsSH  SourceType = "skills.sh"
	SourceTypeGitHub    SourceType = "github"
	SourceTypeGitLab    SourceType = "gitlab"
	SourceTypeBitbucket SourceType = "bitbucket"
	SourceTypeCodeberg  SourceType = "codeberg"
)

// ExternalSkill represents a skill from any source.
type ExternalSkill struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Content     string     `json:"content,omitempty"` // May be lazy-loaded
	Tags        []string   `json:"tags,omitempty"`
	Source      SourceType `json:"source"`
	SourceURL   string     `json:"source_url"` // Link to original
	RepoOwner   string     `json:"repo_owner,omitempty"`
	RepoName    string     `json:"repo_name,omitempty"`
	Stars       int        `json:"stars,omitempty"`
	ContentURL  string     `json:"content_url,omitempty"` // For lazy content fetch
	Version     string     `json:"version,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at,omitempty"`
}

// SearchOptions configures a search request.
type SearchOptions struct {
	Query   string
	Tags    []string
	Page    int
	PerPage int
}

// SearchResult contains search results from a source.
type SearchResult struct {
	Skills     []*ExternalSkill
	Total      int
	Page       int
	PerPage    int
	SearchTime time.Duration
	Source     SourceType
}

// Source defines the interface for skill sources.
type Source interface {
	// Name returns the source type identifier.
	Name() SourceType

	// Search finds skills matching the options.
	Search(ctx context.Context, opts SearchOptions) (*SearchResult, error)

	// GetSkill retrieves a specific skill by ID.
	GetSkill(ctx context.Context, id string) (*ExternalSkill, error)

	// GetContent fetches the full content for a skill (lazy loading).
	GetContent(ctx context.Context, skill *ExternalSkill) (string, error)

	// Enabled returns whether this source is enabled.
	Enabled() bool
}

// SourceConfig holds configuration for external sources.
type SourceConfig struct {
	GitHubToken    string
	GitLabToken    string
	BitbucketUser  string
	BitbucketPass  string
	CodebergToken  string
	EnabledSources []SourceType
}

// DefaultEnabledSources returns the default set of enabled sources.
func DefaultEnabledSources() []SourceType {
	return []SourceType{
		SourceTypeLocal,
		SourceTypeSkillsSH,
		SourceTypeGitHub,
		SourceTypeGitLab,
		SourceTypeBitbucket,
		SourceTypeCodeberg,
	}
}

// IsSourceEnabled checks if a source type is in the enabled list.
func IsSourceEnabled(sources []SourceType, source SourceType) bool {
	for _, s := range sources {
		if s == source {
			return true
		}
	}
	return false
}

// SourceLabel returns a human-readable label for a source.
func SourceLabel(s SourceType) string {
	labels := map[SourceType]string{
		SourceTypeLocal:     "Local",
		SourceTypeSkillsSH:  "SKILLS.sh",
		SourceTypeGitHub:    "GitHub",
		SourceTypeGitLab:    "GitLab",
		SourceTypeBitbucket: "Bitbucket",
		SourceTypeCodeberg:  "Codeberg",
	}
	if label, ok := labels[s]; ok {
		return label
	}
	return string(s)
}

// SourceColor returns a CSS color class for a source badge.
func SourceColor(s SourceType) string {
	colors := map[SourceType]string{
		SourceTypeLocal:     "border-terminal-accent text-terminal-accent",
		SourceTypeSkillsSH:  "border-purple-500 text-purple-400",
		SourceTypeGitHub:    "border-gray-400 text-gray-300",
		SourceTypeGitLab:    "border-orange-500 text-orange-400",
		SourceTypeBitbucket: "border-blue-500 text-blue-400",
		SourceTypeCodeberg:  "border-green-500 text-green-400",
	}
	if color, ok := colors[s]; ok {
		return color
	}
	return "border-terminal-border text-terminal-muted"
}
