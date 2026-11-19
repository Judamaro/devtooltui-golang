package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools"
)

func keyEsc() tea.KeyPressMsg   { return tea.KeyPressMsg{Code: tea.KeyEscape} }
func keyEnter() tea.KeyPressMsg { return tea.KeyPressMsg{Code: tea.KeyEnter} }

func TestPaletteJumpsToTool(t *testing.T) {
	a := newAppWith(t, 100, 30, tools.All()...)
	send(a, keyCtrl('p'))
	if a.palette == nil {
		t.Fatal("ctrl+p no abrió la paleta")
	}
	press(a, "b64")
	testutil.Golden(t, "tui", "paleta-b64-100x30", plainView(a))
	send(a, keyEnter())
	if a.palette != nil || a.active != "base64" || a.sidebar.Selected() != "base64" {
		t.Errorf("palette abierta = %v, active = %q, sidebar = %q", a.palette != nil, a.active, a.sidebar.Selected())
	}
}

func TestPaletteRunsAction(t *testing.T) {
	clip := &fakeClip{}
	a := newClipApp(t, clip, false)
	press(a, "tab", "tab", "[1]")
	send(a, keyCtrl('p'))
	press(a, "copiar")
	send(a, keyEnter())
	if clip.data != "[\n  1\n]" {
		t.Errorf("la acción no copió: %q", clip.data)
	}
}

func TestPaletteClose(t *testing.T) {
	a := newAppWith(t, 100, 30, tools.All()...)
	send(a, keyCtrl('p'))
	press(a, "q") // con la paleta abierta, "q" se escribe en la búsqueda
	if a.palette == nil {
		t.Fatal("q cerró la paleta o la app")
	}
	send(a, keyEsc())
	if a.palette != nil || a.active != "json-format" {
		t.Errorf("Esc no cerró sin cambiar de herramienta: %v %q", a.palette != nil, a.active)
	}
}

// tie es un detector de puntaje fijo para forzar un empate.
type tie struct {
	id    string
	score float64
}

func (t tie) Spec() tool.Spec {
	return tool.Spec{ID: t.id, Name: t.id, Category: tool.Text, Live: true,
		Inputs: []tool.InputSpec{{Key: "in", Label: "Entrada"}}}
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
	if a.palette == nil || len(a.palette.Filtered()) != 2 {
		t.Fatalf("no se abrió el selector con las 2 candidatas")
	}
	send(a, tea.KeyPressMsg{Code: tea.KeyDown})
	send(a, keyEnter())
	if a.active != "b" || a.workspace().Output() != "b:dato" || a.focus != focusInput {
		t.Errorf("active = %q, salida = %q, focus = %d", a.active, a.workspace().Output(), a.focus)
	}
}
