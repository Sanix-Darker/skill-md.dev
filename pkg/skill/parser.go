package skill

import (
	"regexp"
	"strings"

	"github.com/adrg/frontmatter"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

var headerRegex = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// CodeBlock represents a code block in the markdown.
type CodeBlock struct {
	Language string
	Content  string
}

// ParsedSection represents a section with richer parsed content.
type ParsedSection struct {
	Title      string
	Level      int
	Content    string
	CodeBlocks []CodeBlock
	Children   []*ParsedSection
}

// Parse parses a SKILL.md file content into a Skill struct.
func Parse(content string) (*Skill, error) {
	skill := &Skill{Raw: content}

	// Parse frontmatter
	rest, err := frontmatter.Parse(strings.NewReader(content), &skill.Frontmatter)
	if err != nil {
		// No frontmatter, treat entire content as body
		skill.Content = content
	} else {
		skill.Content = string(rest)
	}

	// Parse sections from content
	skill.Sections = parseSections(skill.Content)

	return skill, nil
}

// parseSections extracts sections from markdown content.
func parseSections(content string) []Section {
	lines := strings.Split(content, "\n")
	var sections []Section
	var currentSection *Section
	var contentBuilder strings.Builder

	for _, line := range lines {
		if matches := headerRegex.FindStringSubmatch(line); matches != nil {
			// Save previous section
			if currentSection != nil {
				currentSection.Content = strings.TrimSpace(contentBuilder.String())
				sections = append(sections, *currentSection)
				contentBuilder.Reset()
			}

			// Start new section
			level := len(matches[1])
			title := matches[2]
			currentSection = &Section{
				Title: title,
				Level: level,
			}
		} else if currentSection != nil {
			contentBuilder.WriteString(line)
			contentBuilder.WriteString("\n")
		}
	}

	// Save last section
	if currentSection != nil {
		currentSection.Content = strings.TrimSpace(contentBuilder.String())
		sections = append(sections, *currentSection)
	}

	return sections
}

// GetSectionByTitle returns the first section matching the title.
func (s *Skill) GetSectionByTitle(title string) *Section {
	title = strings.ToLower(title)
	for i := range s.Sections {
		if strings.ToLower(s.Sections[i].Title) == title {
			return &s.Sections[i]
		}
	}
	return nil
}

// ASTParser provides Goldmark-based markdown parsing.
type ASTParser struct {
	md goldmark.Markdown
}

// NewASTParser creates a new AST-based parser.
func NewASTParser() *ASTParser {
	return &ASTParser{
		md: goldmark.New(),
	}
}

// ParseAST parses markdown content into an AST and extracts structured data.
func (p *ASTParser) ParseAST(content []byte) (*ParsedSkill, error) {
	result := &ParsedSkill{
		Sections:   make([]*ParsedSection, 0),
		CodeBlocks: make([]CodeBlock, 0),
	}

	// Extract frontmatter first
	var fm Frontmatter
	contentStr := string(content)
	rest, err := frontmatter.Parse(strings.NewReader(contentStr), &fm)
	if err == nil {
		result.Frontmatter = fm
		content = rest
	}

	// Parse markdown AST
	reader := text.NewReader(content)
	doc := p.md.Parser().Parse(reader)

	// Walk the AST
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch node := n.(type) {
		case *ast.Heading:
			title := extractTextContent(node, content)
			section := &ParsedSection{
				Title:      title,
				Level:      node.Level,
				CodeBlocks: make([]CodeBlock, 0),
				Children:   make([]*ParsedSection, 0),
			}
			result.Sections = append(result.Sections, section)

		case *ast.FencedCodeBlock:
			lang := string(node.Language(content))
			code := extractCodeBlockContent(node, content)
			block := CodeBlock{
				Language: lang,
				Content:  code,
			}
			result.CodeBlocks = append(result.CodeBlocks, block)

			// Add to current section if any
			if len(result.Sections) > 0 {
				current := result.Sections[len(result.Sections)-1]
				current.CodeBlocks = append(current.CodeBlocks, block)
			}
		}

		return ast.WalkContinue, nil
	})

	return result, nil
}

// ParsedSkill represents a fully parsed SKILL.md with AST information.
type ParsedSkill struct {
	Frontmatter Frontmatter
	Sections    []*ParsedSection
	CodeBlocks  []CodeBlock
}

// GetSectionsByLevel returns all sections at a specific level.
func (p *ParsedSkill) GetSectionsByLevel(level int) []*ParsedSection {
	result := make([]*ParsedSection, 0)
	for _, s := range p.Sections {
		if s.Level == level {
			result = append(result, s)
		}
	}
	return result
}

// GetCodeBlocksByLanguage returns code blocks for a specific language.
func (p *ParsedSkill) GetCodeBlocksByLanguage(lang string) []CodeBlock {
	result := make([]CodeBlock, 0)
	lang = strings.ToLower(lang)
	for _, cb := range p.CodeBlocks {
		if strings.ToLower(cb.Language) == lang {
			result = append(result, cb)
		}
	}
	return result
}

// extractTextContent extracts text from a heading node.
func extractTextContent(n ast.Node, source []byte) string {
	var buf strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if text, ok := c.(*ast.Text); ok {
			buf.Write(text.Segment.Value(source))
		}
	}
	return buf.String()
}

// extractCodeBlockContent extracts content from a fenced code block.
func extractCodeBlockContent(n *ast.FencedCodeBlock, source []byte) string {
	var buf strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		buf.Write(line.Value(source))
	}
	return strings.TrimSuffix(buf.String(), "\n")
}
