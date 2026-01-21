package skill

import (
	"fmt"
	"strings"
)

// renderParameters recursively renders JSON Schema parameters as YAML
func renderParameters(b *strings.Builder, params map[string]interface{}, indent string) {
	for key, value := range params {
		switch v := value.(type) {
		case string:
			b.WriteString(fmt.Sprintf("%s%s: %q\n", indent, key, v))
		case int, int64, float64:
			b.WriteString(fmt.Sprintf("%s%s: %v\n", indent, key, v))
		case bool:
			b.WriteString(fmt.Sprintf("%s%s: %t\n", indent, key, v))
		case []interface{}:
			b.WriteString(fmt.Sprintf("%s%s:\n", indent, key))
			for _, item := range v {
				if str, ok := item.(string); ok {
					b.WriteString(fmt.Sprintf("%s  - %q\n", indent, str))
				} else if m, ok := item.(map[string]interface{}); ok {
					b.WriteString(fmt.Sprintf("%s  -\n", indent))
					renderParameters(b, m, indent+"    ")
				}
			}
		case map[string]interface{}:
			b.WriteString(fmt.Sprintf("%s%s:\n", indent, key))
			renderParameters(b, v, indent+"  ")
		}
	}
}

// Render generates the SKILL.md content from a Skill struct.
func Render(s *Skill) string {
	if s == nil {
		return ""
	}

	var b strings.Builder

	// Write frontmatter
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("name: %q\n", s.Frontmatter.Name))
	b.WriteString(fmt.Sprintf("version: %q\n", s.Frontmatter.Version))
	if s.Frontmatter.Description != "" {
		b.WriteString(fmt.Sprintf("description: %q\n", s.Frontmatter.Description))
	}
	if s.Frontmatter.Author != "" {
		b.WriteString(fmt.Sprintf("author: %q\n", s.Frontmatter.Author))
	}
	if len(s.Frontmatter.Tags) > 0 {
		b.WriteString("tags:\n")
		for _, tag := range s.Frontmatter.Tags {
			b.WriteString(fmt.Sprintf("  - %q\n", tag))
		}
	}
	if s.Frontmatter.Source != "" {
		b.WriteString(fmt.Sprintf("source: %q\n", s.Frontmatter.Source))
	}
	if s.Frontmatter.SourceType != "" {
		b.WriteString(fmt.Sprintf("source_type: %q\n", s.Frontmatter.SourceType))
	}
	if s.Frontmatter.CreatedAt != "" {
		b.WriteString(fmt.Sprintf("created_at: %q\n", s.Frontmatter.CreatedAt))
	}
	if s.Frontmatter.UpdatedAt != "" {
		b.WriteString(fmt.Sprintf("updated_at: %q\n", s.Frontmatter.UpdatedAt))
	}
	// Enhanced metadata fields
	if s.Frontmatter.Difficulty != "" {
		b.WriteString(fmt.Sprintf("difficulty: %q\n", s.Frontmatter.Difficulty))
	}
	if s.Frontmatter.EndpointCount > 0 {
		b.WriteString(fmt.Sprintf("endpoint_count: %d\n", s.Frontmatter.EndpointCount))
	}
	if len(s.Frontmatter.AuthMethods) > 0 {
		b.WriteString("auth_methods:\n")
		for _, method := range s.Frontmatter.AuthMethods {
			b.WriteString(fmt.Sprintf("  - %q\n", method))
		}
	}
	if s.Frontmatter.BaseURL != "" {
		b.WriteString(fmt.Sprintf("base_url: %q\n", s.Frontmatter.BaseURL))
	}
	if s.Frontmatter.HasExamples {
		b.WriteString("has_examples: true\n")
	}
	// AI Agent Metadata (MCP Compatible)
	if s.Frontmatter.MCPCompatible {
		b.WriteString("mcp_compatible: true\n")
	}
	if len(s.Frontmatter.ToolDefinitions) > 0 {
		b.WriteString("tools:\n")
		for _, tool := range s.Frontmatter.ToolDefinitions {
			b.WriteString(fmt.Sprintf("  - name: %q\n", tool.Name))
			b.WriteString(fmt.Sprintf("    description: %q\n", tool.Description))
			if len(tool.Parameters) > 0 {
				b.WriteString("    parameters:\n")
				renderParameters(&b, tool.Parameters, "      ")
			}
			if len(tool.Required) > 0 {
				b.WriteString("    required:\n")
				for _, req := range tool.Required {
					b.WriteString(fmt.Sprintf("      - %q\n", req))
				}
			}
		}
	}
	if s.Frontmatter.MaxTokensPerCall > 0 {
		b.WriteString(fmt.Sprintf("max_tokens_per_call: %d\n", s.Frontmatter.MaxTokensPerCall))
	}
	if s.Frontmatter.RetryStrategy != nil {
		b.WriteString("retry_strategy:\n")
		b.WriteString(fmt.Sprintf("  max_retries: %d\n", s.Frontmatter.RetryStrategy.MaxRetries))
		b.WriteString(fmt.Sprintf("  backoff_type: %q\n", s.Frontmatter.RetryStrategy.BackoffType))
		b.WriteString(fmt.Sprintf("  initial_delay_ms: %d\n", s.Frontmatter.RetryStrategy.InitialDelayMs))
	}
	if s.Frontmatter.RateLimits != nil {
		b.WriteString("rate_limits:\n")
		if s.Frontmatter.RateLimits.RequestsPerMinute > 0 {
			b.WriteString(fmt.Sprintf("  requests_per_minute: %d\n", s.Frontmatter.RateLimits.RequestsPerMinute))
		}
		if s.Frontmatter.RateLimits.RequestsPerHour > 0 {
			b.WriteString(fmt.Sprintf("  requests_per_hour: %d\n", s.Frontmatter.RateLimits.RequestsPerHour))
		}
		if s.Frontmatter.RateLimits.RequestsPerDay > 0 {
			b.WriteString(fmt.Sprintf("  requests_per_day: %d\n", s.Frontmatter.RateLimits.RequestsPerDay))
		}
		if s.Frontmatter.RateLimits.BurstLimit > 0 {
			b.WriteString(fmt.Sprintf("  burst_limit: %d\n", s.Frontmatter.RateLimits.BurstLimit))
		}
		if s.Frontmatter.RateLimits.RetryAfterHeader != "" {
			b.WriteString(fmt.Sprintf("  retry_after_header: %q\n", s.Frontmatter.RateLimits.RetryAfterHeader))
		}
	}
	// Protocol-specific fields
	if s.Frontmatter.Protocol != "" {
		b.WriteString(fmt.Sprintf("protocol: %q\n", s.Frontmatter.Protocol))
	}
	if s.Frontmatter.ChannelCount > 0 {
		b.WriteString(fmt.Sprintf("channel_count: %d\n", s.Frontmatter.ChannelCount))
	}
	if s.Frontmatter.ServiceCount > 0 {
		b.WriteString(fmt.Sprintf("service_count: %d\n", s.Frontmatter.ServiceCount))
	}
	if s.Frontmatter.MessageCount > 0 {
		b.WriteString(fmt.Sprintf("message_count: %d\n", s.Frontmatter.MessageCount))
	}
	if len(s.Frontmatter.Servers) > 0 {
		b.WriteString("servers:\n")
		for _, server := range s.Frontmatter.Servers {
			b.WriteString(fmt.Sprintf("  - %q\n", server))
		}
	}
	b.WriteString("---\n\n")

	// Write sections
	for _, section := range s.Sections {
		b.WriteString(strings.Repeat("#", section.Level))
		b.WriteString(" ")
		b.WriteString(section.Title)
		b.WriteString("\n\n")
		if section.Content != "" {
			b.WriteString(section.Content)
			b.WriteString("\n\n")
		}
	}

	return strings.TrimSuffix(b.String(), "\n")
}

// RenderMinimal generates a minimal SKILL.md without frontmatter.
func RenderMinimal(s *Skill) string {
	var b strings.Builder

	b.WriteString("# ")
	b.WriteString(s.Frontmatter.Name)
	b.WriteString("\n\n")

	if s.Frontmatter.Description != "" {
		b.WriteString(s.Frontmatter.Description)
		b.WriteString("\n\n")
	}

	for _, section := range s.Sections {
		b.WriteString(strings.Repeat("#", section.Level))
		b.WriteString(" ")
		b.WriteString(section.Title)
		b.WriteString("\n\n")
		if section.Content != "" {
			b.WriteString(section.Content)
			b.WriteString("\n\n")
		}
	}

	return strings.TrimSuffix(b.String(), "\n")
}
