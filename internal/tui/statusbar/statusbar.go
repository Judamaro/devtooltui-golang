// Package statusbar draws the TUI's last row: the active tool's status on
// the left and the contextual help on the right.
package statusbar

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Styles are defined by tui/theme.go.
type Styles struct {
	OK, Error, Warning, Running, Help lipgloss.Style
}

// Status is what the bar shows about the active tool.
type Status struct {
	Running    bool
	Stale      bool // the input changed and F5 must be pressed
	Err        error
	Elapsed    time.Duration
	Meta       []tool.KV
	LargeBytes int // > 0: the input is in read-only mode
	LargeLines int
	Note       string // secondary notice (e.g. output without colour)
}

// View returns a row exactly width columns wide.
func View(st Status, help string, width int, s Styles) string {
	left := Text(st, s)
	right := s.Help.Render(help)
	rw := lipgloss.Width(right)
	if rw+1 > width/2 { // the help must not eat the status
		right, rw = "", 0
	}
	avail := width - rw
	if rw > 0 {
		avail-- // minimum gap
	}
	left = lipgloss.NewStyle().MaxWidth(avail).Render(left)
	gap := max(width-lipgloss.Width(left)-rw, 0)
	return left + strings.Repeat(" ", gap) + right
}

// Text is the status part, without the help.
func Text(st Status, s Styles) string {
	var parts []string
	if st.LargeBytes > 0 {
		parts = append(parts, s.Warning.Render(fmt.Sprintf("⚠ %s · %d lines · read-only (f5 run, ctrl+l clear)",
			HumanBytes(st.LargeBytes), st.LargeLines)))
	}
	switch {
	case st.Running:
		parts = append(parts, s.Running.Render("⋯ running"))
	case st.Err != nil:
		parts = append(parts, s.Error.Render("✗ "+st.Err.Error()))
	case st.Stale:
		parts = append(parts, s.Warning.Render("● press f5 to run"))
	default:
		ok := []string{"✓ " + FormatDuration(st.Elapsed)}
		for _, kv := range st.Meta {
			ok = append(ok, kv.Key+": "+kv.Value)
		}
		parts = append(parts, s.OK.Render(strings.Join(ok, " · ")))
	}
	if st.Note != "" {
		parts = append(parts, s.Warning.Render(st.Note))
	}
	return strings.Join(parts, " · ")
}

// FormatDuration shows µs below 1 ms and ms with one decimal above.
func FormatDuration(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return fmt.Sprintf("%d µs", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%.1f ms", float64(d)/float64(time.Millisecond))
	default:
		return fmt.Sprintf("%.2f s", d.Seconds())
	}
}

// HumanBytes formats a size in B, KB or MB.
func HumanBytes(n int) string {
	switch {
	case n < 1<<10:
		return fmt.Sprintf("%d B", n)
	case n < 1<<20:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
}
