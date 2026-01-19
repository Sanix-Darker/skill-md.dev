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
	Difficulty    string   `yaml:"difficulty,omitempty" json:"difficulty,omitempty"`       // novice/intermediate/advanced
	EndpointCount int      `yaml:"endpoint_count,omitempty" json:"endpoint_count,omitempty"`
	AuthMethods   []string `yaml:"auth_methods,omitempty" json:"auth_methods,omitempty"`
	BaseURL       string   `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	HasExamples   bool     `yaml:"has_examples,omitempty" json:"has_examples,omitempty"`
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
