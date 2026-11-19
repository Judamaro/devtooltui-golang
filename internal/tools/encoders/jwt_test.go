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

// 2026-01-01 12:00 UTC: el token hs256 se emitió 12 h antes y expira 12 h después.
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
		{"hs256-sin-secreto", "hs256", nil},
		{"hs256-secreto-ok", "hs256", tool.Values{"secret": "secreto"}},
		{"hs256-secreto-malo", "hs256", tool.Values{"secret": "otro"}},
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
		{"vigente", jwtNow(), hs, "vigente"},
		{"expirado", time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC), hs, "expirado"},
		{"todavia no valido", jwtNow(), testutil.ReadInput(t, "jwt", "rs256"), "todavía no válido"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := encoders.JWT{Now: func() time.Time { return tt.now }}.Run(context.Background(),
				tool.Request{Inputs: map[string]string{"input": tt.in}})
			if err != nil {
				t.Fatal(err)
			}
			if got := res.Meta[len(res.Meta)-1].Value; got != tt.state {
				t.Errorf("Estado = %q, want %q", got, tt.state)
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
		{"valida", hs, "secreto", "✓ firma HS256 válida"},
		{"invalida", hs, "nope", "✗ firma inválida"},
		{"sin secreto", hs, "", "sin verificar"},
		{"alg none", "eyJhbGciOiJub25lIn0.eyJzdWIiOiJ4In0.", "s", `alg "none"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runJWT(t, tt.in, tool.Values{"secret": tt.secret})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(res.Text, tt.want) {
				t.Errorf("la salida no contiene %q:\n%s", tt.want, res.Text)
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
		{"dos segmentos", "abc.def", 1, 1},
		{"jwe", "a.b.c.d.e", 1, 1},
		{"header no json", "aG9sYQ.e30.c2ln", 1, 1},
		{"payload invalido", "eyJhbGciOiJIUzI1NiJ9.@@@.c2ln", 1, 22},
		{"firma invalida", "eyJhbGciOiJIUzI1NiJ9.e30.!!", 1, 26},
		{"con bearer y espacios", "  Bearer abc", 1, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runJWT(t, tt.in, nil)
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

func TestJWTPayloadNoJSON(t *testing.T) {
	// payload "hola" (no JSON): se muestra tal cual y no hay fechas.
	res, err := runJWT(t, "eyJhbGciOiJIUzI1NiJ9.aG9sYQ.c2ln", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Text, "hola") || strings.Contains(res.Text, "Fechas") {
		t.Errorf("salida inesperada:\n%s", res.Text)
	}
}

func TestJWTDetect(t *testing.T) {
	tests := []struct {
		name   string
		sample string
		want   float64
	}{
		{"token", testutil.ReadInput(t, "jwt", "hs256"), 0.98},
		{"con bearer", testutil.ReadInput(t, "jwt", "rs256"), 0.98},
		{"header sin alg", "eyJ0eXAiOiJKV1QifQ.e30.c2ln", 0},
		{"no base64", "a.b.c", 0},
		{"dos segmentos", "eyJhbGciOiJIUzI1NiJ9.e30", 0},
		{"vacio", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (encoders.JWT{}).Detect(tt.sample); got != tt.want {
				t.Errorf("Detect = %v, want %v", got, tt.want)
			}
		})
	}
}

// FuzzJWT: ninguna entrada provoca pánico y todo error es *InputError.
func FuzzJWT(f *testing.F) {
	f.Add("eyJhbGciOiJIUzI1NiJ9.e30.c2ln", "s")
	f.Add("Bearer a.b.c", "")
	f.Add("....", "x")
	f.Fuzz(func(t *testing.T, in, secret string) {
		_ = (encoders.JWT{}).Detect(in)
		_, err := runJWT(t, in, tool.Values{"secret": secret})
		if err != nil {
			if _, ok := tool.AsInputError(err); !ok {
				t.Fatalf("error que no es *InputError: %v", err)
			}
		}
	})
}

// keys genera pares de claves y devuelve un token firmado y la clave
// pública en PEM para cada algoritmo asimétrico.
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
				{"clave correcta", pub, "✓ firma " + alg + " válida"},
				{"otra clave", otherPub, "✗ firma inválida para esa clave pública"},
				{"sin clave", "", "pégala en la segunda entrada"},
				{"pem roto", "-----BEGIN PUBLIC KEY-----\nxx\n-----END PUBLIC KEY-----", "✗ la clave no es una clave pública PEM válida"},
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
						t.Errorf("la salida no contiene %q:\n%s", tt.want, res.Text)
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
	if !strings.Contains(res.Text, "✗ la clave no es una clave pública PEM válida para RS256") {
		t.Errorf("no avisa el tipo de clave equivocado:\n%s", res.Text)
	}
}
