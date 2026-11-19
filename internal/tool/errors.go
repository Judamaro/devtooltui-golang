package tool

import (
	"errors"
	"fmt"
)

// InputError es un error de entrada con posición, para subrayar la línea en
// la TUI y reportarla en la CLI. Line y Col empiezan en 1; 0 = desconocido.
type InputError struct {
	Input     string
	Line, Col int
	Msg       string
}

func (e *InputError) Error() string {
	switch {
	case e.Line > 0 && e.Col > 0:
		return fmt.Sprintf("%d:%d: %s", e.Line, e.Col, e.Msg)
	case e.Line > 0:
		return fmt.Sprintf("%d: %s", e.Line, e.Msg)
	default:
		return e.Msg
	}
}

// AsInputError extrae un *InputError de la cadena de err.
func AsInputError(err error) (*InputError, bool) {
	var ie *InputError
	if errors.As(err, &ie) {
		return ie, true
	}
	return nil, false
}

// Position convierte un offset en bytes de s a línea y columna (en runas),
// ambas empezando en 1. Útil para construir un InputError.
func Position(s string, offset int) (line, col int) {
	if offset > len(s) {
		offset = len(s)
	}
	line, col = 1, 1
	for _, r := range s[:max(offset, 0)] {
		if r == '\n' {
			line++
			col = 1
			continue
		}
		col++
	}
	return line, col
}
