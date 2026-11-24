package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tui/sidebar"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace"
)

// component is the shape of every boxed section of the screen (ADR 0005):
// a pointer whose Update returns only a command, a View without side
// effects, and the size of its box interior set by the App on
// tea.WindowSizeMsg. A child reports to its parent with a typed message
// (sidebar.SelectedMsg, options.ChangedMsg), never with an extra return
// value. Focus, Blur and SetStyles are optional and keep their own names.
type component interface {
	Update(msg tea.Msg) tea.Cmd
	View() string
	SetSize(w, h int)
}

var (
	_ component = (*sidebar.Model)(nil)
	_ component = (*workspace.Model)(nil)
)
