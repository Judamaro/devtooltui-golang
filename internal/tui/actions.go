package tui

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tui/palette"
	"github.com/Judamaro/devtooltui-golang/internal/tui/statusbar"
)

// useOutput copies the output into the input to chain tools
// (Ctrl+R).
func (a *App) useOutput() tea.Cmd {
	ws := a.workspace()
	if ws == nil || ws.Output() == "" {
		return flash("no output to use as input")
	}
	if !ws.HasInput() {
		return flash("this tool has no input")
	}
	return tea.Batch(ws.SetInput(ws.Output()), flash("output used as input"))
}

// Messages of the effects below: Update never touches the disk or runs
// an export itself (ADR 0005); it returns a Cmd and handles its message.
type (
	// editorFileMsg: the temporary file for the editor is ready (or not).
	editorFileMsg struct {
		path string
		err  error
	}
	// editorDoneMsg arrives when the external editor exits.
	editorDoneMsg struct {
		path string
		err  error
	}
	// editorTextMsg carries what the editor left in the file.
	editorTextMsg struct {
		text string
		err  error
	}
	// exportedMsg carries the binary form of the output, offered by Ctrl+S.
	exportedMsg struct {
		toolID string
		data   []byte
		ext    string
		err    error
	}
	// savedMsg reports Ctrl+S.
	savedMsg struct {
		path     string // absolute when known
		size     int
		replaced bool
		err      error
	}
)

// editorCommand builds the $VISUAL or $EDITOR command (with its arguments,
// e.g. "code --wait"); without either it uses vi (notepad on Windows).
func editorCommand(path string) *exec.Cmd {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
		if runtime.GOOS == "windows" {
			editor = "notepad"
		}
	}
	fields := strings.Fields(editor)
	return exec.Command(fields[0], append(fields[1:], path)...) //nolint:gosec // the user chooses the editor
}

// editInput opens the active input in the external editor (Ctrl+E). It is
// also the way to edit an input in large mode. Writing the temporary file
// happens in a Cmd; editorFileMsg then starts the editor.
func (a *App) editInput() tea.Cmd {
	ws := a.workspace()
	if ws == nil || !ws.HasInput() {
		return flash("this tool has no input")
	}
	text := ws.ActiveValue()
	return func() tea.Msg {
		f, err := os.CreateTemp("", "devtooltui-*.txt")
		if err != nil {
			return editorFileMsg{err: err}
		}
		_, werr := f.WriteString(text)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			_ = os.Remove(f.Name())
			return editorFileMsg{err: werr}
		}
		return editorFileMsg{path: f.Name()}
	}
}

func (a *App) handleEditorFile(msg editorFileMsg) tea.Cmd {
	if msg.err != nil {
		return flash("could not write the temporary file: " + msg.err.Error())
	}
	newCmd := a.opts.Editor
	if newCmd == nil {
		newCmd = editorCommand
	}
	path := msg.path
	return tea.ExecProcess(newCmd(path), func(err error) tea.Msg { return editorDoneMsg{path: path, err: err} })
}

// handleEditorDone reads the edited text and removes the temporary file,
// in a Cmd.
func (a *App) handleEditorDone(msg editorDoneMsg) tea.Cmd {
	return func() tea.Msg {
		defer os.Remove(msg.path)
		if msg.err != nil {
			return editorTextMsg{err: fmt.Errorf("the editor exited with an error: %w", msg.err)}
		}
		data, err := os.ReadFile(msg.path)
		if err != nil {
			return editorTextMsg{err: fmt.Errorf("could not read the edited text: %w", err)}
		}
		return editorTextMsg{text: string(data)}
	}
}

// handleEditorText loads the edited text into the input.
func (a *App) handleEditorText(msg editorTextMsg) tea.Cmd {
	if msg.err != nil {
		return flash(msg.err.Error())
	}
	ws := a.workspace()
	if ws == nil {
		return nil
	}
	text := msg.text
	// Editors add a final line break; drop it if it was not there before.
	if !strings.HasSuffix(ws.ActiveValue(), "\n") {
		text = strings.TrimSuffix(text, "\n")
	}
	return tea.Batch(ws.SetInput(text), a.setFocus(focusInput))
}

var langExt = map[string]string{"json": ".json", "yaml": ".yaml", "xml": ".xml", "diff": ".diff", "diff-side": ".txt"}

// askSave asks for the path to save the output to (Ctrl+S). A tool with an
// Exporter (qr) runs it first, in a Cmd (a plugin's export is a process):
// exportedMsg then opens the prompt suggesting its extension.
func (a *App) askSave() tea.Cmd {
	ws := a.workspace()
	if ws == nil || ws.Output() == "" {
		return flash("no output to save")
	}
	a.export, a.exportExt = nil, ""
	if cmd := ws.ExportCmd(func(id string, data []byte, ext string, err error) tea.Msg {
		return exportedMsg{toolID: id, data: data, ext: ext, err: err}
	}); cmd != nil {
		return cmd
	}
	return a.openSavePrompt("")
}

// handleExported keeps the export for saveOutput and opens the prompt. A
// failed export still lets the user save the text.
func (a *App) handleExported(msg exportedMsg) tea.Cmd {
	if msg.toolID != a.active {
		return nil // the user moved on to another tool
	}
	if msg.err == nil {
		a.export, a.exportExt = msg.data, msg.ext
		return a.openSavePrompt(msg.ext)
	}
	return a.openSavePrompt("")
}

func (a *App) openSavePrompt(ext string) tea.Cmd {
	if ext == "" {
		if ext = langExt[a.workspace().OutputLang()]; ext == "" {
			ext = ".txt"
		}
	}
	return a.pushOverlay(palette.NewPrompt("save", "Save the output to", "output"+ext, a.theme.Palette, a.keys.Palette))
}

// saveOutput writes the export, when the path has its extension, or the
// text output; the write happens in a Cmd that answers savedMsg.
func (a *App) saveOutput(path string) tea.Cmd {
	ws := a.workspace()
	path = strings.TrimSpace(path)
	if ws == nil || path == "" {
		return flash("not saved: missing path")
	}
	data := []byte(ws.Output())
	if a.export != nil && strings.EqualFold(filepath.Ext(path), a.exportExt) {
		data = a.export
	} else if !bytes.HasSuffix(data, []byte("\n")) {
		data = append(data, '\n')
	}
	a.export, a.exportExt = nil, ""
	return func() tea.Msg { return writeFile(path, data) }
}

func writeFile(path string, data []byte) savedMsg {
	msg := savedMsg{path: path, size: len(data)}
	if _, err := os.Stat(path); err == nil {
		msg.replaced = true
	} else if !errors.Is(err, os.ErrNotExist) {
		msg.err = err
		return msg
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		msg.err = err
		return msg
	}
	if abs, err := filepath.Abs(path); err == nil {
		msg.path = abs
	}
	return msg
}

func (a *App) handleSaved(msg savedMsg) tea.Cmd {
	if msg.err != nil {
		return flash("not saved: " + msg.err.Error())
	}
	verb := "saved"
	if msg.replaced {
		verb = "replaced"
	}
	return flash(fmt.Sprintf("%s %s (%s)", verb, msg.path, statusbar.HumanBytes(msg.size)))
}

// openHelp shows the full help over the view (?).
func (a *App) openHelp() tea.Cmd { return a.pushOverlay(&helpOverlay{app: a}) }
