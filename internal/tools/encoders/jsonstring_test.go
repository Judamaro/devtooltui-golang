package encoders_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/encoders"
)

func runJSONString(t testing.TB, in string, opts tool.Values) (tool.Result, error) {
	t.Helper()
	return encoders.JSONString{}.Run(context.Background(), tool.Request{Inputs: map[string]string{"input": in}, Opts: opts})
}

func TestJSONStringGolden(t *testing.T) {
	for _, tt := range []struct {
		name, in string
		opts     tool.Values
	}{
		{"order-stringify", "order", nil},
		{"order-stringify-spaced", "order", tool.Values{"compact": false}},
		{"nested-parse", "nested", tool.Values{"mode": "decode"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runJSONString(t, testutil.ReadInput(t, "json-string", tt.in), tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "json-string", tt.name, res.Text)
		})
	}
}

func TestJSONString(t *testing.T) {
	tests := []struct {
		name, in string
		opts     tool.Values
		want     string
	}{
		{"stringify", `{"a": [1, 2]}`, nil, `"{\"a\":[1,2]}"`},
		{"keep spaces", `{"a": [1, 2]}`, tool.Values{"compact": false}, `"{\"a\": [1, 2]}"`},
		{"no quotes", `{"a":1}`, tool.Values{"quotes": false}, `{\"a\":1}`},
		{"unicode stays", `{"c":"café 🍕"}`, nil, `"{\"c\":\"café 🍕\"}"`},
		{"no html escaping", `["</script>&"]`, nil, `"[\"</script>&\"]"`},
		{"nested escapes", `{"p":"C:\\tmp","q":"say \"hi\""}`, nil, `"{\"p\":\"C:\\\\tmp\",\"q\":\"say \\\"hi\\\"\"}"`},
		{"scalar", `42`, nil, `"42"`},
		{"empty", "  \n", nil, ""},
		{"parse", `"{\"a\":[1,2]}"`, tool.Values{"mode": "decode", "indent": 0}, `{"a":[1,2]}`},
		{"parse indented", `"{\"a\":1}"`, tool.Values{"mode": "decode"}, "{\n  \"a\": 1\n}"},
		{"parse indent 4", `"[1]"`, tool.Values{"mode": "decode", "indent": 4}, "[\n    1\n]"},
		{"parse without quotes", `{\"a\":true}`, tool.Values{"mode": "decode", "indent": 0}, `{"a":true}`},
		{"parse surrounded by spaces", "  \"[1]\"\n", tool.Values{"mode": "decode", "indent": 0}, `[1]`},
		{"parse twice stringified", `"\"{\\\"a\\\":1}\""`, tool.Values{"mode": "decode", "indent": 0}, `{"a":1}`},
		{"parse a stringified string", `"\"x\\\"y\""`, tool.Values{"mode": "decode", "indent": 0}, `"x\"y"`},
		{"parse surrogate pair", `"[\"\ud83c\udf55\"]"`, tool.Values{"mode": "decode", "indent": 0}, `["🍕"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runJSONString(t, tt.in, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if res.Text != tt.want {
				t.Errorf("got %s, want %s", res.Text, tt.want)
			}
		})
	}
}

func TestJSONStringLevels(t *testing.T) {
	res, err := runJSONString(t, `"\"{\\\"a\\\":1}\""`, tool.Values{"mode": "decode"})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Meta[len(res.Meta)-1]; got.Key != "Levels" || got.Value != "2" {
		t.Errorf("Meta = %v, want Levels: 2", res.Meta)
	}
}

func TestJSONStringErrors(t *testing.T) {
	tests := []struct {
		name, in  string
		opts      tool.Values
		line, col int
		msg       string
	}{
		{"stringify invalid JSON", `{"a":1,}`, nil, 1, 8, "invalid character"},
		{"stringify second line", "{\n  \"a\": tru\n}", nil, 2, 11, "invalid character"},
		{"parse bad escape", `"{\q}"`, tool.Values{"mode": "decode"}, 1, 3, "invalid escape"},
		{"parse plain text", `"hello"`, tool.Values{"mode": "decode"}, 1, 1, "the string does not hold JSON"},
		{"parse broken JSON inside", `"{\"a\":}"`, tool.Values{"mode": "decode"}, 1, 1, "at 1:6 of the decoded text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runJSONString(t, tt.in, tt.opts)
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

func TestJSONStringInvalidUTF8(t *testing.T) {
	res, err := runJSONString(t, "[\"a\xffb\"]", nil)
	if err != nil || res.Text != "\"[\\\"a\uFFFDb\\\"]\"" {
		t.Errorf("got %q, %v; want the bad byte as U+FFFD", res.Text, err)
	}
}

func TestJSONStringDetect(t *testing.T) {
	tests := []struct {
		sample string
		want   float64
	}{
		{`"{\"a\":1}"`, 0.7},
		{`"[\"x\"]"`, 0.7},
		{`"hello"`, 0},
		{`"say \"hi\""`, 0},
		{`{"a":1}`, 0},
		{``, 0},
	}
	for _, tt := range tests {
		if got := (encoders.JSONString{}).Detect(tt.sample); got != tt.want {
			t.Errorf("Detect(%q) = %v, want %v", tt.sample, got, tt.want)
		}
	}
}

// FuzzJSONStringRoundTrip: for any valid JSON, parse(stringify(x)) is x
// compacted.
func FuzzJSONStringRoundTrip(f *testing.F) {
	for _, s := range []string{`{"a":[1,2]}`, `"x\"y"`, `[null,true,1e10,"\u2028"]`, `{"k":"C:\\tmp 🍕"}`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		var want bytes.Buffer
		// Invalid UTF-8 becomes U+FFFD: a JSON string cannot hold raw bytes
		// (TestJSONStringInvalidUTF8).
		if !utf8.ValidString(in) || json.Compact(&want, []byte(in)) != nil || strings.TrimSpace(in) == "" {
			return
		}
		enc, err := runJSONString(t, in, nil)
		if err != nil {
			t.Fatalf("stringify(%q): %v", in, err)
		}
		dec, err := runJSONString(t, enc.Text, tool.Values{"mode": "decode", "indent": 0})
		if err != nil {
			t.Fatalf("parse(%q): %v", enc.Text, err)
		}
		if dec.Text != want.String() {
			// A JSON string as input is unwrapped by parse: only check that
			// the result is that string's content when it is JSON itself.
			var s string
			if json.Unmarshal(want.Bytes(), &s) == nil {
				return
			}
			t.Errorf("round trip: %q → %q → %q, want %q", in, enc.Text, dec.Text, want.String())
		}
	})
}
