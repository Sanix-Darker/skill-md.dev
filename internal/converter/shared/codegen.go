// Package shared provides common utilities for spec-to-SKILL.md converters.
package shared

import (
	"fmt"
	"strings"
)

// QuickStartConfig holds configuration for generating Quick Start sections.
type QuickStartConfig struct {
	Protocol    string   // e.g., "REST", "gRPC", "Kafka", "MQTT"
	BaseURL     string   // Base URL or server address
	AuthHeader  string   // e.g., "Authorization"
	AuthExample string   // e.g., "Bearer YOUR_TOKEN"
	ContentType string   // e.g., "application/json"
	Steps       []string // Custom steps (optional)
}

// GenerateQuickStart generates a standardized Quick Start section.
func GenerateQuickStart(cfg QuickStartConfig) string {
	var b strings.Builder

	b.WriteString("### Getting Started\n\n")

	if len(cfg.Steps) > 0 {
		for i, step := range cfg.Steps {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
		}
	} else {
		b.WriteString(fmt.Sprintf("1. **Base URL**: `%s`\n", cfg.BaseURL))
		if cfg.AuthHeader != "" {
			b.WriteString(fmt.Sprintf("2. **Authenticate** using `%s: %s`\n", cfg.AuthHeader, cfg.AuthExample))
			b.WriteString("3. **Make requests** to the API endpoints\n")
		} else {
			b.WriteString("2. **Make requests** to the API endpoints\n")
		}
	}

	return strings.TrimSpace(b.String())
}

// BestPracticesConfig holds configuration for best practices sections.
type BestPracticesConfig struct {
	Protocol string // REST, gRPC, Kafka, MQTT, AMQP, SOAP, WebSocket
	Custom   []BestPractice
}

// BestPractice represents a single best practice item.
type BestPractice struct {
	Category string
	Items    []string
}

// GenerateBestPractices generates protocol-specific best practices.
func GenerateBestPractices(protocol string) string {
	practices := getBestPracticesForProtocol(protocol)

	var b strings.Builder
	for _, p := range practices {
		b.WriteString(fmt.Sprintf("### %s\n\n", p.Category))
		for _, item := range p.Items {
			b.WriteString(fmt.Sprintf("- %s\n", item))
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func getBestPracticesForProtocol(protocol string) []BestPractice {
	common := []BestPractice{
		{
			Category: "Error Handling",
			Items: []string{
				"Implement proper error handling for all operations",
				"Use exponential backoff for retries",
				"Log errors with sufficient context for debugging",
			},
		},
		{
			Category: "Performance",
			Items: []string{
				"Reuse connections when possible",
				"Implement appropriate timeouts",
				"Consider caching where applicable",
			},
		},
	}

	switch strings.ToLower(protocol) {
	case "rest", "http", "openapi":
		return append(common, []BestPractice{
			{
				Category: "Request Handling",
				Items: []string{
					"Use appropriate HTTP methods (GET, POST, PUT, DELETE)",
					"Include Content-Type headers for requests with bodies",
					"Handle pagination for collection endpoints",
					"Validate input data before sending",
				},
			},
			{
				Category: "Security",
				Items: []string{
					"Store credentials securely (environment variables)",
					"Use HTTPS for all production requests",
					"Implement proper token refresh logic",
				},
			},
		}...)
	case "grpc", "proto", "protobuf":
		return append(common, []BestPractice{
			{
				Category: "gRPC Best Practices",
				Items: []string{
					"Use deadlines/timeouts on all calls",
					"Reuse channels and stubs when possible",
					"Handle streaming cancellation properly",
					"Consider message size limits (default 4MB)",
				},
			},
			{
				Category: "Schema Evolution",
				Items: []string{
					"Never change field numbers",
					"Mark deprecated fields with [deprecated = true]",
					"Use reserved for removed fields",
					"Add new fields as optional",
				},
			},
		}...)
	case "kafka":
		return append(common, []BestPractice{
			{
				Category: "Kafka Best Practices",
				Items: []string{
					"Choose appropriate partition keys for ordering",
					"Configure retention policies based on use case",
					"Use consumer groups for parallel processing",
					"Implement idempotent producers for reliability",
				},
			},
		}...)
	case "mqtt":
		return append(common, []BestPractice{
			{
				Category: "MQTT Best Practices",
				Items: []string{
					"Choose appropriate QoS levels per use case",
					"Use retained messages for state initialization",
					"Implement clean session handling",
					"Use wildcards carefully in subscriptions",
				},
			},
		}...)
	case "amqp", "rabbitmq":
		return append(common, []BestPractice{
			{
				Category: "AMQP Best Practices",
				Items: []string{
					"Use durable queues for important messages",
					"Configure appropriate exchange types",
					"Acknowledge messages after processing",
					"Use publisher confirms for reliability",
				},
			},
		}...)
	case "soap", "wsdl":
		return append(common, []BestPractice{
			{
				Category: "SOAP Best Practices",
				Items: []string{
					"Validate XML against schema before sending",
					"Include SOAPAction header when required",
					"Handle SOAP faults appropriately",
					"Use WS-Security for production",
				},
			},
		}...)
	case "websocket", "ws":
		return append(common, []BestPractice{
			{
				Category: "WebSocket Best Practices",
				Items: []string{
					"Implement heartbeat/ping-pong for connection health",
					"Handle automatic reconnection on disconnect",
					"Use message framing for large payloads",
					"Implement backpressure for high-throughput scenarios",
				},
			},
		}...)
	case "graphql":
		return append(common, []BestPractice{
			{
				Category: "GraphQL Best Practices",
				Items: []string{
					"Request only the fields you need",
					"Use fragments for reusable field selections",
					"Implement proper pagination (cursor-based preferred)",
					"Use variables instead of string interpolation",
				},
			},
		}...)
	}

	return common
}

// CodeExampleConfig holds configuration for code example generation.
type CodeExampleConfig struct {
	Language    string // python, javascript, go, curl, etc.
	Method      string // HTTP method or operation type
	URL         string // Full URL or endpoint
	Headers     map[string]string
	Body        string // JSON body
	Description string
}

// GenerateCodeExample generates a code example for a specific language.
func GenerateCodeExample(cfg CodeExampleConfig) string {
	switch strings.ToLower(cfg.Language) {
	case "curl":
		return generateCurlExample(cfg)
	case "python":
		return generatePythonExample(cfg)
	case "javascript", "js":
		return generateJavaScriptExample(cfg)
	case "go", "golang":
		return generateGoExample(cfg)
	default:
		return generateCurlExample(cfg)
	}
}

func generateCurlExample(cfg CodeExampleConfig) string {
	var b strings.Builder
	b.WriteString("```bash\n")
	b.WriteString(fmt.Sprintf("curl -X %s \"%s\"", strings.ToUpper(cfg.Method), cfg.URL))

	for key, value := range cfg.Headers {
		b.WriteString(fmt.Sprintf(" \\\n  -H \"%s: %s\"", key, value))
	}

	if cfg.Body != "" && (cfg.Method == "POST" || cfg.Method == "PUT" || cfg.Method == "PATCH") {
		b.WriteString(fmt.Sprintf(" \\\n  -d '%s'", cfg.Body))
	}

	b.WriteString("\n```")
	return b.String()
}

func generatePythonExample(cfg CodeExampleConfig) string {
	var b strings.Builder
	b.WriteString("```python\n")
	b.WriteString("import requests\nimport os\n\n")

	b.WriteString(fmt.Sprintf("response = requests.%s(\n", strings.ToLower(cfg.Method)))
	b.WriteString(fmt.Sprintf("    '%s',\n", cfg.URL))

	if len(cfg.Headers) > 0 {
		b.WriteString("    headers={\n")
		for key, value := range cfg.Headers {
			// Replace YOUR_TOKEN or similar with env var
			if strings.Contains(value, "YOUR_") {
				value = fmt.Sprintf("f'{os.environ[\"API_KEY\"]}'")
				b.WriteString(fmt.Sprintf("        '%s': %s,\n", key, value))
			} else {
				b.WriteString(fmt.Sprintf("        '%s': '%s',\n", key, value))
			}
		}
		b.WriteString("    },\n")
	}

	if cfg.Body != "" && (cfg.Method == "POST" || cfg.Method == "PUT" || cfg.Method == "PATCH") {
		b.WriteString(fmt.Sprintf("    json=%s,\n", cfg.Body))
	}

	b.WriteString(")\n\n")
	b.WriteString("data = response.json()\nprint(data)\n")
	b.WriteString("```")
	return b.String()
}

func generateJavaScriptExample(cfg CodeExampleConfig) string {
	var b strings.Builder
	b.WriteString("```javascript\n")
	b.WriteString(fmt.Sprintf("const response = await fetch('%s', {\n", cfg.URL))
	b.WriteString(fmt.Sprintf("  method: '%s',\n", strings.ToUpper(cfg.Method)))

	if len(cfg.Headers) > 0 {
		b.WriteString("  headers: {\n")
		for key, value := range cfg.Headers {
			if strings.Contains(value, "YOUR_") {
				b.WriteString(fmt.Sprintf("    '%s': `Bearer ${process.env.API_KEY}`,\n", key))
			} else {
				b.WriteString(fmt.Sprintf("    '%s': '%s',\n", key, value))
			}
		}
		b.WriteString("  },\n")
	}

	if cfg.Body != "" && (cfg.Method == "POST" || cfg.Method == "PUT" || cfg.Method == "PATCH") {
		b.WriteString(fmt.Sprintf("  body: JSON.stringify(%s),\n", cfg.Body))
	}

	b.WriteString("});\n\n")
	b.WriteString("const data = await response.json();\nconsole.log(data);\n")
	b.WriteString("```")
	return b.String()
}

func generateGoExample(cfg CodeExampleConfig) string {
	var b strings.Builder
	b.WriteString("```go\n")
	b.WriteString("package main\n\n")
	b.WriteString("import (\n")
	b.WriteString("    \"net/http\"\n")
	b.WriteString("    \"os\"\n")
	if cfg.Body != "" {
		b.WriteString("    \"strings\"\n")
	}
	b.WriteString(")\n\n")

	b.WriteString("func main() {\n")

	if cfg.Body != "" {
		b.WriteString(fmt.Sprintf("    body := `%s`\n", cfg.Body))
		b.WriteString(fmt.Sprintf("    req, _ := http.NewRequest(\"%s\", \"%s\", strings.NewReader(body))\n",
			strings.ToUpper(cfg.Method), cfg.URL))
	} else {
		b.WriteString(fmt.Sprintf("    req, _ := http.NewRequest(\"%s\", \"%s\", nil)\n",
			strings.ToUpper(cfg.Method), cfg.URL))
	}

	for key, value := range cfg.Headers {
		if strings.Contains(value, "YOUR_") {
			b.WriteString(fmt.Sprintf("    req.Header.Set(\"%s\", \"Bearer \"+os.Getenv(\"API_KEY\"))\n", key))
		} else {
			b.WriteString(fmt.Sprintf("    req.Header.Set(\"%s\", \"%s\")\n", key, value))
		}
	}

	b.WriteString("\n    resp, _ := http.DefaultClient.Do(req)\n")
	b.WriteString("    defer resp.Body.Close()\n")
	b.WriteString("}\n")
	b.WriteString("```")
	return b.String()
}

// GenerateSDKQuickStart generates a comprehensive SDK quick start with multiple languages.
func GenerateSDKQuickStart(method, url, body string, headers map[string]string) string {
	var b strings.Builder

	// cURL
	b.WriteString("### cURL\n\n")
	b.WriteString(GenerateCodeExample(CodeExampleConfig{
		Language: "curl",
		Method:   method,
		URL:      url,
		Headers:  headers,
		Body:     body,
	}))
	b.WriteString("\n\n")

	// JavaScript
	b.WriteString("### JavaScript (fetch)\n\n")
	b.WriteString(GenerateCodeExample(CodeExampleConfig{
		Language: "javascript",
		Method:   method,
		URL:      url,
		Headers:  headers,
		Body:     body,
	}))
	b.WriteString("\n\n")

	// Python
	b.WriteString("### Python (requests)\n\n")
	b.WriteString(GenerateCodeExample(CodeExampleConfig{
		Language: "python",
		Method:   method,
		URL:      url,
		Headers:  headers,
		Body:     body,
	}))
	b.WriteString("\n\n")

	// Go
	b.WriteString("### Go (net/http)\n\n")
	b.WriteString(GenerateCodeExample(CodeExampleConfig{
		Language: "go",
		Method:   method,
		URL:      url,
		Headers:  headers,
		Body:     body,
	}))

	return strings.TrimSpace(b.String())
}

// OverviewTableRow represents a row in an overview table.
type OverviewTableRow struct {
	Property string
	Value    string
}

// GenerateOverviewTable generates a markdown table for API overview.
func GenerateOverviewTable(rows []OverviewTableRow) string {
	var b strings.Builder

	b.WriteString("| Property | Value |\n")
	b.WriteString("|----------|-------|\n")

	for _, row := range rows {
		b.WriteString(fmt.Sprintf("| **%s** | %s |\n", row.Property, row.Value))
	}

	return strings.TrimSpace(b.String())
}

// Truncate truncates a string to maxLen with ellipsis.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// SanitizeToolName converts a path/name to a valid tool name.
func SanitizeToolName(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "{", "")
	name = strings.ReplaceAll(name, "}", "")
	name = strings.Trim(name, "_")
	return strings.ToLower(name)
}
