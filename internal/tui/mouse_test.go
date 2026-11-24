package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Judamaro/devtooltui-golang/internal/tools"
	"github.com/Judamaro/devtooltui-golang/internal/tools/formatters"
)

// find returns the screen cell where text first appears on the view
// (rows and columns in cells, as the terminal reports clicks).
func find(t *testing.T, a *App, text string) (x, y int) {
	t.Helper()
	for row, line := range strings.Split(ansi.Strip(a.View().Content), "\n") {
		if i := strings.Index(line, text); i >= 0 {
			return ansi.StringWidth(line[:i]), row
		}
	}
	t.Fatalf("%q is not on screen:\n%s", text, plainView(a))
	return 0, 0
}

func click(a *App, x, y int) {
	send(a, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
}

func TestClickSidebarTool(t *testing.T) {
	a := newAppWith(t, 100, 30, tools.All()...)
	press(a, "tab") // focus away from the sidebar
	x, y := find(t, a, "XML")
	click(a, x, y)
	if a.active != "xml-format" || a.focus != focusSidebar {
		t.Errorf("active = %q, focus = %v; want xml-format and the sidebar", a.active, a.focus)
	}
	// A category header changes nothing.
	x, y = find(t, a, "Encoders")
	click(a, x, y)
	if a.active != "xml-format" {
		t.Errorf("a click on a category opened %q", a.active)
	}
}

func TestClickOptions(t *testing.T) {
	a := newApp(t, 100, 30)
	press(a, "tab", "tab", "hola")
	x, y := find(t, a, "encode ▶")
	click(a, x+len("encode "), y) // the ▶
	ws := a.workspace()
	if v := ws.Values()["mode"]; v != "decode" || a.focus != focusOptions {
		t.Fatalf("mode = %v, focus = %v; want decode and the options", v, a.focus)
	}
	if ws.Output() != "86895a" { // "hola" decoded, shown as hex
		t.Errorf("the change did not run the tool:\n%s", plainView(a))
	}
	x, y = find(t, a, "◀ decode")
	click(a, x, y) // the ◀ goes back
	if v := ws.Values()["mode"]; v != "encode" {
		t.Errorf("◀: mode = %v", v)
	}
	x, y = find(t, a, "Padding: [x]")
	click(a, x, y) // anywhere on a toggle flips it
	if ws.Values()["padding"] != false {
		t.Errorf("padding = %v after a click", ws.Values()["padding"])
	}
	x, y = find(t, a, "Alphabet")
	click(a, x, y) // the label only selects it
	if v := ws.Values()["variant"]; v != "standard" {
		t.Errorf("a click on the label changed variant to %v", v)
	}
}

func TestClickBoxesFocus(t *testing.T) {
	a := newApp(t, 100, 30)
	_, y := find(t, a, "┌Output")
	click(a, 50, y+2)
	if a.focus != focusOutput {
		t.Errorf("focus = %v, want output", a.focus)
	}
	_, y = find(t, a, "┌Input")
	click(a, 50, y+3)
	if a.focus != focusInput || !a.capturesText() {
		t.Errorf("focus = %v, want the input", a.focus)
	}
	click(a, 10, 29) // the statusbar does nothing
	if a.focus != focusInput {
		t.Errorf("a click on the statusbar moved the focus to %v", a.focus)
	}
}

func TestClickPlacesCursor(t *testing.T) {
	a := newApp(t, 100, 30)
	press(a, "tab", "tab")
	send(a, tea.PasteMsg{Content: "abc\ndefgh\nij"})
	x, y := find(t, a, "defgh")
	click(a, x+3, y) // on the "g"
	press(a, "X")
	if got := a.workspace().ActiveValue(); got != "abc\ndefXgh\nij" {
		t.Errorf("typed at the wrong place: %q", got)
	}
	_, y = find(t, a, "┌Output")
	click(a, 40, y-2) // below the text: end of the last line
	press(a, "Y")
	if got := a.workspace().ActiveValue(); got != "abc\ndefXgh\nijY" {
		t.Errorf("a click past the text: %q", got)
	}
}

func TestWheelScrollsOutput(t *testing.T) {
	a := newAppWith(t, 100, 30, formatters.JSONFormat{})
	press(a, "tab", "tab")
	send(a, tea.PasteMsg{Content: "[" + strings.Repeat("1,", 40) + "1]"})
	_, y := find(t, a, "┌Output")
	before := strings.Split(plainView(a), "\n")[y+1]
	send(a, tea.MouseWheelMsg{X: 60, Y: y + 3, Button: tea.MouseWheelDown})
	after := strings.Split(plainView(a), "\n")[y+1]
	if before == after {
		t.Errorf("the wheel did not scroll the output: %q", after)
	}
	if a.focus != focusInput {
		t.Error("the wheel must not move the focus")
	}
}

func TestWheelOnTools(t *testing.T) {
	a := newAppWith(t, 100, 30, tools.All()...)
	first := a.active
	send(a, tea.MouseWheelMsg{X: 5, Y: 5, Button: tea.MouseWheelDown})
	if a.active == first {
		t.Error("the wheel on Tools did not move the selection")
	}
}

func TestClickPalette(t *testing.T) {
	a := newAppWith(t, 100, 30, tools.All()...)
	send(a, keyCtrl('p'))
	press(a, "json")
	x, y := find(t, a, "JSON ↔ YAML")
	click(a, x, y)
	if a.palette != nil || a.active != "json-yaml" {
		t.Errorf("palette open = %v, active = %q; want json-yaml", a.palette != nil, a.active)
	}
	send(a, keyCtrl('p'))
	click(a, 1, 28) // no View needed: the layout comes from SetSize
	if a.palette != nil {
		t.Error("a click outside did not close the palette")
	}
}

func TestClickClosesOverlays(t *testing.T) {
	a := newApp(t, 100, 30)
	press(a, "?")
	click(a, 50, 15)
	if a.showHelp {
		t.Error("a click did not close the help")
	}
	press(a, "tab", "tab", "x")
	send(a, keyCtrl('s'))
	click(a, 1, 28)
	if a.prompt != nil {
		t.Error("a click outside did not close the save prompt")
	}
}

func TestMouseMode(t *testing.T) {
	a := newApp(t, 100, 30)
	if a.View().MouseMode != tea.MouseModeNone {
		t.Error("the mouse is on without Options.Mouse")
	}
	a.opts.Mouse = true
	if a.View().MouseMode != tea.MouseModeCellMotion {
		t.Error("Options.Mouse did not turn the mouse on")
	}
}
