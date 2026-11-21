// Package config loads the optional TOML configuration. Problems never stop
// the program: they become warnings (statusbar in the TUI, stderr in the
// CLI) and the affected values keep their defaults.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// EnvVar overrides the config path (the --config flag wins over it).
const EnvVar = "DEVTOOLTUI_CONFIG"

// Limits for numeric settings.
const (
	MaxDebounceMS   = 2000
	MinLiveMaxBytes = 1 << 10
	MaxLiveMaxBytes = 1 << 20
)

// Config is the content of config.toml.
type Config struct {
	Theme         string // auto | dark | light
	DetectOnStart bool
	DebounceMS    int
	LiveMaxBytes  int // above it, the input is read-only and runs with F5 (ADR 0002)
	History       History
	Plugins       Plugins
	Tools         map[string]tool.Values // validated defaults per tool ID
	Keys          map[string]string      // keymap overrides, validated by the TUI
}

// Plugins is the [plugins] table.
type Plugins struct {
	Enabled bool // look for devtooltui-<name> executables in the PATH
}

// History is the [history] table.
type History struct {
	Enabled         bool
	RememberOptions bool
	StoreInputs     bool // inputs may hold secrets (JWT, HMAC): off by default
}

// Default is the configuration without a file.
func Default() Config {
	return Config{
		Theme:        "auto",
		DebounceMS:   120,
		LiveMaxBytes: 32 << 10,
		History:      History{Enabled: true, RememberOptions: true},
		Plugins:      Plugins{Enabled: true},
		Tools:        map[string]tool.Values{},
		Keys:         map[string]string{},
	}
}

// Path returns the config file to read: flag, then $DEVTOOLTUI_CONFIG, then
// <user config dir>/devtooltui/config.toml. explicit tells whether the user
// chose it, so a missing file deserves a warning.
func Path(flag string, getenv func(string) string, userConfigDir func() (string, error)) (path string, explicit bool, err error) {
	if flag != "" {
		return flag, true, nil
	}
	if p := getenv(EnvVar); p != "" {
		return p, true, nil
	}
	dir, err := userConfigDir()
	if err != nil {
		return "", false, err
	}
	return filepath.Join(dir, "devtooltui", "config.toml"), false, nil
}

// Load reads path. A missing file gives the defaults (with a warning when
// explicit). Tool defaults are checked against the registry's Specs.
func Load(path string, explicit bool, reg *registry.Registry) (Config, []string) {
	c := Default()
	data, err := os.ReadFile(path) //nolint:gosec // the user chooses the config file
	if errors.Is(err, fs.ErrNotExist) {
		if explicit {
			return c, []string{fmt.Sprintf("config %s not found; using defaults", path)}
		}
		return c, nil
	}
	if err != nil {
		return c, []string{fmt.Sprintf("config: %v", err)}
	}
	return Parse(data, filepath.Base(path), reg)
}

// Parse decodes TOML; name prefixes the warnings.
func Parse(data []byte, name string, reg *registry.Registry) (Config, []string) {
	c := Default()
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		var de *toml.DecodeError
		if errors.As(err, &de) {
			row, col := de.Position()
			return c, []string{fmt.Sprintf("%s:%d:%d: %s; using defaults", name, row, col, de.Error())}
		}
		return c, []string{fmt.Sprintf("%s: %v; using defaults", name, err)}
	}
	p := parser{name: name}
	for _, k := range sortedKeys(raw) {
		v := raw[k]
		switch k {
		case "theme":
			if s, ok := p.str(k, v); ok {
				if slices.Contains([]string{"auto", "dark", "light"}, s) {
					c.Theme = s
				} else {
					p.warn("theme %q: use auto, dark or light", s)
				}
			}
		case "detect_on_start":
			if b, ok := p.boolean(k, v); ok {
				c.DetectOnStart = b
			}
		case "debounce_ms":
			if n, ok := p.integer(k, v, 0, MaxDebounceMS); ok {
				c.DebounceMS = n
			}
		case "live_max_bytes":
			if n, ok := p.integer(k, v, MinLiveMaxBytes, MaxLiveMaxBytes); ok {
				c.LiveMaxBytes = n
			}
		case "history":
			p.history(&c.History, v)
		case "plugins":
			p.plugins(&c.Plugins, v)
		case "tools":
			p.tools(c.Tools, v, reg)
		case "keys":
			p.keys(c.Keys, v)
		default:
			p.warn("unknown key %q", k)
		}
	}
	return c, p.warnings
}

type parser struct {
	name     string
	warnings []string
}

func (p *parser) warn(format string, a ...any) {
	p.warnings = append(p.warnings, p.name+": "+fmt.Sprintf(format, a...))
}

func (p *parser) str(key string, v any) (string, bool) {
	s, ok := v.(string)
	if !ok {
		p.warn("%s must be a string", key)
	}
	return s, ok
}

func (p *parser) boolean(key string, v any) (bool, bool) {
	b, ok := v.(bool)
	if !ok {
		p.warn("%s must be true or false", key)
	}
	return b, ok
}

func (p *parser) integer(key string, v any, lo, hi int) (int, bool) {
	n, ok := v.(int64)
	if !ok {
		p.warn("%s must be an integer", key)
		return 0, false
	}
	if n < int64(lo) || n > int64(hi) {
		p.warn("%s = %d is outside [%d, %d]", key, n, lo, hi)
		return 0, false
	}
	return int(n), true
}

func (p *parser) table(key string, v any) (map[string]any, bool) {
	t, ok := v.(map[string]any)
	if !ok {
		p.warn("%s must be a table ([%s])", key, key)
	}
	return t, ok
}

func (p *parser) history(h *History, v any) {
	t, ok := p.table("history", v)
	if !ok {
		return
	}
	for _, k := range sortedKeys(t) {
		var dst *bool
		switch k {
		case "enabled":
			dst = &h.Enabled
		case "remember_options":
			dst = &h.RememberOptions
		case "store_inputs":
			dst = &h.StoreInputs
		default:
			p.warn("unknown key \"history.%s\"", k)
			continue
		}
		if b, ok := p.boolean("history."+k, t[k]); ok {
			*dst = b
		}
	}
}

func (p *parser) plugins(pl *Plugins, v any) {
	t, ok := p.table("plugins", v)
	if !ok {
		return
	}
	for _, k := range sortedKeys(t) {
		if k != "enabled" {
			p.warn("unknown key \"plugins.%s\"", k)
			continue
		}
		if b, ok := p.boolean("plugins.enabled", t[k]); ok {
			pl.Enabled = b
		}
	}
}

// PluginsEnabled reads only [plugins] enabled from the file at path. The
// plugins must be known before the full config is parsed, since
// [tools.<id>] is checked against every tool, theirs included. Any
// problem means the default (true); Load reports it later.
func PluginsEnabled(path string) bool {
	data, err := os.ReadFile(path) //nolint:gosec // the user chooses the config file
	if err != nil {
		return true
	}
	var c struct {
		Plugins struct {
			Enabled *bool `toml:"enabled"`
		} `toml:"plugins"`
	}
	if toml.Unmarshal(data, &c) != nil || c.Plugins.Enabled == nil {
		return true
	}
	return *c.Plugins.Enabled
}

// tools validates [tools.<id>] against the Spec options; bad values are
// dropped with a warning.
func (p *parser) tools(dst map[string]tool.Values, v any, reg *registry.Registry) {
	t, ok := p.table("tools", v)
	if !ok {
		return
	}
	for _, id := range sortedKeys(t) {
		tl, found := reg.Get(id)
		if !found {
			p.warn("unknown tool %q in [tools]", id)
			continue
		}
		opts, ok := p.table("tools."+id, t[id])
		if !ok {
			continue
		}
		spec := tl.Spec()
		vals := tool.Values{}
		for _, k := range sortedKeys(opts) {
			i := slices.IndexFunc(spec.Options, func(o tool.Option) bool { return o.Key == k })
			if i < 0 {
				p.warn("tools.%s: unknown option %q", id, k)
				continue
			}
			if val, msg := OptionValue(spec.Options[i], opts[k]); msg != "" {
				p.warn("tools.%s.%s: %s", id, k, msg)
			} else {
				vals[k] = val
			}
		}
		if len(vals) > 0 {
			dst[id] = vals
		}
	}
}

// OptionValue checks v (decoded from TOML or JSON) against the option kind,
// as registry.New does for Spec defaults, and returns it with the Go type
// of that kind. On failure it returns a message.
func OptionValue(o tool.Option, v any) (any, string) {
	switch o.Kind {
	case tool.Toggle:
		if b, ok := v.(bool); ok {
			return b, ""
		}
		return nil, "must be true or false"
	case tool.Number:
		var n int64
		switch x := v.(type) {
		case int64:
			n = x
		case int:
			n = int64(x)
		case float64: // JSON
			if x != math.Trunc(x) || math.Abs(x) > 1<<53 {
				return nil, "must be an integer"
			}
			n = int64(x)
		default:
			return nil, "must be an integer"
		}
		if n < int64(o.Min) || n > int64(o.Max) {
			return nil, fmt.Sprintf("%d is outside [%d, %d]", n, o.Min, o.Max)
		}
		return int(n), ""
	case tool.Select:
		s, ok := v.(string)
		if !ok || !slices.Contains(o.Choices, s) {
			return nil, "must be one of: " + strings.Join(o.Choices, ", ")
		}
		return s, ""
	case tool.String:
		if s, ok := v.(string); ok {
			return s, ""
		}
		return nil, "must be a string"
	}
	return nil, "unsupported option"
}

func (p *parser) keys(dst map[string]string, v any) {
	t, ok := p.table("keys", v)
	if !ok {
		return
	}
	for _, k := range sortedKeys(t) {
		if s, ok := p.str("keys."+k, t[k]); ok {
			dst[k] = s
		}
	}
}

func sortedKeys[V any](m map[string]V) []string { return slices.Sorted(maps.Keys(m)) }
