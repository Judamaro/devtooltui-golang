package encoders_test

import (
	"context"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/encoders"
)

func runURL(t *testing.T, in string, opts tool.Values) (tool.Result, error) {
	t.Helper()
	return encoders.URL{}.Run(context.Background(), tool.Request{
		Inputs: map[string]string{"input": in},
		Opts:   opts,
	})
}

func TestURLGolden(t *testing.T) {
	tests := []struct {
		name, in string
		opts     tool.Values
	}{
		{"encode-query", "text", nil},
		{"encode-path", "text", tool.Values{"escape": "path"}},
		{"decode-query", "query-encoded", tool.Values{"mode": "decode"}},
		{"parse-full", "full", tool.Values{"mode": "parse"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runURL(t, testutil.ReadInput(t, "url", tt.in), tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "url", tt.name, res.Text)
		})
	}
}

func TestURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		opts tool.Values
		want string
	}{
		{"encode empty", "", nil, ""},
		{"encode query space", "a b", nil, "a+b"},
		{"encode path space", "a b", tool.Values{"escape": "path"}, "a%20b"},
		{"encode path slash", "a/b", tool.Values{"escape": "path"}, "a%2Fb"},
		{"decode query plus", "a+b%21", tool.Values{"mode": "decode"}, "a b!"},
		{"decode path literal plus", "a+b%20c", tool.Values{"mode": "decode", "escape": "path"}, "a+b c"},
		{"decode lowercase", "%c3%b1", tool.Values{"mode": "decode"}, "ñ"},
		{"parse empty", "  ", tool.Values{"mode": "parse"}, ""},
		{"parse without scheme", "example.com/a", tool.Values{"mode": "parse"}, "Path:       example.com/a"},
		{"parse opaque", "mailto:ana@example.com", tool.Values{"mode": "parse"}, "Scheme:     mailto\nOpaque:     ana@example.com"},
		{"parse ipv6", "http://[::1]:80/", tool.Values{"mode": "parse"}, "Scheme:     http\nHost:       ::1\nPort:       80\nPath:       /"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runURL(t, tt.in, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if res.Text != tt.want {
				t.Errorf("Text = %q, want %q", res.Text, tt.want)
			}
		})
	}
}

func TestURLErrors(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		opts      tool.Values
		line, col int
	}{
		{"non-hex escape", "abc%zz", tool.Values{"mode": "decode"}, 1, 4},
		{"truncated escape", "abc%4", tool.Values{"mode": "decode"}, 1, 4},
		{"second line", "ok\n%G1", tool.Values{"mode": "decode", "escape": "path"}, 2, 1},
		{"parse invalid host", "http://[::1", tool.Values{"mode": "parse"}, 1, 0},
		{"parse several lines", "http://a\nhttp://b", tool.Values{"mode": "parse"}, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runURL(t, tt.in, tt.opts)
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

func TestURLDetect(t *testing.T) {
	tests := []struct {
		name   string
		sample string
		want   float64
	}{
		{"encoded query", "q%3Dhola%26x%3D1", 0.55},
		{"url with escapes", "https://e.com/a%20b", 0.55},
		{"with spaces", "a%20b c", 0},
		{"no escapes", "https://e.com/", 0},
		{"invalid escape", "100%zz", 0},
		{"empty", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (encoders.URL{}).Detect(tt.sample); got != tt.want {
				t.Errorf("Detect = %v, want %v", got, tt.want)
			}
		})
	}
}

func FuzzURLRoundTrip(f *testing.F) {
	for _, s := range []string{"", "a b&c=d", "ñ/%+", "\x00\xff"} {
		f.Add(s, true)
	}
	f.Fuzz(func(t *testing.T, s string, query bool) {
		escape := "path"
		if query {
			escape = "query"
		}
		enc, err := runURL(t, s, tool.Values{"escape": escape})
		if err != nil {
			t.Fatal(err)
		}
		dec, err := runURL(t, enc.Text, tool.Values{"mode": "decode", "escape": escape})
		if err != nil {
			t.Fatal(err)
		}
		if dec.Text != s {
			t.Errorf("round trip: %q → %q → %q", s, enc.Text, dec.Text)
		}
	})
}
