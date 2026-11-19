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
		{"decimal a hex", "255", nil, "Hexadecimal", "0xff"},
		{"prefijo binario", "0b1010", nil, "Decimal", "10"},
		{"prefijo octal", "0o777", nil, "Decimal", "511"},
		{"hex mayusculas", "0XFF", nil, "Binario", "0b1111_1111"},
		{"base forzada", "ff", tool.Values{"from": "16"}, "Decimal", "255"},
		{"base forzada con su prefijo", "0x10", tool.Values{"from": "16"}, "Decimal", "16"},
		{"separadores", "1 000_000", nil, "Decimal", "1_000_000"},
		{"agrupado decimal", "1234567", nil, "Decimal", "1_234_567"},
		{"sin agrupar", "1234567", tool.Values{"group": false}, "Decimal", "1234567"},
		{"cero", "0", nil, "Binario", "0b0"},
		{"signo mas", "+8", nil, "Octal", "0o10"},
		{"negativo hex", "-0x10", nil, "Decimal", "-16"},
		{"mas de 64 bits", "18446744073709551616", tool.Values{"group": false}, "Hexadecimal", "0x10000000000000000"},
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
		{"255", "8", "cabe en int64"},
		{"18446744073709551615", "64", "cabe en uint64"},
		{"18446744073709551616", "65", "no cabe en 64 bits"},
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
		{"letra en decimal", "12a4", nil, 1, 3},
		{"dos en binario", "0b1012", nil, 1, 6},
		{"base forzada", "  789", tool.Values{"from": "8"}, 1, 4},
		{"prefijo solo", "0x", nil, 1, 2},
		{"signo solo", "-", nil, 1, 2},
		{"prefijo de otra base", "0x10", tool.Values{"from": "2"}, 1, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runNB(t, tt.in, tt.opts)
			ie, ok := tool.AsInputError(err)
			if !ok {
				t.Fatalf("error = %v, want *InputError", err)
			}
			if ie.Line != tt.line || ie.Col != tt.col {
				t.Errorf("posición = %d:%d, want %d:%d (%s)", ie.Line, ie.Col, tt.line, tt.col, ie.Msg)
			}
		})
	}
}

// FuzzNumberBaseRoundTrip: cada salida, releída en auto, da el mismo número.
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
		for _, label := range []string{"Binario", "Octal", "Decimal", "Hexadecimal"} {
			back, err := runNB(t, line(res.Text, label), nil)
			if err != nil {
				t.Fatalf("%s %q no se puede releer: %v", label, line(res.Text, label), err)
			}
			if got := strings.ReplaceAll(line(back.Text, "Decimal"), "_", ""); got != want.String() {
				t.Errorf("%s: %s → %s", label, want, got)
			}
		}
	})
}
