// Package plugin runs external tools: executables named devtooltui-<name>
// in the PATH that speak JSON over stdin and stdout, like git's
// subcommands. docs/plugins.md describes the protocol; ADR 0004 explains
// why processes and not WASM.
//
//	devtooltui-<name> spec   → {"protocol": 1, "tools": [Spec…]}
//	devtooltui-<name> run    ← {"protocol": 1, "tool": id, "inputs": {…}, "options": {…}}
//	                         → {"result": {…}} or {"error": {…}}
package plugin

import (
	"fmt"
	"math"
	"slices"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Protocol is the version this devtooltui speaks.
const Protocol = 1

// SpecMessage is the output of "spec".
type SpecMessage struct {
	Protocol int        `json:"protocol"`
	Tools    []ToolSpec `json:"tools"`
}

// ToolSpec is tool.Spec in JSON.
type ToolSpec struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Category    string       `json:"category"`
	Description string       `json:"description"`
	Keywords    []string     `json:"keywords,omitempty"`
	Inputs      []InputSpec  `json:"inputs,omitempty"`
	Options     []OptionSpec `json:"options,omitempty"`
	Output      string       `json:"output,omitempty"` // plain (default), highlighted, diff, matches, image
	Live        *bool        `json:"live,omitempty"`   // default true
	Detect      *DetectSpec  `json:"detect,omitempty"` // smart detection without running the plugin
	Export      bool         `json:"export,omitempty"` // it answers "export" (binary output)
}

// DetectSpec is declarative detection: devtooltui matches Regex (RE2)
// against the sample itself and scores Score (0..1] on a match, because
// starting a process takes longer than a detector may.
type DetectSpec struct {
	Regex string  `json:"regex"`
	Score float64 `json:"score"`
}

// ExportResponse is the output of "export": exactly one of Export and
// Error.
type ExportResponse struct {
	Export *ExportData `json:"export,omitempty"`
	Error  *InputError `json:"error,omitempty"`
}

// ExportData is the binary output in Base64 and its extension (".png").
type ExportData struct {
	Data string `json:"data"`
	Ext  string `json:"ext"`
}

// InputSpec is tool.InputSpec in JSON.
type InputSpec struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Lang      string `json:"lang,omitempty"`
	Multiline bool   `json:"multiline,omitempty"`
}

// OptionSpec is tool.Option in JSON; kind is toggle, select, number or
// string.
type OptionSpec struct {
	Key     string   `json:"key"`
	Label   string   `json:"label"`
	Help    string   `json:"help,omitempty"`
	Kind    string   `json:"kind"`
	Choices []string `json:"choices,omitempty"`
	Min     int      `json:"min,omitempty"`
	Max     int      `json:"max,omitempty"`
	Default any      `json:"default"`
}

// RunRequest is the input of "run".
type RunRequest struct {
	Protocol int               `json:"protocol"`
	Tool     string            `json:"tool"`
	Inputs   map[string]string `json:"inputs"`
	Options  map[string]any    `json:"options"`
}

// RunResponse is the output of "run": exactly one of Result and Error.
type RunResponse struct {
	Result *Result     `json:"result,omitempty"`
	Error  *InputError `json:"error,omitempty"`
}

// Result is tool.Result in JSON, the same shape as the CLI's --json.
type Result struct {
	Text  string `json:"text"`
	Lang  string `json:"lang,omitempty"`
	Meta  []KV   `json:"meta,omitempty"`
	Diff  []Diff `json:"diff,omitempty"`
	Spans []Span `json:"spans,omitempty"`
}

// KV is a meta entry.
type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Diff is a diff fragment; op is equal, insert or delete.
type Diff struct {
	Op   string `json:"op"`
	Text string `json:"text"`
}

// Span is a highlighted byte range of an input.
type Span struct {
	Input string `json:"input"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Group int    `json:"group"`
}

// InputError is tool.InputError in JSON.
type InputError struct {
	Input   string `json:"input"`
	Line    int    `json:"line"`
	Col     int    `json:"col"`
	Message string `json:"message"`
}

var (
	optionKinds = map[string]tool.OptionKind{"toggle": tool.Toggle, "select": tool.Select, "number": tool.Number, "string": tool.String}
	outputKinds = map[string]tool.OutputKind{"": tool.PlainText, "plain": tool.PlainText, "highlighted": tool.Highlighted,
		"diff": tool.DiffView, "matches": tool.Matches, "image": tool.Image}
	diffOps = map[string]tool.DiffKind{"equal": tool.Equal, "insert": tool.Insert, "delete": tool.Delete}
)

// toSpec converts a plugin's spec. A category devtooltui does not know goes
// to Plugins; defaults decoded from JSON (float64) become the Go type of
// their kind. registry.New validates the rest.
func (s ToolSpec) toSpec() (tool.Spec, error) {
	cat := tool.Category(s.Category)
	if !slices.Contains(tool.Categories(), cat) {
		cat = tool.Plugins
	}
	out, ok := outputKinds[s.Output]
	if !ok {
		return tool.Spec{}, fmt.Errorf("%s: unknown output %q", s.ID, s.Output)
	}
	spec := tool.Spec{
		ID: s.ID, Name: s.Name, Category: cat, Description: s.Description,
		Keywords: s.Keywords, Output: out, Live: s.Live == nil || *s.Live,
	}
	for _, in := range s.Inputs {
		spec.Inputs = append(spec.Inputs, tool.InputSpec{Key: in.Key, Label: in.Label, Lang: in.Lang, Multiline: in.Multiline})
	}
	for _, o := range s.Options {
		kind, ok := optionKinds[o.Kind]
		if !ok {
			return tool.Spec{}, fmt.Errorf("%s: option %q has unknown kind %q", s.ID, o.Key, o.Kind)
		}
		def := o.Default
		if f, isFloat := def.(float64); isFloat && kind == tool.Number && f == math.Trunc(f) && math.Abs(f) < 1<<53 {
			def = int(f)
		}
		spec.Options = append(spec.Options, tool.Option{
			Key: o.Key, Label: o.Label, Help: o.Help, Kind: kind,
			Choices: o.Choices, Min: o.Min, Max: o.Max, Default: def,
		})
	}
	return spec, nil
}

func (r *Result) toResult() (tool.Result, error) {
	res := tool.Result{Text: r.Text, Lang: r.Lang}
	for _, kv := range r.Meta {
		res.Meta = append(res.Meta, tool.KV{Key: kv.Key, Value: kv.Value})
	}
	for _, d := range r.Diff {
		op, ok := diffOps[d.Op]
		if !ok {
			return tool.Result{}, fmt.Errorf("unknown diff op %q", d.Op)
		}
		res.Diff = append(res.Diff, tool.DiffOp{Kind: op, Text: d.Text})
	}
	for _, sp := range r.Spans {
		res.Spans = append(res.Spans, tool.Span{Input: sp.Input, Start: sp.Start, End: sp.End, Group: sp.Group})
	}
	return res, nil
}
