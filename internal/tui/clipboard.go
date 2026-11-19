package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/Judamaro/devtooltui-golang/internal/detect"
	"github.com/Judamaro/devtooltui-golang/internal/tui/palette"
)

// Clipboard is what the TUI needs from the clipboard.
type Clipboard interface {
	Read() (string, error)
	Write(text string) error
	UseOSC52() bool // true: copy with tea.SetClipboard (OSC 52)
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

// flashMsg shows a notice in the statusbar until the next key.
type flashMsg string

func (a *App) readClipboard(p clipPurpose) tea.Cmd {
	if a.opts.Clipboard == nil {
		return flash("no clipboard available")
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
		return flash("no output to copy")
	}
	out := ws.Output()
	clip := a.opts.Clipboard
	if clip == nil || clip.UseOSC52() {
		return tea.Batch(tea.SetClipboard(out), flash("output copied (OSC 52)"))
	}
	return func() tea.Msg { return clipWroteMsg{err: clip.Write(out)} }
}

func flash(s string) tea.Cmd { return func() tea.Msg { return flashMsg(s) } }

// handleClipRead pastes or detects with what was read from the clipboard.
func (a *App) handleClipRead(msg clipReadMsg) tea.Cmd {
	if msg.err != nil {
		return flash("clipboard: " + msg.err.Error())
	}
	if msg.text == "" {
		return flash("the clipboard is empty")
	}
	if msg.purpose == clipPaste {
		ws := a.workspace()
		if ws == nil || !ws.HasInput() {
			return flash("this tool has no input")
		}
		focusCmd := a.setFocus(focusInput)
		return tea.Batch(focusCmd, ws.Update(tea.PasteMsg{Content: msg.text}))
	}

	out := detect.Run(a.reg, msg.text)
	switch {
	case out.Best != nil:
		id := out.Best.Tool.Spec().ID
		cmd := a.jumpTo(id, msg.text)
		return tea.Batch(cmd, flash(fmt.Sprintf("detected: %s (%.2f)", id, out.Best.Score)))
	case out.Ambiguous:
		var ids []string
		for _, c := range out.Candidates {
			ids = append(ids, c.Tool.Spec().ID)
		}
		return a.openPicker(ids, msg.text)
	default:
		return flash("the clipboard format was not recognised")
	}
}

// jumpTo activates tool id, loads text into its input and focuses it.
func (a *App) jumpTo(id, text string) tea.Cmd {
	activate := a.activate(id)
	a.sidebar.Select(id)
	ws := a.workspace()
	if ws == nil {
		return activate
	}
	return tea.Batch(activate, ws.SetInput(text), a.setFocus(focusInput))
}

// openPicker opens the palette with only the tied candidates; choosing one
// jumps to that tool with text loaded.
func (a *App) openPicker(ids []string, text string) tea.Cmd {
	var items []palette.Item
	for _, id := range ids {
		if t, ok := a.reg.Get(id); ok {
			items = append(items, toolItem(t.Spec()))
		}
	}
	a.pendingText = text
	return a.openPalette("Several tools match: pick one", items)
}
