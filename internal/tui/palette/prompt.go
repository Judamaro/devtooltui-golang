package palette

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// PromptDoneMsg is sent when the prompt is confirmed with Enter.
type PromptDoneMsg struct {
	ID, Value string
}

// Prompt asks for a short text (e.g. the path for Ctrl+S) in a box drawn
// over the view. Esc closes it with ClosedMsg.
type Prompt struct {
	id, title string
	input     textinput.Model
	styles    Styles
	keys      KeyMap
}

// NewPrompt opens a prompt with an initial value.
func NewPrompt(id, title, value string, styles Styles, keys KeyMap) *Prompt {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.SetStyles(styles.Input)
	ti.SetValue(value)
	ti.CursorEnd()
	ti.Focus()
	return &Prompt{id: id, title: title, input: ti, styles: styles, keys: keys}
}

func (p *Prompt) Init() tea.Cmd { return textinput.Blink }

func (p *Prompt) Update(msg tea.Msg) tea.Cmd {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(k, p.keys.Close):
			return func() tea.Msg { return ClosedMsg{} }
		case key.Matches(k, p.keys.Choose):
			done := PromptDoneMsg{ID: p.id, Value: p.input.Value()}
			return func() tea.Msg { return done }
		}
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return cmd
}

func (p *Prompt) Overlay(base string, width, height int) string {
	boxW := min(64, width-4)
	if boxW < 20 {
		return base
	}
	p.input.SetWidth(boxW - 6)
	body := p.styles.Title.Render(p.title) + "\n" + p.input.View() + "\n" +
		p.styles.Detail.Render("enter confirm · esc cancel")
	return Place(base, p.styles.Box.Width(boxW).Render(body), width, height)
}
