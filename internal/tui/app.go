// Package tui es el frontend de terminal construido con Bubble Tea v2.
package tui

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/tui/sidebar"
	"github.com/Judamaro/devtooltui-golang/internal/tui/statusbar"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace"
)

const (
	sidebarWidth    = 26 // incluye la columna del separador
	sidebarMinWidth = 90 // por debajo, el sidebar se colapsa
)

type focus int

const (
	focusSidebar focus = iota
	focusOptions
	focusInput
	focusOutput
	focusCount
)

// Options configura la App.
type Options struct {
	Debounce time.Duration // espera entre la última tecla y la ejecución
}

// DefaultOptions son los valores del plan (debounce de 120 ms).
func DefaultOptions() Options { return Options{Debounce: 120 * time.Millisecond} }

// App es el modelo raíz.
type App struct {
	opts    Options
	initCmd tea.Cmd
	reg     *registry.Registry
	theme   Theme
	keys    KeyMap
	help    help.Model
	sidebar sidebar.Model
	spaces  map[string]*workspace.Model
	active  string
	focus   focus

	width, height int
	wide          bool // ancho ≥ sidebarMinWidth en el último resize
	showSidebar   bool
}

// New crea la App con la primera herramienta del sidebar activa.
func New(reg *registry.Registry, opts Options) *App {
	theme := DarkTheme()
	keys := DefaultKeyMap()
	h := help.New()
	h.Styles = theme.Help
	a := &App{
		opts:        opts,
		reg:         reg,
		theme:       theme,
		keys:        keys,
		help:        h,
		sidebar:     sidebar.New(reg.ByCategory(), theme.Sidebar, keys.Sidebar),
		spaces:      map[string]*workspace.Model{},
		showSidebar: true,
		wide:        true,
	}
	a.initCmd = a.activate(a.sidebar.Selected())
	a.setFocus(focusSidebar)
	return a
}

func (a *App) Init() tea.Cmd { return a.initCmd }

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		if wide := msg.Width >= sidebarMinWidth; wide != a.wide {
			a.wide = wide
			a.showSidebar = wide
		}
		a.layout()
		if !a.showSidebar && a.focus == focusSidebar {
			return a, a.setFocus(focusInput)
		}
		return a, nil

	case sidebar.SelectedMsg:
		return a, a.activate(msg.ID)

	case workspace.DebounceMsg:
		if ws, ok := a.spaces[msg.ToolID]; ok {
			return a, ws.HandleDebounce(msg)
		}
		return a, nil

	case workspace.ResultMsg:
		if ws, ok := a.spaces[msg.ToolID]; ok {
			ws.HandleResult(msg)
		}
		return a, nil

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, a.keys.QuitOutside) && !a.capturesText():
			return a, tea.Quit
		case key.Matches(msg, a.keys.NextFocus):
			return a, a.rotateFocus(1)
		case key.Matches(msg, a.keys.PrevFocus):
			return a, a.rotateFocus(-1)
		case key.Matches(msg, a.keys.Run):
			if ws := a.workspace(); ws != nil {
				return a, ws.Run()
			}
			return a, nil
		case key.Matches(msg, a.keys.ClearInput):
			if ws := a.workspace(); ws != nil {
				return a, ws.ClearInput()
			}
			return a, nil
		case key.Matches(msg, a.keys.ToggleSidebar):
			a.showSidebar = !a.showSidebar
			a.layout()
			if !a.showSidebar && a.focus == focusSidebar {
				return a, a.setFocus(focusInput)
			}
			return a, nil
		}
		if a.focus == focusSidebar {
			var cmd tea.Cmd
			a.sidebar, cmd = a.sidebar.Update(msg)
			return a, cmd
		}
	}

	if ws := a.workspace(); ws != nil {
		return a, ws.Update(msg)
	}
	return a, nil
}

func (a *App) View() tea.View {
	v := tea.NewView(a.render())
	v.AltScreen = true
	return v
}

func (a *App) render() string {
	if a.width == 0 || a.height == 0 {
		return ""
	}
	body := ""
	if ws := a.workspace(); ws != nil {
		body = ws.View()
	}
	if a.showSidebar {
		h := a.height - 1
		sep := a.theme.Separator.Render(strings.TrimSuffix(strings.Repeat("│\n", h), "\n"))
		body = lipgloss.JoinHorizontal(lipgloss.Top, a.sidebar.View(), sep, body)
	}
	var st statusbar.Status
	if ws := a.workspace(); ws != nil {
		st = ws.Status()
	}
	return body + "\n" + statusbar.View(st, a.help.View(a.keys), a.width, a.theme.Status)
}

func (a *App) workspace() *workspace.Model { return a.spaces[a.active] }

func (a *App) capturesText() bool {
	ws := a.workspace()
	return a.focus != focusSidebar && ws != nil && ws.CapturesText()
}

func (a *App) activate(id string) tea.Cmd {
	t, ok := a.reg.Get(id)
	if !ok {
		return nil
	}
	var initCmd tea.Cmd
	if _, ok := a.spaces[id]; !ok {
		ws := workspace.New(t, a.theme.Workspace, a.keys.Options, a.opts.Debounce)
		a.spaces[id] = ws
		initCmd = ws.Init()
	}
	a.active = id
	a.layout()
	return tea.Batch(initCmd, a.setFocus(a.focus))
}

func (a *App) layout() {
	h := max(a.height-1, 1) // última línea: statusbar
	w := a.width
	if a.showSidebar {
		a.sidebar.SetSize(sidebarWidth-1, h)
		w -= sidebarWidth
	}
	a.help.SetWidth(a.width)
	if ws := a.workspace(); ws != nil {
		ws.SetSize(max(w, 1), h)
	}
}

func (a *App) rotateFocus(delta int) tea.Cmd {
	f := a.focus
	for range focusCount {
		f = (f + focus(delta) + focusCount) % focusCount
		if f == focusSidebar && !a.showSidebar {
			continue
		}
		ws := a.workspace()
		if f == focusInput && (ws == nil || !ws.HasInput()) {
			continue
		}
		if f == focusOptions && (ws == nil || !ws.HasOptions()) {
			continue
		}
		break
	}
	return a.setFocus(f)
}

func (a *App) setFocus(f focus) tea.Cmd {
	a.focus = f
	if f == focusSidebar {
		a.sidebar.Focus()
	} else {
		a.sidebar.Blur()
	}
	ws := a.workspace()
	if ws == nil {
		return nil
	}
	switch f {
	case focusOptions:
		return ws.SetFocus(workspace.FocusOptions)
	case focusInput:
		return ws.SetFocus(workspace.FocusInput)
	case focusOutput:
		return ws.SetFocus(workspace.FocusOutput)
	default:
		return ws.SetFocus(workspace.FocusNone)
	}
}

// Run arranca el programa de Bubble Tea.
func Run(reg *registry.Registry) error {
	_, err := tea.NewProgram(New(reg, DefaultOptions())).Run()
	return err
}
