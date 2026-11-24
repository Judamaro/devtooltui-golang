package workspace

import (
	"context"
	"testing"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace/options"
)

type stub struct{ spec tool.Spec }

func (s stub) Spec() tool.Spec { return s.spec }
func (stub) Run(context.Context, tool.Request) (tool.Result, error) {
	return tool.Result{}, nil
}

var clickKeys = options.KeyMap{
	Prev: key.NewBinding(key.WithKeys("up")), Next: key.NewBinding(key.WithKeys("down")),
	Decrease: key.NewBinding(key.WithKeys("left")), Increase: key.NewBinding(key.WithKeys("right")),
	Toggle: key.NewBinding(key.WithKeys("space")),
}

func TestClickAt(t *testing.T) {
	f := newField(tool.InputSpec{Key: "in"}, textarea.DefaultDarkStyles())
	f.setSize(13, 6)                                      // 3 columns of line numbers: 10 of text
	f.setValue("abc\n0123456789abcde\nxy", DefaultLimits) // the second line takes two rows
	f.ta.Focus()
	tests := []struct {
		row, col     int
		line, column int
		name         string
	}{
		{0, 3 + 1, 0, 1, "first line"},
		{1, 3 + 4, 1, 4, "second line"},
		{2, 3 + 2, 1, 12, "wrapped part of the second line"},
		{3, 3 + 9, 2, 2, "past the end of a line"},
		{5, 3, 2, 0, "below the text"},
		{0, 0, 0, 0, "on the line numbers"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f.clickAt(tt.row, tt.col)
			if l, c := f.ta.Line(), f.ta.Column(); l != tt.line || c != tt.column {
				t.Errorf("cursor at %d:%d, want %d:%d", l, c, tt.line, tt.column)
			}
		})
	}
}

func TestClickNumberOption(t *testing.T) {
	spec := tool.Spec{ID: "x", Name: "X", Category: tool.Text, Options: []tool.Option{
		{Key: "n", Label: "N", Kind: tool.Number, Min: 1, Max: 9, Default: 3},
		{Key: "s", Label: "S", Kind: tool.String, Default: ""},
	}}
	m := New(stub{spec}, Styles{}, clickKeys, 0)
	m.SetSize(80, 24)
	f, ok, _ := m.Click(1, 1) // inside the tool box, on "N: 3"
	if !ok || f != FocusOptions {
		t.Fatalf("Click = %v, %v", f, ok)
	}
	m.SetFocus(f)
	if !m.opts.CapturesText() {
		t.Error("a click on a Number must focus its text field")
	}
}
