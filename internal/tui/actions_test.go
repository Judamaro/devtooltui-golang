package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tools"
	"github.com/Judamaro/devtooltui-golang/internal/tools/encoders"
	"github.com/Judamaro/devtooltui-golang/internal/tools/formatters"
	"github.com/Judamaro/devtooltui-golang/internal/tui/palette"
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
	if !isOpen[*palette.Prompt](a) || !strings.Contains(ansi.Strip(a.View().Content), "output.json") {
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
	if isOpen[*palette.Prompt](a) || !strings.HasPrefix(a.flash, "saved ") {
		t.Errorf("prompt open = %v, flash = %q", isOpen[*palette.Prompt](a), a.flash)
	}
}

func TestHelpOverlay(t *testing.T) {
	a := newApp(t, 100, 30)
	press(a, "?")
	if !isOpen[*helpOverlay](a) || !strings.Contains(ansi.Strip(a.View().Content), "Shortcuts") {
		t.Fatal("? did not show the help")
	}
	press(a, "x")
	if isOpen[*helpOverlay](a) {
		t.Error("a key did not close the help")
	}
	press(a, "tab", "tab", "?") // inside the input, "?" is text
	if isOpen[*helpOverlay](a) || a.workspace().ActiveValue() != "?" {
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

func TestSaveExport(t *testing.T) {
	a := newAppWith(t, 100, 30, encoders.QR{})
	press(a, "tab", "tab", "hola")
	sendSlow(a, keyCtrl('s'))
	if !isOpen[*palette.Prompt](a) || !strings.Contains(ansi.Strip(a.View().Content), "output.png") {
		t.Fatal("ctrl+s did not suggest output.png")
	}
	dir := t.TempDir()
	png := filepath.Join(dir, "code.PNG")
	send(a, keyCtrl('u'))
	press(a, png)
	send(a, keyEnter())
	data, err := os.ReadFile(png)
	if err != nil || !bytes.HasPrefix(data, []byte("\x89PNG")) {
		t.Fatalf("code.PNG = %d bytes, %v; want a PNG", len(data), err)
	}

	// Another extension saves the text art.
	txt := filepath.Join(dir, "code.txt")
	sendSlow(a, keyCtrl('s'))
	send(a, keyCtrl('u'))
	press(a, txt)
	send(a, keyEnter())
	data, err = os.ReadFile(txt)
	if err != nil || !strings.Contains(string(data), "▀") {
		t.Fatalf("code.txt = %q, %v; want the text art", data, err)
	}
}

// TestUpdateDoesNoIO: saving and exporting happen in Cmds, never inside
// Update (ADR 0005). Update returns the Cmd; only running it writes.
func TestUpdateDoesNoIO(t *testing.T) {
	a := newAppWith(t, 100, 30, encoders.QR{})
	press(a, "tab", "tab", "hola")

	// Ctrl+S on an exporting tool: Update only returns the export Cmd.
	_, cmd := a.Update(keyCtrl('s'))
	if isOpen[*palette.Prompt](a) || cmd == nil {
		t.Fatal("the prompt must wait for the export, which runs in a Cmd")
	}
	send(a, cmd()) // the export, waited for however long it takes
	if !isOpen[*palette.Prompt](a) || a.exportExt != ".png" {
		t.Fatalf("after the export: prompt = %v, ext = %q", isOpen[*palette.Prompt](a), a.exportExt)
	}

	path := filepath.Join(t.TempDir(), "code.png")
	_, cmd = a.Update(palette.PromptDoneMsg{ID: "save", Value: path})
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("Update wrote the file itself")
	}
	testutil.RunCmds(cmd, func(m tea.Msg) tea.Cmd { _, c := a.Update(m); return c })
	if data, err := os.ReadFile(path); err != nil || !bytes.HasPrefix(data, []byte("\x89PNG")) {
		t.Fatalf("after the Cmd: %d bytes, %v", len(data), err)
	}
	if !strings.HasPrefix(a.flash, "saved ") {
		t.Errorf("flash = %q", a.flash)
	}
}
