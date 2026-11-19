// Package testers agrupa las herramientas para probar expresiones.
package testers

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"strconv"
	"strings"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// MaxMatches limita las coincidencias listadas.
const MaxMatches = 10000

// Regex prueba expresiones regulares RE2 sobre un texto.
type Regex struct{}

func (Regex) Spec() tool.Spec {
	return tool.Spec{
		ID:          "regex",
		Name:        "RegEx",
		Category:    tool.Testers,
		Description: "Prueba expresiones regulares RE2 (sin lookaround ni backreferences)",
		Keywords:    []string{"regexp", "regular", "expression", "match", "re2", "grep"},
		Inputs:      []tool.InputSpec{{Key: "text", Label: "Texto", Multiline: true}},
		Options: []tool.Option{
			{Key: "pattern", Label: "Patrón", Kind: tool.String, Default: ""},
			{Key: "ignore_case", Label: "i", Help: "Ignorar mayúsculas", Kind: tool.Toggle, Default: false},
			{Key: "multiline", Label: "m", Help: "^ y $ por línea", Kind: tool.Toggle, Default: false},
			{Key: "dotall", Label: "s", Help: ". incluye \\n", Kind: tool.Toggle, Default: false},
		},
		Output: tool.Matches,
		Live:   true,
	}
}

func (r Regex) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(r.Spec())
	pattern := opts.String("pattern")
	if pattern == "" {
		return tool.Result{Text: "Escribe un patrón en las opciones."}, nil
	}
	var flags string
	for _, f := range []struct{ key, flag string }{{"ignore_case", "i"}, {"multiline", "m"}, {"dotall", "s"}} {
		if opts.Bool(f.key) {
			flags += f.flag
		}
	}
	full := pattern
	if flags != "" {
		full = "(?" + flags + ")" + pattern
	}
	re, err := regexp.Compile(full)
	if err != nil {
		return tool.Result{}, patternError(pattern, err)
	}

	text := req.Inputs["text"]
	matches := re.FindAllStringSubmatchIndex(text, MaxMatches)
	names := re.SubexpNames()
	pos := newPosTracker(text)

	var b strings.Builder
	var spans []tool.Span
	fmt.Fprintf(&b, "%d coincidencia", len(matches))
	if len(matches) != 1 {
		b.WriteString("s")
	}
	if len(matches) == MaxMatches {
		fmt.Fprintf(&b, " (límite de %d)", MaxMatches)
	}
	for i, m := range matches {
		if i%1024 == 0 && ctx.Err() != nil {
			return tool.Result{}, ctx.Err()
		}
		sl, sc := pos.at(m[0])
		el, ec := pos.at(m[1])
		fmt.Fprintf(&b, "\n#%d  %d:%d–%d:%d  %s", i+1, sl, sc, el, ec, quote(text[m[0]:m[1]]))
		spans = append(spans, tool.Span{Input: "text", Start: m[0], End: m[1]})
		for g := 1; g < len(m)/2; g++ {
			label := strconv.Itoa(g)
			if names[g] != "" {
				label += " (" + names[g] + ")"
			}
			if m[2*g] < 0 {
				fmt.Fprintf(&b, "\n    %s: —", label)
				continue
			}
			fmt.Fprintf(&b, "\n    %s: %s", label, quote(text[m[2*g]:m[2*g+1]]))
			spans = append(spans, tool.Span{Input: "text", Start: m[2*g], End: m[2*g+1], Group: g})
		}
	}
	return tool.Result{
		Text:  b.String(),
		Spans: spans,
		Meta: []tool.KV{
			{Key: "Coincidencias", Value: strconv.Itoa(len(matches))},
			{Key: "Grupos", Value: strconv.Itoa(re.NumSubexp())},
		},
	}, nil
}

// quote muestra la coincidencia entre comillas, recortada si es larga.
func quote(s string) string {
	const maxRunes = 60
	if r := []rune(s); len(r) > maxRunes {
		return strconv.Quote(string(r[:maxRunes])) + "…"
	}
	return strconv.Quote(s)
}

// patternError ubica el error en el patrón: RE2 no da offsets, pero sí el
// fragmento culpable.
func patternError(pattern string, err error) error {
	ie := &tool.InputError{Input: "pattern", Line: 1, Msg: err.Error()}
	var se *syntax.Error
	if !errors.As(err, &se) {
		return ie
	}
	ie.Msg = string(se.Code)
	if se.Expr != "" {
		ie.Msg += ": " + se.Expr
		if i := strings.Index(pattern, se.Expr); i >= 0 {
			_, ie.Col = tool.Position(pattern, i)
		}
	}
	switch {
	case strings.HasPrefix(se.Expr, "(?<=") || strings.HasPrefix(se.Expr, "(?<!"):
		// Desde Go 1.22 "(?<" abre un grupo con nombre: llega como
		// ErrInvalidNamedCapture.
		ie.Msg += " (RE2 no soporta lookbehind)"
	case se.Code == syntax.ErrInvalidPerlOp && (strings.HasPrefix(se.Expr, "(?=") || strings.HasPrefix(se.Expr, "(?!")):
		ie.Msg += " (RE2 no soporta lookahead)"
	case se.Code == syntax.ErrInvalidEscape && len(se.Expr) == 2 && se.Expr[1] >= '1' && se.Expr[1] <= '9':
		ie.Msg += " (RE2 no soporta backreferences)"
	}
	return ie
}

// posTracker convierte offsets crecientes en línea:columna (en runas) en
// una sola pasada.
type posTracker struct {
	text      string
	off       int
	line, col int
}

func newPosTracker(text string) *posTracker { return &posTracker{text: text, line: 1, col: 1} }

func (p *posTracker) at(offset int) (int, int) {
	if offset < p.off { // no debería pasar: los offsets llegan ordenados
		return tool.Position(p.text, offset)
	}
	for _, r := range p.text[p.off:offset] {
		if r == '\n' {
			p.line++
			p.col = 1
		} else {
			p.col++
		}
	}
	p.off = offset
	return p.line, p.col
}
