// Package formatters groups the formatting tools.
package formatters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// JSONFormat indents, minifies and sorts JSON.
type JSONFormat struct{}

func (JSONFormat) Spec() tool.Spec {
	return tool.Spec{
		ID:          "json-format",
		Name:        "JSON",
		Category:    tool.Formatters,
		Description: "Indent, minify and sort the keys of JSON",
		Keywords:    []string{"json", "pretty", "minify", "beautify", "format"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Input", Lang: "json", Multiline: true}},
		Options: []tool.Option{
			{Key: "indent", Label: "Indent", Help: "Spaces per level; 0 = minify", Kind: tool.Number, Min: 0, Max: 8, Default: 2},
			{Key: "sort_keys", Label: "Sort keys", Kind: tool.Toggle, Default: false},
		},
		Output: tool.Highlighted,
		Live:   true,
	}
}

func (j JSONFormat) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(j.Spec())
	in := req.Inputs["input"]
	if strings.TrimSpace(in) == "" {
		return tool.Result{Lang: "json"}, nil
	}
	indent := strings.Repeat(" ", opts.Int("indent"))

	var out []byte
	var err error
	if opts.Bool("sort_keys") {
		out, err = formatSorted(in, indent)
	} else {
		out, err = formatPreserving(in, indent)
	}
	if err != nil {
		return tool.Result{}, jsonInputError(in, err)
	}
	return tool.Result{
		Text: string(out),
		Lang: "json",
		Meta: []tool.KV{
			{Key: "JSON", Value: "valid"},
			{Key: "Size", Value: fmt.Sprintf("%d B → %d B", len(in), len(out))},
		},
	}, nil
}

// formatPreserving keeps key order and the original text of numbers.
func formatPreserving(in, indent string) ([]byte, error) {
	var buf bytes.Buffer
	buf.Grow(len(in) + len(in)/2)
	var err error
	if indent == "" {
		err = json.Compact(&buf, []byte(in))
	} else {
		err = json.Indent(&buf, []byte(in), "", indent)
	}
	return bytes.TrimSpace(buf.Bytes()), err
}

// formatSorted decodes and re-encodes: objects come out with sorted keys.
// UseNumber keeps the precision of numbers.
func formatSorted(in, indent string) ([]byte, error) {
	dec := json.NewDecoder(strings.NewReader(in))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		// Something follows the value: Compact validates it with the exact offset.
		var discard bytes.Buffer
		if cerr := json.Compact(&discard, []byte(in)); cerr != nil {
			return nil, cerr
		}
		return nil, fmt.Errorf("data after the JSON value")
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", indent)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buf.Bytes()), nil
}

func jsonInputError(in string, err error) error {
	offset := len(in)
	msg := err.Error()
	var syn *json.SyntaxError
	switch {
	case errors.As(err, &syn):
		// Offset is the number of bytes read when failing: the culprit is the
		// previous character, except on an unexpected end of input.
		offset = int(syn.Offset)
		if offset > 0 && offset <= len(in) && !strings.Contains(msg, "end of JSON input") {
			offset--
		}
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		msg = "unexpected end of input"
	}
	line, col := tool.Position(in, offset)
	return &tool.InputError{Input: "input", Line: line, Col: col, Msg: msg}
}

// Detect recognises a valid JSON object or array.
func (JSONFormat) Detect(sample string) float64 {
	s := strings.TrimSpace(sample)
	if s == "" || (s[0] != '{' && s[0] != '[') {
		return 0
	}
	if json.Valid([]byte(s)) {
		return 0.9
	}
	return 0
}
