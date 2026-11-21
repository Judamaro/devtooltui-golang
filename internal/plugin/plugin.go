package plugin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Prefix names the plugin executables: devtooltui-<name>.
const Prefix = "devtooltui-"

// Limits for a plugin process.
const (
	SpecTimeout = 5 * time.Second
	RunTimeout  = 10 * time.Second
	MaxOutput   = 64 << 20 // stdout
	maxStderr   = 4 << 10  // kept for error messages
)

// runTimeout is RunTimeout; tests shorten it.
var runTimeout = RunTimeout

// Candidate is an executable found in the PATH.
type Candidate struct {
	Name string // without the prefix (and .exe)
	Path string
}

// Discover lists devtooltui-* executables in the directories of pathList
// (the PATH). Like exec.LookPath, the first directory wins for a name. On
// Windows only .exe files count; elsewhere, files with an execute bit.
func Discover(pathList string) []Candidate {
	var out []Candidate
	seen := map[string]bool{}
	for _, dir := range filepath.SplitList(pathList) {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name, ok := pluginName(e.Name())
			if !ok || seen[name] || e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil || !executable(info) {
				continue
			}
			seen[name] = true
			out = append(out, Candidate{Name: name, Path: filepath.Join(dir, e.Name())})
		}
	}
	slices.SortFunc(out, func(a, b Candidate) int { return strings.Compare(a.Name, b.Name) })
	return out
}

func pluginName(file string) (string, bool) {
	name, ok := strings.CutPrefix(file, Prefix)
	if !ok || name == "" {
		return "", false
	}
	if runtime.GOOS == "windows" {
		base, isExe := strings.CutSuffix(strings.ToLower(name), ".exe")
		return base, isExe && base != ""
	}
	return name, true
}

func executable(info os.FileInfo) bool {
	if runtime.GOOS == "windows" {
		return info.Mode().IsRegular()
	}
	return info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

// Info describes a plugin for "devtooltui plugin list".
type Info struct {
	Name     string
	Path     string
	Protocol int
	Tools    []string // IDs that were registered
	Problems []string // why the plugin, or some of its tools, were skipped
}

// Load asks every candidate for its spec (in parallel; from cache when the
// executable has not changed) and returns the tools that can join the
// built-in ones: valid specs whose IDs are free. cache may be nil; Load
// updates it.
func Load(ctx context.Context, cands []Candidate, builtin []tool.Tool, cache *Cache) ([]tool.Tool, []Info) {
	type loaded struct {
		msg SpecMessage
		err error
	}
	results := make([]loaded, len(cands))
	stamps := make([]stamp, len(cands))
	var wg sync.WaitGroup
	for i, c := range cands {
		stamps[i] = stampOf(c.Path)
		if msg, ok := cache.get(c.Path, stamps[i]); ok {
			results[i].msg = msg
			continue
		}
		wg.Go(func() {
			results[i].msg, results[i].err = readSpec(ctx, c.Path)
		})
	}
	wg.Wait()
	for i, c := range cands {
		if results[i].err == nil {
			cache.put(c.Path, stamps[i], results[i].msg)
		}
	}
	cache.keep(cands)

	taken := map[string]string{}
	for _, t := range builtin {
		taken[t.Spec().ID] = "a built-in tool"
	}
	var tools []tool.Tool
	infos := make([]Info, len(cands))
	for i, c := range cands {
		info := Info{Name: c.Name, Path: c.Path, Protocol: results[i].msg.Protocol}
		switch {
		case results[i].err != nil:
			info.Problems = append(info.Problems, results[i].err.Error())
		case results[i].msg.Protocol != Protocol:
			info.Problems = append(info.Problems, fmt.Sprintf("speaks protocol %d; this devtooltui speaks %d", results[i].msg.Protocol, Protocol))
		default:
			for _, ts := range results[i].msg.Tools {
				t, err := newTool(ts, c)
				if err == nil {
					// The registry checks IDs, kinds and defaults.
					_, err = registry.New(t)
				}
				switch {
				case err != nil:
					info.Problems = append(info.Problems, err.Error())
				case taken[ts.ID] != "":
					info.Problems = append(info.Problems, fmt.Sprintf("%s: the ID is taken by %s", ts.ID, taken[ts.ID]))
				default:
					taken[ts.ID] = "plugin " + c.Name
					tools = append(tools, t)
					info.Tools = append(info.Tools, ts.ID)
				}
			}
		}
		infos[i] = info
	}
	return tools, infos
}

func readSpec(ctx context.Context, path string) (SpecMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, SpecTimeout)
	defer cancel()
	out, err := call(ctx, path, "spec", nil)
	if err != nil {
		return SpecMessage{}, err
	}
	var msg SpecMessage
	if err := json.Unmarshal(out, &msg); err != nil {
		return SpecMessage{}, fmt.Errorf("spec is not valid JSON: %w", err)
	}
	return msg, nil
}

// call runs path with one argument, stdin as given, and returns stdout. A
// non-zero exit includes the end of stderr in the error.
func call(ctx context.Context, path, arg string, stdin []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, path, arg) //nolint:gosec // the user put the plugin in the PATH
	cmd.Env = append(os.Environ(), fmt.Sprintf("DEVTOOLTUI_PROTOCOL=%d", Protocol))
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout limitedBuffer
	stdout.max = MaxOutput
	var stderr limitedBuffer
	stderr.max = maxStderr
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.WaitDelay = time.Second // do not wait forever for children that keep the pipes open
	err := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			return nil, fmt.Errorf("%s %s took too long", filepath.Base(path), arg)
		}
		return nil, ctxErr
	}
	if stdout.overflow {
		return nil, fmt.Errorf("%s %s wrote more than %d MB", filepath.Base(path), arg, MaxOutput>>20)
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("%s %s: %w: %s", filepath.Base(path), arg, err, msg)
		}
		return nil, fmt.Errorf("%s %s: %w", filepath.Base(path), arg, err)
	}
	return stdout.Bytes(), nil
}

// limitedBuffer keeps up to max bytes and notes the overflow.
type limitedBuffer struct {
	bytes.Buffer
	max      int
	overflow bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := b.max - b.Len(); len(p) > room {
		b.overflow = true
		b.Buffer.Write(p[:max(room, 0)])
		return len(p), nil
	}
	return b.Buffer.Write(p)
}

var _ io.Writer = (*limitedBuffer)(nil)

// Tool adapts one tool of a plugin to tool.Tool. It is the only Tool that
// does I/O: Run starts the plugin's process (see contract rule 1).
type Tool struct {
	spec   tool.Spec
	path   string
	plugin string
}

func (t *Tool) Spec() tool.Spec { return t.spec }

// Plugin returns the plugin name and executable path.
func (t *Tool) Plugin() (name, path string) { return t.plugin, t.path }

func (t *Tool) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	out, err := t.request(ctx, "run", req)
	if err != nil {
		return tool.Result{}, err
	}
	var resp RunResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return tool.Result{}, fmt.Errorf("plugin %s: run did not answer valid JSON: %w", t.plugin, err)
	}
	switch {
	case resp.Error != nil && resp.Result == nil:
		return tool.Result{}, t.inputError(resp.Error)
	case resp.Result != nil && resp.Error == nil:
		res, err := resp.Result.toResult()
		if err != nil {
			return tool.Result{}, fmt.Errorf("plugin %s: %w", t.plugin, err)
		}
		if err := checkSpans(res.Spans, req.Inputs); err != nil {
			return tool.Result{}, fmt.Errorf("plugin %s: %w", t.plugin, err)
		}
		return res, nil
	}
	return tool.Result{}, fmt.Errorf("plugin %s: run must answer exactly one of result and error", t.plugin)
}

// newTool builds the adapter for one tool of a plugin: a *Tool, wrapped so
// that it implements tool.Detector and tool.Exporter only when the plugin
// declares them (the registry looks at the interfaces).
func newTool(ts ToolSpec, c Candidate) (tool.Tool, error) {
	spec, err := ts.toSpec()
	if err != nil {
		return nil, err
	}
	base := &Tool{spec: spec, path: c.Path, plugin: c.Name}
	var det *detectTool
	if ts.Detect != nil {
		re, err := regexp.Compile(ts.Detect.Regex)
		if err != nil {
			return nil, fmt.Errorf("%s: detect.regex: %w", ts.ID, err)
		}
		if ts.Detect.Score <= 0 || ts.Detect.Score > 1 {
			return nil, fmt.Errorf("%s: detect.score must be in (0, 1], got %v", ts.ID, ts.Detect.Score)
		}
		det = &detectTool{Tool: base, re: re, score: ts.Detect.Score}
	}
	switch {
	case det != nil && ts.Export:
		return &detectExportTool{det}, nil
	case det != nil:
		return det, nil
	case ts.Export:
		return &exportTool{base}, nil
	}
	return base, nil
}

type detectTool struct {
	*Tool
	re    *regexp.Regexp
	score float64
}

// Detect matches the declared regex in-process.
func (t *detectTool) Detect(sample string) float64 {
	if t.re.MatchString(sample) {
		return t.score
	}
	return 0
}

type exportTool struct{ *Tool }

func (t *exportTool) Export(ctx context.Context, req tool.Request) ([]byte, string, error) {
	return t.export(ctx, req)
}

type detectExportTool struct{ *detectTool }

func (t *detectExportTool) Export(ctx context.Context, req tool.Request) ([]byte, string, error) {
	return t.export(ctx, req)
}

// request runs one command of the protocol with the request on stdin.
func (t *Tool) request(ctx context.Context, arg string, req tool.Request) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(RunRequest{
		Protocol: Protocol, Tool: t.spec.ID,
		Inputs:  req.Inputs,
		Options: req.Opts.WithDefaults(t.spec),
	})
	if err != nil {
		return nil, err
	}
	rctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()
	out, err := call(rctx, t.path, arg, body)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("plugin %s: %w", t.plugin, err)
	}
	return out, nil
}

func (t *Tool) inputError(e *InputError) error {
	ie := &tool.InputError{Input: e.Input, Line: e.Line, Col: e.Col, Msg: e.Message}
	if ie.Input == "" && len(t.spec.Inputs) > 0 {
		ie.Input = t.spec.Inputs[0].Key
	}
	return ie
}

func (t *Tool) export(ctx context.Context, req tool.Request) ([]byte, string, error) {
	out, err := t.request(ctx, "export", req)
	if err != nil {
		return nil, "", err
	}
	var resp ExportResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, "", fmt.Errorf("plugin %s: export did not answer valid JSON: %w", t.plugin, err)
	}
	switch {
	case resp.Error != nil && resp.Export == nil:
		return nil, "", t.inputError(resp.Error)
	case resp.Export != nil && resp.Error == nil:
		data, err := base64.StdEncoding.DecodeString(resp.Export.Data)
		if err != nil {
			return nil, "", fmt.Errorf("plugin %s: export data is not Base64: %w", t.plugin, err)
		}
		ext := resp.Export.Ext
		if !strings.HasPrefix(ext, ".") || len(ext) < 2 || strings.ContainsAny(ext, `/\`) {
			return nil, "", fmt.Errorf("plugin %s: export ext %q must look like \".png\"", t.plugin, ext)
		}
		return data, ext, nil
	}
	return nil, "", fmt.Errorf("plugin %s: export must answer exactly one of export and error", t.plugin)
}

// checkSpans refuses spans outside their input: render slices the input
// with them.
func checkSpans(spans []tool.Span, inputs map[string]string) error {
	for _, sp := range spans {
		in, ok := inputs[sp.Input]
		if !ok || sp.Start < 0 || sp.Start > sp.End || sp.End > len(in) {
			return fmt.Errorf("span %+v is outside its input", sp)
		}
	}
	return nil
}

// Registry builds the registry with the built-in tools and the plugins
// found in pathList. cacheFile ("" for none) keeps their specs between
// runs. Broken plugins never stop it: their problems are in the returned
// Info.
func Registry(ctx context.Context, pathList, cacheFile string, builtin []tool.Tool) (*registry.Registry, []Info, error) {
	cands := Discover(pathList)
	var cache *Cache
	if cacheFile != "" && len(cands) > 0 {
		cache = LoadCache(cacheFile)
	}
	extra, infos := Load(ctx, cands, builtin, cache)
	if cache != nil && cache.changed {
		_ = cache.Save(cacheFile) // a cache that cannot be written only costs time
	}
	reg, err := registry.New(append(slices.Clone(builtin), extra...)...)
	return reg, infos, err
}
