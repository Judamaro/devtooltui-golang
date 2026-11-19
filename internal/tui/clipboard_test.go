package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tools"
)

type fakeClip struct {
	data    string
	osc52   bool
	readErr error
}

func (f *fakeClip) Read() (string, error) { return f.data, f.readErr }
func (f *fakeClip) Write(s string) error  { f.data = s; return nil }
func (f *fakeClip) UseOSC52() bool        { return f.osc52 }

func newClipApp(t *testing.T, clip *fakeClip, detectOnStart bool) *App {
	t.Helper()
	reg, err := registry.New(tools.All()...)
	if err != nil {
		t.Fatal(err)
	}
	a := New(reg, Options{Clipboard: clip, DetectOnStart: detectOnStart})
	send(a, tea.WindowSizeMsg{Width: 100, Height: 30})
	testutil.RunCmds(a.Init(), func(msg tea.Msg) tea.Cmd { _, c := a.Update(msg); return c })
	return a
}

func TestCopyOutput(t *testing.T) {
	tests := []struct {
		name      string
		osc52     bool
		wantData  string
		wantFlash string
	}{
		{"local", false, "{}", "salida copiada al portapapeles"},
		{"osc52", true, "", "salida copiada (OSC 52)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clip := &fakeClip{osc52: tt.osc52}
			a := newClipApp(t, clip, false)
			press(a, "tab", "tab", "{}") // json-format es la primera herramienta
			send(a, keyCtrl('y'))
			if clip.data != tt.wantData || a.flash != tt.wantFlash {
				t.Errorf("data = %q, flash = %q; want %q, %q", clip.data, a.flash, tt.wantData, tt.wantFlash)
			}
		})
	}
}

func TestPasteFromClipboard(t *testing.T) {
	clip := &fakeClip{data: `{"b":1}`}
	a := newClipApp(t, clip, false) // foco en el sidebar
	send(a, keyCtrl('o'))
	if a.focus != focusInput || !strings.Contains(a.workspace().Output(), `"b": 1`) {
		t.Errorf("focus = %d, salida = %q", a.focus, a.workspace().Output())
	}

	clip.readErr = errors.New("no hay portapapeles del sistema: instala wl-clipboard")
	send(a, keyCtrl('o'))
	if !strings.Contains(a.flash, "wl-clipboard") {
		t.Errorf("flash = %q", a.flash)
	}
	press(a, "x") // cualquier tecla borra el aviso
	if a.flash != "" {
		t.Errorf("el aviso no se borró: %q", a.flash)
	}
}

func TestDetectClipboard(t *testing.T) {
	jwt := testutil.ReadInput(t, "jwt", "hs256")
	tests := []struct {
		name      string
		data      string
		onStart   bool
		wantTool  string
		wantFlash string
	}{
		{"jwt", jwt, false, "jwt", "detectado: jwt (0.98)"},
		{"al arrancar con --clipboard", "f47ac10b-58cc-4372-a567-0e02b2c3d479", true, "uuid", "detectado: uuid (0.90)"},
		{"sin formato", "hola mundo", false, "json-format", "no se reconoció"},
		{"vacio", "", false, "json-format", "vacío"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newClipApp(t, &fakeClip{data: tt.data}, tt.onStart)
			if !tt.onStart {
				send(a, keyCtrl('d'))
			}
			if a.active != tt.wantTool || !strings.Contains(a.flash, tt.wantFlash) {
				t.Errorf("active = %q, flash = %q; want %q, %q", a.active, a.flash, tt.wantTool, tt.wantFlash)
			}
			if tt.wantTool == "jwt" && a.sidebar.Selected() != "jwt" {
				t.Errorf("el sidebar no siguió el salto: %s", a.sidebar.Selected())
			}
		})
	}
}
