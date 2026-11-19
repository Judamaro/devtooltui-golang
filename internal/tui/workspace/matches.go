package workspace

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// renderMatches devuelve text con las coincidencias completas (Group 0)
// resaltadas. Los estilos se aplican por línea para que lipgloss no rellene
// los bloques multilínea.
func renderMatches(text string, spans []tool.Span, style lipgloss.Style) string {
	var b strings.Builder
	b.Grow(len(text) + len(spans)*16)
	prev := 0
	for _, s := range spans {
		if s.Group != 0 || s.Start >= s.End || s.Start < prev || s.End > len(text) {
			continue
		}
		b.WriteString(text[prev:s.Start])
		for i, part := range strings.Split(text[s.Start:s.End], "\n") {
			if i > 0 {
				b.WriteByte('\n')
			}
			if part != "" {
				b.WriteString(style.Render(part))
			}
		}
		prev = s.End
	}
	b.WriteString(text[prev:])
	return b.String()
}
