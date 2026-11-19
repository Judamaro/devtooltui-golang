package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// pair es una herramienta de prueba con dos entradas.
type pair struct{}

func (pair) Spec() tool.Spec {
	return tool.Spec{ID: "pair", Name: "Pair", Category: tool.Text, Description: "Une A y B", Live: true,
		Inputs: []tool.InputSpec{{Key: "a", Label: "Texto A"}, {Key: "b", Label: "Texto B"}}}
}

func (pair) Run(_ context.Context, req tool.Request) (tool.Result, error) {
	return tool.Result{Text: req.Inputs["a"] + "|" + req.Inputs["b"]}, nil
}

func TestTwoInputsGolden(t *testing.T) {
	tests := []struct {
		name string
		w, h int
	}{
		{"dos-entradas-lado-a-lado-130x30", 130, 30},
		{"dos-entradas-apiladas-100x30", 100, 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAppWith(t, tt.w, tt.h, pair{})
			press(a, "tab", "uno", "tab", "dos")
			raw := ansi.Strip(a.View().Content)
			if n := strings.Count(raw, "\n") + 1; n != tt.h {
				t.Errorf("la vista tiene %d líneas, want %d", n, tt.h)
			}
			for i, line := range strings.Split(raw, "\n") {
				if w := ansi.StringWidth(line); w > tt.w {
					t.Errorf("línea %d mide %d columnas, máximo %d", i+1, w, tt.w)
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
		{"tab entra en A", []string{"tab"}, focusInput, 0},
		{"tab pasa a B", []string{"tab", "tab"}, focusInput, 1},
		{"tercer tab a salida", []string{"tab", "tab", "tab"}, focusOutput, 1},
		{"shift+tab desde salida vuelve a B", []string{"tab", "tab", "tab", "shift+tab"}, focusInput, 1},
		{"shift+tab desde B a A", []string{"tab", "tab", "shift+tab"}, focusInput, 0},
		{"shift+tab desde sidebar entra por B", []string{"shift+tab", "shift+tab"}, focusInput, 1},
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
	press(a, "tab", "uno", "tab", "dos", "shift+tab", "!")
	if got := a.workspace().Output(); got != "uno!|dos" {
		t.Errorf("salida = %q, want %q", got, "uno!|dos")
	}
	press(a, "tab") // a B
	send(a, keyCtrl('l'))
	if got := a.workspace().Output(); got != "uno!|" {
		t.Errorf("ctrl+l debe limpiar solo la entrada con foco: %q", got)
	}
}
