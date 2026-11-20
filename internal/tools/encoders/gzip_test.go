package encoders_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/encoders"
)

func runGzip(t testing.TB, in string, opts tool.Values) (tool.Result, error) {
	t.Helper()
	return encoders.Gzip{}.Run(context.Background(), tool.Request{
		Inputs: map[string]string{"input": in},
		Opts:   opts,
	})
}

func gz(t testing.TB, data []byte) string {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestGzipGolden(t *testing.T) {
	in := testutil.ReadInput(t, "gzip", "encode-text")
	for _, tt := range []struct {
		name string
		opts tool.Values
	}{
		{"encode-text", nil},
		{"encode-text-level1", tool.Values{"level": 1}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runGzip(t, in, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "gzip", tt.name, res.Text)
			dec, err := runGzip(t, res.Text, tool.Values{"mode": "decode"})
			if err != nil || dec.Text != in {
				t.Errorf("decode = %q, %v; want the input back", dec.Text, err)
			}
		})
	}
}

func TestGzip(t *testing.T) {
	hello := gz(t, []byte("hello"))
	tests := []struct {
		name string
		in   string
		opts tool.Values
		want string
	}{
		{"decode", hello, tool.Values{"mode": "decode"}, "hello"},
		{"decode wrapped", hello[:10] + "\n" + hello[10:] + "\n", tool.Values{"mode": "decode"}, "hello"},
		{"decode url-safe", strings.NewReplacer("+", "-", "/", "_").Replace(gz(t, []byte{0xfb, 0xff, 0xfe})), tool.Values{"mode": "decode"}, "fbfffe"},
		{"decode binary as hex", gz(t, []byte{0, 1, 2}), tool.Values{"mode": "decode"}, "000102"},
		{"decode forced hex", hello, tool.Values{"mode": "decode", "output": "hex"}, "68656c6c6f"},
		{"decode empty", "  \n", tool.Values{"mode": "decode"}, ""},
		{"decode empty payload", gz(t, nil), tool.Values{"mode": "decode"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runGzip(t, tt.in, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if res.Text != tt.want {
				t.Errorf("Text = %q, want %q", res.Text, tt.want)
			}
		})
	}
}

func TestGzipMeta(t *testing.T) {
	res, err := runGzip(t, strings.Repeat("a", 1000), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, kv := range res.Meta {
		got[kv.Key] = kv.Value
	}
	if got["Original"] != "1000 B" || !strings.HasSuffix(got["Ratio"], " %") {
		t.Errorf("Meta = %v", res.Meta)
	}
	if empty, _ := runGzip(t, "", nil); empty.Meta[2].Value != "–" {
		t.Errorf("empty Ratio = %q, want –", empty.Meta[2].Value)
	}
}

func TestGzipErrors(t *testing.T) {
	hello := gz(t, []byte("hello"))
	raw, _ := base64.StdEncoding.DecodeString(hello)
	truncated := base64.StdEncoding.EncodeToString(raw[:len(raw)-6])
	tests := []struct {
		name      string
		in        string
		line, col int
		msg       string
	}{
		{"bad base64", "H4sI\nAA*A", 2, 3, "invalid Base64"},
		{"not gzip", base64.StdEncoding.EncodeToString([]byte("plain text")), 1, 1, "not GZip data"},
		{"truncated", truncated, 1, 1, "corrupt GZip data"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runGzip(t, tt.in, tool.Values{"mode": "decode"})
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

func TestGzipBomb(t *testing.T) {
	bomb := gz(t, make([]byte, encoders.GzipMaxOutput+1))
	_, err := runGzip(t, bomb, tool.Values{"mode": "decode"})
	if ie, ok := tool.AsInputError(err); !ok || !strings.Contains(ie.Msg, "exceeds") {
		t.Errorf("error = %v, want the size limit", err)
	}
}

// cancelAfter reports cancellation from the (n+1)th call to Err on.
type cancelAfter struct {
	context.Context
	n int
}

func (c *cancelAfter) Err() error {
	c.n--
	if c.n < 0 {
		return context.Canceled
	}
	return nil
}

func TestGzipCancelledWhileDecoding(t *testing.T) {
	// The check at the start of Run passes; the decoding loop must see it.
	ctx := &cancelAfter{Context: context.Background(), n: 1}
	req := tool.Request{Inputs: map[string]string{"input": gz(t, []byte("hello"))}, Opts: tool.Values{"mode": "decode"}}
	if _, err := (encoders.Gzip{}).Run(ctx, req); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestGzipDetect(t *testing.T) {
	tests := []struct {
		name   string
		sample string
		want   float64
	}{
		{"gzip", gz(t, []byte("hello world")), 0.85},
		{"cut sample", gz(t, []byte("hello world"))[:30], 0.85},
		{"plain base64", "aG9sYSBtdW5kbw==", 0},
		{"too short", "H4sIAA==", 0},
		{"empty", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (encoders.Gzip{}).Detect(tt.sample); got != tt.want {
				t.Errorf("Detect(%q) = %v, want %v", tt.sample, got, tt.want)
			}
		})
	}
}

func FuzzGzipRoundTrip(f *testing.F) {
	for _, s := range []string{"", "hello", "\x00\xff", "ñandú 🚀"} {
		f.Add(s, 6)
	}
	f.Fuzz(func(t *testing.T, s string, level int) {
		enc, err := runGzip(t, s, tool.Values{"level": ((level%9)+9)%9 + 1})
		if err != nil {
			t.Fatal(err)
		}
		dec, err := runGzip(t, enc.Text, tool.Values{"mode": "decode", "output": "text"})
		if err != nil {
			t.Fatal(err)
		}
		if dec.Text != s {
			t.Errorf("round trip: %q → %q → %q", s, enc.Text, dec.Text)
		}
	})
}
