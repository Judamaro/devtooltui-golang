package encoders_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/encoders"
)

// 2026-01-01 12:00 UTC: the hs256 token was issued 12 h before and expires 12 h later.
var jwtNow = func() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) }

func runJWT(t *testing.T, in string, opts tool.Values) (tool.Result, error) {
	t.Helper()
	return encoders.JWT{Now: jwtNow}.Run(context.Background(), tool.Request{
		Inputs: map[string]string{"input": in},
		Opts:   opts,
	})
}

func TestJWTGolden(t *testing.T) {
	tests := []struct {
		name, in string
		opts     tool.Values
	}{
		{"hs256-no-secret", "hs256", nil},
		{"hs256-secret-ok", "hs256", tool.Values{"secret": "secreto"}},
		{"hs256-secret-bad", "hs256", tool.Values{"secret": "otro"}},
		{"rs256-bearer", "rs256", tool.Values{"secret": "x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runJWT(t, testutil.ReadInput(t, "jwt", tt.in), tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "jwt", tt.name, res.Text)
		})
	}
}

func TestJWTMeta(t *testing.T) {
	hs := testutil.ReadInput(t, "jwt", "hs256")
	tests := []struct {
		name  string
		now   time.Time
		in    string
		state string
	}{
		{"valid", jwtNow(), hs, "valid"},
		{"expired", time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC), hs, "expired"},
		{"not yet valid", jwtNow(), testutil.ReadInput(t, "jwt", "rs256"), "not yet valid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := encoders.JWT{Now: func() time.Time { return tt.now }}.Run(context.Background(),
				tool.Request{Inputs: map[string]string{"input": tt.in}})
			if err != nil {
				t.Fatal(err)
			}
			if got := res.Meta[len(res.Meta)-1].Value; got != tt.state {
				t.Errorf("Status = %q, want %q", got, tt.state)
			}
		})
	}
}

func TestJWTSignature(t *testing.T) {
	hs := testutil.ReadInput(t, "jwt", "hs256")
	tests := []struct {
		name   string
		in     string
		secret string
		want   string
	}{
		{"valid", hs, "secreto", "✓ valid HS256 signature"},
		{"invalid", hs, "nope", "✗ invalid signature"},
		{"no secret", hs, "", "not verified"},
		{"alg none", "eyJhbGciOiJub25lIn0.eyJzdWIiOiJ4In0.", "s", `alg "none"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runJWT(t, tt.in, tool.Values{"secret": tt.secret})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(res.Text, tt.want) {
				t.Errorf("the output does not contain %q:\n%s", tt.want, res.Text)
			}
		})
	}
}

func TestJWTErrors(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		line, col int
	}{
		{"two segments", "abc.def", 1, 1},
		{"jwe", "a.b.c.d.e", 1, 1},
		{"header not json", "aG9sYQ.e30.c2ln", 1, 1},
		{"invalid payload", "eyJhbGciOiJIUzI1NiJ9.@@@.c2ln", 1, 22},
		{"invalid signature", "eyJhbGciOiJIUzI1NiJ9.e30.!!", 1, 26},
		{"with bearer and spaces", "  Bearer abc", 1, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runJWT(t, tt.in, nil)
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

func TestJWTPayloadNoJSON(t *testing.T) {
	// payload "hola" (not JSON): shown as is, with no dates section.
	res, err := runJWT(t, "eyJhbGciOiJIUzI1NiJ9.aG9sYQ.c2ln", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Text, "hola") || strings.Contains(res.Text, "Dates") {
		t.Errorf("unexpected output:\n%s", res.Text)
	}
}

func TestJWTDetect(t *testing.T) {
	tests := []struct {
		name   string
		sample string
		want   float64
	}{
		{"token", testutil.ReadInput(t, "jwt", "hs256"), 0.98},
		{"with bearer", testutil.ReadInput(t, "jwt", "rs256"), 0.98},
		{"header without alg", "eyJ0eXAiOiJKV1QifQ.e30.c2ln", 0},
		{"no base64", "a.b.c", 0},
		{"two segments", "eyJhbGciOiJIUzI1NiJ9.e30", 0},
		{"empty", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (encoders.JWT{}).Detect(tt.sample); got != tt.want {
				t.Errorf("Detect = %v, want %v", got, tt.want)
			}
		})
	}
}

// FuzzJWT: no input panics and every error is an *InputError.
func FuzzJWT(f *testing.F) {
	f.Add("eyJhbGciOiJIUzI1NiJ9.e30.c2ln", "s")
	f.Add("Bearer a.b.c", "")
	f.Add("....", "x")
	f.Fuzz(func(t *testing.T, in, secret string) {
		_ = (encoders.JWT{}).Detect(in)
		_, err := runJWT(t, in, tool.Values{"secret": secret})
		if err != nil {
			if _, ok := tool.AsInputError(err); !ok {
				t.Fatalf("error that is not *InputError: %v", err)
			}
		}
	})
}

// signedWithPEM generates a key pair and returns a signed token and the PEM
// public key for each asymmetric algorithm.
func signedWithPEM(t *testing.T, alg string) (token, pubPEM string) {
	t.Helper()
	var priv, pub any
	switch alg[:2] {
	case "RS", "PS":
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
		priv, pub = k, &k.PublicKey
	case "ES":
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		priv, pub = k, &k.PublicKey
	default: // EdDSA
		p, k, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		priv, pub = k, p
	}
	tok, err := jwt.NewWithClaims(jwt.GetSigningMethod(alg), jwt.MapClaims{"sub": "x"}).SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return tok, string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func TestJWTPublicKeys(t *testing.T) {
	for _, alg := range []string{"RS256", "PS384", "ES256", "EdDSA"} {
		t.Run(alg, func(t *testing.T) {
			tok, pub := signedWithPEM(t, alg)
			_, otherPub := signedWithPEM(t, alg)
			tests := []struct {
				name, key, want string
			}{
				{"right key", pub, "✓ valid " + alg + " signature"},
				{"other key", otherPub, "✗ invalid signature for that public key"},
				{"no key", "", "paste it in the second input"},
				{"broken pem", "-----BEGIN PUBLIC KEY-----\nxx\n-----END PUBLIC KEY-----", "✗ the key is not a valid PEM public key"},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					res, err := encoders.JWT{Now: jwtNow}.Run(context.Background(), tool.Request{
						Inputs: map[string]string{"input": tok, "key": tt.key},
					})
					if err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(res.Text, tt.want) {
						t.Errorf("the output does not contain %q:\n%s", tt.want, res.Text)
					}
				})
			}
		})
	}
}

func TestJWTKeyTypeMismatch(t *testing.T) {
	tok, _ := signedWithPEM(t, "RS256")
	_, ecPub := signedWithPEM(t, "ES256")
	res, err := encoders.JWT{Now: jwtNow}.Run(context.Background(), tool.Request{
		Inputs: map[string]string{"input": tok, "key": ecPub},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Text, "✗ the key is not a valid PEM public key for RS256") {
		t.Errorf("does not report the wrong key type:\n%s", res.Text)
	}
}
