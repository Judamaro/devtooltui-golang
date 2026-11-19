package workspace

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// field es una entrada de la herramienta: un textarea editable o, en modo
// de entrada grande (ADR 0002), un viewport de solo lectura.
type field struct {
	spec      tool.InputSpec
	ta        textarea.Model
	large     bool
	largeText string
	largeView viewport.Model
}

func newField(spec tool.InputSpec, styles textarea.Styles) *field {
	ta := textarea.New()
	ta.Placeholder = "Escribe o pega aquí…"
	ta.CharLimit = 0
	ta.MaxHeight = 0
	ta.MaxWidth = 0
	ta.SetStyles(styles)
	return &field{spec: spec, ta: ta, largeView: viewport.New()}
}

func (f *field) value() string {
	if f.large {
		return f.largeText
	}
	return f.ta.Value()
}

func (f *field) setSize(w, h int) {
	f.ta.SetWidth(w)
	f.ta.SetHeight(max(h, 1))
	f.largeView.SetWidth(w)
	f.largeView.SetHeight(max(h, 1))
}

func (f *field) setLarge(s string) {
	f.large = true
	f.largeText = s
	f.largeView.SetContent(s)
	f.ta.Reset()
}

// setValue reemplaza el contenido sin pasar por el textarea (que ignora
// los mensajes si no tiene el foco).
func (f *field) setValue(s string, lim Limits) {
	f.clear()
	if exceeds(s, lim) {
		f.setLarge(s)
		return
	}
	f.ta.SetValue(s)
	f.ta.ShowLineNumbers = len(s) <= lim.LineNumbersMaxBytes
}

func (f *field) clear() {
	f.large = false
	f.largeText = ""
	f.largeView.SetContent("")
	f.ta.Reset()
	f.ta.ShowLineNumbers = true
}

func (f *field) view() string {
	if f.large {
		return f.largeView.View()
	}
	return f.ta.View()
}

// update procesa un mensaje y devuelve si cambió el valor y si hay que
// ejecutar enseguida (pegado que entra en modo grande).
func (f *field) update(msg tea.Msg, lim Limits) (changed, now bool, cmd tea.Cmd) {
	if f.large {
		if p, ok := msg.(tea.PasteMsg); ok {
			f.setLarge(f.largeText + p.Content)
			return true, true, nil
		}
		f.largeView, cmd = f.largeView.Update(msg)
		return false, false, cmd
	}
	before := f.ta.Value()
	if p, ok := msg.(tea.PasteMsg); ok && exceeds(before+p.Content, lim) {
		// Se agrega al final: el textarea truncaría o se congelaría.
		f.setLarge(before + p.Content)
		return true, true, nil
	}
	f.ta, cmd = f.ta.Update(msg)
	after := f.ta.Value()
	if after == before {
		return false, false, cmd
	}
	if exceeds(after, lim) {
		f.setLarge(after)
		return true, true, cmd
	}
	f.ta.ShowLineNumbers = len(after) <= lim.LineNumbersMaxBytes
	return true, false, cmd
}

func exceeds(s string, lim Limits) bool {
	return len(s) > lim.MaxBytes || strings.Count(s, "\n")+1 > lim.MaxLines
}
