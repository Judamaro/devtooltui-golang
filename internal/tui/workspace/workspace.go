// Package workspace muestra las opciones, la entrada y la salida de la
// herramienta activa.
package workspace

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Styles los define tui/theme.go.
type Styles struct {
	Title, Description, Options, Section, SectionFocused, Error, Meta lipgloss.Style
	Textarea                                                          textarea.Styles
}

// Model es el estado de una herramienta. La TUI guarda uno por herramienta,
// así que al cambiar de herramienta no se pierde lo escrito.
type Model struct {
	tool    tool.Tool
	spec    tool.Spec
	opts    tool.Values
	input   textarea.Model
	output  viewport.Model
	result  tool.Result
	err     error
	elapsed time.Duration
	styles  Styles
	width   int
	focus   Focus
}

// Focus indica qué panel del workspace tiene el foco.
type Focus int

const (
	FocusNone Focus = iota
	FocusInput
	FocusOutput
)

func New(t tool.Tool, styles Styles) *Model {
	spec := t.Spec()
	ta := textarea.New()
	ta.Placeholder = "Escribe o pega aquí…"
	ta.CharLimit = 0
	ta.MaxHeight = 0
	ta.MaxWidth = 0
	ta.SetStyles(styles.Textarea)
	m := &Model{
		tool:   t,
		spec:   spec,
		opts:   tool.Values{}.WithDefaults(spec),
		input:  ta,
		output: viewport.New(),
		styles: styles,
	}
	m.run()
	return m
}

// HasInput indica si la herramienta tiene un campo de entrada.
func (m *Model) HasInput() bool { return len(m.spec.Inputs) > 0 }

// SetSize reparte el alto entre la entrada y la salida.
func (m *Model) SetSize(w, h int) {
	m.width = w
	const chrome = 5 // título, opciones, 2 separadores, línea de estado
	avail := max(h-chrome, 2)
	inH := avail / 2
	if !m.HasInput() {
		inH = 0
	}
	m.input.SetWidth(w)
	m.input.SetHeight(max(inH, 1))
	m.output.SetWidth(w)
	m.output.SetHeight(avail - inH)
}

// SetFocus cambia el panel con foco.
func (m *Model) SetFocus(f Focus) tea.Cmd {
	m.focus = f
	if f == FocusInput && m.HasInput() {
		return m.input.Focus()
	}
	m.input.Blur()
	return nil
}

// Update recibe los mensajes cuando el workspace tiene el foco.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.focus {
	case FocusInput:
		if !m.HasInput() {
			return nil
		}
		before := m.input.Value()
		m.input, cmd = m.input.Update(msg)
		if m.input.Value() != before {
			m.run()
		}
	case FocusOutput:
		m.output, cmd = m.output.Update(msg)
	case FocusNone:
		// Mensajes internos (p. ej. parpadeo del cursor) siguen llegando.
		m.input, cmd = m.input.Update(msg)
	}
	return cmd
}

// run ejecuta la herramienta de forma síncrona. El debounce y la
// cancelación llegan en la fase 1.
func (m *Model) run() {
	inputs := map[string]string{}
	for _, in := range m.spec.Inputs {
		inputs[in.Key] = m.input.Value()
	}
	start := time.Now()
	res, err := m.tool.Run(context.Background(), tool.Request{Inputs: inputs, Opts: m.opts})
	m.elapsed = time.Since(start)
	m.err = err
	if err != nil {
		return // la salida anterior se mantiene
	}
	m.result = res
	m.output.SetContent(res.Text)
}

// Output devuelve el texto de la última ejecución correcta.
func (m *Model) Output() string { return m.result.Text }

// Err devuelve el error de la última ejecución, si lo hubo.
func (m *Model) Err() error { return m.err }

func (m *Model) View() string {
	var b strings.Builder
	title := m.styles.Title.Render(m.spec.Name)
	desc := m.styles.Description.Render(" " + m.spec.Description)
	b.WriteString(clip(title+desc, m.width) + "\n")
	b.WriteString(clip(m.styles.Options.Render(m.optionsText()), m.width) + "\n")
	if m.HasInput() {
		b.WriteString(m.section(m.spec.Inputs[0].Label, m.focus == FocusInput) + "\n")
		b.WriteString(m.input.View() + "\n")
	}
	b.WriteString(m.section("Salida", m.focus == FocusOutput) + "\n")
	b.WriteString(m.output.View() + "\n")
	b.WriteString(clip(m.statusText(), m.width))
	return b.String()
}

func (m *Model) optionsText() string {
	parts := make([]string, 0, len(m.spec.Options))
	for _, o := range m.spec.Options {
		var v string
		switch o.Kind {
		case tool.Toggle:
			v = "[ ]"
			if m.opts.Bool(o.Key) {
				v = "[x]"
			}
		case tool.Number:
			v = fmt.Sprint(m.opts.Int(o.Key))
		default:
			v = m.opts.String(o.Key)
		}
		parts = append(parts, o.Label+": "+v)
	}
	return strings.Join(parts, " · ")
}

func (m *Model) statusText() string {
	if m.err != nil {
		return m.styles.Error.Render("✗ " + m.err.Error())
	}
	parts := []string{fmt.Sprintf("✓ %s", m.elapsed.Round(time.Microsecond))}
	for _, kv := range m.result.Meta {
		parts = append(parts, kv.Key+": "+kv.Value)
	}
	return m.styles.Meta.Render(strings.Join(parts, " · "))
}

func (m *Model) section(label string, focused bool) string {
	st := m.styles.Section
	if focused {
		st = m.styles.SectionFocused
	}
	head := "── " + label + " "
	fill := max(m.width-lipgloss.Width(head), 0)
	return st.Render(head + strings.Repeat("─", fill))
}

func clip(s string, w int) string {
	return lipgloss.NewStyle().MaxWidth(w).Render(s)
}
