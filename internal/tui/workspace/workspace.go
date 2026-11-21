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
	"github.com/Judamaro/devtooltui-golang/internal/tui/frame"
	"github.com/Judamaro/devtooltui-golang/internal/tui/statusbar"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace/options"
)

// Styles are defined by tui/theme.go.
type Styles struct {
	Title, Description, OutputStale lipgloss.Style
	Gutter                          lipgloss.Style // line numbers of the error excerpt
	Frame                           frame.Styles   // the boxes of each section
	ErrorText, ErrorMark            lipgloss.Style // InputError excerpt
	Textarea                        textarea.Styles
	Options                         options.Styles
	Render                          render.Styles // output colours
}

// Limits defines when the input stops being edited live (ADR 0002).
type Limits struct {
	MaxBytes            int // above: read-only viewport + F5
	MaxLines            int // same; the textarea truncates at 10,000 lines
	LineNumbersMaxBytes int // above this, line numbers are hidden
}

// DefaultLimits are the values approved in ADR 0002.
var DefaultLimits = Limits{MaxBytes: 32 << 10, MaxLines: 5000, LineNumbersMaxBytes: 16 << 10}

// LimitsFor derives the limits from live_max_bytes: line numbers hide at
// half of it, at most 16 KB, as in ADR 0002.
func LimitsFor(maxBytes int) Limits {
	l := DefaultLimits
	if maxBytes > 0 {
		l.MaxBytes = maxBytes
		l.LineNumbersMaxBytes = min(DefaultLimits.LineNumbersMaxBytes, maxBytes/2)
	}
	return l
}

// Model is one tool's state. The TUI keeps one per tool, so switching tools
// does not lose what was typed.
type Model struct {
	tool    tool.Tool
	spec    tool.Spec
	opts    options.Model
	optKeys options.KeyMap
	inputs  []*field
	active  int // focused input when focus == FocusInput
	output  viewport.Model
	limits  Limits
	stale   bool // the input changed and has not run yet (F5)
	wide    bool // the terminal allows two inputs side by side

	// Async runs: seq identifies the current run; results with another seq
	// are dropped.
	debounce time.Duration
	seq      int
	cancel   context.CancelFunc
	running  bool

	// Box heights (borders included) from the last resize.
	headH, outH int
	inH         []int

	result      tool.Result
	noColor     bool
	renderWidth int // width the last run was rendered for (Markdown wraps)
	err         error
	elapsed     time.Duration
	styles      Styles
	width       int
	height      int
	focus       Focus
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
		optKeys:  optKeys,
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

// SetOptions replaces the option values (config defaults, history); keys
// that are missing take the Spec defaults. Call it before Init.
func (m *Model) SetOptions(v tool.Values) {
	m.opts = options.New(m.spec, v.WithDefaults(m.spec), m.styles.Options, m.optKeys)
}

// Rewrap reruns the tool when its output wraps to the width (Markdown) and
// the width changed since it was rendered.
func (m *Model) Rewrap() tea.Cmd {
	if m.result.Lang != "markdown" || m.renderWidth == m.innerWidth() {
		return nil
	}
	return m.runNow()
}

// Values returns a copy of the current option values.
func (m *Model) Values() tool.Values { return maps.Clone(m.opts.Values()) }

// Inputs returns the current inputs by key.
func (m *Model) Inputs() map[string]string { return m.request().Inputs }

// SetInputs loads saved inputs (history), honouring large input mode. Call
// it before Init.
func (m *Model) SetInputs(in map[string]string) {
	for _, f := range m.inputs {
		if v, ok := in[f.spec.Key]; ok {
			f.setValue(v, m.limits)
		}
	}
}

// SetStyles changes the styles (theme switch) and reruns the tool, since
// the output was coloured with the old ones.
func (m *Model) SetStyles(styles Styles) tea.Cmd {
	m.styles = styles
	m.opts.SetStyles(styles.Options)
	for _, f := range m.inputs {
		f.ta.SetStyles(styles.Textarea)
	}
	return m.runNow()
}

// SetLimits changes the large input thresholds (live_max_bytes).
func (m *Model) SetLimits(l Limits) { m.limits = l }

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

// resize splits the height between the boxes: the tool (title and
// options), the inputs and the output. Each box takes 2 rows and 2 columns
// of border; the inputs get half of what is left, the output the rest.
func (m *Model) resize() {
	w, h := m.width, m.height
	inner := max(w-2, 1)
	m.headH = max(len(m.opts.Lines(inner)), 1) + 2
	inputBorders := 2 * len(m.inputs)
	if m.split() {
		inputBorders = 2
	}
	avail := max(h-m.headH-inputBorders-2, 1+len(m.inputs))
	inBody := 0
	if m.HasInput() {
		inBody = avail / 2
	}
	m.outH = avail - inBody + 2
	m.inH = m.inH[:0]
	switch {
	case m.split():
		lw := w / 2
		m.inputs[0].setSize(max(lw-2, 1), inBody)
		m.inputs[1].setSize(max(w-lw-2, 1), inBody)
		m.inH = append(m.inH, inBody+2)
	case len(m.inputs) > 0:
		each := inBody / len(m.inputs)
		for i, f := range m.inputs {
			fh := max(each, 1)
			if i == len(m.inputs)-1 {
				fh = max(inBody-each*(len(m.inputs)-1), 1)
			}
			f.setSize(inner, fh)
			m.inH = append(m.inH, fh+2)
		}
	}
	m.output.SetWidth(inner)
	m.output.SetHeight(m.outH - 2)
}

// Click handles a click at (x, y) inside the workspace: the box under it
// takes the focus, which Click returns with ok, and the click acts there
// (an option, the cursor of an input). Clicks on the tool box of a tool
// without options, or outside the boxes, change nothing.
func (m *Model) Click(x, y int) (f Focus, ok bool, cmd tea.Cmd) {
	inner := m.innerWidth()
	in := func(top, h int) (int, int, bool) { // interior row and column of a box
		r, c := y-top-1, x-1
		return r, c, r >= 0 && r < h-2 && c >= 0 && c < inner
	}
	if y < m.headH {
		if !m.HasOptions() {
			return 0, false, nil
		}
		if r, c, inside := in(0, m.headH); inside {
			changed, ocmd := m.opts.Click(r, c, inner)
			cmd = ocmd
			if changed {
				m.resize() // a value may change the panel's height
				cmd = tea.Batch(cmd, m.runNow())
			}
		}
		return FocusOptions, true, cmd
	}
	top := m.headH
	switch {
	case m.split() && y < top+m.inH[0]:
		i, left := 0, 0
		if lw := m.width / 2; x >= lw {
			i, left = 1, lw
		}
		m.active = i
		if r, c := y-top-1, x-left-1; r >= 0 && c >= 0 {
			m.inputs[i].clickAt(r, c)
		}
		return FocusInput, true, nil
	case !m.split():
		for i, h := range m.inH {
			if y < top+h {
				m.active = i
				if r, c, inside := in(top, h); inside {
					m.inputs[i].clickAt(r, c)
				}
				return FocusInput, true, nil
			}
			top += h
		}
	}
	return FocusOutput, true, nil // the rest is the Output box
}

// Wheel scrolls the box under (x, y): the output viewport or an input.
func (m *Model) Wheel(x, y int, msg tea.MouseWheelMsg) tea.Cmd {
	if y < m.headH {
		return nil
	}
	top := m.headH
	if m.split() && y < top+m.inH[0] {
		i := 0
		if x >= m.width/2 {
			i = 1
		}
		return m.inputs[i].wheel(msg)
	}
	if !m.split() {
		for i, h := range m.inH {
			if y < top+h {
				return m.inputs[i].wheel(msg)
			}
			top += h
		}
	}
	var cmd tea.Cmd
	m.output, cmd = m.output.Update(msg)
	return cmd
}

// innerWidth is the width inside the boxes.
func (m *Model) innerWidth() int { return max(m.width-2, 1) }

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

	req := m.request()
	t, id, kind, rs, width := m.tool, m.spec.ID, m.spec.Output, m.styles.Render, m.innerWidth()
	m.renderWidth = width
	return func() tea.Msg {
		start := time.Now()
		res, err := t.Run(ctx, req)
		msg := ResultMsg{ToolID: id, Seq: seq, Result: res, Err: err, Elapsed: time.Since(start)}
		// Colouring also runs outside the UI loop.
		if err == nil && ctx.Err() == nil {
			msg.Rendered, msg.NoColor = render.Result(res, kind, req.Inputs, width, rs)
		}
		return msg
	}
}

// request copies the current inputs and options.
func (m *Model) request() tool.Request {
	inputs := make(map[string]string, len(m.inputs))
	for _, f := range m.inputs {
		inputs[f.spec.Key] = f.value()
	}
	return tool.Request{Inputs: inputs, Opts: maps.Clone(m.opts.Values())}
}

// Export runs the tool's Exporter on the current inputs and options. ok is
// false when the tool has none. It runs in the caller's goroutine: exports
// are small (a QR PNG takes milliseconds).
func (m *Model) Export(ctx context.Context) (data []byte, ext string, ok bool, err error) {
	e, ok := m.tool.(tool.Exporter)
	if !ok {
		return nil, "", false, nil
	}
	data, ext, err = e.Export(ctx, m.request())
	return data, ext, true, err
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
	w, fs := m.width, m.styles.Frame
	title := m.styles.Title.Render(m.spec.Name) + m.styles.Description.Render(" · "+m.spec.Description)
	head := strings.Join(m.opts.Lines(m.innerWidth()), "\n")
	if m.opts.Empty() {
		head = m.styles.Description.Render(m.spec.Description)
	}
	parts := []string{frame.Box(title, head, w, m.headH, m.focus == FocusOptions, fs)}
	switch {
	case m.split():
		lw := w / 2
		left := frame.Box(m.inputs[0].spec.Label, m.inputs[0].view(), lw, m.inH[0], m.inputFocused(0), fs)
		right := frame.Box(m.inputs[1].spec.Label, m.inputs[1].view(), w-lw, m.inH[0], m.inputFocused(1), fs)
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, left, right))
	default:
		for i, f := range m.inputs {
			parts = append(parts, frame.Box(f.spec.Label, f.view(), w, m.inH[i], m.inputFocused(i), fs))
		}
	}
	parts = append(parts, frame.Box("Output", m.outputBody(), w, m.outH, m.focus == FocusOutput, fs))
	lines := strings.Split(strings.Join(parts, "\n"), "\n")
	return strings.Join(lines[:min(len(lines), max(m.height, 1))], "\n")
}

// outputBody is the output viewport or, after an error, the excerpt above
// the previous output, dimmed. The excerpt takes rows from the top, so the
// box keeps its height.
func (m *Model) outputBody() string {
	out := m.output.View()
	if m.err == nil {
		return out
	}
	h := m.output.Height()
	ex := m.errorExcerpt()
	ex = ex[:min(len(ex), h)]
	lines := strings.Split(out, "\n")
	lines = lines[:max(min(len(lines), h-len(ex)), 0)]
	rows := slices.Clone(ex)
	if len(lines) > 0 {
		rows = append(rows, m.styles.OutputStale.Render(strings.Join(lines, "\n")))
	}
	return strings.Join(rows, "\n")
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

func clip(s string, w int) string {
	return lipgloss.NewStyle().MaxWidth(w).Render(s)
}
