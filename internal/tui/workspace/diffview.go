package workspace

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// DiffStyles los define tui/theme.go.
type DiffStyles struct {
	Insert, Delete, Change, Hunk lipgloss.Style
}

// renderDiff colorea una salida DiffView. Result.Lang dice el formato:
// "diff" (unificado: prefijo +/-/@@), "diff-side" (marcador </>/| al inicio)
// o vacío (por palabras: se pintan las operaciones de Result.Diff).
func renderDiff(res tool.Result, s DiffStyles) string {
	switch res.Lang {
	case "diff", "diff-side":
		lines := strings.Split(res.Text, "\n")
		for i, l := range lines {
			if st, ok := diffLineStyle(l, res.Lang == "diff-side", s); ok {
				lines[i] = st.Render(l)
			}
		}
		return strings.Join(lines, "\n")
	}
	if len(res.Diff) == 0 {
		return res.Text
	}
	var b strings.Builder
	for _, op := range res.Diff {
		var st lipgloss.Style
		switch op.Kind {
		case tool.Insert:
			st = s.Insert
		case tool.Delete:
			st = s.Delete
		default:
			b.WriteString(op.Text)
			continue
		}
		for i, part := range strings.Split(op.Text, "\n") {
			if i > 0 {
				b.WriteByte('\n')
			}
			if part != "" {
				b.WriteString(st.Render(part))
			}
		}
	}
	return b.String()
}

func diffLineStyle(l string, side bool, s DiffStyles) (lipgloss.Style, bool) {
	if l == "" {
		return lipgloss.Style{}, false
	}
	if side {
		switch l[0] {
		case '<':
			return s.Delete, true
		case '>':
			return s.Insert, true
		case '|':
			return s.Change, true
		}
		return lipgloss.Style{}, false
	}
	switch {
	case strings.HasPrefix(l, "+++"), strings.HasPrefix(l, "---"), strings.HasPrefix(l, "@@"):
		return s.Hunk, true
	case l[0] == '+':
		return s.Insert, true
	case l[0] == '-':
		return s.Delete, true
	}
	return lipgloss.Style{}, false
}
