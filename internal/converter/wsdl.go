// Package converter provides spec-to-SKILL.md converters.
package converter

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"

	"github.com/sanixdarker/skillforge/pkg/skill"
)

// WSDLConverter converts WSDL specifications to skills.
type WSDLConverter struct{}

// WSDL XML types
type wsdlDefinitions struct {
	XMLName      xml.Name       `xml:"definitions"`
	Name         string         `xml:"name,attr"`
	TargetNS     string         `xml:"targetNamespace,attr"`
	Documentation string        `xml:"documentation"`
	Types        wsdlTypes      `xml:"types"`
	Messages     []wsdlMessage  `xml:"message"`
	PortTypes    []wsdlPortType `xml:"portType"`
	Bindings     []wsdlBinding  `xml:"binding"`
	Services     []wsdlService  `xml:"service"`
}

type wsdlTypes struct {
	Schemas []xsdSchema `xml:"schema"`
}

type xsdSchema struct {
	TargetNS     string           `xml:"targetNamespace,attr"`
	Elements     []xsdElement     `xml:"element"`
	ComplexTypes []xsdComplexType `xml:"complexType"`
	SimpleTypes  []xsdSimpleType  `xml:"simpleType"`
}

type xsdElement struct {
	Name      string `xml:"name,attr"`
	Type      string `xml:"type,attr"`
	MinOccurs string `xml:"minOccurs,attr"`
	MaxOccurs string `xml:"maxOccurs,attr"`
	Nillable  string `xml:"nillable,attr"`
	ComplexType *xsdComplexType `xml:"complexType"`
}

type xsdComplexType struct {
	Name       string       `xml:"name,attr"`
	Sequence   *xsdSequence `xml:"sequence"`
	All        *xsdSequence `xml:"all"`
	Annotation *xsdAnnotation `xml:"annotation"`
}

type xsdSequence struct {
	Elements []xsdElement `xml:"element"`
}

type xsdSimpleType struct {
	Name        string          `xml:"name,attr"`
	Restriction *xsdRestriction `xml:"restriction"`
}

type xsdRestriction struct {
	Base        string           `xml:"base,attr"`
	Enumerations []xsdEnumeration `xml:"enumeration"`
	MinLength   *xsdFacet        `xml:"minLength"`
	MaxLength   *xsdFacet        `xml:"maxLength"`
	Pattern     *xsdFacet        `xml:"pattern"`
}

type xsdEnumeration struct {
	Value string `xml:"value,attr"`
}

type xsdFacet struct {
	Value string `xml:"value,attr"`
}

type xsdAnnotation struct {
	Documentation string `xml:"documentation"`
}

type wsdlMessage struct {
	Name  string       `xml:"name,attr"`
	Parts []wsdlPart   `xml:"part"`
}

type wsdlPart struct {
	Name    string `xml:"name,attr"`
	Element string `xml:"element,attr"`
	Type    string `xml:"type,attr"`
}

type wsdlPortType struct {
	Name       string          `xml:"name,attr"`
	Operations []wsdlOperation `xml:"operation"`
}

type wsdlOperation struct {
	Name          string       `xml:"name,attr"`
	Documentation string       `xml:"documentation"`
	Input         *wsdlParam   `xml:"input"`
	Output        *wsdlParam   `xml:"output"`
	Fault         []wsdlFault  `xml:"fault"`
}

type wsdlParam struct {
	Name    string `xml:"name,attr"`
	Message string `xml:"message,attr"`
}

type wsdlFault struct {
	Name    string `xml:"name,attr"`
	Message string `xml:"message,attr"`
}

type wsdlBinding struct {
	Name       string              `xml:"name,attr"`
	Type       string              `xml:"type,attr"`
	SoapBinding *soapBinding       `xml:"binding"`
	Operations []wsdlBindingOp     `xml:"operation"`
}

type soapBinding struct {
	Style     string `xml:"style,attr"`
	Transport string `xml:"transport,attr"`
}

type wsdlBindingOp struct {
	Name          string         `xml:"name,attr"`
	SoapOperation *soapOperation `xml:"operation"`
}

type soapOperation struct {
	SoapAction string `xml:"soapAction,attr"`
}

type wsdlService struct {
	Name          string      `xml:"name,attr"`
	Documentation string      `xml:"documentation"`
	Ports         []wsdlPort  `xml:"port"`
}

type wsdlPort struct {
	Name    string      `xml:"name,attr"`
	Binding string      `xml:"binding,attr"`
	Address *soapAddress `xml:"address"`
}

type soapAddress struct {
	Location string `xml:"location,attr"`
}

func (c *WSDLConverter) Name() string {
	return "wsdl"
}

func (c *WSDLConverter) CanHandle(filename string, content []byte) bool {
	ext := getExtension(filename)
	if ext == ".wsdl" {
		return true
	}
	// Check for WSDL content
	return bytes.Contains(content, []byte("<definitions")) &&
		(bytes.Contains(content, []byte("wsdl")) || bytes.Contains(content, []byte("schemas.xmlsoap.org")))
}

func (c *WSDLConverter) Convert(content []byte, opts *Options) (*skill.Skill, error) {
	var wsdl wsdlDefinitions
	if err := xml.Unmarshal(content, &wsdl); err != nil {
		return nil, fmt.Errorf("failed to parse WSDL: %w", err)
	}

	return c.buildSkill(&wsdl, opts), nil
}

func (c *WSDLConverter) buildSkill(wsdl *wsdlDefinitions, opts *Options) *skill.Skill {
	name := wsdl.Name
	if opts != nil && opts.Name != "" {
		name = opts.Name
	}
	if name == "" && len(wsdl.Services) > 0 {
		name = wsdl.Services[0].Name
	}
	if name == "" {
		name = "SOAP Web Service"
	}

	description := wsdl.Documentation
	if description == "" && len(wsdl.Services) > 0 && wsdl.Services[0].Documentation != "" {
		description = wsdl.Services[0].Documentation
	}
	if description == "" {
		description = fmt.Sprintf("SOAP web service documentation for %s", name)
	}

	s := skill.NewSkill(name, description)
	s.Frontmatter.SourceType = "wsdl"
	if opts != nil && opts.SourcePath != "" {
		s.Frontmatter.Source = opts.SourcePath
	}

	// Set metadata
	s.Frontmatter.Protocol = "soap"

	// Extract endpoint URL
	if len(wsdl.Services) > 0 && len(wsdl.Services[0].Ports) > 0 {
		if addr := wsdl.Services[0].Ports[0].Address; addr != nil {
			s.Frontmatter.BaseURL = addr.Location
		}
	}

	// Count operations
	opCount := 0
	for _, pt := range wsdl.PortTypes {
		opCount += len(pt.Operations)
	}
	s.Frontmatter.EndpointCount = opCount
	s.Frontmatter.ServiceCount = len(wsdl.Services)

	// Calculate difficulty
	s.Frontmatter.Difficulty = c.calculateDifficulty(wsdl)
	s.Frontmatter.HasExamples = true
	s.Frontmatter.Tags = []string{"soap", "wsdl", "xml", "web-service"}

	// MCP-compatible settings
	s.Frontmatter.MCPCompatible = true
	s.Frontmatter.ToolDefinitions = c.buildToolDefinitions(wsdl)
	s.Frontmatter.RetryStrategy = &skill.RetryStrategy{
		MaxRetries:     3,
		BackoffType:    "exponential",
		InitialDelayMs: 1000,
	}

	// Build sections
	s.AddSection("Quick Start", 2, c.buildQuickStart(wsdl))
	s.AddSection("Overview", 2, c.buildOverview(wsdl))
	s.AddSection("Services", 2, c.buildServicesSection(wsdl))
	s.AddSection("Ports", 2, c.buildPortsSection(wsdl))
	s.AddSection("Operations", 2, c.buildOperationsSection(wsdl))
	s.AddSection("Messages", 2, c.buildMessagesSection(wsdl))
	s.AddSection("Types", 2, c.buildTypesSection(wsdl))
	s.AddSection("Code Examples", 2, c.buildCodeExamples(wsdl))
	s.AddSection("Tool Definitions", 2, c.buildToolDefinitionsSection(wsdl))
	s.AddSection("Best Practices", 2, c.buildBestPractices())

	return s
}

func (c *WSDLConverter) calculateDifficulty(wsdl *wsdlDefinitions) string {
	opCount := 0
	for _, pt := range wsdl.PortTypes {
		opCount += len(pt.Operations)
	}
	typeCount := 0
	for _, schema := range wsdl.Types.Schemas {
		typeCount += len(schema.ComplexTypes) + len(schema.SimpleTypes)
	}

	complexity := opCount + typeCount
	if complexity <= 5 {
		return "novice"
	} else if complexity <= 15 {
		return "intermediate"
	}
	return "advanced"
}

func (c *WSDLConverter) buildQuickStart(wsdl *wsdlDefinitions) string {
	var b strings.Builder

	var endpoint string
	if len(wsdl.Services) > 0 && len(wsdl.Services[0].Ports) > 0 {
		if addr := wsdl.Services[0].Ports[0].Address; addr != nil {
			endpoint = addr.Location
		}
	}

	var firstOp string
	var soapAction string
	if len(wsdl.PortTypes) > 0 && len(wsdl.PortTypes[0].Operations) > 0 {
		firstOp = wsdl.PortTypes[0].Operations[0].Name
	}
	// Find soap action
	for _, binding := range wsdl.Bindings {
		for _, op := range binding.Operations {
			if op.Name == firstOp && op.SoapOperation != nil {
				soapAction = op.SoapOperation.SoapAction
				break
			}
		}
	}

	b.WriteString("### Getting Started\n\n")
	b.WriteString(fmt.Sprintf("1. **Endpoint**: `%s`\n", endpoint))
	b.WriteString("2. **Construct** SOAP envelope with your request\n")
	b.WriteString("3. **Send** HTTP POST with Content-Type: text/xml\n\n")

	b.WriteString("### Quick Example (cURL)\n\n")
	b.WriteString("```bash\n")
	b.WriteString(fmt.Sprintf("curl -X POST \"%s\" \\\n", endpoint))
	b.WriteString("  -H \"Content-Type: text/xml; charset=utf-8\" \\\n")
	if soapAction != "" {
		b.WriteString(fmt.Sprintf("  -H \"SOAPAction: \\\"%s\\\"\" \\\n", soapAction))
	}
	b.WriteString("  -d '<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<soap:Envelope xmlns:soap=\"http://schemas.xmlsoap.org/soap/envelope/\">\n")
	b.WriteString("  <soap:Body>\n")
	b.WriteString(fmt.Sprintf("    <%s xmlns=\"%s\">\n", firstOp, wsdl.TargetNS))
	b.WriteString("      <!-- Request parameters -->\n")
	b.WriteString(fmt.Sprintf("    </%s>\n", firstOp))
	b.WriteString("  </soap:Body>\n")
	b.WriteString("</soap:Envelope>'\n")
	b.WriteString("```\n")

	return strings.TrimSpace(b.String())
}

func (c *WSDLConverter) buildOverview(wsdl *wsdlDefinitions) string {
	var b strings.Builder

	if wsdl.Documentation != "" {
		b.WriteString(wsdl.Documentation)
		b.WriteString("\n\n")
	}

	b.WriteString("| Property | Value |\n")
	b.WriteString("|----------|-------|\n")

	if wsdl.Name != "" {
		b.WriteString(fmt.Sprintf("| **Name** | %s |\n", wsdl.Name))
	}
	if wsdl.TargetNS != "" {
		b.WriteString(fmt.Sprintf("| **Namespace** | %s |\n", wsdl.TargetNS))
	}

	b.WriteString(fmt.Sprintf("| **Services** | %d |\n", len(wsdl.Services)))
	b.WriteString(fmt.Sprintf("| **Port Types** | %d |\n", len(wsdl.PortTypes)))

	opCount := 0
	for _, pt := range wsdl.PortTypes {
		opCount += len(pt.Operations)
	}
	b.WriteString(fmt.Sprintf("| **Operations** | %d |\n", opCount))
	b.WriteString(fmt.Sprintf("| **Messages** | %d |\n", len(wsdl.Messages)))

	// Binding style
	for _, binding := range wsdl.Bindings {
		if binding.SoapBinding != nil {
			b.WriteString(fmt.Sprintf("| **Style** | %s |\n", binding.SoapBinding.Style))
			break
		}
	}

	return strings.TrimSpace(b.String())
}

func (c *WSDLConverter) buildServicesSection(wsdl *wsdlDefinitions) string {
	var b strings.Builder

	if len(wsdl.Services) == 0 {
		b.WriteString("No services defined.\n")
		return b.String()
	}

	for _, svc := range wsdl.Services {
		b.WriteString(fmt.Sprintf("### %s\n\n", svc.Name))

		if svc.Documentation != "" {
			b.WriteString(svc.Documentation)
			b.WriteString("\n\n")
		}

		b.WriteString("**Ports:**\n")
		for _, port := range svc.Ports {
			b.WriteString(fmt.Sprintf("- **%s**\n", port.Name))
			if port.Address != nil {
				b.WriteString(fmt.Sprintf("  - Endpoint: `%s`\n", port.Address.Location))
			}
			b.WriteString(fmt.Sprintf("  - Binding: `%s`\n", c.localName(port.Binding)))
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *WSDLConverter) buildPortsSection(wsdl *wsdlDefinitions) string {
	var b strings.Builder

	if len(wsdl.PortTypes) == 0 {
		b.WriteString("No port types defined.\n")
		return b.String()
	}

	for _, pt := range wsdl.PortTypes {
		b.WriteString(fmt.Sprintf("### %s\n\n", pt.Name))

		b.WriteString("| Operation | Input | Output |\n")
		b.WriteString("|-----------|-------|--------|\n")

		for _, op := range pt.Operations {
			input := "-"
			output := "-"
			if op.Input != nil {
				input = c.localName(op.Input.Message)
			}
			if op.Output != nil {
				output = c.localName(op.Output.Message)
			}
			b.WriteString(fmt.Sprintf("| `%s` | `%s` | `%s` |\n", op.Name, input, output))
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *WSDLConverter) buildOperationsSection(wsdl *wsdlDefinitions) string {
	var b strings.Builder

	// Build map of SOAP actions
	soapActions := make(map[string]string)
	for _, binding := range wsdl.Bindings {
		for _, op := range binding.Operations {
			if op.SoapOperation != nil {
				soapActions[op.Name] = op.SoapOperation.SoapAction
			}
		}
	}

	for _, pt := range wsdl.PortTypes {
		for _, op := range pt.Operations {
			b.WriteString(fmt.Sprintf("### %s\n\n", op.Name))

			if op.Documentation != "" {
				b.WriteString(op.Documentation)
				b.WriteString("\n\n")
			}

			if action, ok := soapActions[op.Name]; ok && action != "" {
				b.WriteString(fmt.Sprintf("**SOAPAction**: `%s`\n\n", action))
			}

			// Input message
			if op.Input != nil {
				msgName := c.localName(op.Input.Message)
				b.WriteString("**Request Message**: ")
				b.WriteString(fmt.Sprintf("`%s`\n\n", msgName))

				// Show message structure
				for _, msg := range wsdl.Messages {
					if msg.Name == msgName {
						if len(msg.Parts) > 0 {
							b.WriteString("| Part | Element/Type |\n")
							b.WriteString("|------|-------------|\n")
							for _, part := range msg.Parts {
								elemType := part.Element
								if elemType == "" {
									elemType = part.Type
								}
								b.WriteString(fmt.Sprintf("| `%s` | `%s` |\n", part.Name, c.localName(elemType)))
							}
							b.WriteString("\n")
						}
						break
					}
				}
			}

			// Output message
			if op.Output != nil {
				msgName := c.localName(op.Output.Message)
				b.WriteString("**Response Message**: ")
				b.WriteString(fmt.Sprintf("`%s`\n\n", msgName))
			}

			// Faults
			if len(op.Fault) > 0 {
				b.WriteString("**Faults**:\n")
				for _, fault := range op.Fault {
					b.WriteString(fmt.Sprintf("- `%s`: %s\n", fault.Name, c.localName(fault.Message)))
				}
				b.WriteString("\n")
			}

			// SOAP envelope example
			b.WriteString("**Example Request:**\n")
			b.WriteString("```xml\n")
			b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
			b.WriteString("<soap:Envelope xmlns:soap=\"http://schemas.xmlsoap.org/soap/envelope/\"\n")
			b.WriteString(fmt.Sprintf("               xmlns:tns=\"%s\">\n", wsdl.TargetNS))
			b.WriteString("  <soap:Header>\n")
			b.WriteString("    <!-- Optional headers -->\n")
			b.WriteString("  </soap:Header>\n")
			b.WriteString("  <soap:Body>\n")
			b.WriteString(fmt.Sprintf("    <tns:%s>\n", op.Name))
			b.WriteString("      <!-- Request parameters -->\n")
			b.WriteString(fmt.Sprintf("    </tns:%s>\n", op.Name))
			b.WriteString("  </soap:Body>\n")
			b.WriteString("</soap:Envelope>\n")
			b.WriteString("```\n\n")
		}
	}

	return strings.TrimSpace(b.String())
}

func (c *WSDLConverter) buildMessagesSection(wsdl *wsdlDefinitions) string {
	var b strings.Builder

	if len(wsdl.Messages) == 0 {
		b.WriteString("No messages defined.\n")
		return b.String()
	}

	// Sort messages
	msgNames := make([]string, len(wsdl.Messages))
	msgMap := make(map[string]wsdlMessage)
	for i, msg := range wsdl.Messages {
		msgNames[i] = msg.Name
		msgMap[msg.Name] = msg
	}
	sort.Strings(msgNames)

	for _, name := range msgNames {
		msg := msgMap[name]
		b.WriteString(fmt.Sprintf("### %s\n\n", msg.Name))

		if len(msg.Parts) == 0 {
			b.WriteString("No parts defined.\n\n")
			continue
		}

		b.WriteString("| Part | Element/Type |\n")
		b.WriteString("|------|-------------|\n")
		for _, part := range msg.Parts {
			elemType := part.Element
			if elemType == "" {
				elemType = part.Type
			}
			b.WriteString(fmt.Sprintf("| `%s` | `%s` |\n", part.Name, c.localName(elemType)))
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *WSDLConverter) buildTypesSection(wsdl *wsdlDefinitions) string {
	var b strings.Builder

	if len(wsdl.Types.Schemas) == 0 {
		b.WriteString("No types defined.\n")
		return b.String()
	}

	for _, schema := range wsdl.Types.Schemas {
		// Elements
		if len(schema.Elements) > 0 {
			b.WriteString("### Elements\n\n")
			for _, elem := range schema.Elements {
				b.WriteString(fmt.Sprintf("#### %s\n\n", elem.Name))

				if elem.Type != "" {
					b.WriteString(fmt.Sprintf("Type: `%s`\n\n", c.localName(elem.Type)))
				}

				if elem.ComplexType != nil {
					c.writeComplexType(&b, elem.ComplexType)
				}
			}
		}

		// Complex types
		if len(schema.ComplexTypes) > 0 {
			b.WriteString("### Complex Types\n\n")
			for _, ct := range schema.ComplexTypes {
				b.WriteString(fmt.Sprintf("#### %s\n\n", ct.Name))
				c.writeComplexType(&b, &ct)
			}
		}

		// Simple types (enums, restrictions)
		if len(schema.SimpleTypes) > 0 {
			b.WriteString("### Simple Types\n\n")
			for _, st := range schema.SimpleTypes {
				b.WriteString(fmt.Sprintf("#### %s\n\n", st.Name))

				if st.Restriction != nil {
					b.WriteString(fmt.Sprintf("Base: `%s`\n\n", c.localName(st.Restriction.Base)))

					if len(st.Restriction.Enumerations) > 0 {
						b.WriteString("Allowed values:\n")
						for _, enum := range st.Restriction.Enumerations {
							b.WriteString(fmt.Sprintf("- `%s`\n", enum.Value))
						}
						b.WriteString("\n")
					}

					if st.Restriction.Pattern != nil {
						b.WriteString(fmt.Sprintf("Pattern: `%s`\n\n", st.Restriction.Pattern.Value))
					}
				}
			}
		}
	}

	return strings.TrimSpace(b.String())
}

func (c *WSDLConverter) writeComplexType(b *strings.Builder, ct *xsdComplexType) {
	if ct.Annotation != nil && ct.Annotation.Documentation != "" {
		b.WriteString(ct.Annotation.Documentation)
		b.WriteString("\n\n")
	}

	var elements []xsdElement
	if ct.Sequence != nil {
		elements = ct.Sequence.Elements
	} else if ct.All != nil {
		elements = ct.All.Elements
	}

	if len(elements) > 0 {
		b.WriteString("| Field | Type | Required |\n")
		b.WriteString("|-------|------|----------|\n")
		for _, elem := range elements {
			required := "Yes"
			if elem.MinOccurs == "0" || elem.Nillable == "true" {
				required = "No"
			}
			elemType := elem.Type
			if elemType == "" {
				elemType = "complex"
			}
			b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n", elem.Name, c.localName(elemType), required))
		}
		b.WriteString("\n")
	}
}

func (c *WSDLConverter) buildCodeExamples(wsdl *wsdlDefinitions) string {
	var b strings.Builder

	var endpoint string
	if len(wsdl.Services) > 0 && len(wsdl.Services[0].Ports) > 0 {
		if addr := wsdl.Services[0].Ports[0].Address; addr != nil {
			endpoint = addr.Location
		}
	}

	var firstOp string
	var soapAction string
	if len(wsdl.PortTypes) > 0 && len(wsdl.PortTypes[0].Operations) > 0 {
		firstOp = wsdl.PortTypes[0].Operations[0].Name
	}
	for _, binding := range wsdl.Bindings {
		for _, op := range binding.Operations {
			if op.Name == firstOp && op.SoapOperation != nil {
				soapAction = op.SoapOperation.SoapAction
				break
			}
		}
	}

	b.WriteString("### Python (zeep)\n\n")
	b.WriteString("```python\n")
	b.WriteString("from zeep import Client\n\n")
	b.WriteString(fmt.Sprintf("# Load WSDL\nclient = Client('%s?wsdl')\n\n", endpoint))
	b.WriteString(fmt.Sprintf("# Call operation\nresult = client.service.%s(\n", firstOp))
	b.WriteString("    # Add parameters here\n")
	b.WriteString(")\nprint(result)\n")
	b.WriteString("```\n\n")

	b.WriteString("### Python (requests - raw SOAP)\n\n")
	b.WriteString("```python\n")
	b.WriteString("import requests\n\n")
	b.WriteString("soap_envelope = '''<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<soap:Envelope xmlns:soap=\"http://schemas.xmlsoap.org/soap/envelope/\"\n")
	b.WriteString(fmt.Sprintf("               xmlns:tns=\"%s\">\n", wsdl.TargetNS))
	b.WriteString("  <soap:Body>\n")
	b.WriteString(fmt.Sprintf("    <tns:%s>\n", firstOp))
	b.WriteString("      <!-- Parameters -->\n")
	b.WriteString(fmt.Sprintf("    </tns:%s>\n", firstOp))
	b.WriteString("  </soap:Body>\n")
	b.WriteString("</soap:Envelope>'''\n\n")
	b.WriteString("headers = {\n")
	b.WriteString("    'Content-Type': 'text/xml; charset=utf-8',\n")
	if soapAction != "" {
		b.WriteString(fmt.Sprintf("    'SOAPAction': '\"%s\"',\n", soapAction))
	}
	b.WriteString("}\n\n")
	b.WriteString(fmt.Sprintf("response = requests.post('%s', data=soap_envelope, headers=headers)\n", endpoint))
	b.WriteString("print(response.text)\n")
	b.WriteString("```\n\n")

	b.WriteString("### JavaScript (soap)\n\n")
	b.WriteString("```javascript\n")
	b.WriteString("const soap = require('soap');\n\n")
	b.WriteString(fmt.Sprintf("soap.createClient('%s?wsdl', (err, client) => {\n", endpoint))
	b.WriteString(fmt.Sprintf("  client.%s({ /* parameters */ }, (err, result) => {\n", firstOp))
	b.WriteString("    console.log(result);\n")
	b.WriteString("  });\n")
	b.WriteString("});\n")
	b.WriteString("```\n\n")

	b.WriteString("### Go (net/http)\n\n")
	b.WriteString("```go\n")
	b.WriteString("package main\n\n")
	b.WriteString("import (\n    \"bytes\"\n    \"net/http\"\n)\n\n")
	b.WriteString("func main() {\n")
	b.WriteString("    envelope := `<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<soap:Envelope xmlns:soap=\"http://schemas.xmlsoap.org/soap/envelope/\">\n")
	b.WriteString("  <soap:Body>\n")
	b.WriteString(fmt.Sprintf("    <%s xmlns=\"%s\">\n", firstOp, wsdl.TargetNS))
	b.WriteString(fmt.Sprintf("    </%s>\n", firstOp))
	b.WriteString("  </soap:Body>\n")
	b.WriteString("</soap:Envelope>`\n\n")
	b.WriteString(fmt.Sprintf("    req, _ := http.NewRequest(\"POST\", \"%s\", bytes.NewBufferString(envelope))\n", endpoint))
	b.WriteString("    req.Header.Set(\"Content-Type\", \"text/xml; charset=utf-8\")\n")
	if soapAction != "" {
		b.WriteString(fmt.Sprintf("    req.Header.Set(\"SOAPAction\", \"\\\"%s\\\"\")\n", soapAction))
	}
	b.WriteString("\n    resp, _ := http.DefaultClient.Do(req)\n")
	b.WriteString("    defer resp.Body.Close()\n")
	b.WriteString("}\n")
	b.WriteString("```\n")

	return strings.TrimSpace(b.String())
}

func (c *WSDLConverter) buildToolDefinitions(wsdl *wsdlDefinitions) []skill.ToolDefinition {
	tools := make([]skill.ToolDefinition, 0)

	for _, pt := range wsdl.PortTypes {
		for _, op := range pt.Operations {
			toolName := strings.ToLower(op.Name)

			desc := op.Documentation
			if desc == "" {
				desc = fmt.Sprintf("Call SOAP operation %s", op.Name)
			}

			params := map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"parameters": map[string]interface{}{
						"type":        "object",
						"description": "SOAP request parameters",
					},
				},
			}

			tools = append(tools, skill.ToolDefinition{
				Name:        toolName,
				Description: truncate(desc, 200),
				Parameters:  params,
				Required:    []string{"parameters"},
			})
		}
	}

	return tools
}

func (c *WSDLConverter) buildToolDefinitionsSection(wsdl *wsdlDefinitions) string {
	var b strings.Builder

	b.WriteString("MCP-compatible tool definitions for AI agents:\n\n")
	b.WriteString("```yaml\n")
	b.WriteString("tools:\n")

	for _, pt := range wsdl.PortTypes {
		for _, op := range pt.Operations {
			b.WriteString(fmt.Sprintf("  - name: %s\n", strings.ToLower(op.Name)))
			desc := op.Documentation
			if desc == "" {
				desc = fmt.Sprintf("Call %s operation", op.Name)
			}
			b.WriteString(fmt.Sprintf("    description: %s\n", truncate(desc, 60)))
			b.WriteString("    parameters:\n")
			b.WriteString("      type: object\n")
			b.WriteString("      properties:\n")
			b.WriteString("        request:\n")
			b.WriteString("          type: object\n")
			b.WriteString("          description: SOAP request parameters\n")
			b.WriteString("      required: [request]\n")
		}
	}

	b.WriteString("```\n")
	return strings.TrimSpace(b.String())
}

func (c *WSDLConverter) buildBestPractices() string {
	var b strings.Builder

	b.WriteString("### SOAP Request Construction\n\n")
	b.WriteString("- Always include the correct namespace declarations\n")
	b.WriteString("- Use proper XML encoding (UTF-8)\n")
	b.WriteString("- Include SOAPAction header when required\n")
	b.WriteString("- Validate XML against schema before sending\n\n")

	b.WriteString("### Error Handling\n\n")
	b.WriteString("- Check for SOAP Fault responses\n")
	b.WriteString("- Parse fault codes and messages\n")
	b.WriteString("- Implement retry logic for transient failures\n")
	b.WriteString("- Log full request/response for debugging\n\n")

	b.WriteString("### Security (WS-Security)\n\n")
	b.WriteString("- Use HTTPS for transport security\n")
	b.WriteString("- Add WS-Security headers when required\n")
	b.WriteString("- Sign messages for integrity\n")
	b.WriteString("- Encrypt sensitive data in payload\n\n")

	b.WriteString("### Performance\n\n")
	b.WriteString("- Reuse HTTP connections\n")
	b.WriteString("- Consider using MTOM for large attachments\n")
	b.WriteString("- Cache WSDL client instances\n")
	b.WriteString("- Set appropriate timeouts\n")

	return strings.TrimSpace(b.String())
}

func (c *WSDLConverter) localName(qname string) string {
	if idx := strings.LastIndex(qname, ":"); idx >= 0 {
		return qname[idx+1:]
	}
	return qname
}
