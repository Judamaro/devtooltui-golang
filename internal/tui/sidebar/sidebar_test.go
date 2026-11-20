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
		view      []string // lines that must be visible
		hidden    []string // texts that must not be visible
	}{
		{"navigates", []string{"down", "down"}, "url", false, []string{"▾ Formatters"}, nil},
		{"filters and hides categories", []string{"/", "b64"}, "base64", true, []string{"/ b64", "▾ Encoders", "▸ Base64"}, []string{"Formatters", "URL"}},
		{"keywords", []string{"/", "pretty"}, "json-format", true, []string{"▸ JSON"}, []string{"Base64"}},
		{"j and k are text while filtering", []string{"/", "json"}, "json-format", true, []string{"/ json"}, nil},
		{"enter keeps the filter", []string{"/", "u", "enter"}, "url", false, []string{"/ u"}, []string{"▏"}},
		{"esc clears it", []string{"/", "b64", "esc"}, "base64", false, []string{"▾ Formatters"}, []string{"/ "}},
		{"backspace", []string{"/", "zz", "backspace", "backspace"}, "json-format", true, []string{"▸ JSON", "Base64"}, nil},
		{"no results", []string{"/", "zzz"}, "json-format", true, []string{"(no results)"}, nil},
		{"navigates within the filter", []string{"/", "s", "down"}, "base64", true, []string{"▸ Base64"}, []string{"URL"}},
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
					t.Errorf("%q is not visible:\n%s", w, view)
				}
			}
			for _, h := range tt.hidden {
				if strings.Contains(view, h) {
					t.Errorf("%q is visible:\n%s", h, view)
				}
			}
		})
	}
}

func TestSelectClearsHidingFilter(t *testing.T) {
	m := press(newSidebar(t), "/", "b64", "enter")
	m.Select("url")
	if m.Selected() != "url" || strings.Contains(ansi.Strip(m.View()), "/ b64") {
		t.Errorf("Select did not clear the filter hiding the tool")
	}
}

func TestScrollMarkers(t *testing.T) {
	var tools []tool.Tool
	for i := range 12 {
		tools = append(tools, t1{id: string(rune('a'+i)) + "-tool", name: "Tool " + string(rune('A'+i)), cat: tool.Text})
	}
	reg, err := registry.New(tools...)
	if err != nil {
		t.Fatal(err)
	}
	m := sidebar.New(reg.ByCategory(), sidebar.Styles{}, keys)
	m.SetSize(25, 6) // 13 rows (a header and 12 tools) in 6
	m.Focus()
	lines := func() []string { return strings.Split(ansi.Strip(m.View()), "\n") }

	got := lines()
	if strings.TrimSpace(got[5]) != "↓ 8 more" || strings.Contains(got[0], "more") {
		t.Errorf("at the top:\n%s", strings.Join(got, "\n"))
	}
	for i := range 11 {
		m = press(m, "down")
		got = lines()
		if len(got) != 6 {
			t.Fatalf("%d lines, want 6", len(got))
		}
		want := "▸ Tool " + string(rune('B'+i))
		found := false
		for _, l := range got {
			found = found || strings.TrimSpace(l) == want
		}
		if !found {
			t.Fatalf("after %d downs the cursor is hidden:\n%s", i+1, strings.Join(got, "\n"))
		}
	}
	if strings.TrimSpace(got[0]) != "↑ 8 more" || strings.Contains(got[5], "more") {
		t.Errorf("at the bottom:\n%s", strings.Join(got, "\n"))
	}
}
