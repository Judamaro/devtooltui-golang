package render

import (
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Styles reúne los estilos de coloreado (la TUI los toma de theme.go).
type Styles struct {
	Chroma *chroma.Style  // salidas Highlighted
	Match  lipgloss.Style // coincidencias de las salidas Matches
	Diff   DiffStyles     // salidas DiffView
}

// Result colorea el resultado según el tipo de salida de la herramienta.
// inputs son las entradas de la ejecución (Matches resalta sobre ellas).
// Devuelve rendered vacío si no aplica, y noColor si se omitió por tamaño.
func Result(res tool.Result, kind tool.OutputKind, inputs map[string]string, s Styles) (rendered string, noColor bool) {
	switch {
	case kind == tool.Highlighted && res.Lang != "":
		if len(res.Text) > MaxBytes {
			return "", true
		}
		out, _ := Highlight(res.Text, res.Lang, s.Chroma)
		return out, false
	case kind == tool.DiffView:
		if len(res.Text) > MaxBytes {
			return "", true
		}
		return Diff(res, s.Diff), false
	case kind == tool.Matches && len(res.Spans) > 0:
		// El texto con las coincidencias resaltadas va sobre la lista.
		subject := inputs[res.Spans[0].Input]
		if len(subject) > MaxBytes {
			return "", true
		}
		return Matches(subject, res.Spans, s.Match) + "\n\n" + res.Text, false
	}
	return "", false
}
