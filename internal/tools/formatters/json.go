// Package formatters agrupa las herramientas de formateo.
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

// JSONFormat indenta, minifica y ordena JSON.
type JSONFormat struct{}

func (JSONFormat) Spec() tool.Spec {
	return tool.Spec{
		ID:          "json-format",
		Name:        "JSON",
		Category:    tool.Formatters,
		Description: "Indenta, minifica y ordena claves de JSON",
		Keywords:    []string{"json", "pretty", "minify", "beautify", "format"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Entrada", Lang: "json", Multiline: true}},
		Options: []tool.Option{
			{Key: "indent", Label: "Indentación", Help: "Espacios por nivel; 0 = minificar", Kind: tool.Number, Min: 0, Max: 8, Default: 2},
			{Key: "sort_keys", Label: "Ordenar claves", Kind: tool.Toggle, Default: false},
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
			{Key: "JSON", Value: "válido"},
			{Key: "Tamaño", Value: fmt.Sprintf("%d B → %d B", len(in), len(out))},
		},
	}, nil
}

// formatPreserving conserva el orden de las claves y el texto de los números.
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

// formatSorted decodifica y vuelve a codificar: los objetos salen con las
// claves ordenadas. UseNumber conserva la precisión de los números.
func formatSorted(in, indent string) ([]byte, error) {
	dec := json.NewDecoder(strings.NewReader(in))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		// Hay algo después del valor: lo valida Compact con su offset exacto.
		var discard bytes.Buffer
		if cerr := json.Compact(&discard, []byte(in)); cerr != nil {
			return nil, cerr
		}
		return nil, fmt.Errorf("datos después del valor JSON")
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
		// Offset es la cantidad de bytes leídos al fallar: el carácter
		// culpable es el anterior, salvo en un fin de entrada inesperado.
		offset = int(syn.Offset)
		if offset > 0 && offset <= len(in) && !strings.Contains(msg, "end of JSON input") {
			offset--
		}
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		msg = "fin de entrada inesperado"
	}
	line, col := tool.Position(in, offset)
	return &tool.InputError{Input: "input", Line: line, Col: col, Msg: msg}
}

// Detect reconoce un objeto o arreglo JSON válido.
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
