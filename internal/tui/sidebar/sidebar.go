// Package sidebar muestra las categorías y herramientas del registro.
package sidebar

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Judamaro/devtooltui-golang/internal/registry"
)

// Styles los define tui/theme.go.
type Styles struct {
	Category, Item, Selected, SelectedBlurred lipgloss.Style
}

// KeyMap son las teclas de navegación del sidebar.
type KeyMap struct {
	Up, Down key.Binding
}

// SelectedMsg se emite cuando cambia la herramienta seleccionada.
type SelectedMsg struct{ ID string }

type entry struct {
	header string // no vacío = encabezado de categoría
	id     string
	name   string
}

// Model es el sidebar. El cursor solo se posa sobre herramientas.
type Model struct {
	entries []entry
	cursor  int // índice en entries
	focused bool
	width   int
	height  int
	styles  Styles
	keys    KeyMap
}

func New(groups []registry.Group, styles Styles, keys KeyMap) Model {
	m := Model{styles: styles, keys: keys, cursor: -1}
	for _, g := range groups {
		m.entries = append(m.entries, entry{header: string(g.Category)})
		for _, t := range g.Tools {
			s := t.Spec()
			m.entries = append(m.entries, entry{id: s.ID, name: s.Name})
			if m.cursor < 0 {
				m.cursor = len(m.entries) - 1
			}
		}
	}
	return m
}

// Selected devuelve el ID de la herramienta bajo el cursor.
func (m Model) Selected() string {
	if m.cursor < 0 {
		return ""
	}
	return m.entries[m.cursor].id
}

func (m *Model) SetSize(w, h int) { m.width, m.height = w, h }
func (m *Model) Focus()           { m.focused = true }
func (m *Model) Blur()            { m.focused = false }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok || m.cursor < 0 {
		return m, nil
	}
	prev := m.cursor
	switch {
	case key.Matches(k, m.keys.Up):
		m.move(-1)
	case key.Matches(k, m.keys.Down):
		m.move(1)
	}
	if m.cursor == prev {
		return m, nil
	}
	id := m.Selected()
	return m, func() tea.Msg { return SelectedMsg{ID: id} }
}

func (m *Model) move(delta int) {
	for i := m.cursor + delta; i >= 0 && i < len(m.entries); i += delta {
		if m.entries[i].header == "" {
			m.cursor = i
			return
		}
	}
}

func (m Model) View() string {
	lines := make([]string, 0, len(m.entries))
	for i, e := range m.entries {
		switch {
		case e.header != "":
			lines = append(lines, m.styles.Category.Render("▾ "+e.header))
		case i == m.cursor && m.focused:
			lines = append(lines, m.styles.Selected.Render("  ▸ "+e.name))
		case i == m.cursor:
			lines = append(lines, m.styles.SelectedBlurred.Render("  ▸ "+e.name))
		default:
			lines = append(lines, m.styles.Item.Render("    "+e.name))
		}
	}
	// Mantener el cursor visible si la lista no cabe.
	if start := m.cursor - m.height + 1; m.height > 0 && start > 0 {
		lines = lines[start:]
	}
	return lipgloss.NewStyle().Width(m.width).MaxWidth(m.width).Height(m.height).MaxHeight(m.height).
		Render(strings.Join(lines, "\n"))
}
