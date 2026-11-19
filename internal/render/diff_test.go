package render

import (
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

func TestRenderDiff(t *testing.T) {
	tag := func(name string) lipgloss.Style {
		return lipgloss.NewStyle().Transform(func(s string) string { return name + "(" + s + ")" })
	}
	st := DiffStyles{Insert: tag("ins"), Delete: tag("del"), Change: tag("chg"), Hunk: tag("hunk")}
	tests := []struct {
		name string
		res  tool.Result
		want string
	}{
		{"unified", tool.Result{Lang: "diff", Text: "--- A\n+++ B\n@@ -1 +1 @@\n a\n-b\n+c"},
			"hunk(--- A)\nhunk(+++ B)\nhunk(@@ -1 +1 @@)\n a\ndel(-b)\nins(+c)"},
		{"side by side", tool.Result{Lang: "diff-side", Text: "  a │ a\n| b │ c\n<  x │\n>    │ y"},
			"  a │ a\nchg(| b │ c)\ndel(<  x │)\nins(>    │ y)"},
		{"by words", tool.Result{Diff: []tool.DiffOp{{Kind: tool.Equal, Text: "the "}, {Kind: tool.Delete, Text: "cat"}, {Kind: tool.Insert, Text: "dog\nblack"}}},
			"the del(cat)ins(dog)\nins(black)"},
		{"no differences", tool.Result{Text: "No differences"}, "No differences"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Diff(tt.res, st); got != tt.want {
				t.Errorf("=\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}
