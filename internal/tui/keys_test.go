package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

func TestKeyMapApply(t *testing.T) {
	tests := []struct {
		name     string
		over     map[string]string
		check    func(t *testing.T, k KeyMap)
		warnings []string
	}{
		{"one key", map[string]string{"copy_output": "alt+c"}, func(t *testing.T, k KeyMap) {
			if got := k.CopyOutput.Keys(); len(got) != 1 || got[0] != "alt+c" {
				t.Errorf("keys = %v", got)
			}
			if h := k.CopyOutput.Help(); h.Key != "alt+c" || h.Desc != "copy output" {
				t.Errorf("help = %+v", h)
			}
		}, nil},
		{"several keys", map[string]string{"run": " F5 , Ctrl+G "}, func(t *testing.T, k KeyMap) {
			if got := k.Run.Keys(); len(got) != 2 || got[0] != "f5" || got[1] != "ctrl+g" {
				t.Errorf("keys = %v", got)
			}
		}, nil},
		{"none disables", map[string]string{"quit_outside": "none"}, func(t *testing.T, k KeyMap) {
			if k.QuitOutside.Enabled() {
				t.Error("quit_outside is still enabled")
			}
		}, nil},
		{"conflict keeps the default", map[string]string{"copy_output": "ctrl+p"}, func(t *testing.T, k KeyMap) {
			if k.CopyOutput.Keys()[0] != "ctrl+y" {
				t.Errorf("keys = %v", k.CopyOutput.Keys())
			}
		}, []string{`keys.copy_output: "ctrl+p" is already used by open_palette`}},
		{"a freed key can be reused", map[string]string{"open_palette": "ctrl+k", "copy_output": "ctrl+p"}, func(t *testing.T, k KeyMap) {
			if k.CopyOutput.Keys()[0] != "ctrl+p" || k.OpenPalette.Keys()[0] != "ctrl+k" {
				t.Errorf("copy = %v, palette = %v", k.CopyOutput.Keys(), k.OpenPalette.Keys())
			}
		}, nil},
		{"swap", map[string]string{"open_palette": "ctrl+y", "copy_output": "ctrl+p"}, func(t *testing.T, k KeyMap) {
			if k.CopyOutput.Keys()[0] != "ctrl+p" || k.OpenPalette.Keys()[0] != "ctrl+y" {
				t.Errorf("copy = %v, palette = %v", k.CopyOutput.Keys(), k.OpenPalette.Keys())
			}
		}, nil},
		{"unknown and empty", map[string]string{"teleport": "ctrl+t", "help": " , "}, nil,
			[]string{`keys.help is empty`, `unknown action "teleport"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := DefaultKeyMap()
			warns := k.Apply(tt.over)
			if tt.check != nil {
				tt.check(t, k)
			}
			if len(warns) != len(tt.warnings) {
				t.Fatalf("warnings = %q, want %d", warns, len(tt.warnings))
			}
			for i, w := range tt.warnings {
				if !strings.Contains(warns[i], w) {
					t.Errorf("warning %d = %q, want %q", i, warns[i], w)
				}
			}
		})
	}
}

func TestKeyActions(t *testing.T) {
	acts := KeyActions()
	if len(acts) != 16 || acts[0] != "clear_input" || acts[len(acts)-1] != "use_output" {
		t.Errorf("KeyActions = %v", acts)
	}
}

func TestConfiguredKeysWork(t *testing.T) {
	clip := &fakeClip{}
	a := newClipAppWith(t, clip, Options{Keys: map[string]string{"copy_output": "alt+c", "teleport": "x"}})
	press(a, "tab", "tab", "[1]")
	send(a, keyCtrl('y'))
	if clip.data != "" {
		t.Error("the old key still copies")
	}
	send(a, tea.KeyPressMsg{Code: 'c', Mod: tea.ModAlt})
	if clip.data != "[\n  1\n]" {
		t.Errorf("alt+c did not copy: %q", clip.data)
	}
	if !key.Matches(tea.KeyPressMsg{Code: 'c', Mod: tea.ModAlt}, a.keys.CopyOutput) {
		t.Error("binding not updated")
	}
}
