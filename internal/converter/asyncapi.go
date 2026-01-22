// Package converter provides spec-to-SKILL.md converters.
package converter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/sanixdarker/skill-md/pkg/skill"
	"gopkg.in/yaml.v2"
)

// AsyncAPIConverter converts AsyncAPI specifications to skills.
type AsyncAPIConverter struct{}

// AsyncAPI spec types for parsing
type asyncAPISpec struct {
	AsyncAPI    string                     `yaml:"asyncapi" json:"asyncapi"`
	Info        asyncAPIInfo               `yaml:"info" json:"info"`
	Servers     map[string]asyncAPIServer  `yaml:"servers" json:"servers"`
	Channels    map[string]asyncAPIChannel `yaml:"channels" json:"channels"`
	Components  *asyncAPIComponents        `yaml:"components" json:"components"`
	Tags        []asyncAPITag              `yaml:"tags" json:"tags"`
	DefaultHost string                     `yaml:"defaultContentType" json:"defaultContentType"`
}

type asyncAPIInfo struct {
	Title          string        `yaml:"title" json:"title"`
	Version        string        `yaml:"version" json:"version"`
	Description    string        `yaml:"description" json:"description"`
	TermsOfService string        `yaml:"termsOfService" json:"termsOfService"`
	Contact        *asyncContact `yaml:"contact" json:"contact"`
	License        *asyncLicense `yaml:"license" json:"license"`
}

type asyncContact struct {
	Name  string `yaml:"name" json:"name"`
	URL   string `yaml:"url" json:"url"`
	Email string `yaml:"email" json:"email"`
}

type asyncLicense struct {
	Name string `yaml:"name" json:"name"`
	URL  string `yaml:"url" json:"url"`
}

type asyncAPIServer struct {
	URL         string                 `yaml:"url" json:"url"`
	Protocol    string                 `yaml:"protocol" json:"protocol"`
	Description string                 `yaml:"description" json:"description"`
	Security    []map[string][]string  `yaml:"security" json:"security"`
	Bindings    map[string]interface{} `yaml:"bindings" json:"bindings"`
	Variables   map[string]asyncAPIVar `yaml:"variables" json:"variables"`
}

type asyncAPIVar struct {
	Enum        []string `yaml:"enum" json:"enum"`
	Default     string   `yaml:"default" json:"default"`
	Description string   `yaml:"description" json:"description"`
}

type asyncAPIChannel struct {
	Description string                 `yaml:"description" json:"description"`
	Subscribe   *asyncAPIOperation     `yaml:"subscribe" json:"subscribe"`
	Publish     *asyncAPIOperation     `yaml:"publish" json:"publish"`
	Parameters  map[string]interface{} `yaml:"parameters" json:"parameters"`
	Bindings    map[string]interface{} `yaml:"bindings" json:"bindings"`
	// AsyncAPI 3.x style
	Messages map[string]asyncAPIMessageRef `yaml:"messages" json:"messages"`
}

type asyncAPIOperation struct {
	OperationID string                 `yaml:"operationId" json:"operationId"`
	Summary     string                 `yaml:"summary" json:"summary"`
	Description string                 `yaml:"description" json:"description"`
	Message     *asyncAPIMessageRef    `yaml:"message" json:"message"`
	Tags        []asyncAPITag          `yaml:"tags" json:"tags"`
	Bindings    map[string]interface{} `yaml:"bindings" json:"bindings"`
}

type asyncAPIMessageRef struct {
	Ref         string                 `yaml:"$ref" json:"$ref"`
	Name        string                 `yaml:"name" json:"name"`
	Title       string                 `yaml:"title" json:"title"`
	Summary     string                 `yaml:"summary" json:"summary"`
	Description string                 `yaml:"description" json:"description"`
	ContentType string                 `yaml:"contentType" json:"contentType"`
	Payload     map[string]interface{} `yaml:"payload" json:"payload"`
	Headers     map[string]interface{} `yaml:"headers" json:"headers"`
	Examples    []interface{}          `yaml:"examples" json:"examples"`
}

type asyncAPIComponents struct {
	Messages        map[string]asyncAPIMessageRef     `yaml:"messages" json:"messages"`
	Schemas         map[string]map[string]interface{} `yaml:"schemas" json:"schemas"`
	SecuritySchemes map[string]asyncAPISecurityScheme `yaml:"securitySchemes" json:"securitySchemes"`
}

type asyncAPISecurityScheme struct {
	Type        string `yaml:"type" json:"type"`
	Description string `yaml:"description" json:"description"`
	Name        string `yaml:"name" json:"name"`
	In          string `yaml:"in" json:"in"`
	Scheme      string `yaml:"scheme" json:"scheme"`
}

type asyncAPITag struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
}

func (c *AsyncAPIConverter) Name() string {
	return "asyncapi"
}

func (c *AsyncAPIConverter) CanHandle(filename string, content []byte) bool {
	ext := getExtension(filename)
	if ext != ".yaml" && ext != ".yml" && ext != ".json" {
		return false
	}
	// Check for asyncapi version key
	return bytes.Contains(content, []byte("asyncapi:")) ||
		bytes.Contains(content, []byte("\"asyncapi\":"))
}

func (c *AsyncAPIConverter) Convert(content []byte, opts *Options) (*skill.Skill, error) {
	var spec asyncAPISpec

	// Try YAML first, then JSON
	if err := yaml.Unmarshal(content, &spec); err != nil {
		if err := json.Unmarshal(content, &spec); err != nil {
			return nil, fmt.Errorf("failed to parse AsyncAPI spec: %w", err)
		}
	}

	if spec.AsyncAPI == "" {
		return nil, fmt.Errorf("not a valid AsyncAPI specification")
	}

	return c.buildSkill(&spec, opts), nil
}

func (c *AsyncAPIConverter) buildSkill(spec *asyncAPISpec, opts *Options) *skill.Skill {
	name := spec.Info.Title
	if opts != nil && opts.Name != "" {
		name = opts.Name
	}
	if name == "" {
		name = "AsyncAPI Skill"
	}

	description := spec.Info.Description
	if description == "" {
		description = fmt.Sprintf("Event-driven API documentation for %s", name)
	}

	s := skill.NewSkill(name, description)
	s.Frontmatter.SourceType = "asyncapi"
	if opts != nil && opts.SourcePath != "" {
		s.Frontmatter.Source = opts.SourcePath
	}
	s.Frontmatter.Version = spec.Info.Version

	// Set protocol-specific metadata
	protocols := c.extractProtocols(spec)
	if len(protocols) > 0 {
		s.Frontmatter.Protocol = protocols[0]
	}
	s.Frontmatter.ChannelCount = len(spec.Channels)
	s.Frontmatter.MessageCount = c.countMessages(spec)

	// Extract servers
	for _, server := range spec.Servers {
		s.Frontmatter.Servers = append(s.Frontmatter.Servers, server.URL)
	}

	// Extract auth methods
	s.Frontmatter.AuthMethods = c.extractAuthMethods(spec)

	// Calculate difficulty
	s.Frontmatter.Difficulty = c.calculateDifficulty(spec)
	s.Frontmatter.HasExamples = true

	// Set tags
	tags := []string{"asyncapi", "event-driven"}
	for _, t := range spec.Tags {
		tags = append(tags, t.Name)
	}
	tags = append(tags, protocols...)
	s.Frontmatter.Tags = tags

	// Build MCP-compatible tool definitions
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
	s.AddSection("Servers", 2, c.buildServersSection(spec))
	s.AddSection("Channels", 2, c.buildChannelsSection(spec))
	s.AddSection("Messages", 2, c.buildMessagesSection(spec))

	if len(c.extractBindings(spec)) > 0 {
		s.AddSection("Bindings", 2, c.buildBindingsSection(spec))
	}

	if len(s.Frontmatter.AuthMethods) > 0 {
		s.AddSection("Security", 2, c.buildSecuritySection(spec))
	}

	s.AddSection("Code Examples", 2, c.buildCodeExamples(spec))
	s.AddSection("Tool Definitions", 2, c.buildToolDefinitionsSection(spec))
	s.AddSection("Best Practices", 2, c.buildBestPractices(spec))

	return s
}

func (c *AsyncAPIConverter) extractProtocols(spec *asyncAPISpec) []string {
	protocols := make(map[string]bool)
	for _, server := range spec.Servers {
		if server.Protocol != "" {
			protocols[strings.ToLower(server.Protocol)] = true
		}
	}
	result := make([]string, 0, len(protocols))
	for p := range protocols {
		result = append(result, p)
	}
	sort.Strings(result)
	return result
}

func (c *AsyncAPIConverter) countMessages(spec *asyncAPISpec) int {
	count := 0
	if spec.Components != nil {
		count = len(spec.Components.Messages)
	}
	for _, ch := range spec.Channels {
		if ch.Messages != nil {
			count += len(ch.Messages)
		}
	}
	return count
}

func (c *AsyncAPIConverter) extractAuthMethods(spec *asyncAPISpec) []string {
	methods := make(map[string]bool)
	if spec.Components != nil {
		for _, scheme := range spec.Components.SecuritySchemes {
			if scheme.Type != "" {
				methods[scheme.Type] = true
			}
		}
	}
	result := make([]string, 0, len(methods))
	for m := range methods {
		result = append(result, m)
	}
	return result
}

func (c *AsyncAPIConverter) calculateDifficulty(spec *asyncAPISpec) string {
	complexity := len(spec.Channels) + c.countMessages(spec)
	if complexity <= 3 {
		return "novice"
	} else if complexity <= 10 {
		return "intermediate"
	}
	return "advanced"
}

func (c *AsyncAPIConverter) extractBindings(spec *asyncAPISpec) []string {
	bindings := make(map[string]bool)
	for _, server := range spec.Servers {
		for b := range server.Bindings {
			bindings[b] = true
		}
	}
	for _, ch := range spec.Channels {
		for b := range ch.Bindings {
			bindings[b] = true
		}
	}
	result := make([]string, 0, len(bindings))
	for b := range bindings {
		result = append(result, b)
	}
	return result
}

func (c *AsyncAPIConverter) buildQuickStart(spec *asyncAPISpec) string {
	var b strings.Builder

	protocols := c.extractProtocols(spec)
	protocol := "message broker"
	if len(protocols) > 0 {
		protocol = protocols[0]
	}

	var serverURL string
	for _, srv := range spec.Servers {
		serverURL = srv.URL
		break
	}

	b.WriteString("### Getting Started\n\n")
	b.WriteString(fmt.Sprintf("1. **Connect** to the %s server at `%s`\n", protocol, serverURL))
	b.WriteString("2. **Subscribe** to channels you want to receive messages from\n")
	b.WriteString("3. **Publish** messages to channels to send events\n\n")

	// Quick example based on protocol
	switch protocol {
	case "kafka":
		b.WriteString("```python\n")
		b.WriteString("from kafka import KafkaConsumer, KafkaProducer\n\n")
		b.WriteString(fmt.Sprintf("# Connect to Kafka\nproducer = KafkaProducer(bootstrap_servers='%s')\n", serverURL))
		b.WriteString(fmt.Sprintf("consumer = KafkaConsumer(bootstrap_servers='%s')\n", serverURL))
		b.WriteString("```\n")
	case "mqtt":
		b.WriteString("```python\n")
		b.WriteString("import paho.mqtt.client as mqtt\n\n")
		b.WriteString("# Connect to MQTT broker\n")
		b.WriteString(fmt.Sprintf("client = mqtt.Client()\nclient.connect('%s')\n", serverURL))
		b.WriteString("```\n")
	case "amqp":
		b.WriteString("```python\n")
		b.WriteString("import pika\n\n")
		b.WriteString("# Connect to RabbitMQ\n")
		b.WriteString(fmt.Sprintf("connection = pika.BlockingConnection(pika.URLParameters('%s'))\n", serverURL))
		b.WriteString("channel = connection.channel()\n")
		b.WriteString("```\n")
	case "ws", "websocket":
		b.WriteString("```javascript\n")
		b.WriteString(fmt.Sprintf("const ws = new WebSocket('%s');\n\n", serverURL))
		b.WriteString("ws.onmessage = (event) => console.log(event.data);\n")
		b.WriteString("ws.send(JSON.stringify({ type: 'subscribe', channel: 'events' }));\n")
		b.WriteString("```\n")
	default:
		b.WriteString("```javascript\n")
		b.WriteString("// Connect to message broker\n")
		b.WriteString(fmt.Sprintf("const connection = await connect('%s');\n", serverURL))
		b.WriteString("```\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *AsyncAPIConverter) buildOverview(spec *asyncAPISpec) string {
	var b strings.Builder

	if spec.Info.Description != "" {
		b.WriteString(spec.Info.Description)
		b.WriteString("\n\n")
	}

	b.WriteString("| Property | Value |\n")
	b.WriteString("|----------|-------|\n")
	b.WriteString(fmt.Sprintf("| **AsyncAPI Version** | %s |\n", spec.AsyncAPI))
	b.WriteString(fmt.Sprintf("| **API Version** | %s |\n", spec.Info.Version))
	b.WriteString(fmt.Sprintf("| **Channels** | %d |\n", len(spec.Channels)))
	b.WriteString(fmt.Sprintf("| **Messages** | %d |\n", c.countMessages(spec)))

	protocols := c.extractProtocols(spec)
	if len(protocols) > 0 {
		b.WriteString(fmt.Sprintf("| **Protocols** | %s |\n", strings.Join(protocols, ", ")))
	}

	if spec.Info.Contact != nil && spec.Info.Contact.Email != "" {
		b.WriteString(fmt.Sprintf("| **Contact** | %s |\n", spec.Info.Contact.Email))
	}

	if spec.Info.License != nil && spec.Info.License.Name != "" {
		b.WriteString(fmt.Sprintf("| **License** | %s |\n", spec.Info.License.Name))
	}

	return strings.TrimSpace(b.String())
}

func (c *AsyncAPIConverter) buildServersSection(spec *asyncAPISpec) string {
	var b strings.Builder

	if len(spec.Servers) == 0 {
		b.WriteString("No servers defined in specification.\n")
		return b.String()
	}

	for name, server := range spec.Servers {
		b.WriteString(fmt.Sprintf("### %s\n\n", name))
		b.WriteString(fmt.Sprintf("- **URL**: `%s`\n", server.URL))
		b.WriteString(fmt.Sprintf("- **Protocol**: %s\n", server.Protocol))
		if server.Description != "" {
			b.WriteString(fmt.Sprintf("- **Description**: %s\n", server.Description))
		}

		if len(server.Variables) > 0 {
			b.WriteString("\n**Variables:**\n")
			for varName, v := range server.Variables {
				b.WriteString(fmt.Sprintf("- `{%s}`: %s (default: `%s`)\n", varName, v.Description, v.Default))
			}
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *AsyncAPIConverter) buildChannelsSection(spec *asyncAPISpec) string {
	var b strings.Builder

	// Sort channel names for consistent output
	channelNames := make([]string, 0, len(spec.Channels))
	for name := range spec.Channels {
		channelNames = append(channelNames, name)
	}
	sort.Strings(channelNames)

	for _, name := range channelNames {
		ch := spec.Channels[name]
		b.WriteString(fmt.Sprintf("### `%s`\n\n", name))

		if ch.Description != "" {
			b.WriteString(ch.Description)
			b.WriteString("\n\n")
		}

		if ch.Subscribe != nil {
			b.WriteString("**Subscribe** (receive messages)\n")
			if ch.Subscribe.Summary != "" {
				b.WriteString(fmt.Sprintf("- %s\n", ch.Subscribe.Summary))
			}
			if ch.Subscribe.Message != nil {
				msgName := ch.Subscribe.Message.Name
				if msgName == "" && ch.Subscribe.Message.Ref != "" {
					parts := strings.Split(ch.Subscribe.Message.Ref, "/")
					msgName = parts[len(parts)-1]
				}
				if msgName != "" {
					b.WriteString(fmt.Sprintf("- Message: `%s`\n", msgName))
				}
			}
			b.WriteString("\n")
		}

		if ch.Publish != nil {
			b.WriteString("**Publish** (send messages)\n")
			if ch.Publish.Summary != "" {
				b.WriteString(fmt.Sprintf("- %s\n", ch.Publish.Summary))
			}
			if ch.Publish.Message != nil {
				msgName := ch.Publish.Message.Name
				if msgName == "" && ch.Publish.Message.Ref != "" {
					parts := strings.Split(ch.Publish.Message.Ref, "/")
					msgName = parts[len(parts)-1]
				}
				if msgName != "" {
					b.WriteString(fmt.Sprintf("- Message: `%s`\n", msgName))
				}
			}
			b.WriteString("\n")
		}

		if len(ch.Parameters) > 0 {
			b.WriteString("**Parameters:**\n")
			for pName := range ch.Parameters {
				b.WriteString(fmt.Sprintf("- `{%s}`\n", pName))
			}
			b.WriteString("\n")
		}
	}

	return strings.TrimSpace(b.String())
}

func (c *AsyncAPIConverter) buildMessagesSection(spec *asyncAPISpec) string {
	var b strings.Builder

	messages := make(map[string]asyncAPIMessageRef)

	// Collect from components
	if spec.Components != nil {
		for name, msg := range spec.Components.Messages {
			messages[name] = msg
		}
	}

	// Collect inline messages from channels
	for _, ch := range spec.Channels {
		if ch.Subscribe != nil && ch.Subscribe.Message != nil && ch.Subscribe.Message.Name != "" {
			messages[ch.Subscribe.Message.Name] = *ch.Subscribe.Message
		}
		if ch.Publish != nil && ch.Publish.Message != nil && ch.Publish.Message.Name != "" {
			messages[ch.Publish.Message.Name] = *ch.Publish.Message
		}
	}

	if len(messages) == 0 {
		b.WriteString("No messages defined.\n")
		return b.String()
	}

	// Sort message names
	msgNames := make([]string, 0, len(messages))
	for name := range messages {
		msgNames = append(msgNames, name)
	}
	sort.Strings(msgNames)

	for _, name := range msgNames {
		msg := messages[name]
		title := msg.Title
		if title == "" {
			title = name
		}
		b.WriteString(fmt.Sprintf("### %s\n\n", title))

		if msg.Summary != "" {
			b.WriteString(msg.Summary)
			b.WriteString("\n\n")
		} else if msg.Description != "" {
			b.WriteString(msg.Description)
			b.WriteString("\n\n")
		}

		if msg.ContentType != "" {
			b.WriteString(fmt.Sprintf("**Content-Type**: `%s`\n\n", msg.ContentType))
		}

		if len(msg.Payload) > 0 {
			b.WriteString("**Payload Schema:**\n")
			b.WriteString("```json\n")
			payload, _ := json.MarshalIndent(msg.Payload, "", "  ")
			b.WriteString(string(payload))
			b.WriteString("\n```\n\n")

			// Generate example
			b.WriteString("**Example:**\n")
			b.WriteString("```json\n")
			example := c.generateMessageExample(msg.Payload)
			exampleJSON, _ := json.MarshalIndent(example, "", "  ")
			b.WriteString(string(exampleJSON))
			b.WriteString("\n```\n\n")
		}
	}

	return strings.TrimSpace(b.String())
}

func (c *AsyncAPIConverter) generateMessageExample(schema map[string]interface{}) map[string]interface{} {
	example := make(map[string]interface{})

	props, ok := schema["properties"].(map[interface{}]interface{})
	if !ok {
		if props2, ok2 := schema["properties"].(map[string]interface{}); ok2 {
			for k, v := range props2 {
				example[k] = c.generateFieldExample(v)
			}
		}
		return example
	}

	for k, v := range props {
		key := fmt.Sprintf("%v", k)
		example[key] = c.generateFieldExample(v)
	}

	return example
}

func (c *AsyncAPIConverter) generateFieldExample(field interface{}) interface{} {
	fieldMap, ok := field.(map[interface{}]interface{})
	if !ok {
		if fm, ok2 := field.(map[string]interface{}); ok2 {
			return c.generateFieldExampleFromStringMap(fm)
		}
		return "value"
	}

	typeVal, _ := fieldMap["type"].(string)
	switch typeVal {
	case "string":
		if format, ok := fieldMap["format"].(string); ok {
			switch format {
			case "date-time":
				return "2024-01-15T10:30:00Z"
			case "email":
				return "user@example.com"
			case "uuid":
				return "550e8400-e29b-41d4-a716-446655440000"
			}
		}
		return "string"
	case "integer", "number":
		return 42
	case "boolean":
		return true
	case "array":
		return []interface{}{"item1", "item2"}
	case "object":
		if props, ok := fieldMap["properties"]; ok {
			return c.generateFieldExample(props)
		}
		return map[string]interface{}{}
	default:
		return "value"
	}
}

func (c *AsyncAPIConverter) generateFieldExampleFromStringMap(fm map[string]interface{}) interface{} {
	typeVal, _ := fm["type"].(string)
	switch typeVal {
	case "string":
		if format, ok := fm["format"].(string); ok {
			switch format {
			case "date-time":
				return "2024-01-15T10:30:00Z"
			case "email":
				return "user@example.com"
			case "uuid":
				return "550e8400-e29b-41d4-a716-446655440000"
			}
		}
		return "string"
	case "integer", "number":
		return 42
	case "boolean":
		return true
	case "array":
		return []interface{}{"item1", "item2"}
	case "object":
		return map[string]interface{}{}
	default:
		return "value"
	}
}

func (c *AsyncAPIConverter) buildBindingsSection(spec *asyncAPISpec) string {
	var b strings.Builder

	bindings := c.extractBindings(spec)
	if len(bindings) == 0 {
		return "No protocol-specific bindings defined."
	}

	b.WriteString("Protocol-specific configurations:\n\n")

	for _, binding := range bindings {
		b.WriteString(fmt.Sprintf("### %s Binding\n\n", strings.Title(binding)))

		switch binding {
		case "kafka":
			b.WriteString("- **Consumer Group**: Configure consumer groups for message partitioning\n")
			b.WriteString("- **Partitions**: Messages can be distributed across partitions\n")
			b.WriteString("- **Key**: Use message keys for ordering guarantees\n")
		case "mqtt":
			b.WriteString("- **QoS Levels**: 0 (at most once), 1 (at least once), 2 (exactly once)\n")
			b.WriteString("- **Retain**: Messages can be retained for new subscribers\n")
		case "amqp":
			b.WriteString("- **Exchange Types**: direct, topic, fanout, headers\n")
			b.WriteString("- **Queues**: Durable or transient message storage\n")
		case "ws", "websocket":
			b.WriteString("- **Heartbeat**: Keep connections alive with ping/pong\n")
			b.WriteString("- **Reconnection**: Implement automatic reconnection logic\n")
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *AsyncAPIConverter) buildSecuritySection(spec *asyncAPISpec) string {
	var b strings.Builder

	if spec.Components == nil || len(spec.Components.SecuritySchemes) == 0 {
		return "No security schemes defined."
	}

	for name, scheme := range spec.Components.SecuritySchemes {
		b.WriteString(fmt.Sprintf("### %s\n\n", name))
		b.WriteString(fmt.Sprintf("**Type**: %s\n", scheme.Type))
		if scheme.Description != "" {
			b.WriteString(fmt.Sprintf("\n%s\n", scheme.Description))
		}

		switch scheme.Type {
		case "userPassword":
			b.WriteString("\n```python\n")
			b.WriteString("# Authenticate with username/password\n")
			b.WriteString("credentials = {'username': 'user', 'password': 'pass'}\n")
			b.WriteString("```\n")
		case "apiKey":
			b.WriteString(fmt.Sprintf("\n**Location**: %s\n", scheme.In))
			b.WriteString(fmt.Sprintf("**Key Name**: %s\n", scheme.Name))
			b.WriteString("\n```python\n")
			b.WriteString(fmt.Sprintf("headers = {'%s': 'YOUR_API_KEY'}\n", scheme.Name))
			b.WriteString("```\n")
		case "oauth2":
			b.WriteString("\n```python\n")
			b.WriteString("# OAuth2 authentication\n")
			b.WriteString("token = get_oauth_token(client_id, client_secret)\n")
			b.WriteString("```\n")
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *AsyncAPIConverter) buildCodeExamples(spec *asyncAPISpec) string {
	var b strings.Builder

	protocols := c.extractProtocols(spec)
	var serverURL string
	var protocol string
	for _, srv := range spec.Servers {
		serverURL = srv.URL
		protocol = srv.Protocol
		break
	}

	if len(protocols) > 0 {
		protocol = protocols[0]
	}

	// Get first channel for examples
	var channelName string
	for name := range spec.Channels {
		channelName = name
		break
	}

	b.WriteString("### Python\n\n")
	switch protocol {
	case "kafka":
		b.WriteString("```python\n")
		b.WriteString("from kafka import KafkaConsumer, KafkaProducer\n")
		b.WriteString("import json\n\n")
		b.WriteString(fmt.Sprintf("# Producer\nproducer = KafkaProducer(\n    bootstrap_servers='%s',\n    value_serializer=lambda v: json.dumps(v).encode('utf-8')\n)\n\n", serverURL))
		b.WriteString(fmt.Sprintf("producer.send('%s', {'event': 'data'})\nproducer.flush()\n\n", channelName))
		b.WriteString(fmt.Sprintf("# Consumer\nconsumer = KafkaConsumer(\n    '%s',\n    bootstrap_servers='%s',\n    value_deserializer=lambda m: json.loads(m.decode('utf-8'))\n)\n\n", channelName, serverURL))
		b.WriteString("for message in consumer:\n    print(message.value)\n")
		b.WriteString("```\n\n")
	case "mqtt":
		b.WriteString("```python\n")
		b.WriteString("import paho.mqtt.client as mqtt\n")
		b.WriteString("import json\n\n")
		b.WriteString("def on_message(client, userdata, msg):\n    print(json.loads(msg.payload))\n\n")
		b.WriteString(fmt.Sprintf("client = mqtt.Client()\nclient.on_message = on_message\nclient.connect('%s')\n\n", serverURL))
		b.WriteString(fmt.Sprintf("# Subscribe\nclient.subscribe('%s')\n\n", channelName))
		b.WriteString(fmt.Sprintf("# Publish\nclient.publish('%s', json.dumps({'event': 'data'}))\n\n", channelName))
		b.WriteString("client.loop_forever()\n")
		b.WriteString("```\n\n")
	case "amqp":
		b.WriteString("```python\n")
		b.WriteString("import pika\nimport json\n\n")
		b.WriteString(fmt.Sprintf("connection = pika.BlockingConnection(pika.URLParameters('%s'))\n", serverURL))
		b.WriteString("channel = connection.channel()\n\n")
		b.WriteString(fmt.Sprintf("# Declare queue\nchannel.queue_declare(queue='%s')\n\n", channelName))
		b.WriteString("# Publish\nchannel.basic_publish(exchange='', routing_key='%s', body=json.dumps({'event': 'data'}))\n\n")
		b.WriteString("# Consume\ndef callback(ch, method, properties, body):\n    print(json.loads(body))\n\n")
		b.WriteString(fmt.Sprintf("channel.basic_consume(queue='%s', on_message_callback=callback, auto_ack=True)\n", channelName))
		b.WriteString("channel.start_consuming()\n")
		b.WriteString("```\n\n")
	default:
		b.WriteString("```python\n")
		b.WriteString("import asyncio\nimport websockets\nimport json\n\n")
		b.WriteString("async def connect():\n")
		b.WriteString(fmt.Sprintf("    async with websockets.connect('%s') as ws:\n", serverURL))
		b.WriteString("        # Subscribe\n")
		b.WriteString(fmt.Sprintf("        await ws.send(json.dumps({'action': 'subscribe', 'channel': '%s'}))\n", channelName))
		b.WriteString("        \n        # Receive messages\n")
		b.WriteString("        async for message in ws:\n            print(json.loads(message))\n\n")
		b.WriteString("asyncio.run(connect())\n")
		b.WriteString("```\n\n")
	}

	b.WriteString("### JavaScript/Node.js\n\n")
	switch protocol {
	case "kafka":
		b.WriteString("```javascript\n")
		b.WriteString("const { Kafka } = require('kafkajs');\n\n")
		b.WriteString(fmt.Sprintf("const kafka = new Kafka({ brokers: ['%s'] });\n\n", serverURL))
		b.WriteString("// Producer\nconst producer = kafka.producer();\nawait producer.connect();\n")
		b.WriteString(fmt.Sprintf("await producer.send({ topic: '%s', messages: [{ value: JSON.stringify({ event: 'data' }) }] });\n\n", channelName))
		b.WriteString("// Consumer\nconst consumer = kafka.consumer({ groupId: 'my-group' });\nawait consumer.connect();\n")
		b.WriteString(fmt.Sprintf("await consumer.subscribe({ topic: '%s' });\n", channelName))
		b.WriteString("await consumer.run({ eachMessage: async ({ message }) => console.log(JSON.parse(message.value)) });\n")
		b.WriteString("```\n\n")
	case "mqtt":
		b.WriteString("```javascript\n")
		b.WriteString("const mqtt = require('mqtt');\n\n")
		b.WriteString(fmt.Sprintf("const client = mqtt.connect('%s');\n\n", serverURL))
		b.WriteString(fmt.Sprintf("client.on('connect', () => {\n  client.subscribe('%s');\n  client.publish('%s', JSON.stringify({ event: 'data' }));\n});\n\n", channelName, channelName))
		b.WriteString("client.on('message', (topic, message) => console.log(JSON.parse(message.toString())));\n")
		b.WriteString("```\n\n")
	default:
		b.WriteString("```javascript\n")
		b.WriteString(fmt.Sprintf("const ws = new WebSocket('%s');\n\n", serverURL))
		b.WriteString("ws.onopen = () => {\n")
		b.WriteString(fmt.Sprintf("  ws.send(JSON.stringify({ action: 'subscribe', channel: '%s' }));\n};\n\n", channelName))
		b.WriteString("ws.onmessage = (event) => console.log(JSON.parse(event.data));\n")
		b.WriteString("```\n\n")
	}

	b.WriteString("### Go\n\n")
	switch protocol {
	case "kafka":
		b.WriteString("```go\n")
		b.WriteString("package main\n\nimport (\n    \"github.com/segmentio/kafka-go\"\n    \"encoding/json\"\n    \"context\"\n)\n\n")
		b.WriteString("func main() {\n")
		b.WriteString(fmt.Sprintf("    // Producer\n    w := kafka.NewWriter(kafka.WriterConfig{\n        Brokers: []string{\"%s\"},\n        Topic:   \"%s\",\n    })\n", serverURL, channelName))
		b.WriteString("    data, _ := json.Marshal(map[string]string{\"event\": \"data\"})\n")
		b.WriteString("    w.WriteMessages(context.Background(), kafka.Message{Value: data})\n\n")
		b.WriteString(fmt.Sprintf("    // Consumer\n    r := kafka.NewReader(kafka.ReaderConfig{\n        Brokers: []string{\"%s\"},\n        Topic:   \"%s\",\n        GroupID: \"my-group\",\n    })\n", serverURL, channelName))
		b.WriteString("    for {\n        m, _ := r.ReadMessage(context.Background())\n        fmt.Println(string(m.Value))\n    }\n}\n")
		b.WriteString("```\n")
	default:
		b.WriteString("```go\n")
		b.WriteString("package main\n\nimport (\n    \"github.com/gorilla/websocket\"\n    \"encoding/json\"\n)\n\n")
		b.WriteString("func main() {\n")
		b.WriteString(fmt.Sprintf("    c, _, _ := websocket.DefaultDialer.Dial(\"%s\", nil)\n", serverURL))
		b.WriteString("    defer c.Close()\n\n")
		b.WriteString(fmt.Sprintf("    c.WriteJSON(map[string]string{\"action\": \"subscribe\", \"channel\": \"%s\"})\n\n", channelName))
		b.WriteString("    for {\n        _, msg, _ := c.ReadMessage()\n        fmt.Println(string(msg))\n    }\n}\n")
		b.WriteString("```\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *AsyncAPIConverter) buildToolDefinitions(spec *asyncAPISpec) []skill.ToolDefinition {
	tools := make([]skill.ToolDefinition, 0)

	for channelName, ch := range spec.Channels {
		// Publish tool
		if ch.Publish != nil {
			toolName := fmt.Sprintf("publish_to_%s", sanitizeToolName(channelName))
			desc := fmt.Sprintf("Publish a message to the %s channel", channelName)
			if ch.Publish.Summary != "" {
				desc = ch.Publish.Summary
			}

			params := map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"message": map[string]interface{}{
						"type":        "object",
						"description": "Message payload to publish",
					},
				},
			}

			tools = append(tools, skill.ToolDefinition{
				Name:        toolName,
				Description: desc,
				Parameters:  params,
				Required:    []string{"message"},
			})
		}

		// Subscribe tool
		if ch.Subscribe != nil {
			toolName := fmt.Sprintf("subscribe_to_%s", sanitizeToolName(channelName))
			desc := fmt.Sprintf("Subscribe to messages from the %s channel", channelName)
			if ch.Subscribe.Summary != "" {
				desc = ch.Subscribe.Summary
			}

			params := map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"callback": map[string]interface{}{
						"type":        "string",
						"description": "Callback URL or handler for received messages",
					},
				},
			}

			tools = append(tools, skill.ToolDefinition{
				Name:        toolName,
				Description: desc,
				Parameters:  params,
			})
		}
	}

	return tools
}

func (c *AsyncAPIConverter) buildToolDefinitionsSection(spec *asyncAPISpec) string {
	var b strings.Builder

	b.WriteString("MCP-compatible tool definitions for AI agents:\n\n")
	b.WriteString("```yaml\n")
	b.WriteString("tools:\n")

	for channelName, ch := range spec.Channels {
		if ch.Publish != nil {
			b.WriteString(fmt.Sprintf("  - name: publish_to_%s\n", sanitizeToolName(channelName)))
			b.WriteString(fmt.Sprintf("    description: Publish to %s\n", channelName))
			b.WriteString("    parameters:\n")
			b.WriteString("      type: object\n")
			b.WriteString("      properties:\n")
			b.WriteString("        message:\n")
			b.WriteString("          type: object\n")
			b.WriteString("          description: Message payload\n")
			b.WriteString("      required: [message]\n")
		}
		if ch.Subscribe != nil {
			b.WriteString(fmt.Sprintf("  - name: subscribe_to_%s\n", sanitizeToolName(channelName)))
			b.WriteString(fmt.Sprintf("    description: Subscribe to %s\n", channelName))
			b.WriteString("    parameters:\n")
			b.WriteString("      type: object\n")
			b.WriteString("      properties:\n")
			b.WriteString("        handler:\n")
			b.WriteString("          type: string\n")
			b.WriteString("          description: Message handler reference\n")
		}
	}
	b.WriteString("```\n")

	return strings.TrimSpace(b.String())
}

func (c *AsyncAPIConverter) buildBestPractices(spec *asyncAPISpec) string {
	var b strings.Builder

	protocols := c.extractProtocols(spec)

	b.WriteString("### Message Design\n\n")
	b.WriteString("- Use consistent message schemas across channels\n")
	b.WriteString("- Include correlation IDs for request tracking\n")
	b.WriteString("- Version your message formats for backwards compatibility\n")
	b.WriteString("- Use content-type headers to specify encoding\n\n")

	b.WriteString("### Error Handling\n\n")
	b.WriteString("- Implement dead letter queues for failed messages\n")
	b.WriteString("- Use exponential backoff for retries\n")
	b.WriteString("- Log message processing failures with context\n")
	b.WriteString("- Consider idempotency for at-least-once delivery\n\n")

	b.WriteString("### Performance\n\n")
	b.WriteString("- Batch messages when possible to reduce overhead\n")
	b.WriteString("- Use message compression for large payloads\n")
	b.WriteString("- Monitor consumer lag and throughput\n")

	for _, protocol := range protocols {
		switch protocol {
		case "kafka":
			b.WriteString("\n### Kafka Best Practices\n\n")
			b.WriteString("- Choose appropriate partition keys for ordering\n")
			b.WriteString("- Configure retention policies based on use case\n")
			b.WriteString("- Use consumer groups for parallel processing\n")
			b.WriteString("- Monitor broker health and replication\n")
		case "mqtt":
			b.WriteString("\n### MQTT Best Practices\n\n")
			b.WriteString("- Choose appropriate QoS levels per use case\n")
			b.WriteString("- Use retained messages for state initialization\n")
			b.WriteString("- Implement clean session handling\n")
			b.WriteString("- Use wildcards carefully in subscriptions\n")
		case "amqp":
			b.WriteString("\n### AMQP Best Practices\n\n")
			b.WriteString("- Use durable queues for important messages\n")
			b.WriteString("- Configure appropriate exchange types\n")
			b.WriteString("- Acknowledge messages after processing\n")
			b.WriteString("- Use publisher confirms for reliability\n")
		}
	}

	return strings.TrimSpace(b.String())
}

func sanitizeToolName(name string) string {
	// Replace common separators with underscores
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "{", "")
	name = strings.ReplaceAll(name, "}", "")
	name = strings.Trim(name, "_")
	return strings.ToLower(name)
}
