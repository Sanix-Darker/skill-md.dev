// Package converter provides spec-to-SKILL.md converters.
package converter

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/sanixdarker/skillforge/pkg/skill"
)

// ProtobufConverter converts Protocol Buffer definitions to skills.
type ProtobufConverter struct{}

// Parsed proto types
type protoFile struct {
	Syntax   string
	Package  string
	Imports  []string
	Options  map[string]string
	Services []protoService
	Messages []protoMessage
	Enums    []protoEnum
}

type protoService struct {
	Name     string
	Comments string
	Methods  []protoMethod
}

type protoMethod struct {
	Name            string
	Comments        string
	InputType       string
	OutputType      string
	ClientStreaming bool
	ServerStreaming bool
}

type protoMessage struct {
	Name     string
	Comments string
	Fields   []protoField
	Nested   []protoMessage
	Enums    []protoEnum
}

type protoField struct {
	Name     string
	Type     string
	Number   int
	Repeated bool
	Optional bool
	Comments string
	MapKey   string
	MapValue string
}

type protoEnum struct {
	Name     string
	Comments string
	Values   []protoEnumValue
}

type protoEnumValue struct {
	Name     string
	Number   int
	Comments string
}

func (c *ProtobufConverter) Name() string {
	return "proto"
}

func (c *ProtobufConverter) CanHandle(filename string, content []byte) bool {
	ext := getExtension(filename)
	if ext == ".proto" {
		return true
	}
	// Also check content for proto syntax
	return bytes.Contains(content, []byte("syntax = \"proto")) &&
		(bytes.Contains(content, []byte("message ")) || bytes.Contains(content, []byte("service ")))
}

func (c *ProtobufConverter) Convert(content []byte, opts *Options) (*skill.Skill, error) {
	proto, err := c.parseProto(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proto file: %w", err)
	}

	return c.buildSkill(proto, opts), nil
}

func (c *ProtobufConverter) parseProto(content []byte) (*protoFile, error) {
	proto := &protoFile{
		Options:  make(map[string]string),
		Services: []protoService{},
		Messages: []protoMessage{},
		Enums:    []protoEnum{},
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	var currentComments []string
	var braceDepth int
	var currentContext string // "service", "message", "enum"
	var currentServiceIdx, currentMessageIdx, currentEnumIdx int
	var nestedMessages []int // stack of message indices for nested messages

	// Regex patterns
	syntaxRe := regexp.MustCompile(`^syntax\s*=\s*"([^"]+)"`)
	packageRe := regexp.MustCompile(`^package\s+([^;]+)`)
	importRe := regexp.MustCompile(`^import\s+"([^"]+)"`)
	optionRe := regexp.MustCompile(`^option\s+(\w+)\s*=\s*"?([^";]+)"?`)
	serviceRe := regexp.MustCompile(`^service\s+(\w+)\s*\{?`)
	messageRe := regexp.MustCompile(`^message\s+(\w+)\s*\{?`)
	enumRe := regexp.MustCompile(`^enum\s+(\w+)\s*\{?`)
	rpcRe := regexp.MustCompile(`rpc\s+(\w+)\s*\(\s*(stream\s+)?(\w+)\s*\)\s*returns\s*\(\s*(stream\s+)?(\w+)\s*\)`)
	fieldRe := regexp.MustCompile(`^\s*(repeated\s+|optional\s+)?(map<(\w+),\s*(\w+)>|[\w.]+)\s+(\w+)\s*=\s*(\d+)`)
	enumValueRe := regexp.MustCompile(`^\s*(\w+)\s*=\s*(-?\d+)`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Collect comments
		if strings.HasPrefix(line, "//") {
			comment := strings.TrimPrefix(line, "//")
			comment = strings.TrimSpace(comment)
			currentComments = append(currentComments, comment)
			continue
		}

		// Handle block comments
		if strings.HasPrefix(line, "/*") {
			// Simple block comment handling
			if strings.Contains(line, "*/") {
				comment := strings.TrimPrefix(line, "/*")
				comment = strings.TrimSuffix(comment, "*/")
				currentComments = append(currentComments, strings.TrimSpace(comment))
			}
			continue
		}

		// Track brace depth
		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")

		// Parse syntax
		if matches := syntaxRe.FindStringSubmatch(line); len(matches) > 1 {
			proto.Syntax = matches[1]
			currentComments = nil
			continue
		}

		// Parse package
		if matches := packageRe.FindStringSubmatch(line); len(matches) > 1 {
			proto.Package = strings.TrimSuffix(matches[1], ";")
			currentComments = nil
			continue
		}

		// Parse import
		if matches := importRe.FindStringSubmatch(line); len(matches) > 1 {
			proto.Imports = append(proto.Imports, matches[1])
			currentComments = nil
			continue
		}

		// Parse option
		if matches := optionRe.FindStringSubmatch(line); len(matches) > 2 {
			proto.Options[matches[1]] = matches[2]
			currentComments = nil
			continue
		}

		// Parse service
		if matches := serviceRe.FindStringSubmatch(line); len(matches) > 1 {
			svc := protoService{
				Name:     matches[1],
				Comments: strings.Join(currentComments, " "),
				Methods:  []protoMethod{},
			}
			proto.Services = append(proto.Services, svc)
			currentServiceIdx = len(proto.Services) - 1
			currentContext = "service"
			currentComments = nil
			continue
		}

		// Parse message
		if matches := messageRe.FindStringSubmatch(line); len(matches) > 1 {
			msg := protoMessage{
				Name:     matches[1],
				Comments: strings.Join(currentComments, " "),
				Fields:   []protoField{},
				Nested:   []protoMessage{},
				Enums:    []protoEnum{},
			}

			if currentContext == "message" && len(nestedMessages) > 0 {
				// Nested message
				parentIdx := nestedMessages[len(nestedMessages)-1]
				proto.Messages[parentIdx].Nested = append(proto.Messages[parentIdx].Nested, msg)
				nestedMessages = append(nestedMessages, len(proto.Messages))
			} else {
				proto.Messages = append(proto.Messages, msg)
				currentMessageIdx = len(proto.Messages) - 1
				nestedMessages = []int{currentMessageIdx}
			}
			currentContext = "message"
			currentComments = nil
			continue
		}

		// Parse enum
		if matches := enumRe.FindStringSubmatch(line); len(matches) > 1 {
			enum := protoEnum{
				Name:     matches[1],
				Comments: strings.Join(currentComments, " "),
				Values:   []protoEnumValue{},
			}

			if currentContext == "message" && len(nestedMessages) > 0 {
				parentIdx := nestedMessages[len(nestedMessages)-1]
				proto.Messages[parentIdx].Enums = append(proto.Messages[parentIdx].Enums, enum)
				currentEnumIdx = len(proto.Messages[parentIdx].Enums) - 1
			} else {
				proto.Enums = append(proto.Enums, enum)
				currentEnumIdx = len(proto.Enums) - 1
			}
			currentContext = "enum"
			currentComments = nil
			continue
		}

		// Parse RPC method
		if currentContext == "service" && strings.Contains(line, "rpc") {
			if matches := rpcRe.FindStringSubmatch(line); len(matches) > 5 {
				method := protoMethod{
					Name:            matches[1],
					Comments:        strings.Join(currentComments, " "),
					ClientStreaming: matches[2] != "",
					InputType:       matches[3],
					ServerStreaming: matches[4] != "",
					OutputType:      matches[5],
				}
				if currentServiceIdx < len(proto.Services) {
					proto.Services[currentServiceIdx].Methods = append(proto.Services[currentServiceIdx].Methods, method)
				}
				currentComments = nil
				continue
			}
		}

		// Parse field
		if currentContext == "message" && len(nestedMessages) > 0 {
			if matches := fieldRe.FindStringSubmatch(line); len(matches) > 5 {
				field := protoField{
					Repeated: strings.Contains(matches[1], "repeated"),
					Optional: strings.Contains(matches[1], "optional"),
					Comments: strings.Join(currentComments, " "),
				}

				if matches[3] != "" && matches[4] != "" {
					// Map type
					field.Type = "map"
					field.MapKey = matches[3]
					field.MapValue = matches[4]
				} else {
					field.Type = matches[2]
				}
				field.Name = matches[5]
				// Parse field number (matches[6])

				msgIdx := nestedMessages[len(nestedMessages)-1]
				if msgIdx < len(proto.Messages) {
					proto.Messages[msgIdx].Fields = append(proto.Messages[msgIdx].Fields, field)
				}
				currentComments = nil
				continue
			}
		}

		// Parse enum value
		if currentContext == "enum" {
			if matches := enumValueRe.FindStringSubmatch(line); len(matches) > 2 {
				val := protoEnumValue{
					Name:     matches[1],
					Comments: strings.Join(currentComments, " "),
				}
				if currentEnumIdx < len(proto.Enums) {
					proto.Enums[currentEnumIdx].Values = append(proto.Enums[currentEnumIdx].Values, val)
				}
				currentComments = nil
				continue
			}
		}

		// Reset context when closing brace at depth 0
		if braceDepth == 0 && strings.Contains(line, "}") {
			currentContext = ""
			nestedMessages = nil
		} else if braceDepth == 1 && strings.Contains(line, "}") && currentContext == "message" && len(nestedMessages) > 1 {
			nestedMessages = nestedMessages[:len(nestedMessages)-1]
		}
	}

	return proto, nil
}

func (c *ProtobufConverter) buildSkill(proto *protoFile, opts *Options) *skill.Skill {
	name := proto.Package
	if opts != nil && opts.Name != "" {
		name = opts.Name
	}
	if name == "" {
		name = "gRPC Service"
	}

	description := fmt.Sprintf("gRPC service definitions for %s", name)
	if len(proto.Services) > 0 && proto.Services[0].Comments != "" {
		description = proto.Services[0].Comments
	}

	s := skill.NewSkill(name, description)
	s.Frontmatter.SourceType = "proto"
	if opts != nil && opts.SourcePath != "" {
		s.Frontmatter.Source = opts.SourcePath
	}

	// Set metadata
	s.Frontmatter.Protocol = "grpc"
	s.Frontmatter.ServiceCount = len(proto.Services)
	s.Frontmatter.MessageCount = len(proto.Messages)

	// Count endpoints (RPC methods)
	endpointCount := 0
	for _, svc := range proto.Services {
		endpointCount += len(svc.Methods)
	}
	s.Frontmatter.EndpointCount = endpointCount

	// Calculate difficulty
	s.Frontmatter.Difficulty = c.calculateDifficulty(proto)
	s.Frontmatter.HasExamples = true
	s.Frontmatter.Tags = []string{"grpc", "protobuf", "rpc", "api"}

	// MCP-compatible settings
	s.Frontmatter.MCPCompatible = true
	s.Frontmatter.ToolDefinitions = c.buildToolDefinitions(proto)
	s.Frontmatter.RetryStrategy = &skill.RetryStrategy{
		MaxRetries:     3,
		BackoffType:    "exponential",
		InitialDelayMs: 100,
	}

	// Build sections
	s.AddSection("Quick Start", 2, c.buildQuickStart(proto))
	s.AddSection("Overview", 2, c.buildOverview(proto))
	s.AddSection("Services", 2, c.buildServicesSection(proto))
	s.AddSection("Methods", 2, c.buildMethodsSection(proto))
	s.AddSection("Messages", 2, c.buildMessagesSection(proto))

	if len(proto.Enums) > 0 {
		s.AddSection("Enums", 2, c.buildEnumsSection(proto))
	}

	s.AddSection("Code Examples", 2, c.buildCodeExamples(proto))
	s.AddSection("Streaming Patterns", 2, c.buildStreamingPatterns(proto))
	s.AddSection("Tool Definitions", 2, c.buildToolDefinitionsSection(proto))
	s.AddSection("Best Practices", 2, c.buildBestPractices(proto))

	return s
}

func (c *ProtobufConverter) calculateDifficulty(proto *protoFile) string {
	complexity := len(proto.Services) + len(proto.Messages)
	for _, svc := range proto.Services {
		complexity += len(svc.Methods)
		for _, m := range svc.Methods {
			if m.ClientStreaming || m.ServerStreaming {
				complexity += 2 // Streaming adds complexity
			}
		}
	}

	if complexity <= 5 {
		return "novice"
	} else if complexity <= 15 {
		return "intermediate"
	}
	return "advanced"
}

func (c *ProtobufConverter) buildQuickStart(proto *protoFile) string {
	var b strings.Builder

	pkg := proto.Package
	if pkg == "" {
		pkg = "myservice"
	}

	b.WriteString("### Getting Started\n\n")
	b.WriteString("1. **Generate** client code from the `.proto` file\n")
	b.WriteString("2. **Connect** to the gRPC server\n")
	b.WriteString("3. **Call** service methods\n\n")

	b.WriteString("### Quick Example (Python)\n\n")
	b.WriteString("```python\n")
	b.WriteString("import grpc\n")
	b.WriteString(fmt.Sprintf("from %s import %s_pb2, %s_pb2_grpc\n\n", pkg, pkg, pkg))
	b.WriteString("# Connect to server\n")
	b.WriteString("channel = grpc.insecure_channel('localhost:50051')\n")

	if len(proto.Services) > 0 {
		svc := proto.Services[0]
		b.WriteString(fmt.Sprintf("stub = %s_pb2_grpc.%sStub(channel)\n\n", pkg, svc.Name))

		if len(svc.Methods) > 0 {
			m := svc.Methods[0]
			b.WriteString("# Call method\n")
			b.WriteString(fmt.Sprintf("request = %s_pb2.%s()\n", pkg, m.InputType))
			b.WriteString(fmt.Sprintf("response = stub.%s(request)\n", m.Name))
		}
	}
	b.WriteString("```\n")

	return strings.TrimSpace(b.String())
}

func (c *ProtobufConverter) buildOverview(proto *protoFile) string {
	var b strings.Builder

	b.WriteString("| Property | Value |\n")
	b.WriteString("|----------|-------|\n")
	b.WriteString(fmt.Sprintf("| **Syntax** | %s |\n", proto.Syntax))
	if proto.Package != "" {
		b.WriteString(fmt.Sprintf("| **Package** | %s |\n", proto.Package))
	}
	b.WriteString(fmt.Sprintf("| **Services** | %d |\n", len(proto.Services)))

	methodCount := 0
	for _, svc := range proto.Services {
		methodCount += len(svc.Methods)
	}
	b.WriteString(fmt.Sprintf("| **Methods** | %d |\n", methodCount))
	b.WriteString(fmt.Sprintf("| **Messages** | %d |\n", len(proto.Messages)))
	b.WriteString(fmt.Sprintf("| **Enums** | %d |\n", len(proto.Enums)))

	if len(proto.Imports) > 0 {
		b.WriteString(fmt.Sprintf("| **Imports** | %d |\n", len(proto.Imports)))
	}

	return strings.TrimSpace(b.String())
}

func (c *ProtobufConverter) buildServicesSection(proto *protoFile) string {
	var b strings.Builder

	if len(proto.Services) == 0 {
		b.WriteString("No services defined.\n")
		return b.String()
	}

	for _, svc := range proto.Services {
		b.WriteString(fmt.Sprintf("### %s\n\n", svc.Name))

		if svc.Comments != "" {
			b.WriteString(svc.Comments)
			b.WriteString("\n\n")
		}

		b.WriteString(fmt.Sprintf("**Methods**: %d\n\n", len(svc.Methods)))

		if len(svc.Methods) > 0 {
			b.WriteString("| Method | Request | Response | Streaming |\n")
			b.WriteString("|--------|---------|----------|----------|\n")
			for _, m := range svc.Methods {
				streaming := "-"
				if m.ClientStreaming && m.ServerStreaming {
					streaming = "Bidirectional"
				} else if m.ClientStreaming {
					streaming = "Client"
				} else if m.ServerStreaming {
					streaming = "Server"
				}
				b.WriteString(fmt.Sprintf("| `%s` | `%s` | `%s` | %s |\n",
					m.Name, m.InputType, m.OutputType, streaming))
			}
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *ProtobufConverter) buildMethodsSection(proto *protoFile) string {
	var b strings.Builder

	for _, svc := range proto.Services {
		for _, m := range svc.Methods {
			b.WriteString(fmt.Sprintf("### %s.%s\n\n", svc.Name, m.Name))

			if m.Comments != "" {
				b.WriteString(m.Comments)
				b.WriteString("\n\n")
			}

			b.WriteString("**Signature:**\n")
			b.WriteString("```protobuf\n")

			input := m.InputType
			output := m.OutputType
			if m.ClientStreaming {
				input = "stream " + input
			}
			if m.ServerStreaming {
				output = "stream " + output
			}

			b.WriteString(fmt.Sprintf("rpc %s(%s) returns (%s)\n", m.Name, input, output))
			b.WriteString("```\n\n")

			// Add type info
			b.WriteString(fmt.Sprintf("- **Request**: `%s`\n", m.InputType))
			b.WriteString(fmt.Sprintf("- **Response**: `%s`\n", m.OutputType))

			if m.ClientStreaming || m.ServerStreaming {
				if m.ClientStreaming && m.ServerStreaming {
					b.WriteString("- **Streaming**: Bidirectional (client and server)\n")
				} else if m.ClientStreaming {
					b.WriteString("- **Streaming**: Client-side streaming\n")
				} else {
					b.WriteString("- **Streaming**: Server-side streaming\n")
				}
			}
			b.WriteString("\n")
		}
	}

	return strings.TrimSpace(b.String())
}

func (c *ProtobufConverter) buildMessagesSection(proto *protoFile) string {
	var b strings.Builder

	if len(proto.Messages) == 0 {
		b.WriteString("No messages defined.\n")
		return b.String()
	}

	// Sort messages
	msgNames := make([]string, len(proto.Messages))
	msgMap := make(map[string]protoMessage)
	for i, msg := range proto.Messages {
		msgNames[i] = msg.Name
		msgMap[msg.Name] = msg
	}
	sort.Strings(msgNames)

	for _, name := range msgNames {
		msg := msgMap[name]
		b.WriteString(fmt.Sprintf("### %s\n\n", msg.Name))

		if msg.Comments != "" {
			b.WriteString(msg.Comments)
			b.WriteString("\n\n")
		}

		if len(msg.Fields) > 0 {
			b.WriteString("| Field | Type | Description |\n")
			b.WriteString("|-------|------|-------------|\n")
			for _, f := range msg.Fields {
				typeStr := f.Type
				if f.Repeated {
					typeStr = "repeated " + typeStr
				}
				if f.Type == "map" {
					typeStr = fmt.Sprintf("map<%s, %s>", f.MapKey, f.MapValue)
				}
				desc := f.Comments
				if desc == "" {
					desc = "-"
				}
				b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n", f.Name, typeStr, desc))
			}
			b.WriteString("\n")
		}

		// Show proto definition
		b.WriteString("**Definition:**\n")
		b.WriteString("```protobuf\n")
		b.WriteString(fmt.Sprintf("message %s {\n", msg.Name))
		for i, f := range msg.Fields {
			prefix := ""
			if f.Repeated {
				prefix = "repeated "
			} else if f.Optional {
				prefix = "optional "
			}
			typeStr := f.Type
			if f.Type == "map" {
				typeStr = fmt.Sprintf("map<%s, %s>", f.MapKey, f.MapValue)
			}
			b.WriteString(fmt.Sprintf("  %s%s %s = %d;\n", prefix, typeStr, f.Name, i+1))
		}
		b.WriteString("}\n")
		b.WriteString("```\n\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *ProtobufConverter) buildEnumsSection(proto *protoFile) string {
	var b strings.Builder

	for _, enum := range proto.Enums {
		b.WriteString(fmt.Sprintf("### %s\n\n", enum.Name))

		if enum.Comments != "" {
			b.WriteString(enum.Comments)
			b.WriteString("\n\n")
		}

		b.WriteString("| Value | Number | Description |\n")
		b.WriteString("|-------|--------|-------------|\n")
		for _, v := range enum.Values {
			desc := v.Comments
			if desc == "" {
				desc = "-"
			}
			b.WriteString(fmt.Sprintf("| `%s` | %d | %s |\n", v.Name, v.Number, desc))
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *ProtobufConverter) buildCodeExamples(proto *protoFile) string {
	var b strings.Builder

	pkg := proto.Package
	if pkg == "" {
		pkg = "myservice"
	}

	var svcName, methodName, inputType, outputType string
	if len(proto.Services) > 0 {
		svcName = proto.Services[0].Name
		if len(proto.Services[0].Methods) > 0 {
			m := proto.Services[0].Methods[0]
			methodName = m.Name
			inputType = m.InputType
			outputType = m.OutputType
		}
	}

	b.WriteString("### Python\n\n")
	b.WriteString("```python\n")
	b.WriteString("import grpc\n")
	b.WriteString(fmt.Sprintf("from %s import %s_pb2, %s_pb2_grpc\n\n", pkg, pkg, pkg))
	b.WriteString("# Create channel and stub\n")
	b.WriteString("channel = grpc.insecure_channel('localhost:50051')\n")
	b.WriteString(fmt.Sprintf("stub = %s_pb2_grpc.%sStub(channel)\n\n", pkg, svcName))
	b.WriteString("# Create request\n")
	b.WriteString(fmt.Sprintf("request = %s_pb2.%s(\n", pkg, inputType))
	b.WriteString("    # Set fields here\n")
	b.WriteString(")\n\n")
	b.WriteString("# Make call\n")
	b.WriteString(fmt.Sprintf("response = stub.%s(request)\n", methodName))
	b.WriteString(fmt.Sprintf("print(response)  # %s\n", outputType))
	b.WriteString("```\n\n")

	b.WriteString("### Go\n\n")
	b.WriteString("```go\n")
	b.WriteString("package main\n\n")
	b.WriteString("import (\n")
	b.WriteString("    \"context\"\n")
	b.WriteString("    \"google.golang.org/grpc\"\n")
	b.WriteString(fmt.Sprintf("    pb \"%s\"\n", pkg))
	b.WriteString(")\n\n")
	b.WriteString("func main() {\n")
	b.WriteString("    // Connect to server\n")
	b.WriteString("    conn, _ := grpc.Dial(\"localhost:50051\", grpc.WithInsecure())\n")
	b.WriteString("    defer conn.Close()\n\n")
	b.WriteString(fmt.Sprintf("    client := pb.New%sClient(conn)\n\n", svcName))
	b.WriteString("    // Create request\n")
	b.WriteString(fmt.Sprintf("    req := &pb.%s{\n", inputType))
	b.WriteString("        // Set fields here\n")
	b.WriteString("    }\n\n")
	b.WriteString("    // Make call\n")
	b.WriteString(fmt.Sprintf("    resp, _ := client.%s(context.Background(), req)\n", methodName))
	b.WriteString("    fmt.Println(resp)\n")
	b.WriteString("}\n")
	b.WriteString("```\n\n")

	b.WriteString("### JavaScript/Node.js\n\n")
	b.WriteString("```javascript\n")
	b.WriteString("const grpc = require('@grpc/grpc-js');\n")
	b.WriteString("const protoLoader = require('@grpc/proto-loader');\n\n")
	b.WriteString(fmt.Sprintf("const packageDef = protoLoader.loadSync('%s.proto');\n", pkg))
	b.WriteString(fmt.Sprintf("const proto = grpc.loadPackageDefinition(packageDef).%s;\n\n", pkg))
	b.WriteString(fmt.Sprintf("const client = new proto.%s('localhost:50051', grpc.credentials.createInsecure());\n\n", svcName))
	b.WriteString(fmt.Sprintf("client.%s({ /* request fields */ }, (err, response) => {\n", methodName))
	b.WriteString("    console.log(response);\n")
	b.WriteString("});\n")
	b.WriteString("```\n")

	return strings.TrimSpace(b.String())
}

func (c *ProtobufConverter) buildStreamingPatterns(proto *protoFile) string {
	var b strings.Builder

	hasClientStream := false
	hasServerStream := false
	hasBidi := false

	for _, svc := range proto.Services {
		for _, m := range svc.Methods {
			if m.ClientStreaming && m.ServerStreaming {
				hasBidi = true
			} else if m.ClientStreaming {
				hasClientStream = true
			} else if m.ServerStreaming {
				hasServerStream = true
			}
		}
	}

	if hasServerStream {
		b.WriteString("### Server Streaming\n\n")
		b.WriteString("Server sends multiple responses for a single request.\n\n")
		b.WriteString("```python\n")
		b.WriteString("# Python example\n")
		b.WriteString("for response in stub.ServerStreamMethod(request):\n")
		b.WriteString("    print(response)\n")
		b.WriteString("```\n\n")
	}

	if hasClientStream {
		b.WriteString("### Client Streaming\n\n")
		b.WriteString("Client sends multiple requests, server responds once.\n\n")
		b.WriteString("```python\n")
		b.WriteString("# Python example\n")
		b.WriteString("def request_iterator():\n")
		b.WriteString("    for item in items:\n")
		b.WriteString("        yield RequestMessage(data=item)\n\n")
		b.WriteString("response = stub.ClientStreamMethod(request_iterator())\n")
		b.WriteString("```\n\n")
	}

	if hasBidi {
		b.WriteString("### Bidirectional Streaming\n\n")
		b.WriteString("Both client and server send streams of messages.\n\n")
		b.WriteString("```python\n")
		b.WriteString("# Python example\n")
		b.WriteString("def request_iterator():\n")
		b.WriteString("    for item in items:\n")
		b.WriteString("        yield RequestMessage(data=item)\n\n")
		b.WriteString("responses = stub.BidiStreamMethod(request_iterator())\n")
		b.WriteString("for response in responses:\n")
		b.WriteString("    print(response)\n")
		b.WriteString("```\n\n")
	}

	if !hasClientStream && !hasServerStream && !hasBidi {
		b.WriteString("This service uses unary (request-response) calls only.\n\n")
		b.WriteString("```python\n")
		b.WriteString("# Unary call\n")
		b.WriteString("response = stub.UnaryMethod(request)\n")
		b.WriteString("```\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *ProtobufConverter) buildToolDefinitions(proto *protoFile) []skill.ToolDefinition {
	tools := make([]skill.ToolDefinition, 0)

	for _, svc := range proto.Services {
		for _, m := range svc.Methods {
			toolName := fmt.Sprintf("%s_%s", strings.ToLower(svc.Name), strings.ToLower(m.Name))

			desc := m.Comments
			if desc == "" {
				desc = fmt.Sprintf("Call %s.%s RPC method", svc.Name, m.Name)
			}

			params := map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"request": map[string]interface{}{
						"type":        "object",
						"description": fmt.Sprintf("Request message of type %s", m.InputType),
					},
				},
			}

			tools = append(tools, skill.ToolDefinition{
				Name:        toolName,
				Description: desc,
				Parameters:  params,
				Required:    []string{"request"},
			})
		}
	}

	return tools
}

func (c *ProtobufConverter) buildToolDefinitionsSection(proto *protoFile) string {
	var b strings.Builder

	b.WriteString("MCP-compatible tool definitions for AI agents:\n\n")
	b.WriteString("```yaml\n")
	b.WriteString("tools:\n")

	for _, svc := range proto.Services {
		for _, m := range svc.Methods {
			toolName := fmt.Sprintf("%s_%s", strings.ToLower(svc.Name), strings.ToLower(m.Name))
			b.WriteString(fmt.Sprintf("  - name: %s\n", toolName))
			b.WriteString(fmt.Sprintf("    description: Call %s.%s\n", svc.Name, m.Name))
			b.WriteString("    parameters:\n")
			b.WriteString("      type: object\n")
			b.WriteString("      properties:\n")
			b.WriteString("        request:\n")
			b.WriteString("          type: object\n")
			b.WriteString(fmt.Sprintf("          description: %s message\n", m.InputType))
			b.WriteString("      required: [request]\n")
		}
	}

	b.WriteString("```\n")
	return strings.TrimSpace(b.String())
}

func (c *ProtobufConverter) buildBestPractices(proto *protoFile) string {
	var b strings.Builder

	b.WriteString("### Error Handling\n\n")
	b.WriteString("- Use gRPC status codes appropriately\n")
	b.WriteString("- Include error details in `google.rpc.Status`\n")
	b.WriteString("- Handle `UNAVAILABLE` with retry logic\n")
	b.WriteString("- Use deadlines/timeouts on all calls\n\n")

	b.WriteString("### Performance\n\n")
	b.WriteString("- Reuse channels and stubs when possible\n")
	b.WriteString("- Use connection pooling for high throughput\n")
	b.WriteString("- Consider message size limits (default 4MB)\n")
	b.WriteString("- Use compression for large messages\n\n")

	b.WriteString("### Schema Evolution\n\n")
	b.WriteString("- Never change field numbers\n")
	b.WriteString("- Mark deprecated fields with `[deprecated = true]`\n")
	b.WriteString("- Use `reserved` for removed fields\n")
	b.WriteString("- Add new fields as optional\n\n")

	b.WriteString("### Security\n\n")
	b.WriteString("- Use TLS for production (not `insecure_channel`)\n")
	b.WriteString("- Implement authentication interceptors\n")
	b.WriteString("- Validate input messages\n")
	b.WriteString("- Use per-RPC credentials when needed\n")

	return strings.TrimSpace(b.String())
}
