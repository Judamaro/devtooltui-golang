package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/Judamaro/devtooltui-golang/internal/detect"
	"github.com/Judamaro/devtooltui-golang/internal/tui/palette"
)

// Clipboard es lo que la TUI necesita del portapapeles.
type Clipboard interface {
	Read() (string, error)
	Write(text string) error
	UseOSC52() bool // true: copiar con tea.SetClipboard (OSC 52)
}

type clipPurpose int

const (
	clipPaste clipPurpose = iota
	clipDetect
)

type clipReadMsg struct {
	purpose clipPurpose
	text    string
	err     error
}

type clipWroteMsg struct{ err error }

// flashMsg muestra un aviso en la statusbar hasta la siguente tecla.
type flashMsg string

func (a *App) readClipboard(p clipPurpose) tea.Cmd {
	if a.opts.Clipboard == nil {
		return flash("no hay portapapeles disponible")
	}
	clip := a.opts.Clipboard
	return func() tea.Msg {
		text, err := clip.Read()
		return clipReadMsg{purpose: p, text: text, err: err}
	}
}

func (a *App) copyOutput() tea.Cmd {
	ws := a.workspace()
	if ws == nil || ws.Output() == "" {
		return flash("no hay salida para copiar")
	}
	out := ws.Output()
	clip := a.opts.Clipboard
	if clip == nil || clip.UseOSC52() {
		return tea.Batch(tea.SetClipboard(out), flash("salida copiada (OSC 52)"))
	}
	return func() tea.Msg { return clipWroteMsg{err: clip.Write(out)} }
}

func flash(s string) tea.Cmd { return func() tea.Msg { return flashMsg(s) } }

// handleClipRead pega o detecta con lo leído del portapapeles.
func (a *App) handleClipRead(msg clipReadMsg) tea.Cmd {
	if msg.err != nil {
		return flash("portapapeles: " + msg.err.Error())
	}
	if msg.text == "" {
		return flash("el portapapeles está vacío")
	}
	if msg.purpose == clipPaste {
		ws := a.workspace()
		if ws == nil || !ws.HasInput() {
			return flash("esta herramienta no tiene entrada")
		}
		focusCmd := a.setFocus(focusInput)
		return tea.Batch(focusCmd, ws.Update(tea.PasteMsg{Content: msg.text}))
	}

	out := detect.Run(a.reg, msg.text)
	switch {
	case out.Best != nil:
		id := out.Best.Tool.Spec().ID
		cmd := a.jumpTo(id, msg.text)
		return tea.Batch(cmd, flash(fmt.Sprintf("detectado: %s (%.2f)", id, out.Best.Score)))
	case out.Ambiguous:
		var ids []string
		for _, c := range out.Candidates {
			ids = append(ids, c.Tool.Spec().ID)
		}
		return a.openPicker(ids, msg.text)
	default:
		return flash("no se reconoció el formato del portapapeles")
	}
}

// jumpTo activa la herramienta id, carga text en su entrada y la enfoca.
func (a *App) jumpTo(id, text string) tea.Cmd {
	activate := a.activate(id)
	a.sidebar.Select(id)
	ws := a.workspace()
	if ws == nil {
		return activate
	}
	return tea.Batch(activate, ws.SetInput(text), a.setFocus(focusInput))
}

// openPicker abre la paleta solo con las candidatas empatadas; al elegir,
// se salta a esa herramienta con text cargado.
func (a *App) openPicker(ids []string, text string) tea.Cmd {
	var items []palette.Item
	for _, id := range ids {
		if t, ok := a.reg.Get(id); ok {
			items = append(items, toolItem(t.Spec()))
		}
	}
	a.pendingText = text
	return a.openPalette("Varias herramientas posibles: elige una", items)
}
