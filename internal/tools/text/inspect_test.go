package text_test

import (
	"context"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/text"
)

func runInspect(t *testing.T, in string, opts tool.Values) tool.Result {
	t.Helper()
	res, err := text.Inspect{}.Run(context.Background(), tool.Request{Inputs: map[string]string{"input": in}, Opts: opts})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestInspectGolden(t *testing.T) {
	in := testutil.ReadInput(t, "inspect", "fruits")
	for _, tt := range []struct {
		name string
		opts tool.Values
	}{
		{"fruits-stats", nil},
		{"fruits-sort", tool.Values{"action": "sort"}},
		{"fruits-dedupe-ignore-case", tool.Values{"action": "dedupe", "ignore_case": true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			testutil.Golden(t, "inspect", tt.name, runInspect(t, in, tt.opts).Text)
		})
	}
}

func TestInspectTransforms(t *testing.T) {
	tests := []struct {
		name string
		in   string
		opts tool.Values
		want string
	}{
		{"sort", "b\na\nB\nA", tool.Values{"action": "sort"}, "A\nB\na\nb"},
		{"sort ignore case", "b\na\nB\nA", tool.Values{"action": "sort", "ignore_case": true}, "A\na\nB\nb"},
		{"sort keeps trailing newline", "b\na\n", tool.Values{"action": "sort"}, "a\nb\n"},
		{"sort CRLF", "b\r\na\r\n", tool.Values{"action": "sort"}, "a\r\nb\r\n"},
		{"dedupe keeps first", "a\nb\na\nc\nb", tool.Values{"action": "dedupe"}, "a\nb\nc"},
		{"dedupe ignore case", "Go\ngo\nGO\nrust", tool.Values{"action": "dedupe", "ignore_case": true}, "Go\nrust"},
		{"reverse", "1\n2\n3\n", tool.Values{"action": "reverse"}, "3\n2\n1\n"},
		{"empty", "", tool.Values{"action": "sort"}, ""},
		{"only newline", "\n", tool.Values{"action": "reverse"}, "\n"},
		{"mixed endings keep CR", "b\r\na\n", tool.Values{"action": "sort"}, "a\nb\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runInspect(t, tt.in, tt.opts).Text; got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInspectDedupeMeta(t *testing.T) {
	res := runInspect(t, "a\na\nb", tool.Values{"action": "dedupe"})
	if res.Meta[0].Value != "3 → 2" || res.Meta[1].Value != "1" {
		t.Errorf("Meta = %v", res.Meta)
	}
}

func TestInspectStats(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want map[string]string // Meta
		row  string            // a line the stats must contain
	}{
		{"empty", "", map[string]string{"Characters": "0", "Words": "0", "Lines": "0"}, "Line endings: none"},
		{"no final newline", "one two\nthree", map[string]string{"Words": "3", "Lines": "2"}, "Line endings: LF (1)"},
		{"final newline", "a\n", map[string]string{"Lines": "1"}, "Encoding:     ASCII"},
		{"crlf", "a\r\nb\r\n", map[string]string{"Lines": "2"}, "Line endings: CRLF (2)"},
		{"mixed", "a\r\nb\nc\r", nil, "Line endings: mixed: LF (1), CRLF (1), CR (1)"},
		{"runes", "ñandú 🍕", map[string]string{"Characters": "7"}, "Encoding:     UTF-8"},
		{"bom", "\uFEFFhi", nil, "Encoding:     UTF-8 with BOM"},
		{"invalid", "ok\nx\xffy", nil, "Encoding:     not valid UTF-8 (first bad byte at 2:2)"},
		{"longest", "ab\nabcd\nabc", nil, "Longest line: 4 characters (line 2)"},
		{"blank and unique", "a\n\n  \na\n", nil, "Blank lines:  2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := runInspect(t, tt.in, nil)
			meta := map[string]string{}
			for _, kv := range res.Meta {
				meta[kv.Key] = kv.Value
			}
			for k, v := range tt.want {
				if meta[k] != v {
					t.Errorf("%s = %q, want %q", k, meta[k], v)
				}
			}
			if !containsLine(res.Text, tt.row) {
				t.Errorf("stats without %q:\n%s", tt.row, res.Text)
			}
		})
	}
}

func containsLine(text, line string) bool {
	for _, l := range splitNL(text) {
		if l == line {
			return true
		}
	}
	return false
}

func splitNL(s string) []string {
	var out []string
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
