package render

import (
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
)

// Markdown renders src for the terminal with Glamour, wrapped at width
// (80 when width <= 0). Code blocks use Chroma's 16-colour formatter, like
// the rest of the output.
func Markdown(src string, width int, style ansi.StyleConfig) (string, error) {
	if width <= 0 {
		width = 80
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(width),
		glamour.WithChromaFormatter("terminal16"),
	)
	if err != nil {
		return "", err
	}
	out, err := r.Render(src)
	return strings.Trim(out, "\n"), err
}
