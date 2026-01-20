// Package extractor provides content extraction utilities.
package extractor

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	readability "github.com/go-shiori/go-readability"
)

// ExtractedContent represents content extracted from a URL.
type ExtractedContent struct {
	Title       string
	Author      string
	Description string
	Content     string
	TextContent string
	URL         string
	SiteName    string
	Image       string
	Favicon     string
	PublishedAt time.Time

	// Structured extraction
	CodeBlocks  []CodeBlock
	Endpoints   []EndpointInfo
	Headers     []Header
	Tables      []Table
	Lists       []List
	ContentType ContentType
}

// ContentType indicates the type of content detected.
type ContentType string

const (
	ContentTypeAPI      ContentType = "api"
	ContentTypeTutorial ContentType = "tutorial"
	ContentTypeArticle  ContentType = "article"
	ContentTypeUnknown  ContentType = "unknown"
)

// CodeBlock represents a code snippet.
type CodeBlock struct {
	Language string
	Code     string
}

// EndpointInfo represents a detected API endpoint.
type EndpointInfo struct {
	Method      string
	Path        string
	Description string
}

// Header represents a section header.
type Header struct {
	Level   int
	Text    string
	ID      string
	Content string
}

// Table represents a data table.
type Table struct {
	Headers []string
	Rows    [][]string
}

// List represents a list structure.
type List struct {
	Ordered bool
	Items   []string
}

// Extractor provides content extraction functionality.
type Extractor struct{}

// NewExtractor creates a new content extractor.
func NewExtractor() *Extractor {
	return &Extractor{}
}

// Extract extracts content from HTML.
func (e *Extractor) Extract(htmlContent []byte, pageURL string) (*ExtractedContent, error) {
	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Use readability to extract main content
	article, err := readability.FromReader(bytes.NewReader(htmlContent), parsedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to extract content: %w", err)
	}

	// Parse HTML for structured extraction
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	content := &ExtractedContent{
		Title:       article.Title,
		Author:      article.Byline,
		Description: article.Excerpt,
		Content:     article.Content,
		TextContent: article.TextContent,
		URL:         pageURL,
		SiteName:    article.SiteName,
		Image:       article.Image,
		Favicon:     article.Favicon,
	}

	if article.PublishedTime != nil {
		content.PublishedAt = *article.PublishedTime
	}

	// Extract structured content
	content.CodeBlocks = e.extractCodeBlocks(doc)
	content.Endpoints = e.extractEndpoints(doc, article.TextContent)
	content.Headers = e.extractHeaders(doc)
	content.Tables = e.extractTables(doc)
	content.Lists = e.extractLists(doc)
	content.ContentType = e.detectContentType(content)

	return content, nil
}

// extractCodeBlocks extracts code snippets from the document.
func (e *Extractor) extractCodeBlocks(doc *goquery.Document) []CodeBlock {
	var blocks []CodeBlock

	doc.Find("pre code, pre, code").Each(func(i int, s *goquery.Selection) {
		// Get parent pre if this is a code inside pre
		parent := s.Parent()
		if parent.Is("pre") && s.Is("code") {
			return // Skip, will be handled by parent
		}

		code := strings.TrimSpace(s.Text())
		if len(code) < 10 {
			return // Skip very short snippets
		}

		// Try to detect language
		lang := ""
		if class, exists := s.Attr("class"); exists {
			// Common patterns: language-js, lang-python, highlight-javascript
			langPattern := regexp.MustCompile(`(?:language-|lang-|highlight-)(\w+)`)
			if matches := langPattern.FindStringSubmatch(class); len(matches) > 1 {
				lang = matches[1]
			}
		}

		// Additional language detection from data attributes
		if lang == "" {
			if dataLang, exists := s.Attr("data-language"); exists {
				lang = dataLang
			} else if dataLang, exists := s.Attr("data-lang"); exists {
				lang = dataLang
			}
		}

		// Heuristic language detection
		if lang == "" {
			lang = e.detectLanguage(code)
		}

		blocks = append(blocks, CodeBlock{
			Language: lang,
			Code:     code,
		})
	})

	return blocks
}

// detectLanguage attempts to detect the programming language of code.
func (e *Extractor) detectLanguage(code string) string {
	code = strings.ToLower(code)

	// Check for common patterns
	patterns := map[string][]string{
		"bash": {"#!/bin/bash", "curl ", "wget ", "$ "},
		"javascript": {"const ", "let ", "function ", "=>", "async ", "await "},
		"typescript": {"interface ", "type ", ": string", ": number"},
		"python": {"def ", "import ", "from ", "print(", "if __name__"},
		"go": {"func ", "package ", "import (", "type ", "struct {"},
		"json": {`{"`, `":`},
		"yaml": {"---", "  -"},
		"graphql": {"query ", "mutation ", "type Query", "type Mutation"},
		"sql": {"select ", "insert ", "update ", "delete ", "create table"},
		"html": {"<html", "<div", "<span", "<!doctype"},
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

// extractEndpoints detects API endpoints from content.
func (e *Extractor) extractEndpoints(doc *goquery.Document, textContent string) []EndpointInfo {
	var endpoints []EndpointInfo
	seen := make(map[string]bool)

	// Pattern for HTTP methods and paths
	endpointPattern := regexp.MustCompile(`(?i)(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s+(/[^\s"'<>]+)`)

	matches := endpointPattern.FindAllStringSubmatch(textContent, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			method := strings.ToUpper(match[1])
			path := match[2]

			// Clean up path
			path = strings.TrimRight(path, ".,;:)")

			key := method + " " + path
			if !seen[key] {
				seen[key] = true
				endpoints = append(endpoints, EndpointInfo{
					Method: method,
					Path:   path,
				})
			}
		}
	}

	// Also look for URL patterns in code blocks
	doc.Find("pre code, code").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		matches := endpointPattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) >= 3 {
				method := strings.ToUpper(match[1])
				path := match[2]
				path = strings.TrimRight(path, ".,;:)")

				key := method + " " + path
				if !seen[key] {
					seen[key] = true
					endpoints = append(endpoints, EndpointInfo{
						Method: method,
						Path:   path,
					})
				}
			}
		}
	})

	return endpoints
}

// extractHeaders extracts section headers from the document.
func (e *Extractor) extractHeaders(doc *goquery.Document) []Header {
	var headers []Header

	doc.Find("h1, h2, h3, h4, h5, h6").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text == "" {
			return
		}

		level := 1
		switch s.Nodes[0].Data {
		case "h1":
			level = 1
		case "h2":
			level = 2
		case "h3":
			level = 3
		case "h4":
			level = 4
		case "h5":
			level = 5
		case "h6":
			level = 6
		}

		id, _ := s.Attr("id")

		headers = append(headers, Header{
			Level: level,
			Text:  text,
			ID:    id,
		})
	})

	return headers
}

// extractTables extracts data tables from the document.
func (e *Extractor) extractTables(doc *goquery.Document) []Table {
	var tables []Table

	doc.Find("table").Each(func(i int, s *goquery.Selection) {
		var table Table

		// Extract headers
		s.Find("thead th, thead td, tr:first-child th").Each(func(i int, th *goquery.Selection) {
			table.Headers = append(table.Headers, strings.TrimSpace(th.Text()))
		})

		// Extract rows
		s.Find("tbody tr, tr").Each(func(i int, tr *goquery.Selection) {
			// Skip header row
			if tr.Find("th").Length() > 0 && len(table.Headers) > 0 {
				return
			}

			var row []string
			tr.Find("td").Each(func(j int, td *goquery.Selection) {
				row = append(row, strings.TrimSpace(td.Text()))
			})

			if len(row) > 0 {
				table.Rows = append(table.Rows, row)
			}
		})

		if len(table.Headers) > 0 || len(table.Rows) > 0 {
			tables = append(tables, table)
		}
	})

	return tables
}

// extractLists extracts list structures from the document.
func (e *Extractor) extractLists(doc *goquery.Document) []List {
	var lists []List

	doc.Find("ul, ol").Each(func(i int, s *goquery.Selection) {
		list := List{
			Ordered: s.Is("ol"),
		}

		s.Find("> li").Each(func(j int, li *goquery.Selection) {
			text := strings.TrimSpace(li.Text())
			if text != "" {
				// Limit length
				if len(text) > 500 {
					text = text[:500] + "..."
				}
				list.Items = append(list.Items, text)
			}
		})

		if len(list.Items) > 0 {
			lists = append(lists, list)
		}
	})

	return lists
}

// detectContentType determines the type of content.
func (e *Extractor) detectContentType(content *ExtractedContent) ContentType {
	text := strings.ToLower(content.TextContent)
	title := strings.ToLower(content.Title)

	// API documentation indicators
	apiIndicators := []string{
		"api reference", "api documentation", "rest api",
		"endpoint", "request", "response", "authentication",
		"api key", "bearer token", "http method",
	}

	apiScore := 0
	for _, indicator := range apiIndicators {
		if strings.Contains(text, indicator) || strings.Contains(title, indicator) {
			apiScore++
		}
	}

	// Tutorial indicators
	tutorialIndicators := []string{
		"tutorial", "how to", "step by step", "getting started",
		"guide", "walkthrough", "learn", "example",
	}

	tutorialScore := 0
	for _, indicator := range tutorialIndicators {
		if strings.Contains(text, indicator) || strings.Contains(title, indicator) {
			tutorialScore++
		}
	}

	// Additional scoring based on structure
	if len(content.Endpoints) > 3 {
		apiScore += 2
	}

	if len(content.CodeBlocks) > 2 {
		tutorialScore++
		apiScore++
	}

	// Determine type
	if apiScore >= 3 || len(content.Endpoints) > 5 {
		return ContentTypeAPI
	}

	if tutorialScore >= 2 {
		return ContentTypeTutorial
	}

	if len(content.Headers) > 3 || len(content.TextContent) > 500 {
		return ContentTypeArticle
	}

	return ContentTypeUnknown
}
