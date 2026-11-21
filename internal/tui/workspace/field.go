package workspace

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// field is one tool input: an editable textarea or, in large input mode
// (ADR 0002), a read-only viewport.
type field struct {
	spec      tool.InputSpec
	ta        textarea.Model
	large     bool
	largeText string
	largeView viewport.Model
}

func newField(spec tool.InputSpec, styles textarea.Styles) *field {
	ta := textarea.New()
	ta.Placeholder = "Type or paste here…"
	ta.Prompt = "" // the box border shows the focus
	ta.CharLimit = 0
	ta.MaxHeight = 0
	ta.MaxWidth = 0
	ta.SetStyles(styles)
	return &field{spec: spec, ta: ta, largeView: viewport.New()}
}

func (f *field) value() string {
	if f.large {
		return f.largeText
	}
	return f.ta.Value()
}

func (f *field) setSize(w, h int) {
	f.ta.SetWidth(w)
	f.ta.SetHeight(max(h, 1))
	f.largeView.SetWidth(w)
	f.largeView.SetHeight(max(h, 1))
}

func (f *field) setLarge(s string) {
	f.large = true
	f.largeText = s
	f.largeView.SetContent(s)
	f.ta.Reset()
}

// setValue replaces the content without going through the textarea (which
// ignores messages when unfocused).
func (f *field) setValue(s string, lim Limits) {
	f.clear()
	if exceeds(s, lim) {
		f.setLarge(s)
		return
	}
	f.ta.SetValue(s)
	f.ta.ShowLineNumbers = len(s) <= lim.LineNumbersMaxBytes
}

func (f *field) clear() {
	f.large = false
	f.largeText = ""
	f.largeView.SetContent("")
	f.ta.Reset()
	f.ta.ShowLineNumbers = true
}

func (f *field) view() string {
	if f.large {
		return f.largeView.View()
	}
	return f.ta.View()
}

// update handles a message and reports whether the value changed and whether
// it must run right away (a paste that switches to large mode).
func (f *field) update(msg tea.Msg, lim Limits) (changed, now bool, cmd tea.Cmd) {
	if f.large {
		if p, ok := msg.(tea.PasteMsg); ok {
			f.setLarge(f.largeText + p.Content)
			return true, true, nil
		}
		f.largeView, cmd = f.largeView.Update(msg)
		return false, false, cmd
	}
	before := f.ta.Value()
	if p, ok := msg.(tea.PasteMsg); ok && exceeds(before+p.Content, lim) {
		// Appended at the end: the textarea would truncate or freeze.
		f.setLarge(before + p.Content)
		return true, true, nil
	}
	f.ta, cmd = f.ta.Update(msg)
	after := f.ta.Value()
	if after == before {
		return false, false, cmd
	}
	if exceeds(after, lim) {
		f.setLarge(after)
		return true, true, cmd
	}
	f.ta.ShowLineNumbers = len(after) <= lim.LineNumbersMaxBytes
	return true, false, cmd
}

func exceeds(s string, lim Limits) bool {
	return len(s) > lim.MaxBytes || strings.Count(s, "\n")+1 > lim.MaxLines
}

// clickAt puts the cursor where the input was clicked: row and col are
// inside the box. The textarea has no mouse support, so it walks there:
// back to the start, down row visual rows (CursorDown follows soft wraps)
// and to the column, past the line numbers. Past the text it stops at the
// end of the line or at the last line. A large (read-only) input ignores it.
func (f *field) clickAt(row, col int) {
	if f.large {
		return
	}
	target := f.ta.ScrollYOffset() + max(row, 0)
	f.ta.MoveToBegin()
	for range target {
		line, off := f.ta.Line(), f.ta.LineInfo().RowOffset
		f.ta.CursorDown()
		if f.ta.Line() == line && f.ta.LineInfo().RowOffset == off {
			break // the last visual row
		}
	}
	gutter := 0
	if f.ta.ShowLineNumbers {
		gutter = lineNumberWidth
	}
	f.ta.SetCursorColumn(f.ta.LineInfo().StartColumn + max(col-gutter, 0))
}

// lineNumberWidth is the column the textarea's line numbers take (" 1 ").
const lineNumberWidth = 3

// wheel scrolls a large input's viewport, or moves the textarea's cursor
// three lines, which scrolls it too.
func (f *field) wheel(msg tea.MouseWheelMsg) tea.Cmd {
	if f.large {
		var cmd tea.Cmd
		f.largeView, cmd = f.largeView.Update(msg)
		return cmd
	}
	for range 3 {
		switch msg.Mouse().Button {
		case tea.MouseWheelUp:
			f.ta.CursorUp()
		case tea.MouseWheelDown:
			f.ta.CursorDown()
		}
	}
	return nil
}
