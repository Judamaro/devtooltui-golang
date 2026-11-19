package options_test

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace/options"
)

var keys = options.KeyMap{
	Prev:     key.NewBinding(key.WithKeys("up")),
	Next:     key.NewBinding(key.WithKeys("down")),
	Decrease: key.NewBinding(key.WithKeys("left")),
	Increase: key.NewBinding(key.WithKeys("right")),
	Toggle:   key.NewBinding(key.WithKeys("space")),
}

var spec = tool.Spec{Options: []tool.Option{
	{Key: "mode", Label: "Modo", Kind: tool.Select, Choices: []string{"encode", "decode", "parse"}, Default: "encode"},
	{Key: "pad", Label: "Padding", Kind: tool.Toggle, Default: true},
	{Key: "indent", Label: "Indentación", Kind: tool.Number, Min: 0, Max: 8, Default: 2},
	{Key: "secret", Label: "Clave", Kind: tool.String, Default: ""},
}}

func newModel() options.Model {
	m := options.New(spec, tool.Values{}.WithDefaults(spec), options.Styles{}, keys)
	m.Focus()
	return m
}

func k(s string) tea.KeyPressMsg {
	switch s {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

func TestUpdate(t *testing.T) {
	tests := []struct {
		name        string
		keys        []string
		key         string
		want        any
		lastChanged bool
		captures    bool
	}{
		{"select siguiente", []string{"right"}, "mode", "decode", true, false},
		{"select anterior da la vuelta", []string{"left"}, "mode", "parse", true, false},
		{"toggle con espacio", []string{"down", "space"}, "pad", false, true, false},
		{"toggle con flecha", []string{"down", "right"}, "pad", false, true, false},
		{"number incrementa", []string{"down", "down", "right"}, "indent", 3, true, true},
		{"number no baja del minimo", []string{"down", "down", "left", "left", "left"}, "indent", 0, false, true},
		{"number tecleado", []string{"down", "down", "backspace", "5"}, "indent", 5, true, true},
		{"number fuera de rango no aplica", []string{"down", "down", "backspace", "9"}, "indent", 2, false, true},
		{"string tecleado", []string{"up", "a", "b"}, "secret", "ab", true, true},
		{"navegar no cambia", []string{"down"}, "mode", "encode", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newModel()
			var changed bool
			for _, s := range tt.keys {
				changed, _ = m.Update(k(s))
			}
			if got := m.Values()[tt.key]; got != tt.want {
				t.Errorf("%s = %v, want %v", tt.key, got, tt.want)
			}
			if changed != tt.lastChanged {
				t.Errorf("changed = %v, want %v", changed, tt.lastChanged)
			}
			if m.CapturesText() != tt.captures {
				t.Errorf("CapturesText() = %v, want %v", m.CapturesText(), tt.captures)
			}
		})
	}
}

func TestLines(t *testing.T) {
	tests := []struct {
		name  string
		width int
		want  []string
	}{
		{"todo en una linea", 200, []string{"Modo: ◀ encode ▶   Padding: [x]   Indentación: 2   Clave:"}},
		{"se parte", 40, []string{"Modo: ◀ encode ▶   Padding: [x]", "Indentación: 2   Clave:"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := options.New(spec, tool.Values{}.WithDefaults(spec), options.Styles{}, keys)
			got := m.Lines(tt.width)
			for i := range got {
				got[i] = strings.TrimRight(ansi.Strip(got[i]), " ")
			}
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("Lines() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEmpty(t *testing.T) {
	m := options.New(tool.Spec{}, tool.Values{}, options.Styles{}, keys)
	if !m.Empty() || len(m.Lines(80)) != 0 || m.CapturesText() {
		t.Error("panel sin opciones no está vacío")
	}
	if changed, _ := m.Update(k("right")); changed {
		t.Error("Update en panel vacío cambió algo")
	}
}
