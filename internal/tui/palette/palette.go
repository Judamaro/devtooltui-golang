// Package palette is the command palette (Ctrl+P): fuzzy search over tools
// and actions, drawn over the view.
package palette

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"
)

// Kind tells tools from actions.
type Kind int

const (
	ToolItem Kind = iota
	ActionItem
)

// Item is a palette entry.
type Item struct {
	Kind   Kind
	ID     string // tool or action ID
	Title  string
	Detail string // category or shortcut
	Search string // extra text to search (keywords)
}

// ChosenMsg is sent when an item is chosen with Enter.
type ChosenMsg struct{ Item Item }

// ClosedMsg is sent when closing without choosing (Esc).
type ClosedMsg struct{}

// Styles are defined by tui/theme.go.
type Styles struct {
	Box, Title, Item, Selected, Detail lipgloss.Style
	Input                              textinput.Styles
}

// KeyMap holds the palette keys.
type KeyMap struct {
	Up, Down, Choose, Close key.Binding
}

const maxRows = 10

// Model is the open palette.
type Model struct {
	title    string
	items    []Item
	filtered []Item
	cursor   int
	input    textinput.Model
	styles   Styles
	keys     KeyMap
	box      geometry // where Overlay drew it, for clicks
	first    int      // index of the first visible item
	itemsTop int      // screen row of the first visible item
	shown    int      // visible items
}

// geometry is a drawn box on screen.
type geometry struct{ x, y, w, h int }

func (g geometry) contains(x, y int) bool {
	return x >= g.x && x < g.x+g.w && y >= g.y && y < g.y+g.h
}

// place returns where Place draws box.
func place(box string, width, height int) geometry {
	w, h := lipgloss.Width(box), lipgloss.Height(box)
	return geometry{x: max((width-w)/2, 0), y: min(2, max(height-h, 0)), w: w, h: h}
}

// Contains reports whether (x, y) is on the palette as last drawn.
func (m *Model) Contains(x, y int) bool { return m.box.contains(x, y) }

// Click chooses the item at (x, y), as Enter does on it; it returns nil
// when there is no item there.
func (m *Model) Click(x, y int) tea.Cmd {
	if !m.box.contains(x, y) {
		return nil
	}
	i := y - m.itemsTop
	if i < 0 || i >= m.shown {
		return nil
	}
	m.cursor = m.first + i
	it := m.filtered[m.cursor]
	return func() tea.Msg { return ChosenMsg{Item: it} }
}

// New opens the palette with items, headed by title.
func New(title string, items []Item, styles Styles, keys KeyMap) *Model {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.Placeholder = "search for a tool or action…"
	ti.SetStyles(styles.Input)
	ti.Focus()
	m := &Model{title: title, items: items, input: ti, styles: styles, keys: keys}
	m.filter()
	return m
}

// Init starts the search field's cursor.
func (m *Model) Init() tea.Cmd { return textinput.Blink }

// Filtered returns the visible items (for tests).
func (m *Model) Filtered() []Item { return m.filtered }

func (m *Model) filter() {
	q := strings.TrimSpace(m.input.Value())
	if q == "" {
		m.filtered = m.items
	} else {
		targets := make([]string, len(m.items))
		for i, it := range m.items {
			targets[i] = it.Title + " " + it.ID + " " + it.Search
		}
		m.filtered = m.filtered[:0:0]
		for _, match := range fuzzy.Find(q, targets) {
			m.filtered = append(m.filtered, m.items[match.Index])
		}
	}
	m.cursor = min(m.cursor, max(len(m.filtered)-1, 0))
}

// Update handles keys and typing.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(k, m.keys.Close):
			return func() tea.Msg { return ClosedMsg{} }
		case key.Matches(k, m.keys.Choose):
			if len(m.filtered) == 0 {
				return nil
			}
			it := m.filtered[m.cursor]
			return func() tea.Msg { return ChosenMsg{Item: it} }
		case key.Matches(k, m.keys.Up):
			m.cursor = max(m.cursor-1, 0)
			return nil
		case key.Matches(k, m.keys.Down):
			m.cursor = min(m.cursor+1, max(len(m.filtered)-1, 0))
			return nil
		}
	}
	before := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != before {
		m.cursor = 0
		m.filter()
	}
	return cmd
}

// Overlay draws the palette centred near the top of base.
func (m *Model) Overlay(base string, width, height int) string {
	boxW := min(64, width-4)
	if boxW < 20 {
		return base
	}
	inner := boxW - 4 // border + padding
	m.input.SetWidth(inner - 2)
	var lines []string
	lines = append(lines, m.styles.Title.Render(m.title), m.input.View(), "")
	start := max(0, m.cursor-maxRows+1)
	end := min(len(m.filtered), start+maxRows)
	if len(m.filtered) == 0 {
		lines = append(lines, m.styles.Detail.Render("no results"))
	}
	for i := start; i < end; i++ {
		it := m.filtered[i]
		detail := m.styles.Detail.Render(it.Detail)
		title := it.Title
		gap := max(inner-2-lipgloss.Width(title)-lipgloss.Width(detail), 1)
		row := title + strings.Repeat(" ", gap) + detail
		if i == m.cursor {
			lines = append(lines, m.styles.Selected.Render("▸ "+row))
		} else {
			lines = append(lines, m.styles.Item.Render("  "+row))
		}
	}
	if len(m.filtered) > maxRows {
		lines = append(lines, m.styles.Detail.Render(fmt.Sprintf("  %d of %d", end-start, len(m.filtered))))
	}
	box := m.styles.Box.Width(boxW).Render(strings.Join(lines, "\n"))
	m.box = place(box, width, height)
	// Items start after the top border, the title, the search field and a
	// blank line.
	m.first, m.shown = start, end-start
	m.itemsTop = m.box.y + m.styles.Box.GetBorderTopSize() + m.styles.Box.GetPaddingTop() + 3
	return Place(base, box, width, height)
}

// Place draws box horizontally centred near the top edge of base, without
// changing its size.
func Place(base, box string, width, height int) string {
	g := place(box, width, height)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(box).X(g.x).Y(g.y).Z(1),
	).Render()
}
