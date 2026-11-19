// Package options genera el panel de opciones de una herramienta a partir
// de Spec.Options: Toggle es un checkbox, Select un selector ◀ ▶ y Number o
// String un textinput validado.
package options

import (
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Styles los define tui/theme.go.
type Styles struct {
	Label, Value, Focused, Invalid lipgloss.Style
	Input                          textinput.Styles
}

// KeyMap son las teclas del panel.
type KeyMap struct {
	Prev, Next         key.Binding // opción anterior / siguiente
	Decrease, Increase key.Binding // valor anterior / siguiente
	Toggle             key.Binding
}

const (
	separator  = "   "
	inputWidth = 16
)

type item struct {
	opt     tool.Option
	input   *textinput.Model // solo Number y String
	invalid bool
}

// Model es el panel de opciones.
type Model struct {
	items   []item
	values  tool.Values
	cursor  int
	focused bool
	styles  Styles
	keys    KeyMap
}

// New crea el panel con los valores iniciales de values.
func New(spec tool.Spec, values tool.Values, styles Styles, keys KeyMap) Model {
	m := Model{values: values, styles: styles, keys: keys}
	for _, o := range spec.Options {
		it := item{opt: o}
		if o.Kind == tool.Number || o.Kind == tool.String {
			ti := textinput.New()
			ti.Prompt = ""
			ti.SetWidth(inputWidth)
			ti.SetStyles(styles.Input)
			ti.SetValue(m.text(o))
			it.input = &ti
		}
		m.items = append(m.items, it)
	}
	return m
}

// Empty indica si la herramienta no tiene opciones.
func (m Model) Empty() bool { return len(m.items) == 0 }

// Values devuelve los valores actuales.
func (m Model) Values() tool.Values { return m.values }

// CapturesText indica si la opción enfocada es un campo de texto, en cuyo
// caso teclas como "q" se escriben en lugar de ejecutar atajos.
func (m Model) CapturesText() bool {
	return m.focused && len(m.items) > 0 && m.items[m.cursor].input != nil
}

func (m *Model) Focus() tea.Cmd {
	m.focused = true
	return m.focusCurrent()
}

func (m *Model) Blur() {
	m.focused = false
	for _, it := range m.items {
		if it.input != nil {
			it.input.Blur()
		}
	}
}

func (m *Model) focusCurrent() tea.Cmd {
	var cmd tea.Cmd
	for i, it := range m.items {
		if it.input == nil {
			continue
		}
		if i == m.cursor && m.focused {
			cmd = it.input.Focus()
			it.input.CursorEnd()
		} else {
			it.input.Blur()
		}
	}
	return cmd
}

// Update procesa un mensaje y devuelve si cambió algún valor.
func (m *Model) Update(msg tea.Msg) (changed bool, cmd tea.Cmd) {
	if len(m.items) == 0 {
		return false, nil
	}
	it := &m.items[m.cursor]
	k, isKey := msg.(tea.KeyPressMsg)
	if !isKey {
		if it.input != nil {
			*it.input, cmd = it.input.Update(msg)
		}
		return false, cmd
	}
	switch {
	case key.Matches(k, m.keys.Prev):
		m.cursor = (m.cursor - 1 + len(m.items)) % len(m.items)
		return false, m.focusCurrent()
	case key.Matches(k, m.keys.Next):
		m.cursor = (m.cursor + 1) % len(m.items)
		return false, m.focusCurrent()
	}

	o := it.opt
	switch o.Kind {
	case tool.Toggle:
		if key.Matches(k, m.keys.Toggle, m.keys.Decrease, m.keys.Increase) {
			m.values[o.Key] = !m.values.Bool(o.Key)
			return true, nil
		}
	case tool.Select:
		if d := m.delta(k); d != 0 {
			i := slices.Index(o.Choices, m.values.String(o.Key))
			m.values[o.Key] = o.Choices[(i+d+len(o.Choices))%len(o.Choices)]
			return true, nil
		}
	case tool.Number:
		if d := m.delta(k); d != 0 {
			n := min(max(m.values.Int(o.Key)+d, o.Min), o.Max)
			it.input.SetValue(strconv.Itoa(n))
			it.invalid = false
			changed = n != m.values.Int(o.Key)
			m.values[o.Key] = n
			return changed, nil
		}
		*it.input, cmd = it.input.Update(msg)
		n, err := strconv.Atoi(strings.TrimSpace(it.input.Value()))
		it.invalid = err != nil || n < o.Min || n > o.Max
		if !it.invalid && n != m.values.Int(o.Key) {
			m.values[o.Key] = n
			return true, cmd
		}
		return false, cmd
	case tool.String:
		before := it.input.Value()
		*it.input, cmd = it.input.Update(msg)
		if v := it.input.Value(); v != before {
			m.values[o.Key] = v
			return true, cmd
		}
		return false, cmd
	}
	return false, nil
}

func (m Model) delta(k tea.KeyPressMsg) int {
	switch {
	case key.Matches(k, m.keys.Decrease):
		return -1
	case key.Matches(k, m.keys.Increase):
		return 1
	}
	return 0
}

// text es la representación del valor de una opción.
func (m Model) text(o tool.Option) string {
	switch o.Kind {
	case tool.Toggle:
		if m.values.Bool(o.Key) {
			return "[x]"
		}
		return "[ ]"
	case tool.Number:
		return strconv.Itoa(m.values.Int(o.Key))
	default:
		return m.values.String(o.Key)
	}
}

// Lines devuelve el panel repartido en líneas de como máximo width columnas.
func (m Model) Lines(width int) []string {
	var lines []string
	cur := ""
	for i, it := range m.items {
		cell := m.renderItem(i, it)
		switch {
		case cur == "":
			cur = cell
		case lipgloss.Width(cur)+len(separator)+lipgloss.Width(cell) <= width:
			cur += separator + cell
		default:
			lines = append(lines, cur)
			cur = cell
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func (m Model) renderItem(i int, it item) string {
	focused := m.focused && i == m.cursor
	label := m.styles.Label.Render(it.opt.Label + ": ")
	var val string
	switch {
	case it.input != nil && focused:
		val = it.input.View()
	case it.opt.Kind == tool.Select:
		val = "◀ " + m.text(it.opt) + " ▶"
	default:
		val = m.text(it.opt)
	}
	st := m.styles.Value
	switch {
	case it.invalid:
		st = m.styles.Invalid
	case focused:
		st = m.styles.Focused
	}
	return label + st.Render(val)
}
