// Package tui es el frontend de terminal construido con Bubble Tea v2.
package tui

import (
	"os/exec"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/tui/palette"
	"github.com/Judamaro/devtooltui-golang/internal/tui/sidebar"
	"github.com/Judamaro/devtooltui-golang/internal/tui/statusbar"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace"
)

const (
	sidebarWidth    = 26  // incluye la columna del separador
	sidebarMinWidth = 90  // por debajo, el sidebar se colapsa
	splitMinWidth   = 120 // desde aquí, dos entradas van lado a lado
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
	Debounce      time.Duration // espera entre la última tecla y la ejecución
	Clipboard     Clipboard     // nil: sin portapapeles
	DetectOnStart bool          // --clipboard: detectar el portapapeles al abrir
	// Editor arma el comando del editor externo (Ctrl+E); nil usa
	// $VISUAL/$EDITOR. Los tests lo reemplazan.
	Editor func(path string) *exec.Cmd
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

	flash       string          // aviso transitorio en la statusbar
	palette     *palette.Model  // nil: cerrada
	prompt      *palette.Prompt // nil: cerrado
	showHelp    bool
	pendingText string // texto a cargar al elegir en el selector de detección

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
	if opts.DetectOnStart {
		a.initCmd = tea.Batch(a.initCmd, a.readClipboard(clipDetect))
	}
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

	case palette.ClosedMsg:
		a.palette, a.prompt, a.pendingText = nil, nil, ""
		return a, nil

	case palette.PromptDoneMsg:
		a.prompt = nil
		if msg.ID == "save" {
			return a, a.saveOutput(msg.Value)
		}
		return a, nil

	case editorDoneMsg:
		return a, a.handleEditorDone(msg)

	case palette.ChosenMsg:
		return a, a.choose(msg.Item)

	case flashMsg:
		a.flash = string(msg)
		return a, nil

	case clipReadMsg:
		return a, a.handleClipRead(msg)

	case clipWroteMsg:
		if msg.err != nil {
			a.flash = "no se pudo copiar: " + msg.err.Error()
		} else {
			a.flash = "salida copiada al portapapeles"
		}
		return a, nil

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
		a.flash = ""
		switch {
		case a.showHelp:
			a.showHelp = false // cualquier tecla cierra la ayuda
			return a, nil
		case a.prompt != nil:
			return a, a.prompt.Update(msg)
		case a.palette != nil:
			return a, a.palette.Update(msg)
		}
		switch {
		case key.Matches(msg, a.keys.Help) && !a.capturesText():
			a.showHelp = true
			return a, nil
		case key.Matches(msg, a.keys.UseOutput):
			return a, a.useOutput()
		case key.Matches(msg, a.keys.EditInput):
			return a, a.editInput()
		case key.Matches(msg, a.keys.SaveOutput):
			return a, a.askSave()
		case key.Matches(msg, a.keys.OpenPalette):
			return a, a.openPalette("Paleta de comandos", a.paletteItems())
		case key.Matches(msg, a.keys.CopyOutput):
			return a, a.copyOutput()
		case key.Matches(msg, a.keys.PasteInput):
			return a, a.readClipboard(clipPaste)
		case key.Matches(msg, a.keys.DetectClip):
			return a, a.readClipboard(clipDetect)
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
			return a, a.toggleSidebar()
		}
		if a.focus == focusSidebar {
			var cmd tea.Cmd
			a.sidebar, cmd = a.sidebar.Update(msg)
			return a, cmd
		}
	}

	switch { // parpadeo del cursor de la paleta o del prompt
	case a.prompt != nil:
		return a, a.prompt.Update(msg)
	case a.palette != nil:
		return a, a.palette.Update(msg)
	}
	if ws := a.workspace(); ws != nil {
		return a, ws.Update(msg)
	}
	return a, nil
}

func (a *App) View() tea.View {
	content := a.render()
	switch {
	case a.showHelp:
		content = a.helpOverlay(content)
	case a.prompt != nil:
		content = a.prompt.Overlay(content, a.width, a.height)
	case a.palette != nil:
		content = a.palette.Overlay(content, a.width, a.height)
	}
	v := tea.NewView(content)
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
	if a.flash != "" {
		st.Note = a.flash
	}
	return body + "\n" + statusbar.View(st, a.help.View(a.keys), a.width, a.theme.Status)
}

func (a *App) workspace() *workspace.Model { return a.spaces[a.active] }

func (a *App) capturesText() bool {
	if a.focus == focusSidebar {
		return a.sidebar.Filtering()
	}
	ws := a.workspace()
	return ws != nil && ws.CapturesText()
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
		ws.SetSize(max(w, 1), h, a.width >= splitMinWidth)
	}
}

func (a *App) rotateFocus(delta int) tea.Cmd {
	// Con varias entradas, Tab recorre primero las del workspace.
	if ws := a.workspace(); ws != nil && a.focus == focusInput {
		if moved, cmd := ws.MoveInput(delta); moved {
			return cmd
		}
	}
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
	if ws := a.workspace(); ws != nil && f == focusInput {
		ws.FocusEdge(delta > 0)
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
func Run(reg *registry.Registry, opts Options) error {
	_, err := tea.NewProgram(New(reg, opts)).Run()
	return err
}
