package encoders_test

import (
	"context"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/encoders"
)

func runBase64(t *testing.T, in string, opts tool.Values) (tool.Result, error) {
	t.Helper()
	return encoders.Base64{}.Run(context.Background(), tool.Request{
		Inputs: map[string]string{"input": in},
		Opts:   opts,
	})
}

func TestBase64Golden(t *testing.T) {
	tests := []struct {
		name string
		opts tool.Values
	}{
		{"encode-text", nil},
		{"encode-utf8-url-nopad", tool.Values{"variant": "url", "padding": false}},
		{"decode-wrapped", tool.Values{"mode": "decode"}},
		{"decode-binary", tool.Values{"mode": "decode"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runBase64(t, testutil.ReadInput(t, "base64", tt.name), tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "base64", tt.name, res.Text)
		})
	}
}

func TestBase64(t *testing.T) {
	tests := []struct {
		name string
		in   string
		opts tool.Values
		want string
	}{
		{"encode empty", "", nil, ""},
		{"encode hola", "hola", nil, "aG9sYQ=="},
		{"encode without padding", "hola", tool.Values{"padding": false}, "aG9sYQ"},
		{"encode url", "\xfb\xff", tool.Values{"variant": "url"}, "-_8="},
		{"encode standard", "\xfb\xff", nil, "+/8="},
		{"decode with padding", "aG9sYQ==", tool.Values{"mode": "decode"}, "hola"},
		{"decode without padding", "aG9sYQ", tool.Values{"mode": "decode"}, "hola"},
		{"decode with spaces", " aG9s\nYQ== \n", tool.Values{"mode": "decode"}, "hola"},
		{"decode url as text", "-_8", tool.Values{"mode": "decode", "variant": "url", "output": "text"}, "\xfb\xff"},
		{"decode binary auto to hex", "-_8", tool.Values{"mode": "decode", "variant": "url"}, "fbff"},
		{"decode with NUL auto to hex", "AGhvbGE=", tool.Values{"mode": "decode"}, "00686f6c61"},
		{"decode text forced to hex", "aG9sYQ==", tool.Values{"mode": "decode", "output": "hex"}, "686f6c61"},
		{"decode text auto", "aG9sYQ==", tool.Values{"mode": "decode", "output": "auto"}, "hola"},
		{"encode ignores output", "hola", tool.Values{"output": "hex"}, "aG9sYQ=="},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runBase64(t, tt.in, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if res.Text != tt.want {
				t.Errorf("Text = %q, want %q", res.Text, tt.want)
			}
		})
	}
}

func TestBase64Errors(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		opts      tool.Values
		line, col int
	}{
		{"invalid character", "aG9s*Q==", tool.Values{"mode": "decode"}, 1, 5},
		{"second line", "aG9s\nY!==", tool.Values{"mode": "decode"}, 2, 2},
		{"url in standard alphabet", "-_8", tool.Values{"mode": "decode"}, 1, 1},
		{"wrong length", "aG9sY", tool.Values{"mode": "decode"}, 1, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runBase64(t, tt.in, tt.opts)
			ie, ok := tool.AsInputError(err)
			if !ok {
				t.Fatalf("error = %v, want *InputError", err)
			}
			if ie.Line != tt.line || ie.Col != tt.col {
				t.Errorf("position = %d:%d, want %d:%d", ie.Line, ie.Col, tt.line, tt.col)
			}
		})
	}
}

func TestBase64Detect(t *testing.T) {
	tests := []struct {
		name   string
		sample string
		want   float64
	}{
		{"encoded text", "aG9sYSBtdW5kbw==", 0.6},
		{"url-safe", "aG9sYT8_Pz8-", 0.6},
		{"too short", "aG9s", 0},
		{"not a multiple of 4", "aG9sYSBtdW5kbw=", 0},
		{"binary", "+/8AAQIDBAU=", 0},
		{"plain text", "hello world!", 0},
		{"empty", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (encoders.Base64{}).Detect(tt.sample); got != tt.want {
				t.Errorf("Detect(%q) = %v, want %v", tt.sample, got, tt.want)
			}
		})
	}
}

func FuzzBase64RoundTrip(f *testing.F) {
	for _, s := range []string{"", "hola", "\x00\xff", "ñandú 🚀"} {
		f.Add(s, false, true)
	}
	f.Fuzz(func(t *testing.T, s string, urlSafe, padding bool) {
		variant := "standard"
		if urlSafe {
			variant = "url"
		}
		enc, err := runBase64(t, s, tool.Values{"variant": variant, "padding": padding})
		if err != nil {
			t.Fatal(err)
		}
		dec, err := runBase64(t, enc.Text, tool.Values{"mode": "decode", "variant": variant, "output": "text"})
		if err != nil {
			t.Fatal(err)
		}
		if dec.Text != s {
			t.Errorf("round trip: %q → %q → %q", s, enc.Text, dec.Text)
		}
	})
}
