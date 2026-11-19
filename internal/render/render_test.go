package render

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

func TestResult(t *testing.T) {
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	s := Styles{Chroma: testStyle, Match: lipgloss.NewStyle().Reverse(true), Diff: DiffStyles{Insert: green, Delete: green, Change: green, Hunk: green}}
	big := strings.Repeat("x", MaxBytes+1)
	tests := []struct {
		name        string
		res         tool.Result
		kind        tool.OutputKind
		inputs      map[string]string
		wantColored bool
		wantNoColor bool
	}{
		{"highlighted", tool.Result{Text: `{"a":1}`, Lang: "json"}, tool.Highlighted, nil, true, false},
		{"large highlighted", tool.Result{Text: big, Lang: "json"}, tool.Highlighted, nil, false, true},
		{"highlighted without lang", tool.Result{Text: "x"}, tool.Highlighted, nil, false, false},
		{"plain", tool.Result{Text: "x"}, tool.PlainText, nil, false, false},
		{"diff", tool.Result{Text: "+a", Lang: "diff"}, tool.DiffView, nil, true, false},
		{"large diff", tool.Result{Text: big, Lang: "diff"}, tool.DiffView, nil, false, true},
		{"matches", tool.Result{Text: "1 match", Spans: []tool.Span{{Input: "t", Start: 0, End: 1}}}, tool.Matches,
			map[string]string{"t": "ab"}, true, false},
		{"matches with large input", tool.Result{Spans: []tool.Span{{Input: "t", End: 1}}}, tool.Matches,
			map[string]string{"t": big}, false, true},
		{"matches without spans", tool.Result{Text: "0 matches"}, tool.Matches, nil, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, noColor := Result(tt.res, tt.kind, tt.inputs, s)
			if got := strings.Contains(out, "\x1b["); got != tt.wantColored {
				t.Errorf("coloured = %v, want %v (%q)", got, tt.wantColored, out)
			}
			if noColor != tt.wantNoColor {
				t.Errorf("noColor = %v, want %v", noColor, tt.wantNoColor)
			}
		})
	}
}
