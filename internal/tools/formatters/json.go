// Package formatters groups the formatting tools.
package formatters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
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

// formatSorted sorts object keys without decoding into maps: Compact
// validates (with the exact error offset), sortCompact reorders the members
// and Indent lays the result out. Numbers and strings keep their original
// text; with repeated keys the last one wins, as encoding/json does.
func formatSorted(in, indent string) ([]byte, error) {
	var compact bytes.Buffer
	compact.Grow(len(in))
	if err := json.Compact(&compact, []byte(in)); err != nil {
		return nil, err
	}
	src := compact.Bytes()
	out := make([]byte, 0, len(src))
	out, _ = sortCompact(out, src, 0)
	if indent == "" {
		return out, nil
	}
	var buf bytes.Buffer
	buf.Grow(len(out) + len(out)/2)
	if err := json.Indent(&buf, out, "", indent); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type jsonMember struct {
	key        string // decoded, for comparison
	start, end int    // the whole "key":value in src
}

// sortCompact copies the value at src[i:] (compact, valid JSON) to out with
// sorted object keys and returns the index right after the value.
func sortCompact(out, src []byte, i int) ([]byte, int) {
	switch src[i] {
	case '{':
		var members []jsonMember
		j := i + 1
		for src[j] != '}' {
			kEnd := skipString(src, j)
			vEnd := skipValue(src, kEnd+1) // +1 skips ':'
			members = append(members, jsonMember{key: decodeKey(src[j:kEnd]), start: j, end: vEnd})
			j = vEnd
			if src[j] == ',' {
				j++
			}
		}
		slices.SortStableFunc(members, func(a, b jsonMember) int { return strings.Compare(a.key, b.key) })
		out = append(out, '{')
		first := true
		for k, m := range members {
			if k+1 < len(members) && members[k+1].key == m.key {
				continue // a later duplicate wins
			}
			if !first {
				out = append(out, ',')
			}
			first = false
			kEnd := skipString(src, m.start)
			out = append(out, src[m.start:kEnd+1]...) // "key":
			out, _ = sortCompact(out, src, kEnd+1)
		}
		return append(out, '}'), j + 1
	case '[':
		out = append(out, '[')
		j := i + 1
		for src[j] != ']' {
			if j > i+1 {
				out = append(out, ',')
			}
			out, j = sortCompact(out, src, j)
			if src[j] == ',' {
				j++
			}
		}
		return append(out, ']'), j + 1
	default:
		end := skipValue(src, i)
		return append(out, src[i:end]...), end
	}
}

// skipValue returns the index right after the compact value at src[i:].
func skipValue(src []byte, i int) int {
	switch src[i] {
	case '"':
		return skipString(src, i)
	case '{', '[':
		depth := 0
		for j := i; j < len(src); j++ {
			switch src[j] {
			case '"':
				j = skipString(src, j) - 1
			case '{', '[':
				depth++
			case '}', ']':
				depth--
				if depth == 0 {
					return j + 1
				}
			}
		}
		return len(src)
	default: // number, true, false, null
		j := i
		for j < len(src) && src[j] != ',' && src[j] != '}' && src[j] != ']' {
			j++
		}
		return j
	}
}

// skipString returns the index right after the string that opens at src[i].
func skipString(src []byte, i int) int {
	for j := i + 1; j < len(src); j++ {
		switch src[j] {
		case '\\':
			j++
		case '"':
			return j + 1
		}
	}
	return len(src)
}

func decodeKey(raw []byte) string {
	if !bytes.ContainsRune(raw, '\\') {
		return string(raw[1 : len(raw)-1])
	}
	var k string
	if err := json.Unmarshal(raw, &k); err != nil {
		return string(raw)
	}
	return k
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
