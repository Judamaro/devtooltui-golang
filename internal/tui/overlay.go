package tui

import (
	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tui/palette"
)

// overlay is a box drawn over the screen (ADR 0005): the help, the command
// palette, the save prompt. The App keeps them in a stack; the top one
// receives every key, click and message the App does not handle itself,
// and a click outside it (Contains false) closes it. Overlays close by
// answering palette.ClosedMsg, which pops the top. A new overlay only
// implements this interface: App.Update does not change.
type overlay interface {
	Init() tea.Cmd
	Update(msg tea.Msg) tea.Cmd
	SetSize(w, h int)
	Overlay(base string) string // base with the overlay composed over it
	Contains(x, y int) bool
	Click(x, y int) tea.Cmd // a click inside the overlay
}

var (
	_ overlay = (*palette.Model)(nil)
	_ overlay = (*palette.Prompt)(nil)
	_ overlay = (*helpOverlay)(nil)
)

// pushOverlay opens o over the others.
func (a *App) pushOverlay(o overlay) tea.Cmd {
	o.SetSize(a.width, a.height)
	a.overlays = append(a.overlays, o)
	return o.Init()
}

// popOverlay closes the top overlay. The text waiting for the detection
// picker goes with the palette.
func (a *App) popOverlay() {
	n := len(a.overlays)
	if n == 0 {
		return
	}
	if _, ok := a.overlays[n-1].(*palette.Model); ok {
		a.pendingText = ""
	}
	a.overlays[n-1] = nil
	a.overlays = a.overlays[:n-1]
}

// topOverlay returns the open overlay on top, or nil.
func (a *App) topOverlay() overlay {
	if n := len(a.overlays); n > 0 {
		return a.overlays[n-1]
	}
	return nil
}

// topAs returns the top overlay when it is a T.
func topAs[T overlay](a *App) (T, bool) {
	t, ok := a.topOverlay().(T)
	return t, ok
}

// helpOverlay is the full help (?). Any key or click closes it.
type helpOverlay struct {
	app           *App // the theme and keymap may change while it is open
	width, height int
}

func (h *helpOverlay) Init() tea.Cmd          { return nil }
func (h *helpOverlay) SetSize(w, ht int)      { h.width, h.height = w, ht }
func (h *helpOverlay) Contains(_, _ int) bool { return false }
func (h *helpOverlay) Click(_, _ int) tea.Cmd { return nil }

func (h *helpOverlay) Update(msg tea.Msg) tea.Cmd {
	if _, ok := msg.(tea.KeyPressMsg); ok {
		return func() tea.Msg { return palette.ClosedMsg{} }
	}
	return nil
}

func (h *helpOverlay) Overlay(base string) string {
	t := h.app.theme
	m := help.New()
	m.Styles = t.Help
	m.ShowAll = true
	m.SetWidth(max(h.width-8, 20))
	body := t.Palette.Title.Render("Shortcuts") + "\n\n" + m.View(h.app.keys) + "\n\n" +
		t.Palette.Detail.Render("press any key to close")
	return palette.Place(base, t.Palette.Box.Render(body), h.width, h.height)
}
