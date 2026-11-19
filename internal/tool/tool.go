// Package tool define el contrato que implementan todas las herramientas.
//
// Una herramienta es una Spec declarativa más una función pura
// Run(ctx, Request) → Result. La TUI y la CLI construyen su interfaz
// leyendo la Spec; ninguna herramienta escribe código de UI.
//
// Reglas del contrato:
//  1. Run no hace I/O ni guarda estado global, y respeta ctx.
//  2. Las herramientas reversibles usan una opción "mode" (Select) con
//     los valores "encode" y "decode".
//  3. Los errores de entrada devuelven *InputError; cualquier otro error
//     se considera un bug.
//  4. Las salidas binarias se representan en Base64 o hex según la
//     opción "output".
package tool

import "context"

type Category string

const (
	Converters Category = "Conversores"
	Encoders   Category = "Codificadores"
	Formatters Category = "Formateadores"
	Generators Category = "Generadores"
	Testers    Category = "Testers"
	Text       Category = "Texto"
)

// Categories devuelve las categorías en el orden en que se muestran.
func Categories() []Category {
	return []Category{Formatters, Encoders, Converters, Generators, Testers, Text}
}

type OptionKind int

const (
	Toggle OptionKind = iota // bool
	Select                   // una de Choices
	Number                   // int con Min/Max
	String                   // texto corto (secreto JWT, separador…)
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
	Lang       string // resaltado: "json", "xml", ""…
	Multiline  bool
}

type OutputKind int

const (
	PlainText   OutputKind = iota
	Highlighted            // usa Result.Lang
	DiffView               // usa Result.Diff
	Matches                // usa Result.Spans (regex, JSONPath)
	Image                  // bloques Unicode (QR)
)

type Spec struct {
	ID          string // "base64", "json-format"… (nombre del subcomando CLI)
	Name        string
	Category    Category
	Description string
	Keywords    []string
	Inputs      []InputSpec // 0 (generadores), 1 o 2 (diff, regex)
	Options     []Option
	Output      OutputKind
	Live        bool // false = recalcular solo con F5 (operaciones costosas)
}

type Request struct {
	Inputs map[string]string
	Opts   Values
}

type Result struct {
	Text  string
	Lang  string
	Diff  []DiffOp
	Spans []Span // rangos resaltados en la entrada
	Meta  []KV   // "Tamaño: 1.2 KB", "exp: 2026-12-01"… para la statusbar
}

// DiffKind indica el tipo de un fragmento de diff.
type DiffKind int

const (
	Equal DiffKind = iota
	Insert
	Delete
)

// DiffOp es un fragmento de un diff.
type DiffOp struct {
	Kind DiffKind
	Text string
}

// Span es un rango resaltado en una entrada, con offsets en bytes [Start, End).
type Span struct {
	Input      string // clave de InputSpec
	Start, End int
	Group      int // 0 = coincidencia completa, n = grupo de captura
}

// KV es un par clave/valor de metadatos para la statusbar.
type KV struct {
	Key, Value string
}

type Tool interface {
	Spec() Spec
	Run(ctx context.Context, req Request) (Result, error)
}

// Detector es opcional: participa en la detección inteligente.
type Detector interface {
	Detect(sample string) float64 // 0..1
}
