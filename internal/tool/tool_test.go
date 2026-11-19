package tool_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

func TestInputErrorError(t *testing.T) {
	tests := []struct {
		name string
		err  *tool.InputError
		want string
	}{
		{"linea y columna", &tool.InputError{Line: 3, Col: 7, Msg: "coma inesperada"}, "3:7: coma inesperada"},
		{"solo linea", &tool.InputError{Line: 2, Msg: "fin inesperado"}, "2: fin inesperado"},
		{"sin posicion", &tool.InputError{Msg: "base64 inválido"}, "base64 inválido"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAsInputError(t *testing.T) {
	ie := &tool.InputError{Msg: "x"}
	tests := []struct {
		name string
		err  error
		ok   bool
	}{
		{"directo", ie, true},
		{"envuelto", fmt.Errorf("ctx: %w", ie), true},
		{"otro error", errors.New("bug"), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tool.AsInputError(tt.err)
			if ok != tt.ok || (ok && got != ie) {
				t.Errorf("AsInputError() = %v, %v; want ok=%v", got, ok, tt.ok)
			}
		})
	}
}

func TestPosition(t *testing.T) {
	tests := []struct {
		name      string
		s         string
		offset    int
		line, col int
	}{
		{"inicio", "abc", 0, 1, 1},
		{"misma linea", "abc", 2, 1, 3},
		{"segunda linea", "ab\ncd", 4, 2, 2},
		{"runas multibyte", "ñé\nx", len("ñé"), 1, 3},
		{"offset fuera de rango", "ab", 99, 1, 3},
		{"offset negativo", "ab", -1, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, c := tool.Position(tt.s, tt.offset)
			if l != tt.line || c != tt.col {
				t.Errorf("Position() = %d:%d, want %d:%d", l, c, tt.line, tt.col)
			}
		})
	}
}

func TestValues(t *testing.T) {
	v := tool.Values{"b": true, "s": "x", "i": 4, "i64": int64(9), "wrong": "1"}
	tests := []struct {
		name string
		got  any
		want any
	}{
		{"bool", v.Bool("b"), true},
		{"bool faltante", v.Bool("nope"), false},
		{"string", v.String("s"), "x"},
		{"string tipo incorrecto", v.String("i"), ""},
		{"int", v.Int("i"), 4},
		{"int64", v.Int("i64"), 9},
		{"int tipo incorrecto", v.Int("wrong"), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestValuesWithDefaults(t *testing.T) {
	spec := tool.Spec{Options: []tool.Option{
		{Key: "mode", Kind: tool.Select, Choices: []string{"encode", "decode"}, Default: "encode"},
		{Key: "padding", Kind: tool.Toggle, Default: true},
	}}
	in := tool.Values{"mode": "decode"}
	got := in.WithDefaults(spec)
	if got.String("mode") != "decode" || !got.Bool("padding") {
		t.Errorf("WithDefaults() = %v", got)
	}
	if _, ok := in["padding"]; ok {
		t.Error("WithDefaults modificó el mapa original")
	}
}
