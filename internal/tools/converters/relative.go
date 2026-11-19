// Package converters groups the conversion tools.
package converters

import (
	"fmt"
	"math"
	"time"
)

// Relative describes d in English: "in 3 h", "2 d ago", "now".
func Relative(d time.Duration) string {
	abs := d.Abs()
	var amount string
	switch {
	case abs < time.Second:
		return "now"
	case abs < time.Minute:
		amount = fmt.Sprintf("%d s", int(abs.Seconds()))
	case abs < time.Hour:
		amount = fmt.Sprintf("%d min", int(abs.Minutes()))
	case abs < 48*time.Hour:
		amount = fmt.Sprintf("%d h", int(abs.Hours()))
	case abs < 2*365*24*time.Hour:
		amount = fmt.Sprintf("%d d", int(abs.Hours()/24))
	default:
		amount = fmt.Sprintf("%d years", int(math.Floor(abs.Hours()/24/365.25)))
	}
	if d > 0 {
		return "in " + amount
	}
	return amount + " ago"
}
