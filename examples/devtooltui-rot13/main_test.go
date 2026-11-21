package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/plugin"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

func TestRun(t *testing.T) {
	tests := []struct {
		in, variant, want string
	}{
		{"Hello, World!", "rot13", "Uryyb, Jbeyq!"},
		{"Uryyb, Jbeyq!", "rot13", "Hello, World!"},
		{"Hello, World!", "rot47", "w6==@[ (@C=5P"},
		{"ñandú", "rot13", "ñnaqú"},
	}
	for _, tt := range tests {
		res := run(request{Inputs: map[string]string{"input": tt.in}, Options: map[string]any{"variant": tt.variant}})
		if res.Result == nil || res.Result.Text != tt.want {
			t.Errorf("%s(%q) = %+v, want %q", tt.variant, tt.in, res.Result, tt.want)
		}
	}
	res := run(request{Inputs: map[string]string{"input": "ok\nab\x00c"}})
	if res.Error == nil || res.Error.Line != 2 || res.Error.Col != 3 {
		t.Errorf("NUL = %+v, want an error at 2:3", res.Error)
	}
}

func TestHandle(t *testing.T) {
	var out bytes.Buffer
	if err := handle("spec", nil, &out); err != nil || !strings.Contains(out.String(), `"protocol":1`) {
		t.Errorf("spec = %q, %v", out.String(), err)
	}
	if err := handle("nope", nil, &out); err == nil {
		t.Error("an unknown command must fail")
	}
	if err := handle("run", strings.NewReader("{"), &out); err == nil {
		t.Error("a bad request must fail")
	}
}

// TestEndToEnd builds the plugin, finds it in a PATH and runs it through
// devtooltui's adapter: what users get.
func TestEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the plugin")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "devtooltui-rot13")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	build := exec.Command("go", "build", "-o", exe, ".")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	reg, infos, err := plugin.Registry(context.Background(), dir, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 || len(infos[0].Problems) != 0 {
		t.Fatalf("infos = %+v", infos)
	}
	rot, ok := reg.Get("rot13")
	if !ok || rot.Spec().Category != tool.Encoders {
		t.Fatalf("rot13 = %v, %v", rot, ok)
	}
	res, err := rot.Run(context.Background(), tool.Request{Inputs: map[string]string{"input": "Hello"}, Opts: tool.Values{"variant": "rot13"}})
	if err != nil || res.Text != "Uryyb" || res.Meta[0].Value != "5" {
		t.Errorf("Run = %+v, %v", res, err)
	}
	_, err = rot.Run(context.Background(), tool.Request{Inputs: map[string]string{"input": "a\x00"}})
	if ie, ok := tool.AsInputError(err); !ok || ie.Col != 2 {
		t.Errorf("NUL = %v, want an InputError at column 2", err)
	}
}
