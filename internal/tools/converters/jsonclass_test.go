package converters_test

import (
	"context"
	"go/format"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/converters"
)

func runJSONClass(t testing.TB, in string, opts tool.Values) (tool.Result, error) {
	t.Helper()
	return converters.JSONClass{}.Run(context.Background(), tool.Request{Inputs: map[string]string{"input": in}, Opts: opts})
}

func TestJSONClassGolden(t *testing.T) {
	in := testutil.ReadInput(t, "json-class", "order")
	for _, tt := range []struct {
		name string
		opts tool.Values
	}{
		{"order-go", tool.Values{"name": "Order"}},
		{"order-csharp-record", tool.Values{"lang": "csharp", "name": "Order"}},
		{"order-csharp-class", tool.Values{"lang": "csharp", "csharp": "class"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runJSONClass(t, in, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "json-class", tt.name, res.Text)
		})
	}
}

func TestJSONClass(t *testing.T) {
	tests := []struct {
		name, in string
		opts     tool.Values
		contains []string
	}{
		{"int and float merge", `[{"n": 1}, {"n": 2.5}]`, nil, []string{"type Root []RootItem", "N float64 `json:\"n\"`"}},
		{"null makes it optional", `{"a": null, "b": [1, null]}`, nil, []string{"A any `json:\"a,omitempty\"`", "B []*int64 `json:\"b\"`"}},
		{"missing in some objects", `[{"a": 1, "b": "x"}, {"a": 2}]`, tool.Values{"omitempty": false}, []string{"A int64  `json:\"a\"`", "B *string `json:\"b\"`"}},
		{"mixed types", `{"x": [1, "a"]}`, nil, []string{"X []any"}},
		{"empty object is a map", `{"m": {}}`, nil, []string{"M map[string]any"}},
		{"csharp empty object", `{"m": {}}`, tool.Values{"lang": "csharp"}, []string{"public Dictionary<string, object> M", "using System.Collections.Generic;"}},
		{"empty array", `{"x": []}`, nil, []string{"X []any"}},
		{"initialisms", `{"user_id": 1, "api_url": "u", "html": "h"}`, nil, []string{"UserID int64", "APIURL string", "HTML   string"}},
		{"bad identifiers", `{"1st": 1, "": 2, "a b": 3, "a-b": 4}`, nil, []string{"Field1st", "Field  int64", "AB     int64", "AB2    int64"}},
		{"nested names", `{"user": {"address": {"city": "x"}}, "address": {"zip": "1"}}`, nil, []string{"type User struct", "type Address struct", "type Address2 struct"}},
		{"plural arrays", `{"categories": [{"a": 1}], "boxes": [{"b": 1}], "class": [{"c": 1}]}`, nil, []string{"[]Category", "[]Boxe", "[]ClassItem"}},
		{"root name", `{"a": 1}`, tool.Values{"name": "my order"}, []string{"type MyOrder struct"}},
		{"csharp nullable", `[{"a": 1}, {"b": "x"}]`, tool.Values{"lang": "csharp"}, []string{"public long? A { get; init; }", "public string? B { get; init; }", "// The JSON is a List<RootItem>."}},
		{"csharp lists", `{"tags": ["a"]}`, tool.Values{"lang": "csharp"}, []string{"using System.Collections.Generic;", "public List<string> Tags"}},
		{"csharp member named like the type", `{"root": 1}`, tool.Values{"lang": "csharp"}, []string{"public long Root2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runJSONClass(t, tt.in, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			norm := func(s string) string { return strings.Join(strings.Fields(s), " ") }
			for _, want := range tt.contains {
				if !strings.Contains(norm(res.Text), norm(want)) {
					t.Errorf("missing %q in:\n%s", want, res.Text)
				}
			}
		})
	}
}

func TestJSONClassErrors(t *testing.T) {
	tests := []struct {
		in        string
		line, col int
		msg       string
	}{
		{`{"a": 1,}`, 1, 9, "invalid character"},
		{`[1, 2]`, 1, 1, "expected a JSON object"},
		{`"text"`, 1, 1, "expected a JSON object"},
		{`[]`, 1, 1, "expected a JSON object"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			_, err := runJSONClass(t, tt.in, nil)
			ie, ok := tool.AsInputError(err)
			if !ok {
				t.Fatalf("error = %v, want *InputError", err)
			}
			if ie.Line != tt.line || ie.Col != tt.col || !strings.Contains(ie.Msg, tt.msg) {
				t.Errorf("error = %d:%d %q, want %d:%d %q", ie.Line, ie.Col, ie.Msg, tt.line, tt.col, tt.msg)
			}
		})
	}
}

// FuzzJSONClass: any JSON object gives Go that gofmt accepts.
func FuzzJSONClass(f *testing.F) {
	for _, s := range []string{`{"a":1}`, `[{"a":[{"b":null}]},{"c":true}]`, `{"1":{"":[]},"x y":"z"}`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		res, err := runJSONClass(t, in, nil)
		if err != nil {
			if _, ok := tool.AsInputError(err); !ok {
				t.Fatalf("error that is not *InputError: %v", err)
			}
			return
		}
		if res.Text == "" {
			return
		}
		if _, err := format.Source([]byte(res.Text)); err != nil {
			t.Fatalf("invalid Go for %q: %v\n%s", in, err, res.Text)
		}
	})
}
