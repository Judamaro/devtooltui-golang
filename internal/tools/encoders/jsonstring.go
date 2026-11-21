package encoders

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/formatters"
)

// maxStringLevels bounds how many nested string layers decode unwraps.
const maxStringLevels = 10

// JSONString turns JSON into a JSON string literal, like JSON.stringify of
// the text, and back: what is needed to embed JSON inside JSON, an
// environment variable or a log line, and to read it out again.
type JSONString struct{}

func (JSONString) Spec() tool.Spec {
	return tool.Spec{
		ID:          "json-string",
		Name:        "JSON stringify",
		Category:    tool.Encoders,
		Description: "Turn JSON into an escaped string (stringify) and back (parse)",
		Keywords:    []string{"stringify", "parse", "escape", "unescape", "embed", "quote", "json"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "JSON or JSON string", Lang: "json", Multiline: true}},
		Options: []tool.Option{
			{Key: "mode", Label: "Mode", Help: "encode = stringify, decode = parse", Kind: tool.Select, Choices: []string{"encode", "decode"}, Default: "encode"},
			{Key: "compact", Label: "Compact", Help: "encode: drop the whitespace of the JSON first", Kind: tool.Toggle, Default: true},
			{Key: "quotes", Label: "Quotes", Help: "encode: wrap the string in quotes", Kind: tool.Toggle, Default: true},
			{Key: "indent", Label: "Indent", Help: "decode: spaces per level of the JSON; 0 = compact", Kind: tool.Number, Min: 0, Max: 8, Default: 2},
		},
		Output: tool.Highlighted,
		Live:   true,
	}
}

func (j JSONString) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(j.Spec())
	in := req.Inputs["input"]
	if strings.TrimSpace(in) == "" {
		return tool.Result{}, nil
	}
	if opts.String("mode") == "decode" {
		return parseJSONString(in, opts.Int("indent"))
	}
	return stringifyJSON(in, opts.Bool("compact"), opts.Bool("quotes"))
}

func stringifyJSON(in string, compact, quotes bool) (tool.Result, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(in)); err != nil {
		return tool.Result{}, formatters.JSONInputError(in, err)
	}
	src := buf.String()
	if !compact {
		src = strings.TrimSpace(in)
	}
	out := escapeC(src, false)
	if quotes {
		out = `"` + out + `"`
	}
	return tool.Result{
		Text: out,
		Lang: "json",
		Meta: []tool.KV{{Key: "Size", Value: fmt.Sprintf("%d B → %d B", len(in), len(out))}},
	}, nil
}

func parseJSONString(in string, indent int) (tool.Result, error) {
	s := strings.TrimSpace(in)
	lead := strings.Index(in, s)
	text, off, msg := unescape("json", s)
	if msg != "" {
		line, col := tool.Position(in, lead+off)
		return tool.Result{}, &tool.InputError{Input: "input", Line: line, Col: col, Msg: msg}
	}
	// JSON stringified more than once decodes to another string that holds
	// JSON: unwrap it. A string whose content is not JSON (the JSON was just
	// a string, "x") is the result itself.
	levels := 1
	for levels < maxStringLevels {
		t := strings.TrimSpace(text)
		var inner string
		if len(t) < 2 || t[0] != '"' || json.Unmarshal([]byte(t), &inner) != nil || !json.Valid([]byte(inner)) {
			break
		}
		text = inner
		levels++
	}
	var buf bytes.Buffer
	var err error
	if indent > 0 {
		err = json.Indent(&buf, []byte(text), "", strings.Repeat(" ", min(indent, 8)))
	} else {
		err = json.Compact(&buf, []byte(text))
	}
	if err != nil {
		// The position is inside the decoded text, not the input.
		ie := &tool.InputError{Input: "input", Line: 1, Col: 1, Msg: "the string does not hold JSON"}
		var inner *tool.InputError
		if errors.As(formatters.JSONInputError(text, err), &inner) {
			ie.Msg = fmt.Sprintf("the string does not hold JSON: %s (at %d:%d of the decoded text)", inner.Msg, inner.Line, inner.Col)
		}
		return tool.Result{}, ie
	}
	meta := []tool.KV{{Key: "Size", Value: fmt.Sprintf("%d B → %d B", len(in), buf.Len())}}
	if levels > 1 {
		meta = append(meta, tool.KV{Key: "Levels", Value: fmt.Sprint(levels)})
	}
	return tool.Result{Text: strings.TrimSpace(buf.String()), Lang: "json", Meta: meta}, nil
}

// Detect recognises stringified JSON: a quoted string that holds escaped
// quotes next to an object or array.
func (JSONString) Detect(sample string) float64 {
	s := strings.TrimSpace(sample)
	if len(s) < 4 || s[0] != '"' || s[len(s)-1] != '"' || !strings.Contains(s, `\"`) {
		return 0
	}
	if inner := strings.TrimSpace(s[1:]); strings.HasPrefix(inner, "{") || strings.HasPrefix(inner, "[") {
		return 0.7
	}
	return 0
}
