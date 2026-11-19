// Package statusbar dibuja la última fila de la TUI: el estado de la
// herramienta activa a la izquierda y la ayuda contextual a la derecha.
package statusbar

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Styles los define tui/theme.go.
type Styles struct {
	OK, Error, Warning, Running, Help lipgloss.Style
}

// Status es lo que muestra la barra sobre la herramienta activa.
type Status struct {
	Running    bool
	Stale      bool // la entrada cambió y hay que presionar F5
	Err        error
	Elapsed    time.Duration
	Meta       []tool.KV
	LargeBytes int // > 0: la entrada está en modo de solo lectura
	LargeLines int
}

// View devuelve una fila de exactamente width columnas.
func View(st Status, help string, width int, s Styles) string {
	left := Text(st, s)
	right := s.Help.Render(help)
	rw := lipgloss.Width(right)
	if rw+1 > width/2 { // la ayuda no puede comerse el estado
		right, rw = "", 0
	}
	avail := width - rw
	if rw > 0 {
		avail-- // separación mínima
	}
	left = lipgloss.NewStyle().MaxWidth(avail).Render(left)
	gap := max(width-lipgloss.Width(left)-rw, 0)
	return left + strings.Repeat(" ", gap) + right
}

// Text es la parte de estado, sin la ayuda.
func Text(st Status, s Styles) string {
	var parts []string
	if st.LargeBytes > 0 {
		parts = append(parts, s.Warning.Render(fmt.Sprintf("⚠ %s · %d líneas · solo lectura (f5 ejecutar, ctrl+l limpiar)",
			HumanBytes(st.LargeBytes), st.LargeLines)))
	}
	switch {
	case st.Running:
		parts = append(parts, s.Running.Render("⋯ ejecutando"))
	case st.Err != nil:
		parts = append(parts, s.Error.Render("✗ "+st.Err.Error()))
	case st.Stale:
		parts = append(parts, s.Warning.Render("● f5 para ejecutar"))
	default:
		ok := []string{"✓ " + FormatDuration(st.Elapsed)}
		for _, kv := range st.Meta {
			ok = append(ok, kv.Key+": "+kv.Value)
		}
		parts = append(parts, s.OK.Render(strings.Join(ok, " · ")))
	}
	return strings.Join(parts, " · ")
}

// FormatDuration muestra µs por debajo de 1 ms y ms con un decimal arriba.
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

// HumanBytes formatea un tamaño en B, KB o MB.
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
