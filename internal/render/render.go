package render

import (
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Styles gathers the colouring styles (the TUI takes them from theme.go).
type Styles struct {
	Chroma *chroma.Style  // Highlighted outputs
	Match  lipgloss.Style // matches in Matches outputs
	Diff   DiffStyles     // DiffView outputs
}

// Result colours the result according to the tool's output kind. inputs are
// the run's inputs (Matches highlights over them). It returns an empty
// rendered when it does not apply, and noColor when skipped for size.
func Result(res tool.Result, kind tool.OutputKind, inputs map[string]string, s Styles) (rendered string, noColor bool) {
	switch {
	case res.Lang == "color":
		for _, kv := range res.Meta {
			if kv.Key == "Hex" {
				return Swatch(kv.Value) + "\n" + res.Text, false
			}
		}
	case kind == tool.Highlighted && res.Lang != "":
		if len(res.Text) > MaxBytes {
			return "", true
		}
		out, _ := Highlight(res.Text, res.Lang, s.Chroma)
		return out, false
	case kind == tool.DiffView:
		if len(res.Text) > MaxBytes {
			return "", true
		}
		return Diff(res, s.Diff), false
	case kind == tool.Matches && len(res.Spans) > 0:
		// The text with highlighted matches goes above the list.
		subject := inputs[res.Spans[0].Input]
		if len(subject) > MaxBytes {
			return "", true
		}
		return Matches(subject, res.Spans, s.Match) + "\n\n" + res.Text, false
	}
	return "", false
}
