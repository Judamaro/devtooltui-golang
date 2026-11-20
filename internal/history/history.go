// Package history remembers, between TUI sessions, the last tool and each
// tool's options (and, only when the user asks for it, the inputs). The state
// lives in <user cache dir>/devtooltui/state.json, written atomically with
// mode 0600.
package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"

	"github.com/Judamaro/devtooltui-golang/internal/config"
	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Version of the state.json format.
const Version = 1

// MaxInputBytes is the largest input stored; bigger ones are skipped.
const MaxInputBytes = 256 << 10

// State is the content of state.json.
type State struct {
	Version  int                  `json:"version"`
	LastTool string               `json:"last_tool,omitempty"`
	Tools    map[string]ToolState `json:"tools,omitempty"`
}

// ToolState is what is remembered for one tool.
type ToolState struct {
	Options tool.Values       `json:"options,omitempty"`
	Inputs  map[string]string `json:"inputs,omitempty"`
}

// Path returns <user cache dir>/devtooltui/state.json.
func Path(userCacheDir func() (string, error)) (string, error) {
	dir, err := userCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "devtooltui", "state.json"), nil
}

// Load reads the state and keeps only what still fits the registry: known
// tools, known inputs and option values valid for their Spec. A missing or
// unreadable file is an empty state: history is a convenience, never an
// error.
func Load(path string, reg *registry.Registry) State {
	s := State{Version: Version, Tools: map[string]ToolState{}}
	data, err := os.ReadFile(path) //nolint:gosec // our own state file
	if err != nil {
		return s
	}
	var raw State
	if json.Unmarshal(data, &raw) != nil || raw.Version != Version {
		return s
	}
	if _, ok := reg.Get(raw.LastTool); ok {
		s.LastTool = raw.LastTool
	}
	for id, ts := range raw.Tools {
		t, ok := reg.Get(id)
		if !ok {
			continue
		}
		spec := t.Spec()
		clean := ToolState{}
		for k, v := range ts.Options {
			i := slices.IndexFunc(spec.Options, func(o tool.Option) bool { return o.Key == k })
			if i < 0 {
				continue
			}
			if val, msg := config.OptionValue(spec.Options[i], v); msg == "" {
				if clean.Options == nil {
					clean.Options = tool.Values{}
				}
				clean.Options[k] = val
			}
		}
		for k, v := range ts.Inputs {
			if slices.ContainsFunc(spec.Inputs, func(in tool.InputSpec) bool { return in.Key == k }) {
				if clean.Inputs == nil {
					clean.Inputs = map[string]string{}
				}
				clean.Inputs[k] = v
			}
		}
		if clean.Options != nil || clean.Inputs != nil {
			s.Tools[id] = clean
		}
	}
	return s
}

// Session is what a TUI session leaves: the active tool and, for each tool
// it opened, its options and inputs.
type Session struct {
	LastTool string
	Options  map[string]tool.Values
	Inputs   map[string]map[string]string
}

// Merge applies a session over the previous state following the [history]
// settings. Tools not opened in the session keep their entries; with
// store_inputs off every stored input is dropped, also old ones.
func Merge(prev State, sess Session, h config.History) State {
	s := State{Version: Version, LastTool: prev.LastTool, Tools: map[string]ToolState{}}
	for id, ts := range prev.Tools {
		s.Tools[id] = ts
	}
	if sess.LastTool != "" {
		s.LastTool = sess.LastTool
	}
	if h.RememberOptions {
		for id, v := range sess.Options {
			ts := s.Tools[id]
			ts.Options = v
			s.Tools[id] = ts
		}
	}
	if h.StoreInputs {
		for id, in := range sess.Inputs {
			ts := s.Tools[id]
			ts.Inputs = nil
			for k, v := range in {
				if v != "" && len(v) <= MaxInputBytes {
					if ts.Inputs == nil {
						ts.Inputs = map[string]string{}
					}
					ts.Inputs[k] = v
				}
			}
			s.Tools[id] = ts
		}
	}
	for id, ts := range s.Tools {
		if !h.RememberOptions {
			ts.Options = nil
		}
		if !h.StoreInputs {
			ts.Inputs = nil
		}
		if ts.Options == nil && ts.Inputs == nil {
			delete(s.Tools, id)
		} else {
			s.Tools[id] = ts
		}
	}
	return s
}

// Save writes the state atomically (temporary file + rename) with mode 0600,
// creating the directory with 0700.
func Save(path string, s State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".state-*.json") // created with mode 0600
	if err != nil {
		return err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	if _, err := f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	ok = true
	return nil
}
