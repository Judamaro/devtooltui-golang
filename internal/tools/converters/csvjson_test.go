package converters_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/converters"
)

func runCSV(t testing.TB, in string, opts tool.Values) (tool.Result, error) {
	t.Helper()
	return converters.CSVJSON{}.Run(context.Background(), tool.Request{Inputs: map[string]string{"input": in}, Opts: opts})
}

func TestCSVJSONGolden(t *testing.T) {
	for _, tt := range []struct {
		name, in string
		opts     tool.Values
	}{
		{"people-to-json", "people", nil},
		{"people-to-json-types", "people", tool.Values{"types": true}},
		{"records-to-csv", "records", tool.Values{"mode": "to-csv"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runCSV(t, testutil.ReadInput(t, "csv-json", tt.in), tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "csv-json", tt.name, res.Text)
		})
	}
}

func TestCSVJSON(t *testing.T) {
	tests := []struct {
		name, in string
		opts     tool.Values
		want     string
	}{
		{"no header", "a,b\n1,2\n", tool.Values{"header": false}, "[\n  [\"a\", \"b\"],\n  [\"1\", \"2\"]\n]"},
		{"header only", "a,b\n", nil, "[]"},
		{"semicolon", "a;b\n1;2", tool.Values{"delimiter": "semicolon"}, "[\n  {\"a\": \"1\", \"b\": \"2\"}\n]"},
		{"tab", "a\tb\n1\t2", tool.Values{"delimiter": "tab"}, "[\n  {\"a\": \"1\", \"b\": \"2\"}\n]"},
		{"repeated and empty headers", "a,a,\n1,2,3", nil, "[\n  {\"a\": \"1\", \"a_2\": \"2\", \"column_3\": \"3\"}\n]"},
		{"types", "n,f,b,e,s\n-1.5e3,0,false,,007x", tool.Values{"types": true}, "[\n  {\"n\": -1.5e3, \"f\": 0, \"b\": false, \"e\": null, \"s\": \"007x\"}\n]"},
		{"no html escaping", "a\n<&>", nil, "[\n  {\"a\": \"<&>\"}\n]"},
		{"to csv arrays", "[[1, \"x\"], [true, null]]", tool.Values{"mode": "to-csv"}, "1,x\ntrue,"},
		{"to csv without header", `[{"a": 1}]`, tool.Values{"mode": "to-csv", "header": false}, "1"},
		{"to csv quotes", `[{"a": "x,y", "b": "say \"hi\""}]`, tool.Values{"mode": "to-csv"}, "a,b\n\"x,y\",\"say \"\"hi\"\"\""},
		{"to csv pipe", `[{"a": 1, "b": 2}]`, tool.Values{"mode": "to-csv", "delimiter": "pipe"}, "a|b\n1|2"},
		{"to csv nested", `[{"o": {"k": [1, 2]}}]`, tool.Values{"mode": "to-csv"}, "o\n\"{\"\"k\"\":[1,2]}\""},
		{"to csv empty array", `[]`, tool.Values{"mode": "to-csv"}, ""},
		{"to csv single empty field", `[[""], ["x"]]`, tool.Values{"mode": "to-csv"}, "\"\"\nx"},
		{"invalid UTF-8 headers stay apart", "\xff,\xfe\n1,2", nil, "[\n  {\"\ufffd\": \"1\", \"\ufffd_2\": \"2\"}\n]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runCSV(t, tt.in, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if res.Text != tt.want {
				t.Errorf("got %q, want %q", res.Text, tt.want)
			}
		})
	}
}

func TestCSVJSONErrors(t *testing.T) {
	tests := []struct {
		name, in  string
		opts      tool.Values
		line, col int
		msg       string
	}{
		{"field count", "a,b\n1,2\n3\n", nil, 3, 1, "different number of fields"},
		{"bare quote", "a,b\n1,x\"y\n", nil, 2, 4, "bare \""},
		{"unclosed quote", "a\n\"abc\n", nil, 2, 1, "quoted field not closed"},
		{"json syntax", "[{\"a\": 1,}]", tool.Values{"mode": "to-csv"}, 1, 10, "invalid character"},
		{"not an array", `{"a": 1}`, tool.Values{"mode": "to-csv"}, 1, 1, "expected a JSON array"},
		{"mixed elements", `[{"a": 1}, [1]]`, tool.Values{"mode": "to-csv"}, 1, 1, "element 1"},
		{"scalar element", `[1]`, tool.Values{"mode": "to-csv"}, 1, 1, "element 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runCSV(t, tt.in, tt.opts)
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

// FuzzCSVJSONRoundTrip: CSV → JSON → CSV → JSON gives the same JSON.
func FuzzCSVJSONRoundTrip(f *testing.F) {
	for _, s := range []string{"a,b\n1,2", "x\n\"q,\"\"\"\n", "a;b\n;", "é,ü\n🍕,\t"} {
		f.Add(s, true)
		f.Add(s, false)
	}
	f.Fuzz(func(t *testing.T, s string, header bool) {
		opts := tool.Values{"header": header}
		j1, err := runCSV(t, s, opts)
		if err != nil {
			if _, ok := tool.AsInputError(err); !ok {
				t.Fatalf("error that is not *InputError: %v", err)
			}
			return
		}
		if j1.Text == "" || j1.Text == "[]" {
			return
		}
		c, err := runCSV(t, j1.Text, tool.Values{"mode": "to-csv", "header": header})
		if err != nil {
			t.Fatalf("to-csv(%q): %v", j1.Text, err)
		}
		j2, err := runCSV(t, c.Text, opts)
		if err != nil {
			t.Fatalf("to-json(%q): %v", c.Text, err)
		}
		if j2.Text != j1.Text {
			t.Errorf("round trip:\n%q\n→ %q\n→ %q\n→ %q", s, j1.Text, c.Text, j2.Text)
		}
	})
}
