# Skill Forge

Convert technical specifications into SKILL.md format for AI agents.

![Go Version](https://img.shields.io/badge/go-1.23+-blue)
![License](https://img.shields.io/badge/license-MIT-green)

## Features

- **Convert** - Transform OpenAPI, GraphQL, Postman, and plain text specs into SKILL.md format
- **Merge** - Combine multiple SKILL.md files with intelligent deduplication
- **Browse** - Search and explore the skill registry
- **Web UI** - Dark terminal-themed interface with HTMX
- **CLI** - Full-featured command line interface

## Installation

### Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/sanixdarker/skillforge/main/scripts/install.sh | bash
```

### From Source

```bash
go install github.com/sanixdarker/skillforge/cmd/skillforge@latest
```

### Docker

```bash
docker run -p 8080:8080 sanixdarker/skillforge
```

## Usage

### Web Server

Start the web server:

```bash
skillforge serve
# Server running at http://localhost:8080
```

Options:
- `--port, -p` - Port to listen on (default: 8080)
- `--db` - Path to SQLite database (default: ./skillforge.db)
- `--debug` - Enable debug mode

### Convert

Convert a specification file to SKILL.md:

```bash
# Auto-detect format
skillforge convert api.yaml

# Specify format
skillforge convert schema.graphql -f graphql

# Save to file
skillforge convert api.yaml -o skill.md

# Custom name
skillforge convert api.yaml -n "My API Skill"
```

Supported formats:
- `openapi` - OpenAPI 3.x (YAML/JSON)
- `graphql` - GraphQL schema
- `postman` - Postman collection
- `text` - Plain text

### Merge

Merge multiple SKILL.md files:

```bash
# Basic merge
skillforge merge skill1.md skill2.md

# Save to file
skillforge merge skill1.md skill2.md -o combined.md

# With deduplication
skillforge merge skill1.md skill2.md --dedupe

# Custom name
skillforge merge skill1.md skill2.md -n "Combined Skills"
```

### Validate

Validate a SKILL.md file:

```bash
skillforge validate skill.md
```

## SKILL.md Format

SKILL.md is a structured markdown format for AI agent skills:

```markdown
---
name: "API Skill"
version: "1.0.0"
description: "API operations and endpoints"
tags:
  - "api"
  - "rest"
source_type: "openapi"
---

## Overview

Description of the skill and its capabilities.

## Endpoints

### GET /users

Retrieve all users.

**Parameters**:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| limit | integer | No | Max results |

**Responses**:
| Code | Description |
|------|-------------|
| 200 | Success |
```

## Development

### Requirements

- Go 1.23+
- Make (optional)

### Build

```bash
# Build binary
make build

# Build for all platforms
make build-all

# Run in development mode
make dev
```

### Test

```bash
make test
make test-coverage
```

### Docker

```bash
# Build image
make docker

# Run with docker compose
make docker-compose
```

## Project Structure

```
skillforge/
├── cmd/skillforge/        # CLI entry point
├── internal/
│   ├── app/               # Application container
│   ├── cli/               # CLI commands
│   ├── converter/         # Spec converters
│   ├── merger/            # Skill merging
│   ├── registry/          # Skill registry
│   ├── server/            # HTTP server
│   └── storage/           # Database
├── pkg/skill/             # Public skill types
├── web/                   # Web assets
└── scripts/               # Install scripts
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Author

sanix darker
