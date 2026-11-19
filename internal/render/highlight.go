// Package render colorea las salidas de las herramientas para la terminal.
// Lo usan la TUI y la CLI; las herramientas no saben nada de colores.
package render

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
)

// MaxBytes: por encima de este tamaño la salida no se colorea.
// BenchmarkHighlight (Ryzen 7 7700): 64 KB ≈ 35 ms, 256 KB ≈ 127 ms,
// 1 MB ≈ 446 ms; 64 KB es lo que cabe en el presupuesto de 50 ms.
const MaxBytes = 64 << 10

// Highlight colorea text con el lexer de lang usando 16 colores ANSI.
// Devuelve ok=false si no hay lexer o falla el coloreado.
func Highlight(text, lang string, style *chroma.Style) (string, bool) {
	if style == nil || lang == "" {
		return "", false
	}
	lexer := lexers.Get(lang)
	if lexer == nil {
		return "", false
	}
	it, err := chroma.Coalesce(lexer).Tokenise(nil, text)
	if err != nil {
		return "", false
	}
	var b strings.Builder
	b.Grow(len(text) * 2)
	if err := formatters.TTY16.Format(&b, style, it); err != nil {
		return "", false
	}
	return b.String(), true
}
