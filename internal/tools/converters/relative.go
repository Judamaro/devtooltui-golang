// Package converters agrupa las herramientas de conversión.
package converters

import (
	"fmt"
	"math"
	"time"
)

// Relative describe d en español: "en 3 h", "hace 2 d", "ahora".
func Relative(d time.Duration) string {
	abs := d.Abs()
	var amount string
	switch {
	case abs < time.Second:
		return "ahora"
	case abs < time.Minute:
		amount = fmt.Sprintf("%d s", int(abs.Seconds()))
	case abs < time.Hour:
		amount = fmt.Sprintf("%d min", int(abs.Minutes()))
	case abs < 48*time.Hour:
		amount = fmt.Sprintf("%d h", int(abs.Hours()))
	case abs < 2*365*24*time.Hour:
		amount = fmt.Sprintf("%d d", int(abs.Hours()/24))
	default:
		amount = fmt.Sprintf("%d años", int(math.Floor(abs.Hours()/24/365.25)))
	}
	if d > 0 {
		return "en " + amount
	}
	return "hace " + amount
}
