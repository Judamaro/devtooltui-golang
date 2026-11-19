package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/Judamaro/devtooltui-golang/internal/cli"
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

// TestMain registra el binario "devtooltui" para los scripts .txtar: corre
// la CLI real con stdin y stdout que no son terminales.
func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"devtooltui": func() {
			reg, err := registry.New(tools.All()...)
			if err != nil {
				os.Exit(cli.ExitInternal)
			}
			os.Exit(cli.Execute(os.Args[1:], cli.Deps{
				Registry: reg, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
				Getenv: os.Getenv, RenderStyles: tui.RenderStyles(), Version: "test", Commit: "abc",
			}))
		},
	})
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
		{"ok archivo", []string{"json-format", file}, "", cli.ExitOK, ""},
		{"entrada invalida", []string{"json-format"}, `{"a":}`, cli.ExitInput, "<stdin>:1:6:"},
		{"error en opcion", []string{"regex", "--pattern", "("}, "x", cli.ExitInput, "--pattern:1:1:"},
		{"zona invalida", []string{"timestamp", "--tz", "Marte/X"}, "0", cli.ExitInput, "--tz:"},
		{"archivo inexistente", []string{"json-format", filepath.Join(dir, "nope")}, "", cli.ExitInput, "nope"},
		{"comando desconocido", []string{"nope"}, "", cli.ExitUsage, "unknown command"},
		{"flag desconocido", []string{"json-format", "--nope"}, "", cli.ExitUsage, "unknown flag"},
		{"select invalido", []string{"base64", "--variant", "raro"}, "", cli.ExitUsage, "standard, url"},
		{"number fuera de rango", []string{"uuid", "--count", "0"}, "", cli.ExitUsage, "entre 1 y 1000"},
		{"demasiados archivos", []string{"json-format", file, file}, "", cli.ExitUsage, "como máximo 1"},
		{"modo y --mode distintos", []string{"base64", "decode", "--mode", "encode"}, "", cli.ExitUsage, "no coinciden"},
		{"stdin dos veces", []string{"diff", "-", "-"}, "a", cli.ExitUsage, "una vez"},
		{"tui sin terminal", nil, "", cli.ExitUsage, "necesita una terminal"},
		{"max-input", []string{"base64", "--max-input", "3"}, "hola", cli.ExitInput, "supera --max-input"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, _, errb := deps(t, tt.stdin)
			if code := cli.Execute(tt.args, d); code != tt.code {
				t.Errorf("código = %d, want %d (stderr: %s)", code, tt.code, errb)
			}
			if !strings.Contains(errb.String(), tt.stderr) {
				t.Errorf("stderr = %q, want que contenga %q", errb, tt.stderr)
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
		{"terminal colorea", true, "", nil, true},
		{"pipe no colorea", false, "", nil, false},
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
				t.Fatalf("código %d", code)
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
		t.Fatalf("código %d", code)
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
		t.Errorf("list --json inesperado: %+v", list[0])
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
		t.Fatalf("código %d: %s", code, errb)
	}
	if clip.data != "aG9sYQ==" || out.String() != "aG9sYQ==\n" {
		t.Errorf("portapapeles = %q, stdout = %q", clip.data, out)
	}

	d, _, errb = deps(t, "hola")
	if code := cli.Execute([]string{"base64", "--copy"}, d); code != 0 || !strings.Contains(errb.String(), "no hay portapapeles") {
		t.Errorf("sin portapapeles: código %d, stderr %q", code, errb)
	}
}
