package tui

import (
	"charm.land/bubbles/v2/key"

	"github.com/Judamaro/devtooltui-golang/internal/tui/sidebar"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace/options"
)

// KeyMap es el keymap central de la TUI.
type KeyMap struct {
	NextFocus     key.Binding
	PrevFocus     key.Binding
	ToggleSidebar key.Binding
	Run           key.Binding // F5: herramientas no-live y entradas grandes
	ClearInput    key.Binding // sale del modo de entrada grande
	Quit          key.Binding
	QuitOutside   key.Binding // "q": solo fuera de los campos de texto
	Sidebar       sidebar.KeyMap
	Options       options.KeyMap
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		NextFocus:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "foco")),
		PrevFocus:     key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "foco anterior")),
		ToggleSidebar: key.NewBinding(key.WithKeys("ctrl+b"), key.WithHelp("ctrl+b", "sidebar")),
		Run:           key.NewBinding(key.WithKeys("f5"), key.WithHelp("f5", "ejecutar")),
		ClearInput:    key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("ctrl+l", "limpiar entrada")),
		Quit:          key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "salir")),
		QuitOutside:   key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "salir")),
		Sidebar: sidebar.KeyMap{
			Up:   key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "subir")),
			Down: key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "bajar")),
		},
		Options: options.KeyMap{
			Prev:     key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "opción anterior")),
			Next:     key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "opción siguiente")),
			Decrease: key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "valor anterior")),
			Increase: key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "valor siguiente")),
			Toggle:   key.NewBinding(key.WithKeys("space"), key.WithHelp("espacio", "activar")),
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
		{k.NextFocus, k.PrevFocus, k.ToggleSidebar, k.Run, k.ClearInput},
		{k.Sidebar.Up, k.Sidebar.Down, k.Quit, k.QuitOutside},
		{k.Options.Prev, k.Options.Next, k.Options.Decrease, k.Options.Increase, k.Options.Toggle},
	}
}
