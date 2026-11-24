package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tui/palette"
)

// action is a palette action, tied to a keymap shortcut.
type action struct {
	id, title string
	binding   key.Binding
	run       func(a *App) tea.Cmd
}

func (a *App) actions() []action {
	k := a.keys
	return []action{
		{"copy", "Copy output", k.CopyOutput, (*App).copyOutput},
		{"paste", "Paste the clipboard into the input", k.PasteInput, func(a *App) tea.Cmd { return a.readClipboard(clipPaste) }},
		{"detect", "Detect the clipboard", k.DetectClip, func(a *App) tea.Cmd { return a.readClipboard(clipDetect) }},
		{"run", "Run", k.Run, func(a *App) tea.Cmd {
			if ws := a.workspace(); ws != nil {
				return ws.Run()
			}
			return nil
		}},
		{"clear", "Clear the input", k.ClearInput, func(a *App) tea.Cmd {
			if ws := a.workspace(); ws != nil {
				return ws.ClearInput()
			}
			return nil
		}},
		{"use-output", "Use the output as input", k.UseOutput, (*App).useOutput},
		{"edit", "Edit the input in $EDITOR", k.EditInput, (*App).editInput},
		{"save", "Save the output to a file", k.SaveOutput, (*App).askSave},
		{"help", "Help: all shortcuts", k.Help, func(a *App) tea.Cmd { a.showHelp = true; return nil }},
		{"sidebar", "Show or hide the sidebar", k.ToggleSidebar, (*App).toggleSidebar},
		{"quit", "Quit", k.Quit, func(*App) tea.Cmd { return tea.Quit }},
	}
}

func toolItem(s tool.Spec) palette.Item {
	search := s.ID
	for _, kw := range s.Keywords {
		search += " " + kw
	}
	return palette.Item{Kind: palette.ToolItem, ID: s.ID, Title: s.Name, Detail: string(s.Category), Search: search}
}

func (a *App) paletteItems() []palette.Item {
	var items []palette.Item
	for _, t := range a.reg.All() {
		items = append(items, toolItem(t.Spec()))
	}
	for _, ac := range a.actions() {
		items = append(items, palette.Item{Kind: palette.ActionItem, ID: ac.id, Title: ac.title, Detail: ac.binding.Help().Key})
	}
	return items
}

func (a *App) openPalette(title string, items []palette.Item) tea.Cmd {
	a.palette = palette.New(title, items, a.theme.Palette, a.keys.Palette)
	a.palette.SetSize(a.width, a.height)
	return a.palette.Init()
}

// choose closes the palette and runs what was chosen.
func (a *App) choose(it palette.Item) tea.Cmd {
	text := a.pendingText
	a.palette, a.pendingText = nil, ""
	if it.Kind == palette.ActionItem {
		for _, ac := range a.actions() {
			if ac.id == it.ID {
				return ac.run(a)
			}
		}
		return nil
	}
	if text != "" {
		return a.jumpTo(it.ID, text)
	}
	a.sidebar.Select(it.ID)
	return a.activate(it.ID)
}

func (a *App) toggleSidebar() tea.Cmd {
	a.showSidebar = !a.showSidebar
	a.layout()
	if !a.showSidebar && a.focus == focusSidebar {
		return a.setFocus(focusInput)
	}
	return nil
}
