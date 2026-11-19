package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tui/palette"
)

// action es una acción de la paleta, ligada a un atajo del keymap.
type action struct {
	id, title string
	binding   key.Binding
	run       func(a *App) tea.Cmd
}

func (a *App) actions() []action {
	k := a.keys
	return []action{
		{"copy", "Copiar salida", k.CopyOutput, (*App).copyOutput},
		{"paste", "Pegar el portapapeles en la entrada", k.PasteInput, func(a *App) tea.Cmd { return a.readClipboard(clipPaste) }},
		{"detect", "Detectar el portapapeles", k.DetectClip, func(a *App) tea.Cmd { return a.readClipboard(clipDetect) }},
		{"run", "Ejecutar", k.Run, func(a *App) tea.Cmd {
			if ws := a.workspace(); ws != nil {
				return ws.Run()
			}
			return nil
		}},
		{"clear", "Limpiar la entrada", k.ClearInput, func(a *App) tea.Cmd {
			if ws := a.workspace(); ws != nil {
				return ws.ClearInput()
			}
			return nil
		}},
		{"sidebar", "Mostrar u ocultar el sidebar", k.ToggleSidebar, (*App).toggleSidebar},
		{"quit", "Salir", k.Quit, func(*App) tea.Cmd { return tea.Quit }},
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
	return a.palette.Init()
}

// choose cierra la paleta y ejecuta lo elegido.
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
