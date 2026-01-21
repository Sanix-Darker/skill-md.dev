package converter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sanixdarker/skill-md/pkg/skill"
)

// PostmanConverter converts Postman collections to SKILL.md.
type PostmanConverter struct{}

func (c *PostmanConverter) Name() string {
	return "postman"
}

func (c *PostmanConverter) CanHandle(filename string, content []byte) bool {
	ext := getExtension(filename)
	if ext != ".json" {
		return false
	}
	// Check for Postman collection indicators
	return bytes.Contains(content, []byte(`"_postman_id"`)) ||
		bytes.Contains(content, []byte(`"schema": "https://schema.getpostman.com`))
}

// PostmanCollection represents a Postman collection.
type PostmanCollection struct {
	Info      PostmanInfo      `json:"info"`
	Items     []PostmanItem    `json:"item"`
	Variables []PostmanVar     `json:"variable,omitempty"`
	Auth      *PostmanAuth     `json:"auth,omitempty"`
}

type PostmanInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Schema      string `json:"schema"`
}

type PostmanItem struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Request     *PostmanReq   `json:"request,omitempty"`
	Items       []PostmanItem `json:"item,omitempty"`
	Response    []PostmanResp `json:"response,omitempty"`
}

type PostmanReq struct {
	Method      string          `json:"method"`
	URL         PostmanURL      `json:"url"`
	Description string          `json:"description"`
	Header      []PostmanHeader `json:"header"`
	Body        *PostmanBody    `json:"body,omitempty"`
	Auth        *PostmanAuth    `json:"auth,omitempty"`
}

type PostmanURL struct {
	Raw   string   `json:"raw"`
	Host  []string `json:"host"`
	Path  []string `json:"path"`
	Query []struct {
		Key         string `json:"key"`
		Value       string `json:"value"`
		Description string `json:"description"`
		Disabled    bool   `json:"disabled,omitempty"`
	} `json:"query"`
}

type PostmanHeader struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Disabled    bool   `json:"disabled,omitempty"`
}

type PostmanBody struct {
	Mode       string            `json:"mode"`
	Raw        string            `json:"raw"`
	Options    *PostmanBodyOpts  `json:"options,omitempty"`
	FormData   []PostmanFormData `json:"formdata,omitempty"`
	URLEncoded []PostmanFormData `json:"urlencoded,omitempty"`
}

type PostmanBodyOpts struct {
	Raw struct {
		Language string `json:"language"`
	} `json:"raw"`
}

type PostmanFormData struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

type PostmanAuth struct {
	Type   string           `json:"type"`
	Bearer []PostmanAuthKV  `json:"bearer,omitempty"`
	Basic  []PostmanAuthKV  `json:"basic,omitempty"`
	ApiKey []PostmanAuthKV  `json:"apikey,omitempty"`
	OAuth2 []PostmanAuthKV  `json:"oauth2,omitempty"`
}

type PostmanAuthKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

type PostmanVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type PostmanResp struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Code   int    `json:"code"`
	Body   string `json:"body"`
}

func (c *PostmanConverter) Convert(content []byte, opts *Options) (*skill.Skill, error) {
	var collection PostmanCollection
	if err := json.Unmarshal(content, &collection); err != nil {
		return nil, fmt.Errorf("failed to parse Postman collection: %w", err)
	}

	s := c.buildSkill(&collection, opts)
	return s, nil
}

func (c *PostmanConverter) buildSkill(col *PostmanCollection, opts *Options) *skill.Skill {
	name := col.Info.Name
	if opts != nil && opts.Name != "" {
		name = opts.Name
	}

	description := col.Info.Description
	if description == "" {
		description = "API collection converted from Postman"
	}

	s := skill.NewSkill(name, description)
	s.Frontmatter.SourceType = "postman"
	s.Frontmatter.Tags = []string{"api", "postman"}
	if opts != nil && opts.SourcePath != "" {
		s.Frontmatter.Source = opts.SourcePath
	}

	// Count endpoints
	endpointCount := c.countEndpoints(col.Items)
	s.Frontmatter.EndpointCount = endpointCount

	// Extract auth methods
	if col.Auth != nil {
		s.Frontmatter.AuthMethods = []string{col.Auth.Type}
	}

	// Determine difficulty
	if endpointCount <= 5 {
		s.Frontmatter.Difficulty = "novice"
	} else if endpointCount <= 20 {
		s.Frontmatter.Difficulty = "intermediate"
	} else {
		s.Frontmatter.Difficulty = "advanced"
	}

	// Check for examples (responses)
	s.Frontmatter.HasExamples = c.hasExamples(col.Items)

	// Add Quick Start section
	s.AddSection("Quick Start", 2, c.buildQuickStart(col))

	// Add overview
	s.AddSection("Overview", 2, c.buildOverview(col))

	// Add authentication section if auth is present
	if col.Auth != nil {
		s.AddSection("Authentication", 2, c.buildAuthSection(col.Auth))
	}

	// Add variables section if present
	if len(col.Variables) > 0 {
		s.AddSection("Variables", 2, c.buildVariablesSection(col.Variables))
	}

	// Add endpoints
	s.AddSection("Endpoints", 2, c.buildEndpointsSection(col.Items, 0, col))

	// Add best practices
	s.AddSection("Best Practices", 2, c.buildBestPracticesSection())

	return s
}

func (c *PostmanConverter) countEndpoints(items []PostmanItem) int {
	count := 0
	for _, item := range items {
		if item.Request != nil {
			count++
		}
		if len(item.Items) > 0 {
			count += c.countEndpoints(item.Items)
		}
	}
	return count
}

func (c *PostmanConverter) hasExamples(items []PostmanItem) bool {
	for _, item := range items {
		if len(item.Response) > 0 {
			return true
		}
		if len(item.Items) > 0 && c.hasExamples(item.Items) {
			return true
		}
	}
	return false
}

func (c *PostmanConverter) buildQuickStart(col *PostmanCollection) string {
	var b strings.Builder

	b.WriteString("Get started with this API collection quickly.\n\n")

	// Step 1: Base URL
	b.WriteString("### 1. Set Your Base URL\n\n")
	baseURL := c.extractBaseURL(col)
	if baseURL != "" {
		b.WriteString(fmt.Sprintf("```\n%s\n```\n\n", baseURL))
	} else {
		b.WriteString("```\nhttps://api.example.com\n```\n\n")
	}

	// Step 2: Authentication
	if col.Auth != nil {
		b.WriteString("### 2. Authenticate\n\n")
		switch col.Auth.Type {
		case "bearer":
			b.WriteString("Add your Bearer token to all requests:\n\n")
			b.WriteString("```\nAuthorization: Bearer YOUR_TOKEN\n```\n\n")
		case "basic":
			b.WriteString("Use HTTP Basic authentication:\n\n")
			b.WriteString("```\nAuthorization: Basic BASE64_ENCODED_CREDENTIALS\n```\n\n")
		case "apikey":
			b.WriteString("Add your API key to requests:\n\n")
			b.WriteString("```\nX-API-Key: YOUR_API_KEY\n```\n\n")
		case "oauth2":
			b.WriteString("Configure OAuth 2.0 authentication. See the Authentication section for details.\n\n")
		}
	}

	// Step 3: Make first request
	b.WriteString("### 3. Make Your First Request\n\n")
	firstReq := c.findFirstRequest(col.Items)
	if firstReq != nil && firstReq.Request != nil {
		req := firstReq.Request
		url := req.URL.Raw
		if url == "" && len(req.URL.Host) > 0 {
			url = strings.Join(req.URL.Host, ".") + "/" + strings.Join(req.URL.Path, "/")
		}

		// cURL example
		b.WriteString("**cURL:**\n")
		b.WriteString("```bash\n")
		b.WriteString(fmt.Sprintf("curl -X %s \"%s\" \\\n", req.Method, url))
		b.WriteString("  -H \"Accept: application/json\"")
		if req.Method == "POST" || req.Method == "PUT" || req.Method == "PATCH" {
			b.WriteString(" \\\n  -H \"Content-Type: application/json\"")
			if req.Body != nil && req.Body.Raw != "" {
				// Escape for shell
				body := strings.ReplaceAll(req.Body.Raw, "'", "'\\''")
				if len(body) > 100 {
					body = body[:100] + "..."
				}
				b.WriteString(fmt.Sprintf(" \\\n  -d '%s'", body))
			}
		}
		b.WriteString("\n```\n\n")

		// JavaScript example
		b.WriteString("**JavaScript (fetch):**\n")
		b.WriteString("```javascript\n")
		b.WriteString(fmt.Sprintf("const response = await fetch('%s', {\n", url))
		b.WriteString(fmt.Sprintf("  method: '%s',\n", req.Method))
		b.WriteString("  headers: { 'Accept': 'application/json' }\n")
		b.WriteString("});\n")
		b.WriteString("const data = await response.json();\n")
		b.WriteString("```\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *PostmanConverter) extractBaseURL(col *PostmanCollection) string {
	// Try to get from variables
	for _, v := range col.Variables {
		if v.Key == "baseUrl" || v.Key == "base_url" || v.Key == "baseURL" {
			return v.Value
		}
	}
	// Try to extract from first request
	firstReq := c.findFirstRequest(col.Items)
	if firstReq != nil && firstReq.Request != nil && len(firstReq.Request.URL.Host) > 0 {
		return "https://" + strings.Join(firstReq.Request.URL.Host, ".")
	}
	return ""
}

func (c *PostmanConverter) findFirstRequest(items []PostmanItem) *PostmanItem {
	for i := range items {
		if items[i].Request != nil {
			return &items[i]
		}
		if len(items[i].Items) > 0 {
			if found := c.findFirstRequest(items[i].Items); found != nil {
				return found
			}
		}
	}
	return nil
}

func (c *PostmanConverter) buildOverview(col *PostmanCollection) string {
	var b strings.Builder

	if col.Info.Description != "" {
		b.WriteString(col.Info.Description)
		b.WriteString("\n\n")
	}

	// Collection statistics
	endpointCount := c.countEndpoints(col.Items)
	folderCount := c.countFolders(col.Items)

	b.WriteString("### Collection Statistics\n\n")
	b.WriteString("| Metric | Count |\n")
	b.WriteString("|--------|-------|\n")
	b.WriteString(fmt.Sprintf("| Endpoints | %d |\n", endpointCount))
	if folderCount > 0 {
		b.WriteString(fmt.Sprintf("| Folders | %d |\n", folderCount))
	}
	if len(col.Variables) > 0 {
		b.WriteString(fmt.Sprintf("| Variables | %d |\n", len(col.Variables)))
	}
	b.WriteString("\n")

	// List folder structure
	if folderCount > 0 {
		b.WriteString("### Structure\n\n")
		for _, item := range col.Items {
			if len(item.Items) > 0 {
				reqCount := c.countEndpoints(item.Items)
				b.WriteString(fmt.Sprintf("- **%s** (%d endpoints)\n", item.Name, reqCount))
			}
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *PostmanConverter) countFolders(items []PostmanItem) int {
	count := 0
	for _, item := range items {
		if len(item.Items) > 0 {
			count++
			count += c.countFolders(item.Items)
		}
	}
	return count
}

func (c *PostmanConverter) buildAuthSection(auth *PostmanAuth) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("**Type**: `%s`\n\n", auth.Type))

	switch auth.Type {
	case "bearer":
		b.WriteString("Add the Bearer token to the Authorization header.\n\n")
		b.WriteString("**Example**:\n")
		b.WriteString("```bash\n")
		b.WriteString("curl -H \"Authorization: Bearer YOUR_TOKEN\" \\\n")
		b.WriteString("     https://api.example.com/resource\n")
		b.WriteString("```\n\n")

		b.WriteString("**JavaScript**:\n")
		b.WriteString("```javascript\n")
		b.WriteString("fetch('/api/resource', {\n")
		b.WriteString("  headers: {\n")
		b.WriteString("    'Authorization': `Bearer ${token}`\n")
		b.WriteString("  }\n")
		b.WriteString("});\n")
		b.WriteString("```\n")

	case "basic":
		b.WriteString("Use HTTP Basic authentication with username and password.\n\n")
		b.WriteString("**Example**:\n")
		b.WriteString("```bash\n")
		b.WriteString("curl -u username:password https://api.example.com/resource\n")
		b.WriteString("```\n\n")

		b.WriteString("**JavaScript**:\n")
		b.WriteString("```javascript\n")
		b.WriteString("const credentials = btoa(`${username}:${password}`);\n")
		b.WriteString("fetch('/api/resource', {\n")
		b.WriteString("  headers: {\n")
		b.WriteString("    'Authorization': `Basic ${credentials}`\n")
		b.WriteString("  }\n")
		b.WriteString("});\n")
		b.WriteString("```\n")

	case "apikey":
		keyName := "X-API-Key"
		keyIn := "header"
		for _, kv := range auth.ApiKey {
			if kv.Key == "key" {
				keyName = kv.Value
			}
			if kv.Key == "in" {
				keyIn = kv.Value
			}
		}
		b.WriteString(fmt.Sprintf("Add the API key as `%s` in the %s.\n\n", keyName, keyIn))
		b.WriteString("**Example**:\n")
		if keyIn == "header" {
			b.WriteString("```bash\n")
			b.WriteString(fmt.Sprintf("curl -H \"%s: your-api-key\" https://api.example.com/resource\n", keyName))
			b.WriteString("```\n")
		} else {
			b.WriteString("```bash\n")
			b.WriteString(fmt.Sprintf("curl \"https://api.example.com/resource?%s=your-api-key\"\n", keyName))
			b.WriteString("```\n")
		}

	case "oauth2":
		b.WriteString("OAuth 2.0 authentication flow.\n\n")
		b.WriteString("**Steps**:\n")
		b.WriteString("1. Obtain access token from authorization server\n")
		b.WriteString("2. Include token in Authorization header\n")
		b.WriteString("3. Refresh token when expired\n\n")
		b.WriteString("**Example**:\n")
		b.WriteString("```javascript\n")
		b.WriteString("// After obtaining token from OAuth flow\n")
		b.WriteString("fetch('/api/resource', {\n")
		b.WriteString("  headers: {\n")
		b.WriteString("    'Authorization': `Bearer ${accessToken}`\n")
		b.WriteString("  }\n")
		b.WriteString("});\n")
		b.WriteString("```\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *PostmanConverter) buildVariablesSection(vars []PostmanVar) string {
	var b strings.Builder

	b.WriteString("Collection variables that can be used in requests.\n\n")
	b.WriteString("| Variable | Value | Usage |\n")
	b.WriteString("|----------|-------|-------|\n")

	for _, v := range vars {
		value := v.Value
		if value == "" {
			value = "(empty)"
		}
		// Mask sensitive values
		if strings.Contains(strings.ToLower(v.Key), "key") ||
			strings.Contains(strings.ToLower(v.Key), "secret") ||
			strings.Contains(strings.ToLower(v.Key), "password") ||
			strings.Contains(strings.ToLower(v.Key), "token") {
			if len(value) > 4 {
				value = value[:4] + "****"
			}
		}
		b.WriteString(fmt.Sprintf("| `%s` | `%s` | `{{%s}}` |\n", v.Key, value, v.Key))
	}

	b.WriteString("\n")
	b.WriteString("**Usage**: Replace `{{variableName}}` in URLs and request bodies with actual values.\n")

	return strings.TrimSpace(b.String())
}

func (c *PostmanConverter) buildEndpointsSection(items []PostmanItem, depth int, col *PostmanCollection) string {
	var b strings.Builder

	for _, item := range items {
		if item.Request != nil {
			b.WriteString(c.buildRequestSection(&item, depth, col))
		} else if len(item.Items) > 0 {
			// Folder
			level := 3 + depth
			if level > 6 {
				level = 6
			}
			b.WriteString(fmt.Sprintf("%s %s\n\n", strings.Repeat("#", level), item.Name))
			if item.Description != "" {
				b.WriteString(item.Description)
				b.WriteString("\n\n")
			}
			b.WriteString(c.buildEndpointsSection(item.Items, depth+1, col))
		}
	}

	return strings.TrimSpace(b.String())
}

func (c *PostmanConverter) buildRequestSection(item *PostmanItem, depth int, col *PostmanCollection) string {
	var b strings.Builder

	level := 3 + depth
	if level > 6 {
		level = 6
	}

	req := item.Request
	path := "/" + strings.Join(req.URL.Path, "/")

	b.WriteString(fmt.Sprintf("%s %s %s\n\n", strings.Repeat("#", level), req.Method, path))

	if item.Description != "" {
		b.WriteString(item.Description)
		b.WriteString("\n\n")
	} else if req.Description != "" {
		b.WriteString(req.Description)
		b.WriteString("\n\n")
	}

	// URL
	if req.URL.Raw != "" {
		b.WriteString(fmt.Sprintf("**URL**: `%s`\n\n", req.URL.Raw))
	}

	// Query parameters (excluding disabled)
	activeQuery := []struct {
		Key         string
		Value       string
		Description string
	}{}
	for _, q := range req.URL.Query {
		if !q.Disabled {
			activeQuery = append(activeQuery, struct {
				Key         string
				Value       string
				Description string
			}{q.Key, q.Value, q.Description})
		}
	}

	if len(activeQuery) > 0 {
		b.WriteString("**Query Parameters**:\n\n")
		b.WriteString("| Name | Value | Description |\n")
		b.WriteString("|------|-------|-------------|\n")
		for _, q := range activeQuery {
			desc := strings.ReplaceAll(q.Description, "\n", " ")
			b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n", q.Key, q.Value, desc))
		}
		b.WriteString("\n")
	}

	// Headers (excluding disabled)
	activeHeaders := []PostmanHeader{}
	for _, h := range req.Header {
		if !h.Disabled {
			activeHeaders = append(activeHeaders, h)
		}
	}

	if len(activeHeaders) > 0 {
		b.WriteString("**Headers**:\n\n")
		b.WriteString("| Name | Value | Description |\n")
		b.WriteString("|------|-------|-------------|\n")
		for _, h := range activeHeaders {
			desc := strings.ReplaceAll(h.Description, "\n", " ")
			value := h.Value
			// Mask authorization values
			if strings.EqualFold(h.Key, "authorization") && len(value) > 10 {
				value = value[:10] + "..."
			}
			b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n", h.Key, value, desc))
		}
		b.WriteString("\n")
	}

	// Body
	if req.Body != nil {
		b.WriteString("**Request Body**:\n\n")

		switch req.Body.Mode {
		case "raw":
			lang := "json"
			if req.Body.Options != nil && req.Body.Options.Raw.Language != "" {
				lang = req.Body.Options.Raw.Language
			}
			b.WriteString(fmt.Sprintf("```%s\n", lang))
			body := req.Body.Raw
			// Pretty print JSON if possible
			if lang == "json" {
				var parsed interface{}
				if err := json.Unmarshal([]byte(body), &parsed); err == nil {
					if pretty, err := json.MarshalIndent(parsed, "", "  "); err == nil {
						body = string(pretty)
					}
				}
			}
			b.WriteString(body)
			b.WriteString("\n```\n\n")

		case "formdata":
			b.WriteString("**Form Data**:\n\n")
			b.WriteString("| Field | Value | Type |\n")
			b.WriteString("|-------|-------|------|\n")
			for _, f := range req.Body.FormData {
				b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n", f.Key, f.Value, f.Type))
			}
			b.WriteString("\n")

		case "urlencoded":
			b.WriteString("**URL Encoded**:\n\n")
			b.WriteString("| Field | Value |\n")
			b.WriteString("|-------|-------|\n")
			for _, f := range req.Body.URLEncoded {
				b.WriteString(fmt.Sprintf("| `%s` | `%s` |\n", f.Key, f.Value))
			}
			b.WriteString("\n")
		}
	}

	// Example responses
	if len(item.Response) > 0 {
		b.WriteString("**Example Responses**:\n\n")
		for _, resp := range item.Response {
			b.WriteString(fmt.Sprintf("<details>\n<summary>%s (%d %s)</summary>\n\n",
				resp.Name, resp.Code, resp.Status))
			if resp.Body != "" {
				b.WriteString("```json\n")
				// Try to pretty print
				var parsed interface{}
				if err := json.Unmarshal([]byte(resp.Body), &parsed); err == nil {
					if pretty, err := json.MarshalIndent(parsed, "", "  "); err == nil {
						b.WriteString(string(pretty))
					} else {
						b.WriteString(resp.Body)
					}
				} else {
					b.WriteString(resp.Body)
				}
				b.WriteString("\n```\n")
			}
			b.WriteString("\n</details>\n\n")
		}
	}

	// Code examples
	b.WriteString(c.buildCodeExamples(req, col))

	return b.String()
}

func (c *PostmanConverter) buildCodeExamples(req *PostmanReq, col *PostmanCollection) string {
	var b strings.Builder

	url := req.URL.Raw
	if url == "" && len(req.URL.Host) > 0 {
		url = "https://" + strings.Join(req.URL.Host, ".") + "/" + strings.Join(req.URL.Path, "/")
	}

	b.WriteString("**Code Examples**:\n\n")

	// cURL
	b.WriteString("<details>\n<summary>cURL</summary>\n\n")
	b.WriteString("```bash\n")
	b.WriteString(fmt.Sprintf("curl -X %s \"%s\"", req.Method, url))
	for _, h := range req.Header {
		if !h.Disabled {
			b.WriteString(fmt.Sprintf(" \\\n  -H \"%s: %s\"", h.Key, h.Value))
		}
	}
	if req.Body != nil && req.Body.Mode == "raw" && req.Body.Raw != "" {
		b.WriteString(" \\\n  -d @- << 'EOF'\n")
		b.WriteString(req.Body.Raw)
		b.WriteString("\nEOF")
	}
	b.WriteString("\n```\n\n")
	b.WriteString("</details>\n\n")

	// JavaScript
	b.WriteString("<details>\n<summary>JavaScript</summary>\n\n")
	b.WriteString("```javascript\n")
	b.WriteString(fmt.Sprintf("const response = await fetch('%s', {\n", url))
	b.WriteString(fmt.Sprintf("  method: '%s',\n", req.Method))

	if len(req.Header) > 0 || req.Body != nil {
		b.WriteString("  headers: {\n")
		for _, h := range req.Header {
			if !h.Disabled {
				b.WriteString(fmt.Sprintf("    '%s': '%s',\n", h.Key, h.Value))
			}
		}
		if req.Body != nil && req.Body.Mode == "raw" {
			b.WriteString("    'Content-Type': 'application/json',\n")
		}
		b.WriteString("  },\n")
	}

	if req.Body != nil && req.Body.Mode == "raw" && req.Body.Raw != "" {
		b.WriteString("  body: JSON.stringify(")
		var parsed interface{}
		if err := json.Unmarshal([]byte(req.Body.Raw), &parsed); err == nil {
			b.WriteString(req.Body.Raw)
		} else {
			b.WriteString("{ /* request body */ }")
		}
		b.WriteString(")\n")
	}

	b.WriteString("});\n")
	b.WriteString("const data = await response.json();\n")
	b.WriteString("```\n\n")
	b.WriteString("</details>\n\n")

	// Python
	b.WriteString("<details>\n<summary>Python</summary>\n\n")
	b.WriteString("```python\n")
	b.WriteString("import requests\n\n")

	if len(req.Header) > 0 {
		b.WriteString("headers = {\n")
		for _, h := range req.Header {
			if !h.Disabled {
				b.WriteString(fmt.Sprintf("    '%s': '%s',\n", h.Key, h.Value))
			}
		}
		b.WriteString("}\n\n")
	}

	if req.Body != nil && req.Body.Mode == "raw" && req.Body.Raw != "" {
		b.WriteString(fmt.Sprintf("response = requests.%s(\n", strings.ToLower(req.Method)))
		b.WriteString(fmt.Sprintf("    '%s',\n", url))
		if len(req.Header) > 0 {
			b.WriteString("    headers=headers,\n")
		}
		b.WriteString("    json=")
		var parsed interface{}
		if err := json.Unmarshal([]byte(req.Body.Raw), &parsed); err == nil {
			b.WriteString(req.Body.Raw)
		} else {
			b.WriteString("{ }")
		}
		b.WriteString("\n)\n")
	} else {
		if len(req.Header) > 0 {
			b.WriteString(fmt.Sprintf("response = requests.%s('%s', headers=headers)\n",
				strings.ToLower(req.Method), url))
		} else {
			b.WriteString(fmt.Sprintf("response = requests.%s('%s')\n",
				strings.ToLower(req.Method), url))
		}
	}
	b.WriteString("data = response.json()\n")
	b.WriteString("```\n\n")
	b.WriteString("</details>\n\n")

	return b.String()
}

func (c *PostmanConverter) buildBestPracticesSection() string {
	var b strings.Builder

	b.WriteString("Follow these recommendations for optimal API usage.\n\n")

	b.WriteString("### Using Postman Variables\n\n")
	b.WriteString("- Set up environment variables for different environments (dev, staging, prod)\n")
	b.WriteString("- Use collection variables for shared values\n")
	b.WriteString("- Store sensitive data in environment variables, not in collection\n\n")

	b.WriteString("### Testing Requests\n\n")
	b.WriteString("```javascript\n")
	b.WriteString("// In Postman Tests tab\n")
	b.WriteString("pm.test('Status is 200', () => {\n")
	b.WriteString("  pm.response.to.have.status(200);\n")
	b.WriteString("});\n\n")
	b.WriteString("pm.test('Response has data', () => {\n")
	b.WriteString("  const json = pm.response.json();\n")
	b.WriteString("  pm.expect(json).to.have.property('data');\n")
	b.WriteString("});\n")
	b.WriteString("```\n\n")

	b.WriteString("### Error Handling\n\n")
	b.WriteString("- Check response status codes before processing data\n")
	b.WriteString("- Handle rate limiting (429) with exponential backoff\n")
	b.WriteString("- Log error responses for debugging\n\n")

	b.WriteString("### Authentication\n\n")
	b.WriteString("- Store tokens securely\n")
	b.WriteString("- Refresh tokens before they expire\n")
	b.WriteString("- Use environment-specific credentials\n")

	return strings.TrimSpace(b.String())
}
