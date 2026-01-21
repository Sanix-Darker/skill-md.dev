package converter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v2 "github.com/pb33f/libopenapi/datamodel/high/v2"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/sanixdarker/skill-md/pkg/skill"
)

// OpenAPIConverter converts OpenAPI 3.x specs to SKILL.md.
type OpenAPIConverter struct{}

func (c *OpenAPIConverter) Name() string {
	return "openapi"
}

func (c *OpenAPIConverter) CanHandle(filename string, content []byte) bool {
	ext := getExtension(filename)
	if ext != ".yaml" && ext != ".yml" && ext != ".json" {
		return false
	}
	// Check for OpenAPI indicators
	return bytes.Contains(content, []byte("openapi:")) ||
		bytes.Contains(content, []byte(`"openapi":`)) ||
		bytes.Contains(content, []byte("swagger:")) ||
		bytes.Contains(content, []byte(`"swagger":`))
}

func (c *OpenAPIConverter) Convert(content []byte, opts *Options) (*skill.Skill, error) {
	// Strip BOM (Byte Order Mark) if present
	content = stripBOM(content)

	// Try to detect if it's OpenAPI 2.x (Swagger) or 3.x
	isSwagger := bytes.Contains(content, []byte("swagger:")) || bytes.Contains(content, []byte(`"swagger":`))

	doc, err := libopenapi.NewDocument(content)
	if err != nil {
		// Provide more helpful error message
		if bytes.Contains(content, []byte("openapi:")) || bytes.Contains(content, []byte(`"openapi":`)) {
			return nil, fmt.Errorf("failed to parse OpenAPI 3.x document: %w. Ensure the file is a valid OpenAPI 3.x specification", err)
		}
		if isSwagger {
			return nil, fmt.Errorf("failed to parse Swagger 2.x document: %w. Ensure the file is a valid Swagger 2.x specification", err)
		}
		return nil, fmt.Errorf("failed to parse OpenAPI document: %w", err)
	}

	// Check document version and build appropriate model
	version := doc.GetVersion()

	// Handle OpenAPI 3.x
	if strings.HasPrefix(version, "3.") {
		model, err := doc.BuildV3Model()
		if err != nil {
			return nil, fmt.Errorf("failed to build OpenAPI 3.x model: %w", err)
		}
		return c.buildSkill(model, opts), nil
	}

	// Handle Swagger 2.x (OpenAPI 2.0)
	if strings.HasPrefix(version, "2.") {
		model, err := doc.BuildV2Model()
		if err != nil {
			return nil, fmt.Errorf("failed to build Swagger 2.x model: %w", err)
		}
		return c.buildSkillFromV2(model, opts), nil
	}

	return nil, fmt.Errorf("unsupported OpenAPI version: %s. Supported versions are 2.x (Swagger) and 3.x", version)
}

// stripBOM removes UTF-8 BOM if present at the beginning of content.
func stripBOM(content []byte) []byte {
	if len(content) >= 3 && content[0] == 0xEF && content[1] == 0xBB && content[2] == 0xBF {
		return content[3:]
	}
	return content
}

func (c *OpenAPIConverter) buildSkill(model *libopenapi.DocumentModel[v3.Document], opts *Options) *skill.Skill {
	doc := model.Model

	name := "API Skill"
	if opts != nil && opts.Name != "" {
		name = opts.Name
	} else if doc.Info != nil && doc.Info.Title != "" {
		name = doc.Info.Title
	}

	description := ""
	if doc.Info != nil && doc.Info.Description != "" {
		description = doc.Info.Description
	}

	s := skill.NewSkill(name, description)
	s.Frontmatter.SourceType = "openapi"
	if opts != nil && opts.SourcePath != "" {
		s.Frontmatter.Source = opts.SourcePath
	}

	// Add version info
	if doc.Info != nil && doc.Info.Version != "" {
		s.Frontmatter.Version = doc.Info.Version
	}

	// Extract tags as skill tags
	if doc.Tags != nil {
		for _, tag := range doc.Tags {
			s.Frontmatter.Tags = append(s.Frontmatter.Tags, tag.Name)
		}
	}

	// Count endpoints for metadata
	endpointCount := c.countEndpoints(&doc)
	s.Frontmatter.EndpointCount = endpointCount

	// Extract auth methods for metadata
	authMethods := c.extractAuthMethods(&doc)
	s.Frontmatter.AuthMethods = authMethods

	// Set base URL if available
	if len(doc.Servers) > 0 {
		s.Frontmatter.BaseURL = doc.Servers[0].URL
	}

	// Determine difficulty based on complexity
	s.Frontmatter.Difficulty = c.determineDifficulty(&doc, endpointCount)

	// Check if spec has examples
	s.Frontmatter.HasExamples = c.hasExamples(&doc)

	// Add Quick Start section (NEW)
	s.AddSection("Quick Start", 2, c.buildQuickStart(&doc))

	// Add overview section
	s.AddSection("Overview", 2, c.buildOverview(&doc))

	// Add authentication section if security schemes exist
	if doc.Components != nil && doc.Components.SecuritySchemes != nil && doc.Components.SecuritySchemes.Len() > 0 {
		s.AddSection("Authentication", 2, c.buildAuthSection(&doc))
	}

	// Add endpoints section
	s.AddSection("Endpoints", 2, c.buildEndpointsSection(&doc))

	// Add schemas section if components exist
	if doc.Components != nil && doc.Components.Schemas != nil && doc.Components.Schemas.Len() > 0 {
		s.AddSection("Data Models", 2, c.buildSchemasSection(&doc))
	}

	// Add Error Handling section (NEW)
	s.AddSection("Error Handling", 2, c.buildErrorHandlingSection(&doc))

	// Add Rate Limiting section if detected (NEW)
	if rateLimiting := c.buildRateLimitingSection(&doc); rateLimiting != "" {
		s.AddSection("Rate Limiting", 2, rateLimiting)
	}

	// Add Best Practices section (NEW)
	s.AddSection("Best Practices", 2, c.buildBestPracticesSection(&doc))

	return s
}

func (c *OpenAPIConverter) countEndpoints(doc *v3.Document) int {
	count := 0
	if doc.Paths == nil {
		return 0
	}
	for pair := doc.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
		item := pair.Value()
		if item.Get != nil {
			count++
		}
		if item.Post != nil {
			count++
		}
		if item.Put != nil {
			count++
		}
		if item.Delete != nil {
			count++
		}
		if item.Patch != nil {
			count++
		}
		if item.Head != nil {
			count++
		}
		if item.Options != nil {
			count++
		}
	}
	return count
}

func (c *OpenAPIConverter) extractAuthMethods(doc *v3.Document) []string {
	var methods []string
	if doc.Components == nil || doc.Components.SecuritySchemes == nil {
		return methods
	}
	for pair := doc.Components.SecuritySchemes.First(); pair != nil; pair = pair.Next() {
		scheme := pair.Value()
		methods = append(methods, scheme.Type)
	}
	return methods
}

func (c *OpenAPIConverter) determineDifficulty(doc *v3.Document, endpointCount int) string {
	hasAuth := doc.Components != nil && doc.Components.SecuritySchemes != nil && doc.Components.SecuritySchemes.Len() > 0
	schemaCount := 0
	if doc.Components != nil && doc.Components.Schemas != nil {
		schemaCount = doc.Components.Schemas.Len()
	}

	if endpointCount <= 5 && schemaCount <= 5 && !hasAuth {
		return "novice"
	} else if endpointCount <= 20 && schemaCount <= 20 {
		return "intermediate"
	}
	return "advanced"
}

func (c *OpenAPIConverter) hasExamples(doc *v3.Document) bool {
	if doc.Components != nil && doc.Components.Schemas != nil {
		for pair := doc.Components.Schemas.First(); pair != nil; pair = pair.Next() {
			schema := pair.Value().Schema()
			if schema != nil && schema.Example != nil {
				return true
			}
		}
	}
	return false
}

func (c *OpenAPIConverter) buildQuickStart(doc *v3.Document) string {
	var b strings.Builder

	b.WriteString("Get started with this API in minutes.\n\n")

	// Step 1: Base URL
	b.WriteString("### 1. Set Your Base URL\n\n")
	if len(doc.Servers) > 0 {
		b.WriteString(fmt.Sprintf("```\n%s\n```\n\n", doc.Servers[0].URL))
	} else {
		b.WriteString("```\nhttps://api.example.com\n```\n\n")
	}

	// Step 2: Authentication
	if doc.Components != nil && doc.Components.SecuritySchemes != nil && doc.Components.SecuritySchemes.Len() > 0 {
		b.WriteString("### 2. Authenticate\n\n")
		for pair := doc.Components.SecuritySchemes.First(); pair != nil; pair = pair.Next() {
			scheme := pair.Value()
			switch scheme.Type {
			case "http":
				if scheme.Scheme == "bearer" {
					b.WriteString("Add your Bearer token to all requests:\n\n")
					b.WriteString("```\nAuthorization: Bearer YOUR_TOKEN\n```\n\n")
				} else if scheme.Scheme == "basic" {
					b.WriteString("Use HTTP Basic authentication:\n\n")
					b.WriteString("```\nAuthorization: Basic BASE64_ENCODED_CREDENTIALS\n```\n\n")
				}
			case "apiKey":
				b.WriteString(fmt.Sprintf("Add your API key to the `%s` %s:\n\n", scheme.Name, scheme.In))
				if scheme.In == "header" {
					b.WriteString(fmt.Sprintf("```\n%s: YOUR_API_KEY\n```\n\n", scheme.Name))
				} else if scheme.In == "query" {
					b.WriteString(fmt.Sprintf("```\n?%s=YOUR_API_KEY\n```\n\n", scheme.Name))
				}
			case "oauth2":
				b.WriteString("Configure OAuth 2.0 authentication. See the Authentication section for flow details.\n\n")
			}
			break // Just show the first auth method for quick start
		}
	}

	// Step 3: Make your first request
	b.WriteString("### 3. Make Your First Request\n\n")
	if doc.Paths != nil {
		// Find a simple GET endpoint to demonstrate
		for pair := doc.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
			path := pair.Key()
			item := pair.Value()
			if item.Get != nil {
				baseURL := "https://api.example.com"
				if len(doc.Servers) > 0 {
					baseURL = strings.TrimSuffix(doc.Servers[0].URL, "/")
				}
				b.WriteString("**cURL:**\n")
				b.WriteString("```bash\n")
				b.WriteString(fmt.Sprintf("curl -X GET \"%s%s\" \\\n", baseURL, path))
				b.WriteString("  -H \"Accept: application/json\"\n")
				b.WriteString("```\n\n")

				b.WriteString("**JavaScript (fetch):**\n")
				b.WriteString("```javascript\n")
				b.WriteString(fmt.Sprintf("const response = await fetch('%s%s', {\n", baseURL, path))
				b.WriteString("  method: 'GET',\n")
				b.WriteString("  headers: { 'Accept': 'application/json' }\n")
				b.WriteString("});\n")
				b.WriteString("const data = await response.json();\n")
				b.WriteString("```\n\n")

				b.WriteString("**Python (requests):**\n")
				b.WriteString("```python\n")
				b.WriteString("import requests\n\n")
				b.WriteString(fmt.Sprintf("response = requests.get('%s%s')\n", baseURL, path))
				b.WriteString("data = response.json()\n")
				b.WriteString("```\n")
				break
			}
		}
	}

	return strings.TrimSpace(b.String())
}

func (c *OpenAPIConverter) buildOverview(doc *v3.Document) string {
	var b strings.Builder

	if doc.Info != nil {
		if doc.Info.Description != "" {
			b.WriteString(doc.Info.Description)
			b.WriteString("\n\n")
		}

		if doc.Info.Contact != nil {
			b.WriteString("**Contact**: ")
			if doc.Info.Contact.Name != "" {
				b.WriteString(doc.Info.Contact.Name)
			}
			if doc.Info.Contact.Email != "" {
				b.WriteString(fmt.Sprintf(" <%s>", doc.Info.Contact.Email))
			}
			if doc.Info.Contact.URL != "" {
				b.WriteString(fmt.Sprintf(" ([website](%s))", doc.Info.Contact.URL))
			}
			b.WriteString("\n")
		}

		if doc.Info.License != nil {
			b.WriteString(fmt.Sprintf("**License**: %s", doc.Info.License.Name))
			if doc.Info.License.URL != "" {
				b.WriteString(fmt.Sprintf(" ([details](%s))", doc.Info.License.URL))
			}
			b.WriteString("\n")
		}

		if doc.Info.TermsOfService != "" {
			b.WriteString(fmt.Sprintf("**Terms of Service**: %s\n", doc.Info.TermsOfService))
		}
	}

	// Add servers
	if len(doc.Servers) > 0 {
		b.WriteString("\n**Base URLs**:\n")
		for _, server := range doc.Servers {
			b.WriteString(fmt.Sprintf("- `%s`", server.URL))
			if server.Description != "" {
				b.WriteString(fmt.Sprintf(" - %s", server.Description))
			}
			b.WriteString("\n")
		}
	}

	// Add external docs if available
	if doc.ExternalDocs != nil && doc.ExternalDocs.URL != "" {
		b.WriteString(fmt.Sprintf("\n**External Documentation**: [%s](%s)\n",
			doc.ExternalDocs.Description, doc.ExternalDocs.URL))
	}

	return strings.TrimSpace(b.String())
}

func (c *OpenAPIConverter) buildAuthSection(doc *v3.Document) string {
	var b strings.Builder

	for pair := doc.Components.SecuritySchemes.First(); pair != nil; pair = pair.Next() {
		name := pair.Key()
		scheme := pair.Value()
		b.WriteString(fmt.Sprintf("### %s\n\n", name))
		b.WriteString(fmt.Sprintf("**Type**: `%s`\n", scheme.Type))

		if scheme.Scheme != "" {
			b.WriteString(fmt.Sprintf("**Scheme**: `%s`\n", scheme.Scheme))
		}
		if scheme.BearerFormat != "" {
			b.WriteString(fmt.Sprintf("**Bearer Format**: `%s`\n", scheme.BearerFormat))
		}
		if scheme.In != "" {
			b.WriteString(fmt.Sprintf("**In**: `%s`\n", scheme.In))
		}
		if scheme.Name != "" {
			b.WriteString(fmt.Sprintf("**Parameter Name**: `%s`\n", scheme.Name))
		}
		if scheme.Description != "" {
			b.WriteString(fmt.Sprintf("\n%s\n", scheme.Description))
		}

		// Add code examples for each auth type
		b.WriteString("\n**Example Usage**:\n\n")
		switch scheme.Type {
		case "http":
			if scheme.Scheme == "bearer" {
				b.WriteString("```bash\n")
				b.WriteString("curl -H \"Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...\" \\\n")
				b.WriteString("     https://api.example.com/resource\n")
				b.WriteString("```\n")
			} else if scheme.Scheme == "basic" {
				b.WriteString("```bash\n")
				b.WriteString("curl -u username:password https://api.example.com/resource\n")
				b.WriteString("```\n")
			}
		case "apiKey":
			if scheme.In == "header" {
				b.WriteString("```bash\n")
				b.WriteString(fmt.Sprintf("curl -H \"%s: your-api-key\" https://api.example.com/resource\n", scheme.Name))
				b.WriteString("```\n")
			} else if scheme.In == "query" {
				b.WriteString("```bash\n")
				b.WriteString(fmt.Sprintf("curl \"https://api.example.com/resource?%s=your-api-key\"\n", scheme.Name))
				b.WriteString("```\n")
			}
		case "oauth2":
			b.WriteString("```javascript\n")
			b.WriteString("// OAuth 2.0 flow\n")
			b.WriteString("const token = await getOAuthToken(clientId, clientSecret);\n")
			b.WriteString("fetch('/resource', {\n")
			b.WriteString("  headers: { 'Authorization': `Bearer ${token}` }\n")
			b.WriteString("});\n")
			b.WriteString("```\n")
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *OpenAPIConverter) buildEndpointsSection(doc *v3.Document) string {
	var b strings.Builder

	if doc.Paths == nil {
		return "No endpoints defined."
	}

	// Collect and sort paths
	var paths []string
	for pair := doc.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
		paths = append(paths, pair.Key())
	}
	sort.Strings(paths)

	for _, path := range paths {
		pathItem, _ := doc.Paths.PathItems.Get(path)
		b.WriteString(c.buildPathSection(path, pathItem, doc))
	}

	return strings.TrimSpace(b.String())
}

func (c *OpenAPIConverter) buildPathSection(path string, item *v3.PathItem, doc *v3.Document) string {
	var b strings.Builder

	operations := []struct {
		method string
		op     *v3.Operation
	}{
		{"GET", item.Get},
		{"POST", item.Post},
		{"PUT", item.Put},
		{"DELETE", item.Delete},
		{"PATCH", item.Patch},
		{"HEAD", item.Head},
		{"OPTIONS", item.Options},
	}

	for _, entry := range operations {
		if entry.op == nil {
			continue
		}

		method := entry.method
		op := entry.op

		b.WriteString(fmt.Sprintf("### %s %s\n\n", method, path))

		// Tags
		if len(op.Tags) > 0 {
			b.WriteString(fmt.Sprintf("**Tags**: %s\n\n", strings.Join(op.Tags, ", ")))
		}

		if op.Summary != "" {
			b.WriteString(fmt.Sprintf("**%s**\n\n", op.Summary))
		}

		if op.Description != "" {
			b.WriteString(op.Description)
			b.WriteString("\n\n")
		}

		// Deprecated warning
		if op.Deprecated != nil && *op.Deprecated {
			b.WriteString("> ⚠️ **Deprecated**: This endpoint is deprecated and may be removed in future versions.\n\n")
		}

		// Parameters
		if len(op.Parameters) > 0 {
			b.WriteString("**Parameters**:\n\n")
			b.WriteString("| Name | In | Type | Required | Description |\n")
			b.WriteString("|------|-----|------|----------|-------------|\n")
			for _, param := range op.Parameters {
				required := "No"
				if param.Required != nil && *param.Required {
					required = "Yes"
				}
				paramType := "string"
				defaultVal := ""
				if param.Schema != nil && param.Schema.Schema() != nil {
					schema := param.Schema.Schema()
					if len(schema.Type) > 0 {
						paramType = schema.Type[0]
					}
					if schema.Default != nil {
						defaultVal = fmt.Sprintf(" (default: `%v`)", schema.Default)
					}
				}
				desc := strings.ReplaceAll(param.Description, "\n", " ") + defaultVal
				b.WriteString(fmt.Sprintf("| `%s` | %s | `%s` | %s | %s |\n",
					param.Name, param.In, paramType, required, desc))
			}
			b.WriteString("\n")
		}

		// Request body with example
		if op.RequestBody != nil && op.RequestBody.Content != nil {
			b.WriteString("**Request Body**:\n\n")
			required := ""
			if op.RequestBody.Required != nil && *op.RequestBody.Required {
				required = " (required)"
			}
			if op.RequestBody.Description != "" {
				b.WriteString(fmt.Sprintf("%s%s\n\n", op.RequestBody.Description, required))
			}
			for pair := op.RequestBody.Content.First(); pair != nil; pair = pair.Next() {
				contentType := pair.Key()
				mediaType := pair.Value()
				b.WriteString(fmt.Sprintf("Content-Type: `%s`\n\n", contentType))

				// Add example if available
				if mediaType.Example != nil {
					b.WriteString("Example:\n")
					b.WriteString("```json\n")
					exampleJSON, _ := json.MarshalIndent(mediaType.Example, "", "  ")
					b.WriteString(string(exampleJSON))
					b.WriteString("\n```\n\n")
				} else if mediaType.Schema != nil && mediaType.Schema.Schema() != nil {
					// Generate example from schema
					example := c.generateSchemaExample(mediaType.Schema.Schema())
					if example != "" {
						b.WriteString("Example:\n")
						b.WriteString("```json\n")
						b.WriteString(example)
						b.WriteString("\n```\n\n")
					}
				}
			}
		}

		// Responses with examples
		if op.Responses != nil && op.Responses.Codes != nil {
			b.WriteString("**Responses**:\n\n")
			for pair := op.Responses.Codes.First(); pair != nil; pair = pair.Next() {
				code := pair.Key()
				resp := pair.Value()
				desc := ""
				if resp.Description != "" {
					desc = strings.ReplaceAll(resp.Description, "\n", " ")
				}
				b.WriteString(fmt.Sprintf("#### %s - %s\n\n", code, desc))

				if resp.Content != nil {
					for contentPair := resp.Content.First(); contentPair != nil; contentPair = contentPair.Next() {
						mediaType := contentPair.Value()
						if mediaType.Example != nil {
							b.WriteString("```json\n")
							exampleJSON, _ := json.MarshalIndent(mediaType.Example, "", "  ")
							b.WriteString(string(exampleJSON))
							b.WriteString("\n```\n\n")
						}
					}
				}
			}
		}

		// Code examples
		b.WriteString(c.buildCodeExamples(method, path, op, doc))
	}

	return b.String()
}

func (c *OpenAPIConverter) buildCodeExamples(method, path string, op *v3.Operation, doc *v3.Document) string {
	var b strings.Builder

	baseURL := "https://api.example.com"
	if len(doc.Servers) > 0 {
		baseURL = strings.TrimSuffix(doc.Servers[0].URL, "/")
	}

	b.WriteString("**Code Examples**:\n\n")

	// cURL example
	b.WriteString("<details>\n<summary>cURL</summary>\n\n")
	b.WriteString("```bash\n")
	b.WriteString(fmt.Sprintf("curl -X %s \"%s%s\" \\\n", method, baseURL, path))
	b.WriteString("  -H \"Accept: application/json\"")
	if method == "POST" || method == "PUT" || method == "PATCH" {
		b.WriteString(" \\\n  -H \"Content-Type: application/json\"")
		b.WriteString(" \\\n  -d '{\"key\": \"value\"}'")
	}
	b.WriteString("\n```\n\n")
	b.WriteString("</details>\n\n")

	// JavaScript example
	b.WriteString("<details>\n<summary>JavaScript</summary>\n\n")
	b.WriteString("```javascript\n")
	if method == "GET" || method == "DELETE" {
		b.WriteString(fmt.Sprintf("const response = await fetch('%s%s', {\n", baseURL, path))
		b.WriteString(fmt.Sprintf("  method: '%s',\n", method))
		b.WriteString("  headers: {\n")
		b.WriteString("    'Accept': 'application/json'\n")
		b.WriteString("  }\n")
		b.WriteString("});\n")
	} else {
		b.WriteString(fmt.Sprintf("const response = await fetch('%s%s', {\n", baseURL, path))
		b.WriteString(fmt.Sprintf("  method: '%s',\n", method))
		b.WriteString("  headers: {\n")
		b.WriteString("    'Accept': 'application/json',\n")
		b.WriteString("    'Content-Type': 'application/json'\n")
		b.WriteString("  },\n")
		b.WriteString("  body: JSON.stringify({ key: 'value' })\n")
		b.WriteString("});\n")
	}
	b.WriteString("const data = await response.json();\n")
	b.WriteString("```\n\n")
	b.WriteString("</details>\n\n")

	// Python example
	b.WriteString("<details>\n<summary>Python</summary>\n\n")
	b.WriteString("```python\n")
	b.WriteString("import requests\n\n")
	if method == "GET" {
		b.WriteString(fmt.Sprintf("response = requests.get('%s%s')\n", baseURL, path))
	} else if method == "DELETE" {
		b.WriteString(fmt.Sprintf("response = requests.delete('%s%s')\n", baseURL, path))
	} else {
		b.WriteString(fmt.Sprintf("response = requests.%s(\n", strings.ToLower(method)))
		b.WriteString(fmt.Sprintf("    '%s%s',\n", baseURL, path))
		b.WriteString("    json={'key': 'value'}\n")
		b.WriteString(")\n")
	}
	b.WriteString("data = response.json()\n")
	b.WriteString("```\n\n")
	b.WriteString("</details>\n\n")

	return b.String()
}

func (c *OpenAPIConverter) generateSchemaExample(schema *base.Schema) string {
	if schema == nil {
		return ""
	}

	example := make(map[string]interface{})

	if schema.Properties != nil {
		for pair := schema.Properties.First(); pair != nil; pair = pair.Next() {
			propName := pair.Key()
			prop := pair.Value().Schema()
			if prop != nil {
				if prop.Example != nil {
					example[propName] = prop.Example
				} else if len(prop.Type) > 0 {
					switch prop.Type[0] {
					case "string":
						if prop.Format == "date-time" {
							example[propName] = "2024-01-15T10:30:00Z"
						} else if prop.Format == "email" {
							example[propName] = "user@example.com"
						} else if prop.Format == "uri" {
							example[propName] = "https://example.com"
						} else {
							example[propName] = "string"
						}
					case "integer":
						example[propName] = 0
					case "number":
						example[propName] = 0.0
					case "boolean":
						example[propName] = false
					case "array":
						example[propName] = []interface{}{}
					case "object":
						example[propName] = map[string]interface{}{}
					}
				}
			}
		}
	}

	if len(example) == 0 {
		return ""
	}

	jsonBytes, err := json.MarshalIndent(example, "", "  ")
	if err != nil {
		return ""
	}
	return string(jsonBytes)
}

func (c *OpenAPIConverter) buildSchemasSection(doc *v3.Document) string {
	var b strings.Builder

	// Collect and sort schema names
	var schemaNames []string
	for pair := doc.Components.Schemas.First(); pair != nil; pair = pair.Next() {
		schemaNames = append(schemaNames, pair.Key())
	}
	sort.Strings(schemaNames)

	for _, name := range schemaNames {
		schemaProxy, _ := doc.Components.Schemas.Get(name)
		schema := schemaProxy.Schema()

		b.WriteString(fmt.Sprintf("### %s\n\n", name))

		if schema.Description != "" {
			b.WriteString(schema.Description)
			b.WriteString("\n\n")
		}

		// Show type
		if len(schema.Type) > 0 {
			b.WriteString(fmt.Sprintf("**Type**: `%s`\n\n", schema.Type[0]))
		}

		// List properties
		if schema.Properties != nil && schema.Properties.Len() > 0 {
			b.WriteString("| Property | Type | Required | Description |\n")
			b.WriteString("|----------|------|----------|-------------|\n")

			requiredSet := make(map[string]bool)
			for _, r := range schema.Required {
				requiredSet[r] = true
			}

			for pair := schema.Properties.First(); pair != nil; pair = pair.Next() {
				propName := pair.Key()
				prop := pair.Value().Schema()

				propType := "any"
				if len(prop.Type) > 0 {
					propType = prop.Type[0]
					if prop.Format != "" {
						propType = fmt.Sprintf("%s (%s)", propType, prop.Format)
					}
				}

				required := "No"
				if requiredSet[propName] {
					required = "Yes"
				}

				desc := strings.ReplaceAll(prop.Description, "\n", " ")

				// Add constraints
				constraints := []string{}
				if prop.MinLength != nil {
					constraints = append(constraints, fmt.Sprintf("minLength: %d", *prop.MinLength))
				}
				if prop.MaxLength != nil {
					constraints = append(constraints, fmt.Sprintf("maxLength: %d", *prop.MaxLength))
				}
				if prop.Minimum != nil {
					constraints = append(constraints, fmt.Sprintf("min: %v", *prop.Minimum))
				}
				if prop.Maximum != nil {
					constraints = append(constraints, fmt.Sprintf("max: %v", *prop.Maximum))
				}
				if prop.Pattern != "" {
					constraints = append(constraints, fmt.Sprintf("pattern: `%s`", prop.Pattern))
				}
				if len(prop.Enum) > 0 {
					enumVals := make([]string, 0, len(prop.Enum))
					for _, e := range prop.Enum {
						enumVals = append(enumVals, fmt.Sprintf("`%v`", e.Value))
					}
					constraints = append(constraints, fmt.Sprintf("enum: %s", strings.Join(enumVals, ", ")))
				}

				if len(constraints) > 0 {
					if desc != "" {
						desc += " "
					}
					desc += fmt.Sprintf("(%s)", strings.Join(constraints, ", "))
				}

				b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s | %s |\n", propName, propType, required, desc))
			}
			b.WriteString("\n")
		}

		// Add example if available
		if schema.Example != nil {
			b.WriteString("**Example**:\n\n")
			b.WriteString("```json\n")
			exampleJSON, _ := json.MarshalIndent(schema.Example, "", "  ")
			b.WriteString(string(exampleJSON))
			b.WriteString("\n```\n\n")
		}
	}

	return strings.TrimSpace(b.String())
}

func (c *OpenAPIConverter) buildErrorHandlingSection(doc *v3.Document) string {
	var b strings.Builder

	b.WriteString("This section documents common error responses and how to handle them.\n\n")

	// Collect error codes from all operations
	errorCodes := make(map[string]string)
	if doc.Paths != nil {
		for pair := doc.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
			item := pair.Value()
			ops := []*v3.Operation{item.Get, item.Post, item.Put, item.Delete, item.Patch}
			for _, op := range ops {
				if op != nil && op.Responses != nil && op.Responses.Codes != nil {
					for respPair := op.Responses.Codes.First(); respPair != nil; respPair = respPair.Next() {
						code := respPair.Key()
						// Only collect 4xx and 5xx codes
						if strings.HasPrefix(code, "4") || strings.HasPrefix(code, "5") {
							resp := respPair.Value()
							if resp.Description != "" {
								errorCodes[code] = resp.Description
							}
						}
					}
				}
			}
		}
	}

	if len(errorCodes) > 0 {
		b.WriteString("### Error Codes\n\n")
		b.WriteString("| Code | Description | Recommended Action |\n")
		b.WriteString("|------|-------------|-------------------|\n")

		// Sort codes
		codes := make([]string, 0, len(errorCodes))
		for code := range errorCodes {
			codes = append(codes, code)
		}
		sort.Strings(codes)

		for _, code := range codes {
			action := c.getErrorAction(code)
			desc := strings.ReplaceAll(errorCodes[code], "\n", " ")
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", code, desc, action))
		}
		b.WriteString("\n")
	}

	// Standard error handling advice
	b.WriteString("### Error Response Format\n\n")
	b.WriteString("Typical error responses follow this structure:\n\n")
	b.WriteString("```json\n")
	b.WriteString("{\n")
	b.WriteString("  \"error\": {\n")
	b.WriteString("    \"code\": \"ERROR_CODE\",\n")
	b.WriteString("    \"message\": \"Human-readable error description\",\n")
	b.WriteString("    \"details\": {}\n")
	b.WriteString("  }\n")
	b.WriteString("}\n")
	b.WriteString("```\n\n")

	b.WriteString("### Handling Errors\n\n")
	b.WriteString("```javascript\n")
	b.WriteString("try {\n")
	b.WriteString("  const response = await fetch(url);\n")
	b.WriteString("  if (!response.ok) {\n")
	b.WriteString("    const error = await response.json();\n")
	b.WriteString("    switch (response.status) {\n")
	b.WriteString("      case 400: throw new ValidationError(error.message);\n")
	b.WriteString("      case 401: throw new AuthError('Please re-authenticate');\n")
	b.WriteString("      case 403: throw new ForbiddenError('Access denied');\n")
	b.WriteString("      case 404: throw new NotFoundError('Resource not found');\n")
	b.WriteString("      case 429: await delay(error.retryAfter); return retry();\n")
	b.WriteString("      default: throw new APIError(error.message);\n")
	b.WriteString("    }\n")
	b.WriteString("  }\n")
	b.WriteString("  return response.json();\n")
	b.WriteString("} catch (error) {\n")
	b.WriteString("  console.error('API Error:', error);\n")
	b.WriteString("  throw error;\n")
	b.WriteString("}\n")
	b.WriteString("```\n")

	return strings.TrimSpace(b.String())
}

func (c *OpenAPIConverter) getErrorAction(code string) string {
	actions := map[string]string{
		"400": "Check request parameters and body format",
		"401": "Refresh or obtain new authentication credentials",
		"403": "Verify permissions for the requested resource",
		"404": "Verify the resource ID or path exists",
		"405": "Use the correct HTTP method for this endpoint",
		"409": "Resolve conflict, possibly retry with updated data",
		"422": "Fix validation errors in request body",
		"429": "Wait and retry with exponential backoff",
		"500": "Retry request; report if persistent",
		"502": "Retry request after a short delay",
		"503": "Service temporarily unavailable; retry later",
		"504": "Retry request; check for long-running operations",
	}
	if action, ok := actions[code]; ok {
		return action
	}
	return "Handle appropriately based on context"
}

func (c *OpenAPIConverter) buildRateLimitingSection(doc *v3.Document) string {
	// Check if rate limiting headers are documented
	hasRateLimit := false
	if doc.Paths != nil {
		for pair := doc.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
			item := pair.Value()
			ops := []*v3.Operation{item.Get, item.Post, item.Put, item.Delete, item.Patch}
			for _, op := range ops {
				if op != nil && op.Responses != nil && op.Responses.Codes != nil {
					for respPair := op.Responses.Codes.First(); respPair != nil; respPair = respPair.Next() {
						code := respPair.Key()
						if code == "429" {
							hasRateLimit = true
							break
						}
					}
				}
			}
		}
	}

	if !hasRateLimit {
		return ""
	}

	var b strings.Builder

	b.WriteString("This API implements rate limiting to ensure fair usage and service stability.\n\n")

	b.WriteString("### Rate Limit Headers\n\n")
	b.WriteString("Check these headers in API responses:\n\n")
	b.WriteString("| Header | Description |\n")
	b.WriteString("|--------|-------------|\n")
	b.WriteString("| `X-RateLimit-Limit` | Maximum requests allowed per window |\n")
	b.WriteString("| `X-RateLimit-Remaining` | Requests remaining in current window |\n")
	b.WriteString("| `X-RateLimit-Reset` | Unix timestamp when the window resets |\n")
	b.WriteString("| `Retry-After` | Seconds to wait before retrying (on 429) |\n\n")

	b.WriteString("### Handling Rate Limits\n\n")
	b.WriteString("```javascript\n")
	b.WriteString("async function fetchWithRateLimit(url, options = {}) {\n")
	b.WriteString("  const response = await fetch(url, options);\n")
	b.WriteString("  \n")
	b.WriteString("  // Check remaining requests\n")
	b.WriteString("  const remaining = response.headers.get('X-RateLimit-Remaining');\n")
	b.WriteString("  if (remaining && parseInt(remaining) < 10) {\n")
	b.WriteString("    console.warn('Approaching rate limit');\n")
	b.WriteString("  }\n")
	b.WriteString("  \n")
	b.WriteString("  if (response.status === 429) {\n")
	b.WriteString("    const retryAfter = response.headers.get('Retry-After') || 60;\n")
	b.WriteString("    await new Promise(r => setTimeout(r, retryAfter * 1000));\n")
	b.WriteString("    return fetchWithRateLimit(url, options); // Retry\n")
	b.WriteString("  }\n")
	b.WriteString("  \n")
	b.WriteString("  return response;\n")
	b.WriteString("}\n")
	b.WriteString("```\n")

	return strings.TrimSpace(b.String())
}

func (c *OpenAPIConverter) buildBestPracticesSection(doc *v3.Document) string {
	var b strings.Builder

	b.WriteString("Follow these recommendations for optimal API usage.\n\n")

	b.WriteString("### Authentication\n\n")
	b.WriteString("- Store credentials securely (environment variables, secret managers)\n")
	b.WriteString("- Never commit API keys to version control\n")
	b.WriteString("- Rotate credentials periodically\n")
	b.WriteString("- Use the minimum required permissions\n\n")

	b.WriteString("### Request Handling\n\n")
	b.WriteString("- Always set appropriate `Content-Type` and `Accept` headers\n")
	b.WriteString("- Validate input before sending requests\n")
	b.WriteString("- Use HTTPS for all requests\n")
	b.WriteString("- Implement request timeouts (recommended: 30 seconds)\n\n")

	b.WriteString("### Error Handling\n\n")
	b.WriteString("- Implement exponential backoff for retries\n")
	b.WriteString("- Log errors with request context for debugging\n")
	b.WriteString("- Handle network errors gracefully\n")
	b.WriteString("- Don't expose raw error messages to end users\n\n")

	b.WriteString("### Performance\n\n")
	b.WriteString("- Use pagination for list endpoints\n")
	b.WriteString("- Cache responses where appropriate\n")
	b.WriteString("- Batch requests when possible\n")
	b.WriteString("- Use compression (`Accept-Encoding: gzip`)\n\n")

	// Add specific tips based on the API
	if doc.Components != nil && doc.Components.SecuritySchemes != nil && doc.Components.SecuritySchemes.Len() > 0 {
		b.WriteString("### Security Considerations\n\n")
		for pair := doc.Components.SecuritySchemes.First(); pair != nil; pair = pair.Next() {
			scheme := pair.Value()
			switch scheme.Type {
			case "oauth2":
				b.WriteString("- Store OAuth tokens securely\n")
				b.WriteString("- Implement token refresh before expiration\n")
				b.WriteString("- Use PKCE for public clients\n")
			case "apiKey":
				b.WriteString("- Rotate API keys regularly\n")
				b.WriteString("- Use different keys for development and production\n")
			case "http":
				if scheme.Scheme == "bearer" {
					b.WriteString("- Validate JWT tokens on each request\n")
					b.WriteString("- Check token expiration proactively\n")
				}
			}
			break
		}
	}

	return strings.TrimSpace(b.String())
}

// buildSkillFromV2 builds a skill from a Swagger 2.x (OpenAPI 2.0) model.
func (c *OpenAPIConverter) buildSkillFromV2(model *libopenapi.DocumentModel[v2.Swagger], opts *Options) *skill.Skill {
	doc := model.Model

	name := "API Skill"
	if opts != nil && opts.Name != "" {
		name = opts.Name
	} else if doc.Info != nil && doc.Info.Title != "" {
		name = doc.Info.Title
	}

	description := ""
	if doc.Info != nil && doc.Info.Description != "" {
		description = doc.Info.Description
	}

	s := skill.NewSkill(name, description)
	s.Frontmatter.SourceType = "swagger"
	if opts != nil && opts.SourcePath != "" {
		s.Frontmatter.Source = opts.SourcePath
	}

	// Add version info
	if doc.Info != nil && doc.Info.Version != "" {
		s.Frontmatter.Version = doc.Info.Version
	}

	// Extract tags as skill tags
	if doc.Tags != nil {
		for _, tag := range doc.Tags {
			s.Frontmatter.Tags = append(s.Frontmatter.Tags, tag.Name)
		}
	}

	// Count endpoints for metadata
	endpointCount := c.countEndpointsV2(&doc)
	s.Frontmatter.EndpointCount = endpointCount

	// Extract auth methods for metadata
	authMethods := c.extractAuthMethodsV2(&doc)
	s.Frontmatter.AuthMethods = authMethods

	// Set base URL if available
	if doc.Host != "" {
		scheme := "https"
		if len(doc.Schemes) > 0 {
			scheme = doc.Schemes[0]
		}
		basePath := doc.BasePath
		if basePath == "" {
			basePath = "/"
		}
		s.Frontmatter.BaseURL = fmt.Sprintf("%s://%s%s", scheme, doc.Host, basePath)
	}

	// Determine difficulty based on complexity
	s.Frontmatter.Difficulty = c.determineDifficultyV2(&doc, endpointCount)

	// Add Quick Start section
	s.AddSection("Quick Start", 2, c.buildQuickStartV2(&doc))

	// Add overview section
	s.AddSection("Overview", 2, c.buildOverviewV2(&doc))

	// Add authentication section if security definitions exist
	if doc.SecurityDefinitions != nil && doc.SecurityDefinitions.Definitions.Len() > 0 {
		s.AddSection("Authentication", 2, c.buildAuthSectionV2(&doc))
	}

	// Add endpoints section
	s.AddSection("Endpoints", 2, c.buildEndpointsSectionV2(&doc))

	// Add schemas section if definitions exist
	if doc.Definitions != nil && doc.Definitions.Definitions.Len() > 0 {
		s.AddSection("Data Models", 2, c.buildSchemasSectionV2(&doc))
	}

	// Add Error Handling section
	s.AddSection("Error Handling", 2, c.buildErrorHandlingSectionV2(&doc))

	// Add Best Practices section
	s.AddSection("Best Practices", 2, c.buildBestPracticesSectionV2(&doc))

	return s
}

func (c *OpenAPIConverter) countEndpointsV2(doc *v2.Swagger) int {
	count := 0
	if doc.Paths == nil {
		return 0
	}
	for pair := doc.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
		item := pair.Value()
		if item.Get != nil {
			count++
		}
		if item.Post != nil {
			count++
		}
		if item.Put != nil {
			count++
		}
		if item.Delete != nil {
			count++
		}
		if item.Patch != nil {
			count++
		}
		if item.Head != nil {
			count++
		}
		if item.Options != nil {
			count++
		}
	}
	return count
}

func (c *OpenAPIConverter) extractAuthMethodsV2(doc *v2.Swagger) []string {
	var methods []string
	if doc.SecurityDefinitions == nil {
		return methods
	}
	for pair := doc.SecurityDefinitions.Definitions.First(); pair != nil; pair = pair.Next() {
		scheme := pair.Value()
		methods = append(methods, scheme.Type)
	}
	return methods
}

func (c *OpenAPIConverter) determineDifficultyV2(doc *v2.Swagger, endpointCount int) string {
	hasAuth := doc.SecurityDefinitions != nil && doc.SecurityDefinitions.Definitions.Len() > 0
	schemaCount := 0
	if doc.Definitions != nil {
		schemaCount = doc.Definitions.Definitions.Len()
	}

	if endpointCount <= 5 && schemaCount <= 5 && !hasAuth {
		return "novice"
	} else if endpointCount <= 20 && schemaCount <= 20 {
		return "intermediate"
	}
	return "advanced"
}

func (c *OpenAPIConverter) buildQuickStartV2(doc *v2.Swagger) string {
	var b strings.Builder

	b.WriteString("Get started with this API in minutes.\n\n")

	// Step 1: Base URL
	b.WriteString("### 1. Set Your Base URL\n\n")
	if doc.Host != "" {
		scheme := "https"
		if len(doc.Schemes) > 0 {
			scheme = doc.Schemes[0]
		}
		basePath := doc.BasePath
		if basePath == "" {
			basePath = "/"
		}
		b.WriteString(fmt.Sprintf("```\n%s://%s%s\n```\n\n", scheme, doc.Host, basePath))
	} else {
		b.WriteString("```\nhttps://api.example.com\n```\n\n")
	}

	// Step 2: Authentication
	if doc.SecurityDefinitions != nil && doc.SecurityDefinitions.Definitions.Len() > 0 {
		b.WriteString("### 2. Authenticate\n\n")
		for pair := doc.SecurityDefinitions.Definitions.First(); pair != nil; pair = pair.Next() {
			scheme := pair.Value()
			switch scheme.Type {
			case "basic":
				b.WriteString("Use HTTP Basic authentication:\n\n")
				b.WriteString("```\nAuthorization: Basic BASE64_ENCODED_CREDENTIALS\n```\n\n")
			case "apiKey":
				b.WriteString(fmt.Sprintf("Add your API key to the `%s` %s:\n\n", scheme.Name, scheme.In))
				if scheme.In == "header" {
					b.WriteString(fmt.Sprintf("```\n%s: YOUR_API_KEY\n```\n\n", scheme.Name))
				} else if scheme.In == "query" {
					b.WriteString(fmt.Sprintf("```\n?%s=YOUR_API_KEY\n```\n\n", scheme.Name))
				}
			case "oauth2":
				b.WriteString("Configure OAuth 2.0 authentication. See the Authentication section for flow details.\n\n")
			}
			break // Just show the first auth method for quick start
		}
	}

	// Step 3: Make your first request
	b.WriteString("### 3. Make Your First Request\n\n")
	if doc.Paths != nil {
		// Find a simple GET endpoint to demonstrate
		for pair := doc.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
			path := pair.Key()
			item := pair.Value()
			if item.Get != nil {
				baseURL := "https://api.example.com"
				if doc.Host != "" {
					scheme := "https"
					if len(doc.Schemes) > 0 {
						scheme = doc.Schemes[0]
					}
					basePath := doc.BasePath
					if basePath == "" {
						basePath = ""
					}
					baseURL = fmt.Sprintf("%s://%s%s", scheme, doc.Host, strings.TrimSuffix(basePath, "/"))
				}
				b.WriteString("**cURL:**\n")
				b.WriteString("```bash\n")
				b.WriteString(fmt.Sprintf("curl -X GET \"%s%s\" \\\n", baseURL, path))
				b.WriteString("  -H \"Accept: application/json\"\n")
				b.WriteString("```\n\n")
				break
			}
		}
	}

	return strings.TrimSpace(b.String())
}

func (c *OpenAPIConverter) buildOverviewV2(doc *v2.Swagger) string {
	var b strings.Builder

	if doc.Info != nil {
		if doc.Info.Description != "" {
			b.WriteString(doc.Info.Description)
			b.WriteString("\n\n")
		}

		if doc.Info.Contact != nil {
			b.WriteString("**Contact**: ")
			if doc.Info.Contact.Name != "" {
				b.WriteString(doc.Info.Contact.Name)
			}
			if doc.Info.Contact.Email != "" {
				b.WriteString(fmt.Sprintf(" <%s>", doc.Info.Contact.Email))
			}
			if doc.Info.Contact.URL != "" {
				b.WriteString(fmt.Sprintf(" ([website](%s))", doc.Info.Contact.URL))
			}
			b.WriteString("\n")
		}

		if doc.Info.License != nil {
			b.WriteString(fmt.Sprintf("**License**: %s", doc.Info.License.Name))
			if doc.Info.License.URL != "" {
				b.WriteString(fmt.Sprintf(" ([details](%s))", doc.Info.License.URL))
			}
			b.WriteString("\n")
		}

		if doc.Info.TermsOfService != "" {
			b.WriteString(fmt.Sprintf("**Terms of Service**: %s\n", doc.Info.TermsOfService))
		}
	}

	// Add base URL
	if doc.Host != "" {
		scheme := "https"
		if len(doc.Schemes) > 0 {
			scheme = doc.Schemes[0]
		}
		basePath := doc.BasePath
		if basePath == "" {
			basePath = "/"
		}
		b.WriteString(fmt.Sprintf("\n**Base URL**: `%s://%s%s`\n", scheme, doc.Host, basePath))
	}

	// Add external docs if available
	if doc.ExternalDocs != nil && doc.ExternalDocs.URL != "" {
		b.WriteString(fmt.Sprintf("\n**External Documentation**: [%s](%s)\n",
			doc.ExternalDocs.Description, doc.ExternalDocs.URL))
	}

	return strings.TrimSpace(b.String())
}

func (c *OpenAPIConverter) buildAuthSectionV2(doc *v2.Swagger) string {
	var b strings.Builder

	for pair := doc.SecurityDefinitions.Definitions.First(); pair != nil; pair = pair.Next() {
		name := pair.Key()
		scheme := pair.Value()
		b.WriteString(fmt.Sprintf("### %s\n\n", name))
		b.WriteString(fmt.Sprintf("**Type**: `%s`\n", scheme.Type))

		if scheme.In != "" {
			b.WriteString(fmt.Sprintf("**In**: `%s`\n", scheme.In))
		}
		if scheme.Name != "" {
			b.WriteString(fmt.Sprintf("**Parameter Name**: `%s`\n", scheme.Name))
		}
		if scheme.Description != "" {
			b.WriteString(fmt.Sprintf("\n%s\n", scheme.Description))
		}

		b.WriteString("\n**Example Usage**:\n\n")
		switch scheme.Type {
		case "basic":
			b.WriteString("```bash\n")
			b.WriteString("curl -u username:password https://api.example.com/resource\n")
			b.WriteString("```\n")
		case "apiKey":
			if scheme.In == "header" {
				b.WriteString("```bash\n")
				b.WriteString(fmt.Sprintf("curl -H \"%s: your-api-key\" https://api.example.com/resource\n", scheme.Name))
				b.WriteString("```\n")
			} else if scheme.In == "query" {
				b.WriteString("```bash\n")
				b.WriteString(fmt.Sprintf("curl \"https://api.example.com/resource?%s=your-api-key\"\n", scheme.Name))
				b.WriteString("```\n")
			}
		case "oauth2":
			b.WriteString("```javascript\n")
			b.WriteString("// OAuth 2.0 flow\n")
			b.WriteString("const token = await getOAuthToken(clientId, clientSecret);\n")
			b.WriteString("fetch('/resource', {\n")
			b.WriteString("  headers: { 'Authorization': `Bearer ${token}` }\n")
			b.WriteString("});\n")
			b.WriteString("```\n")
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *OpenAPIConverter) buildEndpointsSectionV2(doc *v2.Swagger) string {
	var b strings.Builder

	if doc.Paths == nil {
		return "No endpoints defined."
	}

	// Collect and sort paths
	var paths []string
	for pair := doc.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
		paths = append(paths, pair.Key())
	}
	sort.Strings(paths)

	for _, path := range paths {
		pathItem, _ := doc.Paths.PathItems.Get(path)
		b.WriteString(c.buildPathSectionV2(path, pathItem, doc))
	}

	return strings.TrimSpace(b.String())
}

func (c *OpenAPIConverter) buildPathSectionV2(path string, item *v2.PathItem, doc *v2.Swagger) string {
	var b strings.Builder

	operations := []struct {
		method string
		op     *v2.Operation
	}{
		{"GET", item.Get},
		{"POST", item.Post},
		{"PUT", item.Put},
		{"DELETE", item.Delete},
		{"PATCH", item.Patch},
		{"HEAD", item.Head},
		{"OPTIONS", item.Options},
	}

	for _, entry := range operations {
		if entry.op == nil {
			continue
		}

		method := entry.method
		op := entry.op

		b.WriteString(fmt.Sprintf("### %s %s\n\n", method, path))

		// Tags
		if len(op.Tags) > 0 {
			b.WriteString(fmt.Sprintf("**Tags**: %s\n\n", strings.Join(op.Tags, ", ")))
		}

		if op.Summary != "" {
			b.WriteString(fmt.Sprintf("**%s**\n\n", op.Summary))
		}

		if op.Description != "" {
			b.WriteString(op.Description)
			b.WriteString("\n\n")
		}

		// Deprecated warning
		if op.Deprecated {
			b.WriteString("> ⚠️ **Deprecated**: This endpoint is deprecated and may be removed in future versions.\n\n")
		}

		// Parameters
		if len(op.Parameters) > 0 {
			b.WriteString("**Parameters**:\n\n")
			b.WriteString("| Name | In | Type | Required | Description |\n")
			b.WriteString("|------|-----|------|----------|-------------|\n")
			for _, param := range op.Parameters {
				required := "No"
				if param.Required != nil && *param.Required {
					required = "Yes"
				}
				paramType := "string"
				if param.Type != "" {
					paramType = param.Type
				}
				desc := strings.ReplaceAll(param.Description, "\n", " ")
				b.WriteString(fmt.Sprintf("| `%s` | %s | `%s` | %s | %s |\n",
					param.Name, param.In, paramType, required, desc))
			}
			b.WriteString("\n")
		}

		// Responses
		if op.Responses != nil && op.Responses.Codes != nil {
			b.WriteString("**Responses**:\n\n")
			for pair := op.Responses.Codes.First(); pair != nil; pair = pair.Next() {
				code := pair.Key()
				resp := pair.Value()
				desc := ""
				if resp.Description != "" {
					desc = strings.ReplaceAll(resp.Description, "\n", " ")
				}
				b.WriteString(fmt.Sprintf("- **%s**: %s\n", code, desc))
			}
			b.WriteString("\n")
		}

		// Code examples
		b.WriteString(c.buildCodeExamplesV2(method, path, doc))
	}

	return b.String()
}

func (c *OpenAPIConverter) buildCodeExamplesV2(method, path string, doc *v2.Swagger) string {
	var b strings.Builder

	baseURL := "https://api.example.com"
	if doc.Host != "" {
		scheme := "https"
		if len(doc.Schemes) > 0 {
			scheme = doc.Schemes[0]
		}
		basePath := strings.TrimSuffix(doc.BasePath, "/")
		baseURL = fmt.Sprintf("%s://%s%s", scheme, doc.Host, basePath)
	}

	b.WriteString("**Code Examples**:\n\n")

	// cURL example
	b.WriteString("<details>\n<summary>cURL</summary>\n\n")
	b.WriteString("```bash\n")
	b.WriteString(fmt.Sprintf("curl -X %s \"%s%s\" \\\n", method, baseURL, path))
	b.WriteString("  -H \"Accept: application/json\"")
	if method == "POST" || method == "PUT" || method == "PATCH" {
		b.WriteString(" \\\n  -H \"Content-Type: application/json\"")
		b.WriteString(" \\\n  -d '{\"key\": \"value\"}'")
	}
	b.WriteString("\n```\n\n")
	b.WriteString("</details>\n\n")

	return b.String()
}

func (c *OpenAPIConverter) buildSchemasSectionV2(doc *v2.Swagger) string {
	var b strings.Builder

	// Collect and sort schema names
	var schemaNames []string
	for pair := doc.Definitions.Definitions.First(); pair != nil; pair = pair.Next() {
		schemaNames = append(schemaNames, pair.Key())
	}
	sort.Strings(schemaNames)

	for _, name := range schemaNames {
		schemaProxy, _ := doc.Definitions.Definitions.Get(name)
		schema := schemaProxy.Schema()

		b.WriteString(fmt.Sprintf("### %s\n\n", name))

		if schema.Description != "" {
			b.WriteString(schema.Description)
			b.WriteString("\n\n")
		}

		// Show type
		if len(schema.Type) > 0 {
			b.WriteString(fmt.Sprintf("**Type**: `%s`\n\n", schema.Type[0]))
		}

		// List properties
		if schema.Properties != nil && schema.Properties.Len() > 0 {
			b.WriteString("| Property | Type | Required | Description |\n")
			b.WriteString("|----------|------|----------|-------------|\n")

			requiredSet := make(map[string]bool)
			for _, r := range schema.Required {
				requiredSet[r] = true
			}

			for pair := schema.Properties.First(); pair != nil; pair = pair.Next() {
				propName := pair.Key()
				prop := pair.Value().Schema()

				propType := "any"
				if len(prop.Type) > 0 {
					propType = prop.Type[0]
					if prop.Format != "" {
						propType = fmt.Sprintf("%s (%s)", propType, prop.Format)
					}
				}

				required := "No"
				if requiredSet[propName] {
					required = "Yes"
				}

				desc := strings.ReplaceAll(prop.Description, "\n", " ")
				b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s | %s |\n", propName, propType, required, desc))
			}
			b.WriteString("\n")
		}
	}

	return strings.TrimSpace(b.String())
}

func (c *OpenAPIConverter) buildErrorHandlingSectionV2(doc *v2.Swagger) string {
	var b strings.Builder

	b.WriteString("This section documents common error responses and how to handle them.\n\n")

	// Collect error codes from all operations
	errorCodes := make(map[string]string)
	if doc.Paths != nil {
		for pair := doc.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
			item := pair.Value()
			ops := []*v2.Operation{item.Get, item.Post, item.Put, item.Delete, item.Patch}
			for _, op := range ops {
				if op != nil && op.Responses != nil && op.Responses.Codes != nil {
					for respPair := op.Responses.Codes.First(); respPair != nil; respPair = respPair.Next() {
						code := respPair.Key()
						// Only collect 4xx and 5xx codes
						if strings.HasPrefix(code, "4") || strings.HasPrefix(code, "5") {
							resp := respPair.Value()
							if resp.Description != "" {
								errorCodes[code] = resp.Description
							}
						}
					}
				}
			}
		}
	}

	if len(errorCodes) > 0 {
		b.WriteString("### Error Codes\n\n")
		b.WriteString("| Code | Description | Recommended Action |\n")
		b.WriteString("|------|-------------|-------------------|\n")

		// Sort codes
		codes := make([]string, 0, len(errorCodes))
		for code := range errorCodes {
			codes = append(codes, code)
		}
		sort.Strings(codes)

		for _, code := range codes {
			action := c.getErrorAction(code)
			desc := strings.ReplaceAll(errorCodes[code], "\n", " ")
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", code, desc, action))
		}
		b.WriteString("\n")
	}

	b.WriteString("### Handling Errors\n\n")
	b.WriteString("Always check the HTTP status code and handle errors appropriately.\n")

	return strings.TrimSpace(b.String())
}

func (c *OpenAPIConverter) buildBestPracticesSectionV2(doc *v2.Swagger) string {
	var b strings.Builder

	b.WriteString("Follow these recommendations for optimal API usage.\n\n")

	b.WriteString("### Authentication\n\n")
	b.WriteString("- Store credentials securely (environment variables, secret managers)\n")
	b.WriteString("- Never commit API keys to version control\n")
	b.WriteString("- Rotate credentials periodically\n\n")

	b.WriteString("### Request Handling\n\n")
	b.WriteString("- Always set appropriate `Content-Type` and `Accept` headers\n")
	b.WriteString("- Validate input before sending requests\n")
	b.WriteString("- Use HTTPS for all requests\n")
	b.WriteString("- Implement request timeouts\n\n")

	b.WriteString("### Error Handling\n\n")
	b.WriteString("- Implement exponential backoff for retries\n")
	b.WriteString("- Log errors with request context for debugging\n")
	b.WriteString("- Handle network errors gracefully\n")

	return strings.TrimSpace(b.String())
}
