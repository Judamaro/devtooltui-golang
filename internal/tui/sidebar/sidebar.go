// Package sidebar shows the registry's categories and tools, with a fuzzy
// filter opened with "/".
package sidebar

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"

	"github.com/Judamaro/devtooltui-golang/internal/registry"
)

// Styles are defined by tui/theme.go.
type Styles struct {
	Category, Item, Selected, SelectedBlurred, Filter lipgloss.Style
}

// KeyMap holds the sidebar keys.
type KeyMap struct {
	Up, Down    key.Binding
	Filter      key.Binding // starts filtering ("/")
	EndFilter   key.Binding // stops filtering and keeps the filter (Enter)
	ClearFilter key.Binding // clears the filter (Esc)
}

// SelectedMsg is sent when the selected tool changes.
type SelectedMsg struct{ ID string }

type entry struct {
	header string // non-empty = category header
	id     string
	name   string
	search string // name, ID and keywords
}

// Model is the sidebar. The cursor only rests on visible tools.
type Model struct {
	entries   []entry
	visible   []bool
	cursor    int // index into entries
	focused   bool
	filtering bool // typing the filter
	filter    string
	width     int
	height    int
	styles    Styles
	keys      KeyMap
}

func New(groups []registry.Group, styles Styles, keys KeyMap) Model {
	m := Model{styles: styles, keys: keys, cursor: -1}
	for _, g := range groups {
		m.entries = append(m.entries, entry{header: string(g.Category)})
		for _, t := range g.Tools {
			s := t.Spec()
			m.entries = append(m.entries, entry{id: s.ID, name: s.Name,
				search: s.Name + " " + s.ID + " " + strings.Join(s.Keywords, " ")})
			if m.cursor < 0 {
				m.cursor = len(m.entries) - 1
			}
		}
	}
	m.applyFilter()
	return m
}

// Selected returns the ID of the tool under the cursor.
func (m Model) Selected() string {
	if m.cursor < 0 {
		return ""
	}
	return m.entries[m.cursor].id
}

// Filtering reports whether the filter is being typed (keys are text).
func (m Model) Filtering() bool { return m.filtering }

// Select moves the cursor to tool id (clearing the filter if it hides it).
func (m *Model) Select(id string) {
	for i, e := range m.entries {
		if e.header == "" && e.id == id {
			if !m.visible[i] {
				m.filter, m.filtering = "", false
				m.applyFilter()
			}
			m.cursor = i
			return
		}
	}
}

func (m *Model) SetSize(w, h int) { m.width, m.height = w, h }
func (m *Model) Focus()           { m.focused = true }
func (m *Model) Blur()            { m.focused, m.filtering = false, false }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok || m.cursor < 0 {
		return m, nil
	}
	prev := m.cursor
	switch {
	case m.filtering && key.Matches(k, m.keys.EndFilter):
		m.filtering = false
	case key.Matches(k, m.keys.ClearFilter) && (m.filtering || m.filter != ""):
		m.filtering, m.filter = false, ""
		m.applyFilter()
	case m.filtering && k.Code == tea.KeyBackspace:
		if r := []rune(m.filter); len(r) > 0 {
			m.filter = string(r[:len(r)-1])
			m.applyFilter()
		}
	case m.filtering && k.Text != "": // while filtering, "j" and "k" are text
		m.filter += k.Text
		m.applyFilter()
	case key.Matches(k, m.keys.Up):
		m.move(-1)
	case key.Matches(k, m.keys.Down):
		m.move(1)
	case !m.filtering && key.Matches(k, m.keys.Filter):
		m.filtering = true
	}
	if m.cursor == prev {
		return m, nil
	}
	id := m.Selected()
	return m, func() tea.Msg { return SelectedMsg{ID: id} }
}

// applyFilter recomputes which entries are visible and leaves the cursor on
// a visible tool (the current one if it is still visible).
func (m *Model) applyFilter() {
	m.visible = make([]bool, len(m.entries))
	q := strings.TrimSpace(m.filter)
	header := -1
	for i, e := range m.entries {
		if e.header != "" {
			header = i
			continue
		}
		if q == "" || len(fuzzy.Find(q, []string{e.search})) > 0 {
			m.visible[i] = true
			if header >= 0 {
				m.visible[header] = true
			}
		}
	}
	if m.cursor >= 0 && m.visible[m.cursor] {
		return
	}
	for i := range m.entries {
		if m.visible[i] && m.entries[i].header == "" {
			m.cursor = i
			return
		}
	}
}

func (m *Model) move(delta int) {
	for i := m.cursor + delta; i >= 0 && i < len(m.entries); i += delta {
		if m.entries[i].header == "" && m.visible[i] {
			m.cursor = i
			return
		}
	}
}

func (m Model) View() string {
	var lines []string
	if m.filtering || m.filter != "" {
		cursor := ""
		if m.filtering {
			cursor = "▏"
		}
		lines = append(lines, m.styles.Filter.Render("/ "+m.filter+cursor))
	}
	bodyStart := len(lines)
	cursorLine := -1
	for i, e := range m.entries {
		if !m.visible[i] {
			continue
		}
		switch {
		case e.header != "":
			lines = append(lines, m.styles.Category.Render("▾ "+e.header))
		case i == m.cursor && m.focused:
			cursorLine = len(lines)
			lines = append(lines, m.styles.Selected.Render("  ▸ "+e.name))
		case i == m.cursor:
			cursorLine = len(lines)
			lines = append(lines, m.styles.SelectedBlurred.Render("  ▸ "+e.name))
		default:
			lines = append(lines, m.styles.Item.Render("    "+e.name))
		}
	}
	if len(lines) == bodyStart {
		lines = append(lines, m.styles.Item.Render("  (no results)"))
	}
	// Keep the cursor visible when the list does not fit (the filter stays put).
	if start := cursorLine - m.height + 1; m.height > 0 && start > bodyStart {
		lines = append(lines[:bodyStart], lines[start:]...)
	}
	return lipgloss.NewStyle().Width(m.width).MaxWidth(m.width).Height(m.height).MaxHeight(m.height).
		Render(strings.Join(lines, "\n"))
}
