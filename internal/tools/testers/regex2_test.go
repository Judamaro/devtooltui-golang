package testers_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

func r2(pattern string, extra ...any) tool.Values {
	v := tool.Values{"engine": "regexp2", "pattern": pattern}
	for i := 0; i+1 < len(extra); i += 2 {
		v[extra[i].(string)] = extra[i+1]
	}
	return v
}

func TestRegexp2Golden(t *testing.T) {
	res, err := runRegex(t, "Total: $120, tax: $9.60, ref #42", r2(`(?<=\$)(?<amount>\d+(?:\.\d+)?)`))
	if err != nil {
		t.Fatal(err)
	}
	testutil.Golden(t, "regex", "regexp2-lookbehind-named", res.Text)
}

func TestRegexp2(t *testing.T) {
	tests := []struct {
		name, text string
		opts       tool.Values
		matches    []string // whole matches, in order
	}{
		{"lookahead", "wow! yes no!", r2(`\w+(?=!)`), []string{"wow", "no"}},
		{"negative lookahead", "a1 b2 c1", r2(`[a-z](?!1)`), []string{"b"}},
		{"lookbehind", "$5 €7 $9", r2(`(?<=\$)\d`), []string{"5", "9"}},
		{"backreference", "aa ab cc", r2(`(\w)\1`), []string{"aa", "cc"}},
		{"ignore case", "Go GO go", r2(`go`, "ignore_case", true), []string{"Go", "GO", "go"}},
		{"multiline", "a\nb", r2(`^\w$`, "multiline", true), []string{"a", "b"}},
		{"dotall", "a\nb", r2(`a.b`, "dotall", true), []string{"a\nb"}},
		{"multibyte offsets", "ñandú café ünï", r2(`\w+(?= )`), []string{"ñandú", "café"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runRegex(t, tt.text, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, sp := range res.Spans {
				if sp.Group == 0 {
					got = append(got, tt.text[sp.Start:sp.End]) // byte offsets
				}
			}
			if strings.Join(got, "|") != strings.Join(tt.matches, "|") {
				t.Errorf("matches = %q, want %q", got, tt.matches)
			}
		})
	}
}

func TestRegexp2Groups(t *testing.T) {
	res, err := runRegex(t, "2026-11-20 and 1999-01-02", r2(`(?<year>\d{4})-(\d{2})-(?<day>\d{2})(x)?`))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"(year): \"2026\"", "(day): \"20\"", "\"11\"", ": —"} {
		if !strings.Contains(res.Text, want) {
			t.Errorf("missing %q in:\n%s", want, res.Text)
		}
	}
	if res.Meta[0].Value != "2" || res.Meta[1].Value != "4" {
		t.Errorf("Meta = %v, want 2 matches and 4 groups", res.Meta)
	}
}

func TestRegexp2Errors(t *testing.T) {
	_, err := runRegex(t, "x", r2(`(?<=a`))
	if ie, ok := tool.AsInputError(err); !ok || ie.Input != "pattern" || ie.Line != 1 || ie.Col != 1 {
		t.Errorf("syntax error = %v, want an InputError on pattern", err)
	}
	// RE2 still explains what it lacks.
	_, err = runRegex(t, "x", tool.Values{"pattern": `(?<=a)b`})
	if ie, ok := tool.AsInputError(err); !ok || !strings.Contains(ie.Msg, "does not support lookbehind") {
		t.Errorf("RE2 lookbehind error = %v", err)
	}
}

func TestRegexp2Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("takes a second")
	}
	start := time.Now()
	_, err := runRegex(t, strings.Repeat("a", 40)+"b", r2(`^(a+)+$`))
	ie, ok := tool.AsInputError(err)
	if !ok || !strings.Contains(ie.Msg, "catastrophic backtracking") {
		t.Fatalf("error = %v, want the timeout", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("the timeout took %s", d)
	}
}
