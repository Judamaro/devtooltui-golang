package formatters_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/formatters"
)

func runJSON(t testing.TB, in string, opts tool.Values) (tool.Result, error) {
	t.Helper()
	return formatters.JSONFormat{}.Run(context.Background(), tool.Request{
		Inputs: map[string]string{"input": in},
		Opts:   opts,
	})
}

func TestJSONFormatGolden(t *testing.T) {
	tests := []struct {
		name, in string
		opts     tool.Values
	}{
		{"nested-indent2", "nested", nil},
		{"nested-indent4", "nested", tool.Values{"indent": 4}},
		{"nested-minify", "nested", tool.Values{"indent": 0}},
		{"nested-sorted", "nested", tool.Values{"sort_keys": true}},
		{"nested-sorted-minify", "nested", tool.Values{"sort_keys": true, "indent": 0}},
		{"array-indent2", "array", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runJSON(t, testutil.ReadInput(t, "json-format", tt.in), tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "json-format", tt.name, res.Text)
		})
	}
}

func TestJSONFormat(t *testing.T) {
	tests := []struct {
		name string
		in   string
		opts tool.Values
		want string
	}{
		{"vacio", "", nil, ""},
		{"solo espacios", "  \n ", nil, ""},
		{"escalar", " 42 ", nil, "42"},
		{"string", `"hola"`, nil, `"hola"`},
		{"conserva orden", `{"b":1,"a":2}`, tool.Values{"indent": 0}, `{"b":1,"a":2}`},
		{"ordena", `{"b":1,"a":2}`, tool.Values{"indent": 0, "sort_keys": true}, `{"a":2,"b":1}`},
		{"conserva numeros", `[1.50,1e10]`, tool.Values{"indent": 0}, `[1.50,1e10]`},
		{"ordenado conserva numeros", `[1.50,1e10]`, tool.Values{"indent": 0, "sort_keys": true}, `[1.50,1e10]`},
		{"sin escapar html al ordenar", `{"a":"<&>"}`, tool.Values{"indent": 0, "sort_keys": true}, `{"a":"<&>"}`},
		{"indent 1", `{"a":[1]}`, tool.Values{"indent": 1}, "{\n \"a\": [\n  1\n ]\n}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runJSON(t, tt.in, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if res.Text != tt.want {
				t.Errorf("Text = %q, want %q", res.Text, tt.want)
			}
		})
	}
}

func TestJSONFormatErrors(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		sorted    bool
		line, col int
	}{
		{"coma de mas", `{"a":1,}`, false, 1, 8},
		{"coma de mas ordenado", `{"a":1,}`, true, 1, 8},
		{"segunda linea", "{\n  \"a\": tru\n}", false, 2, 11},
		{"sin cerrar", `{"a":[1,2`, false, 1, 10},
		{"sin cerrar ordenado", `{"a":[1,2`, true, 1, 10},
		{"dos valores", `{} {}`, false, 1, 4},
		{"dos valores ordenado", `{} {}`, true, 1, 4},
		{"comilla simple", `{'a':1}`, false, 1, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runJSON(t, tt.in, tool.Values{"sort_keys": tt.sorted})
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

func TestJSONFormatDetect(t *testing.T) {
	tests := []struct {
		name   string
		sample string
		want   float64
	}{
		{"objeto", ` {"a":1} `, 0.9},
		{"arreglo", `[1,2]`, 0.9},
		{"escalar no", `42`, 0},
		{"invalido", `{"a":}`, 0},
		{"texto", `hola`, 0},
		{"vacio", ``, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (formatters.JSONFormat{}).Detect(tt.sample); got != tt.want {
				t.Errorf("Detect = %v, want %v", got, tt.want)
			}
		})
	}
}

// FuzzJSONFormatIdempotent: formatear dos veces da lo mismo que una.
func FuzzJSONFormatIdempotent(f *testing.F) {
	for _, s := range []string{`{"b":1,"a":[1,2,{"c":null}]}`, `[]`, `"x"`, `1.5e3`, `{"a":"<é>"}`} {
		f.Add(s, 2, false)
	}
	f.Fuzz(func(t *testing.T, in string, indent int, sorted bool) {
		if !json.Valid([]byte(in)) {
			return
		}
		opts := tool.Values{"indent": ((indent % 9) + 9) % 9, "sort_keys": sorted}
		once, err := runJSON(t, in, opts)
		if err != nil {
			t.Fatalf("JSON válido rechazado: %v", err)
		}
		twice, err := runJSON(t, once.Text, opts)
		if err != nil {
			t.Fatalf("la salida no es JSON válido: %v\n%s", err, once.Text)
		}
		if once.Text != twice.Text {
			t.Errorf("no es idempotente:\n%s\n---\n%s", once.Text, twice.Text)
		}
	})
}

func genJSON(n int) string {
	var b strings.Builder
	b.WriteString("[")
	for i := 0; b.Len() < n; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"id":%d,"name":"elemento-%06d","tags":["a","bb"],"valid":%t,"score":%d.5}`, i, i, i%2 == 0, i)
	}
	b.WriteString("]")
	return b.String()
}

// BenchmarkJSONFormat1MB: el objetivo del plan es menos de 30 ms.
func BenchmarkJSONFormat1MB(b *testing.B) {
	in := genJSON(1 << 20)
	for _, tc := range []struct {
		name string
		opts tool.Values
	}{
		{"indent", nil},
		{"minify", tool.Values{"indent": 0}},
		{"sorted", tool.Values{"sort_keys": true}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(in)))
			for b.Loop() {
				if _, err := runJSON(b, in, tc.opts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
