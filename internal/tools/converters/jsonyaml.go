package converters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// JSONYAML convierte JSON ↔ YAML conservando el orden de las claves.
type JSONYAML struct{}

func (JSONYAML) Spec() tool.Spec {
	return tool.Spec{
		ID:          "json-yaml",
		Name:        "JSON ↔ YAML",
		Category:    tool.Converters,
		Description: "Convierte entre JSON y YAML conservando el orden de las claves",
		Keywords:    []string{"yaml", "yml", "json", "convert"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Entrada", Multiline: true}},
		Options: []tool.Option{
			{Key: "mode", Label: "Modo", Kind: tool.Select, Choices: []string{"to-yaml", "to-json"}, Default: "to-yaml"},
			{Key: "indent", Label: "Indentación", Help: "En JSON, 0 = minificar; YAML usa al menos 2", Kind: tool.Number, Min: 0, Max: 8, Default: 2},
		},
		Output: tool.Highlighted,
		Live:   true,
	}
}

func (c JSONYAML) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(c.Spec())
	in := req.Inputs["input"]
	if strings.TrimSpace(in) == "" {
		return tool.Result{}, nil
	}
	if opts.String("mode") == "to-json" {
		return yamlToJSON(in, opts.Int("indent"))
	}
	return jsonToYAML(in, max(opts.Int("indent"), 2))
}

func jsonToYAML(in string, indent int) (tool.Result, error) {
	var probe any
	dec := json.NewDecoder(strings.NewReader(in))
	if err := dec.Decode(&probe); err != nil {
		return tool.Result{}, jsonError(in, err)
	}
	if dec.More() {
		return tool.Result{}, jsonError(in, &json.SyntaxError{Offset: dec.InputOffset() + 1})
	}
	// JSON es YAML: se parsea a un Node (conserva el orden) y se le quita
	// el estilo de flujo para que salga en bloque.
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(in), &node); err != nil {
		return tool.Result{}, &tool.InputError{Input: "input", Msg: err.Error()}
	}
	clearStyle(&node)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(indent)
	if err := enc.Encode(&node); err != nil {
		return tool.Result{}, fmt.Errorf("codificando YAML: %w", err)
	}
	// Sin recortar: si el último valor es un bloque "|", su salto de línea
	// final es parte del valor.
	out := buf.String()
	return tool.Result{Text: out, Lang: "yaml", Meta: sizeMeta(in, out)}, nil
}

// clearStyle quita el estilo de flujo y las comillas; el encoder vuelve a
// citar los strings que lo necesiten ("true", "1", "a: b").
func clearStyle(n *yaml.Node) {
	n.Style = 0
	if n.Kind == yaml.ScalarNode && n.Tag == "!!str" && needsQuotedBlock(n.Value) {
		// El encoder emite un bloque "|" inválido si una línea empieza con
		// espacio o tab (lo encontró el fuzz): se fuerza comillas dobles.
		n.Style = yaml.DoubleQuotedStyle
	}
	for _, c := range n.Content {
		clearStyle(c)
	}
}

// needsQuotedBlock detecta strings que el encoder de yaml emite de forma
// que no se releen igual (casos encontrados por el fuzz): espacios Unicode
// al inicio o al final, caracteres no imprimibles, o líneas con sangría.
func needsQuotedBlock(s string) bool {
	if s == "" {
		return false
	}
	first, _ := utf8.DecodeRuneInString(s)
	last, _ := utf8.DecodeLastRuneInString(s)
	if (unicode.IsSpace(first) && first != '\n') || (unicode.IsSpace(last) && last != '\n') {
		return true
	}
	for _, r := range s {
		if r != '\n' && r != '\t' && !unicode.IsPrint(r) {
			return true
		}
	}
	if !strings.Contains(s, "\n") {
		return false
	}
	for line := range strings.SplitSeq(s, "\n") {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			return true
		}
	}
	return false
}

func yamlToJSON(in string, indent int) (tool.Result, error) {
	dec := yaml.NewDecoder(strings.NewReader(in))
	var docs []any
	for {
		var node yaml.Node
		err := dec.Decode(&node)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return tool.Result{}, yamlError(err)
		}
		v, err := nodeToJSON(&node, 0)
		if err != nil {
			return tool.Result{}, err
		}
		docs = append(docs, v)
	}
	var value any = docs
	switch len(docs) {
	case 0:
		return tool.Result{}, nil
	case 1:
		value = docs[0]
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if indent > 0 {
		enc.SetIndent("", strings.Repeat(" ", indent))
	}
	if err := enc.Encode(value); err != nil {
		return tool.Result{}, fmt.Errorf("codificando JSON: %w", err)
	}
	out := strings.TrimRight(buf.String(), "\n")
	meta := sizeMeta(in, out)
	if len(docs) > 1 {
		meta = append(meta, tool.KV{Key: "Documentos", Value: strconv.Itoa(len(docs))})
	}
	return tool.Result{Text: out, Lang: "json", Meta: meta}, nil
}

// orderedMap se serializa como objeto JSON conservando el orden.
type orderedMap struct {
	keys   []string
	values map[string]any
}

func (o *orderedMap) set(k string, v any) {
	if _, ok := o.values[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.values[k] = v
}

func (o *orderedMap) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, err := marshalNoEscape(k)
		if err != nil {
			return nil, err
		}
		vb, err := marshalNoEscape(o.values[k])
		if err != nil {
			return nil, err
		}
		b.Write(kb)
		b.WriteByte(':')
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func marshalNoEscape(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

const maxDepth = 1000 // corta los ciclos de aliases

var jsonNumberRe = regexp.MustCompile(`^-?(0|[1-9]\d*)(\.\d+)?([eE][-+]?\d+)?$`)

func nodeToJSON(n *yaml.Node, depth int) (any, error) {
	if depth > maxDepth {
		return nil, &tool.InputError{Input: "input", Line: n.Line, Col: n.Column, Msg: "anidamiento demasiado profundo (¿aliases cíclicos?)"}
	}
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, nil
		}
		return nodeToJSON(n.Content[0], depth+1)
	case yaml.AliasNode:
		return nodeToJSON(n.Alias, depth+1)
	case yaml.SequenceNode:
		out := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			v, err := nodeToJSON(c, depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	case yaml.MappingNode:
		return mappingToJSON(n, depth)
	case yaml.ScalarNode:
		return scalarToJSON(n)
	}
	return nil, &tool.InputError{Input: "input", Line: n.Line, Col: n.Column, Msg: "nodo YAML desconocido"}
}

func mappingToJSON(n *yaml.Node, depth int) (any, error) {
	obj := &orderedMap{values: map[string]any{}}
	var merges []*yaml.Node
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if k.Kind == yaml.ScalarNode && k.Tag == "!!merge" {
			merges = append(merges, v)
			continue
		}
		if k.Kind != yaml.ScalarNode {
			return nil, &tool.InputError{Input: "input", Line: k.Line, Col: k.Column, Msg: "JSON solo admite claves escalares"}
		}
		val, err := nodeToJSON(v, depth+1)
		if err != nil {
			return nil, err
		}
		obj.set(k.Value, val)
	}
	// Las claves de merge (<<) no pisan las claves explícitas.
	for _, m := range merges {
		srcs := []*yaml.Node{m}
		if m.Kind == yaml.SequenceNode {
			srcs = m.Content
		}
		for _, src := range srcs {
			v, err := nodeToJSON(src, depth+1)
			if err != nil {
				return nil, err
			}
			sub, ok := v.(*orderedMap)
			if !ok {
				return nil, &tool.InputError{Input: "input", Line: src.Line, Col: src.Column, Msg: "<< necesita un mapa"}
			}
			for _, k := range sub.keys {
				if _, exists := obj.values[k]; !exists {
					obj.set(k, sub.values[k])
				}
			}
		}
	}
	return obj, nil
}

func scalarToJSON(n *yaml.Node) (any, error) {
	switch n.ShortTag() {
	case "!!null":
		return nil, nil
	case "!!bool":
		var b bool
		if err := n.Decode(&b); err != nil {
			return n.Value, nil
		}
		return b, nil
	case "!!int", "!!float":
		if jsonNumberRe.MatchString(n.Value) {
			return json.Number(n.Value), nil // sin perder precisión
		}
		var v any
		if err := n.Decode(&v); err != nil {
			return n.Value, nil
		}
		if f, ok := v.(float64); ok && (f != f || f > 1e308 || f < -1e308) {
			return nil, &tool.InputError{Input: "input", Line: n.Line, Col: n.Column, Msg: fmt.Sprintf("JSON no admite %s", n.Value)}
		}
		return v, nil
	default: // !!str, !!timestamp, !!binary y tags propios: texto
		return n.Value, nil
	}
}

var yamlLineRe = regexp.MustCompile(`line (\d+)(?:, column (\d+))?`)

func yamlError(err error) error {
	msg := strings.TrimPrefix(err.Error(), "yaml: ")
	ie := &tool.InputError{Input: "input", Msg: msg}
	if m := yamlLineRe.FindStringSubmatch(msg); m != nil {
		ie.Line, _ = strconv.Atoi(m[1])
		if m[2] != "" {
			ie.Col, _ = strconv.Atoi(m[2])
		}
		ie.Msg = strings.TrimSpace(strings.TrimPrefix(yamlLineRe.ReplaceAllString(msg, ""), ":"))
	}
	return ie
}

func jsonError(in string, err error) error {
	offset := len(in)
	var syn *json.SyntaxError
	msg := "fin de entrada inesperado"
	if errors.As(err, &syn) {
		offset = max(int(syn.Offset)-1, 0)
		if syn.Error() != "" {
			msg = syn.Error()
		} else {
			msg = "datos después del valor JSON"
		}
	} else if !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		msg = err.Error()
	}
	line, col := tool.Position(in, offset)
	return &tool.InputError{Input: "input", Line: line, Col: col, Msg: msg}
}

func sizeMeta(in, out string) []tool.KV {
	return []tool.KV{{Key: "Tamaño", Value: fmt.Sprintf("%d B → %d B", len(in), len(out))}}
}

var yamlKeyRe = regexp.MustCompile(`(?m)^\s*(-\s+)?[\w"'.-][^:\n]*:(\s|$)`)

// Detect es el respaldo: texto con "clave:" que YAML parsea como mapa o
// lista (YAML acepta casi todo, por eso el puntaje es bajo).
func (JSONYAML) Detect(sample string) float64 {
	s := strings.TrimSpace(sample)
	if s == "" || s[0] == '{' || s[0] == '[' || !yamlKeyRe.MatchString(s) {
		return 0
	}
	var node yaml.Node
	if yaml.Unmarshal([]byte(s), &node) != nil || len(node.Content) == 0 {
		return 0
	}
	if k := node.Content[0].Kind; k != yaml.MappingNode && k != yaml.SequenceNode {
		return 0
	}
	return 0.40
}
