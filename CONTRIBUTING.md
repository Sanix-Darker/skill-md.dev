# Contributing to Skill MD

Thank you for your interest in contributing to Skill MD!

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/skill-md.git`
3. Create a feature branch: `git checkout -b feat/your-feature`
4. Make your changes
5. Run tests: `go test ./...`
6. Build: `go build ./cmd/skillforge`
7. Commit with conventional format
8. Push and open a Pull Request

## Development Setup

### Requirements

- Go 1.23+
- Make (optional)

### Build

```bash
go build ./cmd/skillforge
```

### Run

```bash
./skillforge serve
```

### Test

```bash
go test ./...
```

## Adding a New Converter

Skill MD supports adding new input formats. Here's how to add a new converter:

### 1. Create the Converter File

Create `internal/converter/yourformat.go`:

```go
package converter

import (
    "github.com/sanixdarker/skill-md/pkg/skill"
)

type YourFormatConverter struct{}

func (c *YourFormatConverter) Name() string {
    return "yourformat"
}

func (c *YourFormatConverter) CanHandle(filename string, content []byte) bool {
    // Return true if this converter can handle the content
    ext := getExtension(filename)
    return ext == ".yourext"
}

func (c *YourFormatConverter) Convert(content []byte, opts *Options) (*skill.Skill, error) {
    // 1. Parse the input format
    // 2. Build the skill
    s := skill.NewSkill("Name", "Description")
    s.Frontmatter.SourceType = "yourformat"

    // 3. Add sections
    s.AddSection("Quick Start", 2, "Getting started content...")
    s.AddSection("Overview", 2, "Overview content...")

    // 4. Add MCP-compatible tool definitions (recommended)
    s.Frontmatter.MCPCompatible = true
    s.Frontmatter.ToolDefinitions = []skill.ToolDefinition{
        {
            Name:        "operation_name",
            Description: "What this operation does",
            Parameters: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "param1": map[string]interface{}{
                        "type":        "string",
                        "description": "Parameter description",
                    },
                },
            },
            Required: []string{"param1"},
        },
    }

    return s, nil
}
```

### 2. Register the Converter

Add to `internal/converter/converter.go` in `NewManager()`:

```go
m.Register(&YourFormatConverter{})
```

### 3. Add Test Files

Add sample files to `testdata/`:
- `testdata/sample.yourext`

### 4. Test

```bash
go build ./cmd/skillforge
./skillforge convert testdata/sample.yourext -f yourformat
```

## Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Keep functions focused and small
- Add comments for non-obvious logic

## Commit Guidelines

Use conventional commit format:

- `feat:` - New features
- `fix:` - Bug fixes
- `docs:` - Documentation changes
- `refactor:` - Code refactoring
- `test:` - Test additions/changes
- `chore:` - Maintenance tasks

Examples:
```
feat: add AsyncAPI converter
fix: handle empty response bodies in OpenAPI
docs: add converter development guide
```

## Pull Request Process

1. Ensure all tests pass
2. Update documentation if needed
3. Add test files for new converters
4. Use descriptive PR title and description
5. Reference any related issues

## Project Structure

```
skillforge/
├── cmd/skillforge/        # CLI entry point
├── internal/
│   ├── app/               # Application container
│   ├── cli/               # CLI commands
│   ├── converter/         # Spec converters (add new ones here)
│   │   └── shared/        # Shared utilities for converters
│   ├── merger/            # Skill merging
│   ├── registry/          # Skill registry service
│   ├── server/            # HTTP server and handlers
│   │   ├── handlers/      # Request handlers
│   │   └── middleware/    # HTTP middleware
│   ├── storage/           # SQLite database
│   ├── ssh/               # SSH server
│   └── tui/               # Terminal UI
├── pkg/skill/             # Public skill types and parser
├── web/                   # Web templates and assets
│   ├── static/            # CSS, JS, images
│   └── templates/         # HTML templates
└── testdata/              # Test fixtures
```

## Questions?

Open an issue on GitHub if you have questions or need help.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
