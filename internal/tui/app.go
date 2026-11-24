// Package tui is the terminal frontend built with Bubble Tea v2.
package tui

import (
	"fmt"
	"maps"
	"os/exec"
	"slices"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Judamaro/devtooltui-golang/internal/history"
	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tui/frame"
	"github.com/Judamaro/devtooltui-golang/internal/tui/palette"
	"github.com/Judamaro/devtooltui-golang/internal/tui/sidebar"
	"github.com/Judamaro/devtooltui-golang/internal/tui/statusbar"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace"
)

const (
	sidebarWidth    = 26  // the Tools box, border included
	sidebarMinWidth = 90  // below this, the sidebar collapses
	splitMinWidth   = 120 // from here on, two inputs sit side by side
)

type focus int

const (
	focusSidebar focus = iota
	focusOptions
	focusInput
	focusOutput
	focusCount
)

// Options configures the App.
type Options struct {
	Debounce      time.Duration // wait between the last key and the run
	Clipboard     Clipboard     // nil: no clipboard
	DetectOnStart bool          // --clipboard: detect the clipboard on start
	// ToolDefaults are the [tools.<id>] option values from the config.
	ToolDefaults map[string]tool.Values
	// Limits are the large input thresholds; zero uses DefaultLimits.
	Limits workspace.Limits
	// Warnings (config problems) are shown in the statusbar on start.
	Warnings []string
	// Theme is "dark" (also when empty), "light" or "auto", which asks the
	// terminal for its background colour.
	Theme string
	// Mouse turns on clicks and the wheel (config "mouse", on by default).
	// The terminal then needs Shift to select text with the mouse.
	Mouse bool
	// Keys are the [keys] overrides from the config.
	Keys map[string]string
	// Restore is the previous session from the history: last tool, options
	// (over ToolDefaults) and inputs.
	Restore history.Session
	// Editor builds the external editor command (Ctrl+E); nil uses
	// $VISUAL/$EDITOR. Tests replace it.
	Editor func(path string) *exec.Cmd
}

// DefaultOptions are the plan's values (120 ms debounce).
func DefaultOptions() Options { return Options{Debounce: 120 * time.Millisecond} }

// App is the root model.
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

	flash       string          // transient notice in the statusbar
	palette     *palette.Model  // nil: closed
	prompt      *palette.Prompt // nil: closed
	showHelp    bool
	pendingText string // text to load when choosing in the detection picker
	export      []byte // binary export offered by the save prompt (Exporter)
	exportExt   string // its extension; other paths save the text output

	width, height int
	wide          bool // width ≥ sidebarMinWidth on the last resize
	showSidebar   bool
}

// New builds the App with the sidebar's first tool active.
func New(reg *registry.Registry, opts Options) *App {
	theme := ThemeFor(opts.Theme != "light")
	keys := DefaultKeyMap()
	opts.Warnings = append(slices.Clone(opts.Warnings), keys.Apply(opts.Keys)...)
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
	if _, ok := reg.Get(opts.Restore.LastTool); ok {
		a.sidebar.Select(opts.Restore.LastTool)
	}
	a.initCmd = a.activate(a.sidebar.Selected())
	a.setFocus(focusSidebar)
	if opts.DetectOnStart {
		a.initCmd = tea.Batch(a.initCmd, a.readClipboard(clipDetect))
	}
	if n := len(opts.Warnings); n > 0 {
		msg := opts.Warnings[0]
		if n > 1 {
			msg += fmt.Sprintf(" (+%d more)", n-1)
		}
		a.initCmd = tea.Batch(a.initCmd, flash("config: "+msg))
	}
	return a
}

func (a *App) Init() tea.Cmd {
	if a.opts.Theme == "auto" {
		return tea.Batch(a.initCmd, tea.RequestBackgroundColor)
	}
	return a.initCmd
}

// setTheme switches between the dark and light themes; workspaces rerun so
// their output is coloured again.
func (a *App) setTheme(dark bool) tea.Cmd {
	if dark == a.theme.Dark {
		return nil
	}
	a.theme = ThemeFor(dark)
	a.help.Styles = a.theme.Help
	a.sidebar.SetStyles(a.theme.Sidebar)
	cmds := make([]tea.Cmd, 0, len(a.spaces))
	for _, ws := range a.spaces {
		cmds = append(cmds, ws.SetStyles(a.theme.Workspace))
	}
	return tea.Batch(cmds...)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		if a.opts.Theme == "auto" {
			return a, a.setTheme(msg.IsDark())
		}
		return a, nil

	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		if wide := msg.Width >= sidebarMinWidth; wide != a.wide {
			a.wide = wide
			a.showSidebar = wide
		}
		a.layout()
		var rewrap tea.Cmd
		if ws := a.workspace(); ws != nil {
			rewrap = ws.Rewrap()
		}
		if !a.showSidebar && a.focus == focusSidebar {
			return a, tea.Batch(rewrap, a.setFocus(focusInput))
		}
		return a, rewrap

	case tea.MouseClickMsg:
		if msg.Mouse().Button == tea.MouseLeft {
			return a, a.click(msg.Mouse().X, msg.Mouse().Y)
		}
		return a, nil

	case tea.MouseWheelMsg:
		return a, a.wheel(msg)

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

	case editorFileMsg:
		return a, a.handleEditorFile(msg)

	case editorDoneMsg:
		return a, a.handleEditorDone(msg)

	case editorTextMsg:
		return a, a.handleEditorText(msg)

	case exportedMsg:
		return a, a.handleExported(msg)

	case savedMsg:
		return a, a.handleSaved(msg)

	case palette.ChosenMsg:
		return a, a.choose(msg.Item)

	case flashMsg:
		a.flash = string(msg)
		return a, nil

	case clipReadMsg:
		return a, a.handleClipRead(msg)

	case clipWroteMsg:
		if msg.err != nil {
			a.flash = "could not copy: " + msg.err.Error()
		} else {
			a.flash = "output copied to the clipboard"
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
			a.showHelp = false // any key closes the help
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
			return a, a.openPalette("Command palette", a.paletteItems())
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

	switch { // cursor blink of the palette or the prompt
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
	if a.opts.Mouse {
		v.MouseMode = tea.MouseModeCellMotion // clicks and the wheel
	}
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
		tools := frame.Box("Tools", a.sidebar.View(), sidebarWidth, a.height-1, a.focus == focusSidebar, a.theme.Frame)
		body = lipgloss.JoinHorizontal(lipgloss.Top, tools, body)
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
		if a.opts.Limits != (workspace.Limits{}) {
			ws.SetLimits(a.opts.Limits)
		}
		vals := tool.Values{}
		maps.Copy(vals, a.opts.ToolDefaults[id])
		maps.Copy(vals, a.opts.Restore.Options[id]) // the last session wins
		if len(vals) > 0 {
			ws.SetOptions(vals)
		}
		if in := a.opts.Restore.Inputs[id]; len(in) > 0 {
			ws.SetInputs(in)
		}
		a.spaces[id] = ws
		initCmd = ws.Init()
	}
	a.active = id
	a.layout()
	var rewrap tea.Cmd
	if initCmd == nil { // an existing workspace may have been laid out at another width
		rewrap = a.spaces[id].Rewrap()
	}
	return tea.Batch(initCmd, rewrap, a.setFocus(a.focus))
}

func (a *App) layout() {
	h := max(a.height-1, 1) // last line: statusbar
	w := a.width
	if a.showSidebar {
		a.sidebar.SetSize(sidebarWidth-2, max(h-2, 1)) // inside the Tools box
		w -= sidebarWidth
	}
	a.help.SetWidth(a.width)
	if ws := a.workspace(); ws != nil {
		ws.SetSize(max(w, 1), h, a.width >= splitMinWidth)
	}
}

func (a *App) rotateFocus(delta int) tea.Cmd {
	// With several inputs, Tab walks the workspace's inputs first.
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

// Run starts the Bubble Tea program.
// Run runs the TUI and returns the session for the history.
func Run(reg *registry.Registry, opts Options) (history.Session, error) {
	m, err := tea.NewProgram(New(reg, opts)).Run()
	if a, ok := m.(*App); ok && err == nil {
		return a.Session(), nil
	}
	return history.Session{}, err
}

// Session returns the active tool and the options and inputs of every tool
// opened in this session.
func (a *App) Session() history.Session {
	s := history.Session{
		LastTool: a.active,
		Options:  map[string]tool.Values{},
		Inputs:   map[string]map[string]string{},
	}
	for id, ws := range a.spaces {
		s.Options[id] = ws.Values()
		s.Inputs[id] = ws.Inputs()
	}
	return s
}
