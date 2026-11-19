// Package workspace muestra las opciones, la entrada y la salida de la
// herramienta activa.
package workspace

import (
	"context"
	"errors"
	"maps"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Judamaro/devtooltui-golang/internal/render"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tui/statusbar"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace/options"
)

// Styles los define tui/theme.go.
type Styles struct {
	Title, Description, Section, SectionFocused, OutputStale lipgloss.Style
	Textarea                                                 textarea.Styles
	Options                                                  options.Styles
	Render                                                   render.Styles // colores de las salidas
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
	inputs []*field
	active int // entrada con foco cuando focus == FocusInput
	output viewport.Model
	limits Limits
	stale  bool // la entrada cambió y no se ha ejecutado (F5)
	wide   bool // la terminal permite dos entradas lado a lado

	// Ejecución asíncrona: seq identifica la ejecución vigente; los
	// resultados con otra seq se descartan.
	debounce time.Duration
	seq      int
	cancel   context.CancelFunc
	running  bool

	result  tool.Result
	noColor bool
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

// DebounceMsg llega cuando vence el debounce de la ejecución Seq.
type DebounceMsg struct {
	ToolID string
	Seq    int
}

// ResultMsg trae el resultado de la ejecución Seq.
type ResultMsg struct {
	ToolID  string
	Seq     int
	Result  tool.Result
	Err     error
	Elapsed time.Duration

	Rendered string // Result.Text coloreado; vacío si no aplica
	NoColor  bool   // la salida superó render.MaxBytes
}

// New crea el workspace. debounce es la espera entre la última tecla y la
// ejecución; con 0 se ejecuta enseguida (útil en tests).
func New(t tool.Tool, styles Styles, optKeys options.KeyMap, debounce time.Duration) *Model {
	spec := t.Spec()
	m := &Model{
		tool:     t,
		spec:     spec,
		opts:     options.New(spec, tool.Values{}.WithDefaults(spec), styles.Options, optKeys),
		output:   viewport.New(),
		limits:   DefaultLimits,
		debounce: debounce,
		styles:   styles,
	}
	for _, in := range spec.Inputs {
		m.inputs = append(m.inputs, newField(in, styles.Textarea))
	}
	return m
}

// Init lanza la primera ejecución (entrada vacía).
func (m *Model) Init() tea.Cmd { return m.runNow() }

// HasInput indica si la herramienta tiene un campo de entrada.
func (m *Model) HasInput() bool { return len(m.inputs) > 0 }

// MoveInput mueve el foco entre entradas (delta ±1) y devuelve false si ya
// estaba en el extremo, para que la App pase al siguiente panel.
func (m *Model) MoveInput(delta int) (bool, tea.Cmd) {
	next := m.active + delta
	if m.focus != FocusInput || next < 0 || next >= len(m.inputs) {
		return false, nil
	}
	m.active = next
	return true, m.SetFocus(FocusInput)
}

// FocusEdge prepara la entrada que recibe el foco al llegar desde atrás
// (first=true: la primera) o desde adelante (la última).
func (m *Model) FocusEdge(first bool) {
	if first {
		m.active = 0
	} else {
		m.active = max(len(m.inputs)-1, 0)
	}
}

// ActiveInput devuelve el índice de la entrada con foco.
func (m *Model) ActiveInput() int { return m.active }

func (m *Model) split() bool { return len(m.inputs) == 2 && m.wide }

// HasOptions indica si la herramienta tiene opciones.
func (m *Model) HasOptions() bool { return !m.opts.Empty() }

// CapturesText indica si el foco está en un campo de texto (entrada u
// opción Number/String), donde "q" se escribe en lugar de salir.
func (m *Model) CapturesText() bool {
	return m.focus == FocusInput || (m.focus == FocusOptions && m.opts.CapturesText())
}

// SetSize reparte el alto entre las opciones, la entrada y la salida.
// wide indica si dos entradas van lado a lado (lo decide la App según el
// ancho de la terminal).
func (m *Model) SetSize(w, h int, wide bool) {
	m.width, m.height, m.wide = w, h, wide
	m.resize()
}

func (m *Model) resize() {
	w, h := m.width, m.height
	// título, separador de salida y opciones (la statusbar es de la App)
	chrome := 2 + len(m.opts.Lines(w))
	switch {
	case m.split():
		chrome++ // una fila de encabezados para las dos entradas
	default:
		chrome += len(m.inputs) // un encabezado por entrada
	}
	avail := max(h-chrome, 2)
	inH := 0
	if m.HasInput() {
		inH = avail / 2
	}
	switch {
	case m.split():
		lw := (w - 1) / 2
		m.inputs[0].setSize(lw, inH)
		m.inputs[1].setSize(w-1-lw, inH)
	case len(m.inputs) > 0:
		each := inH / len(m.inputs)
		for i, f := range m.inputs {
			fh := each
			if i == len(m.inputs)-1 {
				fh = inH - each*(len(m.inputs)-1)
			}
			f.setSize(w, fh)
		}
	}
	m.output.SetWidth(w)
	m.output.SetHeight(avail - inH)
}

// SetFocus cambia el panel con foco.
func (m *Model) SetFocus(f Focus) tea.Cmd {
	m.focus = f
	for _, in := range m.inputs {
		in.ta.Blur()
	}
	m.opts.Blur()
	switch {
	case f == FocusInput && m.HasInput():
		return m.inputs[m.active].ta.Focus()
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
		m.resize() // el panel puede cambiar de alto al editar un valor
		if changed {
			cmd = tea.Batch(cmd, m.runNow())
		}
	case FocusInput:
		if !m.HasInput() {
			return nil
		}
		return m.updateInput(msg)
	case FocusOutput:
		m.output, cmd = m.output.Update(msg)
	case FocusNone:
		// Mensajes internos (p. ej. parpadeo del cursor) siguen llegando.
		if m.HasInput() {
			m.inputs[m.active].ta, cmd = m.inputs[m.active].ta.Update(msg)
		}
	}
	return cmd
}

func (m *Model) updateInput(msg tea.Msg) tea.Cmd {
	changed, now, cmd := m.inputs[m.active].update(msg, m.limits)
	switch {
	case !changed:
		return cmd
	case now:
		return tea.Batch(cmd, m.runNow()) // pegado grande: una vez; después, F5
	}
	m.stale = true
	if m.spec.Live && !m.Large() {
		return tea.Batch(cmd, m.schedule())
	}
	return cmd
}

// Large indica si alguna entrada está en modo de solo lectura.
func (m *Model) Large() bool {
	for _, f := range m.inputs {
		if f.large {
			return true
		}
	}
	return false
}

// ClearInput vacía la entrada con foco (o la primera) y sale del modo de
// entrada grande.
func (m *Model) ClearInput() tea.Cmd {
	if !m.HasInput() {
		return nil
	}
	m.inputs[m.active].clear()
	return m.runNow()
}

// SetInput reemplaza el contenido de la primera entrada (o de la que tenga
// el foco) por text, como si se pegara; respeta el modo de entrada grande.
func (m *Model) SetInput(text string) tea.Cmd {
	if !m.HasInput() {
		return nil
	}
	if m.focus != FocusInput {
		m.active = 0
	}
	m.inputs[m.active].setValue(text, m.limits)
	return m.runNow()
}

// ActiveValue devuelve el contenido de la entrada que recibirá SetInput.
func (m *Model) ActiveValue() string {
	if !m.HasInput() {
		return ""
	}
	if m.focus != FocusInput {
		return m.inputs[0].value()
	}
	return m.inputs[m.active].value()
}

// OutputLang es el lenguaje de la última salida (para sugerir extensión).
func (m *Model) OutputLang() string { return m.result.Lang }

// Run ejecuta la herramienta a pedido (F5), sin debounce.
func (m *Model) Run() tea.Cmd { return m.runNow() }

// Running indica si hay una ejecución en curso.
func (m *Model) Running() bool { return m.running }

// schedule programa una ejecución tras el debounce. Cada llamada invalida
// las anteriores.
func (m *Model) schedule() tea.Cmd {
	m.seq++
	if m.debounce <= 0 {
		return m.launch(m.seq)
	}
	id, seq := m.spec.ID, m.seq
	return tea.Tick(m.debounce, func(time.Time) tea.Msg { return DebounceMsg{ToolID: id, Seq: seq} })
}

// runNow ejecuta sin esperar el debounce.
func (m *Model) runNow() tea.Cmd {
	m.seq++
	return m.launch(m.seq)
}

// HandleDebounce lanza la ejecución si el tick sigue vigente.
func (m *Model) HandleDebounce(msg DebounceMsg) tea.Cmd {
	if msg.Seq != m.seq {
		return nil
	}
	return m.launch(msg.Seq)
}

// launch cancela la ejecución anterior y corre la herramienta en una
// goroutine con una copia de la entrada y las opciones.
func (m *Model) launch(seq int) tea.Cmd {
	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.running = true
	m.stale = false

	inputs := make(map[string]string, len(m.inputs))
	for _, f := range m.inputs {
		inputs[f.spec.Key] = f.value()
	}
	req := tool.Request{Inputs: inputs, Opts: maps.Clone(m.opts.Values())}
	t, id, kind, rs := m.tool, m.spec.ID, m.spec.Output, m.styles.Render
	return func() tea.Msg {
		start := time.Now()
		res, err := t.Run(ctx, req)
		msg := ResultMsg{ToolID: id, Seq: seq, Result: res, Err: err, Elapsed: time.Since(start)}
		// El coloreado también corre fuera del bucle de la UI.
		if err == nil && ctx.Err() == nil {
			msg.Rendered, msg.NoColor = render.Result(res, kind, req.Inputs, rs)
		}
		return msg
	}
}

// HandleResult aplica un resultado si corresponde a la ejecución vigente.
func (m *Model) HandleResult(msg ResultMsg) {
	if msg.Seq != m.seq || errors.Is(msg.Err, context.Canceled) {
		return // resultado viejo o cancelado
	}
	m.running = false
	m.cancel = nil
	m.elapsed = msg.Elapsed
	m.err = msg.Err
	if msg.Err != nil {
		return // la salida anterior se mantiene
	}
	m.result = msg.Result
	m.noColor = msg.NoColor
	if msg.Rendered != "" {
		m.output.SetContent(msg.Rendered)
	} else {
		m.output.SetContent(msg.Result.Text)
	}
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
	if m.split() {
		lw := (m.width - 1) / 2
		heads := m.sectionW(m.inputs[0].spec.Label, m.inputFocused(0), lw) + " " +
			m.sectionW(m.inputs[1].spec.Label, m.inputFocused(1), m.width-1-lw)
		b.WriteString(heads + "\n")
		left := lipgloss.NewStyle().Width(lw).MaxWidth(lw).Render(m.inputs[0].view())
		sep := m.styles.Section.Render(strings.TrimSuffix(strings.Repeat("│\n", lipgloss.Height(left)), "\n"))
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, sep, m.inputs[1].view()) + "\n")
	} else {
		for i, f := range m.inputs {
			b.WriteString(m.section(f.spec.Label, m.inputFocused(i)) + "\n")
			b.WriteString(f.view() + "\n")
		}
	}
	b.WriteString(m.section("Salida", m.focus == FocusOutput) + "\n")
	out := m.output.View()
	if m.err != nil {
		out = m.styles.OutputStale.Render(out) // salida anterior, atenuada
	}
	b.WriteString(out)
	return b.String()
}

// Status es el estado para la statusbar.
func (m *Model) Status() statusbar.Status {
	st := statusbar.Status{
		Running: m.running,
		// "f5 para ejecutar" solo si no se va a ejecutar sola: en las
		// herramientas live, durante el debounce, no hay nada que pedir.
		Stale:   m.stale && !m.running && (!m.spec.Live || m.Large()),
		Err:     m.err,
		Elapsed: m.elapsed,
		Meta:    m.result.Meta,
	}
	if m.noColor {
		st.Note = "sin color: salida de más de " + statusbar.HumanBytes(render.MaxBytes)
	}
	for _, f := range m.inputs {
		if f.large {
			st.LargeBytes += len(f.largeText)
			st.LargeLines += strings.Count(f.largeText, "\n") + 1
		}
	}
	return st
}

func (m *Model) inputFocused(i int) bool { return m.focus == FocusInput && m.active == i }

func (m *Model) section(label string, focused bool) string {
	return m.sectionW(label, focused, m.width)
}

func (m *Model) sectionW(label string, focused bool, width int) string {
	st := m.styles.Section
	if focused {
		st = m.styles.SectionFocused
	}
	head := "── " + label + " "
	if w := lipgloss.Width(head); w > width {
		return st.Render(lipgloss.NewStyle().MaxWidth(width).Render(head))
	}
	fill := max(width-lipgloss.Width(head), 0)
	return st.Render(head + strings.Repeat("─", fill))
}

func clip(s string, w int) string {
	return lipgloss.NewStyle().MaxWidth(w).Render(s)
}
