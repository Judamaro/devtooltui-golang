// Package workspace shows the active tool's options, inputs and output.
package workspace

import (
	"context"
	"errors"
	"maps"
	"slices"
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

// Styles are defined by tui/theme.go.
type Styles struct {
	Title, Description, Section, SectionFocused, OutputStale lipgloss.Style
	ErrorText, ErrorMark                                     lipgloss.Style // InputError excerpt
	Textarea                                                 textarea.Styles
	Options                                                  options.Styles
	Render                                                   render.Styles // output colours
}

// Limits defines when the input stops being edited live (ADR 0002).
type Limits struct {
	MaxBytes            int // above: read-only viewport + F5
	MaxLines            int // same; the textarea truncates at 10,000 lines
	LineNumbersMaxBytes int // above this, line numbers are hidden
}

// DefaultLimits are the values approved in ADR 0002.
var DefaultLimits = Limits{MaxBytes: 32 << 10, MaxLines: 5000, LineNumbersMaxBytes: 16 << 10}

// Model is one tool's state. The TUI keeps one per tool, so switching tools
// does not lose what was typed.
type Model struct {
	tool   tool.Tool
	spec   tool.Spec
	opts   options.Model
	inputs []*field
	active int // focused input when focus == FocusInput
	output viewport.Model
	limits Limits
	stale  bool // the input changed and has not run yet (F5)
	wide   bool // the terminal allows two inputs side by side

	// Async runs: seq identifies the current run; results with another seq
	// are dropped.
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

// Focus tells which workspace panel has focus.
type Focus int

const (
	FocusNone Focus = iota
	FocusOptions
	FocusInput
	FocusOutput
)

// DebounceMsg arrives when run Seq's debounce expires.
type DebounceMsg struct {
	ToolID string
	Seq    int
}

// ResultMsg carries the result of run Seq.
type ResultMsg struct {
	ToolID  string
	Seq     int
	Result  tool.Result
	Err     error
	Elapsed time.Duration

	Rendered string // coloured Result.Text; empty when not applicable
	NoColor  bool   // the output exceeded render.MaxBytes
}

// New builds the workspace. debounce is the wait between the last key and
// the run; 0 runs right away (handy in tests).
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

// Init starts the first run (empty input).
func (m *Model) Init() tea.Cmd { return m.runNow() }

// HasInput reports whether the tool has an input field.
func (m *Model) HasInput() bool { return len(m.inputs) > 0 }

// MoveInput moves focus between inputs (delta ±1) and returns false when it
// was already at the edge, so the App moves on to the next panel.
func (m *Model) MoveInput(delta int) (bool, tea.Cmd) {
	next := m.active + delta
	if m.focus != FocusInput || next < 0 || next >= len(m.inputs) {
		return false, nil
	}
	m.active = next
	return true, m.SetFocus(FocusInput)
}

// FocusEdge prepares the input that receives focus when arriving from
// behind (first=true: the first one) or from ahead (the last one).
func (m *Model) FocusEdge(first bool) {
	if first {
		m.active = 0
	} else {
		m.active = max(len(m.inputs)-1, 0)
	}
}

// ActiveInput returns the index of the focused input.
func (m *Model) ActiveInput() int { return m.active }

func (m *Model) split() bool { return len(m.inputs) == 2 && m.wide }

// HasOptions reports whether the tool has options.
func (m *Model) HasOptions() bool { return !m.opts.Empty() }

// CapturesText reports whether focus is on a text field (input or a
// Number/String option), where "q" is typed instead of quitting.
func (m *Model) CapturesText() bool {
	return m.focus == FocusInput || (m.focus == FocusOptions && m.opts.CapturesText())
}

// SetSize splits the height between options, inputs and output. wide tells
// whether two inputs sit side by side (the App decides from the terminal
// width).
func (m *Model) SetSize(w, h int, wide bool) {
	m.width, m.height, m.wide = w, h, wide
	m.resize()
}

func (m *Model) resize() {
	w, h := m.width, m.height
	// title, output separator and options (the statusbar belongs to the App)
	chrome := 2 + len(m.opts.Lines(w))
	switch {
	case m.split():
		chrome++ // one header row for both inputs
	default:
		chrome += len(m.inputs) // one header per input
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

// SetFocus changes the focused panel.
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

// Update receives messages while the workspace has focus.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.focus {
	case FocusOptions:
		var changed bool
		changed, cmd = m.opts.Update(msg)
		m.resize() // the panel may change height while editing a value
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
		// Internal messages (e.g. cursor blink) keep arriving.
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
		return tea.Batch(cmd, m.runNow()) // large paste: once; afterwards, F5
	}
	m.stale = true
	if m.spec.Live && !m.Large() {
		return tea.Batch(cmd, m.schedule())
	}
	return cmd
}

// Large reports whether any input is in read-only mode.
func (m *Model) Large() bool {
	for _, f := range m.inputs {
		if f.large {
			return true
		}
	}
	return false
}

// ClearInput empties the focused input (or the first one) and leaves large
// input mode.
func (m *Model) ClearInput() tea.Cmd {
	if !m.HasInput() {
		return nil
	}
	m.inputs[m.active].clear()
	return m.runNow()
}

// SetInput replaces the content of the first input (or the focused one)
// with text, as if pasted; it honours large input mode.
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

// ActiveValue returns the content of the input SetInput would replace.
func (m *Model) ActiveValue() string {
	if !m.HasInput() {
		return ""
	}
	if m.focus != FocusInput {
		return m.inputs[0].value()
	}
	return m.inputs[m.active].value()
}

// OutputLang is the language of the last output (to suggest an extension).
func (m *Model) OutputLang() string { return m.result.Lang }

// Run runs the tool on demand (F5), without debounce.
func (m *Model) Run() tea.Cmd { return m.runNow() }

// Running reports whether a run is in progress.
func (m *Model) Running() bool { return m.running }

// schedule queues a run after the debounce. Each call invalidates the
// previous ones.
func (m *Model) schedule() tea.Cmd {
	m.seq++
	if m.debounce <= 0 {
		return m.launch(m.seq)
	}
	id, seq := m.spec.ID, m.seq
	return tea.Tick(m.debounce, func(time.Time) tea.Msg { return DebounceMsg{ToolID: id, Seq: seq} })
}

// runNow runs without waiting for the debounce.
func (m *Model) runNow() tea.Cmd {
	m.seq++
	return m.launch(m.seq)
}

// HandleDebounce starts the run if the tick is still current.
func (m *Model) HandleDebounce(msg DebounceMsg) tea.Cmd {
	if msg.Seq != m.seq {
		return nil
	}
	return m.launch(msg.Seq)
}

// launch cancels the previous run and runs the tool in a goroutine with a
// copy of the inputs and options.
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
		// Colouring also runs outside the UI loop.
		if err == nil && ctx.Err() == nil {
			msg.Rendered, msg.NoColor = render.Result(res, kind, req.Inputs, rs)
		}
		return msg
	}
}

// HandleResult applies a result if it belongs to the current run.
func (m *Model) HandleResult(msg ResultMsg) {
	if msg.Seq != m.seq || errors.Is(msg.Err, context.Canceled) {
		return // stale or cancelled result
	}
	m.running = false
	m.cancel = nil
	m.elapsed = msg.Elapsed
	m.err = msg.Err
	if msg.Err != nil {
		return // the previous output stays
	}
	m.result = msg.Result
	m.noColor = msg.NoColor
	if msg.Rendered != "" {
		m.output.SetContent(msg.Rendered)
	} else {
		m.output.SetContent(msg.Result.Text)
	}
}

// Output returns the text of the last successful run.
func (m *Model) Output() string { return m.result.Text }

// Err returns the error of the last run, if any.
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
	b.WriteString(m.section("Output", m.focus == FocusOutput) + "\n")
	out := m.output.View()
	if m.err == nil {
		b.WriteString(out)
		return b.String()
	}
	// The excerpt takes rows from the top of the output, so the layout
	// keeps its height; the previous output stays below, dimmed.
	h := m.output.Height()
	ex := m.errorExcerpt()
	ex = ex[:min(len(ex), h)]
	lines := strings.Split(out, "\n")
	lines = lines[:max(min(len(lines), h-len(ex)), 0)]
	rows := slices.Clone(ex)
	if len(lines) > 0 {
		rows = append(rows, m.styles.OutputStale.Render(strings.Join(lines, "\n")))
	}
	b.WriteString(strings.Join(rows, "\n"))
	return b.String()
}

// Status is the state for the statusbar.
func (m *Model) Status() statusbar.Status {
	st := statusbar.Status{
		Running: m.running,
		// "press f5 to run" only when it will not run by itself: for live
		// tools, during the debounce, there is nothing to ask for.
		Stale:   m.stale && !m.running && (!m.spec.Live || m.Large()),
		Err:     m.err,
		Elapsed: m.elapsed,
		Meta:    m.result.Meta,
	}
	if m.noColor {
		st.Note = "no colour: output over " + statusbar.HumanBytes(render.MaxBytes)
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
