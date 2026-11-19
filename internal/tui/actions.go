package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"charm.land/bubbles/v2/help"
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

// editorDoneMsg arrives when the external editor exits.
type editorDoneMsg struct {
	path string
	err  error
}

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
// also the way to edit an input in large mode.
func (a *App) editInput() tea.Cmd {
	ws := a.workspace()
	if ws == nil || !ws.HasInput() {
		return flash("this tool has no input")
	}
	f, err := os.CreateTemp("", "devtooltui-*.txt")
	if err != nil {
		return flash("could not create the temporary file: " + err.Error())
	}
	_, werr := f.WriteString(ws.ActiveValue())
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		_ = os.Remove(f.Name())
		return flash("could not write the temporary file: " + werr.Error())
	}
	path := f.Name()
	newCmd := a.opts.Editor
	if newCmd == nil {
		newCmd = editorCommand
	}
	return tea.ExecProcess(newCmd(path), func(err error) tea.Msg { return editorDoneMsg{path: path, err: err} })
}

// handleEditorDone loads the edited text into the input and removes the temp file.
func (a *App) handleEditorDone(msg editorDoneMsg) tea.Cmd {
	defer os.Remove(msg.path)
	if msg.err != nil {
		return flash("the editor exited with an error: " + msg.err.Error())
	}
	data, err := os.ReadFile(msg.path)
	if err != nil {
		return flash("could not read the edited text: " + err.Error())
	}
	ws := a.workspace()
	if ws == nil {
		return nil
	}
	text := string(data)
	// Editors add a final line break; drop it if it was not there before.
	if !strings.HasSuffix(ws.ActiveValue(), "\n") {
		text = strings.TrimSuffix(text, "\n")
	}
	return tea.Batch(ws.SetInput(text), a.setFocus(focusInput))
}

var langExt = map[string]string{"json": ".json", "yaml": ".yaml", "xml": ".xml", "diff": ".diff", "diff-side": ".txt"}

// askSave asks for the path to save the output to (Ctrl+S).
func (a *App) askSave() tea.Cmd {
	ws := a.workspace()
	if ws == nil || ws.Output() == "" {
		return flash("no output to save")
	}
	ext := langExt[ws.OutputLang()]
	if ext == "" {
		ext = ".txt"
	}
	a.prompt = palette.NewPrompt("save", "Save the output to", "output"+ext, a.theme.Palette, a.keys.Palette)
	return a.prompt.Init()
}

func (a *App) saveOutput(path string) tea.Cmd {
	ws := a.workspace()
	path = strings.TrimSpace(path)
	if ws == nil || path == "" {
		return flash("not saved: missing path")
	}
	out := ws.Output()
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	verb := "saved"
	if _, err := os.Stat(path); err == nil {
		verb = "replaced"
	} else if !errors.Is(err, os.ErrNotExist) {
		return flash("not saved: " + err.Error())
	}
	if err := os.WriteFile(path, []byte(out), 0o600); err != nil {
		return flash("not saved: " + err.Error())
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return flash(fmt.Sprintf("%s %s (%s)", verb, abs, statusbar.HumanBytes(len(out))))
}

// helpOverlay draws the full help over the view (?).
func (a *App) helpOverlay(base string) string {
	h := help.New()
	h.Styles = a.theme.Help
	h.ShowAll = true
	h.SetWidth(max(a.width-8, 20))
	body := a.theme.Palette.Title.Render("Shortcuts") + "\n\n" + h.View(a.keys) + "\n\n" +
		a.theme.Palette.Detail.Render("press any key to close")
	box := a.theme.Palette.Box.Render(body)
	return palette.Place(base, box, a.width, a.height)
}
