package history_test

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/config"
	"github.com/Judamaro/devtooltui-golang/internal/history"
	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/encoders"
	"github.com/Judamaro/devtooltui-golang/internal/tools/formatters"
)

func testRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	reg, err := registry.New(formatters.JSONFormat{}, encoders.Base64{})
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

func TestLoadValidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	data := `{
  "version": 1,
  "last_tool": "base64",
  "tools": {
    "json-format": {"options": {"indent": 4, "sort_keys": true, "colour": 1}, "inputs": {"input": "{}", "other": "x"}},
    "base64": {"options": {"mode": "rot13", "padding": "yes", "indent": 2.5}},
    "gone": {"options": {"a": 1}}
  }
}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	got := history.Load(path, testRegistry(t))
	want := history.State{
		Version:  history.Version,
		LastTool: "base64",
		Tools: map[string]history.ToolState{
			"json-format": {Options: tool.Values{"indent": 4, "sort_keys": true}, Inputs: map[string]string{"input": "{}"}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load =\n%+v\nwant\n%+v", got, want)
	}
}

func TestLoadBadFiles(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"missing":     "",
		"not json":    "{",
		"old version": `{"version": 99, "last_tool": "base64"}`,
		"last gone":   `{"version": 1, "last_tool": "nope"}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name+".json")
			if content != "" {
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			s := history.Load(path, testRegistry(t))
			if s.LastTool != "" || len(s.Tools) != 0 || s.Version != history.Version {
				t.Errorf("Load = %+v, want an empty state", s)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	prev := history.State{Version: 1, LastTool: "json-format", Tools: map[string]history.ToolState{
		"json-format": {Options: tool.Values{"indent": 4}, Inputs: map[string]string{"input": "old secret"}},
		"untouched":   {Options: tool.Values{"x": true}},
	}}
	sess := history.Session{
		LastTool: "base64",
		Options:  map[string]tool.Values{"base64": {"mode": "decode"}},
		Inputs:   map[string]map[string]string{"base64": {"input": "aG9sYQ=="}, "json-format": {"input": ""}},
	}
	tests := []struct {
		name string
		h    config.History
		want history.State
	}{
		{"options only (default)", config.History{Enabled: true, RememberOptions: true}, history.State{Version: 1, LastTool: "base64", Tools: map[string]history.ToolState{
			"json-format": {Options: tool.Values{"indent": 4}}, // the old input is dropped
			"untouched":   {Options: tool.Values{"x": true}},
			"base64":      {Options: tool.Values{"mode": "decode"}},
		}}},
		{"options and inputs", config.History{Enabled: true, RememberOptions: true, StoreInputs: true}, history.State{Version: 1, LastTool: "base64", Tools: map[string]history.ToolState{
			"json-format": {Options: tool.Values{"indent": 4}}, // emptied in the session
			"untouched":   {Options: tool.Values{"x": true}},
			"base64":      {Options: tool.Values{"mode": "decode"}, Inputs: map[string]string{"input": "aG9sYQ=="}},
		}}},
		{"nothing but the last tool", config.History{Enabled: true}, history.State{Version: 1, LastTool: "base64", Tools: map[string]history.ToolState{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := history.Merge(prev, sess, tt.h); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Merge =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestMergeSkipsHugeInputs(t *testing.T) {
	big := string(make([]byte, history.MaxInputBytes+1))
	sess := history.Session{Inputs: map[string]map[string]string{"base64": {"input": big}}}
	got := history.Merge(history.State{}, sess, config.History{Enabled: true, StoreInputs: true})
	if len(got.Tools) != 0 {
		t.Errorf("stored a %d byte input", len(big))
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devtooltui", "state.json")
	s := history.State{Version: 1, LastTool: "json-format", Tools: map[string]history.ToolState{
		"json-format": {Options: tool.Values{"indent": 4, "sort_keys": true}},
	}}
	if err := history.Save(path, s); err != nil {
		t.Fatal(err)
	}
	if got := history.Load(path, testRegistry(t)); !reflect.DeepEqual(got, s) {
		t.Errorf("Load after Save =\n%+v\nwant\n%+v", got, s)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("state.json mode = %o, want 600", perm)
		}
		dir, _ := os.Stat(filepath.Dir(path))
		if perm := dir.Mode().Perm(); perm != 0o700 {
			t.Errorf("directory mode = %o, want 700", perm)
		}
	}
	// Saving again replaces the file and leaves no temporary files.
	s.LastTool = "base64"
	if err := history.Save(path, s); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("directory has %d entries, want only state.json", len(entries))
	}
}

func TestPath(t *testing.T) {
	got, err := history.Path(func() (string, error) { return "/c", nil })
	if err != nil || got != filepath.Join("/c", "devtooltui", "state.json") {
		t.Errorf("Path = %q, %v", got, err)
	}
}
