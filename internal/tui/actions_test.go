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
		t.Errorf("input = %q, output = %q", ws.ActiveValue(), ws.Output())
	}
	if a.flash != "output used as input" {
		t.Errorf("flash = %q", a.flash)
	}
}

func TestEditorDone(t *testing.T) {
	a := newApp(t, 100, 30)
	path := filepath.Join(t.TempDir(), "edit.txt")
	if err := os.WriteFile(path, []byte("from the editor\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	send(a, editorDoneMsg{path: path})
	ws := a.workspace()
	if ws.ActiveValue() != "from the editor" || a.focus != focusInput {
		t.Errorf("input = %q, focus = %d", ws.ActiveValue(), a.focus)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("the temporary file was not removed")
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
		t.Error("ctrl+e did not start the editor")
	}
}

func TestSaveOutput(t *testing.T) {
	a := newAppWith(t, 100, 30, formatters.JSONFormat{})
	press(a, "tab", "tab", "[1]")
	send(a, keyCtrl('s'))
	if a.prompt == nil || !strings.Contains(ansi.Strip(a.View().Content), "output.json") {
		t.Fatal("ctrl+s did not open the prompt with output.json")
	}
	path := filepath.Join(t.TempDir(), "out.json")
	send(a, keyCtrl('u')) // clears the suggested value
	press(a, path)
	send(a, keyEnter())
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "[\n  1\n]\n" {
		t.Fatalf("file = %q, %v", data, err)
	}
	if a.prompt != nil || !strings.HasPrefix(a.flash, "saved ") {
		t.Errorf("prompt open = %v, flash = %q", a.prompt != nil, a.flash)
	}
}

func TestHelpOverlay(t *testing.T) {
	a := newApp(t, 100, 30)
	press(a, "?")
	if !a.showHelp || !strings.Contains(ansi.Strip(a.View().Content), "Shortcuts") {
		t.Fatal("? did not show the help")
	}
	press(a, "x")
	if a.showHelp {
		t.Error("a key did not close the help")
	}
	press(a, "tab", "tab", "?") // inside the input, "?" is text
	if a.showHelp || a.workspace().ActiveValue() != "?" {
		t.Errorf("in the input '?' opened the help or was not typed")
	}
}

func TestSidebarFilterInApp(t *testing.T) {
	a := newAppWith(t, 100, 30, tools.All()...)
	press(a, "/", "b64q")
	if a.active != "base64" {
		t.Errorf("active = %q, want base64", a.active)
	}
	press(a, "x") // "q" was typed into the filter: the app is still alive
	if _, cmd := a.Update(tea.KeyPressMsg{Code: 'q', Text: "q"}); cmd != nil && isQuit(cmd()) {
		t.Error("q quit while filtering")
	}
}
