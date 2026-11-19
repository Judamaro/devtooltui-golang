package tui

import (
	"charm.land/bubbles/v2/key"

	"github.com/Judamaro/devtooltui-golang/internal/tui/sidebar"
)

// KeyMap es el keymap central de la TUI.
type KeyMap struct {
	NextFocus     key.Binding
	PrevFocus     key.Binding
	ToggleSidebar key.Binding
	Quit          key.Binding
	QuitOutside   key.Binding // "q": solo fuera de los campos de texto
	Sidebar       sidebar.KeyMap
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		NextFocus:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "foco")),
		PrevFocus:     key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "foco anterior")),
		ToggleSidebar: key.NewBinding(key.WithKeys("ctrl+b"), key.WithHelp("ctrl+b", "sidebar")),
		Quit:          key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "salir")),
		QuitOutside:   key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "salir")),
		Sidebar: sidebar.KeyMap{
			Up:   key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "subir")),
			Down: key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "bajar")),
		},
	}
}

// ShortHelp implementa help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.NextFocus, k.ToggleSidebar, k.Quit}
}

// FullHelp implementa help.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.NextFocus, k.PrevFocus, k.ToggleSidebar},
		{k.Sidebar.Up, k.Sidebar.Down, k.Quit, k.QuitOutside},
	}
}
