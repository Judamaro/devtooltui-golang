package converters_test

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/converters"
)

func runNB(t testing.TB, in string, opts tool.Values) (tool.Result, error) {
	t.Helper()
	return converters.NumberBase{}.Run(context.Background(), tool.Request{
		Inputs: map[string]string{"input": in},
		Opts:   opts,
	})
}

func TestNumberBaseGolden(t *testing.T) {
	tests := []struct {
		name, in string
		opts     tool.Values
	}{
		{"big-hex-grouped", "big-hex", nil},
		{"big-hex-plain", "big-hex", tool.Values{"group": false}},
		{"negative", "negative", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runNB(t, testutil.ReadInput(t, "number-base", tt.in), tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "number-base", tt.name, res.Text)
		})
	}
}

func TestNumberBase(t *testing.T) {
	tests := []struct {
		name, in string
		opts     tool.Values
		label    string
		want     string
	}{
		{"decimal to hex", "255", nil, "Hexadecimal", "0xff"},
		{"binary prefix", "0b1010", nil, "Decimal", "10"},
		{"octal prefix", "0o777", nil, "Decimal", "511"},
		{"uppercase hex", "0XFF", nil, "Binary", "0b1111_1111"},
		{"forced base", "ff", tool.Values{"from": "16"}, "Decimal", "255"},
		{"forced base with its prefix", "0x10", tool.Values{"from": "16"}, "Decimal", "16"},
		{"separators", "1 000_000", nil, "Decimal", "1_000_000"},
		{"grouped decimal", "1234567", nil, "Decimal", "1_234_567"},
		{"ungrouped", "1234567", tool.Values{"group": false}, "Decimal", "1234567"},
		{"zero", "0", nil, "Binary", "0b0"},
		{"plus sign", "+8", nil, "Octal", "0o10"},
		{"negative hex", "-0x10", nil, "Decimal", "-16"},
		{"over 64 bits", "18446744073709551616", tool.Values{"group": false}, "Hexadecimal", "0x10000000000000000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runNB(t, tt.in, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if got := line(res.Text, tt.label); got != tt.want {
				t.Errorf("%s = %q, want %q", tt.label, got, tt.want)
			}
		})
	}
}

func TestNumberBaseMeta(t *testing.T) {
	tests := []struct{ in, bits, rango string }{
		{"255", "8", "fits in int64"},
		{"18446744073709551615", "64", "fits in uint64"},
		{"18446744073709551616", "65", "does not fit in 64 bits"},
	}
	for _, tt := range tests {
		res, err := runNB(t, tt.in, nil)
		if err != nil {
			t.Fatal(err)
		}
		if res.Meta[1].Value != tt.bits || res.Meta[2].Value != tt.rango {
			t.Errorf("%s: Meta = %v", tt.in, res.Meta)
		}
	}
}

func TestNumberBaseErrors(t *testing.T) {
	tests := []struct {
		name, in  string
		opts      tool.Values
		line, col int
	}{
		{"letter in decimal", "12a4", nil, 1, 3},
		{"two in binary", "0b1012", nil, 1, 6},
		{"forced base", "  789", tool.Values{"from": "8"}, 1, 4},
		{"prefix only", "0x", nil, 1, 2},
		{"sign only", "-", nil, 1, 2},
		{"prefix of another base", "0x10", tool.Values{"from": "2"}, 1, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runNB(t, tt.in, tt.opts)
			ie, ok := tool.AsInputError(err)
			if !ok {
				t.Fatalf("error = %v, want *InputError", err)
			}
			if ie.Line != tt.line || ie.Col != tt.col {
				t.Errorf("position = %d:%d, want %d:%d (%s)", ie.Line, ie.Col, tt.line, tt.col, ie.Msg)
			}
		})
	}
}

// FuzzNumberBaseRoundTrip: every output, read back in auto, gives the same number.
func FuzzNumberBaseRoundTrip(f *testing.F) {
	for _, s := range []string{"0", "-1", "255", "18446744073709551616"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, dec string) {
		want, ok := new(big.Int).SetString(dec, 10)
		if !ok {
			return
		}
		res, err := runNB(t, want.String(), nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, label := range []string{"Binary", "Octal", "Decimal", "Hexadecimal"} {
			back, err := runNB(t, line(res.Text, label), nil)
			if err != nil {
				t.Fatalf("%s %q cannot be read back: %v", label, line(res.Text, label), err)
			}
			if got := strings.ReplaceAll(line(back.Text, "Decimal"), "_", ""); got != want.String() {
				t.Errorf("%s: %s → %s", label, want, got)
			}
		}
	})
}
