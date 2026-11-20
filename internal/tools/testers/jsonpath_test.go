package testers_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/testers"
)

func runJSONPath(t *testing.T, in, path string) (tool.Result, error) {
	t.Helper()
	return testers.JSONPath{}.Run(context.Background(), tool.Request{
		Inputs: map[string]string{"input": in},
		Opts:   tool.Values{"path": path},
	})
}

func TestJSONPathGolden(t *testing.T) {
	in := testutil.ReadInput(t, "jsonpath", "store")
	for _, tt := range []struct{ name, path string }{
		{"store-authors", "$.store.book[*].author"},
		{"store-cheap-titles", "$.store.book[?(@.price < 10)].title"},
		{"store-all-prices", "$..price"},
		{"store-last-book", "$.store.book[-1]"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runJSONPath(t, in, tt.path)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "jsonpath", tt.name, res.Text)
		})
	}
}

func TestJSONPath(t *testing.T) {
	tests := []struct {
		name, in, path, want, matches string
	}{
		{"root", `{"a":1}`, "$", "[\n  {\n    \"a\": 1\n  }\n]", "1"},
		{"empty path is root", `[1]`, "  ", "[\n  [\n    1\n  ]\n]", "1"},
		{"no matches", `{"a":1}`, "$.b", "[]", "0"},
		{"bracket key", `{"a b":"x"}`, "$['a b']", "[\n  \"x\"\n]", "1"},
		{"slice", `[1,2,3,4]`, "$[1:3]", "[\n  2,\n  3\n]", "2"},
		{"no html escaping", `{"a":"<&>"}`, "$.a", "[\n  \"<&>\"\n]", "1"},
		{"keeps key order", `{"z":1,"a":2,"m":3}`, "$.*", "[\n  1,\n  2,\n  3\n]", "3"},
		{"repeated key keeps last value", `{"a":1,"b":2,"a":3}`, "$", "[\n  {\n    \"a\": 3,\n    \"b\": 2\n  }\n]", "1"},
		{"integer filter", `[{"n":1},{"n":5},{"n":10}]`, "$[?(@.n > 3)].n", "[\n  5,\n  10\n]", "2"},
		{"empty input", "  ", "$.a", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runJSONPath(t, tt.in, tt.path)
			if err != nil {
				t.Fatal(err)
			}
			if res.Text != tt.want {
				t.Errorf("Text = %q, want %q", res.Text, tt.want)
			}
			if tt.matches != "" && res.Meta[0].Value != tt.matches {
				t.Errorf("Matches = %s, want %s", res.Meta[0].Value, tt.matches)
			}
			if res.Lang != "json" {
				t.Errorf("Lang = %q", res.Lang)
			}
		})
	}
}

func TestJSONPathErrors(t *testing.T) {
	tests := []struct {
		name, in, path string
		input          string
		line, col      int
	}{
		{"bad json", "{\n  \"a\": 1,\n}", "$", "input", 3, 1},
		{"unclosed json", `{"a": [1, 2`, "$", "input", 1, 12},
		{"data after the value", `{} []`, "$", "input", 1, 4},
		{"bad path", `{}`, "$.a[?(@.b ==", "path", 1, 0},
		{"path parse error", `{}`, "$.a]", "path", 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runJSONPath(t, tt.in, tt.path)
			ie, ok := tool.AsInputError(err)
			if !ok {
				t.Fatalf("error = %v, want *InputError", err)
			}
			t.Logf("%d:%d %s", ie.Line, ie.Col, ie.Msg)
			if ie.Input != tt.input || ie.Line != tt.line || (tt.col > 0 && ie.Col != tt.col) || ie.Col < 1 {
				t.Errorf("error = %s %d:%d %q, want %s %d:%d", ie.Input, ie.Line, ie.Col, ie.Msg, tt.input, tt.line, tt.col)
			}
			if strings.Contains(ie.Msg, " in $") {
				t.Errorf("the message repeats the path: %q", ie.Msg)
			}
		})
	}
}

// FuzzJSONPath checks that no document or path makes Run panic or return an
// error that is not an InputError.
func FuzzJSONPath(f *testing.F) {
	f.Add(`{"a":[1,{"b":2}]}`, "$..b")
	f.Add(`[1,2,3]`, "$[?(@ > 1)]")
	f.Add(`{"a":1}`, "$['a','b'][0:2]")
	f.Fuzz(func(t *testing.T, in, path string) {
		_, err := runJSONPath(t, in, path)
		if err != nil {
			if _, ok := tool.AsInputError(err); !ok {
				t.Fatalf("error that is not *InputError: %v", err)
			}
		}
	})
}
