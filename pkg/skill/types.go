// Package skill provides types and utilities for SKILL.md files.
package skill

import "time"

// Skill represents a complete SKILL.md document.
type Skill struct {
	Frontmatter Frontmatter `json:"frontmatter"`
	Content     string      `json:"content"`
	Sections    []Section   `json:"sections"`
	Raw         string      `json:"-"`
}

// ToolDefinition represents an MCP-compatible tool definition with JSON Schema parameters.
type ToolDefinition struct {
	Name        string                 `yaml:"name" json:"name"`
	Description string                 `yaml:"description" json:"description"`
	Parameters  map[string]interface{} `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	Required    []string               `yaml:"required,omitempty" json:"required,omitempty"`
}

// RetryStrategy defines retry behavior for API operations.
type RetryStrategy struct {
	MaxRetries     int    `yaml:"max_retries" json:"max_retries"`
	BackoffType    string `yaml:"backoff_type" json:"backoff_type"` // linear, exponential
	InitialDelayMs int    `yaml:"initial_delay_ms" json:"initial_delay_ms"`
}

// RateLimitInfo documents rate limiting for an API.
type RateLimitInfo struct {
	RequestsPerMinute int    `yaml:"requests_per_minute,omitempty" json:"requests_per_minute,omitempty"`
	RequestsPerHour   int    `yaml:"requests_per_hour,omitempty" json:"requests_per_hour,omitempty"`
	RequestsPerDay    int    `yaml:"requests_per_day,omitempty" json:"requests_per_day,omitempty"`
	BurstLimit        int    `yaml:"burst_limit,omitempty" json:"burst_limit,omitempty"`
	RetryAfterHeader  string `yaml:"retry_after_header,omitempty" json:"retry_after_header,omitempty"`
}

// Frontmatter contains SKILL.md metadata.
type Frontmatter struct {
	Name        string   `yaml:"name" json:"name"`
	Version     string   `yaml:"version" json:"version"`
	Description string   `yaml:"description" json:"description"`
	Author      string   `yaml:"author,omitempty" json:"author,omitempty"`
	Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Source      string   `yaml:"source,omitempty" json:"source,omitempty"`
	SourceType  string   `yaml:"source_type,omitempty" json:"source_type,omitempty"`
	CreatedAt   string   `yaml:"created_at,omitempty" json:"created_at,omitempty"`
	UpdatedAt   string   `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
	// Enhanced metadata fields
	Difficulty    string   `yaml:"difficulty,omitempty" json:"difficulty,omitempty"` // novice/intermediate/advanced
	EndpointCount int      `yaml:"endpoint_count,omitempty" json:"endpoint_count,omitempty"`
	AuthMethods   []string `yaml:"auth_methods,omitempty" json:"auth_methods,omitempty"`
	BaseURL       string   `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	HasExamples   bool     `yaml:"has_examples,omitempty" json:"has_examples,omitempty"`

	// AI Agent Metadata (MCP Compatible)
	MCPCompatible    bool             `yaml:"mcp_compatible,omitempty" json:"mcp_compatible,omitempty"`
	ToolDefinitions  []ToolDefinition `yaml:"tools,omitempty" json:"tools,omitempty"`
	MaxTokensPerCall int              `yaml:"max_tokens_per_call,omitempty" json:"max_tokens_per_call,omitempty"`
	RetryStrategy    *RetryStrategy   `yaml:"retry_strategy,omitempty" json:"retry_strategy,omitempty"`
	RateLimits       *RateLimitInfo   `yaml:"rate_limits,omitempty" json:"rate_limits,omitempty"`

	// Protocol-specific fields (for AsyncAPI, gRPC, etc.)
	Protocol     string   `yaml:"protocol,omitempty" json:"protocol,omitempty"` // http, grpc, websocket, kafka, mqtt, amqp
	ChannelCount int      `yaml:"channel_count,omitempty" json:"channel_count,omitempty"`
	ServiceCount int      `yaml:"service_count,omitempty" json:"service_count,omitempty"`
	MessageCount int      `yaml:"message_count,omitempty" json:"message_count,omitempty"`
	Servers      []string `yaml:"servers,omitempty" json:"servers,omitempty"`
}

// Section represents a section within a SKILL.md file.
type Section struct {
	Title   string `json:"title"`
	Level   int    `json:"level"`
	Content string `json:"content"`
}

// StoredSkill represents a skill stored in the registry.
type StoredSkill struct {
	ID           string    `json:"id"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Description  string    `json:"description"`
	Content      string    `json:"content"`
	ContentHash  string    `json:"content_hash"`
	SourceFormat string    `json:"source_format"`
	Tags         []string  `json:"tags"`
	ViewCount    int64     `json:"view_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NewSkill creates a new Skill with default values.
func NewSkill(name, description string) *Skill {
	return &Skill{
		Frontmatter: Frontmatter{
			Name:        name,
			Version:     "1.0.0",
			Description: description,
			CreatedAt:   time.Now().Format(time.RFC3339),
		},
		Sections: []Section{},
	}
}

// AddSection adds a new section to the skill.
func (s *Skill) AddSection(title string, level int, content string) {
	s.Sections = append(s.Sections, Section{
		Title:   title,
		Level:   level,
		Content: content,
	})
}
