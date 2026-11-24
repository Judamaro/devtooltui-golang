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
	width     int // the screen, from SetSize
	height    int
}

// SetSize tells the prompt the screen size (on open and on resize).
func (p *Prompt) SetSize(width, height int) {
	p.width, p.height = width, height
	p.input.SetWidth(max(boxWidth(width)-6, 1))
}

// box renders the prompt and where it goes, a pure function of the state.
func (p *Prompt) box() (string, geometry, bool) {
	boxW := boxWidth(p.width)
	if boxW < 20 {
		return "", geometry{}, false
	}
	body := p.styles.Title.Render(p.title) + "\n" + p.input.View() + "\n" +
		p.styles.Detail.Render("enter confirm · esc cancel")
	box := p.styles.Box.Width(boxW).Render(body)
	return box, place(box, p.width, p.height), true
}

// Contains reports whether (x, y) is on the prompt.
func (p *Prompt) Contains(x, y int) bool {
	_, g, ok := p.box()
	return ok && g.contains(x, y)
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

// Click inside the prompt does nothing: the field already has the focus.
func (p *Prompt) Click(_, _ int) tea.Cmd { return nil }

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

// Overlay draws the prompt over base without changing it.
func (p *Prompt) Overlay(base string) string {
	box, _, ok := p.box()
	if !ok {
		return base
	}
	return Place(base, box, p.width, p.height)
}
