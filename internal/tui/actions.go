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

// useOutput copia la salida a la entrada para encadenar herramientas
// (Ctrl+R).
func (a *App) useOutput() tea.Cmd {
	ws := a.workspace()
	if ws == nil || ws.Output() == "" {
		return flash("no hay salida para usar como entrada")
	}
	if !ws.HasInput() {
		return flash("esta herramienta no tiene entrada")
	}
	return tea.Batch(ws.SetInput(ws.Output()), flash("salida usada como entrada"))
}

// editorDoneMsg llega cuando el editor externo termina.
type editorDoneMsg struct {
	path string
	err  error
}

// editorCommand arma el comando de $VISUAL o $EDITOR (con sus argumentos,
// p. ej. "code --wait"); si no hay ninguno usa vi (notepad en Windows).
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
	return exec.Command(fields[0], append(fields[1:], path)...) //nolint:gosec // el editor lo elige el usuario
}

// editInput abre la entrada activa en el editor externo (Ctrl+E). También
// es la forma de editar una entrada en modo grande.
func (a *App) editInput() tea.Cmd {
	ws := a.workspace()
	if ws == nil || !ws.HasInput() {
		return flash("esta herramienta no tiene entrada")
	}
	f, err := os.CreateTemp("", "devtooltui-*.txt")
	if err != nil {
		return flash("no se pudo crear el archivo temporal: " + err.Error())
	}
	_, werr := f.WriteString(ws.ActiveValue())
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		_ = os.Remove(f.Name())
		return flash("no se pudo escribir el archivo temporal: " + werr.Error())
	}
	path := f.Name()
	newCmd := a.opts.Editor
	if newCmd == nil {
		newCmd = editorCommand
	}
	return tea.ExecProcess(newCmd(path), func(err error) tea.Msg { return editorDoneMsg{path: path, err: err} })
}

// handleEditorDone carga lo editado en la entrada y borra el temporal.
func (a *App) handleEditorDone(msg editorDoneMsg) tea.Cmd {
	defer os.Remove(msg.path)
	if msg.err != nil {
		return flash("el editor terminó con error: " + msg.err.Error())
	}
	data, err := os.ReadFile(msg.path)
	if err != nil {
		return flash("no se pudo leer lo editado: " + err.Error())
	}
	ws := a.workspace()
	if ws == nil {
		return nil
	}
	text := string(data)
	// Los editores agregan un salto final; se quita si antes no estaba.
	if !strings.HasSuffix(ws.ActiveValue(), "\n") {
		text = strings.TrimSuffix(text, "\n")
	}
	return tea.Batch(ws.SetInput(text), a.setFocus(focusInput))
}

var langExt = map[string]string{"json": ".json", "yaml": ".yaml", "xml": ".xml", "diff": ".diff", "diff-side": ".txt"}

// askSave pide la ruta para guardar la salida (Ctrl+S).
func (a *App) askSave() tea.Cmd {
	ws := a.workspace()
	if ws == nil || ws.Output() == "" {
		return flash("no hay salida para guardar")
	}
	ext := langExt[ws.OutputLang()]
	if ext == "" {
		ext = ".txt"
	}
	a.prompt = palette.NewPrompt("save", "Guardar la salida en", "salida"+ext, a.theme.Palette, a.keys.Palette)
	return a.prompt.Init()
}

func (a *App) saveOutput(path string) tea.Cmd {
	ws := a.workspace()
	path = strings.TrimSpace(path)
	if ws == nil || path == "" {
		return flash("no se guardó: falta la ruta")
	}
	out := ws.Output()
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	verb := "guardado"
	if _, err := os.Stat(path); err == nil {
		verb = "reemplazado"
	} else if !errors.Is(err, os.ErrNotExist) {
		return flash("no se guardó: " + err.Error())
	}
	if err := os.WriteFile(path, []byte(out), 0o600); err != nil {
		return flash("no se guardó: " + err.Error())
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return flash(fmt.Sprintf("%s %s (%s)", verb, abs, statusbar.HumanBytes(len(out))))
}

// helpOverlay dibuja la ayuda completa encima de la vista (?).
func (a *App) helpOverlay(base string) string {
	h := help.New()
	h.Styles = a.theme.Help
	h.ShowAll = true
	h.SetWidth(max(a.width-8, 20))
	body := a.theme.Palette.Title.Render("Atajos") + "\n\n" + h.View(a.keys) + "\n\n" +
		a.theme.Palette.Detail.Render("cualquier tecla cierra")
	box := a.theme.Palette.Box.Render(body)
	return palette.Place(base, box, a.width, a.height)
}
