package testers_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/testers"
)

func runRegex(t testing.TB, text string, opts tool.Values) (tool.Result, error) {
	t.Helper()
	return testers.Regex{}.Run(context.Background(), tool.Request{
		Inputs: map[string]string{"text": text},
		Opts:   opts,
	})
}

func TestRegexGolden(t *testing.T) {
	tests := []struct {
		name string
		opts tool.Values
	}{
		{"dates-with-groups", tool.Values{"pattern": `(?P<year>\d{4})-(\d{2})-(\d{2})`}},
		{"levels-ignore-case", tool.Values{"pattern": `^\S+ \S+ (info|error)`, "ignore_case": true, "multiline": true}},
		{"optional-group", tool.Values{"pattern": `(\d+)(%)?`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runRegex(t, testutil.ReadInput(t, "regex", "log"), tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "regex", tt.name, res.Text)
		})
	}
}

func TestRegexSpans(t *testing.T) {
	res, err := runRegex(t, "a1 b22", tool.Values{"pattern": `([a-z])(\d+)`})
	if err != nil {
		t.Fatal(err)
	}
	want := []tool.Span{
		{Input: "text", Start: 0, End: 2}, {Input: "text", Start: 0, End: 1, Group: 1}, {Input: "text", Start: 1, End: 2, Group: 2},
		{Input: "text", Start: 3, End: 6}, {Input: "text", Start: 3, End: 4, Group: 1}, {Input: "text", Start: 4, End: 6, Group: 2},
	}
	if len(res.Spans) != len(want) {
		t.Fatalf("Spans = %v", res.Spans)
	}
	for i := range want {
		if res.Spans[i] != want[i] {
			t.Errorf("Spans[%d] = %+v, want %+v", i, res.Spans[i], want[i])
		}
	}
}

func TestRegex(t *testing.T) {
	tests := []struct {
		name, text string
		opts       tool.Values
		want       string // first line or fragment
	}{
		{"no pattern", "abc", nil, "Type a pattern"},
		{"no matches", "abc", tool.Values{"pattern": `\d`}, "0 matches"},
		{"one", "abc", tool.Values{"pattern": `b`}, "1 match\n#1  1:2–1:3  \"b\""},
		{"ignore case", "ABC", tool.Values{"pattern": `b`, "ignore_case": true}, "1 match"},
		{"dotall", "a\nb", tool.Values{"pattern": `a.b`, "dotall": true}, "#1  1:1–2:2"},
		{"without dotall does not cross lines", "a\nb", tool.Values{"pattern": `a.b`}, "0 matches"},
		{"columns in runes", "ñandú x", tool.Values{"pattern": `x`}, "#1  1:7–1:8"},
		{"trims long matches", strings.Repeat("a", 100), tool.Values{"pattern": `a+`}, `"…`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runRegex(t, tt.text, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(res.Text, tt.want) {
				t.Errorf("the output does not contain %q:\n%s", tt.want, res.Text)
			}
		})
	}
}

func TestRegexLimit(t *testing.T) {
	res, err := runRegex(t, strings.Repeat("x", testers.MaxMatches+50), tool.Values{"pattern": `x`})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Text, "limit") || res.Meta[0].Value != "10000" {
		t.Errorf("the limit was not applied: %s", res.Meta)
	}
}

func TestRegexErrors(t *testing.T) {
	tests := []struct {
		name, pattern string
		col           int
		hint          string
	}{
		{"unclosed parenthesis", `ab(c`, 1, "missing closing )"},
		{"negative lookbehind", `(?<!a)b`, 1, "lookbehind"},
		{"lookbehind", `x(?<=a)b`, 2, "lookbehind"},
		{"lookahead", `a(?=b)`, 2, "lookahead"},
		{"backreference", `(a)\1`, 4, "backreferences"},
		{"invalid repetition", `a**`, 2, "invalid nested repetition"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runRegex(t, "abc", tool.Values{"pattern": tt.pattern})
			ie, ok := tool.AsInputError(err)
			if !ok {
				t.Fatalf("error = %v, want *InputError", err)
			}
			if ie.Input != "pattern" || ie.Col != tt.col || !strings.Contains(ie.Msg, tt.hint) {
				t.Errorf("= %q col %d (%s), want col %d with %q", ie.Input, ie.Col, ie.Msg, tt.col, tt.hint)
			}
		})
	}
}

func FuzzRegex(f *testing.F) {
	f.Add(`(\w+)@(\w+)`, "ana@x bob@y")
	f.Add(`(?<=a)`, "a")
	f.Fuzz(func(t *testing.T, pattern, text string) {
		_, err := runRegex(t, text, tool.Values{"pattern": pattern, "multiline": true})
		if err != nil {
			if _, ok := tool.AsInputError(err); !ok {
				t.Fatalf("error that is not *InputError: %v", err)
			}
		}
	})
}
