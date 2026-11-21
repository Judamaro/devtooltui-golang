package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/Judamaro/devtooltui-golang/internal/cli"
	"github.com/Judamaro/devtooltui-golang/internal/config"
	"github.com/Judamaro/devtooltui-golang/internal/plugin"
	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/tools"
	"github.com/Judamaro/devtooltui-golang/internal/tui"
)

func deps(t testing.TB, stdin string) (cli.Deps, *bytes.Buffer, *bytes.Buffer) {
	reg, err := registry.New(tools.All()...)
	if err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	return cli.Deps{
		Registry: reg, Stdin: strings.NewReader(stdin), Stdout: &out, Stderr: &errb,
		Getenv: func(string) string { return "" }, RenderStyles: tui.RenderStyles(),
		Version: "test", Commit: "abc",
	}, &out, &errb
}

// TestMain registers the "devtooltui" binary for the .txtar scripts: it runs
// the real CLI with a stdin and stdout that are not terminals.
func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"devtooltui": func() {
			// As cmd/devtooltui does, without the default config file.
			pathList, off := os.Getenv("PATH"), false
			if cfg, _, err := config.Path(cli.ConfigFlag(os.Args[1:]), os.Getenv, noConfigDir); err == nil && !config.PluginsEnabled(cfg) {
				pathList, off = "", true
			}
			reg, plugins, err := plugin.Registry(context.Background(), pathList, "", tools.All())
			if err != nil {
				os.Exit(cli.ExitInternal)
			}
			os.Exit(cli.Execute(os.Args[1:], cli.Deps{
				Registry: reg, Plugins: plugins, PluginsOff: off, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
				Getenv: os.Getenv, RenderStyles: tui.RenderStyles(), Version: "test", Commit: "abc",
			}))
		},
	})
}

func noConfigDir() (string, error) { return "", errors.New("no default config in tests") }

func TestConfigFlag(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"--config", "a.toml", "list"}, "a.toml"},
		{[]string{"list", "--config=b.toml"}, "b.toml"},
		{[]string{"json-format", "--", "--config", "x"}, ""},
		{[]string{"--config"}, ""},
		{nil, ""},
	}
	for _, tt := range tests {
		if got := cli.ConfigFlag(tt.args); got != tt.want {
			t.Errorf("ConfigFlag(%q) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{Dir: filepath.Join("testdata", "script")})
}

func TestExitCodes(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "x.json")
	if err := os.WriteFile(file, []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		args   []string
		stdin  string
		code   int
		stderr string
	}{
		{"ok stdin", []string{"json-format"}, `{"a":1}`, cli.ExitOK, ""},
		{"ok file", []string{"json-format", file}, "", cli.ExitOK, ""},
		{"invalid input", []string{"json-format"}, `{"a":}`, cli.ExitInput, "<stdin>:1:6:"},
		{"error in an option", []string{"regex", "--pattern", "("}, "x", cli.ExitInput, "--pattern:1:1:"},
		{"invalid zone", []string{"timestamp", "--tz", "Mars/X"}, "0", cli.ExitInput, "--tz:"},
		{"missing file", []string{"json-format", filepath.Join(dir, "nope")}, "", cli.ExitInput, "nope"},
		{"unknown command", []string{"nope"}, "", cli.ExitUsage, "unknown command"},
		{"unknown flag", []string{"json-format", "--nope"}, "", cli.ExitUsage, "unknown flag"},
		{"invalid select", []string{"base64", "--variant", "odd"}, "", cli.ExitUsage, "standard, url"},
		{"number out of range", []string{"uuid", "--count", "0"}, "", cli.ExitUsage, "between 1 and 1000"},
		{"too many files", []string{"json-format", file, file}, "", cli.ExitUsage, "at most 1"},
		{"mode and --mode differ", []string{"base64", "decode", "--mode", "encode"}, "", cli.ExitUsage, "do not match"},
		{"stdin twice", []string{"diff", "-", "-"}, "a", cli.ExitUsage, "only be used once"},
		{"tui without a terminal", nil, "", cli.ExitUsage, "needs a terminal"},
		{"max-input", []string{"base64", "--max-input", "3"}, "hola", cli.ExitInput, "exceeds --max-input"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, _, errb := deps(t, tt.stdin)
			if code := cli.Execute(tt.args, d); code != tt.code {
				t.Errorf("code = %d, want %d (stderr: %s)", code, tt.code, errb)
			}
			if !strings.Contains(errb.String(), tt.stderr) {
				t.Errorf("stderr = %q, want it to contain %q", errb, tt.stderr)
			}
		})
	}
}

func TestColor(t *testing.T) {
	tests := []struct {
		name    string
		tty     bool
		noColor string
		args    []string
		want    bool
	}{
		{"terminal is coloured", true, "", nil, true},
		{"pipe is not coloured", false, "", nil, false},
		{"NO_COLOR", true, "1", nil, false},
		{"--no-color", true, "", []string{"--no-color"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, out, _ := deps(t, `{"a":1}`)
			d.StdoutTTY = tt.tty
			d.Getenv = func(k string) string {
				if k == "NO_COLOR" {
					return tt.noColor
				}
				return ""
			}
			if code := cli.Execute(append([]string{"json-format"}, tt.args...), d); code != 0 {
				t.Fatalf("code %d", code)
			}
			if got := strings.Contains(out.String(), "\x1b["); got != tt.want {
				t.Errorf("color = %v, want %v: %q", got, tt.want, out)
			}
		})
	}
}

func TestListJSON(t *testing.T) {
	d, out, _ := deps(t, "")
	if code := cli.Execute([]string{"list", "--json"}, d); code != 0 {
		t.Fatalf("code %d", code)
	}
	var list []struct {
		ID      string `json:"id"`
		Options []struct {
			Flag string `json:"flag"`
		} `json:"options"`
	}
	if err := json.Unmarshal(out.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != len(tools.All()) || list[0].ID != "json-format" || list[0].Options[1].Flag != "--sort-keys" {
		t.Errorf("unexpected list --json: %+v", list[0])
	}
}

type fakeClip struct{ data string }

func (f *fakeClip) Read() (string, error) { return f.data, nil }
func (f *fakeClip) Write(s string) error  { f.data = s; return nil }

func TestCopyFlag(t *testing.T) {
	d, out, errb := deps(t, "hola")
	clip := &fakeClip{}
	d.Clipboard = clip
	if code := cli.Execute([]string{"base64", "--copy"}, d); code != 0 {
		t.Fatalf("code %d: %s", code, errb)
	}
	if clip.data != "aG9sYQ==" || out.String() != "aG9sYQ==\n" {
		t.Errorf("clipboard = %q, stdout = %q", clip.data, out)
	}

	d, _, errb = deps(t, "hola")
	if code := cli.Execute([]string{"base64", "--copy"}, d); code != 0 || !strings.Contains(errb.String(), "no clipboard") {
		t.Errorf("without a clipboard: code %d, stderr %q", code, errb)
	}
}
