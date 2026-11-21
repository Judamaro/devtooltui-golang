package plugin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/encoders"
)

// TestMain turns the test binary into a fake plugin when FAKE_PLUGIN is
// set: the tests run it as a child process with "spec" or "run".
func TestMain(m *testing.M) {
	if mode := os.Getenv("FAKE_PLUGIN"); mode != "" {
		os.Exit(fakePlugin(mode, os.Args[len(os.Args)-1]))
	}
	os.Exit(m.Run())
}

func fakePlugin(mode, cmd string) int {
	out := json.NewEncoder(os.Stdout)
	switch {
	case mode == "badjson":
		fmt.Print("not json")
		return 0
	case mode == "exit":
		fmt.Fprintln(os.Stderr, "boom: no luck")
		return 3
	case mode == "slow" && cmd == "run":
		time.Sleep(time.Minute)
		return 0
	}
	if cmd == "spec" {
		protocol := 1
		if mode == "proto2" {
			protocol = 2
		}
		tools := []map[string]any{
			{"id": "fake-upper", "name": "Upper", "category": "Text", "description": "Upper-case the text",
				"detect": map[string]any{"regex": "^shout:", "score": 0.7}, "export": true,
				"inputs":  []map[string]any{{"key": "text", "label": "Text", "multiline": true}},
				"options": []map[string]any{{"key": "prefix", "label": "Prefix", "kind": "string", "default": ">"}, {"key": "times", "label": "Times", "kind": "number", "min": 1, "max": 3, "default": 1}}},
			{"id": "fake-magic", "name": "Magic", "category": "Wizardry", "description": "Unknown category"},
		}
		switch mode {
		case "collide":
			tools = append(tools, map[string]any{"id": "base64", "name": "Mine", "category": "Text", "description": "x"})
		case "invalid":
			tools = append(tools, map[string]any{"id": "Bad ID", "name": "Bad", "category": "Text", "description": "x"},
				map[string]any{"id": "fake-kind", "name": "K", "category": "Text", "description": "x",
					"options": []map[string]any{{"key": "n", "label": "N", "kind": "number", "min": 1, "max": 2, "default": "two"}}},
				map[string]any{"id": "fake-re", "name": "R", "category": "Text", "description": "x", "detect": map[string]any{"regex": "(", "score": 0.5}},
				map[string]any{"id": "fake-score", "name": "S", "category": "Text", "description": "x", "detect": map[string]any{"regex": "a", "score": 2}})
		}
		_ = out.Encode(map[string]any{"protocol": protocol, "tools": tools})
		return 0
	}
	var req RunRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	text := req.Inputs["text"]
	if cmd == "export" {
		switch {
		case mode == "badext":
			_ = out.Encode(ExportResponse{Export: &ExportData{Data: "eA==", Ext: "png"}})
		case text == "":
			_ = out.Encode(ExportResponse{Error: &InputError{Line: 1, Col: 1, Message: "nothing to export"}})
		default:
			_ = out.Encode(ExportResponse{Export: &ExportData{Data: base64.StdEncoding.EncodeToString([]byte(strings.ToUpper(text))), Ext: ".txt"}})
		}
		return 0
	}
	switch {
	case mode == "twoanswers":
		_ = out.Encode(map[string]any{"result": map[string]any{"text": "x"}, "error": map[string]any{"message": "y"}})
	case strings.Contains(text, "bad"):
		_ = out.Encode(RunResponse{Error: &InputError{Input: "text", Line: 1, Col: strings.Index(text, "bad") + 1, Message: "no bad words"}})
	case text == "span":
		_ = out.Encode(RunResponse{Result: &Result{Text: "x", Spans: []Span{{Input: "text", Start: 0, End: 99}}}})
	default:
		times := int(req.Options["times"].(float64))
		res := &Result{
			Text: req.Options["prefix"].(string) + strings.Repeat(strings.ToUpper(text), times),
			Meta: []KV{{Key: "Tool", Value: req.Tool}},
			Diff: []Diff{{Op: "insert", Text: "x"}},
		}
		_ = out.Encode(RunResponse{Result: res})
	}
	return 0
}

func fake(t *testing.T, mode string) []Candidate {
	t.Helper()
	t.Setenv("FAKE_PLUGIN", mode)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return []Candidate{{Name: "fake", Path: exe}}
}

func builtin() []tool.Tool { return []tool.Tool{encoders.Base64{}} }

func TestLoad(t *testing.T) {
	tools, infos := Load(context.Background(), fake(t, "ok"), builtin())
	if len(tools) != 2 || len(infos) != 1 {
		t.Fatalf("tools = %d, infos = %+v", len(tools), infos)
	}
	up := tools[0].Spec()
	if up.ID != "fake-upper" || up.Category != tool.Text || !up.Live || up.Options[1].Default != 1 {
		t.Errorf("spec = %+v", up)
	}
	if got := tools[1].Spec().Category; got != tool.Plugins {
		t.Errorf("unknown category went to %q, want Plugins", got)
	}
	if infos[0].Protocol != 1 || !slices.Equal(infos[0].Tools, []string{"fake-upper", "fake-magic"}) || len(infos[0].Problems) != 0 {
		t.Errorf("info = %+v", infos[0])
	}
}

func TestLoadProblems(t *testing.T) {
	tests := []struct {
		mode, problem string
		tools         int
	}{
		{"badjson", "spec is not valid JSON", 0},
		{"exit", "exit status 3: boom: no luck", 0},
		{"proto2", "speaks protocol 2; this devtooltui speaks 1", 0},
		{"collide", "base64: the ID is taken by a built-in tool", 2},
		{"invalid", "lowercase kebab-case", 2},
		{"invalid", "fake-re: detect.regex", 2},
		{"invalid", "fake-score: detect.score must be in (0, 1]", 2},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			tools, infos := Load(context.Background(), fake(t, tt.mode), builtin())
			if len(tools) != tt.tools {
				t.Errorf("%d tools, want %d", len(tools), tt.tools)
			}
			joined := strings.Join(infos[0].Problems, "\n")
			if !strings.Contains(joined, tt.problem) {
				t.Errorf("problems = %q, want %q", infos[0].Problems, tt.problem)
			}
			if tt.mode == "invalid" && !strings.Contains(joined, "fake-kind") {
				t.Errorf("the bad default was not reported: %q", joined)
			}
		})
	}
}

func TestLoadTwoPluginsSameID(t *testing.T) {
	c := fake(t, "ok")
	c = append(c, Candidate{Name: "copy", Path: c[0].Path})
	tools, infos := Load(context.Background(), c, nil)
	if len(tools) != 2 || len(infos[1].Tools) != 0 || !strings.Contains(strings.Join(infos[1].Problems, " "), "taken by plugin fake") {
		t.Errorf("tools = %d, second = %+v", len(tools), infos[1])
	}
}

func upper(t *testing.T) tool.Tool {
	t.Helper()
	tools, _ := Load(context.Background(), fake(t, "ok"), nil)
	return tools[0]
}

func TestRun(t *testing.T) {
	up := upper(t)
	res, err := up.Run(context.Background(), tool.Request{Inputs: map[string]string{"text": "abc"}, Opts: tool.Values{"times": 2}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != ">ABCABC" || res.Meta[0].Value != "fake-upper" || res.Diff[0].Kind != tool.Insert {
		t.Errorf("result = %+v", res)
	}
}

func TestRunErrors(t *testing.T) {
	up := upper(t)
	run := func(text string) error {
		_, err := up.Run(context.Background(), tool.Request{Inputs: map[string]string{"text": text}})
		return err
	}
	err := run("a bad day")
	if ie, ok := tool.AsInputError(err); !ok || ie.Input != "text" || ie.Line != 1 || ie.Col != 3 || ie.Msg != "no bad words" {
		t.Errorf("input error = %v", err)
	}
	if err := run("span"); err == nil || !strings.Contains(err.Error(), "outside its input") {
		t.Errorf("a span outside the input = %v", err)
	}
	for _, mode := range []string{"badjson", "exit", "twoanswers"} {
		t.Setenv("FAKE_PLUGIN", mode)
		err := run("x")
		if _, isInput := tool.AsInputError(err); err == nil || isInput {
			t.Errorf("%s: error = %v, want an internal error", mode, err)
		}
	}
}

func TestRunCancelAndTimeout(t *testing.T) {
	up := upper(t)
	t.Setenv("FAKE_PLUGIN", "slow")
	req := tool.Request{Inputs: map[string]string{"text": "x"}}

	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()
	start := time.Now()
	if _, err := up.Run(ctx, req); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled run = %v, want context.Canceled", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("cancelling took %s: the process was not killed", d)
	}

	old := runTimeout
	runTimeout = 200 * time.Millisecond
	defer func() { runTimeout = old }()
	if _, err := up.Run(context.Background(), req); err == nil || !strings.Contains(err.Error(), "took too long") {
		t.Errorf("slow run = %v, want a timeout", err)
	}
}

func TestDiscover(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	write := func(dir, name string, mode os.FileMode) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), mode); err != nil {
			t.Fatal(err)
		}
	}
	exe := ""
	if runtime.GOOS == "windows" {
		exe = ".exe"
	}
	write(a, "devtooltui-one"+exe, 0o755)
	write(a, "devtooltui-two"+exe, 0o755)
	write(b, "devtooltui-one"+exe, 0o755) // shadowed by the first directory
	write(b, "devtooltui-"+exe, 0o755)    // no name
	write(b, "other"+exe, 0o755)
	if runtime.GOOS != "windows" {
		write(b, "devtooltui-noexec", 0o644)
	} else {
		write(b, "devtooltui-script.bat", 0o755)
	}
	if err := os.Mkdir(filepath.Join(b, "devtooltui-dir"+exe), 0o755); err != nil {
		t.Fatal(err)
	}
	got := Discover(strings.Join([]string{a, "", b, filepath.Join(b, "missing")}, string(os.PathListSeparator)))
	want := []Candidate{{"one", filepath.Join(a, "devtooltui-one"+exe)}, {"two", filepath.Join(a, "devtooltui-two"+exe)}}
	if !slices.Equal(got, want) {
		t.Errorf("Discover = %+v, want %+v", got, want)
	}
}

func TestRegistry(t *testing.T) {
	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	name := "devtooltui-fake"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o755); err != nil { //nolint:gosec // a test executable
		t.Fatal(err)
	}
	t.Setenv("FAKE_PLUGIN", "ok")
	reg, infos, err := Registry(context.Background(), dir, builtin())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Get("fake-upper"); !ok || len(reg.All()) != 3 || len(infos) != 1 {
		t.Errorf("registry has %d tools, infos %+v", len(reg.All()), infos)
	}
	groups := reg.ByCategory()
	if last := groups[len(groups)-1]; last.Category != tool.Plugins {
		t.Errorf("last category = %q, want Plugins", last.Category)
	}
}

func TestDetectAndExport(t *testing.T) {
	tools, _ := Load(context.Background(), fake(t, "ok"), nil)
	up, magic := tools[0], tools[1]
	if _, ok := magic.(tool.Detector); ok {
		t.Error("a tool without detect implements Detector")
	}
	if _, ok := magic.(tool.Exporter); ok {
		t.Error("a tool without export implements Exporter")
	}
	d, ok := up.(tool.Detector)
	if !ok {
		t.Fatal("fake-upper declares detect but is no Detector")
	}
	if d.Detect("shout: hi") != 0.7 || d.Detect("whisper") != 0 {
		t.Errorf("Detect = %v / %v", d.Detect("shout: hi"), d.Detect("whisper"))
	}
	e, ok := up.(tool.Exporter)
	if !ok {
		t.Fatal("fake-upper declares export but is no Exporter")
	}
	data, ext, err := e.Export(context.Background(), tool.Request{Inputs: map[string]string{"text": "abc"}})
	if err != nil || string(data) != "ABC" || ext != ".txt" {
		t.Errorf("Export = %q, %q, %v", data, ext, err)
	}
	_, _, err = e.Export(context.Background(), tool.Request{Inputs: map[string]string{"text": ""}})
	if ie, ok := tool.AsInputError(err); !ok || ie.Input != "text" || ie.Msg != "nothing to export" {
		t.Errorf("Export(empty) = %v, want an InputError on the first input", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := e.Export(ctx, tool.Request{}); !errors.Is(err, context.Canceled) {
		t.Errorf("Export with a cancelled ctx = %v", err)
	}
	t.Setenv("FAKE_PLUGIN", "badext")
	if _, _, err := e.Export(context.Background(), tool.Request{Inputs: map[string]string{"text": "x"}}); err == nil || !strings.Contains(err.Error(), "must look like") {
		t.Errorf("a bad extension = %v", err)
	}
}
