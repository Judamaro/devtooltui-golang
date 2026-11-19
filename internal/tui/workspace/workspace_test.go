package workspace_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace"
	"github.com/Judamaro/devtooltui-golang/internal/tui/workspace/options"
)

// counter cuenta las ejecuciones y devuelve la longitud de la entrada.
type counter struct {
	live bool
	runs int
	last string
}

func (c *counter) Spec() tool.Spec {
	return tool.Spec{ID: "count", Name: "Count", Category: tool.Text, Live: c.live,
		Inputs: []tool.InputSpec{{Key: "in", Label: "Entrada"}}}
}

func (c *counter) Run(_ context.Context, req tool.Request) (tool.Result, error) {
	c.runs++
	c.last = req.Inputs["in"]
	return tool.Result{Text: strings.Repeat("x", min(len(c.last), 10))}, nil
}

var optKeys = options.KeyMap{
	Prev: key.NewBinding(key.WithKeys("up")), Next: key.NewBinding(key.WithKeys("down")),
	Decrease: key.NewBinding(key.WithKeys("left")), Increase: key.NewBinding(key.WithKeys("right")),
	Toggle: key.NewBinding(key.WithKeys("space")),
}

func newWS(c tool.Tool) *workspace.Model {
	m := workspace.New(c, workspace.Styles{}, optKeys, 0)
	m.SetSize(80, 24)
	m.SetFocus(workspace.FocusInput)
	run(m, m.Init())
	return m
}

// run ejecuta cmd y aplica los resultados al workspace.
func run(m *workspace.Model, cmd tea.Cmd) {
	testutil.RunCmds(cmd, func(msg tea.Msg) tea.Cmd {
		switch msg := msg.(type) {
		case workspace.ResultMsg:
			m.HandleResult(msg)
		case workspace.DebounceMsg:
			return m.HandleDebounce(msg)
		}
		return nil
	})
}

func update(m *workspace.Model, msg tea.Msg) { run(m, m.Update(msg)) }

func typeText(m *workspace.Model, s string) {
	for _, r := range s {
		update(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func TestLargeInput(t *testing.T) {
	big := strings.Repeat("a", workspace.DefaultLimits.MaxBytes+1)
	manyLines := strings.Repeat("a\n", workspace.DefaultLimits.MaxLines)
	small := strings.Repeat("a", 100)
	tests := []struct {
		name      string
		paste     []string
		wantLarge bool
		wantLen   int
	}{
		{"pegado chico se edita", []string{small}, false, 100},
		{"pegado grande pasa a solo lectura", []string{big}, true, len(big)},
		{"demasiadas líneas pasa a solo lectura", []string{manyLines}, true, len(manyLines)},
		{"dos pegados que suman más del umbral", []string{big[:20000], big[:20000]}, true, 40000},
		{"pegar en modo grande agrega", []string{big, "zz"}, true, len(big) + 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &counter{live: true}
			m := newWS(c)
			for _, p := range tt.paste {
				update(m, tea.PasteMsg{Content: p})
			}
			if m.Large() != tt.wantLarge {
				t.Errorf("Large() = %v, want %v", m.Large(), tt.wantLarge)
			}
			if len(c.last) != tt.wantLen {
				t.Errorf("la herramienta recibió %d bytes, want %d", len(c.last), tt.wantLen)
			}
			if st := m.Status(); tt.wantLarge && st.LargeBytes != tt.wantLen {
				t.Errorf("Status().LargeBytes = %d, want %d", st.LargeBytes, tt.wantLen)
			}
		})
	}
}

func TestLargeInputKeys(t *testing.T) {
	c := &counter{live: true}
	m := newWS(c)
	update(m, tea.PasteMsg{Content: strings.Repeat("a", 40000)})
	runs := c.runs

	typeText(m, "zzz") // en solo lectura se ignora la escritura
	if c.runs != runs || len(c.last) != 40000 {
		t.Fatalf("escribir en modo grande re-ejecutó o cambió la entrada (runs %d→%d)", runs, c.runs)
	}
	run(m, m.Run()) // F5
	if c.runs != runs+1 {
		t.Errorf("Run() no ejecutó la herramienta")
	}
	run(m, m.ClearInput()) // ctrl+l
	if m.Large() || c.last != "" {
		t.Errorf("ClearInput() no salió del modo grande (Large=%v, last=%d bytes)", m.Large(), len(c.last))
	}
	typeText(m, "ab")
	if c.last != "ab" {
		t.Errorf("después de limpiar no se edita en vivo: %q", c.last)
	}
}

func TestNotLive(t *testing.T) {
	c := &counter{live: false}
	m := newWS(c)
	runs := c.runs
	typeText(m, "abc")
	if c.runs != runs {
		t.Errorf("una herramienta no-live se ejecutó al escribir")
	}
	if !m.Status().Stale {
		t.Error("el estado no indica que hay que presionar F5")
	}
	run(m, m.Run())
	if c.last != "abc" {
		t.Errorf("F5 ejecutó con %q", c.last)
	}
}

// blocker se bloquea con la entrada "slow" hasta que cancelan su ctx.
type blocker struct{}

func (blocker) Spec() tool.Spec {
	return tool.Spec{ID: "block", Name: "Block", Category: tool.Text, Live: true,
		Inputs: []tool.InputSpec{{Key: "in", Label: "Entrada"}}}
}

func (blocker) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if req.Inputs["in"] == "slow" {
		<-ctx.Done()
		return tool.Result{}, ctx.Err()
	}
	return tool.Result{Text: "out:" + req.Inputs["in"]}, nil
}

func TestCancelAndDiscardStale(t *testing.T) {
	m := newWS(blocker{})
	m.Update(tea.PasteMsg{Content: "slo"})
	slowCmd := m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"}) // entrada "slow": se bloquea
	if !m.Running() {
		t.Fatal("no se marcó la ejecución en curso")
	}
	fastCmd := m.Update(tea.KeyPressMsg{Code: '!', Text: "!"}) // cancela la anterior

	done := make(chan tea.Msg, 1)
	go func() { done <- runBatch(slowCmd) }()
	select {
	case msg := <-done:
		r, ok := msg.(workspace.ResultMsg)
		if !ok || !errors.Is(r.Err, context.Canceled) {
			t.Fatalf("la ejecución vieja terminó con %#v, want context.Canceled", msg)
		}
		m.HandleResult(r) // debe descartarse
	case <-time.After(2 * time.Second):
		t.Fatal("la ejecución vieja no se canceló")
	}
	if m.Err() != nil {
		t.Errorf("un resultado cancelado se aplicó: %v", m.Err())
	}

	run(m, fastCmd)
	if m.Output() != "out:slow!" || m.Running() {
		t.Errorf("salida = %q, running = %v", m.Output(), m.Running())
	}

	// Un resultado con una seq vieja que llega tarde tampoco se aplica.
	m.HandleResult(workspace.ResultMsg{ToolID: "block", Seq: 1, Result: tool.Result{Text: "viejo"}})
	if m.Output() != "out:slow!" {
		t.Errorf("un resultado viejo pisó la salida: %q", m.Output())
	}
}

// runBatch ejecuta un comando (expandiendo lotes) y devuelve el primer
// ResultMsg, bloqueando hasta que llegue.
func runBatch(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		ch := make(chan tea.Msg, len(batch))
		for _, c := range batch {
			go func() { ch <- runBatch(c) }()
		}
		for range batch {
			if r, ok := (<-ch).(workspace.ResultMsg); ok {
				return r
			}
		}
		return nil
	}
	return msg
}

func TestDebounce(t *testing.T) {
	c := &counter{live: true}
	m := workspace.New(c, workspace.Styles{}, optKeys, 50*time.Millisecond)
	m.SetSize(80, 24)
	m.SetFocus(workspace.FocusInput)
	runs := c.runs

	var ticks []tea.Cmd
	for _, r := range "abc" {
		ticks = append(ticks, m.Update(tea.KeyPressMsg{Code: r, Text: string(r)}))
	}
	// Solo el último tick (seq vigente) lanza la ejecución.
	var launched int
	for _, cmd := range ticks {
		for _, msg := range collect(cmd, workspace.DebounceMsg{}) {
			if next := m.HandleDebounce(msg.(workspace.DebounceMsg)); next != nil {
				launched++
				m.HandleResult(next().(workspace.ResultMsg))
			}
		}
	}
	if launched != 1 || c.runs != runs+1 || c.last != "abc" {
		t.Errorf("launched = %d, runs = %d, last = %q; want 1 ejecución con \"abc\"", launched, c.runs-runs, c.last)
	}
}

// collect ejecuta cmd (con lotes) y devuelve los mensajes del tipo de want.
func collect(cmd tea.Cmd, want tea.Msg) []tea.Msg {
	if cmd == nil {
		return nil
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-ch:
	case <-time.After(200 * time.Millisecond):
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collect(c, want)...)
		}
		return out
	}
	if fmt.Sprintf("%T", msg) == fmt.Sprintf("%T", want) {
		return []tea.Msg{msg}
	}
	return nil
}
