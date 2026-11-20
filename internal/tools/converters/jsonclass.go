package converters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/format"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/formatters"
	"github.com/Judamaro/devtooltui-golang/internal/tools/text"
)

// JSONClass generates Go structs or C# records/classes from a JSON sample.
// Types are inferred from every value seen: arrays of objects merge their
// fields, and fields that are missing or null somewhere become optional.
type JSONClass struct{}

func (JSONClass) Spec() tool.Spec {
	return tool.Spec{
		ID:          "json-class",
		Name:        "JSON → classes",
		Category:    tool.Converters,
		Description: "Generate Go structs or C# records/classes from a JSON sample",
		Keywords:    []string{"struct", "class", "record", "dto", "model", "csharp", "golang", "quicktype"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "JSON", Lang: "json", Multiline: true}},
		Options: []tool.Option{
			{Key: "lang", Label: "Language", Kind: tool.Select, Choices: []string{"go", "csharp"}, Default: "go"},
			{Key: "name", Label: "Root type", Kind: tool.String, Default: "Root"},
			{Key: "csharp", Label: "C# kind", Help: "record (init-only) or class (get/set)", Kind: tool.Select, Choices: []string{"record", "class"}, Default: "record"},
			{Key: "omitempty", Label: "omitempty", Help: "Go: add omitempty to optional fields", Kind: tool.Toggle, Default: true},
		},
		Output: tool.Highlighted,
		Live:   true,
	}
}

// shape is the inferred type of a JSON value.
type shape struct {
	kind     string // "string", "int", "float", "bool", "null", "object", "array", "any"
	fields   []*field
	elem     *shape // arrays
	name     string // object type name, set when emitting
	nullable bool
}

type field struct {
	key      string
	s        *shape
	optional bool // missing in some object, or null
}

func (j JSONClass) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(j.Spec())
	in := req.Inputs["input"]
	if strings.TrimSpace(in) == "" {
		return tool.Result{}, nil
	}
	var discard bytes.Buffer
	if err := json.Compact(&discard, []byte(in)); err != nil {
		return tool.Result{}, formatters.JSONInputError(in, err)
	}
	dec := json.NewDecoder(strings.NewReader(in))
	dec.UseNumber()
	root, err := infer(dec)
	if err != nil {
		return tool.Result{}, formatters.JSONInputError(in, err)
	}
	arrayOfObjects := root.kind == "array" && root.elem != nil && root.elem.kind == "object"
	if root.kind != "object" && !arrayOfObjects {
		return tool.Result{}, &tool.InputError{Input: "input", Line: 1, Col: 1, Msg: "expected a JSON object or an array of objects"}
	}
	name := typeName(strings.TrimSpace(opts.String("name")), "Root")

	g := &generator{used: map[string]bool{}}
	var out, lang string
	switch opts.String("lang") {
	case "csharp":
		out, lang = g.csharp(root, name, opts.String("csharp") == "class"), "csharp"
	default:
		src := g.golang(root, name, opts.Bool("omitempty"))
		formatted, ferr := format.Source([]byte(src))
		if ferr != nil {
			return tool.Result{}, fmt.Errorf("generated Go does not format: %w", ferr) // a bug
		}
		out, lang = strings.TrimSpace(string(formatted)), "go"
	}
	return tool.Result{
		Text: out,
		Lang: lang,
		Meta: []tool.KV{{Key: "Types", Value: fmt.Sprint(len(g.order))}},
	}, nil
}

// infer reads one value from dec, keeping object key order.
func infer(dec *json.Decoder) (*shape, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		if t == '{' {
			s := &shape{kind: "object"}
			for dec.More() {
				k, err := dec.Token()
				if err != nil {
					return nil, err
				}
				v, err := infer(dec)
				if err != nil {
					return nil, err
				}
				key := k.(string)
				if i := slices.IndexFunc(s.fields, func(f *field) bool { return f.key == key }); i >= 0 {
					s.fields[i].s = v // a repeated key: the last value wins
				} else {
					s.fields = append(s.fields, &field{key: key, s: v, optional: v.kind == "null"})
				}
			}
			_, err := dec.Token()
			return s, err
		}
		s := &shape{kind: "array"}
		for dec.More() {
			v, err := infer(dec)
			if err != nil {
				return nil, err
			}
			s.elem = merge(s.elem, v)
		}
		_, err := dec.Token()
		return s, err
	case json.Number:
		if _, err := t.Int64(); err == nil {
			return &shape{kind: "int"}, nil
		}
		return &shape{kind: "float"}, nil
	case string:
		return &shape{kind: "string"}, nil
	case bool:
		return &shape{kind: "bool"}, nil
	}
	return &shape{kind: "null"}, nil
}

// merge combines two inferred types (array elements, the same field in
// several objects).
func merge(a, b *shape) *shape {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	case a.kind == "null":
		c := *b
		c.nullable = true
		return &c
	case b.kind == "null":
		c := *a
		c.nullable = true
		return &c
	}
	nullable := a.nullable || b.nullable
	switch {
	case a.kind == "object" && b.kind == "object":
		out := &shape{kind: "object", nullable: nullable}
		for _, f := range a.fields {
			out.fields = append(out.fields, &field{key: f.key, s: f.s, optional: f.optional})
		}
		for _, f := range b.fields {
			if i := slices.IndexFunc(out.fields, func(o *field) bool { return o.key == f.key }); i >= 0 {
				out.fields[i].s = merge(out.fields[i].s, f.s)
				out.fields[i].optional = out.fields[i].optional || f.optional
			} else {
				out.fields = append(out.fields, &field{key: f.key, s: f.s, optional: true})
			}
		}
		for _, o := range out.fields { // keys missing from b
			if !slices.ContainsFunc(b.fields, func(f *field) bool { return f.key == o.key }) {
				o.optional = true
			}
		}
		return out
	case a.kind == "array" && b.kind == "array":
		return &shape{kind: "array", elem: merge(a.elem, b.elem), nullable: nullable}
	case a.kind == b.kind:
		return &shape{kind: a.kind, nullable: nullable}
	case (a.kind == "int" && b.kind == "float") || (a.kind == "float" && b.kind == "int"):
		return &shape{kind: "float", nullable: nullable}
	}
	return &shape{kind: "any", nullable: nullable}
}

type generator struct {
	used  map[string]bool
	order []*shape // object types in emission order
}

// name gives every object shape a unique type name, depth first.
func (g *generator) name(s *shape, want string) {
	if s == nil {
		return
	}
	switch s.kind {
	case "object":
		if s.name != "" || (len(s.fields) == 0 && len(g.order) > 0) {
			return // named already, or an empty nested object: a map
		}
		n := want
		for i := 2; g.used[n]; i++ {
			n = fmt.Sprintf("%s%d", want, i)
		}
		g.used[n] = true
		s.name = n
		g.order = append(g.order, s)
		for _, f := range s.fields {
			g.name(f.s, typeName(f.key, "Field"))
		}
	case "array":
		g.name(s.elem, singular(want))
	}
}

func (g *generator) golang(root *shape, name string, omitempty bool) string {
	var b strings.Builder
	if root.kind == "array" {
		g.name(root.elem, name+"Item")
		fmt.Fprintf(&b, "type %s []%s\n\n", name, root.elem.name)
	} else {
		g.name(root, name)
	}
	for _, s := range g.order {
		fmt.Fprintf(&b, "type %s struct {\n", s.name)
		names := map[string]bool{}
		for _, f := range s.fields {
			fn := uniqueField(goField(f.key), names)
			tag := f.key
			if omitempty && (f.optional || f.s.nullable) {
				tag += ",omitempty"
			}
			fmt.Fprintf(&b, "\t%s %s `json:%s`\n", fn, goType(f.s, f.optional), strconv.Quote(tag))
		}
		b.WriteString("}\n\n")
	}
	return b.String()
}

func goType(s *shape, optional bool) string {
	var t string
	switch s.kind {
	case "string":
		t = "string"
	case "int":
		t = "int64"
	case "float":
		t = "float64"
	case "bool":
		t = "bool"
	case "object":
		if s.name == "" { // {} tells nothing about its keys
			return "map[string]any"
		}
		t = s.name
	case "array":
		if s.elem == nil {
			return "[]any"
		}
		return "[]" + goType(s.elem, false)
	default:
		return "any"
	}
	if optional || s.nullable {
		return "*" + t
	}
	return t
}

func (g *generator) csharp(root *shape, name string, class bool) string {
	var body strings.Builder
	if root.kind == "array" {
		g.name(root.elem, name+"Item")
		fmt.Fprintf(&body, "// The JSON is a List<%s>.\n\n", root.elem.name)
	} else {
		g.name(root, name)
	}
	kind, accessor := "record", "{ get; init; }"
	if class {
		kind, accessor = "class", "{ get; set; }"
	}
	lists := false
	for i, s := range g.order {
		if i > 0 {
			body.WriteString("\n")
		}
		fmt.Fprintf(&body, "public %s %s\n{\n", kind, s.name)
		names := map[string]bool{s.name: true} // a member cannot be named like its type
		for j, f := range s.fields {
			if j > 0 {
				body.WriteString("\n")
			}
			t := csType(f.s, f.optional)
			lists = lists || strings.Contains(t, "List<") || strings.Contains(t, "Dictionary<")
			fmt.Fprintf(&body, "    [JsonPropertyName(%s)]\n", strconv.Quote(f.key))
			fmt.Fprintf(&body, "    public %s %s %s\n", t, uniqueField(csField(f.key), names), accessor)
		}
		body.WriteString("}\n")
	}
	head := "using System.Text.Json.Serialization;\n"
	if lists {
		head = "using System.Collections.Generic;\n" + head
	}
	return strings.TrimSpace(head + "\n" + body.String())
}

func csType(s *shape, optional bool) string {
	var t string
	switch s.kind {
	case "string":
		t = "string"
	case "int":
		t = "long"
	case "float":
		t = "double"
	case "bool":
		t = "bool"
	case "object":
		t = s.name
		if t == "" {
			t = "Dictionary<string, object>"
		}
	case "array":
		el := "object"
		if s.elem != nil {
			el = csType(s.elem, false)
		}
		t = "List<" + el + ">"
	default:
		return "object?"
	}
	if optional || s.nullable {
		return t + "?"
	}
	return t
}

// words splits a JSON key into words (snake, kebab, camel, spaces).
func words(key string) []string {
	return text.IdentWords(key, false)
}

var goInitialisms = map[string]bool{
	"ID": true, "URL": true, "URI": true, "HTTP": true, "HTTPS": true, "API": true, "JSON": true, "XML": true,
	"HTML": true, "UUID": true, "IP": true, "SQL": true, "CPU": true, "TTL": true, "UI": true, "OS": true,
}

func goField(key string) string {
	var b strings.Builder
	for _, w := range words(key) {
		if up := strings.ToUpper(w); goInitialisms[up] {
			b.WriteString(up)
		} else {
			b.WriteString(text.TitleWord(w))
		}
	}
	return safeIdent(b.String(), "Field")
}

func csField(key string) string {
	var b strings.Builder
	for _, w := range words(key) {
		b.WriteString(text.TitleWord(w))
	}
	return safeIdent(b.String(), "Field")
}

// typeName turns a key or the user's name into an exported identifier.
func typeName(s, fallback string) string {
	return safeIdent(csField(s), fallback)
}

func safeIdent(s, fallback string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return r
		}
		return -1
	}, s)
	switch {
	case s == "":
		return fallback
	case unicode.IsDigit([]rune(s)[0]):
		return fallback + s
	}
	return s
}

func uniqueField(n string, used map[string]bool) string {
	out := n
	for i := 2; used[out]; i++ {
		out = fmt.Sprintf("%s%d", n, i)
	}
	used[out] = true
	return out
}

// singular makes an element type name from a plural key: Items → Item,
// Categories → Category. It leaves words ending in "ss" alone.
func singular(n string) string {
	switch {
	case strings.HasSuffix(n, "ies") && len(n) > 4:
		return n[:len(n)-3] + "y"
	case strings.HasSuffix(n, "ss"):
		return n + "Item"
	case strings.HasSuffix(n, "s") && len(n) > 2:
		return n[:len(n)-1]
	}
	return n + "Item"
}
