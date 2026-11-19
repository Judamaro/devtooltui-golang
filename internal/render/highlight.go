// Package render colours tool outputs for the terminal. The TUI and the CLI
// both use it; tools know nothing about colours.
package render

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
)

// MaxBytes: above this size the output is not coloured.
// BenchmarkHighlight (Ryzen 7 7700): 64 KB ≈ 35 ms, 256 KB ≈ 127 ms,
// 1 MB ≈ 446 ms; 64 KB is what fits the 50 ms budget.
const MaxBytes = 64 << 10

// Highlight colours text with lang's lexer using 16 ANSI colours.
// It returns ok=false when there is no lexer or colouring fails.
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
