package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sanixdarker/skillforge/internal/converter"
	"github.com/sanixdarker/skillforge/internal/registry"
	"github.com/sanixdarker/skillforge/pkg/skill"
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
			"Convert file/URL to skill",
			"Browse skill registry",
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
	title := m.styles.Title.Render("SKILL FORGE")
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
		case "ctrl+enter":
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
	b.WriteString(m.styles.Help.Render("Tab: switch focus | Ctrl+Enter: convert | Esc: back"))

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
