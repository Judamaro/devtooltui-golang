package text_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/text"
)

func runDiff(t testing.TB, a, b string, opts tool.Values) (tool.Result, error) {
	t.Helper()
	return text.Diff{}.Run(context.Background(), tool.Request{
		Inputs: map[string]string{"a": a, "b": b},
		Opts:   opts,
	})
}

func TestDiffGolden(t *testing.T) {
	tests := []struct {
		name string
		opts tool.Values
	}{
		{"go-unified", nil},
		{"go-side-by-side", tool.Values{"view": "side-by-side"}},
		{"go-words", tool.Values{"by": "words"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runDiff(t, testutil.ReadInput(t, "diff", "a"), testutil.ReadInput(t, "diff", "b"), tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "diff", tt.name, res.Text)
		})
	}
}

func TestDiff(t *testing.T) {
	tests := []struct {
		name, a, b string
		opts       tool.Values
		want       string
		meta       string
	}{
		{"identicos", "x\ny\n", "x\ny\n", nil, "Sin diferencias", "idénticos"},
		{"vacios", "", "", nil, "Sin diferencias", "idénticos"},
		{"agrega linea", "a\n", "a\nb\n", nil, "--- A\n+++ B\n@@ -1,1 +1,2 @@\n a\n+b", "+1 −0 líneas"},
		{"desde vacio", "", "x", nil, "--- A\n+++ B\n@@ -0,0 +1,1 @@\n+x", "+1 −0 líneas"},
		{"ignora espacios", "a  b\n", "a b\n", tool.Values{"ignore_space": true}, "Sin diferencias", "idénticos"},
		{"sin ignorar espacios", "a  b\n", "a b\n", nil, "-a  b\n+a b", "+1 −1 líneas"},
		{"palabras", "el gato negro", "el perro negro", tool.Values{"by": "words"}, "el [-gato-]{+perro+} negro", "+1 −1 palabras"},
		{"palabras ignora espacios", "a  b", "a b", tool.Values{"by": "words", "ignore_space": true}, "a  b", "idénticos"},
		{"lado a lado", "a\nb\n", "a\nc\nd\n", tool.Values{"view": "side-by-side"}, "  a          │ a\n| b          │ c\n>            │ d", "+2 −1 líneas"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runDiff(t, tt.a, tt.b, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(res.Text, tt.want) {
				t.Errorf("Text =\n%s\nno contiene\n%s", res.Text, tt.want)
			}
			if res.Meta[0].Value != tt.meta {
				t.Errorf("Meta = %q, want %q", res.Meta[0].Value, tt.meta)
			}
		})
	}
}

// Las operaciones reconstruyen A (Equal+Delete) y B (Equal+Insert).
func FuzzDiffReconstructs(f *testing.F) {
	f.Add("a\nb\nc\n", "a\nx\nc", false, false)
	f.Add("el gato", "el  perro", true, true)
	f.Fuzz(func(t *testing.T, a, b string, words, ignore bool) {
		by := "lines"
		if words {
			by = "words"
		}
		res, err := runDiff(t, a, b, tool.Values{"by": by, "ignore_space": ignore})
		if err != nil {
			t.Fatal(err)
		}
		var ga, gb strings.Builder
		for _, op := range res.Diff {
			if op.Kind != tool.Insert {
				ga.WriteString(op.Text)
			}
			if op.Kind != tool.Delete {
				gb.WriteString(op.Text)
			}
		}
		gotA, gotB, wantA, wantB := ga.String(), gb.String(), a, b
		if !words { // por líneas, el "\n" final no cuenta
			gotA, gotB = strings.TrimSuffix(gotA, "\n"), strings.TrimSuffix(gotB, "\n")
			wantA, wantB = strings.TrimSuffix(a, "\n"), strings.TrimSuffix(b, "\n")
		}
		if gotA != wantA || (!ignore && gotB != wantB) {
			t.Errorf("no reconstruye:\nA %q → %q\nB %q → %q", wantA, gotA, wantB, gotB)
		}
	})
}
