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
		{"line and column", &tool.InputError{Line: 3, Col: 7, Msg: "unexpected comma"}, "3:7: unexpected comma"},
		{"line only", &tool.InputError{Line: 2, Msg: "unexpected end"}, "2: unexpected end"},
		{"no position", &tool.InputError{Msg: "invalid base64"}, "invalid base64"},
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
		{"direct", ie, true},
		{"wrapped", fmt.Errorf("ctx: %w", ie), true},
		{"other error", errors.New("bug"), false},
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
		{"start", "abc", 0, 1, 1},
		{"same line", "abc", 2, 1, 3},
		{"second line", "ab\ncd", 4, 2, 2},
		{"multibyte runes", "ñé\nx", len("ñé"), 1, 3},
		{"offset out of range", "ab", 99, 1, 3},
		{"negative offset", "ab", -1, 1, 1},
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
		{"missing bool", v.Bool("nope"), false},
		{"string", v.String("s"), "x"},
		{"string wrong type", v.String("i"), ""},
		{"int", v.Int("i"), 4},
		{"int64", v.Int("i64"), 9},
		{"int wrong type", v.Int("wrong"), 0},
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
		t.Error("WithDefaults modified the original map")
	}
}
