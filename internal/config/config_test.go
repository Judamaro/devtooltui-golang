package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/config"
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

func TestParse(t *testing.T) {
	reg := testRegistry(t)
	tests := []struct {
		name     string
		toml     string
		check    func(t *testing.T, c config.Config)
		warnings []string // substrings, in order
	}{
		{"empty", "", func(t *testing.T, c config.Config) {
			if !reflect.DeepEqual(c, config.Default()) {
				t.Errorf("got %+v, want the defaults", c)
			}
		}, nil},
		{"plan example", `
theme = "dark"
detect_on_start = true
debounce_ms = 80
live_max_bytes = 65536

[history]
enabled = true
remember_options = false
store_inputs = true

[tools.json-format]
indent = 4
sort_keys = true

[keys]
copy_output = "ctrl+y"
`, func(t *testing.T, c config.Config) {
			want := config.Config{
				Theme: "dark", DetectOnStart: true, DebounceMS: 80, LiveMaxBytes: 65536,
				History: config.History{Enabled: true, StoreInputs: true},
				Tools:   map[string]tool.Values{"json-format": {"indent": 4, "sort_keys": true}},
				Keys:    map[string]string{"copy_output": "ctrl+y"},
			}
			if !reflect.DeepEqual(c, want) {
				t.Errorf("got %+v\nwant %+v", c, want)
			}
		}, nil},
		{"unknown keys warn", "colour = 1\n[history]\nforever = true\n", nil,
			[]string{`unknown key "colour"`, `unknown key "history.forever"`}},
		{"bad values keep defaults", "theme = \"pink\"\ndebounce_ms = 5000\nlive_max_bytes = \"big\"\ndetect_on_start = 1\n",
			func(t *testing.T, c config.Config) {
				if c.Theme != "auto" || c.DebounceMS != 120 || c.LiveMaxBytes != 32<<10 || c.DetectOnStart {
					t.Errorf("defaults changed: %+v", c)
				}
			},
			[]string{"debounce_ms = 5000 is outside", "detect_on_start must be true or false", "live_max_bytes must be an integer", `theme "pink"`}},
		{"tool options are validated", `
[tools.json-format]
indent = 99
sort_keys = "yes"
colour = true
[tools.base64]
variant = "url"
mode = "rot13"
[tools.nope]
x = 1
`, func(t *testing.T, c config.Config) {
			want := map[string]tool.Values{"base64": {"variant": "url"}}
			if !reflect.DeepEqual(c.Tools, want) {
				t.Errorf("Tools = %v, want %v", c.Tools, want)
			}
		}, []string{"tools.base64.mode: must be one of: encode, decode", `tools.json-format: unknown option "colour"`,
			"tools.json-format.indent: 99 is outside [0, 8]", "tools.json-format.sort_keys: must be true or false", `unknown tool "nope"`}},
		{"syntax error", "theme = \"dark\"\ndebounce_ms = = 3\n", func(t *testing.T, c config.Config) {
			if c.Theme != "auto" {
				t.Errorf("a broken file must give the defaults, got theme %q", c.Theme)
			}
		}, []string{"config.toml:2:"}},
		{"tables of the wrong type", "history = 1\ntools = \"x\"\nkeys = [1]\n", nil,
			[]string{"history must be a table", "keys must be a table", "tools must be a table"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, warns := config.Parse([]byte(tt.toml), "config.toml", reg)
			if tt.check != nil {
				tt.check(t, c)
			}
			if len(warns) != len(tt.warnings) {
				t.Fatalf("warnings = %q, want %d", warns, len(tt.warnings))
			}
			for i, w := range tt.warnings {
				if !strings.Contains(warns[i], w) || !strings.HasPrefix(warns[i], "config.toml") {
					t.Errorf("warning %d = %q, want it to contain %q", i, warns[i], w)
				}
			}
		})
	}
}

func TestPath(t *testing.T) {
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == config.EnvVar {
				return v
			}
			return ""
		}
	}
	dir := func() (string, error) { return "/home/u/.config", nil }
	tests := []struct {
		name, flag, env, want string
		explicit              bool
	}{
		{"flag wins", "a.toml", "b.toml", "a.toml", true},
		{"env", "", "b.toml", "b.toml", true},
		{"default", "", "", filepath.Join("/home/u/.config", "devtooltui", "config.toml"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, explicit, err := config.Path(tt.flag, env(tt.env), dir)
			if err != nil || got != tt.want || explicit != tt.explicit {
				t.Errorf("Path = %q, %v, %v; want %q, %v", got, explicit, err, tt.want, tt.explicit)
			}
		})
	}
	if _, _, err := config.Path("", env(""), func() (string, error) { return "", errors.New("no home") }); err == nil {
		t.Error("expected the user config dir error")
	}
}

func TestLoad(t *testing.T) {
	reg := testRegistry(t)
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.toml")
	if c, w := config.Load(missing, false, reg); len(w) != 0 || c.Theme != "auto" {
		t.Errorf("implicit missing file: %v, %v", c, w)
	}
	if _, w := config.Load(missing, true, reg); len(w) != 1 || !strings.Contains(w[0], "not found") {
		t.Errorf("explicit missing file: warnings %q", w)
	}
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("theme = \"light\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if c, w := config.Load(path, true, reg); len(w) != 0 || c.Theme != "light" {
		t.Errorf("Load = %v, %v", c, w)
	}
}
