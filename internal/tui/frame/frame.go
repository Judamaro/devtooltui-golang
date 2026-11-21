// Package frame draws the boxes that group the TUI sections: a square
// border with the title inside its top edge.
//
//	┌Input──────┐
//	│  1 {"a":1}│
//	└───────────┘
package frame

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Styles are defined by tui/theme.go.
type Styles struct {
	Border, BorderFocused lipgloss.Style // the lines
	Title, TitleFocused   lipgloss.Style // the title in the top edge
}

// Box draws body inside a w×h box (border included). The title is cut with
// "…" when it does not fit; body lines are cut at w-2 columns (ANSI aware)
// and padded, and extra lines are dropped, so the box is always exactly
// w×h. Below 2×2 there is no room for a border and it returns body as is.
func Box(title, body string, w, h int, focused bool, s Styles) string {
	if w < 2 || h < 2 {
		return body
	}
	border, titleStyle := s.Border, s.Title
	if focused {
		border, titleStyle = s.BorderFocused, s.TitleFocused
	}
	inner := w - 2

	top := border.Render("┌")
	t := ansi.Truncate(title, inner, "…")
	if t != "" {
		top += titleStyle.Render(t)
	}
	top += border.Render(strings.Repeat("─", inner-ansi.StringWidth(t)) + "┐")

	lines := strings.Split(body, "\n")
	side := border.Render("│")
	rows := make([]string, 0, h)
	rows = append(rows, top)
	for i := range h - 2 {
		line := ""
		if i < len(lines) {
			line = ansi.Truncate(lines[i], inner, "")
		}
		if pad := inner - ansi.StringWidth(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		rows = append(rows, side+line+side)
	}
	rows = append(rows, border.Render("└"+strings.Repeat("─", inner)+"┘"))
	return strings.Join(rows, "\n")
}
