package converter

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/sanixdarker/skill-md/internal/extractor"
	"github.com/sanixdarker/skill-md/pkg/skill"
)

// URLConverter converts web pages to SKILL.md.
type URLConverter struct {
	extractor *extractor.Extractor
}

// NewURLConverter creates a new URL converter.
func NewURLConverter() *URLConverter {
	return &URLConverter{
		extractor: extractor.NewExtractor(),
	}
}

func (c *URLConverter) Name() string {
	return "url"
}

func (c *URLConverter) CanHandle(filename string, content []byte) bool {
	// Check if content is a URL
	text := strings.TrimSpace(string(content))
	if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
		_, err := url.Parse(text)
		return err == nil
	}
	return false
}

func (c *URLConverter) Convert(content []byte, opts *Options) (*skill.Skill, error) {
	// Content should be a URL
	urlStr := strings.TrimSpace(string(content))
	if !strings.HasPrefix(urlStr, "http") {
		urlStr = "https://" + urlStr
	}

	// SSRF protection: validate URL before fetching
	if err := c.validateURL(urlStr); err != nil {
		return nil, err
	}

	// Fetch the page
	htmlContent, err := c.fetchURL(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}

	// Extract content
	extracted, err := c.extractor.Extract(htmlContent, urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to extract content: %w", err)
	}

	// Build skill based on content type
	s := c.buildSkill(extracted, opts)
	return s, nil
}

// ConvertFromURL fetches a URL and converts it to a skill.
func (c *URLConverter) ConvertFromURL(urlStr string, opts *Options) (*skill.Skill, error) {
	return c.Convert([]byte(urlStr), opts)
}

// validateURL checks if a URL is safe to fetch (SSRF protection).
func (c *URLConverter) validateURL(urlStr string) error {
	u, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Only allow http and https schemes
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http and https URLs are allowed")
	}

	// Block requests to private/internal networks
	host := u.Hostname()

	// Check common internal hostnames
	lowerHost := strings.ToLower(host)
	blockedHosts := []string{
		"localhost",
		"127.0.0.1",
		"0.0.0.0",
		"::1",
		"metadata.google.internal",
		"169.254.169.254", // AWS/GCP metadata endpoint
		"metadata",
		"kubernetes.default",
		"kubernetes.default.svc",
	}
	for _, blocked := range blockedHosts {
		if lowerHost == blocked || strings.HasSuffix(lowerHost, "."+blocked) {
			return fmt.Errorf("requests to internal hosts are not allowed")
		}
	}

	// Check if it's an IP address and validate
	ip := net.ParseIP(host)
	if ip != nil {
		if err := validateIP(ip); err != nil {
			return err
		}
	} else {
		// Resolve hostname and check the IP(s)
		ips, err := net.LookupIP(host)
		if err == nil {
			for _, resolvedIP := range ips {
				if err := validateIP(resolvedIP); err != nil {
					return fmt.Errorf("hostname resolves to blocked IP: %w", err)
				}
			}
		}
	}

	return nil
}

// validateIP checks if an IP address is safe (not internal/private)
func validateIP(ip net.IP) error {
	if ip.IsLoopback() {
		return fmt.Errorf("requests to loopback addresses are not allowed")
	}
	if ip.IsPrivate() {
		return fmt.Errorf("requests to private IP addresses are not allowed")
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("requests to link-local addresses are not allowed")
	}
	if ip.IsUnspecified() {
		return fmt.Errorf("requests to unspecified addresses are not allowed")
	}
	// Check IPv4 link-local (169.254.x.x)
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 169 && ip4[1] == 254 {
		return fmt.Errorf("requests to AWS/cloud metadata addresses are not allowed")
	}
	// Check for IPv6 unique local addresses (fc00::/7)
	if len(ip) == 16 && (ip[0]&0xfe) == 0xfc {
		return fmt.Errorf("requests to unique local IPv6 addresses are not allowed")
	}
	return nil
}

func (c *URLConverter) fetchURL(urlStr string) ([]byte, error) {
	var htmlContent []byte
	var fetchErr error

	collector := colly.NewCollector(
		colly.AllowURLRevisit(),
		colly.MaxDepth(1),
	)

	// Set reasonable timeouts
	collector.SetRequestTimeout(30 * time.Second)

	// Set User-Agent to avoid being blocked
	collector.UserAgent = "Mozilla/5.0 (compatible; SkillMD/1.0; +https://github.com/sanixdarker/skill-md)"

	collector.OnResponse(func(r *colly.Response) {
		htmlContent = r.Body
	})

	collector.OnError(func(r *colly.Response, err error) {
		fetchErr = err
	})

	if err := collector.Visit(urlStr); err != nil {
		return nil, err
	}

	if fetchErr != nil {
		return nil, fetchErr
	}

	if len(htmlContent) == 0 {
		return nil, fmt.Errorf("empty response from URL")
	}

	return htmlContent, nil
}

func (c *URLConverter) buildSkill(extracted *extractor.ExtractedContent, opts *Options) *skill.Skill {
	name := extracted.Title
	if opts != nil && opts.Name != "" {
		name = opts.Name
	}
	if name == "" {
		name = "Extracted Content"
	}

	description := extracted.Description
	if description == "" && len(extracted.TextContent) > 0 {
		// Use first 200 chars as description
		description = extracted.TextContent
		if len(description) > 200 {
			description = description[:200] + "..."
		}
	}

	s := skill.NewSkill(name, description)
	s.Frontmatter.SourceType = "url"
	s.Frontmatter.Source = extracted.URL

	// Set tags based on content type
	switch extracted.ContentType {
	case extractor.ContentTypeAPI:
		s.Frontmatter.Tags = []string{"api", "documentation"}
	case extractor.ContentTypeTutorial:
		s.Frontmatter.Tags = []string{"tutorial", "guide"}
	case extractor.ContentTypeArticle:
		s.Frontmatter.Tags = []string{"article", "reference"}
	default:
		s.Frontmatter.Tags = []string{"web", "extracted"}
	}

	if extracted.Author != "" {
		s.Frontmatter.Author = extracted.Author
	}

	// Count endpoints
	s.Frontmatter.EndpointCount = len(extracted.Endpoints)

	// Determine difficulty
	if len(extracted.Endpoints) <= 5 && len(extracted.CodeBlocks) <= 3 {
		s.Frontmatter.Difficulty = "novice"
	} else if len(extracted.Endpoints) <= 15 {
		s.Frontmatter.Difficulty = "intermediate"
	} else {
		s.Frontmatter.Difficulty = "advanced"
	}

	// Has examples if we have code blocks
	s.Frontmatter.HasExamples = len(extracted.CodeBlocks) > 0

	// Build sections based on content type
	switch extracted.ContentType {
	case extractor.ContentTypeAPI:
		c.buildAPISections(s, extracted)
	case extractor.ContentTypeTutorial:
		c.buildTutorialSections(s, extracted)
	default:
		c.buildArticleSections(s, extracted)
	}

	return s
}

func (c *URLConverter) buildAPISections(s *skill.Skill, extracted *extractor.ExtractedContent) {
	// Quick Start
	s.AddSection("Quick Start", 2, c.buildQuickStartSection(extracted))

	// Overview
	s.AddSection("Overview", 2, c.buildOverviewSection(extracted))

	// Endpoints
	if len(extracted.Endpoints) > 0 {
		s.AddSection("Endpoints", 2, c.buildEndpointsSection(extracted))
	}

	// Code Examples
	if len(extracted.CodeBlocks) > 0 {
		s.AddSection("Code Examples", 2, c.buildCodeExamplesSection(extracted))
	}

	// Data Structures (from tables)
	if len(extracted.Tables) > 0 {
		s.AddSection("Data Structures", 2, c.buildTablesSection(extracted))
	}

	// Best Practices
	s.AddSection("Best Practices", 2, c.buildBestPracticesSection(extracted))
}

func (c *URLConverter) buildTutorialSections(s *skill.Skill, extracted *extractor.ExtractedContent) {
	// Overview
	s.AddSection("Overview", 2, c.buildOverviewSection(extracted))

	// Steps (from headers)
	if len(extracted.Headers) > 0 {
		s.AddSection("Steps", 2, c.buildStepsSection(extracted))
	}

	// Code Examples
	if len(extracted.CodeBlocks) > 0 {
		s.AddSection("Code Examples", 2, c.buildCodeExamplesSection(extracted))
	}

	// Key Points (from lists)
	if len(extracted.Lists) > 0 {
		s.AddSection("Key Points", 2, c.buildListsSection(extracted))
	}
}

func (c *URLConverter) buildArticleSections(s *skill.Skill, extracted *extractor.ExtractedContent) {
	// Overview
	s.AddSection("Overview", 2, c.buildOverviewSection(extracted))

	// Content (organized by headers)
	if len(extracted.Headers) > 0 {
		s.AddSection("Content", 2, c.buildContentSection(extracted))
	}

	// Code Examples
	if len(extracted.CodeBlocks) > 0 {
		s.AddSection("Code Examples", 2, c.buildCodeExamplesSection(extracted))
	}

	// References (from tables)
	if len(extracted.Tables) > 0 {
		s.AddSection("Reference Data", 2, c.buildTablesSection(extracted))
	}
}

func (c *URLConverter) buildQuickStartSection(extracted *extractor.ExtractedContent) string {
	var b strings.Builder

	b.WriteString("Get started quickly with this API.\n\n")

	b.WriteString("### Source\n\n")
	b.WriteString(fmt.Sprintf("[%s](%s)\n\n", extracted.Title, extracted.URL))

	if len(extracted.Endpoints) > 0 {
		b.WriteString("### Available Endpoints\n\n")
		count := 3
		if len(extracted.Endpoints) < count {
			count = len(extracted.Endpoints)
		}
		for i := 0; i < count; i++ {
			ep := extracted.Endpoints[i]
			b.WriteString(fmt.Sprintf("- `%s %s`\n", ep.Method, ep.Path))
		}
		if len(extracted.Endpoints) > 3 {
			b.WriteString(fmt.Sprintf("- ...and %d more\n", len(extracted.Endpoints)-3))
		}
	}

	// Show first code example if available
	if len(extracted.CodeBlocks) > 0 {
		b.WriteString("\n### Example Request\n\n")
		block := extracted.CodeBlocks[0]
		lang := block.Language
		if lang == "" {
			lang = "bash"
		}
		// Limit code length for quick start
		code := block.Code
		if len(code) > 500 {
			code = code[:500] + "\n# ..."
		}
		b.WriteString(fmt.Sprintf("```%s\n%s\n```\n", lang, code))
	}

	return strings.TrimSpace(b.String())
}

func (c *URLConverter) buildOverviewSection(extracted *extractor.ExtractedContent) string {
	var b strings.Builder

	if extracted.Description != "" {
		b.WriteString(extracted.Description)
		b.WriteString("\n\n")
	}

	b.WriteString("### Source Information\n\n")
	b.WriteString(fmt.Sprintf("- **URL**: [%s](%s)\n", extracted.Title, extracted.URL))

	if extracted.SiteName != "" {
		b.WriteString(fmt.Sprintf("- **Site**: %s\n", extracted.SiteName))
	}

	if extracted.Author != "" {
		b.WriteString(fmt.Sprintf("- **Author**: %s\n", extracted.Author))
	}

	if !extracted.PublishedAt.IsZero() {
		b.WriteString(fmt.Sprintf("- **Published**: %s\n", extracted.PublishedAt.Format("2006-01-02")))
	}

	// Statistics
	b.WriteString("\n### Content Statistics\n\n")
	b.WriteString("| Metric | Count |\n")
	b.WriteString("|--------|-------|\n")
	if len(extracted.Endpoints) > 0 {
		b.WriteString(fmt.Sprintf("| Endpoints | %d |\n", len(extracted.Endpoints)))
	}
	if len(extracted.CodeBlocks) > 0 {
		b.WriteString(fmt.Sprintf("| Code Examples | %d |\n", len(extracted.CodeBlocks)))
	}
	if len(extracted.Tables) > 0 {
		b.WriteString(fmt.Sprintf("| Data Tables | %d |\n", len(extracted.Tables)))
	}
	if len(extracted.Headers) > 0 {
		b.WriteString(fmt.Sprintf("| Sections | %d |\n", len(extracted.Headers)))
	}

	return strings.TrimSpace(b.String())
}

func (c *URLConverter) buildEndpointsSection(extracted *extractor.ExtractedContent) string {
	var b strings.Builder

	b.WriteString("Detected API endpoints from the documentation.\n\n")

	// Group by method
	byMethod := make(map[string][]extractor.EndpointInfo)
	for _, ep := range extracted.Endpoints {
		byMethod[ep.Method] = append(byMethod[ep.Method], ep)
	}

	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	for _, method := range methods {
		endpoints := byMethod[method]
		if len(endpoints) == 0 {
			continue
		}

		b.WriteString(fmt.Sprintf("### %s\n\n", method))
		for _, ep := range endpoints {
			b.WriteString(fmt.Sprintf("- `%s`", ep.Path))
			if ep.Description != "" {
				b.WriteString(fmt.Sprintf(" - %s", ep.Description))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *URLConverter) buildCodeExamplesSection(extracted *extractor.ExtractedContent) string {
	var b strings.Builder

	b.WriteString("Code examples extracted from the documentation.\n\n")

	for i, block := range extracted.CodeBlocks {
		if i >= 10 {
			b.WriteString(fmt.Sprintf("\n*...and %d more examples*\n", len(extracted.CodeBlocks)-10))
			break
		}

		lang := block.Language
		if lang == "" {
			lang = "text"
		}

		// Title based on language
		title := fmt.Sprintf("Example %d", i+1)
		if lang != "text" {
			title = fmt.Sprintf("%s Example %d", strings.Title(lang), i+1)
		}

		b.WriteString(fmt.Sprintf("### %s\n\n", title))
		b.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n", lang, block.Code))
	}

	return strings.TrimSpace(b.String())
}

func (c *URLConverter) buildTablesSection(extracted *extractor.ExtractedContent) string {
	var b strings.Builder

	b.WriteString("Reference data extracted from tables.\n\n")

	for i, table := range extracted.Tables {
		if i >= 5 {
			b.WriteString(fmt.Sprintf("\n*...and %d more tables*\n", len(extracted.Tables)-5))
			break
		}

		b.WriteString(fmt.Sprintf("### Table %d\n\n", i+1))

		// Headers
		if len(table.Headers) > 0 {
			b.WriteString("|")
			for _, h := range table.Headers {
				b.WriteString(fmt.Sprintf(" %s |", h))
			}
			b.WriteString("\n|")
			for range table.Headers {
				b.WriteString("------|")
			}
			b.WriteString("\n")
		}

		// Rows
		for _, row := range table.Rows {
			b.WriteString("|")
			for _, cell := range row {
				// Truncate long cells
				if len(cell) > 50 {
					cell = cell[:50] + "..."
				}
				b.WriteString(fmt.Sprintf(" %s |", cell))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *URLConverter) buildStepsSection(extracted *extractor.ExtractedContent) string {
	var b strings.Builder

	b.WriteString("Step-by-step guide based on the tutorial structure.\n\n")

	step := 1
	for _, header := range extracted.Headers {
		if header.Level > 3 {
			continue // Skip deep headers
		}

		b.WriteString(fmt.Sprintf("### Step %d: %s\n\n", step, header.Text))
		step++
	}

	return strings.TrimSpace(b.String())
}

func (c *URLConverter) buildListsSection(extracted *extractor.ExtractedContent) string {
	var b strings.Builder

	b.WriteString("Key points from the content.\n\n")

	for i, list := range extracted.Lists {
		if i >= 5 {
			break
		}

		for _, item := range list.Items {
			if list.Ordered {
				b.WriteString(fmt.Sprintf("1. %s\n", item))
			} else {
				b.WriteString(fmt.Sprintf("- %s\n", item))
			}
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func (c *URLConverter) buildContentSection(extracted *extractor.ExtractedContent) string {
	var b strings.Builder

	b.WriteString("Main content organized by section.\n\n")

	for _, header := range extracted.Headers {
		level := header.Level + 1 // Offset since we're in a section
		if level > 6 {
			level = 6
		}
		b.WriteString(fmt.Sprintf("%s %s\n\n", strings.Repeat("#", level), header.Text))
	}

	return strings.TrimSpace(b.String())
}

func (c *URLConverter) buildBestPracticesSection(extracted *extractor.ExtractedContent) string {
	var b strings.Builder

	b.WriteString("Recommendations for using this API effectively.\n\n")

	b.WriteString("### General Guidelines\n\n")
	b.WriteString("- Review the original documentation at the source URL\n")
	b.WriteString("- Test endpoints in a development environment first\n")
	b.WriteString("- Handle errors gracefully in production code\n")
	b.WriteString("- Implement proper authentication as required\n\n")

	b.WriteString("### Source Reference\n\n")
	b.WriteString(fmt.Sprintf("For the most up-to-date information, refer to:\n[%s](%s)\n",
		extracted.Title, extracted.URL))

	return strings.TrimSpace(b.String())
}

// FetchAndConvert fetches a URL and converts to skill (convenience function).
func FetchAndConvert(urlStr string, opts *Options) (*skill.Skill, error) {
	converter := NewURLConverter()
	return converter.ConvertFromURL(urlStr, opts)
}
