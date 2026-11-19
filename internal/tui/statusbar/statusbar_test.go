package statusbar_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tui/statusbar"
)

var styles = statusbar.Styles{}

func TestText(t *testing.T) {
	tests := []struct {
		name string
		st   statusbar.Status
		want string
	}{
		{"ok con meta", statusbar.Status{Elapsed: 1200 * time.Microsecond, Meta: []tool.KV{{Key: "Salida", Value: "8 B"}}}, "✓ 1.2 ms · Salida: 8 B"},
		{"ejecutando gana al error", statusbar.Status{Running: true, Err: errors.New("x")}, "⋯ ejecutando"},
		{"error de entrada", statusbar.Status{Err: &tool.InputError{Line: 3, Col: 7, Msg: "coma de más"}}, "✗ 3:7: coma de más"},
		{"pendiente de f5", statusbar.Status{Stale: true}, "● f5 para ejecutar"},
		{"entrada grande", statusbar.Status{LargeBytes: 40000, LargeLines: 2, Elapsed: 5 * time.Microsecond},
			"⚠ 39.1 KB · 2 líneas · solo lectura (f5 ejecutar, ctrl+l limpiar) · ✓ 5 µs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ansi.Strip(statusbar.Text(tt.st, styles)); got != tt.want {
				t.Errorf("Text() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestView(t *testing.T) {
	st := statusbar.Status{Err: &tool.InputError{Line: 1, Col: 2, Msg: strings.Repeat("largo ", 30)}}
	tests := []struct {
		name     string
		width    int
		help     string
		wantHelp bool
	}{
		{"ancho", 100, "tab foco • ctrl+c salir", true},
		{"angosto oculta la ayuda", 30, "tab foco • ctrl+b sidebar • ctrl+c salir", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ansi.Strip(statusbar.View(st, tt.help, tt.width, styles))
			if w := ansi.StringWidth(got); w != tt.width {
				t.Errorf("ancho = %d, want %d: %q", w, tt.width, got)
			}
			if strings.HasSuffix(got, tt.help) != tt.wantHelp {
				t.Errorf("ayuda visible = %v, want %v: %q", !tt.wantHelp, tt.wantHelp, got)
			}
			if !strings.HasPrefix(got, "✗ 1:2:") {
				t.Errorf("el estado no va a la izquierda: %q", got)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0 µs"}, {999 * time.Microsecond, "999 µs"}, {1500 * time.Microsecond, "1.5 ms"}, {2500 * time.Millisecond, "2.50 s"},
	}
	for _, tt := range tests {
		if got := statusbar.FormatDuration(tt.d); got != tt.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0 B"}, {1023, "1023 B"}, {1024, "1.0 KB"}, {1536, "1.5 KB"}, {1 << 20, "1.0 MB"},
	}
	for _, tt := range tests {
		if got := statusbar.HumanBytes(tt.n); got != tt.want {
			t.Errorf("HumanBytes(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
