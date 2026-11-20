package converters_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/converters"
)

func runColor(t testing.TB, in string) (tool.Result, error) {
	t.Helper()
	return converters.Color{}.Run(context.Background(), tool.Request{Inputs: map[string]string{"input": in}})
}

func TestColorGolden(t *testing.T) {
	for _, tt := range []struct{ name, in string }{
		{"rebeccapurple", "rebeccapurple"},
		{"hsla-css4", "hsl(210deg 60% 40% / 25%)"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runColor(t, tt.in)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "color", tt.name, res.Text)
		})
	}
}

func TestColor(t *testing.T) {
	tests := []struct {
		in   string
		rows []string // lines the output must contain
	}{
		{"#ff8800", []string{"HEX:      #ff8800", "RGB:      rgb(255, 136, 0)", "HSL:      hsl(32, 100%, 50%)", "HSV:      hsv(32, 100%, 100%)"}},
		{"#F80", []string{"HEX:      #ff8800"}},
		{"  ff8800  ", []string{"HEX:      #ff8800"}},
		{"#ff880080", []string{"HEX:      #ff880080", "RGB:      rgba(255, 136, 0, 0.502)"}},
		{"rgb(255, 0, 0)", []string{"HEX:      #ff0000", "Name:     red"}},
		{"RGBA(0 128 0 / 1)", []string{"HEX:      #008000", "Name:     green"}},
		{"rgb(100%, 50%, 0%)", []string{"RGB:      rgb(255, 128, 0)"}},
		{"hsl(0, 0%, 50%)", []string{"HEX:      #808080", "Name:     gray"}},
		{"hsl(-120, 100%, 50%)", []string{"HEX:      #0000ff"}},
		{"White", []string{"HEX:      #ffffff", "Contrast: 1.00:1 on white, 21.00:1 on black"}},
		{"black", []string{"Contrast: 21.00:1 on white, 1.00:1 on black"}},
		{"transparent", []string{"HEX:      #00000000"}},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			res, err := runColor(t, tt.in)
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range tt.rows {
				if !strings.Contains(res.Text+"\n", r+"\n") {
					t.Errorf("missing %q in:\n%s", r, res.Text)
				}
			}
			if res.Lang != converters.ColorLang || res.Meta[0].Key != "Hex" {
				t.Errorf("Lang = %q, Meta = %v", res.Lang, res.Meta)
			}
		})
	}
}

func TestColorEmpty(t *testing.T) {
	if res, err := runColor(t, " \n"); err != nil || res.Text != "" {
		t.Errorf("Run(empty) = %q, %v", res.Text, err)
	}
}

func TestColorErrors(t *testing.T) {
	tests := []struct {
		in        string
		line, col int
		msg       string
	}{
		{"#12345g", 1, 7, "not a hex digit"},
		{"#12345", 1, 1, "3, 4, 6 or 8 digits"},
		{"rgb(10, 300, 0)", 1, 9, "out of range"},
		{"  rgb(10, x, 0)", 1, 11, "not a number"},
		{"hsl(10, 50%, 120%)", 1, 14, "out of range"},
		{"rgb(1, 2)", 1, 4, "takes 3 values"},
		{"rgb(1, 2, 3, 2)", 1, 14, "alpha"},
		{"rgb(1, 2, 3", 1, 12, "missing )"},
		{"blurple", 1, 1, "not a colour"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			_, err := runColor(t, tt.in)
			ie, ok := tool.AsInputError(err)
			if !ok {
				t.Fatalf("error = %v, want *InputError", err)
			}
			if ie.Line != tt.line || ie.Col != tt.col || !strings.Contains(ie.Msg, tt.msg) {
				t.Errorf("error = %d:%d %q, want %d:%d %q", ie.Line, ie.Col, ie.Msg, tt.line, tt.col, tt.msg)
			}
		})
	}
}

func TestColorDetect(t *testing.T) {
	tests := []struct {
		sample string
		want   float64
	}{
		{"#ff8800", 0.8},
		{"#fff", 0.8},
		{"rgb(1, 2, 3)", 0.8},
		{"hsla(1, 2%, 3%, 0.5)", 0.8},
		{"#ff88", 0},
		{"red", 0},
		{"ff8800", 0},
	}
	for _, tt := range tests {
		if got := (converters.Color{}).Detect(tt.sample); got != tt.want {
			t.Errorf("Detect(%q) = %v, want %v", tt.sample, got, tt.want)
		}
	}
}

func FuzzColor(f *testing.F) {
	for _, s := range []string{"#abc", "rgb(1,2,3)", "hsl(1 2% 3% / 50%)", "red", "rgb(1e400,0,0)"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_, err := runColor(t, s)
		if err != nil {
			if _, ok := tool.AsInputError(err); !ok {
				t.Fatalf("error that is not *InputError: %v", err)
			}
		}
	})
}
