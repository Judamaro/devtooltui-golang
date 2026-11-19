package tui

import (
	"charm.land/bubbles/v2/key"

	"github.com/Judamaro/devtooltui-golang/internal/tui/palette"
	"github.com/Judamaro/devtooltui-golang/internal/tui/sidebar"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace/options"
)

// KeyMap is the TUI's central keymap.
type KeyMap struct {
	NextFocus     key.Binding
	PrevFocus     key.Binding
	ToggleSidebar key.Binding
	Run           key.Binding // F5: non-live tools and large inputs
	ClearInput    key.Binding // leaves large input mode
	CopyOutput    key.Binding
	PasteInput    key.Binding
	DetectClip    key.Binding
	OpenPalette   key.Binding
	UseOutput     key.Binding
	EditInput     key.Binding
	SaveOutput    key.Binding
	Help          key.Binding
	Quit          key.Binding
	QuitOutside   key.Binding // "q": only outside text fields
	Sidebar       sidebar.KeyMap
	Options       options.KeyMap
	Palette       palette.KeyMap
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		NextFocus:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "focus")),
		PrevFocus:     key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "previous focus")),
		ToggleSidebar: key.NewBinding(key.WithKeys("ctrl+b"), key.WithHelp("ctrl+b", "sidebar")),
		Run:           key.NewBinding(key.WithKeys("f5"), key.WithHelp("f5", "run")),
		ClearInput:    key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("ctrl+l", "clear input")),
		CopyOutput:    key.NewBinding(key.WithKeys("ctrl+y"), key.WithHelp("ctrl+y", "copy output")),
		PasteInput:    key.NewBinding(key.WithKeys("ctrl+o"), key.WithHelp("ctrl+o", "paste")),
		DetectClip:    key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "detect clipboard")),
		OpenPalette:   key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "palette")),
		UseOutput:     key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "output → input")),
		EditInput:     key.NewBinding(key.WithKeys("ctrl+e"), key.WithHelp("ctrl+e", "edit in $EDITOR")),
		SaveOutput:    key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save output")),
		Help:          key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Palette: palette.KeyMap{
			Up:     key.NewBinding(key.WithKeys("up", "ctrl+k")),
			Down:   key.NewBinding(key.WithKeys("down", "ctrl+j")),
			Choose: key.NewBinding(key.WithKeys("enter")),
			Close:  key.NewBinding(key.WithKeys("esc")),
		},
		Quit:        key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
		QuitOutside: key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		Sidebar: sidebar.KeyMap{
			Up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
			Down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
			Filter:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
			EndFilter:   key.NewBinding(key.WithKeys("enter")),
			ClearFilter: key.NewBinding(key.WithKeys("esc")),
		},
		Options: options.KeyMap{
			Prev:     key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "previous option")),
			Next:     key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "next option")),
			Decrease: key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "previous value")),
			Increase: key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "next value")),
			Toggle:   key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle")),
		},
	}
}

// ShortHelp implements help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.NextFocus, k.OpenPalette, k.CopyOutput, k.Help}
}

// FullHelp implements help.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.NextFocus, k.PrevFocus, k.ToggleSidebar, k.Run, k.ClearInput},
		{k.OpenPalette, k.CopyOutput, k.PasteInput, k.DetectClip},
		{k.Sidebar.Up, k.Sidebar.Down, k.Sidebar.Filter, k.Quit, k.QuitOutside},
		{k.UseOutput, k.EditInput, k.SaveOutput, k.Help},
		{k.Options.Prev, k.Options.Next, k.Options.Decrease, k.Options.Increase, k.Options.Toggle},
	}
}
