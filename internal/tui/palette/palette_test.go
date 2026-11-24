package palette_test

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Judamaro/devtooltui-golang/internal/tui/palette"
)

var keys = palette.KeyMap{
	Up: key.NewBinding(key.WithKeys("up")), Down: key.NewBinding(key.WithKeys("down")),
	Choose: key.NewBinding(key.WithKeys("enter")), Close: key.NewBinding(key.WithKeys("esc")),
}

var items = []palette.Item{
	{Kind: palette.ToolItem, ID: "json-format", Title: "JSON", Detail: "Formateadores", Search: "pretty minify"},
	{Kind: palette.ToolItem, ID: "base64", Title: "Base64", Detail: "Codificadores"},
	{Kind: palette.ActionItem, ID: "copy", Title: "Copy output", Detail: "ctrl+y"},
}

func typeText(m *palette.Model, s string) {
	for _, r := range s {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func ids(its []palette.Item) string {
	var s []string
	for _, it := range its {
		s = append(s, it.ID)
	}
	return strings.Join(s, ",")
}

func TestFilter(t *testing.T) {
	tests := []struct{ q, want string }{
		{"", "json-format,base64,copy"},
		{"b64", "base64"},
		{"pretty", "json-format"},
		{"copy", "copy"},
		{"zzz", ""},
	}
	for _, tt := range tests {
		t.Run(tt.q, func(t *testing.T) {
			m := palette.New("Palette", items, palette.Styles{}, keys)
			typeText(m, tt.q)
			if got := ids(m.Filtered()); got != tt.want {
				t.Errorf("Filtered = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestChooseAndClose(t *testing.T) {
	m := palette.New("Palette", items, palette.Styles{}, keys)
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	msg := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})()
	if c, ok := msg.(palette.ChosenMsg); !ok || c.Item.ID != "base64" {
		t.Errorf("Enter = %#v, want base64", msg)
	}
	if _, ok := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})().(palette.ClosedMsg); !ok {
		t.Error("Esc does not close")
	}
	typeText(m, "zzz")
	if cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		t.Error("Enter without results should not choose anything")
	}
}

func TestOverlay(t *testing.T) {
	base := strings.TrimSuffix(strings.Repeat(strings.Repeat(".", 80)+"\n", 24), "\n")
	m := palette.New("Palette", items, palette.Styles{}, keys)
	m.SetSize(80, 24)
	out := ansi.Strip(m.Overlay(base))
	lines := strings.Split(out, "\n")
	if len(lines) != 24 {
		t.Fatalf("the view changed height: %d", len(lines))
	}
	if !strings.Contains(out, "Palette") || !strings.Contains(out, "▸ JSON") || !strings.HasPrefix(lines[0], "....") {
		t.Errorf("unexpected overlay:\n%s", out)
	}
}
