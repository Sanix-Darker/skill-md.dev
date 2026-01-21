package converter

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/ledongthuc/pdf"
	"github.com/sanixdarker/skill-md/pkg/skill"
)

// PDFConverter converts PDF documents to SKILL.md.
type PDFConverter struct{}

func (c *PDFConverter) Name() string {
	return "pdf"
}

func (c *PDFConverter) CanHandle(filename string, content []byte) bool {
	ext := getExtension(filename)
	if ext == ".pdf" {
		return true
	}
	// Check for PDF magic bytes
	return len(content) > 4 && string(content[:4]) == "%PDF"
}

func (c *PDFConverter) Convert(content []byte, opts *Options) (*skill.Skill, error) {
	// Parse PDF
	reader := bytes.NewReader(content)
	pdfReader, err := pdf.NewReader(reader, int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse PDF: %w", err)
	}

	// Extract text from all pages
	var textBuilder strings.Builder
	numPages := pdfReader.NumPage()

	for i := 1; i <= numPages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			continue // Skip pages that fail to extract
		}
		textBuilder.WriteString(text)
		textBuilder.WriteString("\n\n")
	}

	fullText := textBuilder.String()
	if strings.TrimSpace(fullText) == "" {
		return nil, fmt.Errorf("no text content found in PDF")
	}

	// Build skill from extracted text
	s := c.buildSkill(fullText, numPages, opts)
	return s, nil
}

func (c *PDFConverter) buildSkill(text string, numPages int, opts *Options) *skill.Skill {
	// Try to extract title from first line or heading
	name := "PDF Document"
	if opts != nil && opts.Name != "" {
		name = opts.Name
	} else {
		// Try to find title from first significant line
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if len(line) > 5 && len(line) < 100 && !strings.HasPrefix(line, "http") {
				name = line
				break
			}
		}
	}

	// Extract first paragraph as description
	description := c.extractDescription(text)

	s := skill.NewSkill(name, description)
	s.Frontmatter.SourceType = "pdf"
	s.Frontmatter.Tags = []string{"pdf", "document"}
	if opts != nil && opts.SourcePath != "" {
		s.Frontmatter.Source = opts.SourcePath
	}

	// Analyze content
	headers := c.detectHeaders(text)
	codeBlocks := c.detectCodeBlocks(text)
	endpoints := c.detectEndpoints(text)
	tables := c.detectTables(text)

	// Set metadata
	s.Frontmatter.EndpointCount = len(endpoints)
	s.Frontmatter.HasExamples = len(codeBlocks) > 0

	// Determine difficulty
	if len(endpoints) <= 5 && numPages <= 10 {
		s.Frontmatter.Difficulty = "novice"
	} else if len(endpoints) <= 20 && numPages <= 50 {
		s.Frontmatter.Difficulty = "intermediate"
	} else {
		s.Frontmatter.Difficulty = "advanced"
	}

	// Determine content type and build appropriate sections
	contentType := c.detectContentType(text, headers, endpoints, codeBlocks)

	switch contentType {
	case "api":
		c.buildAPISections(s, text, headers, endpoints, codeBlocks, tables, numPages)
	case "tutorial":
		c.buildTutorialSections(s, text, headers, codeBlocks, numPages)
	default:
		c.buildDocumentSections(s, text, headers, codeBlocks, tables, numPages)
	}

	return s
}

func (c *PDFConverter) extractDescription(text string) string {
	// Find first paragraph (skip title-like first line)
	paragraphs := strings.Split(text, "\n\n")
	for i, para := range paragraphs {
		para = strings.TrimSpace(para)
		if i == 0 && len(para) < 100 {
			continue // Skip potential title
		}
		if len(para) > 50 && len(para) < 500 {
			// Clean up
			para = strings.ReplaceAll(para, "\n", " ")
			para = regexp.MustCompile(`\s+`).ReplaceAllString(para, " ")
			return para
		}
	}
	return "Document extracted from PDF"
}

func (c *PDFConverter) detectHeaders(text string) []struct {
	Level int
	Text  string
} {
	var headers []struct {
		Level int
		Text  string
	}

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Detect markdown-style headers
		if strings.HasPrefix(line, "# ") {
			headers = append(headers, struct {
				Level int
				Text  string
			}{1, strings.TrimPrefix(line, "# ")})
		} else if strings.HasPrefix(line, "## ") {
			headers = append(headers, struct {
				Level int
				Text  string
			}{2, strings.TrimPrefix(line, "## ")})
		} else if strings.HasPrefix(line, "### ") {
			headers = append(headers, struct {
				Level int
				Text  string
			}{3, strings.TrimPrefix(line, "### ")})
		}

		// Detect numbered headers like "1. Introduction" or "1.1 Overview"
		numberedPattern := regexp.MustCompile(`^(\d+\.)+\s+(.+)$`)
		if matches := numberedPattern.FindStringSubmatch(line); len(matches) > 2 {
			level := strings.Count(matches[1], ".")
			if level > 3 {
				level = 3
			}
			headers = append(headers, struct {
				Level int
				Text  string
			}{level, matches[2]})
		}

		// Detect ALL CAPS headers (common in PDFs)
		if len(line) > 3 && len(line) < 80 && line == strings.ToUpper(line) && regexp.MustCompile(`[A-Z]`).MatchString(line) {
			headers = append(headers, struct {
				Level int
				Text  string
			}{2, strings.Title(strings.ToLower(line))})
		}
	}

	return headers
}

func (c *PDFConverter) detectCodeBlocks(text string) []struct {
	Language string
	Code     string
} {
	var blocks []struct {
		Language string
		Code     string
	}

	// Look for code patterns
	lines := strings.Split(text, "\n")
	inCodeBlock := false
	var currentCode strings.Builder
	var currentLang string

	for _, line := range lines {
		// Detect markdown code fences
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End of block
				blocks = append(blocks, struct {
					Language string
					Code     string
				}{currentLang, strings.TrimSpace(currentCode.String())})
				currentCode.Reset()
				currentLang = ""
				inCodeBlock = false
			} else {
				// Start of block
				inCodeBlock = true
				currentLang = strings.TrimPrefix(line, "```")
			}
			continue
		}

		if inCodeBlock {
			currentCode.WriteString(line)
			currentCode.WriteString("\n")
			continue
		}

		// Detect inline code patterns
		if c.looksLikeCode(line) {
			lang := c.detectLanguage(line)
			blocks = append(blocks, struct {
				Language string
				Code     string
			}{lang, strings.TrimSpace(line)})
		}
	}

	return blocks
}

func (c *PDFConverter) looksLikeCode(line string) bool {
	codePatterns := []string{
		`curl\s+`,
		`^GET\s+/`,
		`^POST\s+/`,
		`^PUT\s+/`,
		`^DELETE\s+/`,
		`^import\s+`,
		`^from\s+\w+\s+import`,
		`^const\s+\w+\s*=`,
		`^let\s+\w+\s*=`,
		`^var\s+\w+\s*=`,
		`^func\s+\w+\(`,
		`^def\s+\w+\(`,
		`\{\s*"`,
		`^\s*}\s*$`,
		`^\s*\[\s*$`,
		`^\s*\]\s*$`,
	}

	for _, pattern := range codePatterns {
		if regexp.MustCompile(pattern).MatchString(line) {
			return true
		}
	}
	return false
}

func (c *PDFConverter) detectLanguage(code string) string {
	code = strings.ToLower(code)

	patterns := map[string][]string{
		"bash":       {"curl ", "wget ", "#!/bin/bash"},
		"javascript": {"const ", "let ", "function ", "=>", "async "},
		"python":     {"def ", "import ", "from ", "print("},
		"go":         {"func ", "package ", "import ("},
		"json":       {`{"`, `":`},
		"yaml":       {"---", "  -"},
	}

	for lang, keywords := range patterns {
		for _, kw := range keywords {
			if strings.Contains(code, kw) {
				return lang
			}
		}
	}
	return ""
}

func (c *PDFConverter) detectEndpoints(text string) []struct {
	Method string
	Path   string
} {
	var endpoints []struct {
		Method string
		Path   string
	}

	pattern := regexp.MustCompile(`(?i)(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s+(/[^\s"'<>]+)`)
	matches := pattern.FindAllStringSubmatch(text, -1)

	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) >= 3 {
			method := strings.ToUpper(match[1])
			path := strings.TrimRight(match[2], ".,;:)")

			key := method + " " + path
			if !seen[key] {
				seen[key] = true
				endpoints = append(endpoints, struct {
					Method string
					Path   string
				}{method, path})
			}
		}
	}

	return endpoints
}

func (c *PDFConverter) detectTables(text string) [][]string {
	var tables [][]string

	// Look for table-like patterns (rows with | separators or consistent spacing)
	lines := strings.Split(text, "\n")
	var currentTable []string

	for _, line := range lines {
		if strings.Contains(line, "|") && strings.Count(line, "|") >= 2 {
			currentTable = append(currentTable, line)
		} else if len(currentTable) > 0 {
			if len(currentTable) >= 2 {
				tables = append(tables, currentTable)
			}
			currentTable = nil
		}
	}

	if len(currentTable) >= 2 {
		tables = append(tables, currentTable)
	}

	return tables
}

func (c *PDFConverter) detectContentType(text string, headers []struct {
	Level int
	Text  string
}, endpoints []struct {
	Method string
	Path   string
}, codeBlocks []struct {
	Language string
	Code     string
}) string {
	textLower := strings.ToLower(text)

	// API indicators
	apiScore := 0
	apiKeywords := []string{"api", "endpoint", "request", "response", "authentication", "bearer", "http"}
	for _, kw := range apiKeywords {
		if strings.Contains(textLower, kw) {
			apiScore++
		}
	}
	if len(endpoints) > 3 {
		apiScore += 3
	}

	// Tutorial indicators
	tutorialScore := 0
	tutorialKeywords := []string{"tutorial", "how to", "step", "guide", "learn", "example", "getting started"}
	for _, kw := range tutorialKeywords {
		if strings.Contains(textLower, kw) {
			tutorialScore++
		}
	}

	if apiScore >= 4 {
		return "api"
	}
	if tutorialScore >= 3 {
		return "tutorial"
	}
	return "document"
}

func (c *PDFConverter) buildAPISections(s *skill.Skill, text string, headers []struct {
	Level int
	Text  string
}, endpoints []struct {
	Method string
	Path   string
}, codeBlocks []struct {
	Language string
	Code     string
}, tables [][]string, numPages int) {

	// Quick Start
	var quickStart strings.Builder
	quickStart.WriteString("Get started with this API documentation.\n\n")
	quickStart.WriteString(fmt.Sprintf("This document has %d pages with %d detected endpoints.\n\n", numPages, len(endpoints)))
	if len(endpoints) > 0 {
		quickStart.WriteString("### Sample Endpoints\n\n")
		count := 3
		if len(endpoints) < count {
			count = len(endpoints)
		}
		for i := 0; i < count; i++ {
			quickStart.WriteString(fmt.Sprintf("- `%s %s`\n", endpoints[i].Method, endpoints[i].Path))
		}
	}
	s.AddSection("Quick Start", 2, quickStart.String())

	// Overview
	s.AddSection("Overview", 2, c.extractDescription(text))

	// Endpoints
	if len(endpoints) > 0 {
		var endpointSection strings.Builder
		endpointSection.WriteString("Detected API endpoints from the document.\n\n")

		byMethod := make(map[string][]string)
		for _, ep := range endpoints {
			byMethod[ep.Method] = append(byMethod[ep.Method], ep.Path)
		}

		for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
			if paths, ok := byMethod[method]; ok {
				endpointSection.WriteString(fmt.Sprintf("### %s\n\n", method))
				for _, path := range paths {
					endpointSection.WriteString(fmt.Sprintf("- `%s`\n", path))
				}
				endpointSection.WriteString("\n")
			}
		}

		s.AddSection("Endpoints", 2, endpointSection.String())
	}

	// Code Examples
	if len(codeBlocks) > 0 {
		var codeSection strings.Builder
		codeSection.WriteString("Code examples extracted from the document.\n\n")

		for i, block := range codeBlocks {
			if i >= 5 {
				codeSection.WriteString(fmt.Sprintf("\n*...and %d more examples*\n", len(codeBlocks)-5))
				break
			}
			lang := block.Language
			if lang == "" {
				lang = "text"
			}
			codeSection.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n", lang, block.Code))
		}

		s.AddSection("Code Examples", 2, codeSection.String())
	}

	// Best Practices
	s.AddSection("Best Practices", 2, "Review the original PDF document for complete details and context.")
}

func (c *PDFConverter) buildTutorialSections(s *skill.Skill, text string, headers []struct {
	Level int
	Text  string
}, codeBlocks []struct {
	Language string
	Code     string
}, numPages int) {

	// Overview
	s.AddSection("Overview", 2, c.extractDescription(text))

	// Steps
	if len(headers) > 0 {
		var steps strings.Builder
		steps.WriteString("Tutorial structure based on document sections.\n\n")
		step := 1
		for _, h := range headers {
			if h.Level <= 2 {
				steps.WriteString(fmt.Sprintf("### Step %d: %s\n\n", step, h.Text))
				step++
			}
		}
		s.AddSection("Steps", 2, steps.String())
	}

	// Code Examples
	if len(codeBlocks) > 0 {
		var codeSection strings.Builder
		codeSection.WriteString("Code examples from the tutorial.\n\n")

		for i, block := range codeBlocks {
			if i >= 10 {
				break
			}
			lang := block.Language
			if lang == "" {
				lang = "text"
			}
			codeSection.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n", lang, block.Code))
		}

		s.AddSection("Code Examples", 2, codeSection.String())
	}
}

func (c *PDFConverter) buildDocumentSections(s *skill.Skill, text string, headers []struct {
	Level int
	Text  string
}, codeBlocks []struct {
	Language string
	Code     string
}, tables [][]string, numPages int) {

	// Overview
	var overview strings.Builder
	overview.WriteString(c.extractDescription(text))
	overview.WriteString("\n\n")
	overview.WriteString(fmt.Sprintf("**Document Statistics**: %d pages\n", numPages))
	if len(headers) > 0 {
		overview.WriteString(fmt.Sprintf("**Sections**: %d\n", len(headers)))
	}
	s.AddSection("Overview", 2, overview.String())

	// Table of Contents
	if len(headers) > 0 {
		var toc strings.Builder
		toc.WriteString("Document structure.\n\n")
		for _, h := range headers {
			indent := strings.Repeat("  ", h.Level-1)
			toc.WriteString(fmt.Sprintf("%s- %s\n", indent, h.Text))
		}
		s.AddSection("Contents", 2, toc.String())
	}

	// Code Examples
	if len(codeBlocks) > 0 {
		var codeSection strings.Builder
		codeSection.WriteString("Code snippets from the document.\n\n")

		for i, block := range codeBlocks {
			if i >= 5 {
				break
			}
			lang := block.Language
			if lang == "" {
				lang = "text"
			}
			codeSection.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n", lang, block.Code))
		}

		s.AddSection("Code Examples", 2, codeSection.String())
	}
}
