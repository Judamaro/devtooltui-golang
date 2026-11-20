package workspace

import (
	"errors"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// errorExcerpt points at an InputError like a compiler does: the offending
// input line, with the culprit character underlined, and a caret below it.
// It returns nil when the error has no position or its input is unknown.
func (m *Model) errorExcerpt() []string {
	var ie *tool.InputError
	if !errors.As(m.err, &ie) || ie.Line < 1 || ie.Col < 1 {
		return nil
	}
	src, ok := m.inputValue(ie.Input)
	if !ok {
		return nil
	}
	line, ok := nthLine(src, ie.Line)
	if !ok {
		return nil
	}
	return excerpt(line, ie.Line, ie.Col, ie.Msg, m.width, m.styles)
}

// inputValue returns the content of the input with that key; an empty key
// means the first input.
func (m *Model) inputValue(key string) (string, bool) {
	for _, f := range m.inputs {
		if f.spec.Key == key || key == "" {
			return f.value(), true
		}
	}
	return "", false
}

// nthLine returns line n (1-based) of s without splitting all of it.
func nthLine(s string, n int) (string, bool) {
	for i := 1; i < n; i++ {
		j := strings.IndexByte(s, '\n')
		if j < 0 {
			return "", false
		}
		s = s[j+1:]
	}
	if j := strings.IndexByte(s, '\n'); j >= 0 {
		s = s[:j]
	}
	return strings.TrimSuffix(s, "\r"), true
}

func excerpt(line string, lineNo, col int, msg string, width int, st Styles) []string {
	num := fmt.Sprint(lineNo)
	gutter := num + " │ "
	blank := strings.Repeat(" ", len(num)) + " │ "
	avail := max(width-lipgloss.Width(gutter), 1)

	// Tabs and control characters are one cell, so the caret stays aligned.
	runes := []rune(strings.Map(func(r rune) rune {
		if r < ' ' {
			return ' '
		}
		return r
	}, line))

	// Window long lines around the culprit; "…" marks each cut.
	idx := min(col-1, len(runes)) // the culprit, in runes; len(runes) = end of line
	start, end := 0, len(runes)
	if len(runes)+1 > avail { // +1: room for a mark at the end of the line
		inner := max(avail-2, 1)
		start = max(idx-inner/2, 0)
		end = min(start+inner, len(runes))
		start = max(min(start, end-inner), 0)
	}
	head := string(runes[start:idx])
	if start > 0 {
		head = "…" + head
	}
	mark, tail := " ", ""
	if idx < len(runes) {
		mark, tail = string(runes[idx]), string(runes[idx+1:end])
	}
	if end < len(runes) {
		tail += "…"
	}

	src := st.Section.Render(gutter) + head + st.ErrorMark.Render(mark) + tail
	pad := lipgloss.Width(head)
	note := st.ErrorText.Render("^ " + msg)
	if room := avail - pad; room < 2+lipgloss.Width(msg) && pad > lipgloss.Width(msg) {
		// No room after the caret: the message goes on its left.
		pad -= lipgloss.Width(msg) + 1
		note = st.ErrorText.Render(msg + " ^")
	}
	caret := st.Section.Render(blank) + strings.Repeat(" ", pad) + note
	return []string{clip(src, width), clip(caret, width)}
}
