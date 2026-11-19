package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tools"
)

func newApp(t *testing.T, w, h int) *App {
	t.Helper()
	reg, err := registry.New(tools.All()...)
	if err != nil {
		t.Fatal(err)
	}
	a := New(reg)
	a.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return a
}

func press(a *App, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "tab":
			msg = tea.KeyPressMsg{Code: tea.KeyTab}
		case "shift+tab":
			msg = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
		case "ctrl+b":
			msg = tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl}
		default:
			for _, r := range k {
				msg = tea.KeyPressMsg{Code: r, Text: string(r)}
				a.Update(msg)
			}
			continue
		}
		a.Update(msg)
	}
}

var elapsedRe = regexp.MustCompile(`✓ [0-9.]+[µnm]?s`)

// plainView quita los códigos ANSI y enmascara el tiempo de ejecución para
// que los golden sean deterministas.
func plainView(a *App) string {
	lines := strings.Split(ansi.Strip(a.View().Content), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(elapsedRe.ReplaceAllString(l, "✓ 0s"), " ")
	}
	return strings.Join(lines, "\n")
}

func TestViewGolden(t *testing.T) {
	tests := []struct {
		name string
		w, h int
		do   func(a *App)
	}{
		{"inicio-100x30", 100, 30, func(*App) {}},
		{"base64-hola-100x30", 100, 30, func(a *App) { press(a, "tab", "hola") }},
		{"angosto-80x20", 80, 20, func(a *App) { press(a, "hola") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newApp(t, tt.w, tt.h)
			tt.do(a)
			got := plainView(a)
			if n := strings.Count(got, "\n") + 1; n != tt.h {
				t.Errorf("la vista tiene %d líneas, want %d", n, tt.h)
			}
			for i, line := range strings.Split(got, "\n") {
				if w := ansi.StringWidth(line); w > tt.w {
					t.Errorf("línea %d mide %d columnas, máximo %d", i+1, w, tt.w)
				}
			}
			testutil.Golden(t, "tui", tt.name, got)
		})
	}
}

func TestFocusRotation(t *testing.T) {
	tests := []struct {
		name string
		w    int
		keys []string
		want focus
	}{
		{"inicio en sidebar", 100, nil, focusSidebar},
		{"tab a entrada", 100, []string{"tab"}, focusInput},
		{"tab tab a salida", 100, []string{"tab", "tab"}, focusOutput},
		{"vuelta completa", 100, []string{"tab", "tab", "tab"}, focusSidebar},
		{"shift+tab hacia atrás", 100, []string{"shift+tab"}, focusOutput},
		{"angosto salta sidebar", 80, []string{"tab", "tab"}, focusInput},
		{"ctrl+b oculta y mueve foco", 100, []string{"ctrl+b"}, focusInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newApp(t, tt.w, 30)
			press(a, tt.keys...)
			if a.focus != tt.want {
				t.Errorf("focus = %d, want %d", a.focus, tt.want)
			}
		})
	}
}

func TestSidebarCollapse(t *testing.T) {
	a := newApp(t, 100, 30)
	if !a.showSidebar {
		t.Fatal("sidebar oculto a 100 columnas")
	}
	a.Update(tea.WindowSizeMsg{Width: 89, Height: 30})
	if a.showSidebar {
		t.Error("sidebar visible a 89 columnas")
	}
	press(a, "ctrl+b")
	if !a.showSidebar {
		t.Error("ctrl+b no muestra el sidebar")
	}
	a.Update(tea.WindowSizeMsg{Width: 85, Height: 30})
	if !a.showSidebar {
		t.Error("un resize sin cruzar el umbral pisó la elección del usuario")
	}
}

func TestQuitKeys(t *testing.T) {
	tests := []struct {
		name     string
		keys     []string
		msg      tea.KeyPressMsg
		wantQuit bool
	}{
		{"q en sidebar sale", nil, tea.KeyPressMsg{Code: 'q', Text: "q"}, true},
		{"q en la entrada escribe", []string{"tab"}, tea.KeyPressMsg{Code: 'q', Text: "q"}, false},
		{"ctrl+c en la entrada sale", []string{"tab"}, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newApp(t, 100, 30)
			press(a, tt.keys...)
			_, cmd := a.Update(tt.msg)
			quit := cmd != nil && isQuit(cmd())
			if quit != tt.wantQuit {
				t.Errorf("quit = %v, want %v", quit, tt.wantQuit)
			}
		})
	}
}

func isQuit(msg tea.Msg) bool {
	_, ok := msg.(tea.QuitMsg)
	return ok
}

func TestInputErrorKeepsOutput(t *testing.T) {
	a := newApp(t, 100, 30)
	press(a, "tab", "hola")
	ws := a.workspace()
	if ws.Output() != "aG9sYQ==" {
		t.Fatalf("salida = %q", ws.Output())
	}
	if ws.Err() != nil {
		t.Fatalf("error inesperado: %v", ws.Err())
	}
}

// TestProgram recorre el flujo real con teatest: arrancar, escribir y salir.
// El renderer de v2 solo reescribe las celdas que cambian, así que se
// comprueba el modelo final en lugar de buscar texto en la salida cruda.
func TestProgram(t *testing.T) {
	reg, err := registry.New(tools.All()...)
	if err != nil {
		t.Fatal(err)
	}
	tm := teatest.NewTestModel(t, New(reg), teatest.WithInitialTermSize(100, 30))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(string(b), "Codificadores")
	}, teatest.WithDuration(3*time.Second))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Type("hola")
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	final, ok := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).(*App)
	if !ok {
		t.Fatal("el modelo final no es *App")
	}
	if got := final.workspace().Output(); got != "aG9sYQ==" {
		t.Errorf("salida = %q, want %q", got, "aG9sYQ==")
	}
}
