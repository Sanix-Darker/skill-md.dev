// Package converter provides spec-to-SKILL.md converters.
package converter

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sanixdarker/skillforge/pkg/skill"
)

// Converter defines the interface for spec converters.
type Converter interface {
	// Name returns the converter name.
	Name() string
	// Convert transforms input content into a Skill.
	Convert(content []byte, opts *Options) (*skill.Skill, error)
	// CanHandle returns true if this converter can handle the content.
	CanHandle(filename string, content []byte) bool
}

// Options holds converter options.
type Options struct {
	Name       string
	SourcePath string
}

// Manager manages available converters.
type Manager struct {
	converters []Converter
}

// NewManager creates a new converter manager with all built-in converters.
func NewManager() *Manager {
	m := &Manager{}
	m.Register(&OpenAPIConverter{})
	m.Register(&GraphQLConverter{})
	m.Register(&PostmanConverter{})
	m.Register(&AsyncAPIConverter{})
	m.Register(&ProtobufConverter{})
	m.Register(&RAMLConverter{})
	m.Register(&WSDLConverter{})
	m.Register(&APIBlueprintConverter{})
	m.Register(&PDFConverter{})
	m.Register(NewURLConverter())
	m.Register(&PlainTextConverter{})
	return m
}

// Register adds a converter to the manager.
func (m *Manager) Register(c Converter) {
	m.converters = append(m.converters, c)
}

// Convert converts content using the specified format.
func (m *Manager) Convert(format string, content []byte, opts *Options) (*skill.Skill, error) {
	for _, c := range m.converters {
		if strings.EqualFold(c.Name(), format) {
			return c.Convert(content, opts)
		}
	}
	return nil, fmt.Errorf("unknown format: %s", format)
}

// DetectFormat detects the format of the input content.
func (m *Manager) DetectFormat(filename string, content []byte) string {
	for _, c := range m.converters {
		if c.CanHandle(filename, content) {
			return c.Name()
		}
	}
	return "text"
}

// SupportedFormats returns a list of supported formats.
func (m *Manager) SupportedFormats() []string {
	formats := make([]string, len(m.converters))
	for i, c := range m.converters {
		formats[i] = c.Name()
	}
	return formats
}

// getExtension returns the lowercase file extension.
func getExtension(filename string) string {
	return strings.ToLower(filepath.Ext(filename))
}
