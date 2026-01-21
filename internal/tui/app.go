// Package tui provides a terminal user interface for Skill MD.
package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sanixdarker/skill-md/internal/registry"
)

// View represents different views in the TUI.
type View int

const (
	ViewHome View = iota
	ViewConvert
	ViewBrowse
	ViewHelp
)

// KeyMap defines keyboard shortcuts.
type KeyMap struct {
	Home    key.Binding
	Convert key.Binding
	Browse  key.Binding
	Help    key.Binding
	Quit    key.Binding
	Back    key.Binding
	Enter   key.Binding
	Up      key.Binding
	Down    key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Home: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "home"),
		),
		Convert: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "convert"),
		),
		Browse: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "browse"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("up/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("down/j", "down"),
		),
	}
}

// Styles defines the visual styles for the TUI.
type Styles struct {
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	Header      lipgloss.Style
	Selected    lipgloss.Style
	Normal      lipgloss.Style
	Muted       lipgloss.Style
	Accent      lipgloss.Style
	Error       lipgloss.Style
	Success     lipgloss.Style
	Border      lipgloss.Style
	Box         lipgloss.Style
	MenuItem    lipgloss.Style
	MenuItemSel lipgloss.Style
	Help        lipgloss.Style
}

// DefaultStyles returns the default styling.
func DefaultStyles() Styles {
	accent := lipgloss.Color("#00ff41")
	muted := lipgloss.Color("#666666")
	text := lipgloss.Color("#e0e0e0")
	bg := lipgloss.Color("#0a0a0a")
	surface := lipgloss.Color("#111111")
	border := lipgloss.Color("#333333")

	return Styles{
		Title: lipgloss.NewStyle().
			Foreground(accent).
			Bold(true).
			MarginBottom(1),

		Subtitle: lipgloss.NewStyle().
			Foreground(muted).
			MarginBottom(1),

		Header: lipgloss.NewStyle().
			Foreground(text).
			Bold(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(border).
			MarginBottom(1).
			PaddingBottom(1),

		Selected: lipgloss.NewStyle().
			Foreground(accent).
			Bold(true),

		Normal: lipgloss.NewStyle().
			Foreground(text),

		Muted: lipgloss.NewStyle().
			Foreground(muted),

		Accent: lipgloss.NewStyle().
			Foreground(accent),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff4444")),

		Success: lipgloss.NewStyle().
			Foreground(accent),

		Border: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(border),

		Box: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Background(surface).
			Padding(1, 2),

		MenuItem: lipgloss.NewStyle().
			Foreground(text).
			PaddingLeft(2),

		MenuItemSel: lipgloss.NewStyle().
			Foreground(bg).
			Background(accent).
			Bold(true).
			PaddingLeft(2),

		Help: lipgloss.NewStyle().
			Foreground(muted).
			MarginTop(1),
	}
}

// Model is the main TUI model.
type Model struct {
	registry   *registry.Service
	keys       KeyMap
	styles     Styles
	width      int
	height     int
	view       View
	homeModel  HomeModel
	convertModel ConvertModel
	browseModel  BrowseModel
}

// NewModel creates a new TUI model.
func NewModel(registryService *registry.Service) Model {
	keys := DefaultKeyMap()
	styles := DefaultStyles()

	return Model{
		registry:     registryService,
		keys:         keys,
		styles:       styles,
		view:         ViewHome,
		homeModel:    NewHomeModel(keys, styles),
		convertModel: NewConvertModel(keys, styles),
		browseModel:  NewBrowseModel(keys, styles, registryService),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.homeModel.width = msg.Width
		m.homeModel.height = msg.Height
		m.convertModel.width = msg.Width
		m.convertModel.height = msg.Height
		m.browseModel.width = msg.Width
		m.browseModel.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Global quit handling
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}

		// Navigation from home view
		if m.view == ViewHome {
			switch {
			case key.Matches(msg, m.keys.Enter):
				switch m.homeModel.selected {
				case 0:
					m.view = ViewConvert
				case 1:
					m.view = ViewBrowse
					m.browseModel = m.browseModel.LoadSkills()
				case 2:
					return m, tea.Quit
				}
				return m, nil
			}
		}

		// Back to home from other views
		if m.view != ViewHome && key.Matches(msg, m.keys.Back) {
			m.view = ViewHome
			return m, nil
		}

		// Shortcut keys
		switch {
		case key.Matches(msg, m.keys.Home):
			m.view = ViewHome
			return m, nil
		case key.Matches(msg, m.keys.Convert):
			m.view = ViewConvert
			return m, nil
		case key.Matches(msg, m.keys.Browse):
			m.view = ViewBrowse
			m.browseModel = m.browseModel.LoadSkills()
			return m, nil
		}
	}

	// Delegate to current view
	var cmd tea.Cmd
	switch m.view {
	case ViewHome:
		var newHome tea.Model
		newHome, cmd = m.homeModel.Update(msg)
		m.homeModel = newHome.(HomeModel)
	case ViewConvert:
		var newConvert tea.Model
		newConvert, cmd = m.convertModel.Update(msg)
		m.convertModel = newConvert.(ConvertModel)
	case ViewBrowse:
		var newBrowse tea.Model
		newBrowse, cmd = m.browseModel.Update(msg)
		m.browseModel = newBrowse.(BrowseModel)
	}

	return m, cmd
}

// View implements tea.Model.
func (m Model) View() string {
	var content string

	switch m.view {
	case ViewHome:
		content = m.homeModel.View()
	case ViewConvert:
		content = m.convertModel.View()
	case ViewBrowse:
		content = m.browseModel.View()
	default:
		content = m.homeModel.View()
	}

	return content
}
