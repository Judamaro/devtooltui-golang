package render

import (
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

func TestRenderMatches(t *testing.T) {
	mark := lipgloss.NewStyle().Transform(func(s string) string { return "[" + s + "]" })
	tests := []struct {
		name  string
		text  string
		spans []tool.Span
		want  string
	}{
		{"no spans", "abc", nil, "abc"},
		{"two matches", "a1 b22", []tool.Span{{Start: 0, End: 2}, {Start: 3, End: 6}}, "[a1] [b22]"},
		{"ignores groups", "a1", []tool.Span{{Start: 0, End: 2}, {Start: 1, End: 2, Group: 1}}, "[a1]"},
		{"multi-line per line", "x\ny", []tool.Span{{Start: 0, End: 3}}, "[x]\n[y]"},
		{"ignores empty and out of range", "ab", []tool.Span{{Start: 1, End: 1}, {Start: 1, End: 9}}, "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Matches(tt.text, tt.spans, mark); got != tt.want {
				t.Errorf("= %q, want %q", got, tt.want)
			}
		})
	}
}
