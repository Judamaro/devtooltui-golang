package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Judamaro/devtooltui-golang/internal/tools"
	"github.com/Judamaro/devtooltui-golang/internal/tools/formatters"
)

func TestUseOutput(t *testing.T) {
	a := newApp(t, 100, 30) // base64
	press(a, "tab", "tab", "hola")
	send(a, keyCtrl('r'))
	ws := a.workspace()
	if ws.ActiveValue() != "aG9sYQ==" || ws.Output() != "YUc5c1lRPT0=" {
		t.Errorf("entrada = %q, salida = %q", ws.ActiveValue(), ws.Output())
	}
	if a.flash != "salida usada como entrada" {
		t.Errorf("flash = %q", a.flash)
	}
}

func TestEditorDone(t *testing.T) {
	a := newApp(t, 100, 30)
	path := filepath.Join(t.TempDir(), "edit.txt")
	if err := os.WriteFile(path, []byte("desde el editor\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	send(a, editorDoneMsg{path: path})
	ws := a.workspace()
	if ws.ActiveValue() != "desde el editor" || a.focus != focusInput {
		t.Errorf("entrada = %q, focus = %d", ws.ActiveValue(), a.focus)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("no se borró el archivo temporal")
	}
}

func TestEditorCommand(t *testing.T) {
	tests := []struct {
		visual, editor string
		want           []string
	}{
		{"code --wait", "nano", []string{"code", "--wait", "/tmp/x"}},
		{"", "nano", []string{"nano", "/tmp/x"}},
	}
	for _, tt := range tests {
		t.Setenv("VISUAL", tt.visual)
		t.Setenv("EDITOR", tt.editor)
		if got := editorCommand("/tmp/x").Args; strings.Join(got, " ") != strings.Join(tt.want, " ") {
			t.Errorf("Args = %v, want %v", got, tt.want)
		}
	}
}

func TestEditInputStartsEditor(t *testing.T) {
	a := newApp(t, 100, 30)
	if cmd := a.editInput(); cmd == nil {
		t.Error("ctrl+e no lanzó el editor")
	}
}

func TestSaveOutput(t *testing.T) {
	a := newAppWith(t, 100, 30, formatters.JSONFormat{})
	press(a, "tab", "tab", "[1]")
	send(a, keyCtrl('s'))
	if a.prompt == nil || !strings.Contains(ansi.Strip(a.View().Content), "salida.json") {
		t.Fatal("ctrl+s no abrió el prompt con salida.json")
	}
	path := filepath.Join(t.TempDir(), "out.json")
	send(a, keyCtrl('u')) // borra el valor sugerido
	press(a, path)
	send(a, keyEnter())
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "[\n  1\n]\n" {
		t.Fatalf("archivo = %q, %v", data, err)
	}
	if a.prompt != nil || !strings.HasPrefix(a.flash, "guardado ") {
		t.Errorf("prompt abierto = %v, flash = %q", a.prompt != nil, a.flash)
	}
}

func TestHelpOverlay(t *testing.T) {
	a := newApp(t, 100, 30)
	press(a, "?")
	if !a.showHelp || !strings.Contains(ansi.Strip(a.View().Content), "Atajos") {
		t.Fatal("? no mostró la ayuda")
	}
	press(a, "x")
	if a.showHelp {
		t.Error("una tecla no cerró la ayuda")
	}
	press(a, "tab", "tab", "?") // dentro de la entrada, "?" es texto
	if a.showHelp || a.workspace().ActiveValue() != "?" {
		t.Errorf("en la entrada '?' abrió la ayuda o no se escribió")
	}
}

func TestSidebarFilterInApp(t *testing.T) {
	a := newAppWith(t, 100, 30, tools.All()...)
	press(a, "/", "b64q")
	if a.active != "base64" {
		t.Errorf("active = %q, want base64", a.active)
	}
	press(a, "x") // "q" se escribió en el filtro: la app sigue viva
	if _, cmd := a.Update(tea.KeyPressMsg{Code: 'q', Text: "q"}); cmd != nil && isQuit(cmd()) {
		t.Error("q salió mientras se filtraba")
	}
}
