package validation

import (
	"github.com/sanixdarker/skillf/pkg/skill"
)

func (v *Validator) validateMCP(s *skill.Skill, result *ValidationResult) {
	fm := s.Frontmatter

	// Only validate MCP fields if mcp_compatible is set or tools are defined
	if !fm.MCPCompatible && len(fm.ToolDefinitions) == 0 {
		return
	}

	// If mcp_compatible is true, validate tool definitions
	if fm.MCPCompatible {
		if len(fm.ToolDefinitions) == 0 {
			v.addIssue(result, ValidationIssue{
				Code:       CodeMCPNoTools,
				Message:    "MCP compatible skill should define at least one tool",
				Severity:   SeverityWarning,
				Field:      "frontmatter.tools",
				Suggestion: "Add tool definitions with name, description, and parameters",
			})
		}

		// Check for rate limits (recommended for MCP)
		if fm.RateLimits == nil {
			v.addIssue(result, ValidationIssue{
				Code:       CodeMCPNoRateLimits,
				Message:    "MCP compatible skill should document rate limits",
				Severity:   SeverityInfo,
				Field:      "frontmatter.rate_limits",
				Suggestion: "Add rate_limits with requests_per_minute or similar fields",
			})
		}

		// Check for retry strategy (recommended for MCP)
		if fm.RetryStrategy == nil {
			v.addIssue(result, ValidationIssue{
				Code:       CodeMCPNoRetryStrategy,
				Message:    "MCP compatible skill should document retry strategy",
				Severity:   SeverityInfo,
				Field:      "frontmatter.retry_strategy",
				Suggestion: "Add retry_strategy with max_retries and backoff_type",
			})
		}
	}

	// Validate each tool definition
	for i, tool := range fm.ToolDefinitions {
		v.validateToolDefinition(tool, i, result)
	}
}

func (v *Validator) validateToolDefinition(tool skill.ToolDefinition, index int, result *ValidationResult) {
	field := "frontmatter.tools[" + itoa(index) + "]"

	// Required: tool name
	if tool.Name == "" {
		v.addIssue(result, ValidationIssue{
			Code:       CodeMCPMissingToolName,
			Message:    "Tool definition missing required name field",
			Severity:   SeverityError,
			Field:      field + ".name",
			Suggestion: "Add a unique, descriptive name for the tool",
		})
	}

	// Required: tool description
	if tool.Description == "" {
		v.addIssue(result, ValidationIssue{
			Code:       CodeMCPMissingToolDesc,
			Message:    "Tool '" + tool.Name + "' missing required description",
			Severity:   SeverityError,
			Field:      field + ".description",
			Suggestion: "Add a description explaining what the tool does",
		})
	}

	// Validate parameters if present
	if len(tool.Parameters) > 0 {
		v.validateToolParameters(tool, index, result)
	}
}

func (v *Validator) validateToolParameters(tool skill.ToolDefinition, index int, result *ValidationResult) {
	field := "frontmatter.tools[" + itoa(index) + "].parameters"

	// Check for JSON Schema structure
	typeField, hasType := tool.Parameters["type"]
	if !hasType {
		v.addIssue(result, ValidationIssue{
			Code:       CodeMCPInvalidToolSchema,
			Message:    "Tool '" + tool.Name + "' parameters should have 'type' field",
			Severity:   SeverityWarning,
			Field:      field,
			Suggestion: "Add 'type: object' to parameters for JSON Schema compliance",
		})
		return
	}

	if typeStr, ok := typeField.(string); ok && typeStr == "object" {
		// Check for properties in object type
		_, hasProps := tool.Parameters["properties"]
		if !hasProps {
			v.addIssue(result, ValidationIssue{
				Code:       CodeMCPInvalidToolSchema,
				Message:    "Tool '" + tool.Name + "' object parameters should have 'properties'",
				Severity:   SeverityWarning,
				Field:      field,
				Suggestion: "Add 'properties' field to define parameter schema",
			})
		}
	}
}

func (v *Validator) validateQuality(s *skill.Skill, result *ValidationResult) {
	fm := s.Frontmatter

	// Check for base_url when auth methods are specified
	if len(fm.AuthMethods) > 0 && fm.BaseURL == "" {
		v.addIssue(result, ValidationIssue{
			Code:       CodeMissingBaseURL,
			Message:    "Authentication methods specified but no base_url provided",
			Severity:   SeverityWarning,
			Field:      "frontmatter.base_url",
			Suggestion: "Add base_url to help AI agents construct API requests",
		})
	}

	// Check endpoint count consistency
	if fm.EndpointCount > 0 {
		// Could validate against actual sections, but that requires deeper analysis
		// For now, just ensure it's documented when auth is present
		if len(fm.AuthMethods) == 0 && fm.Protocol != "grpc" {
			v.addIssue(result, ValidationIssue{
				Code:       CodeInconsistentAuth,
				Message:    "Endpoints defined but no authentication methods documented",
				Severity:   SeverityInfo,
				Field:      "frontmatter.auth_methods",
				Suggestion: "Document authentication methods (api_key, oauth2, bearer, etc.)",
			})
		}
	}
}
