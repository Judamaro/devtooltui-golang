// Package palette es la paleta de comandos (Ctrl+P): búsqueda difusa sobre
// herramientas y acciones, dibujada encima de la vista.
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

// Kind distingue herramientas de acciones.
type Kind int

const (
	ToolItem Kind = iota
	ActionItem
)

// Item es una entrada de la paleta.
type Item struct {
	Kind   Kind
	ID     string // ID de la herramienta o de la acción
	Title  string
	Detail string // categoría o atajo
	Search string // texto extra para la búsqueda (keywords)
}

// ChosenMsg se emite al elegir un ítem con Enter.
type ChosenMsg struct{ Item Item }

// ClosedMsg se emite al cerrar sin elegir (Esc).
type ClosedMsg struct{}

// Styles los define tui/theme.go.
type Styles struct {
	Box, Title, Item, Selected, Detail lipgloss.Style
	Input                              textinput.Styles
}

// KeyMap son las teclas de la paleta.
type KeyMap struct {
	Up, Down, Choose, Close key.Binding
}

const maxRows = 10

// Model es la paleta abierta.
type Model struct {
	title    string
	items    []Item
	filtered []Item
	cursor   int
	input    textinput.Model
	styles   Styles
	keys     KeyMap
}

// New abre la paleta con items; title la encabeza.
func New(title string, items []Item, styles Styles, keys KeyMap) *Model {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.Placeholder = "buscar herramienta o acción…"
	ti.SetStyles(styles.Input)
	ti.Focus()
	m := &Model{title: title, items: items, input: ti, styles: styles, keys: keys}
	m.filter()
	return m
}

// Init enciende el cursor del campo de búsqueda.
func (m *Model) Init() tea.Cmd { return textinput.Blink }

// Filtered devuelve los ítems visibles (para tests).
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

// Update procesa teclas y escritura.
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

// Overlay dibuja la paleta centrada en la parte superior de base.
func (m *Model) Overlay(base string, width, height int) string {
	boxW := min(64, width-4)
	if boxW < 20 {
		return base
	}
	inner := boxW - 4 // borde + padding
	m.input.SetWidth(inner - 2)
	var lines []string
	lines = append(lines, m.styles.Title.Render(m.title), m.input.View(), "")
	start := max(0, m.cursor-maxRows+1)
	end := min(len(m.filtered), start+maxRows)
	if len(m.filtered) == 0 {
		lines = append(lines, m.styles.Detail.Render("sin resultados"))
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
		lines = append(lines, m.styles.Detail.Render(fmt.Sprintf("  %d de %d", end-start, len(m.filtered))))
	}
	box := m.styles.Box.Width(boxW).Render(strings.Join(lines, "\n"))
	x := max((width-lipgloss.Width(box))/2, 0)
	y := min(2, max(height-lipgloss.Height(box), 0))
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(box).X(x).Y(y).Z(1),
	).Render()
}
