package converter

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/sanixdarker/skill-md/pkg/skill"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// GraphQLConverter converts GraphQL schemas to SKILL.md.
type GraphQLConverter struct{}

func (c *GraphQLConverter) Name() string {
	return "graphql"
}

func (c *GraphQLConverter) CanHandle(filename string, content []byte) bool {
	ext := getExtension(filename)
	if ext == ".graphql" || ext == ".gql" {
		return true
	}
	// Check for GraphQL indicators
	return bytes.Contains(content, []byte("type Query")) ||
		bytes.Contains(content, []byte("type Mutation")) ||
		bytes.Contains(content, []byte("schema {"))
}

func (c *GraphQLConverter) Convert(content []byte, opts *Options) (*skill.Skill, error) {
	schema, err := gqlparser.LoadSchema(&ast.Source{
		Name:  "schema.graphql",
		Input: string(content),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL schema: %w", err)
	}

	s := c.buildSkill(schema, opts)
	return s, nil
}

func (c *GraphQLConverter) buildSkill(schema *ast.Schema, opts *Options) *skill.Skill {
	name := "GraphQL API Skill"
	if opts != nil && opts.Name != "" {
		name = opts.Name
	}

	s := skill.NewSkill(name, "GraphQL API schema and operations")
	s.Frontmatter.SourceType = "graphql"
	s.Frontmatter.Tags = []string{"graphql", "api"}
	if opts != nil && opts.SourcePath != "" {
		s.Frontmatter.Source = opts.SourcePath
	}

	// Count operations for metadata
	queryCount := 0
	mutationCount := 0
	if schema.Query != nil {
		queryCount = len(schema.Query.Fields)
	}
	if schema.Mutation != nil {
		mutationCount = len(schema.Mutation.Fields)
	}
	s.Frontmatter.EndpointCount = queryCount + mutationCount

	// Determine difficulty
	typeCount := len(schema.Types)
	if queryCount+mutationCount <= 5 && typeCount <= 10 {
		s.Frontmatter.Difficulty = "novice"
	} else if queryCount+mutationCount <= 20 && typeCount <= 30 {
		s.Frontmatter.Difficulty = "intermediate"
	} else {
		s.Frontmatter.Difficulty = "advanced"
	}

	s.Frontmatter.HasExamples = true // We generate examples

	// Add Quick Start section
	s.AddSection("Quick Start", 2, c.buildQuickStart(schema))

	// Add overview
	s.AddSection("Overview", 2, c.buildOverview(schema))

	// Add queries section
	if schema.Query != nil {
		s.AddSection("Queries", 2, c.buildOperationsSection(schema.Query, "query", schema))
	}

	// Add mutations section
	if schema.Mutation != nil {
		s.AddSection("Mutations", 2, c.buildOperationsSection(schema.Mutation, "mutation", schema))
	}

	// Add subscriptions section
	if schema.Subscription != nil {
		s.AddSection("Subscriptions", 2, c.buildOperationsSection(schema.Subscription, "subscription", schema))
	}

	// Add types section
	s.AddSection("Types", 2, c.buildTypesSection(schema))

	// Add directives section if custom directives exist
	if directivesContent := c.buildDirectivesSection(schema); directivesContent != "" {
		s.AddSection("Directives", 2, directivesContent)
	}

	// Add Best Practices section
	s.AddSection("Best Practices", 2, c.buildBestPracticesSection())

	return s
}

func (c *GraphQLConverter) buildQuickStart(schema *ast.Schema) string {
	var b strings.Builder

	b.WriteString("Get started with this GraphQL API quickly.\n\n")

	b.WriteString("### 1. GraphQL Endpoint\n\n")
	b.WriteString("```\nPOST /graphql\nContent-Type: application/json\n```\n\n")

	b.WriteString("### 2. Basic Query Structure\n\n")
	b.WriteString("```graphql\n")
	b.WriteString("query {\n")
	b.WriteString("  # Your query here\n")
	b.WriteString("}\n")
	b.WriteString("```\n\n")

	// Show a simple example query if available
	if schema.Query != nil && len(schema.Query.Fields) > 0 {
		b.WriteString("### 3. Your First Query\n\n")

		// Find a simple query (no required args)
		var simpleField *ast.FieldDefinition
		for _, field := range schema.Query.Fields {
			if len(field.Arguments) == 0 || !c.hasRequiredArgs(field.Arguments) {
				simpleField = field
				break
			}
		}

		if simpleField == nil && len(schema.Query.Fields) > 0 {
			simpleField = schema.Query.Fields[0]
		}

		if simpleField != nil {
			b.WriteString("**GraphQL:**\n")
			b.WriteString("```graphql\n")
			b.WriteString("query {\n")
			b.WriteString(fmt.Sprintf("  %s", simpleField.Name))
			if len(simpleField.Arguments) > 0 {
				args := c.buildExampleArgs(simpleField.Arguments)
				b.WriteString(fmt.Sprintf("(%s)", args))
			}
			b.WriteString(" {\n")
			b.WriteString(c.buildExampleFields(simpleField.Type, schema, "    "))
			b.WriteString("  }\n")
			b.WriteString("}\n")
			b.WriteString("```\n\n")

			// JavaScript example
			b.WriteString("**JavaScript (fetch):**\n")
			b.WriteString("```javascript\n")
			b.WriteString("const response = await fetch('/graphql', {\n")
			b.WriteString("  method: 'POST',\n")
			b.WriteString("  headers: { 'Content-Type': 'application/json' },\n")
			b.WriteString("  body: JSON.stringify({\n")
			b.WriteString(fmt.Sprintf("    query: `query { %s { id } }`\n", simpleField.Name))
			b.WriteString("  })\n")
			b.WriteString("});\n")
			b.WriteString("const { data } = await response.json();\n")
			b.WriteString("```\n\n")

			// cURL example
			b.WriteString("**cURL:**\n")
			b.WriteString("```bash\n")
			b.WriteString("curl -X POST \\\n")
			b.WriteString("  -H \"Content-Type: application/json\" \\\n")
			b.WriteString(fmt.Sprintf("  -d '{\"query\": \"{ %s { id } }\"}' \\\n", simpleField.Name))
			b.WriteString("  https://api.example.com/graphql\n")
			b.WriteString("```\n")
		}
	}

	return strings.TrimSpace(b.String())
}

func (c *GraphQLConverter) buildOverview(schema *ast.Schema) string {
	var b strings.Builder

	b.WriteString("This skill describes a GraphQL API with its queries, mutations, and types.\n\n")

	// Schema statistics
	queryCount := 0
	mutationCount := 0
	subscriptionCount := 0
	typeCount := 0

	if schema.Query != nil {
		queryCount = len(schema.Query.Fields)
	}
	if schema.Mutation != nil {
		mutationCount = len(schema.Mutation.Fields)
	}
	if schema.Subscription != nil {
		subscriptionCount = len(schema.Subscription.Fields)
	}

	for name := range schema.Types {
		if !strings.HasPrefix(name, "__") &&
			name != "Query" && name != "Mutation" && name != "Subscription" &&
			name != "String" && name != "Int" && name != "Float" && name != "Boolean" && name != "ID" {
			typeCount++
		}
	}

	b.WriteString("### Schema Statistics\n\n")
	b.WriteString("| Category | Count |\n")
	b.WriteString("|----------|-------|\n")
	b.WriteString(fmt.Sprintf("| Queries | %d |\n", queryCount))
	b.WriteString(fmt.Sprintf("| Mutations | %d |\n", mutationCount))
	if subscriptionCount > 0 {
		b.WriteString(fmt.Sprintf("| Subscriptions | %d |\n", subscriptionCount))
	}
	b.WriteString(fmt.Sprintf("| Custom Types | %d |\n", typeCount))
	b.WriteString("\n")

	// List available root operations
	b.WriteString("### Available Operations\n\n")
	if queryCount > 0 {
		b.WriteString("- **Queries**: Read data from the API\n")
	}
	if mutationCount > 0 {
		b.WriteString("- **Mutations**: Create, update, or delete data\n")
	}
	if subscriptionCount > 0 {
		b.WriteString("- **Subscriptions**: Subscribe to real-time updates\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *GraphQLConverter) buildOperationsSection(def *ast.Definition, opType string, schema *ast.Schema) string {
	var b strings.Builder

	// Sort fields by name
	fields := make([]*ast.FieldDefinition, len(def.Fields))
	copy(fields, def.Fields)
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Name < fields[j].Name
	})

	for _, field := range fields {
		b.WriteString(fmt.Sprintf("### %s\n\n", field.Name))

		// Deprecation warning
		if c.isDeprecated(field.Directives) {
			reason := c.getDeprecationReason(field.Directives)
			b.WriteString(fmt.Sprintf("> ⚠️ **Deprecated**: %s\n\n", reason))
		}

		if field.Description != "" {
			b.WriteString(field.Description)
			b.WriteString("\n\n")
		}

		// Arguments table
		if len(field.Arguments) > 0 {
			b.WriteString("**Arguments**:\n\n")
			b.WriteString("| Name | Type | Required | Default | Description |\n")
			b.WriteString("|------|------|----------|---------|-------------|\n")
			for _, arg := range field.Arguments {
				required := "No"
				if arg.Type.NonNull {
					required = "Yes"
				}
				defaultVal := "-"
				if arg.DefaultValue != nil {
					defaultVal = fmt.Sprintf("`%s`", arg.DefaultValue.String())
				}
				desc := strings.ReplaceAll(arg.Description, "\n", " ")
				b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s | %s | %s |\n",
					arg.Name, c.formatType(arg.Type), required, defaultVal, desc))
			}
			b.WriteString("\n")
		}

		// Return type
		b.WriteString(fmt.Sprintf("**Returns**: `%s`\n\n", c.formatType(field.Type)))

		// Generate example
		b.WriteString("**Example**:\n\n")
		b.WriteString("```graphql\n")
		b.WriteString(fmt.Sprintf("%s {\n", opType))
		b.WriteString(fmt.Sprintf("  %s", field.Name))
		if len(field.Arguments) > 0 {
			args := c.buildExampleArgs(field.Arguments)
			b.WriteString(fmt.Sprintf("(%s)", args))
		}
		b.WriteString(" {\n")
		b.WriteString(c.buildExampleFields(field.Type, schema, "    "))
		b.WriteString("  }\n")
		b.WriteString("}\n")
		b.WriteString("```\n\n")

		// If mutation, show variables example
		if opType == "mutation" && len(field.Arguments) > 0 {
			b.WriteString("**With Variables**:\n\n")
			b.WriteString("```graphql\n")
			b.WriteString(fmt.Sprintf("mutation %s(", c.capitalize(field.Name)))
			varDefs := []string{}
			for _, arg := range field.Arguments {
				varDefs = append(varDefs, fmt.Sprintf("$%s: %s", arg.Name, c.formatType(arg.Type)))
			}
			b.WriteString(strings.Join(varDefs, ", "))
			b.WriteString(") {\n")
			b.WriteString(fmt.Sprintf("  %s(", field.Name))
			argRefs := []string{}
			for _, arg := range field.Arguments {
				argRefs = append(argRefs, fmt.Sprintf("%s: $%s", arg.Name, arg.Name))
			}
			b.WriteString(strings.Join(argRefs, ", "))
			b.WriteString(") {\n")
			b.WriteString(c.buildExampleFields(field.Type, schema, "    "))
			b.WriteString("  }\n")
			b.WriteString("}\n")
			b.WriteString("```\n\n")

			// Variables JSON
			b.WriteString("**Variables**:\n")
			b.WriteString("```json\n")
			b.WriteString("{\n")
			varExamples := []string{}
			for _, arg := range field.Arguments {
				varExamples = append(varExamples, fmt.Sprintf("  \"%s\": %s", arg.Name, c.getExampleValue(arg.Type)))
			}
			b.WriteString(strings.Join(varExamples, ",\n"))
			b.WriteString("\n}\n")
			b.WriteString("```\n\n")
		}
	}

	return strings.TrimSpace(b.String())
}

func (c *GraphQLConverter) buildTypesSection(schema *ast.Schema) string {
	var b strings.Builder

	// Collect type names (excluding built-ins and Query/Mutation/Subscription)
	var typeNames []string
	for name := range schema.Types {
		if strings.HasPrefix(name, "__") {
			continue
		}
		if name == "Query" || name == "Mutation" || name == "Subscription" {
			continue
		}
		// Skip built-in scalars
		if name == "String" || name == "Int" || name == "Float" || name == "Boolean" || name == "ID" {
			continue
		}
		typeNames = append(typeNames, name)
	}
	sort.Strings(typeNames)

	// Group types by kind
	objects := []string{}
	inputs := []string{}
	interfaces := []string{}
	enums := []string{}
	unions := []string{}
	scalars := []string{}

	for _, name := range typeNames {
		def := schema.Types[name]
		switch def.Kind {
		case ast.Object:
			objects = append(objects, name)
		case ast.InputObject:
			inputs = append(inputs, name)
		case ast.Interface:
			interfaces = append(interfaces, name)
		case ast.Enum:
			enums = append(enums, name)
		case ast.Union:
			unions = append(unions, name)
		case ast.Scalar:
			scalars = append(scalars, name)
		}
	}

	// Type index
	b.WriteString("### Type Index\n\n")
	if len(objects) > 0 {
		b.WriteString(fmt.Sprintf("**Objects**: %s\n\n", strings.Join(c.linkTypes(objects), ", ")))
	}
	if len(inputs) > 0 {
		b.WriteString(fmt.Sprintf("**Inputs**: %s\n\n", strings.Join(c.linkTypes(inputs), ", ")))
	}
	if len(interfaces) > 0 {
		b.WriteString(fmt.Sprintf("**Interfaces**: %s\n\n", strings.Join(c.linkTypes(interfaces), ", ")))
	}
	if len(enums) > 0 {
		b.WriteString(fmt.Sprintf("**Enums**: %s\n\n", strings.Join(c.linkTypes(enums), ", ")))
	}
	if len(unions) > 0 {
		b.WriteString(fmt.Sprintf("**Unions**: %s\n\n", strings.Join(c.linkTypes(unions), ", ")))
	}
	if len(scalars) > 0 {
		b.WriteString(fmt.Sprintf("**Custom Scalars**: %s\n\n", strings.Join(c.linkTypes(scalars), ", ")))
	}

	// Detailed type definitions
	for _, name := range typeNames {
		def := schema.Types[name]
		b.WriteString(c.buildTypeSection(def, schema))
	}

	return strings.TrimSpace(b.String())
}

func (c *GraphQLConverter) buildTypeSection(def *ast.Definition, schema *ast.Schema) string {
	var b strings.Builder

	kind := string(def.Kind)
	b.WriteString(fmt.Sprintf("### %s (%s)\n\n", def.Name, kind))

	// Deprecation warning
	if c.isDeprecated(def.Directives) {
		reason := c.getDeprecationReason(def.Directives)
		b.WriteString(fmt.Sprintf("> ⚠️ **Deprecated**: %s\n\n", reason))
	}

	if def.Description != "" {
		b.WriteString(def.Description)
		b.WriteString("\n\n")
	}

	// Show implements for objects
	if def.Kind == ast.Object && len(def.Interfaces) > 0 {
		interfaces := make([]string, len(def.Interfaces))
		for i, iface := range def.Interfaces {
			interfaces[i] = fmt.Sprintf("`%s`", iface)
		}
		b.WriteString(fmt.Sprintf("**Implements**: %s\n\n", strings.Join(interfaces, ", ")))
	}

	switch def.Kind {
	case ast.Object, ast.InputObject, ast.Interface:
		if len(def.Fields) > 0 {
			b.WriteString("| Field | Type | Description |\n")
			b.WriteString("|-------|------|-------------|\n")
			for _, field := range def.Fields {
				desc := strings.ReplaceAll(field.Description, "\n", " ")
				if c.isDeprecated(field.Directives) {
					desc = "⚠️ " + desc
				}
				b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n",
					field.Name, c.formatType(field.Type), desc))
			}
			b.WriteString("\n")
		}

	case ast.Enum:
		if len(def.EnumValues) > 0 {
			b.WriteString("**Values**:\n\n")
			for _, val := range def.EnumValues {
				deprecated := ""
				if c.isDeprecated(val.Directives) {
					deprecated = " ⚠️ (deprecated)"
				}
				b.WriteString(fmt.Sprintf("- `%s`", val.Name))
				if val.Description != "" {
					b.WriteString(fmt.Sprintf(" - %s", val.Description))
				}
				b.WriteString(deprecated)
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}

	case ast.Union:
		if len(def.Types) > 0 {
			b.WriteString("**Possible Types**: ")
			types := make([]string, len(def.Types))
			for i, t := range def.Types {
				types[i] = fmt.Sprintf("`%s`", t)
			}
			b.WriteString(strings.Join(types, ", "))
			b.WriteString("\n\n")

			// Show usage example
			b.WriteString("**Usage Example**:\n")
			b.WriteString("```graphql\n")
			b.WriteString("{\n")
			b.WriteString(fmt.Sprintf("  # When querying a %s field\n", def.Name))
			b.WriteString("  ... on " + def.Types[0] + " {\n")
			b.WriteString("    # fields specific to " + def.Types[0] + "\n")
			b.WriteString("  }\n")
			if len(def.Types) > 1 {
				b.WriteString("  ... on " + def.Types[1] + " {\n")
				b.WriteString("    # fields specific to " + def.Types[1] + "\n")
				b.WriteString("  }\n")
			}
			b.WriteString("}\n")
			b.WriteString("```\n\n")
		}

	case ast.Scalar:
		b.WriteString("Custom scalar type.\n\n")
		// Add common scalar documentation
		scalarDocs := map[string]string{
			"DateTime": "ISO 8601 formatted date-time string (e.g., `2024-01-15T10:30:00Z`)",
			"Date":     "ISO 8601 formatted date string (e.g., `2024-01-15`)",
			"Time":     "ISO 8601 formatted time string (e.g., `10:30:00`)",
			"JSON":     "Arbitrary JSON value",
			"UUID":     "UUID string (e.g., `123e4567-e89b-12d3-a456-426614174000`)",
			"Email":    "Valid email address string",
			"URL":      "Valid URL string",
		}
		if doc, ok := scalarDocs[def.Name]; ok {
			b.WriteString(fmt.Sprintf("**Format**: %s\n\n", doc))
		}
	}

	return b.String()
}

func (c *GraphQLConverter) buildDirectivesSection(schema *ast.Schema) string {
	var b strings.Builder

	// Collect custom directives (exclude built-in)
	builtIn := map[string]bool{
		"deprecated": true,
		"skip":       true,
		"include":    true,
		"specifiedBy": true,
	}

	var customDirectives []*ast.DirectiveDefinition
	for _, dir := range schema.Directives {
		if !builtIn[dir.Name] {
			customDirectives = append(customDirectives, dir)
		}
	}

	if len(customDirectives) == 0 {
		return ""
	}

	b.WriteString("Custom directives available in this schema.\n\n")

	for _, dir := range customDirectives {
		b.WriteString(fmt.Sprintf("### @%s\n\n", dir.Name))

		if dir.Description != "" {
			b.WriteString(dir.Description)
			b.WriteString("\n\n")
		}

		// Arguments
		if len(dir.Arguments) > 0 {
			b.WriteString("**Arguments**:\n\n")
			b.WriteString("| Name | Type | Description |\n")
			b.WriteString("|------|------|-------------|\n")
			for _, arg := range dir.Arguments {
				desc := strings.ReplaceAll(arg.Description, "\n", " ")
				b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n",
					arg.Name, c.formatType(arg.Type), desc))
			}
			b.WriteString("\n")
		}

		// Locations
		locations := make([]string, len(dir.Locations))
		for i, loc := range dir.Locations {
			locations[i] = string(loc)
		}
		b.WriteString(fmt.Sprintf("**Valid Locations**: %s\n\n", strings.Join(locations, ", ")))
	}

	return strings.TrimSpace(b.String())
}

func (c *GraphQLConverter) buildBestPracticesSection() string {
	var b strings.Builder

	b.WriteString("Follow these recommendations for optimal GraphQL usage.\n\n")

	b.WriteString("### Query Optimization\n\n")
	b.WriteString("- Request only the fields you need\n")
	b.WriteString("- Use fragments for reusable field selections\n")
	b.WriteString("- Batch multiple queries in a single request when possible\n")
	b.WriteString("- Use pagination for large collections\n\n")

	b.WriteString("### Error Handling\n\n")
	b.WriteString("```javascript\n")
	b.WriteString("const response = await fetch('/graphql', {\n")
	b.WriteString("  method: 'POST',\n")
	b.WriteString("  headers: { 'Content-Type': 'application/json' },\n")
	b.WriteString("  body: JSON.stringify({ query, variables })\n")
	b.WriteString("});\n\n")
	b.WriteString("const { data, errors } = await response.json();\n\n")
	b.WriteString("if (errors) {\n")
	b.WriteString("  errors.forEach(error => {\n")
	b.WriteString("    console.error('GraphQL error:', error.message);\n")
	b.WriteString("    if (error.extensions?.code === 'UNAUTHENTICATED') {\n")
	b.WriteString("      // Handle auth error\n")
	b.WriteString("    }\n")
	b.WriteString("  });\n")
	b.WriteString("}\n")
	b.WriteString("```\n\n")

	b.WriteString("### Fragments\n\n")
	b.WriteString("Use fragments to avoid repetition:\n\n")
	b.WriteString("```graphql\n")
	b.WriteString("fragment UserFields on User {\n")
	b.WriteString("  id\n")
	b.WriteString("  name\n")
	b.WriteString("  email\n")
	b.WriteString("}\n\n")
	b.WriteString("query {\n")
	b.WriteString("  currentUser { ...UserFields }\n")
	b.WriteString("  otherUser(id: \"123\") { ...UserFields }\n")
	b.WriteString("}\n")
	b.WriteString("```\n\n")

	b.WriteString("### Caching\n\n")
	b.WriteString("- Use query names for cache identification\n")
	b.WriteString("- Leverage `__typename` for normalized caching\n")
	b.WriteString("- Consider persisted queries for production\n")

	return strings.TrimSpace(b.String())
}

// Helper functions

func (c *GraphQLConverter) formatType(t *ast.Type) string {
	if t == nil {
		return "unknown"
	}

	var result string
	if t.Elem != nil {
		result = fmt.Sprintf("[%s]", c.formatType(t.Elem))
	} else {
		result = t.NamedType
	}

	if t.NonNull {
		result += "!"
	}

	return result
}

func (c *GraphQLConverter) isDeprecated(directives ast.DirectiveList) bool {
	for _, d := range directives {
		if d.Name == "deprecated" {
			return true
		}
	}
	return false
}

func (c *GraphQLConverter) getDeprecationReason(directives ast.DirectiveList) string {
	for _, d := range directives {
		if d.Name == "deprecated" {
			for _, arg := range d.Arguments {
				if arg.Name == "reason" {
					return arg.Value.Raw
				}
			}
			return "This field is deprecated"
		}
	}
	return "This field is deprecated"
}

func (c *GraphQLConverter) hasRequiredArgs(args ast.ArgumentDefinitionList) bool {
	for _, arg := range args {
		if arg.Type.NonNull && arg.DefaultValue == nil {
			return true
		}
	}
	return false
}

func (c *GraphQLConverter) buildExampleArgs(args ast.ArgumentDefinitionList) string {
	var parts []string
	for _, arg := range args {
		value := c.getExampleValue(arg.Type)
		parts = append(parts, fmt.Sprintf("%s: %s", arg.Name, value))
	}
	return strings.Join(parts, ", ")
}

func (c *GraphQLConverter) getExampleValue(t *ast.Type) string {
	typeName := t.NamedType
	if t.Elem != nil {
		typeName = t.Elem.NamedType
	}

	var value string
	switch typeName {
	case "String":
		value = "\"example\""
	case "Int":
		value = "42"
	case "Float":
		value = "3.14"
	case "Boolean":
		value = "true"
	case "ID":
		value = "\"123\""
	default:
		// Assume it's an enum or input type
		value = "null"
	}

	if t.Elem != nil {
		value = "[" + value + "]"
	}

	return value
}

func (c *GraphQLConverter) buildExampleFields(t *ast.Type, schema *ast.Schema, indent string) string {
	var b strings.Builder

	typeName := t.NamedType
	if t.Elem != nil {
		typeName = t.Elem.NamedType
	}

	// Get the type definition
	def, ok := schema.Types[typeName]
	if !ok || def.Kind != ast.Object {
		// It's a scalar, just show id if available
		b.WriteString(indent + "id\n")
		return b.String()
	}

	// Show first few fields
	count := 0
	for _, field := range def.Fields {
		if count >= 3 {
			b.WriteString(indent + "# ... more fields\n")
			break
		}
		// Skip complex nested types for brevity
		if field.Type.Elem == nil {
			fieldType := field.Type.NamedType
			if fieldType == "String" || fieldType == "Int" || fieldType == "Float" ||
				fieldType == "Boolean" || fieldType == "ID" {
				b.WriteString(indent + field.Name + "\n")
				count++
			} else if count == 0 {
				// Include at least one complex field
				b.WriteString(indent + field.Name + " { id }\n")
				count++
			}
		} else if count == 0 {
			b.WriteString(indent + field.Name + " { id }\n")
			count++
		}
	}

	if count == 0 {
		b.WriteString(indent + "id\n")
	}

	return b.String()
}

func (c *GraphQLConverter) capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func (c *GraphQLConverter) linkTypes(types []string) []string {
	result := make([]string, len(types))
	for i, t := range types {
		result[i] = fmt.Sprintf("`%s`", t)
	}
	return result
}
