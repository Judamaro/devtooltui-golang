package workspace_test

import (
	"context"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace/options"
)

// counter cuenta las ejecuciones y devuelve la longitud de la entrada.
type counter struct {
	live bool
	runs int
	last string
}

func (c *counter) Spec() tool.Spec {
	return tool.Spec{ID: "count", Name: "Count", Category: tool.Text, Live: c.live,
		Inputs: []tool.InputSpec{{Key: "in", Label: "Entrada"}}}
}

func (c *counter) Run(_ context.Context, req tool.Request) (tool.Result, error) {
	c.runs++
	c.last = req.Inputs["in"]
	return tool.Result{Text: strings.Repeat("x", min(len(c.last), 10))}, nil
}

var optKeys = options.KeyMap{
	Prev: key.NewBinding(key.WithKeys("up")), Next: key.NewBinding(key.WithKeys("down")),
	Decrease: key.NewBinding(key.WithKeys("left")), Increase: key.NewBinding(key.WithKeys("right")),
	Toggle: key.NewBinding(key.WithKeys("space")),
}

func newWS(c *counter) *workspace.Model {
	m := workspace.New(c, workspace.Styles{}, optKeys)
	m.SetSize(80, 24)
	m.SetFocus(workspace.FocusInput)
	return m
}

func typeText(m *workspace.Model, s string) {
	for _, r := range s {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func TestLargeInput(t *testing.T) {
	big := strings.Repeat("a", workspace.DefaultLimits.MaxBytes+1)
	manyLines := strings.Repeat("a\n", workspace.DefaultLimits.MaxLines)
	small := strings.Repeat("a", 100)
	tests := []struct {
		name      string
		paste     []string
		wantLarge bool
		wantLen   int
	}{
		{"pegado chico se edita", []string{small}, false, 100},
		{"pegado grande pasa a solo lectura", []string{big}, true, len(big)},
		{"demasiadas líneas pasa a solo lectura", []string{manyLines}, true, len(manyLines)},
		{"dos pegados que suman más del umbral", []string{big[:20000], big[:20000]}, true, 40000},
		{"pegar en modo grande agrega", []string{big, "zz"}, true, len(big) + 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &counter{live: true}
			m := newWS(c)
			for _, p := range tt.paste {
				m.Update(tea.PasteMsg{Content: p})
			}
			if m.Large() != tt.wantLarge {
				t.Errorf("Large() = %v, want %v", m.Large(), tt.wantLarge)
			}
			if len(c.last) != tt.wantLen {
				t.Errorf("la herramienta recibió %d bytes, want %d", len(c.last), tt.wantLen)
			}
			view := ansi.Strip(m.View())
			if tt.wantLarge && !strings.Contains(view, "entrada grande") {
				t.Error("la statusbar no avisa del modo de entrada grande")
			}
		})
	}
}

func TestLargeInputKeys(t *testing.T) {
	c := &counter{live: true}
	m := newWS(c)
	m.Update(tea.PasteMsg{Content: strings.Repeat("a", 40000)})
	runs := c.runs

	typeText(m, "zzz") // en solo lectura se ignora la escritura
	if c.runs != runs || len(c.last) != 40000 {
		t.Fatalf("escribir en modo grande re-ejecutó o cambió la entrada (runs %d→%d)", runs, c.runs)
	}
	m.Run() // F5
	if c.runs != runs+1 {
		t.Errorf("Run() no ejecutó la herramienta")
	}
	m.ClearInput() // ctrl+l
	if m.Large() || c.last != "" {
		t.Errorf("ClearInput() no salió del modo grande (Large=%v, last=%d bytes)", m.Large(), len(c.last))
	}
	typeText(m, "ab")
	if c.last != "ab" {
		t.Errorf("después de limpiar no se edita en vivo: %q", c.last)
	}
}

func TestNotLive(t *testing.T) {
	c := &counter{live: false}
	m := newWS(c)
	runs := c.runs
	typeText(m, "abc")
	if c.runs != runs {
		t.Errorf("una herramienta no-live se ejecutó al escribir")
	}
	if !strings.Contains(ansi.Strip(m.View()), "f5 para ejecutar") {
		t.Error("no se avisa que hay que presionar F5")
	}
	m.Run()
	if c.last != "abc" {
		t.Errorf("F5 ejecutó con %q", c.last)
	}
}

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0 B"}, {1023, "1023 B"}, {1024, "1.0 KB"}, {1536, "1.5 KB"}, {1 << 20, "1.0 MB"}, {5 << 20, "5.0 MB"},
	}
	for _, tt := range tests {
		if got := workspace.HumanBytes(tt.n); got != tt.want {
			t.Errorf("HumanBytes(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
