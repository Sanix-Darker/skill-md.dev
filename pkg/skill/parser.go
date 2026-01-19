package skill

import (
	"regexp"
	"strings"

	"github.com/adrg/frontmatter"
)

var headerRegex = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

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
