package tool

import (
	"errors"
	"fmt"
)

// InputError is an input error with a position, so the TUI can underline the
// line and the CLI can report it. Line and Col start at 1; 0 means unknown.
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

// AsInputError extracts an *InputError from err's chain.
func AsInputError(err error) (*InputError, bool) {
	var ie *InputError
	if errors.As(err, &ie) {
		return ie, true
	}
	return nil, false
}

// Position converts a byte offset in s into a line and column (in runes),
// both starting at 1. Handy for building an InputError.
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
