package encoders_test

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"strings"
	"testing"
	"time"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/encoders"
)

func runX509(t *testing.T, in string, now time.Time) (tool.Result, error) {
	t.Helper()
	x := encoders.X509{Now: func() time.Time { return now }}
	return x.Run(context.Background(), tool.Request{Inputs: map[string]string{"input": in}})
}

// The fixture leaf is valid from 2026-09-01 to 2026-12-01.
var x509Now = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

func TestX509Golden(t *testing.T) {
	res, err := runX509(t, testutil.ReadInput(t, "x509", "chain"), x509Now)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Golden(t, "x509", "chain", res.Text)
	if res.Meta[0].Value != "2" || res.Meta[1].Value != "in 10 d" {
		t.Errorf("Meta = %v", res.Meta)
	}
}

func leafDER(t *testing.T) []byte {
	t.Helper()
	block, _ := pem.Decode([]byte(testutil.ReadInput(t, "x509", "chain")))
	return block.Bytes
}

func TestX509Formats(t *testing.T) {
	der := leafDER(t)
	b64 := base64.StdEncoding.EncodeToString(der)
	wrapped := b64[:64] + "\n" + b64[64:]
	for name, in := range map[string]string{
		"base64":     b64,
		"base64 url": base64.RawURLEncoding.EncodeToString(der),
		"wrapped":    wrapped,
		"hex":        hex.EncodeToString(der),
		"colon hex":  colonPairs(strings.ToUpper(hex.EncodeToString(der))),
	} {
		t.Run(name, func(t *testing.T) {
			res, err := runX509(t, in, x509Now)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(res.Text, "Subject:     CN=example.com,O=Example Ltd") {
				t.Errorf("got:\n%s", res.Text)
			}
		})
	}
}

func colonPairs(h string) string {
	var b strings.Builder
	for i := 0; i < len(h); i += 2 {
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(h[i : i+2])
	}
	return b.String()
}

func TestX509Validity(t *testing.T) {
	in := testutil.ReadInput(t, "x509", "chain")
	tests := []struct {
		now  time.Time
		want string
	}{
		{time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), "NOT YET VALID, starts in 31 d"},
		{time.Date(2026, 12, 5, 0, 0, 0, 0, time.UTC), "EXPIRED 4 d ago"},
		{x509Now, "valid, expires in 10 d"},
	}
	for _, tt := range tests {
		res, err := runX509(t, in, tt.now)
		if err != nil {
			t.Fatal(err)
		}
		if first := strings.SplitN(res.Text, "Certificate 2 of 2", 2)[0]; !strings.Contains(first, tt.want) {
			t.Errorf("at %s want %q in:\n%s", tt.now.Format(time.DateOnly), tt.want, first)
		}
	}
}

func TestX509Errors(t *testing.T) {
	chain := testutil.ReadInput(t, "x509", "chain")
	key := "-----BEGIN PRIVATE KEY-----\nMC4CAQAwBQYDK2VwBCIEIAEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEB\n-----END PRIVATE KEY-----\n"
	tests := []struct {
		name, in string
		line     int
		msg      string
	}{
		{"private key", "note\n" + key, 2, "a PRIVATE KEY block"},
		{"broken pem", "-----BEGIN CERTIFICATE-----\nnot base64!\n-----END CERTIFICATE-----\n", 1, "broken PEM block"},
		{"garbage", "hello world", 1, "not a certificate"},
		{"bad der", base64.StdEncoding.EncodeToString([]byte("not a certificate at all")), 1, "not a certificate: expected PEM"},
		{"bad cert in a chain", chain + "\n-----BEGIN CERTIFICATE-----\naGVsbG8=\n-----END CERTIFICATE-----\n", strings.Count(chain, "\n") + 2, "invalid certificate"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runX509(t, tt.in, x509Now)
			ie, ok := tool.AsInputError(err)
			if !ok {
				t.Fatalf("error = %v, want *InputError", err)
			}
			if ie.Line != tt.line || ie.Col != 1 || !strings.Contains(ie.Msg, tt.msg) {
				t.Errorf("error = %d:%d %q, want %d:1 %q", ie.Line, ie.Col, ie.Msg, tt.line, tt.msg)
			}
		})
	}
	if res, err := runX509(t, " \n", x509Now); err != nil || res.Text != "" {
		t.Errorf("empty input = %q, %v", res.Text, err)
	}
}

func TestX509Detect(t *testing.T) {
	if got := (encoders.X509{}).Detect(testutil.ReadInput(t, "x509", "chain")); got != 0.95 {
		t.Errorf("Detect(PEM) = %v", got)
	}
	if got := (encoders.X509{}).Detect("-----BEGIN PRIVATE KEY-----"); got != 0 {
		t.Errorf("Detect(key) = %v", got)
	}
}

// FuzzX509 checks that no input makes Run panic or return an error that is
// not an InputError.
func FuzzX509(f *testing.F) {
	f.Add("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n")
	f.Add("MIIBzTCCAX+gAwIBAgIGChssPU5f")
	f.Add("30:82:01")
	f.Fuzz(func(t *testing.T, in string) {
		_, err := runX509(t, in, x509Now)
		if err != nil {
			if _, ok := tool.AsInputError(err); !ok {
				t.Fatalf("error that is not *InputError: %v", err)
			}
		}
	})
}
