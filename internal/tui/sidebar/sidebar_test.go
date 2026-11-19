package sidebar_test

import (
	"context"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tui/sidebar"
)

type t1 struct {
	id, name string
	cat      tool.Category
	kw       []string
}

func (x t1) Spec() tool.Spec {
	return tool.Spec{ID: x.id, Name: x.name, Category: x.cat, Keywords: x.kw}
}
func (t1) Run(context.Context, tool.Request) (tool.Result, error) { return tool.Result{}, nil }

var keys = sidebar.KeyMap{
	Up: key.NewBinding(key.WithKeys("up", "k")), Down: key.NewBinding(key.WithKeys("down", "j")),
	Filter: key.NewBinding(key.WithKeys("/")), EndFilter: key.NewBinding(key.WithKeys("enter")),
	ClearFilter: key.NewBinding(key.WithKeys("esc")),
}

func newSidebar(t *testing.T) sidebar.Model {
	reg, err := registry.New(
		t1{"json-format", "JSON", tool.Formatters, []string{"pretty"}},
		t1{"base64", "Base64", tool.Encoders, nil},
		t1{"url", "URL", tool.Encoders, nil},
	)
	if err != nil {
		t.Fatal(err)
	}
	m := sidebar.New(reg.ByCategory(), sidebar.Styles{}, keys)
	m.SetSize(25, 10)
	m.Focus()
	return m
}

func press(m sidebar.Model, keys ...string) sidebar.Model {
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "up":
			msg = tea.KeyPressMsg{Code: tea.KeyUp}
		case "down":
			msg = tea.KeyPressMsg{Code: tea.KeyDown}
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
		case "backspace":
			msg = tea.KeyPressMsg{Code: tea.KeyBackspace}
		default:
			for _, r := range k {
				m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
			}
			continue
		}
		m, _ = m.Update(msg)
	}
	return m
}

func TestFilter(t *testing.T) {
	tests := []struct {
		name      string
		keys      []string
		selected  string
		filtering bool
		view      []string // líneas que deben verse
		hidden    []string // textos que no deben verse
	}{
		{"navega", []string{"down", "down"}, "url", false, []string{"▾ Formateadores"}, nil},
		{"filtra y oculta categorias", []string{"/", "b64"}, "base64", true, []string{"/ b64", "▾ Codificadores", "▸ Base64"}, []string{"Formateadores", "URL"}},
		{"keywords", []string{"/", "pretty"}, "json-format", true, []string{"▸ JSON"}, []string{"Base64"}},
		{"j y k son texto al filtrar", []string{"/", "json"}, "json-format", true, []string{"/ json"}, nil},
		{"enter deja el filtro", []string{"/", "u", "enter"}, "url", false, []string{"/ u"}, []string{"▏"}},
		{"esc lo borra", []string{"/", "b64", "esc"}, "base64", false, []string{"▾ Formateadores"}, []string{"/ "}},
		{"backspace", []string{"/", "zz", "backspace", "backspace"}, "json-format", true, []string{"▸ JSON", "Base64"}, nil},
		{"sin resultados", []string{"/", "zzz"}, "json-format", true, []string{"(sin resultados)"}, nil},
		{"navega dentro del filtro", []string{"/", "s", "down"}, "base64", true, []string{"▸ Base64"}, []string{"URL"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := press(newSidebar(t), tt.keys...)
			view := ansi.Strip(m.View())
			if m.Selected() != tt.selected || m.Filtering() != tt.filtering {
				t.Errorf("Selected = %q, Filtering = %v; want %q, %v", m.Selected(), m.Filtering(), tt.selected, tt.filtering)
			}
			for _, w := range tt.view {
				if !strings.Contains(view, w) {
					t.Errorf("no se ve %q:\n%s", w, view)
				}
			}
			for _, h := range tt.hidden {
				if strings.Contains(view, h) {
					t.Errorf("se ve %q:\n%s", h, view)
				}
			}
		})
	}
}

func TestSelectClearsHidingFilter(t *testing.T) {
	m := press(newSidebar(t), "/", "b64", "enter")
	m.Select("url")
	if m.Selected() != "url" || strings.Contains(ansi.Strip(m.View()), "/ b64") {
		t.Errorf("Select no borró el filtro que ocultaba la herramienta")
	}
}
