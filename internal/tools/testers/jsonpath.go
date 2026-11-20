package testers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ohler55/ojg/jp"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/formatters"
)

// JSONPath evaluates a JSONPath expression against a JSON document. The
// matches come out as a JSON array: ojg returns values, not positions in the
// input, so there are no Spans.
type JSONPath struct{}

func (JSONPath) Spec() tool.Spec {
	return tool.Spec{
		ID:          "jsonpath",
		Name:        "JSONPath",
		Category:    tool.Testers,
		Description: "Query JSON with JSONPath expressions ($.store.book[?(@.price < 10)].title)",
		Keywords:    []string{"json", "path", "query", "jq", "filter", "select"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "JSON", Lang: "json", Multiline: true}},
		Options: []tool.Option{
			{Key: "path", Label: "Path", Help: "JSONPath expression; $ is the root", Kind: tool.String, Default: "$"},
		},
		Output: tool.Highlighted,
		Live:   true,
	}
}

func (p JSONPath) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(p.Spec())
	path := strings.TrimSpace(opts.String("path"))
	if path == "" {
		path = "$"
	}
	expr, err := jp.ParseString(path)
	if err != nil {
		return tool.Result{}, pathError(opts.String("path"), err)
	}
	in := req.Inputs["input"]
	if strings.TrimSpace(in) == "" {
		return tool.Result{Lang: "json"}, nil
	}
	data, err := decodeOrdered(in)
	if err != nil {
		return tool.Result{}, formatters.JSONInputError(in, err)
	}
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	results := expr.Get(data)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if results == nil {
		results = []any{}
	}
	if err := enc.Encode(results); err != nil {
		return tool.Result{}, err
	}
	return tool.Result{
		Text: strings.TrimSuffix(buf.String(), "\n"),
		Lang: "json",
		Meta: []tool.KV{{Key: "Matches", Value: fmt.Sprint(len(results))}},
	}, nil
}

// decodeOrdered decodes JSON keeping the key order of objects, so the
// matches come out in document order (Go maps would shuffle them). Numbers
// become int64 when they fit and float64 otherwise, which is what JSONPath
// filters compare.
func decodeOrdered(in string) (any, error) {
	// Compact validates first, with the same error offsets as json-format;
	// the token decoder below only sees valid JSON.
	var discard bytes.Buffer
	if err := json.Compact(&discard, []byte(in)); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(strings.NewReader(in))
	dec.UseNumber()
	return decodeValue(dec)
}

func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		if t == '[' {
			arr := []any{}
			for dec.More() {
				v, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			_, err := dec.Token() // ]
			return arr, err
		}
		obj := &jsonObject{vals: map[string]any{}}
		for dec.More() {
			k, err := dec.Token()
			if err != nil {
				return nil, err
			}
			v, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			obj.SetValueForKey(k.(string), v)
		}
		_, err := dec.Token() // }
		return obj, err
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i, nil
		}
		f, err := t.Float64()
		if err != nil {
			return nil, err
		}
		return f, nil
	}
	return tok, nil // string, bool or nil
}

// jsonObject is a JSON object that remembers key order; it implements
// jp.Keyed. A repeated key keeps its first position and its last value.
type jsonObject struct {
	keys []string
	vals map[string]any
}

func (o *jsonObject) ValueForKey(key string) (any, bool) {
	v, ok := o.vals[key]
	return v, ok
}

func (o *jsonObject) SetValueForKey(key string, value any) {
	if _, ok := o.vals[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.vals[key] = value
}

func (o *jsonObject) RemoveValueForKey(key string) {
	if _, ok := o.vals[key]; ok {
		delete(o.vals, key)
		o.keys = slices.DeleteFunc(o.keys, func(k string) bool { return k == key })
	}
}

func (o *jsonObject) Keys() []string { return slices.Clone(o.keys) }

// MarshalJSON writes the keys in document order, without HTML escaping.
func (o *jsonObject) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	buf.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := enc.Encode(k); err != nil {
			return nil, err
		}
		buf.WriteByte(':')
		if err := enc.Encode(o.vals[k]); err != nil {
			return nil, err
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// ojg reports path errors as "<msg> at <pos> in <path>" (pos from 1).
var pathErrRe = regexp.MustCompile(`^(.*) at (\d+) in `)

func pathError(path string, err error) error {
	ie := &tool.InputError{Input: "path", Line: 1, Col: 1, Msg: err.Error()}
	if m := pathErrRe.FindStringSubmatch(err.Error()); m != nil {
		ie.Msg = m[1]
		if pos, perr := strconv.Atoi(m[2]); perr == nil {
			// The position counts bytes of the trimmed path.
			lead := len(path) - len(strings.TrimLeft(path, " \t\r\n"))
			_, ie.Col = tool.Position(path, min(lead+pos-1, len(path)))
		}
	}
	return ie
}
