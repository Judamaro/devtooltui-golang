package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// pair is a test tool with two inputs.
type pair struct{}

func (pair) Spec() tool.Spec {
	return tool.Spec{ID: "pair", Name: "Pair", Category: tool.Text, Description: "Joins A and B", Live: true,
		Inputs: []tool.InputSpec{{Key: "a", Label: "Text A"}, {Key: "b", Label: "Text B"}}}
}

func (pair) Run(_ context.Context, req tool.Request) (tool.Result, error) {
	return tool.Result{Text: req.Inputs["a"] + "|" + req.Inputs["b"]}, nil
}

func TestTwoInputsGolden(t *testing.T) {
	tests := []struct {
		name string
		w, h int
	}{
		{"two-inputs-side-by-side-130x30", 130, 30},
		{"two-inputs-stacked-100x30", 100, 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAppWith(t, tt.w, tt.h, pair{})
			press(a, "tab", "one", "tab", "two")
			raw := ansi.Strip(a.View().Content)
			if n := strings.Count(raw, "\n") + 1; n != tt.h {
				t.Errorf("the view has %d lines, want %d", n, tt.h)
			}
			for i, line := range strings.Split(raw, "\n") {
				if w := ansi.StringWidth(line); w > tt.w {
					t.Errorf("line %d is %d columns wide, maximum %d", i+1, w, tt.w)
				}
			}
			testutil.Golden(t, "tui", tt.name, plainView(a))
		})
	}
}

func TestTwoInputsFocus(t *testing.T) {
	tests := []struct {
		name       string
		keys       []string
		wantFocus  focus
		wantActive int
	}{
		{"tab enters A", []string{"tab"}, focusInput, 0},
		{"tab moves to B", []string{"tab", "tab"}, focusInput, 1},
		{"third tab to output", []string{"tab", "tab", "tab"}, focusOutput, 1},
		{"shift+tab from output back to B", []string{"tab", "tab", "tab", "shift+tab"}, focusInput, 1},
		{"shift+tab from B to A", []string{"tab", "tab", "shift+tab"}, focusInput, 0},
		{"shift+tab from the sidebar enters through B", []string{"shift+tab", "shift+tab"}, focusInput, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAppWith(t, 130, 30, pair{})
			press(a, tt.keys...)
			if a.focus != tt.wantFocus || a.workspace().ActiveInput() != tt.wantActive {
				t.Errorf("focus = %d, input = %d; want %d, %d", a.focus, a.workspace().ActiveInput(), tt.wantFocus, tt.wantActive)
			}
		})
	}
}

func TestTwoInputsRouting(t *testing.T) {
	a := newAppWith(t, 100, 30, pair{})
	press(a, "tab", "one", "tab", "two", "shift+tab", "!")
	if got := a.workspace().Output(); got != "one!|two" {
		t.Errorf("output = %q, want %q", got, "one!|two")
	}
	press(a, "tab") // to B
	send(a, keyCtrl('l'))
	if got := a.workspace().Output(); got != "one!|" {
		t.Errorf("ctrl+l must clear only the focused input: %q", got)
	}
}
