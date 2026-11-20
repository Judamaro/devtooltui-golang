package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tools"
)

type fakeClip struct {
	data    string
	osc52   bool
	readErr error
}

func (f *fakeClip) Read() (string, error) { return f.data, f.readErr }
func (f *fakeClip) Write(s string) error  { f.data = s; return nil }
func (f *fakeClip) UseOSC52() bool        { return f.osc52 }

func newClipApp(t *testing.T, clip *fakeClip, detectOnStart bool) *App {
	t.Helper()
	return newClipAppWith(t, clip, Options{DetectOnStart: detectOnStart})
}

func newClipAppWith(t *testing.T, clip *fakeClip, opts Options) *App {
	t.Helper()
	reg, err := registry.New(tools.All()...)
	if err != nil {
		t.Fatal(err)
	}
	opts.Clipboard = clip
	a := New(reg, opts)
	send(a, tea.WindowSizeMsg{Width: 100, Height: 30})
	testutil.RunCmds(a.Init(), func(msg tea.Msg) tea.Cmd { _, c := a.Update(msg); return c })
	return a
}

func TestCopyOutput(t *testing.T) {
	tests := []struct {
		name      string
		osc52     bool
		wantData  string
		wantFlash string
	}{
		{"local", false, "{}", "output copied to the clipboard"},
		{"osc52", true, "", "output copied (OSC 52)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clip := &fakeClip{osc52: tt.osc52}
			a := newClipApp(t, clip, false)
			press(a, "tab", "tab", "{}") // json-format is the first tool
			send(a, keyCtrl('y'))
			if clip.data != tt.wantData || a.flash != tt.wantFlash {
				t.Errorf("data = %q, flash = %q; want %q, %q", clip.data, a.flash, tt.wantData, tt.wantFlash)
			}
		})
	}
}

func TestPasteFromClipboard(t *testing.T) {
	clip := &fakeClip{data: `{"b":1}`}
	a := newClipApp(t, clip, false) // focus on the sidebar
	send(a, keyCtrl('o'))
	if a.focus != focusInput || !strings.Contains(a.workspace().Output(), `"b": 1`) {
		t.Errorf("focus = %d, output = %q", a.focus, a.workspace().Output())
	}

	clip.readErr = errors.New("no system clipboard: install wl-clipboard")
	send(a, keyCtrl('o'))
	if !strings.Contains(a.flash, "wl-clipboard") {
		t.Errorf("flash = %q", a.flash)
	}
	press(a, "x") // any key clears the notice
	if a.flash != "" {
		t.Errorf("the notice was not cleared: %q", a.flash)
	}
}

func TestDetectClipboard(t *testing.T) {
	jwt := testutil.ReadInput(t, "jwt", "hs256")
	tests := []struct {
		name      string
		data      string
		onStart   bool
		wantTool  string
		wantFlash string
	}{
		{"jwt", jwt, false, "jwt", "detected: jwt (0.98)"},
		{"on start with --clipboard", "f47ac10b-58cc-4372-a567-0e02b2c3d479", true, "uuid", "detected: uuid (0.90)"},
		{"unknown format", "hello world", false, "json-format", "was not recognised"},
		{"empty", "", false, "json-format", "empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newClipApp(t, &fakeClip{data: tt.data}, tt.onStart)
			if !tt.onStart {
				send(a, keyCtrl('d'))
			}
			if a.active != tt.wantTool || !strings.Contains(a.flash, tt.wantFlash) {
				t.Errorf("active = %q, flash = %q; want %q, %q", a.active, a.flash, tt.wantTool, tt.wantFlash)
			}
			if tt.wantTool == "jwt" && a.sidebar.Selected() != "jwt" {
				t.Errorf("the sidebar did not follow the jump: %s", a.sidebar.Selected())
			}
		})
	}
}
