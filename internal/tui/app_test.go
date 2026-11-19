package tui

import (
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools"
	"github.com/Judamaro/devtooltui-golang/internal/tools/encoders"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace"
)

// newApp arma una App solo con base64, para que los tests de
// comportamiento no dependan del orden del registro completo.
func newApp(t *testing.T, w, h int) *App {
	t.Helper()
	return newAppWith(t, w, h, encoders.Base64{})
}

func newAppWith(t *testing.T, w, h int, ts ...tool.Tool) *App {
	t.Helper()
	reg, err := registry.New(ts...)
	if err != nil {
		t.Fatal(err)
	}
	a := New(reg, Options{Debounce: 0})
	send(a, tea.WindowSizeMsg{Width: w, Height: h})
	testutil.RunCmds(a.Init(), func(msg tea.Msg) tea.Cmd { _, c := a.Update(msg); return c })
	return a
}

// send entrega msg a la App y ejecuta los comandos que devuelva, de modo
// que los resultados asíncronos ya estén aplicados al volver.
func send(a *App, msg tea.Msg) {
	_, cmd := a.Update(msg)
	testutil.RunCmds(cmd, func(m tea.Msg) tea.Cmd {
		if _, ok := m.(tea.QuitMsg); ok {
			return nil
		}
		_, c := a.Update(m)
		return c
	})
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
		case "right":
			msg = tea.KeyPressMsg{Code: tea.KeyRight}
		case "left":
			msg = tea.KeyPressMsg{Code: tea.KeyLeft}
		case "down":
			msg = tea.KeyPressMsg{Code: tea.KeyDown}
		case "up":
			msg = tea.KeyPressMsg{Code: tea.KeyUp}
		case "space":
			msg = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
		default:
			for _, r := range k {
				send(a, tea.KeyPressMsg{Code: r, Text: string(r)})
			}
			continue
		}
		send(a, msg)
	}
}

var elapsedRe = regexp.MustCompile(`✓ [0-9.]+ (µs|ms|s)`)

// plainView quita los códigos ANSI y enmascara el tiempo de ejecución para
// que los golden sean deterministas.
func plainView(a *App) string {
	lines := strings.Split(ansi.Strip(a.View().Content), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(elapsedRe.ReplaceAllString(l, "✓ 0 µs"), " ")
	}
	// El relleno de la statusbar depende del ancho del tiempo enmascarado.
	last := len(lines) - 1
	lines[last] = spacesRe.ReplaceAllString(lines[last], "  ")
	return strings.Join(lines, "\n")
}

var spacesRe = regexp.MustCompile(` {2,}`)

func TestViewGolden(t *testing.T) {
	tests := []struct {
		name string
		w, h int
		do   func(a *App)
	}{
		{"inicio-100x30", 100, 30, func(*App) {}},
		{"todas-las-herramientas-100x30", 100, 30, nil},
		{"base64-hola-100x30", 100, 30, func(a *App) { press(a, "tab", "tab", "hola") }},
		{"opciones-enfocadas-100x30", 100, 30, func(a *App) { press(a, "tab", "right") }},
		{"entrada-grande-100x30", 100, 30, func(a *App) {
			press(a, "tab", "tab")
			send(a, tea.PasteMsg{Content: strings.Repeat("hola mundo\n", 3500)})
		}},
		{"angosto-80x20", 80, 20, func(a *App) { press(a, "hola") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a *App
			if tt.do == nil {
				a = newAppWith(t, tt.w, tt.h, tools.All()...)
			} else {
				a = newApp(t, tt.w, tt.h)
				tt.do(a)
			}
			got := plainView(a)
			raw := ansi.Strip(a.View().Content)
			if n := strings.Count(raw, "\n") + 1; n != tt.h {
				t.Errorf("la vista tiene %d líneas, want %d", n, tt.h)
			}
			for i, line := range strings.Split(raw, "\n") {
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
		{"tab a opciones", 100, []string{"tab"}, focusOptions},
		{"tab tab a entrada", 100, []string{"tab", "tab"}, focusInput},
		{"tres tabs a salida", 100, []string{"tab", "tab", "tab"}, focusOutput},
		{"vuelta completa", 100, []string{"tab", "tab", "tab", "tab"}, focusSidebar},
		{"shift+tab hacia atrás", 100, []string{"shift+tab"}, focusOutput},
		{"angosto salta sidebar", 80, []string{"tab", "tab", "tab"}, focusInput},
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
	send(a, tea.WindowSizeMsg{Width: 89, Height: 30})
	if a.showSidebar {
		t.Error("sidebar visible a 89 columnas")
	}
	press(a, "ctrl+b")
	if !a.showSidebar {
		t.Error("ctrl+b no muestra el sidebar")
	}
	send(a, tea.WindowSizeMsg{Width: 85, Height: 30})
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
		{"q en opciones select sale", []string{"tab"}, tea.KeyPressMsg{Code: 'q', Text: "q"}, true},
		{"q en la entrada escribe", []string{"tab", "tab"}, tea.KeyPressMsg{Code: 'q', Text: "q"}, false},
		{"ctrl+c en la entrada sale", []string{"tab", "tab"}, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}, true},
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

func TestRunAndClearKeys(t *testing.T) {
	a := newApp(t, 100, 30)
	press(a, "tab", "tab")
	send(a, tea.PasteMsg{Content: strings.Repeat("a", 40000)})
	ws := a.workspace()
	if !ws.Large() {
		t.Fatal("no entró en modo grande")
	}
	send(a, tea.KeyPressMsg{Code: tea.KeyF5})
	if ws.Err() != nil {
		t.Fatalf("F5: %v", ws.Err())
	}
	send(a, tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	if ws.Large() || ws.Output() != "" {
		t.Errorf("ctrl+l no limpió la entrada (Large=%v, salida=%d bytes)", ws.Large(), len(ws.Output()))
	}
}

func keyCtrl(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl} }

func isQuit(msg tea.Msg) bool {
	_, ok := msg.(tea.QuitMsg)
	return ok
}

func TestOptionsRerun(t *testing.T) {
	tests := []struct {
		name    string
		keys    []string
		want    string
		wantErr bool
	}{
		{"encode por defecto", []string{"tab", "tab", "hola"}, "aG9sYQ==", false},
		{"cambiar a decode re-ejecuta", []string{"tab", "tab", "aG9sYQ==", "shift+tab", "right"}, "hola", false},
		{"sin padding", []string{"tab", "down", "down", "space", "tab", "hola"}, "aG9sYQ", false},
		{"error mantiene la salida anterior", []string{"tab", "right", "tab", "aG9sYQ==", "*"}, "hola", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newApp(t, 100, 30)
			press(a, tt.keys...)
			ws := a.workspace()
			if ws.Output() != tt.want {
				t.Errorf("salida = %q, want %q", ws.Output(), tt.want)
			}
			if (ws.Err() != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", ws.Err(), tt.wantErr)
			}
		})
	}
}

// TestProgram recorre el flujo real con teatest: arrancar, escribir y salir.
// El renderer de v2 solo reescribe las celdas que cambian, así que se
// comprueba el modelo final en lugar de buscar texto en la salida cruda.
func TestProgram(t *testing.T) {
	reg, err := registry.New(encoders.Base64{})
	if err != nil {
		t.Fatal(err)
	}
	// El filtro avisa cuando llega el resultado de "hola", porque la
	// ejecución es asíncrona y no se puede salir antes.
	done := make(chan struct{})
	var once sync.Once
	filter := func(_ tea.Model, msg tea.Msg) tea.Msg {
		if r, ok := msg.(workspace.ResultMsg); ok && r.Result.Text == "aG9sYQ==" {
			once.Do(func() { close(done) })
		}
		return msg
	}
	tm := teatest.NewTestModel(t, New(reg, DefaultOptions()),
		teatest.WithInitialTermSize(100, 30),
		teatest.WithProgramOptions(tea.WithFilter(filter)))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(string(b), "Codificadores")
	}, teatest.WithDuration(3*time.Second))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Type("hola")
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("no llegó el resultado de la ejecución asíncrona")
	}
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	final, ok := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).(*App)
	if !ok {
		t.Fatal("el modelo final no es *App")
	}
	if got := final.workspace().Output(); got != "aG9sYQ==" {
		t.Errorf("salida = %q, want %q", got, "aG9sYQ==")
	}
}
