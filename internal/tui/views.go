package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sanixdarker/skill-md/internal/converter"
	"github.com/sanixdarker/skill-md/internal/registry"
	"github.com/sanixdarker/skill-md/internal/sources"
	"github.com/sanixdarker/skill-md/pkg/skill"
)

// HomeModel is the home view model.
type HomeModel struct {
	keys     KeyMap
	styles   Styles
	width    int
	height   int
	selected int
	items    []string
}

// NewHomeModel creates a new home model.
func NewHomeModel(keys KeyMap, styles Styles) HomeModel {
	return HomeModel{
		keys:     keys,
		styles:   styles,
		selected: 0,
		items: []string{
			"Convert file/URL to SKILL.md",
			"Search skills (local + external)",
			"Merge multiple skills",
			"Browse local registry",
			"Quit",
		},
	}
}

// Init implements tea.Model.
func (m HomeModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m HomeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.selected > 0 {
				m.selected--
			}
		case key.Matches(msg, m.keys.Down):
			if m.selected < len(m.items)-1 {
				m.selected++
			}
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m HomeModel) View() string {
	var b strings.Builder

	// Header
	title := m.styles.Title.Render("SKILL MD")
	subtitle := m.styles.Subtitle.Render("Convert specs to SKILL.md for AI agents")

	b.WriteString("\n")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(subtitle)
	b.WriteString("\n\n")

	// Menu items
	for i, item := range m.items {
		cursor := "  "
		style := m.styles.MenuItem
		if i == m.selected {
			cursor = m.styles.Accent.Render("> ")
			style = m.styles.MenuItemSel
		}
		b.WriteString(cursor + style.Render(fmt.Sprintf("[%c] %s", 'C'+rune(i*('B'-'C')), item)) + "\n")
	}

	// Help
	b.WriteString("\n")
	b.WriteString(m.styles.Help.Render("Use arrow keys or j/k to navigate, enter to select, q to quit"))

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		lipgloss.NewStyle().Padding(2).Render(b.String()),
	)
}

// ConvertModel is the convert view model.
type ConvertModel struct {
	keys       KeyMap
	styles     Styles
	width      int
	height     int
	textarea   textarea.Model
	input      textinput.Model
	result     string
	err        error
	converting bool
	focus      int // 0 = textarea, 1 = format input
}

// NewConvertModel creates a new convert model.
func NewConvertModel(keys KeyMap, styles Styles) ConvertModel {
	ta := textarea.New()
	ta.Placeholder = "Paste your OpenAPI, GraphQL, or other specification here..."
	ta.CharLimit = 100000
	ta.SetWidth(60)
	ta.SetHeight(10)
	ta.Focus()

	ti := textinput.New()
	ti.Placeholder = "Format (auto, openapi, graphql, postman, url, text)"
	ti.Width = 50

	return ConvertModel{
		keys:     keys,
		styles:   styles,
		textarea: ta,
		input:    ti,
		focus:    0,
	}
}

// Init implements tea.Model.
func (m ConvertModel) Init() tea.Cmd {
	return textarea.Blink
}

// Update implements tea.Model.
func (m ConvertModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.focus = (m.focus + 1) % 2
			if m.focus == 0 {
				m.textarea.Focus()
				m.input.Blur()
			} else {
				m.textarea.Blur()
				m.input.Focus()
			}
			return m, nil
		case "ctrl+s":
			// Trigger conversion
			return m, m.convert()
		}
	case convertResultMsg:
		m.converting = false
		m.result = msg.result
		m.err = msg.err
		return m, nil
	}

	// Update focused component
	var cmd tea.Cmd
	if m.focus == 0 {
		m.textarea, cmd = m.textarea.Update(msg)
	} else {
		m.input, cmd = m.input.Update(msg)
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

type convertResultMsg struct {
	result string
	err    error
}

func (m ConvertModel) convert() tea.Cmd {
	return func() tea.Msg {
		content := m.textarea.Value()
		if content == "" {
			return convertResultMsg{err: fmt.Errorf("no content to convert")}
		}

		format := m.input.Value()
		if format == "" {
			format = "auto"
		}

		mgr := converter.NewManager()
		if format == "auto" {
			format = mgr.DetectFormat("input.txt", []byte(content))
		}

		sk, err := mgr.Convert(format, []byte(content), &converter.Options{
			Name: "Converted Skill",
		})
		if err != nil {
			return convertResultMsg{err: err}
		}

		return convertResultMsg{result: skill.Render(sk)}
	}
}

// View implements tea.Model.
func (m ConvertModel) View() string {
	var b strings.Builder

	// Header
	b.WriteString(m.styles.Title.Render("Convert to SKILL.md"))
	b.WriteString("\n\n")

	// Input area
	b.WriteString(m.styles.Normal.Render("Paste content:"))
	b.WriteString("\n")
	b.WriteString(m.textarea.View())
	b.WriteString("\n\n")

	// Format input
	b.WriteString(m.styles.Normal.Render("Format: "))
	b.WriteString(m.input.View())
	b.WriteString("\n\n")

	// Status/Result
	if m.converting {
		b.WriteString(m.styles.Muted.Render("Converting..."))
	} else if m.err != nil {
		b.WriteString(m.styles.Error.Render("Error: " + m.err.Error()))
	} else if m.result != "" {
		// Show truncated result
		preview := m.result
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		b.WriteString(m.styles.Success.Render("Conversion successful!"))
		b.WriteString("\n\n")
		b.WriteString(m.styles.Muted.Render(preview))
	}

	// Help
	b.WriteString("\n\n")
	b.WriteString(m.styles.Help.Render("Tab: switch focus | Ctrl+S: convert | Esc: back"))

	return lipgloss.NewStyle().Padding(2).Render(b.String())
}

// BrowseModel is the browse view model.
type BrowseModel struct {
	keys     KeyMap
	styles   Styles
	width    int
	height   int
	registry *registry.Service
	skills   []*skill.StoredSkill
	selected int
	err      error
	detail   *skill.StoredSkill
}

// NewBrowseModel creates a new browse model.
func NewBrowseModel(keys KeyMap, styles Styles, registryService *registry.Service) BrowseModel {
	return BrowseModel{
		keys:     keys,
		styles:   styles,
		registry: registryService,
	}
}

// LoadSkills loads skills from the registry.
func (m BrowseModel) LoadSkills() BrowseModel {
	if m.registry == nil {
		m.err = fmt.Errorf("no registry connection")
		return m
	}

	skills, _, err := m.registry.ListSkills(1, 100)
	if err != nil {
		m.err = err
		return m
	}
	m.skills = skills
	m.selected = 0
	m.detail = nil
	return m
}

// Init implements tea.Model.
func (m BrowseModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m BrowseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.detail != nil {
			// In detail view, any key goes back to list
			if key.Matches(msg, m.keys.Back) || key.Matches(msg, m.keys.Enter) {
				m.detail = nil
				return m, nil
			}
		}

		switch {
		case key.Matches(msg, m.keys.Up):
			if m.selected > 0 {
				m.selected--
			}
		case key.Matches(msg, m.keys.Down):
			if m.selected < len(m.skills)-1 {
				m.selected++
			}
		case key.Matches(msg, m.keys.Enter):
			if len(m.skills) > 0 && m.selected < len(m.skills) {
				m.detail = m.skills[m.selected]
			}
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m BrowseModel) View() string {
	var b strings.Builder

	// Header
	b.WriteString(m.styles.Title.Render("Browse Skills"))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(m.styles.Error.Render("Error: " + m.err.Error()))
		b.WriteString("\n\n")
		b.WriteString(m.styles.Help.Render("Esc: back to home"))
		return lipgloss.NewStyle().Padding(2).Render(b.String())
	}

	// Show detail view
	if m.detail != nil {
		b.WriteString(m.styles.Accent.Render(m.detail.Name))
		b.WriteString("\n")
		if m.detail.Description != "" {
			b.WriteString(m.styles.Muted.Render(m.detail.Description))
			b.WriteString("\n")
		}
		b.WriteString("\n")

		// Show metadata
		if m.detail.Version != "" {
			b.WriteString(fmt.Sprintf("Version: %s\n", m.detail.Version))
		}
		if len(m.detail.Tags) > 0 {
			b.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(m.detail.Tags, ", ")))
		}
		if m.detail.SourceFormat != "" {
			b.WriteString(fmt.Sprintf("Source: %s\n", m.detail.SourceFormat))
		}
		b.WriteString(fmt.Sprintf("Views: %d\n", m.detail.ViewCount))

		b.WriteString("\n")
		b.WriteString(m.styles.Help.Render("Enter/Esc: back to list"))
		return lipgloss.NewStyle().Padding(2).Render(b.String())
	}

	// Show list
	if len(m.skills) == 0 {
		b.WriteString(m.styles.Muted.Render("No skills found in registry."))
		b.WriteString("\n\n")
		b.WriteString(m.styles.Help.Render("Esc: back to home"))
		return lipgloss.NewStyle().Padding(2).Render(b.String())
	}

	for i, sk := range m.skills {
		cursor := "  "
		style := m.styles.MenuItem
		if i == m.selected {
			cursor = m.styles.Accent.Render("> ")
			style = m.styles.MenuItemSel
		}

		name := sk.Name
		if name == "" {
			name = "(unnamed)"
		}

		desc := ""
		if sk.Description != "" {
			desc = " - " + truncate(sk.Description, 40)
		}

		b.WriteString(cursor + style.Render(name) + m.styles.Muted.Render(desc) + "\n")
	}

	// Help
	b.WriteString("\n")
	b.WriteString(m.styles.Help.Render("j/k: navigate | Enter: view details | Esc: back"))

	return lipgloss.NewStyle().Padding(2).Render(b.String())
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// SearchModel is the search view model.
type SearchModel struct {
	keys            KeyMap
	styles          Styles
	width           int
	height          int
	registry        *registry.Service
	federatedSource *sources.FederatedSource
	input           textinput.Model
	source          int // 0=all, 1=local, 2=github, 3=gitlab, 4=skills.sh
	sources         []string
	sourceTypes     []sources.SourceType
	results         []searchResult
	selected        int
	err             error
	searching       bool
	detail          *searchResult
}

type searchResult struct {
	Name        string
	Description string
	Source      string
	ID          string
	Content     string
}

// NewSearchModel creates a new search model.
func NewSearchModel(keys KeyMap, styles Styles, registryService *registry.Service, federatedSource *sources.FederatedSource) SearchModel {
	ti := textinput.New()
	ti.Placeholder = "Enter search query..."
	ti.Width = 50
	ti.Focus()

	return SearchModel{
		keys:            keys,
		styles:          styles,
		registry:        registryService,
		federatedSource: federatedSource,
		input:           ti,
		source:          0,
		sources:         []string{"All", "Local", "GitHub", "GitLab", "skills.sh"},
		sourceTypes: []sources.SourceType{
			"",                         // All
			sources.SourceTypeLocal,    // Local
			sources.SourceTypeGitHub,   // GitHub
			sources.SourceTypeGitLab,   // GitLab
			sources.SourceTypeSkillsSH, // skills.sh
		},
	}
}

// Init implements tea.Model.
func (m SearchModel) Init() tea.Cmd {
	return textinput.Blink
}

type searchResultsMsg struct {
	results []searchResult
	err     error
}

// Update implements tea.Model.
func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.detail != nil {
			// In detail view
			m.detail = nil
			return m, nil
		}

		switch msg.String() {
		case "tab":
			m.source = (m.source + 1) % len(m.sources)
			return m, nil
		case "enter":
			if len(m.results) > 0 && !m.searching {
				if m.selected < len(m.results) {
					m.detail = &m.results[m.selected]
				}
				return m, nil
			}
			// Trigger search
			m.searching = true
			return m, m.doSearch()
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.results)-1 {
				m.selected++
			}
		}

	case searchResultsMsg:
		m.searching = false
		m.results = msg.results
		m.err = msg.err
		m.selected = 0
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m SearchModel) doSearch() tea.Cmd {
	return func() tea.Msg {
		query := m.input.Value()
		if query == "" {
			return searchResultsMsg{err: fmt.Errorf("enter a search query")}
		}

		var results []searchResult

		// Search local registry
		if m.source == 0 || m.source == 1 {
			if m.registry != nil {
				skills, _, err := m.registry.SearchSkills(query, 1, 20)
				if err == nil {
					for _, sk := range skills {
						results = append(results, searchResult{
							Name:        sk.Name,
							Description: sk.Description,
							Source:      "local",
							ID:          sk.ID,
							Content:     sk.Content,
						})
					}
				}
			}
		}

		// Search external sources using FederatedSource
		if m.federatedSource != nil && m.source != 1 { // Not local-only
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			opts := sources.SearchOptions{
				Query:   query,
				Page:    1,
				PerPage: 20,
			}

			var sourcesToSearch []sources.SourceType
			if m.source > 1 && m.source < len(m.sourceTypes) {
				// Specific external source selected
				sourcesToSearch = []sources.SourceType{m.sourceTypes[m.source]}
			}
			// If source == 0 (All), sourcesToSearch stays nil to search all

			result, err := m.federatedSource.SearchSources(ctx, opts, sourcesToSearch)
			if err == nil && result != nil {
				for _, sk := range result.Skills {
					// Skip local results as we already have them
					if sk.Source == sources.SourceTypeLocal {
						continue
					}
					results = append(results, searchResult{
						Name:        sk.Name,
						Description: sk.Description,
						Source:      string(sk.Source),
						ID:          sk.ID,
						Content:     sk.Content,
					})
				}
			}
		}

		if len(results) == 0 {
			return searchResultsMsg{err: fmt.Errorf("no results found")}
		}

		return searchResultsMsg{results: results}
	}
}

// View implements tea.Model.
func (m SearchModel) View() string {
	var b strings.Builder

	b.WriteString(m.styles.Title.Render("Search Skills"))
	b.WriteString("\n\n")

	// Source selector
	b.WriteString(m.styles.Normal.Render("Source: "))
	for i, src := range m.sources {
		if i == m.source {
			b.WriteString(m.styles.Accent.Render("[" + src + "] "))
		} else {
			b.WriteString(m.styles.Muted.Render(" " + src + "  "))
		}
	}
	b.WriteString("\n\n")

	// Search input
	b.WriteString(m.styles.Normal.Render("Query: "))
	b.WriteString(m.input.View())
	b.WriteString("\n\n")

	// Detail view
	if m.detail != nil {
		b.WriteString(m.styles.Accent.Render(m.detail.Name))
		b.WriteString("\n")
		if m.detail.Description != "" {
			b.WriteString(m.styles.Muted.Render(m.detail.Description))
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("Source: %s\n", m.detail.Source))
		b.WriteString("\n")
		if m.detail.Content != "" {
			preview := m.detail.Content
			if len(preview) > 500 {
				preview = preview[:500] + "..."
			}
			b.WriteString(m.styles.Muted.Render(preview))
		}
		b.WriteString("\n\n")
		b.WriteString(m.styles.Help.Render("Any key: back to results"))
		return lipgloss.NewStyle().Padding(2).Render(b.String())
	}

	// Status
	if m.searching {
		b.WriteString(m.styles.Muted.Render("Searching..."))
	} else if m.err != nil {
		b.WriteString(m.styles.Error.Render("Error: " + m.err.Error()))
	} else if len(m.results) > 0 {
		b.WriteString(m.styles.Success.Render(fmt.Sprintf("Found %d results:", len(m.results))))
		b.WriteString("\n\n")

		for i, r := range m.results {
			cursor := "  "
			style := m.styles.MenuItem
			if i == m.selected {
				cursor = m.styles.Accent.Render("> ")
				style = m.styles.MenuItemSel
			}

			name := r.Name
			if name == "" {
				name = "(unnamed)"
			}
			srcBadge := m.styles.Muted.Render(" [" + r.Source + "]")
			b.WriteString(cursor + style.Render(name) + srcBadge + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(m.styles.Help.Render("Tab: change source | Enter: search/view | j/k: navigate | Esc: back"))

	return lipgloss.NewStyle().Padding(2).Render(b.String())
}

// MergeModel is the merge view model.
type MergeModel struct {
	keys     KeyMap
	styles   Styles
	width    int
	height   int
	registry *registry.Service
	skills   []*skill.StoredSkill
	selected []bool
	cursor   int
	err      error
	result   string
	merging  bool
}

// NewMergeModel creates a new merge model.
func NewMergeModel(keys KeyMap, styles Styles, registryService *registry.Service) MergeModel {
	return MergeModel{
		keys:     keys,
		styles:   styles,
		registry: registryService,
	}
}

// LoadSkills loads skills from the registry.
func (m MergeModel) LoadSkills() MergeModel {
	if m.registry == nil {
		m.err = fmt.Errorf("no registry connection")
		return m
	}

	skills, _, err := m.registry.ListSkills(1, 100)
	if err != nil {
		m.err = err
		return m
	}
	m.skills = skills
	m.selected = make([]bool, len(skills))
	m.cursor = 0
	m.result = ""
	return m
}

// Init implements tea.Model.
func (m MergeModel) Init() tea.Cmd {
	return nil
}

type mergeResultMsg struct {
	result string
	err    error
}

// Update implements tea.Model.
func (m MergeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.result != "" {
			// Clear result on any key
			m.result = ""
			return m, nil
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.skills)-1 {
				m.cursor++
			}
		case " ": // Space to toggle
			if m.cursor < len(m.selected) {
				m.selected[m.cursor] = !m.selected[m.cursor]
			}
		case "enter":
			// Trigger merge
			return m, m.doMerge()
		case "a": // Select all
			for i := range m.selected {
				m.selected[i] = true
			}
		case "n": // Select none
			for i := range m.selected {
				m.selected[i] = false
			}
		}

	case mergeResultMsg:
		m.merging = false
		m.result = msg.result
		m.err = msg.err
		return m, nil
	}

	return m, nil
}

func (m MergeModel) doMerge() tea.Cmd {
	return func() tea.Msg {
		var selectedSkills []*skill.StoredSkill
		for i, sel := range m.selected {
			if sel && i < len(m.skills) {
				selectedSkills = append(selectedSkills, m.skills[i])
			}
		}

		if len(selectedSkills) < 2 {
			return mergeResultMsg{err: fmt.Errorf("select at least 2 skills to merge")}
		}

		// Merge skills
		var merged strings.Builder
		merged.WriteString("---\n")
		merged.WriteString("name: Merged Skills\n")
		merged.WriteString("description: Combined from multiple skills\n")
		merged.WriteString("version: 1.0.0\n")
		merged.WriteString("---\n\n")
		merged.WriteString("# Merged Skills\n\n")

		for i, sk := range selectedSkills {
			merged.WriteString(fmt.Sprintf("## %d. %s\n\n", i+1, sk.Name))
			if sk.Description != "" {
				merged.WriteString(sk.Description + "\n\n")
			}
			// Extract body content (skip frontmatter)
			content := sk.Content
			if idx := strings.Index(content, "---"); idx != -1 {
				if endIdx := strings.Index(content[idx+3:], "---"); endIdx != -1 {
					content = strings.TrimSpace(content[idx+3+endIdx+3:])
				}
			}
			if content != "" {
				merged.WriteString(content)
				merged.WriteString("\n\n")
			}
		}

		return mergeResultMsg{result: merged.String()}
	}
}

// View implements tea.Model.
func (m MergeModel) View() string {
	var b strings.Builder

	b.WriteString(m.styles.Title.Render("Merge Skills"))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(m.styles.Error.Render("Error: " + m.err.Error()))
		b.WriteString("\n\n")
		b.WriteString(m.styles.Help.Render("Esc: back to home"))
		return lipgloss.NewStyle().Padding(2).Render(b.String())
	}

	// Show result
	if m.result != "" {
		b.WriteString(m.styles.Success.Render("Merge successful!"))
		b.WriteString("\n\n")
		preview := m.result
		if len(preview) > 800 {
			preview = preview[:800] + "..."
		}
		b.WriteString(m.styles.Muted.Render(preview))
		b.WriteString("\n\n")
		b.WriteString(m.styles.Help.Render("Any key: back to selection"))
		return lipgloss.NewStyle().Padding(2).Render(b.String())
	}

	if m.merging {
		b.WriteString(m.styles.Muted.Render("Merging..."))
		return lipgloss.NewStyle().Padding(2).Render(b.String())
	}

	// Count selected
	count := 0
	for _, sel := range m.selected {
		if sel {
			count++
		}
	}
	b.WriteString(m.styles.Normal.Render(fmt.Sprintf("Select skills to merge (%d selected):", count)))
	b.WriteString("\n\n")

	if len(m.skills) == 0 {
		b.WriteString(m.styles.Muted.Render("No skills found in registry."))
		b.WriteString("\n\n")
		b.WriteString(m.styles.Help.Render("Esc: back to home"))
		return lipgloss.NewStyle().Padding(2).Render(b.String())
	}

	for i, sk := range m.skills {
		cursor := "  "
		if i == m.cursor {
			cursor = m.styles.Accent.Render("> ")
		}

		checkbox := "[ ]"
		style := m.styles.MenuItem
		if m.selected[i] {
			checkbox = m.styles.Accent.Render("[x]")
			style = m.styles.Selected
		}

		name := sk.Name
		if name == "" {
			name = "(unnamed)"
		}

		b.WriteString(cursor + checkbox + " " + style.Render(name) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(m.styles.Help.Render("Space: toggle | a: all | n: none | Enter: merge | Esc: back"))

	return lipgloss.NewStyle().Padding(2).Render(b.String())
}
