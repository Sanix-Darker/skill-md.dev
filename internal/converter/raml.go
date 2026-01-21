// Package converter provides spec-to-SKILL.md converters.
package converter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/sanixdarker/skill-md/pkg/skill"
	"gopkg.in/yaml.v2"
)

// RAMLConverter converts RAML specifications to skills.
type RAMLConverter struct{}

// RAML spec types
type ramlSpec struct {
	Title           string                        `yaml:"title"`
	Version         string                        `yaml:"version"`
	BaseURI         string                        `yaml:"baseUri"`
	Description     string                        `yaml:"description"`
	MediaType       interface{}                   `yaml:"mediaType"`
	Protocols       []string                      `yaml:"protocols"`
	Documentation   []ramlDocumentation           `yaml:"documentation"`
	Types           map[string]interface{}        `yaml:"types"`
	Traits          map[string]interface{}        `yaml:"traits"`
	ResourceTypes   map[string]interface{}        `yaml:"resourceTypes"`
	SecuritySchemes map[string]ramlSecurityScheme `yaml:"securitySchemes"`
	SecuredBy       []interface{}                 `yaml:"securedBy"`
	// Resources are parsed separately (keys starting with /)
	Resources map[string]*ramlResource `yaml:"-"`
}

type ramlDocumentation struct {
	Title   string `yaml:"title"`
	Content string `yaml:"content"`
}

type ramlSecurityScheme struct {
	Type        string                 `yaml:"type"`
	Description string                 `yaml:"description"`
	Settings    map[string]interface{} `yaml:"settings"`
}

type ramlResource struct {
	DisplayName  string                   `yaml:"displayName"`
	Description  string                   `yaml:"description"`
	URIParams    map[string]ramlParam     `yaml:"uriParameters"`
	Methods      map[string]*ramlMethod   `yaml:"-"`
	SubResources map[string]*ramlResource `yaml:"-"`
	Is           []string                 `yaml:"is"` // traits
	Type         string                   `yaml:"type"` // resource type
}

type ramlMethod struct {
	DisplayName string                      `yaml:"displayName"`
	Description string                      `yaml:"description"`
	QueryParams map[string]ramlParam        `yaml:"queryParameters"`
	Headers     map[string]ramlParam        `yaml:"headers"`
	Body        map[string]ramlBody         `yaml:"body"`
	Responses   map[string]ramlResponse     `yaml:"responses"`
	SecuredBy   []interface{}               `yaml:"securedBy"`
	Is          []string                    `yaml:"is"` // traits
}

type ramlParam struct {
	Type        string      `yaml:"type"`
	Description string      `yaml:"description"`
	Required    bool        `yaml:"required"`
	Default     interface{} `yaml:"default"`
	Example     interface{} `yaml:"example"`
	Enum        []string    `yaml:"enum"`
	Pattern     string      `yaml:"pattern"`
	MinLength   int         `yaml:"minLength"`
	MaxLength   int         `yaml:"maxLength"`
	Minimum     interface{} `yaml:"minimum"`
	Maximum     interface{} `yaml:"maximum"`
}

type ramlBody struct {
	Type       string                 `yaml:"type"`
	Schema     interface{}            `yaml:"schema"`
	Example    interface{}            `yaml:"example"`
	Properties map[string]interface{} `yaml:"properties"`
}

type ramlResponse struct {
	Description string              `yaml:"description"`
	Headers     map[string]ramlParam `yaml:"headers"`
	Body        map[string]ramlBody  `yaml:"body"`
}

func (c *RAMLConverter) Name() string {
	return "raml"
}

func (c *RAMLConverter) CanHandle(filename string, content []byte) bool {
	ext := getExtension(filename)
	if ext == ".raml" {
		return true
	}
	// Check for RAML header
	return bytes.HasPrefix(content, []byte("#%RAML"))
}

func (c *RAMLConverter) Convert(content []byte, opts *Options) (*skill.Skill, error) {
	spec, err := c.parseRAML(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RAML spec: %w", err)
	}

	return c.buildSkill(spec, opts), nil
}

func (c *RAMLConverter) parseRAML(content []byte) (*ramlSpec, error) {
	// Remove RAML header comment
	lines := bytes.Split(content, []byte("\n"))
	var cleanLines [][]byte
	for _, line := range lines {
		if !bytes.HasPrefix(bytes.TrimSpace(line), []byte("#%RAML")) {
			cleanLines = append(cleanLines, line)
		}
	}
	cleanContent := bytes.Join(cleanLines, []byte("\n"))

	// First pass: parse known fields
	var spec ramlSpec
	if err := yaml.Unmarshal(cleanContent, &spec); err != nil {
		return nil, err
	}

	// Second pass: parse resources (keys starting with /)
	var rawMap map[string]interface{}
	if err := yaml.Unmarshal(cleanContent, &rawMap); err != nil {
		return nil, err
	}

	spec.Resources = make(map[string]*ramlResource)
	for key, value := range rawMap {
		if strings.HasPrefix(key, "/") {
			resource := c.parseResource(value)
			spec.Resources[key] = resource
		}
	}

	return &spec, nil
}

func (c *RAMLConverter) parseResource(data interface{}) *ramlResource {
	resource := &ramlResource{
		Methods:      make(map[string]*ramlMethod),
		SubResources: make(map[string]*ramlResource),
		URIParams:    make(map[string]ramlParam),
	}

	dataMap, ok := data.(map[interface{}]interface{})
	if !ok {
		return resource
	}

	for key, value := range dataMap {
		keyStr := fmt.Sprintf("%v", key)

		switch keyStr {
		case "displayName":
			resource.DisplayName = fmt.Sprintf("%v", value)
		case "description":
			resource.Description = fmt.Sprintf("%v", value)
		case "uriParameters":
			if params, ok := value.(map[interface{}]interface{}); ok {
				for pName, pValue := range params {
					resource.URIParams[fmt.Sprintf("%v", pName)] = c.parseParam(pValue)
				}
			}
		case "is":
			if traits, ok := value.([]interface{}); ok {
				for _, t := range traits {
					resource.Is = append(resource.Is, fmt.Sprintf("%v", t))
				}
			}
		case "type":
			resource.Type = fmt.Sprintf("%v", value)
		case "get", "post", "put", "patch", "delete", "head", "options":
			resource.Methods[keyStr] = c.parseMethod(value)
		default:
			// Check if it's a sub-resource
			if strings.HasPrefix(keyStr, "/") {
				resource.SubResources[keyStr] = c.parseResource(value)
			}
		}
	}

	return resource
}

func (c *RAMLConverter) parseMethod(data interface{}) *ramlMethod {
	method := &ramlMethod{
		QueryParams: make(map[string]ramlParam),
		Headers:     make(map[string]ramlParam),
		Body:        make(map[string]ramlBody),
		Responses:   make(map[string]ramlResponse),
	}

	dataMap, ok := data.(map[interface{}]interface{})
	if !ok {
		return method
	}

	for key, value := range dataMap {
		keyStr := fmt.Sprintf("%v", key)

		switch keyStr {
		case "displayName":
			method.DisplayName = fmt.Sprintf("%v", value)
		case "description":
			method.Description = fmt.Sprintf("%v", value)
		case "queryParameters":
			if params, ok := value.(map[interface{}]interface{}); ok {
				for pName, pValue := range params {
					method.QueryParams[fmt.Sprintf("%v", pName)] = c.parseParam(pValue)
				}
			}
		case "headers":
			if headers, ok := value.(map[interface{}]interface{}); ok {
				for hName, hValue := range headers {
					method.Headers[fmt.Sprintf("%v", hName)] = c.parseParam(hValue)
				}
			}
		case "body":
			if bodies, ok := value.(map[interface{}]interface{}); ok {
				for contentType, bodyData := range bodies {
					method.Body[fmt.Sprintf("%v", contentType)] = c.parseBody(bodyData)
				}
			}
		case "responses":
			if responses, ok := value.(map[interface{}]interface{}); ok {
				for status, respData := range responses {
					method.Responses[fmt.Sprintf("%v", status)] = c.parseResponse(respData)
				}
			}
		case "is":
			if traits, ok := value.([]interface{}); ok {
				for _, t := range traits {
					method.Is = append(method.Is, fmt.Sprintf("%v", t))
				}
			}
		}
	}

	return method
}

func (c *RAMLConverter) parseParam(data interface{}) ramlParam {
	param := ramlParam{Type: "string"}

	dataMap, ok := data.(map[interface{}]interface{})
	if !ok {
		return param
	}

	for key, value := range dataMap {
		keyStr := fmt.Sprintf("%v", key)

		switch keyStr {
		case "type":
			param.Type = fmt.Sprintf("%v", value)
		case "description":
			param.Description = fmt.Sprintf("%v", value)
		case "required":
			param.Required = value == true
		case "default":
			param.Default = value
		case "example":
			param.Example = value
		case "enum":
			if enums, ok := value.([]interface{}); ok {
				for _, e := range enums {
					param.Enum = append(param.Enum, fmt.Sprintf("%v", e))
				}
			}
		case "pattern":
			param.Pattern = fmt.Sprintf("%v", value)
		}
	}

	return param
}

func (c *RAMLConverter) parseBody(data interface{}) ramlBody {
	body := ramlBody{}

	dataMap, ok := data.(map[interface{}]interface{})
	if !ok {
		return body
	}

	for key, value := range dataMap {
		keyStr := fmt.Sprintf("%v", key)

		switch keyStr {
		case "type":
			body.Type = fmt.Sprintf("%v", value)
		case "schema":
			body.Schema = value
		case "example":
			body.Example = value
		case "properties":
			if props, ok := value.(map[interface{}]interface{}); ok {
				body.Properties = make(map[string]interface{})
				for pName, pValue := range props {
					body.Properties[fmt.Sprintf("%v", pName)] = pValue
				}
			}
		}
	}

	return body
}

func (c *RAMLConverter) parseResponse(data interface{}) ramlResponse {
	resp := ramlResponse{
		Headers: make(map[string]ramlParam),
		Body:    make(map[string]ramlBody),
	}

	dataMap, ok := data.(map[interface{}]interface{})
	if !ok {
		return resp
	}

	for key, value := range dataMap {
		keyStr := fmt.Sprintf("%v", key)

		switch keyStr {
		case "description":
			resp.Description = fmt.Sprintf("%v", value)
		case "headers":
			if headers, ok := value.(map[interface{}]interface{}); ok {
				for hName, hValue := range headers {
					resp.Headers[fmt.Sprintf("%v", hName)] = c.parseParam(hValue)
				}
			}
		case "body":
			if bodies, ok := value.(map[interface{}]interface{}); ok {
				for contentType, bodyData := range bodies {
					resp.Body[fmt.Sprintf("%v", contentType)] = c.parseBody(bodyData)
				}
			}
		}
	}

	return resp
}

func (c *RAMLConverter) buildSkill(spec *ramlSpec, opts *Options) *skill.Skill {
	name := spec.Title
	if opts != nil && opts.Name != "" {
		name = opts.Name
	}
	if name == "" {
		name = "RAML API"
	}

	description := spec.Description
	if description == "" {
		description = fmt.Sprintf("API documentation for %s", name)
	}

	s := skill.NewSkill(name, description)
	s.Frontmatter.SourceType = "raml"
	if opts != nil && opts.SourcePath != "" {
		s.Frontmatter.Source = opts.SourcePath
	}
	if spec.Version != "" {
		s.Frontmatter.Version = spec.Version
	}

	// Set metadata
	s.Frontmatter.Protocol = "http"
	s.Frontmatter.BaseURL = spec.BaseURI
	s.Frontmatter.EndpointCount = c.countEndpoints(spec)
	s.Frontmatter.AuthMethods = c.extractAuthMethods(spec)
	s.Frontmatter.Difficulty = c.calculateDifficulty(spec)
	s.Frontmatter.HasExamples = true
	s.Frontmatter.Tags = []string{"raml", "rest", "api"}

	// MCP-compatible settings
	s.Frontmatter.MCPCompatible = true
	s.Frontmatter.ToolDefinitions = c.buildToolDefinitions(spec)
	s.Frontmatter.RetryStrategy = &skill.RetryStrategy{
		MaxRetries:     3,
		BackoffType:    "exponential",
		InitialDelayMs: 1000,
	}

	// Build sections
	s.AddSection("Quick Start", 2, c.buildQuickStart(spec))
	s.AddSection("Overview", 2, c.buildOverview(spec))

	if len(spec.SecuritySchemes) > 0 {
		s.AddSection("Authentication", 2, c.buildAuthSection(spec))
	}

	s.AddSection("Endpoints", 2, c.buildEndpointsSection(spec))

	if len(spec.Types) > 0 {
		s.AddSection("Data Types", 2, c.buildTypesSection(spec))
	}

	if len(spec.Traits) > 0 {
		s.AddSection("Traits", 2, c.buildTraitsSection(spec))
	}

	s.AddSection("Code Examples", 2, c.buildCodeExamples(spec))
	s.AddSection("Tool Definitions", 2, c.buildToolDefinitionsSection(spec))
	s.AddSection("Best Practices", 2, c.buildBestPractices())

	return s
}

func (c *RAMLConverter) countEndpoints(spec *ramlSpec) int {
	count := 0
	var countResource func(r *ramlResource)
	countResource = func(r *ramlResource) {
		count += len(r.Methods)
		for _, sub := range r.SubResources {
			countResource(sub)
		}
	}
	for _, r := range spec.Resources {
		countResource(r)
	}
	return count
}

func (c *RAMLConverter) extractAuthMethods(spec *ramlSpec) []string {
	methods := make([]string, 0)
	for _, scheme := range spec.SecuritySchemes {
		methods = append(methods, scheme.Type)
	}
	return methods
}

func (c *RAMLConverter) calculateDifficulty(spec *ramlSpec) string {
	endpoints := c.countEndpoints(spec)
	types := len(spec.Types)
	complexity := endpoints + types

	if complexity <= 5 {
		return "novice"
	} else if complexity <= 20 {
		return "intermediate"
	}
	return "advanced"
}

func (c *RAMLConverter) buildQuickStart(spec *ramlSpec) string {
	var b strings.Builder

	b.WriteString("### Getting Started\n\n")
	b.WriteString(fmt.Sprintf("1. **Base URL**: `%s`\n", spec.BaseURI))

	if len(spec.SecuritySchemes) > 0 {
		b.WriteString("2. **Authenticate** using one of the supported methods\n")
		b.WriteString("3. **Make requests** to the API endpoints\n\n")
	} else {
		b.WriteString("2. **Make requests** to the API endpoints\n\n")
	}

	// Quick example
	var firstPath string
	var firstMethod string
	for path, resource := range spec.Resources {
		firstPath = path
		for method := range resource.Methods {
			firstMethod = method
			break
		}
		break
	}

	if firstPath != "" && firstMethod != "" {
		b.WriteString("### Quick Example\n\n")
		b.WriteString("```bash\n")
		b.WriteString(fmt.Sprintf("curl -X %s %s%s \\\n", strings.ToUpper(firstMethod), spec.BaseURI, firstPath))
		b.WriteString("  -H \"Authorization: Bearer YOUR_TOKEN\" \\\n")
		b.WriteString("  -H \"Content-Type: application/json\"\n")
		b.WriteString("```\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *RAMLConverter) buildOverview(spec *ramlSpec) string {
	var b strings.Builder

	if spec.Description != "" {
		b.WriteString(spec.Description)
		b.WriteString("\n\n")
	}

	b.WriteString("| Property | Value |\n")
	b.WriteString("|----------|-------|\n")
	b.WriteString(fmt.Sprintf("| **Title** | %s |\n", spec.Title))
	if spec.Version != "" {
		b.WriteString(fmt.Sprintf("| **Version** | %s |\n", spec.Version))
	}
	b.WriteString(fmt.Sprintf("| **Base URI** | %s |\n", spec.BaseURI))
	b.WriteString(fmt.Sprintf("| **Endpoints** | %d |\n", c.countEndpoints(spec)))

	if len(spec.Protocols) > 0 {
		b.WriteString(fmt.Sprintf("| **Protocols** | %s |\n", strings.Join(spec.Protocols, ", ")))
	}

	mediaTypes := c.extractMediaTypes(spec)
	if len(mediaTypes) > 0 {
		b.WriteString(fmt.Sprintf("| **Media Types** | %s |\n", strings.Join(mediaTypes, ", ")))
	}

	// Documentation links
	if len(spec.Documentation) > 0 {
		b.WriteString("\n### Documentation\n\n")
		for _, doc := range spec.Documentation {
			b.WriteString(fmt.Sprintf("- **%s**: %s\n", doc.Title, truncate(doc.Content, 100)))
		}
	}

	return strings.TrimSpace(b.String())
}

func (c *RAMLConverter) extractMediaTypes(spec *ramlSpec) []string {
	switch v := spec.MediaType.(type) {
	case string:
		return []string{v}
	case []interface{}:
		types := make([]string, len(v))
		for i, t := range v {
			types[i] = fmt.Sprintf("%v", t)
		}
		return types
	}
	return []string{"application/json"}
}

func (c *RAMLConverter) buildAuthSection(spec *ramlSpec) string {
	var b strings.Builder

	for name, scheme := range spec.SecuritySchemes {
		b.WriteString(fmt.Sprintf("### %s\n\n", name))
		b.WriteString(fmt.Sprintf("**Type**: %s\n", scheme.Type))

		if scheme.Description != "" {
			b.WriteString(fmt.Sprintf("\n%s\n", scheme.Description))
		}

		b.WriteString("\n")

		switch scheme.Type {
		case "OAuth 2.0":
			b.WriteString("**OAuth 2.0 Settings:**\n")
			if settings := scheme.Settings; settings != nil {
				if authUri, ok := settings["authorizationUri"]; ok {
					b.WriteString(fmt.Sprintf("- Authorization URI: `%v`\n", authUri))
				}
				if tokenUri, ok := settings["accessTokenUri"]; ok {
					b.WriteString(fmt.Sprintf("- Token URI: `%v`\n", tokenUri))
				}
				if grants, ok := settings["authorizationGrants"]; ok {
					b.WriteString(fmt.Sprintf("- Grants: %v\n", grants))
				}
			}
			b.WriteString("\n```javascript\n")
			b.WriteString("// OAuth 2.0 Authorization\n")
			b.WriteString("const token = await getOAuthToken(clientId, clientSecret);\n")
			b.WriteString("headers['Authorization'] = `Bearer ${token}`;\n")
			b.WriteString("```\n")
		case "Basic Authentication":
			b.WriteString("```bash\n")
			b.WriteString("# Base64 encode credentials\n")
			b.WriteString("curl -u username:password https://api.example.com/resource\n")
			b.WriteString("```\n")
		case "x-api-key", "Pass Through":
			if settings := scheme.Settings; settings != nil {
				if headerName, ok := settings["headerName"]; ok {
					b.WriteString(fmt.Sprintf("Header: `%v`\n\n", headerName))
					b.WriteString("```bash\n")
					b.WriteString(fmt.Sprintf("curl -H \"%v: YOUR_API_KEY\" https://api.example.com/resource\n", headerName))
					b.WriteString("```\n")
				}
			}
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *RAMLConverter) buildEndpointsSection(spec *ramlSpec) string {
	var b strings.Builder

	// Sort resource paths
	paths := make([]string, 0, len(spec.Resources))
	for path := range spec.Resources {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var writeResource func(path string, resource *ramlResource, depth int)
	writeResource = func(path string, resource *ramlResource, depth int) {
		// Write resource
		b.WriteString(fmt.Sprintf("%s `%s`\n\n", strings.Repeat("#", 3+depth), path))

		if resource.DisplayName != "" {
			b.WriteString(fmt.Sprintf("**%s**\n\n", resource.DisplayName))
		}

		if resource.Description != "" {
			b.WriteString(resource.Description)
			b.WriteString("\n\n")
		}

		// URI Parameters
		if len(resource.URIParams) > 0 {
			b.WriteString("**URI Parameters:**\n")
			for pName, p := range resource.URIParams {
				req := ""
				if p.Required {
					req = " (required)"
				}
				b.WriteString(fmt.Sprintf("- `{%s}`%s: %s\n", pName, req, p.Description))
			}
			b.WriteString("\n")
		}

		// Methods
		methodOrder := []string{"get", "post", "put", "patch", "delete", "head", "options"}
		for _, method := range methodOrder {
			if m, ok := resource.Methods[method]; ok {
				c.writeMethod(&b, method, m, spec)
			}
		}

		// Sub-resources
		subPaths := make([]string, 0, len(resource.SubResources))
		for subPath := range resource.SubResources {
			subPaths = append(subPaths, subPath)
		}
		sort.Strings(subPaths)

		for _, subPath := range subPaths {
			fullPath := path + subPath
			writeResource(fullPath, resource.SubResources[subPath], depth+1)
		}
	}

	for _, path := range paths {
		writeResource(path, spec.Resources[path], 0)
	}

	return strings.TrimSpace(b.String())
}

func (c *RAMLConverter) writeMethod(b *strings.Builder, method string, m *ramlMethod, spec *ramlSpec) {
	b.WriteString(fmt.Sprintf("#### %s\n\n", strings.ToUpper(method)))

	if m.DisplayName != "" {
		b.WriteString(fmt.Sprintf("**%s**\n\n", m.DisplayName))
	}

	if m.Description != "" {
		b.WriteString(m.Description)
		b.WriteString("\n\n")
	}

	// Query Parameters
	if len(m.QueryParams) > 0 {
		b.WriteString("**Query Parameters:**\n\n")
		b.WriteString("| Name | Type | Required | Description |\n")
		b.WriteString("|------|------|----------|-------------|\n")
		for pName, p := range m.QueryParams {
			req := "No"
			if p.Required {
				req = "Yes"
			}
			desc := p.Description
			if desc == "" {
				desc = "-"
			}
			b.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n", pName, p.Type, req, desc))
		}
		b.WriteString("\n")
	}

	// Request Body
	if len(m.Body) > 0 {
		b.WriteString("**Request Body:**\n\n")
		for contentType, body := range m.Body {
			b.WriteString(fmt.Sprintf("Content-Type: `%s`\n\n", contentType))

			if body.Type != "" {
				b.WriteString(fmt.Sprintf("Type: `%s`\n\n", body.Type))
			}

			if body.Example != nil {
				b.WriteString("Example:\n")
				b.WriteString("```json\n")
				example, _ := json.MarshalIndent(body.Example, "", "  ")
				b.WriteString(string(example))
				b.WriteString("\n```\n\n")
			}
		}
	}

	// Responses
	if len(m.Responses) > 0 {
		b.WriteString("**Responses:**\n\n")
		for status, resp := range m.Responses {
			b.WriteString(fmt.Sprintf("**%s**", status))
			if resp.Description != "" {
				b.WriteString(fmt.Sprintf(": %s", resp.Description))
			}
			b.WriteString("\n\n")

			for contentType, body := range resp.Body {
				if body.Example != nil {
					b.WriteString(fmt.Sprintf("Content-Type: `%s`\n", contentType))
					b.WriteString("```json\n")
					example, _ := json.MarshalIndent(body.Example, "", "  ")
					b.WriteString(string(example))
					b.WriteString("\n```\n\n")
				}
			}
		}
	}
}

func (c *RAMLConverter) buildTypesSection(spec *ramlSpec) string {
	var b strings.Builder

	// Sort type names
	typeNames := make([]string, 0, len(spec.Types))
	for name := range spec.Types {
		typeNames = append(typeNames, name)
	}
	sort.Strings(typeNames)

	for _, name := range typeNames {
		typeDef := spec.Types[name]
		b.WriteString(fmt.Sprintf("### %s\n\n", name))

		typeMap, ok := typeDef.(map[interface{}]interface{})
		if !ok {
			// Simple type reference
			b.WriteString(fmt.Sprintf("Type: `%v`\n\n", typeDef))
			continue
		}

		// Type description
		if desc, ok := typeMap["description"]; ok {
			b.WriteString(fmt.Sprintf("%v\n\n", desc))
		}

		// Properties
		if props, ok := typeMap["properties"].(map[interface{}]interface{}); ok {
			b.WriteString("| Property | Type | Required | Description |\n")
			b.WriteString("|----------|------|----------|-------------|\n")

			for propName, propDef := range props {
				propMap, ok := propDef.(map[interface{}]interface{})
				if !ok {
					b.WriteString(fmt.Sprintf("| `%v` | `%v` | - | - |\n", propName, propDef))
					continue
				}

				propType := "any"
				if t, ok := propMap["type"]; ok {
					propType = fmt.Sprintf("%v", t)
				}

				required := "No"
				if r, ok := propMap["required"]; ok && r == true {
					required = "Yes"
				}

				desc := "-"
				if d, ok := propMap["description"]; ok {
					desc = fmt.Sprintf("%v", d)
				}

				b.WriteString(fmt.Sprintf("| `%v` | `%s` | %s | %s |\n", propName, propType, required, desc))
			}
			b.WriteString("\n")
		}

		// Example
		if example, ok := typeMap["example"]; ok {
			b.WriteString("**Example:**\n")
			b.WriteString("```json\n")
			exampleJSON, _ := json.MarshalIndent(example, "", "  ")
			b.WriteString(string(exampleJSON))
			b.WriteString("\n```\n\n")
		}
	}

	return strings.TrimSpace(b.String())
}

func (c *RAMLConverter) buildTraitsSection(spec *ramlSpec) string {
	var b strings.Builder

	for name, trait := range spec.Traits {
		b.WriteString(fmt.Sprintf("### %s\n\n", name))

		traitMap, ok := trait.(map[interface{}]interface{})
		if !ok {
			continue
		}

		if desc, ok := traitMap["description"]; ok {
			b.WriteString(fmt.Sprintf("%v\n\n", desc))
		}

		// Query parameters from trait
		if params, ok := traitMap["queryParameters"].(map[interface{}]interface{}); ok {
			b.WriteString("**Query Parameters:**\n")
			for pName, pDef := range params {
				pMap, _ := pDef.(map[interface{}]interface{})
				desc := ""
				if d, ok := pMap["description"]; ok {
					desc = fmt.Sprintf(": %v", d)
				}
				b.WriteString(fmt.Sprintf("- `%v`%s\n", pName, desc))
			}
			b.WriteString("\n")
		}
	}

	return strings.TrimSpace(b.String())
}

func (c *RAMLConverter) buildCodeExamples(spec *ramlSpec) string {
	var b strings.Builder

	// Get first endpoint for examples
	var endpoint string
	var method string
	for path, resource := range spec.Resources {
		endpoint = path
		for m := range resource.Methods {
			method = m
			break
		}
		break
	}

	fullURL := spec.BaseURI + endpoint

	b.WriteString("### cURL\n\n")
	b.WriteString("```bash\n")
	b.WriteString(fmt.Sprintf("curl -X %s \"%s\" \\\n", strings.ToUpper(method), fullURL))
	b.WriteString("  -H \"Authorization: Bearer YOUR_TOKEN\" \\\n")
	b.WriteString("  -H \"Content-Type: application/json\"\n")
	b.WriteString("```\n\n")

	b.WriteString("### JavaScript (fetch)\n\n")
	b.WriteString("```javascript\n")
	b.WriteString(fmt.Sprintf("const response = await fetch('%s', {\n", fullURL))
	b.WriteString(fmt.Sprintf("  method: '%s',\n", strings.ToUpper(method)))
	b.WriteString("  headers: {\n")
	b.WriteString("    'Authorization': `Bearer ${process.env.API_TOKEN}`,\n")
	b.WriteString("    'Content-Type': 'application/json',\n")
	b.WriteString("  },\n")
	b.WriteString("});\n\n")
	b.WriteString("const data = await response.json();\n")
	b.WriteString("```\n\n")

	b.WriteString("### Python (requests)\n\n")
	b.WriteString("```python\n")
	b.WriteString("import requests\nimport os\n\n")
	b.WriteString(fmt.Sprintf("response = requests.%s(\n", method))
	b.WriteString(fmt.Sprintf("    '%s',\n", fullURL))
	b.WriteString("    headers={\n")
	b.WriteString("        'Authorization': f'Bearer {os.environ[\"API_TOKEN\"]}',\n")
	b.WriteString("        'Content-Type': 'application/json',\n")
	b.WriteString("    },\n")
	b.WriteString(")\n\n")
	b.WriteString("data = response.json()\n")
	b.WriteString("```\n\n")

	b.WriteString("### Go (net/http)\n\n")
	b.WriteString("```go\n")
	b.WriteString("package main\n\n")
	b.WriteString("import (\n    \"net/http\"\n    \"os\"\n)\n\n")
	b.WriteString("func main() {\n")
	b.WriteString(fmt.Sprintf("    req, _ := http.NewRequest(\"%s\", \"%s\", nil)\n", strings.ToUpper(method), fullURL))
	b.WriteString("    req.Header.Set(\"Authorization\", \"Bearer \"+os.Getenv(\"API_TOKEN\"))\n")
	b.WriteString("    req.Header.Set(\"Content-Type\", \"application/json\")\n\n")
	b.WriteString("    resp, _ := http.DefaultClient.Do(req)\n")
	b.WriteString("    defer resp.Body.Close()\n")
	b.WriteString("}\n")
	b.WriteString("```\n")

	return strings.TrimSpace(b.String())
}

func (c *RAMLConverter) buildToolDefinitions(spec *ramlSpec) []skill.ToolDefinition {
	tools := make([]skill.ToolDefinition, 0)

	var processResource func(path string, resource *ramlResource)
	processResource = func(path string, resource *ramlResource) {
		for method, m := range resource.Methods {
			// Create tool name from path and method
			toolName := c.pathToToolName(method, path)

			desc := m.Description
			if desc == "" {
				desc = fmt.Sprintf("%s %s", strings.ToUpper(method), path)
			}

			params := map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}

			// Add path parameters
			pathParams := regexp.MustCompile(`\{(\w+)\}`).FindAllStringSubmatch(path, -1)
			props := params["properties"].(map[string]interface{})
			required := []string{}

			for _, match := range pathParams {
				paramName := match[1]
				props[paramName] = map[string]interface{}{
					"type":        "string",
					"description": fmt.Sprintf("Path parameter %s", paramName),
				}
				required = append(required, paramName)
			}

			// Add query parameters
			for qName, q := range m.QueryParams {
				props[qName] = map[string]interface{}{
					"type":        q.Type,
					"description": q.Description,
				}
				if q.Required {
					required = append(required, qName)
				}
			}

			// Add body parameter for POST/PUT/PATCH
			if method == "post" || method == "put" || method == "patch" {
				props["body"] = map[string]interface{}{
					"type":        "object",
					"description": "Request body",
				}
			}

			tools = append(tools, skill.ToolDefinition{
				Name:        toolName,
				Description: truncate(desc, 200),
				Parameters:  params,
				Required:    required,
			})
		}

		for subPath, subResource := range resource.SubResources {
			processResource(path+subPath, subResource)
		}
	}

	for path, resource := range spec.Resources {
		processResource(path, resource)
	}

	return tools
}

func (c *RAMLConverter) pathToToolName(method, path string) string {
	// Convert /users/{id}/orders to users_id_orders
	name := strings.ReplaceAll(path, "/", "_")
	name = strings.ReplaceAll(name, "{", "")
	name = strings.ReplaceAll(name, "}", "")
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.Trim(name, "_")
	return fmt.Sprintf("%s_%s", method, strings.ToLower(name))
}

func (c *RAMLConverter) buildToolDefinitionsSection(spec *ramlSpec) string {
	var b strings.Builder

	b.WriteString("MCP-compatible tool definitions for AI agents:\n\n")
	b.WriteString("```yaml\n")
	b.WriteString("tools:\n")

	tools := c.buildToolDefinitions(spec)
	for _, tool := range tools {
		b.WriteString(fmt.Sprintf("  - name: %s\n", tool.Name))
		b.WriteString(fmt.Sprintf("    description: %s\n", truncate(tool.Description, 60)))
		b.WriteString("    parameters:\n")
		b.WriteString("      type: object\n")
		if len(tool.Required) > 0 {
			b.WriteString("      required:\n")
			for _, req := range tool.Required {
				b.WriteString(fmt.Sprintf("        - %s\n", req))
			}
		}
	}

	b.WriteString("```\n")
	return strings.TrimSpace(b.String())
}

func (c *RAMLConverter) buildBestPractices() string {
	var b strings.Builder

	b.WriteString("### Request Handling\n\n")
	b.WriteString("- Always include appropriate Content-Type headers\n")
	b.WriteString("- Use URI templates for parameterized paths\n")
	b.WriteString("- Validate query parameters before sending\n")
	b.WriteString("- Handle pagination for collection endpoints\n\n")

	b.WriteString("### Error Handling\n\n")
	b.WriteString("- Check HTTP status codes in responses\n")
	b.WriteString("- Parse error messages from response body\n")
	b.WriteString("- Implement retry logic for 5xx errors\n")
	b.WriteString("- Log request/response for debugging\n\n")

	b.WriteString("### Authentication\n\n")
	b.WriteString("- Store credentials securely (environment variables)\n")
	b.WriteString("- Refresh OAuth tokens before expiration\n")
	b.WriteString("- Use appropriate auth method per endpoint\n\n")

	b.WriteString("### Performance\n\n")
	b.WriteString("- Use connection pooling for multiple requests\n")
	b.WriteString("- Implement caching for GET requests\n")
	b.WriteString("- Use compression when available\n")
	b.WriteString("- Batch requests when possible\n")

	return strings.TrimSpace(b.String())
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
