package frame

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestBox(t *testing.T) {
	tests := []struct {
		name, title, body string
		w, h              int
		want              string
	}{
		{"fits", "Input", "ab\ncd", 9, 4, "┌Input──┐\n│ab     │\n│cd     │\n└───────┘"},
		{"long title", "Output of the tool", "x", 8, 3, "┌Outpu…┐\n│x     │\n└──────┘"},
		{"no title", "", "x", 4, 3, "┌──┐\n│x │\n└──┘"},
		{"long lines are cut", "T", "abcdefgh", 6, 3, "┌T───┐\n│abcd│\n└────┘"},
		{"extra lines are dropped", "T", "1\n2\n3", 5, 4, "┌T──┐\n│1  │\n│2  │\n└───┘"},
		{"wide runes", "T", "日本語", 6, 3, "┌T───┐\n│日本│\n└────┘"},
		{"empty body", "T", "", 4, 2, "┌T─┐\n└──┘"},
		{"too small", "T", "body", 1, 5, "body"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Box(tt.title, tt.body, tt.w, tt.h, false, Styles{})
			if got != tt.want {
				t.Errorf("got\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestBoxSizeWithANSI(t *testing.T) {
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	body := red.Render(strings.Repeat("x", 30)) + "\n" + red.Render("y")
	s := Styles{Border: red, BorderFocused: red.Bold(true), Title: red, TitleFocused: red}
	for _, focused := range []bool{false, true} {
		out := Box(red.Render("Title"), body, 12, 4, focused, s)
		lines := strings.Split(out, "\n")
		if len(lines) != 4 {
			t.Fatalf("%d lines, want 4", len(lines))
		}
		for i, l := range lines {
			if w := ansi.StringWidth(l); w != 12 {
				t.Errorf("focused=%v line %d is %d columns, want 12: %q", focused, i, w, ansi.Strip(l))
			}
		}
	}
	if plain := ansi.Strip(Box("T", "a", 5, 3, true, s)); plain != "┌T──┐\n│a  │\n└───┘" {
		t.Errorf("focused box = %q", plain)
	}
}
