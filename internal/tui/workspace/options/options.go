// Package options builds a tool's options panel from Spec.Options: Toggle is
// a checkbox, Select a ◀ ▶ selector and Number or String a validated
// textinput.
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

// Styles are defined by tui/theme.go.
type Styles struct {
	Label, Value, Focused, Invalid lipgloss.Style
	Input                          textinput.Styles
}

// KeyMap holds the panel keys.
type KeyMap struct {
	Prev, Next         key.Binding // previous / next option
	Decrease, Increase key.Binding // previous / next value
	Toggle             key.Binding
}

const (
	separator  = "   "
	inputWidth = 16
)

type item struct {
	opt     tool.Option
	input   *textinput.Model // Number and String only
	invalid bool
}

// Model is the options panel.
type Model struct {
	items   []item
	values  tool.Values
	cursor  int
	focused bool
	styles  Styles
	keys    KeyMap
}

// New builds the panel with the initial values from values.
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

// Empty reports whether the tool has no options.
func (m Model) Empty() bool { return len(m.items) == 0 }

// Values returns the current values.
func (m Model) Values() tool.Values { return m.values }

// CapturesText reports whether the focused option is a text field, in which
// case keys like "q" are typed instead of triggering shortcuts.
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

// Update handles a message and reports whether any value changed.
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

// text is the representation of an option's value.
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

// Lines returns the panel split into lines at most width columns wide.
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
