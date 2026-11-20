package render

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Swatch draws two rows of the colour hex ("#rrggbb"). The colour is data,
// not theme, so it is the one style built outside theme.go; the terminal's
// colour profile downsamples it when there is no truecolor.
func Swatch(hex string) string {
	row := lipgloss.NewStyle().Background(lipgloss.Color(hex)).Render(strings.Repeat(" ", 20))
	return row + "\n" + row
}
