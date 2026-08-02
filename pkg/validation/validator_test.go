package validation

import (
	"testing"

	"github.com/sanixdarker/skillf/pkg/skill"
)

func TestValidatorBasic(t *testing.T) {
	tests := []struct {
		name      string
		skill     *skill.Skill
		wantValid bool
		wantCodes []ValidationCode
	}{
		{
			name: "valid skill",
			skill: &skill.Skill{
				Frontmatter: skill.Frontmatter{
					Name:        "Test Skill",
					Version:     "1.0.0",
					Description: "A test skill",
				},
				Sections: []skill.Section{
					{Title: "Overview", Level: 1, Content: "This is an overview."},
					{Title: "Quick Start", Level: 2, Content: "Get started quickly."},
				},
			},
			wantValid: true,
			wantCodes: nil,
		},
		{
			name: "missing name",
			skill: &skill.Skill{
				Frontmatter: skill.Frontmatter{
					Version: "1.0.0",
				},
				Content: "Some content",
			},
			wantValid: false,
			wantCodes: []ValidationCode{CodeMissingName},
		},
		{
			name: "missing version",
			skill: &skill.Skill{
				Frontmatter: skill.Frontmatter{
					Name: "Test",
				},
				Content: "Some content",
			},
			wantValid: false,
			wantCodes: []ValidationCode{CodeMissingVersion},
		},
		{
			name: "invalid version format",
			skill: &skill.Skill{
				Frontmatter: skill.Frontmatter{
					Name:    "Test",
					Version: "not-a-version",
				},
				Content: "Some content",
			},
			wantValid: false,
			wantCodes: []ValidationCode{CodeInvalidVersion},
		},
		{
			name: "no content",
			skill: &skill.Skill{
				Frontmatter: skill.Frontmatter{
					Name:    "Test",
					Version: "1.0.0",
				},
			},
			wantValid: false,
			wantCodes: []ValidationCode{CodeNoContent},
		},
		{
			name: "skipped heading level",
			skill: &skill.Skill{
				Frontmatter: skill.Frontmatter{
					Name:    "Test",
					Version: "1.0.0",
				},
				Sections: []skill.Section{
					{Title: "Overview", Level: 1, Content: "Overview content"},
					{Title: "Deep", Level: 3, Content: "Too deep"}, // skips h2
				},
			},
			wantValid: true, // warnings don't make it invalid
			wantCodes: []ValidationCode{CodeSkippedHeading},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewDefaultValidator()
			result := v.Validate(tt.skill)

			if result.Valid != tt.wantValid {
				t.Errorf("Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			for _, wantCode := range tt.wantCodes {
				found := false
				for _, issue := range result.Issues {
					if issue.Code == wantCode {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected code %s not found in issues", wantCode)
				}
			}
		})
	}
}

func TestValidatorMCP(t *testing.T) {
	tests := []struct {
		name      string
		skill     *skill.Skill
		wantCodes []ValidationCode
	}{
		{
			name: "mcp compatible with no tools",
			skill: &skill.Skill{
				Frontmatter: skill.Frontmatter{
					Name:          "Test",
					Version:       "1.0.0",
					MCPCompatible: true,
				},
				Content: "Content",
			},
			wantCodes: []ValidationCode{CodeMCPNoTools},
		},
		{
			name: "tool without name",
			skill: &skill.Skill{
				Frontmatter: skill.Frontmatter{
					Name:          "Test",
					Version:       "1.0.0",
					MCPCompatible: true,
					ToolDefinitions: []skill.ToolDefinition{
						{Description: "A tool without name"},
					},
				},
				Content: "Content",
			},
			wantCodes: []ValidationCode{CodeMCPMissingToolName},
		},
		{
			name: "tool without description",
			skill: &skill.Skill{
				Frontmatter: skill.Frontmatter{
					Name:          "Test",
					Version:       "1.0.0",
					MCPCompatible: true,
					ToolDefinitions: []skill.ToolDefinition{
						{Name: "my_tool"},
					},
				},
				Content: "Content",
			},
			wantCodes: []ValidationCode{CodeMCPMissingToolDesc},
		},
		{
			name: "valid mcp skill",
			skill: &skill.Skill{
				Frontmatter: skill.Frontmatter{
					Name:          "Test",
					Version:       "1.0.0",
					MCPCompatible: true,
					ToolDefinitions: []skill.ToolDefinition{
						{
							Name:        "my_tool",
							Description: "Does something",
							Parameters: map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"id": map[string]interface{}{
										"type": "string",
									},
								},
							},
						},
					},
					RateLimits: &skill.RateLimitInfo{
						RequestsPerMinute: 60,
					},
					RetryStrategy: &skill.RetryStrategy{
						MaxRetries:  3,
						BackoffType: "exponential",
					},
				},
				Content: "Content",
			},
			wantCodes: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultOptions()
			opts.MCPCompat = true
			v := NewValidator(opts)
			result := v.Validate(tt.skill)

			for _, wantCode := range tt.wantCodes {
				found := false
				for _, issue := range result.Issues {
					if issue.Code == wantCode {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected code %s not found in issues", wantCode)
				}
			}
		})
	}
}

func TestValidatorSeverityFilter(t *testing.T) {
	s := &skill.Skill{
		Frontmatter: skill.Frontmatter{
			Name:    "Test",
			Version: "1.0.0",
		},
		Sections: []skill.Section{
			{Title: "Overview", Level: 1, Content: "Content"},
			{Title: "Empty", Level: 2, Content: ""}, // warning: empty section
		},
	}

	// With default options (MinSeverity = Info), should include warning
	v := NewDefaultValidator()
	result := v.Validate(s)

	foundWarning := false
	for _, issue := range result.Issues {
		if issue.Code == CodeEmptySection {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Error("Expected EmptySection warning with default options")
	}

	// With MinSeverity = Error, should not include warning
	opts := DefaultOptions()
	opts.MinSeverity = SeverityError
	v = NewValidator(opts)
	result = v.Validate(s)

	for _, issue := range result.Issues {
		if issue.Code == CodeEmptySection {
			t.Error("Should not include EmptySection warning when MinSeverity is Error")
		}
	}
}

func TestSeverityParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected Severity
		wantErr  bool
	}{
		{"info", SeverityInfo, false},
		{"warning", SeverityWarning, false},
		{"error", SeverityError, false},
		{"critical", SeverityCritical, false},
		{"unknown", SeverityInfo, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			sev, err := ParseSeverity(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSeverity(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err == nil && sev != tt.expected {
				t.Errorf("ParseSeverity(%q) = %v, want %v", tt.input, sev, tt.expected)
			}
		})
	}
}
