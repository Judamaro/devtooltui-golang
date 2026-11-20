package encoders

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// HTML escapes and unescapes HTML entities.
type HTML struct{}

func (HTML) Spec() tool.Spec {
	return tool.Spec{
		ID:          "html",
		Name:        "HTML entities",
		Category:    tool.Encoders,
		Description: "Escape and unescape HTML entities",
		Keywords:    []string{"entities", "escape", "unescape", "&amp;", "xml"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Input", Multiline: true}},
		Options: []tool.Option{
			{Key: "mode", Label: "Mode", Kind: tool.Select, Choices: []string{"encode", "decode"}, Default: "encode"},
			{Key: "ascii", Label: "Non-ASCII as &#…;", Help: "When encoding, also turn non-ASCII characters into numeric entities", Kind: tool.Toggle, Default: false},
		},
		Output: tool.PlainText,
		Live:   true,
	}
}

func (h HTML) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(h.Spec())
	in := req.Inputs["input"]

	var out string
	var meta []tool.KV
	if opts.String("mode") == "decode" {
		// Unknown entities are left as they are, as browsers do.
		out = html.UnescapeString(in)
		meta = append(meta, tool.KV{Key: "Entities", Value: fmt.Sprint(countEntities(in, -1))})
	} else {
		out = html.EscapeString(in)
		if opts.Bool("ascii") {
			out = numericEntities(out)
		}
	}
	return tool.Result{
		Text: out,
		Meta: append([]tool.KV{
			{Key: "Input", Value: fmt.Sprintf("%d B", len(in))},
			{Key: "Output", Value: fmt.Sprintf("%d B", len(out))},
		}, meta...),
	}, nil
}

// numericEntities turns every character from U+00A0 up into &#N;. Invalid
// UTF-8 bytes and C1 controls (U+0080–U+009F, which decoders map to
// Windows-1252) stay as they are, so decoding gives the input back.
func numericEntities(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r >= 0xA0 && (r != utf8.RuneError || size > 1) {
			fmt.Fprintf(&b, "&#%d;", r)
		} else {
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

var entityRe = regexp.MustCompile(`&(#[0-9]{1,7}|#[xX][0-9a-fA-F]{1,6}|[a-zA-Z][a-zA-Z0-9]{1,31});`)

// countEntities counts the entities that decode, looking at up to n
// candidates (-1 = all).
func countEntities(s string, n int) int {
	count := 0
	for _, m := range entityRe.FindAllString(s, n) {
		if html.UnescapeString(m) != m {
			count++
		}
	}
	return count
}

// Detect recognises text with at least one entity that decodes.
func (HTML) Detect(sample string) float64 {
	if countEntities(sample, 8) > 0 {
		return 0.55
	}
	return 0
}
