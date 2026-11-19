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
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace/options"
)

// Styles los define tui/theme.go.
type Styles struct {
	Title, Description, Section, SectionFocused, Error, Warning, Meta lipgloss.Style
	Textarea                                                          textarea.Styles
	Options                                                           options.Styles
}

// Limits define cuándo la entrada deja de editarse en vivo (ADR 0002).
type Limits struct {
	MaxBytes            int // por encima: viewport de solo lectura + F5
	MaxLines            int // ídem; el textarea trunca a 10 000 líneas
	LineNumbersMaxBytes int // por encima se ocultan los números de línea
}

// DefaultLimits son los valores aprobados en la ADR 0002.
var DefaultLimits = Limits{MaxBytes: 32 << 10, MaxLines: 5000, LineNumbersMaxBytes: 16 << 10}

// Model es el estado de una herramienta. La TUI guarda uno por herramienta,
// así que al cambiar de herramienta no se pierde lo escrito.
type Model struct {
	tool   tool.Tool
	spec   tool.Spec
	opts   options.Model
	input  textarea.Model
	output viewport.Model
	limits Limits

	// Modo de entrada grande: el texto vive en large y se muestra en
	// largeView, de solo lectura.
	large     bool
	largeText string
	largeView viewport.Model
	stale     bool // la entrada cambió y no se ha ejecutado (F5)

	result  tool.Result
	err     error
	elapsed time.Duration
	styles  Styles
	width   int
	height  int
	focus   Focus
}

// Focus indica qué panel del workspace tiene el foco.
type Focus int

const (
	FocusNone Focus = iota
	FocusOptions
	FocusInput
	FocusOutput
)

func New(t tool.Tool, styles Styles, optKeys options.KeyMap) *Model {
	spec := t.Spec()
	ta := textarea.New()
	ta.Placeholder = "Escribe o pega aquí…"
	ta.CharLimit = 0
	ta.MaxHeight = 0
	ta.MaxWidth = 0
	ta.SetStyles(styles.Textarea)
	m := &Model{
		tool:      t,
		spec:      spec,
		opts:      options.New(spec, tool.Values{}.WithDefaults(spec), styles.Options, optKeys),
		input:     ta,
		output:    viewport.New(),
		largeView: viewport.New(),
		limits:    DefaultLimits,
		styles:    styles,
	}
	m.run()
	return m
}

// HasInput indica si la herramienta tiene un campo de entrada.
func (m *Model) HasInput() bool { return len(m.spec.Inputs) > 0 }

// HasOptions indica si la herramienta tiene opciones.
func (m *Model) HasOptions() bool { return !m.opts.Empty() }

// CapturesText indica si el foco está en un campo de texto (entrada u
// opción Number/String), donde "q" se escribe en lugar de salir.
func (m *Model) CapturesText() bool {
	return m.focus == FocusInput || (m.focus == FocusOptions && m.opts.CapturesText())
}

// SetSize reparte el alto entre las opciones, la entrada y la salida.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	m.resize()
}

func (m *Model) resize() {
	w, h := m.width, m.height
	// título, separadores de entrada y salida, línea de estado y opciones
	chrome := 3 + len(m.opts.Lines(w))
	if m.HasInput() {
		chrome++
	}
	avail := max(h-chrome, 2)
	inH := avail / 2
	if !m.HasInput() {
		inH = 0
	}
	m.input.SetWidth(w)
	m.input.SetHeight(max(inH, 1))
	m.largeView.SetWidth(w)
	m.largeView.SetHeight(max(inH, 1))
	m.output.SetWidth(w)
	m.output.SetHeight(avail - inH)
}

// SetFocus cambia el panel con foco.
func (m *Model) SetFocus(f Focus) tea.Cmd {
	m.focus = f
	m.input.Blur()
	m.opts.Blur()
	switch {
	case f == FocusInput && m.HasInput():
		return m.input.Focus()
	case f == FocusOptions:
		return m.opts.Focus()
	}
	return nil
}

// Update recibe los mensajes cuando el workspace tiene el foco.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.focus {
	case FocusOptions:
		var changed bool
		changed, cmd = m.opts.Update(msg)
		if changed {
			m.run()
		}
		m.resize() // el panel puede cambiar de alto al editar un valor
	case FocusInput:
		if !m.HasInput() {
			return nil
		}
		return m.updateInput(msg)
	case FocusOutput:
		m.output, cmd = m.output.Update(msg)
	case FocusNone:
		// Mensajes internos (p. ej. parpadeo del cursor) siguen llegando.
		m.input, cmd = m.input.Update(msg)
	}
	return cmd
}

func (m *Model) updateInput(msg tea.Msg) tea.Cmd {
	if m.large {
		if p, ok := msg.(tea.PasteMsg); ok {
			m.setLarge(m.largeText + p.Content)
			m.run()
			return nil
		}
		var cmd tea.Cmd
		m.largeView, cmd = m.largeView.Update(msg)
		return cmd
	}
	before := m.input.Value()
	if p, ok := msg.(tea.PasteMsg); ok && m.exceeds(before+p.Content) {
		// Se agrega al final: el textarea truncaría o se congelaría.
		m.setLarge(before + p.Content)
		m.run() // una vez al pegar; después, solo con F5
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	after := m.input.Value()
	if after == before {
		return cmd
	}
	if m.exceeds(after) {
		m.setLarge(after)
		m.run()
		return cmd
	}
	m.input.ShowLineNumbers = len(after) <= m.limits.LineNumbersMaxBytes
	m.stale = true
	if m.spec.Live {
		m.run()
	}
	return cmd
}

func (m *Model) exceeds(s string) bool {
	return len(s) > m.limits.MaxBytes || strings.Count(s, "\n")+1 > m.limits.MaxLines
}

func (m *Model) setLarge(s string) {
	m.large = true
	m.largeText = s
	m.largeView.SetContent(s)
	m.input.Reset()
}

// Large indica si la entrada está en modo de solo lectura.
func (m *Model) Large() bool { return m.large }

// ClearInput vacía la entrada y sale del modo de entrada grande.
func (m *Model) ClearInput() {
	m.large = false
	m.largeText = ""
	m.largeView.SetContent("")
	m.input.Reset()
	m.input.ShowLineNumbers = true
	m.run()
}

// Run ejecuta la herramienta a pedido (F5).
func (m *Model) Run() { m.run() }

// run ejecuta la herramienta de forma síncrona. El debounce y la
// cancelación llegan en la fase 1.
func (m *Model) run() {
	inputs := map[string]string{}
	value := m.input.Value()
	if m.large {
		value = m.largeText
	}
	for _, in := range m.spec.Inputs {
		inputs[in.Key] = value
	}
	m.stale = false
	start := time.Now()
	res, err := m.tool.Run(context.Background(), tool.Request{Inputs: inputs, Opts: m.opts.Values()})
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
	for _, line := range m.opts.Lines(m.width) {
		b.WriteString(clip(line, m.width) + "\n")
	}
	if m.HasInput() {
		b.WriteString(m.section(m.spec.Inputs[0].Label, m.focus == FocusInput) + "\n")
		if m.large {
			b.WriteString(m.largeView.View() + "\n")
		} else {
			b.WriteString(m.input.View() + "\n")
		}
	}
	b.WriteString(m.section("Salida", m.focus == FocusOutput) + "\n")
	b.WriteString(m.output.View() + "\n")
	b.WriteString(clip(m.statusText(), m.width))
	return b.String()
}

func (m *Model) statusText() string {
	prefix := ""
	if m.large {
		prefix = m.styles.Warning.Render(fmt.Sprintf("⚠ entrada grande (%s, %d líneas): solo lectura · f5 ejecutar · ctrl+l limpiar",
			HumanBytes(len(m.largeText)), strings.Count(m.largeText, "\n")+1)) + " · "
	}
	if m.err != nil {
		return prefix + m.styles.Error.Render("✗ "+m.err.Error())
	}
	if m.stale {
		return prefix + m.styles.Warning.Render("● f5 para ejecutar")
	}
	parts := []string{fmt.Sprintf("✓ %s", m.elapsed.Round(time.Microsecond))}
	for _, kv := range m.result.Meta {
		parts = append(parts, kv.Key+": "+kv.Value)
	}
	return prefix + m.styles.Meta.Render(strings.Join(parts, " · "))
}

// HumanBytes formatea un tamaño en B, KB o MB.
func HumanBytes(n int) string {
	switch {
	case n < 1<<10:
		return fmt.Sprintf("%d B", n)
	case n < 1<<20:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
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
