// Package converter provides spec-to-SKILL.md converters.
package converter

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/sanixdarker/skillforge/pkg/skill"
)

// APIBlueprintConverter converts API Blueprint specifications to skills.
type APIBlueprintConverter struct{}

// API Blueprint parsed types
type apibSpec struct {
	Format        string
	Host          string
	Name          string
	Description   string
	ResourceGroups []apibResourceGroup
	DataStructures []apibDataStructure
}

type apibResourceGroup struct {
	Name        string
	Description string
	Resources   []apibResource
}

type apibResource struct {
	Name        string
	Description string
	URI         string
	URIParams   []apibParam
	Actions     []apibAction
}

type apibAction struct {
	Method       string
	Name         string
	Description  string
	URIParams    []apibParam
	QueryParams  []apibParam
	Headers      []apibHeader
	Request      *apibPayload
	Responses    []apibResponse
}

type apibParam struct {
	Name        string
	Type        string
	Required    bool
	Default     string
	Example     string
	Description string
	Values      []string // enum values
}

type apibHeader struct {
	Name  string
	Value string
}

type apibPayload struct {
	ContentType string
	Body        string
	Schema      string
}

type apibResponse struct {
	StatusCode  int
	Description string
	ContentType string
	Headers     []apibHeader
	Body        string
	Schema      string
}

type apibDataStructure struct {
	Name       string
	Type       string
	Properties []apibProperty
}

type apibProperty struct {
	Name        string
	Type        string
	Required    bool
	Description string
	Example     string
}

func (c *APIBlueprintConverter) Name() string {
	return "apiblueprint"
}

func (c *APIBlueprintConverter) CanHandle(filename string, content []byte) bool {
	ext := getExtension(filename)
	if ext == ".apib" {
		return true
	}
	// Check for API Blueprint format marker
	return bytes.HasPrefix(content, []byte("FORMAT:")) ||
		bytes.Contains(content, []byte("# Group ")) ||
		(bytes.Contains(content, []byte("HOST:")) && bytes.Contains(content, []byte("## ")))
}

func (c *APIBlueprintConverter) Convert(content []byte, opts *Options) (*skill.Skill, error) {
	spec, err := c.parseAPIBlueprint(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse API Blueprint: %w", err)
	}

	return c.buildSkill(spec, opts), nil
}

func (c *APIBlueprintConverter) parseAPIBlueprint(content string) (*apibSpec, error) {
	spec := &apibSpec{
		ResourceGroups: []apibResourceGroup{},
		DataStructures: []apibDataStructure{},
	}

	lines := strings.Split(content, "\n")

	// Parse metadata
	metadataRe := regexp.MustCompile(`^(\w+):\s*(.+)$`)
	for _, line := range lines {
		if matches := metadataRe.FindStringSubmatch(line); len(matches) > 2 {
			switch strings.ToUpper(matches[1]) {
			case "FORMAT":
				spec.Format = matches[2]
			case "HOST":
				spec.Host = strings.TrimSpace(matches[2])
			}
		}
		if strings.HasPrefix(line, "#") {
			break // End of metadata
		}
	}

	// Parse title (first # heading)
	titleRe := regexp.MustCompile(`(?m)^#\s+(.+?)(?:\s*\[.+\])?$`)
	if matches := titleRe.FindStringSubmatch(content); len(matches) > 1 {
		spec.Name = strings.TrimSpace(matches[1])
	}

	// Parse description (text after title before first ## or # Group)
	descRe := regexp.MustCompile(`(?ms)^#\s+.+?\n\n(.+?)(?:\n##|\n# Group)`)
	if matches := descRe.FindStringSubmatch(content); len(matches) > 1 {
		spec.Description = strings.TrimSpace(matches[1])
	}

	// Parse resource groups
	groupRe := regexp.MustCompile(`(?m)^# Group (.+)$`)
	groupMatches := groupRe.FindAllStringSubmatchIndex(content, -1)

	if len(groupMatches) > 0 {
		for i, match := range groupMatches {
			start := match[0]
			end := len(content)
			if i+1 < len(groupMatches) {
				end = groupMatches[i+1][0]
			}

			groupContent := content[start:end]
			groupName := content[match[2]:match[3]]

			group := c.parseResourceGroup(groupName, groupContent)
			spec.ResourceGroups = append(spec.ResourceGroups, group)
		}
	} else {
		// No groups, parse resources directly
		defaultGroup := c.parseResourceGroup("API", content)
		if len(defaultGroup.Resources) > 0 {
			spec.ResourceGroups = append(spec.ResourceGroups, defaultGroup)
		}
	}

	// Parse data structures
	dsRe := regexp.MustCompile(`(?ms)# Data Structures\s*\n(.+?)(?:\n#[^#]|\z)`)
	if matches := dsRe.FindStringSubmatch(content); len(matches) > 1 {
		spec.DataStructures = c.parseDataStructures(matches[1])
	}

	return spec, nil
}

func (c *APIBlueprintConverter) parseResourceGroup(name, content string) apibResourceGroup {
	group := apibResourceGroup{
		Name:      name,
		Resources: []apibResource{},
	}

	// Parse group description
	descRe := regexp.MustCompile(`(?ms)^# Group .+?\n\n(.+?)(?:\n##|\z)`)
	if matches := descRe.FindStringSubmatch(content); len(matches) > 1 {
		group.Description = strings.TrimSpace(matches[1])
	}

	// Parse resources (## Resource Name [URI])
	resourceRe := regexp.MustCompile(`(?m)^## (.+?) \[(.+?)\]`)
	resourceMatches := resourceRe.FindAllStringSubmatchIndex(content, -1)

	for i, match := range resourceMatches {
		start := match[0]
		end := len(content)
		if i+1 < len(resourceMatches) {
			end = resourceMatches[i+1][0]
		}

		resourceContent := content[start:end]
		resourceName := content[match[2]:match[3]]
		resourceURI := content[match[4]:match[5]]

		resource := c.parseResource(resourceName, resourceURI, resourceContent)
		group.Resources = append(group.Resources, resource)
	}

	return group
}

func (c *APIBlueprintConverter) parseResource(name, uri, content string) apibResource {
	resource := apibResource{
		Name:      name,
		URI:       uri,
		URIParams: []apibParam{},
		Actions:   []apibAction{},
	}

	// Parse resource description
	descRe := regexp.MustCompile(`(?ms)^## .+?\n\n(.+?)(?:\n###|\n\+ Parameters|\z)`)
	if matches := descRe.FindStringSubmatch(content); len(matches) > 1 {
		desc := strings.TrimSpace(matches[1])
		if !strings.HasPrefix(desc, "+") {
			resource.Description = desc
		}
	}

	// Parse URI parameters
	paramsRe := regexp.MustCompile(`(?ms)\+ Parameters\s*\n(.+?)(?:\n###|\n##|\z)`)
	if matches := paramsRe.FindStringSubmatch(content); len(matches) > 1 {
		resource.URIParams = c.parseParameters(matches[1])
	}

	// Parse actions (### Action Name [METHOD URI])
	actionRe := regexp.MustCompile(`(?m)^### (.+?) \[(\w+)(?:\s+(.+?))?\]`)
	actionMatches := actionRe.FindAllStringSubmatchIndex(content, -1)

	for i, match := range actionMatches {
		start := match[0]
		end := len(content)
		if i+1 < len(actionMatches) {
			end = actionMatches[i+1][0]
		}

		actionContent := content[start:end]
		actionName := content[match[2]:match[3]]
		actionMethod := content[match[4]:match[5]]
		actionURI := ""
		if match[6] >= 0 && match[7] >= 0 {
			actionURI = content[match[6]:match[7]]
		}

		action := c.parseAction(actionName, actionMethod, actionURI, actionContent)
		resource.Actions = append(resource.Actions, action)
	}

	return resource
}

func (c *APIBlueprintConverter) parseAction(name, method, uri, content string) apibAction {
	action := apibAction{
		Name:        name,
		Method:      method,
		URIParams:   []apibParam{},
		QueryParams: []apibParam{},
		Headers:     []apibHeader{},
		Responses:   []apibResponse{},
	}

	if uri != "" {
		action.URIParams = c.extractURIParams(uri)
	}

	// Parse action description
	descRe := regexp.MustCompile(`(?ms)^### .+?\n\n(.+?)(?:\n\+ |\z)`)
	if matches := descRe.FindStringSubmatch(content); len(matches) > 1 {
		desc := strings.TrimSpace(matches[1])
		if !strings.HasPrefix(desc, "+") {
			action.Description = desc
		}
	}

	// Parse request
	reqRe := regexp.MustCompile(`(?ms)\+ Request(?: \((.+?)\))?\s*\n(.+?)(?:\n\+ Response|\n###|\z)`)
	if matches := reqRe.FindStringSubmatch(content); len(matches) > 2 {
		action.Request = &apibPayload{
			ContentType: matches[1],
			Body:        c.extractCodeBlock(matches[2]),
		}
	}

	// Parse responses
	respRe := regexp.MustCompile(`(?ms)\+ Response (\d+)(?: \((.+?)\))?\s*\n(.+?)(?:\n\+ Response|\n###|\n##|\z)`)
	respMatches := respRe.FindAllStringSubmatch(content, -1)
	for _, match := range respMatches {
		statusCode := 0
		fmt.Sscanf(match[1], "%d", &statusCode)

		response := apibResponse{
			StatusCode:  statusCode,
			ContentType: match[2],
			Body:        c.extractCodeBlock(match[3]),
		}
		action.Responses = append(action.Responses, response)
	}

	return action
}

func (c *APIBlueprintConverter) parseParameters(content string) []apibParam {
	params := []apibParam{}

	// Parse each parameter line: + name: `example` (type, required/optional) - description
	paramRe := regexp.MustCompile(`(?m)^\s*\+\s+(\w+)(?::\s*\x60?(.+?)\x60?)?\s*(?:\((.+?)\))?\s*(?:-\s*(.+))?$`)
	matches := paramRe.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		param := apibParam{
			Name:     match[1],
			Example:  strings.Trim(match[2], "`"),
			Required: true, // default
		}

		// Parse type and required/optional
		if match[3] != "" {
			parts := strings.Split(match[3], ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part == "optional" {
					param.Required = false
				} else if part == "required" {
					param.Required = true
				} else {
					param.Type = part
				}
			}
		}

		if match[4] != "" {
			param.Description = strings.TrimSpace(match[4])
		}

		params = append(params, param)
	}

	return params
}

func (c *APIBlueprintConverter) extractURIParams(uri string) []apibParam {
	params := []apibParam{}
	paramRe := regexp.MustCompile(`\{(\w+)\}`)
	matches := paramRe.FindAllStringSubmatch(uri, -1)
	for _, match := range matches {
		params = append(params, apibParam{
			Name:     match[1],
			Type:     "string",
			Required: true,
		})
	}
	return params
}

func (c *APIBlueprintConverter) extractCodeBlock(content string) string {
	// Look for code blocks
	codeRe := regexp.MustCompile("(?ms)```(?:\\w+)?\\n(.+?)```")
	if matches := codeRe.FindStringSubmatch(content); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Look for indented blocks
	indentRe := regexp.MustCompile(`(?ms)(?:^|\n)((?:[ ]{8,}|\t{2,}).+?)(?:\n\n|\z)`)
	if matches := indentRe.FindStringSubmatch(content); len(matches) > 1 {
		lines := strings.Split(matches[1], "\n")
		var result []string
		for _, line := range lines {
			result = append(result, strings.TrimPrefix(strings.TrimPrefix(line, "        "), "\t\t"))
		}
		return strings.TrimSpace(strings.Join(result, "\n"))
	}

	return ""
}

func (c *APIBlueprintConverter) parseDataStructures(content string) []apibDataStructure {
	structures := []apibDataStructure{}

	// Parse each data structure: ## Name (Type)
	structRe := regexp.MustCompile(`(?m)^## (\w+)(?: \((.+?)\))?`)
	structMatches := structRe.FindAllStringSubmatchIndex(content, -1)

	for i, match := range structMatches {
		start := match[0]
		end := len(content)
		if i+1 < len(structMatches) {
			end = structMatches[i+1][0]
		}

		structContent := content[start:end]
		structName := content[match[2]:match[3]]
		structType := ""
		if match[4] >= 0 && match[5] >= 0 {
			structType = content[match[4]:match[5]]
		}

		ds := apibDataStructure{
			Name:       structName,
			Type:       structType,
			Properties: c.parseProperties(structContent),
		}
		structures = append(structures, ds)
	}

	return structures
}

func (c *APIBlueprintConverter) parseProperties(content string) []apibProperty {
	props := []apibProperty{}

	// Parse MSON properties: + name: `example` (type, required) - description
	propRe := regexp.MustCompile(`(?m)^\s*\+\s+(\w+)(?::\s*\x60?(.+?)\x60?)?\s*(?:\((.+?)\))?\s*(?:-\s*(.+))?$`)
	matches := propRe.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		prop := apibProperty{
			Name:     match[1],
			Example:  strings.Trim(match[2], "`"),
			Required: true,
		}

		if match[3] != "" {
			parts := strings.Split(match[3], ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part == "optional" {
					prop.Required = false
				} else if part == "required" {
					prop.Required = true
				} else {
					prop.Type = part
				}
			}
		}

		if match[4] != "" {
			prop.Description = strings.TrimSpace(match[4])
		}

		props = append(props, prop)
	}

	return props
}

func (c *APIBlueprintConverter) buildSkill(spec *apibSpec, opts *Options) *skill.Skill {
	name := spec.Name
	if opts != nil && opts.Name != "" {
		name = opts.Name
	}
	if name == "" {
		name = "API Blueprint API"
	}

	description := spec.Description
	if description == "" {
		description = fmt.Sprintf("API documentation for %s", name)
	}

	s := skill.NewSkill(name, description)
	s.Frontmatter.SourceType = "apiblueprint"
	if opts != nil && opts.SourcePath != "" {
		s.Frontmatter.Source = opts.SourcePath
	}

	// Set metadata
	s.Frontmatter.Protocol = "http"
	s.Frontmatter.BaseURL = spec.Host
	s.Frontmatter.EndpointCount = c.countEndpoints(spec)
	s.Frontmatter.Difficulty = c.calculateDifficulty(spec)
	s.Frontmatter.HasExamples = true
	s.Frontmatter.Tags = []string{"api-blueprint", "rest", "api", "markdown"}

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

	for _, group := range spec.ResourceGroups {
		s.AddSection(group.Name, 2, c.buildResourceGroupSection(group, spec))
	}

	if len(spec.DataStructures) > 0 {
		s.AddSection("Data Structures", 2, c.buildDataStructuresSection(spec))
	}

	s.AddSection("Code Examples", 2, c.buildCodeExamples(spec))
	s.AddSection("Tool Definitions", 2, c.buildToolDefinitionsSection(spec))
	s.AddSection("Best Practices", 2, c.buildBestPractices())

	return s
}

func (c *APIBlueprintConverter) countEndpoints(spec *apibSpec) int {
	count := 0
	for _, group := range spec.ResourceGroups {
		for _, resource := range group.Resources {
			count += len(resource.Actions)
		}
	}
	return count
}

func (c *APIBlueprintConverter) calculateDifficulty(spec *apibSpec) string {
	endpoints := c.countEndpoints(spec)
	types := len(spec.DataStructures)
	complexity := endpoints + types

	if complexity <= 5 {
		return "novice"
	} else if complexity <= 15 {
		return "intermediate"
	}
	return "advanced"
}

func (c *APIBlueprintConverter) buildQuickStart(spec *apibSpec) string {
	var b strings.Builder

	b.WriteString("### Getting Started\n\n")

	if spec.Host != "" {
		b.WriteString(fmt.Sprintf("1. **Base URL**: `%s`\n", spec.Host))
	}
	b.WriteString("2. **Make requests** using the documented endpoints\n")
	b.WriteString("3. **Handle responses** according to status codes\n\n")

	// Quick example
	for _, group := range spec.ResourceGroups {
		for _, resource := range group.Resources {
			for _, action := range resource.Actions {
				fullURL := spec.Host + resource.URI
				b.WriteString("### Quick Example\n\n")
				b.WriteString("```bash\n")
				b.WriteString(fmt.Sprintf("curl -X %s \"%s\" \\\n", action.Method, fullURL))
				b.WriteString("  -H \"Content-Type: application/json\"\n")
				b.WriteString("```\n")
				return strings.TrimSpace(b.String())
			}
		}
	}

	return strings.TrimSpace(b.String())
}

func (c *APIBlueprintConverter) buildOverview(spec *apibSpec) string {
	var b strings.Builder

	if spec.Description != "" {
		b.WriteString(spec.Description)
		b.WriteString("\n\n")
	}

	b.WriteString("| Property | Value |\n")
	b.WriteString("|----------|-------|\n")

	if spec.Name != "" {
		b.WriteString(fmt.Sprintf("| **Name** | %s |\n", spec.Name))
	}
	if spec.Host != "" {
		b.WriteString(fmt.Sprintf("| **Host** | %s |\n", spec.Host))
	}
	if spec.Format != "" {
		b.WriteString(fmt.Sprintf("| **Format** | %s |\n", spec.Format))
	}
	b.WriteString(fmt.Sprintf("| **Resource Groups** | %d |\n", len(spec.ResourceGroups)))
	b.WriteString(fmt.Sprintf("| **Endpoints** | %d |\n", c.countEndpoints(spec)))

	if len(spec.DataStructures) > 0 {
		b.WriteString(fmt.Sprintf("| **Data Structures** | %d |\n", len(spec.DataStructures)))
	}

	return strings.TrimSpace(b.String())
}

func (c *APIBlueprintConverter) buildResourceGroupSection(group apibResourceGroup, spec *apibSpec) string {
	var b strings.Builder

	if group.Description != "" {
		b.WriteString(group.Description)
		b.WriteString("\n\n")
	}

	for _, resource := range group.Resources {
		b.WriteString(fmt.Sprintf("### %s `%s`\n\n", resource.Name, resource.URI))

		if resource.Description != "" {
			b.WriteString(resource.Description)
			b.WriteString("\n\n")
		}

		// URI Parameters
		if len(resource.URIParams) > 0 {
			b.WriteString("**URI Parameters:**\n\n")
			b.WriteString("| Name | Type | Required | Description |\n")
			b.WriteString("|------|------|----------|-------------|\n")
			for _, p := range resource.URIParams {
				req := "Yes"
				if !p.Required {
					req = "No"
				}
				desc := p.Description
				if desc == "" {
					desc = "-"
				}
				b.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n", p.Name, p.Type, req, desc))
			}
			b.WriteString("\n")
		}

		// Actions
		for _, action := range resource.Actions {
			c.writeAction(&b, action, resource, spec)
		}
	}

	return strings.TrimSpace(b.String())
}

func (c *APIBlueprintConverter) writeAction(b *strings.Builder, action apibAction, resource apibResource, spec *apibSpec) {
	b.WriteString(fmt.Sprintf("#### %s `%s`\n\n", action.Method, action.Name))

	if action.Description != "" {
		b.WriteString(action.Description)
		b.WriteString("\n\n")
	}

	// Request body example
	if action.Request != nil && action.Request.Body != "" {
		b.WriteString("**Request:**\n\n")
		if action.Request.ContentType != "" {
			b.WriteString(fmt.Sprintf("Content-Type: `%s`\n\n", action.Request.ContentType))
		}
		b.WriteString("```json\n")
		b.WriteString(action.Request.Body)
		b.WriteString("\n```\n\n")
	}

	// Responses
	if len(action.Responses) > 0 {
		b.WriteString("**Responses:**\n\n")
		for _, resp := range action.Responses {
			b.WriteString(fmt.Sprintf("**%d**", resp.StatusCode))
			if resp.Description != "" {
				b.WriteString(fmt.Sprintf(": %s", resp.Description))
			}
			b.WriteString("\n\n")

			if resp.Body != "" {
				if resp.ContentType != "" {
					b.WriteString(fmt.Sprintf("Content-Type: `%s`\n\n", resp.ContentType))
				}
				b.WriteString("```json\n")
				b.WriteString(resp.Body)
				b.WriteString("\n```\n\n")
			}
		}
	}
}

func (c *APIBlueprintConverter) buildDataStructuresSection(spec *apibSpec) string {
	var b strings.Builder

	// Sort by name
	names := make([]string, len(spec.DataStructures))
	dsMap := make(map[string]apibDataStructure)
	for i, ds := range spec.DataStructures {
		names[i] = ds.Name
		dsMap[ds.Name] = ds
	}
	sort.Strings(names)

	for _, name := range names {
		ds := dsMap[name]
		b.WriteString(fmt.Sprintf("### %s\n\n", ds.Name))

		if ds.Type != "" {
			b.WriteString(fmt.Sprintf("Type: `%s`\n\n", ds.Type))
		}

		if len(ds.Properties) > 0 {
			b.WriteString("| Property | Type | Required | Description |\n")
			b.WriteString("|----------|------|----------|-------------|\n")
			for _, p := range ds.Properties {
				req := "Yes"
				if !p.Required {
					req = "No"
				}
				desc := p.Description
				if desc == "" {
					desc = "-"
				}
				pType := p.Type
				if pType == "" {
					pType = "string"
				}
				b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s | %s |\n", p.Name, pType, req, desc))
			}
			b.WriteString("\n")
		}
	}

	return strings.TrimSpace(b.String())
}

func (c *APIBlueprintConverter) buildCodeExamples(spec *apibSpec) string {
	var b strings.Builder

	// Get first endpoint for examples
	var uri, method, body string
	for _, group := range spec.ResourceGroups {
		for _, resource := range group.Resources {
			uri = resource.URI
			for _, action := range resource.Actions {
				method = action.Method
				if action.Request != nil {
					body = action.Request.Body
				}
				break
			}
			break
		}
		break
	}

	fullURL := spec.Host + uri

	b.WriteString("### cURL\n\n")
	b.WriteString("```bash\n")
	b.WriteString(fmt.Sprintf("curl -X %s \"%s\" \\\n", method, fullURL))
	b.WriteString("  -H \"Content-Type: application/json\"")
	if body != "" && (method == "POST" || method == "PUT" || method == "PATCH") {
		b.WriteString(" \\\n  -d '")
		b.WriteString(strings.ReplaceAll(body, "\n", ""))
		b.WriteString("'")
	}
	b.WriteString("\n```\n\n")

	b.WriteString("### JavaScript (fetch)\n\n")
	b.WriteString("```javascript\n")
	b.WriteString(fmt.Sprintf("const response = await fetch('%s', {\n", fullURL))
	b.WriteString(fmt.Sprintf("  method: '%s',\n", method))
	b.WriteString("  headers: {\n")
	b.WriteString("    'Content-Type': 'application/json',\n")
	b.WriteString("  },\n")
	if body != "" && (method == "POST" || method == "PUT" || method == "PATCH") {
		b.WriteString(fmt.Sprintf("  body: JSON.stringify(%s),\n", body))
	}
	b.WriteString("});\n\n")
	b.WriteString("const data = await response.json();\n")
	b.WriteString("```\n\n")

	b.WriteString("### Python (requests)\n\n")
	b.WriteString("```python\n")
	b.WriteString("import requests\n\n")
	b.WriteString(fmt.Sprintf("response = requests.%s(\n", strings.ToLower(method)))
	b.WriteString(fmt.Sprintf("    '%s',\n", fullURL))
	b.WriteString("    headers={'Content-Type': 'application/json'},\n")
	if body != "" && (method == "POST" || method == "PUT" || method == "PATCH") {
		b.WriteString(fmt.Sprintf("    json=%s,\n", body))
	}
	b.WriteString(")\n\n")
	b.WriteString("data = response.json()\n")
	b.WriteString("```\n\n")

	b.WriteString("### Go (net/http)\n\n")
	b.WriteString("```go\n")
	b.WriteString("package main\n\n")
	b.WriteString("import (\n    \"net/http\"\n")
	if body != "" && (method == "POST" || method == "PUT" || method == "PATCH") {
		b.WriteString("    \"strings\"\n")
	}
	b.WriteString(")\n\n")
	b.WriteString("func main() {\n")
	if body != "" && (method == "POST" || method == "PUT" || method == "PATCH") {
		b.WriteString(fmt.Sprintf("    body := `%s`\n", body))
		b.WriteString(fmt.Sprintf("    req, _ := http.NewRequest(\"%s\", \"%s\", strings.NewReader(body))\n", method, fullURL))
	} else {
		b.WriteString(fmt.Sprintf("    req, _ := http.NewRequest(\"%s\", \"%s\", nil)\n", method, fullURL))
	}
	b.WriteString("    req.Header.Set(\"Content-Type\", \"application/json\")\n\n")
	b.WriteString("    resp, _ := http.DefaultClient.Do(req)\n")
	b.WriteString("    defer resp.Body.Close()\n")
	b.WriteString("}\n")
	b.WriteString("```\n")

	return strings.TrimSpace(b.String())
}

func (c *APIBlueprintConverter) buildToolDefinitions(spec *apibSpec) []skill.ToolDefinition {
	tools := make([]skill.ToolDefinition, 0)

	for _, group := range spec.ResourceGroups {
		for _, resource := range group.Resources {
			for _, action := range resource.Actions {
				toolName := c.pathToToolName(action.Method, resource.URI)

				desc := action.Description
				if desc == "" {
					desc = fmt.Sprintf("%s %s", action.Method, resource.URI)
				}

				params := map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				}

				props := params["properties"].(map[string]interface{})
				required := []string{}

				// URI parameters
				for _, p := range resource.URIParams {
					props[p.Name] = map[string]interface{}{
						"type":        p.Type,
						"description": p.Description,
					}
					if p.Required {
						required = append(required, p.Name)
					}
				}

				// Query parameters
				for _, p := range action.QueryParams {
					props[p.Name] = map[string]interface{}{
						"type":        p.Type,
						"description": p.Description,
					}
					if p.Required {
						required = append(required, p.Name)
					}
				}

				// Body parameter
				if action.Request != nil {
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
		}
	}

	return tools
}

func (c *APIBlueprintConverter) pathToToolName(method, path string) string {
	name := strings.ReplaceAll(path, "/", "_")
	name = strings.ReplaceAll(name, "{", "")
	name = strings.ReplaceAll(name, "}", "")
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.Trim(name, "_")
	return fmt.Sprintf("%s_%s", strings.ToLower(method), strings.ToLower(name))
}

func (c *APIBlueprintConverter) buildToolDefinitionsSection(spec *apibSpec) string {
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

func (c *APIBlueprintConverter) buildBestPractices() string {
	var b strings.Builder

	b.WriteString("### Request Handling\n\n")
	b.WriteString("- Use appropriate HTTP methods for operations\n")
	b.WriteString("- Include Content-Type headers for requests with bodies\n")
	b.WriteString("- Handle URI parameters correctly\n")
	b.WriteString("- Validate request data before sending\n\n")

	b.WriteString("### Error Handling\n\n")
	b.WriteString("- Check HTTP status codes in responses\n")
	b.WriteString("- Parse error messages from response bodies\n")
	b.WriteString("- Implement retry logic for transient failures\n")
	b.WriteString("- Log request/response for debugging\n\n")

	b.WriteString("### Performance\n\n")
	b.WriteString("- Use connection pooling for multiple requests\n")
	b.WriteString("- Implement caching for GET requests\n")
	b.WriteString("- Use compression when available\n")
	b.WriteString("- Set appropriate timeouts\n")

	return strings.TrimSpace(b.String())
}
