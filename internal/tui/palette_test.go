package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools"
	"github.com/Judamaro/devtooltui-golang/internal/tui/palette"
)

func keyEsc() tea.KeyPressMsg   { return tea.KeyPressMsg{Code: tea.KeyEscape} }
func keyEnter() tea.KeyPressMsg { return tea.KeyPressMsg{Code: tea.KeyEnter} }

func TestPaletteJumpsToTool(t *testing.T) {
	a := newAppWith(t, 100, 30, tools.All()...)
	send(a, keyCtrl('p'))
	if !isOpen[*palette.Model](a) {
		t.Fatal("ctrl+p did not open the palette")
	}
	press(a, "b64")
	testutil.Golden(t, "tui", "palette-b64-100x30", plainView(a))
	send(a, keyEnter())
	if isOpen[*palette.Model](a) || a.active != "base64" || a.sidebar.Selected() != "base64" {
		t.Errorf("palette open = %v, active = %q, sidebar = %q", isOpen[*palette.Model](a), a.active, a.sidebar.Selected())
	}
}

func TestPaletteRunsAction(t *testing.T) {
	clip := &fakeClip{}
	a := newClipApp(t, clip, false)
	press(a, "tab", "tab", "[1]")
	send(a, keyCtrl('p'))
	press(a, "copy output")
	send(a, keyEnter())
	if clip.data != "[\n  1\n]" {
		t.Errorf("the action did not copy: %q", clip.data)
	}
}

func TestPaletteClose(t *testing.T) {
	a := newAppWith(t, 100, 30, tools.All()...)
	send(a, keyCtrl('p'))
	press(a, "q") // with the palette open, "q" is typed into the search
	if !isOpen[*palette.Model](a) {
		t.Fatal("q closed the palette or the app")
	}
	send(a, keyEsc())
	if isOpen[*palette.Model](a) || a.active != "json-format" {
		t.Errorf("Esc did not close without switching tool: %v %q", isOpen[*palette.Model](a), a.active)
	}
}

// tie is a fixed-score detector to force a tie.
type tie struct {
	id    string
	score float64
}

func (t tie) Spec() tool.Spec {
	return tool.Spec{ID: t.id, Name: t.id, Category: tool.Text, Live: true,
		Inputs: []tool.InputSpec{{Key: "in", Label: "Input"}}}
}
func (t tie) Run(_ context.Context, r tool.Request) (tool.Result, error) {
	return tool.Result{Text: t.id + ":" + r.Inputs["in"]}, nil
}
func (t tie) Detect(string) float64 { return t.score }

func TestDetectTieOpensPicker(t *testing.T) {
	reg, err := registry.New(tie{"a", 0.8}, tie{"b", 0.75}, tie{"c", 0.2})
	if err != nil {
		t.Fatal(err)
	}
	a := New(reg, Options{Clipboard: &fakeClip{data: "dato"}})
	send(a, tea.WindowSizeMsg{Width: 100, Height: 30})
	send(a, keyCtrl('d'))
	if !isOpen[*palette.Model](a) || len(paletteOf(t, a).Filtered()) != 2 {
		t.Fatalf("the picker did not open with the 2 candidates")
	}
	send(a, tea.KeyPressMsg{Code: tea.KeyDown})
	send(a, keyEnter())
	if a.active != "b" || a.workspace().Output() != "b:dato" || a.focus != focusInput {
		t.Errorf("active = %q, output = %q, focus = %d", a.active, a.workspace().Output(), a.focus)
	}
}
