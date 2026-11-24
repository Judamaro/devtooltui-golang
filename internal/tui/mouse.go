package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace"
)

// click handles a left click at screen cell (x, y). The top overlay gets it
// first: a click outside it closes it (the help has no inside). Otherwise
// the box under the pointer takes the focus and the click acts inside it.
func (a *App) click(x, y int) tea.Cmd {
	a.flash = ""
	if o := a.topOverlay(); o != nil {
		if !o.Contains(x, y) {
			a.popOverlay()
			return nil
		}
		return o.Click(x, y)
	}
	if y >= a.height-1 { // the statusbar
		return nil
	}
	x0 := 0
	if a.showSidebar {
		if x < sidebarWidth {
			cmd := a.setFocus(focusSidebar)
			if x > 0 && x < sidebarWidth-1 && y > 0 {
				cmd = tea.Batch(cmd, a.sidebar.Click(y-1))
			}
			return cmd
		}
		x0 = sidebarWidth
	}
	ws := a.workspace()
	if ws == nil {
		return nil
	}
	f, ok, cmd := ws.Click(x-x0, y)
	if !ok {
		return cmd
	}
	return tea.Batch(cmd, a.setFocus(appFocus(f)))
}

// wheel scrolls the box under the pointer, focused or not. On Tools it
// moves the selection, as ↑/↓ do: the list follows its cursor.
func (a *App) wheel(msg tea.MouseWheelMsg) tea.Cmd {
	m := msg.Mouse()
	if len(a.overlays) > 0 || m.Y >= a.height-1 {
		return nil
	}
	x0 := 0
	if a.showSidebar {
		if m.X < sidebarWidth {
			k := tea.KeyPressMsg{Code: tea.KeyDown}
			if m.Button == tea.MouseWheelUp {
				k = tea.KeyPressMsg{Code: tea.KeyUp}
			}
			return a.sidebar.Update(k)
		}
		x0 = sidebarWidth
	}
	if ws := a.workspace(); ws != nil {
		return ws.Wheel(m.X-x0, m.Y, msg)
	}
	return nil
}

func appFocus(f workspace.Focus) focus {
	switch f {
	case workspace.FocusOptions:
		return focusOptions
	case workspace.FocusInput:
		return focusInput
	}
	return focusOutput
}
