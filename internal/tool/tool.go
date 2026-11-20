// Package tool defines the contract every tool implements.
//
// A tool is a declarative Spec plus a pure function
// Run(ctx, Request) → Result. The TUI and the CLI build their interface by
// reading the Spec; no tool writes UI code.
//
// Contract rules:
//  1. Run does no I/O, keeps no global state and honours ctx. The one
//     exception is the plugin adapter (internal/plugin), which starts the
//     plugin's process; the plugin itself must follow the rule.
//  2. Reversible tools use a "mode" option (Select) with the values
//     "encode" and "decode"; conversions between two formats name the
//     target instead ("to-yaml", "to-json").
//  3. Input errors are returned as *InputError; any other error is
//     considered a bug.
//  4. Binary outputs are shown as Base64 or hex depending on the
//     "output" option.
package tool

import "context"

type Category string

const (
	Converters Category = "Converters"
	Encoders   Category = "Encoders"
	Formatters Category = "Formatters"
	Generators Category = "Generators"
	Testers    Category = "Testers"
	Text       Category = "Text"
	// Plugins holds external tools whose category is not one of the above.
	Plugins Category = "Plugins"
)

// Categories returns the categories in display order.
func Categories() []Category {
	return []Category{Formatters, Encoders, Converters, Generators, Testers, Text, Plugins}
}

type OptionKind int

const (
	Toggle OptionKind = iota // bool
	Select                   // one of Choices
	Number                   // int within Min/Max
	String                   // short text (JWT secret, separator…)
)

type Option struct {
	Key, Label, Help string
	Kind             OptionKind
	Choices          []string
	Min, Max         int
	Default          any
}

type InputSpec struct {
	Key, Label string
	Lang       string // highlighting: "json", "xml", ""…
	Multiline  bool
}

type OutputKind int

const (
	PlainText   OutputKind = iota
	Highlighted            // uses Result.Lang
	DiffView               // uses Result.Diff
	Matches                // uses Result.Spans (regex, JSONPath)
	Image                  // Unicode blocks (QR)
)

type Spec struct {
	ID          string // "base64", "json-format"… (CLI subcommand name)
	Name        string
	Category    Category
	Description string
	Keywords    []string
	Inputs      []InputSpec // 0 (generators), 1 or 2 (diff, jwt)
	Options     []Option
	Output      OutputKind
	Live        bool // false = recompute only on F5 (expensive operations)
}

type Request struct {
	Inputs map[string]string
	Opts   Values
}

type Result struct {
	Text  string
	Lang  string
	Diff  []DiffOp
	Spans []Span // highlighted ranges in the input
	Meta  []KV   // "Size: 1.2 KB", "exp: 2026-12-01"… for the statusbar
}

// DiffKind is the kind of a diff fragment.
type DiffKind int

const (
	Equal DiffKind = iota
	Insert
	Delete
)

// DiffOp is a fragment of a diff.
type DiffOp struct {
	Kind DiffKind
	Text string
}

// Span is a highlighted range in an input, as byte offsets [Start, End).
type Span struct {
	Input      string // InputSpec key
	Start, End int
	Group      int // 0 = whole match, n = capture group
}

// KV is a key/value metadata pair for the statusbar.
type KV struct {
	Key, Value string
}

type Tool interface {
	Spec() Spec
	Run(ctx context.Context, req Request) (Result, error)
}

// Detector is optional: it takes part in smart detection.
type Detector interface {
	Detect(sample string) float64 // 0..1
}

// Exporter is optional: tools whose output also has a binary form (the PNG
// of a QR code) implement it. The TUI uses it when Ctrl+S saves to a path
// with that extension, and the CLI with --out. Like Run, it does no I/O and
// honours ctx; ext includes the dot (".png").
type Exporter interface {
	Export(ctx context.Context, req Request) (data []byte, ext string, err error)
}
