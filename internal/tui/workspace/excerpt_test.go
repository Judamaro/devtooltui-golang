package workspace

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestExcerpt(t *testing.T) {
	long := strings.Repeat("a", 40) + "X" + strings.Repeat("b", 40)
	tests := []struct {
		name      string
		line      string
		lineNo    int
		col       int
		width     int
		src, mark string
	}{
		{"first column", `}{"a":1}`, 1, 1, 40, "1 │ }{\"a\":1}", "  │ ^ bad"},
		{"middle", `{"a":1,}`, 3, 8, 40, "3 │ {\"a\":1,}", "  │        ^ bad"},
		{"end of line", `{"a":[1,2`, 1, 10, 40, "1 │ {\"a\":[1,2", "  │          ^ bad"},
		{"tab is one cell", "\t\"a\" x", 1, 6, 40, "1 │  \"a\" x", "  │      ^ bad"},
		{"wide gutter", "x", 120, 1, 40, "120 │ x", "    │ ^ bad"},
		{"long line is windowed", long, 1, 41, 26, "1 │ …aaaaaaaaaaXbbbbbbbbb…", "  │ " + strings.Repeat(" ", 11) + "^ bad"},
		{"long line, culprit at the end", long, 1, 82, 26, "1 │ …bbbbbbbbbbbbbbbbbbbb", "  │ " + strings.Repeat(" ", 17) + "bad ^"},
		{"long line, culprit at the start", long, 1, 1, 26, "1 │ aaaaaaaaaaaaaaaaaaaa…", "  │ ^ bad"},
		{"caret message is clipped", "x", 1, 1, 8, "1 │ x", "  │ ^ ba"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := excerpt(tt.line, tt.lineNo, tt.col, "bad", tt.width, Styles{})
			if len(got) != 2 {
				t.Fatalf("got %d lines, want 2", len(got))
			}
			src, mark := strings.TrimRight(ansi.Strip(got[0]), " "), ansi.Strip(got[1])
			if src != tt.src || mark != tt.mark {
				t.Errorf("got\n%q\n%q\nwant\n%q\n%q", src, mark, tt.src, tt.mark)
			}
			for i, l := range got {
				if w := ansi.StringWidth(l); w > tt.width {
					t.Errorf("line %d is %d columns wide, maximum %d", i+1, w, tt.width)
				}
			}
		})
	}
}

func TestNthLine(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
		ok   bool
	}{
		{"a\nb\r\nc", 2, "b", true},
		{"a\nb", 3, "", false},
		{"", 1, "", true},
		{"a\n", 2, "", true},
	}
	for _, tt := range tests {
		got, ok := nthLine(tt.s, tt.n)
		if got != tt.want || ok != tt.ok {
			t.Errorf("nthLine(%q, %d) = %q, %v; want %q, %v", tt.s, tt.n, got, ok, tt.want, tt.ok)
		}
	}
}
