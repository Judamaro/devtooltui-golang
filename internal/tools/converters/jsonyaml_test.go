package converters_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/converters"
)

func runJY(t testing.TB, in string, opts tool.Values) (tool.Result, error) {
	t.Helper()
	return converters.JSONYAML{}.Run(context.Background(), tool.Request{
		Inputs: map[string]string{"input": in},
		Opts:   opts,
	})
}

func TestJSONYAMLGolden(t *testing.T) {
	tests := []struct {
		name, in string
		opts     tool.Values
	}{
		{"config-to-yaml", "config", nil},
		{"config-to-yaml-indent4", "config", tool.Values{"indent": 4}},
		{"k8s-to-json", "k8s", tool.Values{"mode": "to-json"}},
		{"k8s-to-json-minify", "k8s", tool.Values{"mode": "to-json", "indent": 0}},
		{"multi-to-json", "multi", tool.Values{"mode": "to-json"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runJY(t, testutil.ReadInput(t, "json-yaml", tt.in), tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "json-yaml", tt.name, res.Text)
		})
	}
}

func TestJSONYAML(t *testing.T) {
	toJSON := tool.Values{"mode": "to-json", "indent": 0}
	tests := []struct {
		name, in string
		opts     tool.Values
		want     string
		lang     string
	}{
		{"vacio", "  ", nil, "", ""},
		{"orden de claves a yaml", `{"b":1,"a":2}`, nil, "b: 1\na: 2\n", "yaml"},
		{"string que parece bool", `{"a":"true"}`, nil, "a: \"true\"\n", "yaml"},
		{"string que parece numero", `["1"]`, nil, "- \"1\"\n", "yaml"},
		{"bloque conserva salto final", `{"s":"a\n"}`, nil, "s: |\n  a\n", "yaml"},
		{"orden de claves a json", "b: 1\na: 2", toJSON, `{"b":1,"a":2}`, "json"},
		{"tipos", "a: true\nb: ~\nc: 1.0\nd: hola", toJSON, `{"a":true,"b":null,"c":1.0,"d":"hola"}`, "json"},
		{"yes es string en YAML 1.2", "a: yes", toJSON, `{"a":"yes"}`, "json"},
		{"int grande sin perder precision", "n: 12345678901234567890", toJSON, `{"n":12345678901234567890}`, "json"},
		{"hex", "n: 0x1F", toJSON, `{"n":31}`, "json"},
		{"merge no pisa", "base: &b {x: 1, y: 2}\nhijo:\n  <<: *b\n  y: 3", toJSON, `{"base":{"x":1,"y":2},"hijo":{"y":3,"x":1}}`, "json"},
		{"merge de lista", "a: &a {x: 1}\nb: &b {y: 2}\nc:\n  <<: [*a, *b]", toJSON, `{"a":{"x":1},"b":{"y":2},"c":{"x":1,"y":2}}`, "json"},
		{"sin escapar html", "a: <b>&</b>", toJSON, `{"a":"<b>&</b>"}`, "json"},
		{"escalar suelto", "hola", toJSON, `"hola"`, "json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runJY(t, tt.in, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if res.Text != tt.want || res.Lang != tt.lang {
				t.Errorf("= %q (%s), want %q (%s)", res.Text, res.Lang, tt.want, tt.lang)
			}
		})
	}
}

func TestJSONYAMLErrors(t *testing.T) {
	toJSON := tool.Values{"mode": "to-json"}
	tests := []struct {
		name, in  string
		opts      tool.Values
		line, col int
	}{
		{"json invalido", "{\n  \"a\": tru\n}", nil, 2, 11},
		{"json sin cerrar", `{"a":1`, nil, 1, 7},
		{"json dos valores", `{} {}`, nil, 1, 4},
		{"yaml mal indentado", "a:\n  b: 1\n c: 2", toJSON, 2, 0},
		{"yaml clave compuesta", "? [a, b]\n: 1", toJSON, 1, 3},
		{"yaml infinito", "a: .inf", toJSON, 1, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runJY(t, tt.in, tt.opts)
			ie, ok := tool.AsInputError(err)
			if !ok {
				t.Fatalf("error = %v, want *InputError", err)
			}
			if ie.Line != tt.line || ie.Col != tt.col {
				t.Errorf("posición = %d:%d, want %d:%d (%s)", ie.Line, ie.Col, tt.line, tt.col, ie.Msg)
			}
		})
	}
}

func TestJSONYAMLDetect(t *testing.T) {
	tests := []struct {
		name, sample string
		want         float64
	}{
		{"mapa", "a: 1\nb:\n  c: 2", 0.40},
		{"lista de mapas", "- name: a\n  v: 1", 0.40},
		{"json", `{"a": 1}`, 0},
		{"texto plano", "hola mundo", 0},
		{"url", "https://example.com", 0},
		{"vacio", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (converters.JSONYAML{}).Detect(tt.sample); got != tt.want {
				t.Errorf("Detect = %v, want %v", got, tt.want)
			}
		})
	}
}

// FuzzJSONYAMLRoundTrip: JSON → YAML → JSON conserva el valor.
func FuzzJSONYAMLRoundTrip(f *testing.F) {
	for _, s := range []string{`{"a":[1,"2",true,null]}`, `"x: y"`, `[{"":""}]`, `{"n":1e10,"s":"\u00e9\n"}`, `"yes"`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		var want any
		if json.Unmarshal([]byte(in), &want) != nil {
			return
		}
		y, err := runJY(t, in, nil)
		if err != nil {
			if _, ok := tool.AsInputError(err); ok {
				return // JSON que yaml rechaza (p. ej. caracteres de control)
			}
			t.Fatalf("a YAML: %v", err)
		}
		if strings.TrimSpace(in) == "" {
			return
		}
		back, err := runJY(t, y.Text, tool.Values{"mode": "to-json"})
		if err != nil {
			t.Fatalf("YAML generado inválido: %v\n%s", err, y.Text)
		}
		var got any
		if err := json.Unmarshal([]byte(back.Text), &got); err != nil {
			t.Fatalf("JSON final inválido: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ida y vuelta:\n%s\n→ %s\n→ %s", in, y.Text, back.Text)
		}
	})
}
