package encoders

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/converters"
)

// JWT decodes a token and verifies HMAC signatures with a secret and
// asymmetric ones with a PEM key. Now pins the clock in tests; nil uses time.Now.
type JWT struct {
	Now func() time.Time
}

func (JWT) Spec() tool.Spec {
	return tool.Spec{
		ID:          "jwt",
		Name:        "JWT",
		Category:    tool.Encoders,
		Description: "Decode JWTs and verify signatures (HMAC, RSA, ECDSA, EdDSA)",
		Keywords:    []string{"token", "bearer", "jws", "auth", "claims"},
		Inputs: []tool.InputSpec{
			{Key: "input", Label: "Token", Multiline: true},
			{Key: "key", Label: "Public key (PEM, optional)", Multiline: true},
		},
		Options: []tool.Option{
			{Key: "secret", Label: "Secret", Help: "Verifies HS256/384/512 signatures", Kind: tool.String, Default: ""},
		},
		Output: tool.PlainText,
		Live:   true,
	}
}

var hmacAlgs = []string{"HS256", "HS384", "HS512"}

func (j JWT) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(j.Spec())
	in := req.Inputs["input"]
	token, offset := cleanToken(in)
	if token == "" {
		return tool.Result{}, nil
	}

	parts := strings.Split(token, ".")
	switch {
	case len(parts) == 5:
		return tool.Result{}, j.errAt(in, offset, "this is a JWE (encrypted): it cannot be decoded without the key")
	case len(parts) != 3:
		return tool.Result{}, j.errAt(in, offset, fmt.Sprintf("expected 3 segments separated by '.', got %d", len(parts)))
	}
	segStart := func(i int) int { return offset + len(strings.Join(parts[:i], ".")) + min(i, 1) }

	header, err := decodeSegment(parts[0])
	var hdr map[string]any
	if err != nil || json.Unmarshal(header, &hdr) != nil {
		return tool.Result{}, j.errAt(in, segStart(0), "the header is not Base64URL holding a JSON object")
	}
	payload, err := decodeSegment(parts[1])
	if err != nil {
		return tool.Result{}, j.errAt(in, segStart(1), "the payload is not valid Base64URL")
	}
	if _, err := decodeSegment(parts[2]); err != nil {
		return tool.Result{}, j.errAt(in, segStart(2), "the signature is not valid Base64URL")
	}

	var b strings.Builder
	b.WriteString("── Header\n")
	b.WriteString(prettyJSON(header))
	b.WriteString("\n\n── Payload\n")
	b.WriteString(prettyJSON(payload))

	now := time.Now
	if j.Now != nil {
		now = j.Now
	}
	var claims map[string]any
	_ = json.Unmarshal(payload, &claims) // a non-JSON payload has no claims
	claimLines, state := timeClaims(claims, now())
	if len(claimLines) > 0 {
		b.WriteString("\n\n── Dates\n")
		b.WriteString(strings.Join(claimLines, "\n"))
	}

	alg, _ := hdr["alg"].(string)
	sig := verifySignature(token, alg, opts.String("secret"), req.Inputs["key"])
	b.WriteString("\n\n── Signature\n" + sig)

	meta := []tool.KV{{Key: "alg", Value: alg}}
	if state != "" {
		meta = append(meta, tool.KV{Key: "Status", Value: state})
	}
	return tool.Result{Text: b.String(), Meta: meta}, nil
}

func (JWT) errAt(in string, offset int, msg string) error {
	line, col := tool.Position(in, offset)
	return &tool.InputError{Input: "input", Line: line, Col: col, Msg: msg}
}

// cleanToken strips spaces and a "Bearer " prefix and returns the offset
// where the token starts in the original input.
func cleanToken(in string) (string, int) {
	trimmed := strings.TrimLeft(in, " \t\r\n")
	offset := len(in) - len(trimmed)
	if len(trimmed) > 7 && strings.EqualFold(trimmed[:7], "bearer ") {
		trimmed = trimmed[7:]
		offset += 7
	}
	return strings.TrimSpace(trimmed), offset
}

func decodeSegment(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
}

func prettyJSON(b []byte) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, b, "", "  "); err != nil {
		return string(b) // non-JSON payload: shown as is
	}
	return buf.String()
}

// timeClaims shows exp, nbf and iat as UTC and relative dates.
func timeClaims(claims map[string]any, now time.Time) (lines []string, state string) {
	for _, name := range []string{"exp", "nbf", "iat"} {
		n, ok := claims[name].(float64)
		if !ok {
			continue
		}
		t := time.Unix(int64(n), 0).UTC()
		lines = append(lines, fmt.Sprintf("%s: %s (%s)", name, t.Format("2006-01-02 15:04:05 UTC"), converters.Relative(t.Sub(now))))
		switch {
		case name == "exp" && !now.Before(t):
			state = "expired"
		case name == "nbf" && now.Before(t) && state == "":
			state = "not yet valid"
		}
	}
	if state == "" && len(lines) > 0 {
		state = "valid"
	}
	return lines, state
}

// Asymmetric algorithms by key family.
var (
	rsaAlgs   = []string{"RS256", "RS384", "RS512", "PS256", "PS384", "PS512"}
	ecdsaAlgs = []string{"ES256", "ES384", "ES512"}
	edAlgs    = []string{"EdDSA"}
)

func verifySignature(token, alg, secret, pem string) string {
	var key any
	var valid []string
	switch {
	case strings.EqualFold(alg, "none"):
		return "⚠ alg \"none\": the token is not signed"
	case slices.Contains(hmacAlgs, alg):
		if secret == "" {
			return "not verified (type the secret in the options)"
		}
		key, valid = []byte(secret), hmacAlgs
	case strings.TrimSpace(pem) == "":
		return fmt.Sprintf("not verified: %s needs the public key (paste it in the second input)", alg)
	default:
		var err error
		key, valid, err = parsePublicKey(alg, []byte(pem))
		if err != nil {
			return "✗ " + err.Error()
		}
	}
	_, err := jwt.Parse(token, func(*jwt.Token) (any, error) { return key, nil },
		jwt.WithValidMethods(valid), jwt.WithoutClaimsValidation())
	switch {
	case err == nil:
		return fmt.Sprintf("✓ valid %s signature", alg)
	case errors.Is(err, jwt.ErrSignatureInvalid), errors.Is(err, jwt.ErrTokenSignatureInvalid):
		if slices.Contains(hmacAlgs, alg) {
			return "✗ invalid signature for that secret"
		}
		return "✗ invalid signature for that public key"
	default:
		return "✗ " + err.Error()
	}
}

// parsePublicKey reads a PEM public key or certificate of the type alg
// requires.
func parsePublicKey(alg string, pem []byte) (any, []string, error) {
	var (
		key any
		err error
	)
	switch {
	case slices.Contains(rsaAlgs, alg):
		key, err = jwt.ParseRSAPublicKeyFromPEM(pem)
		if err == nil {
			return key, rsaAlgs, nil
		}
	case slices.Contains(ecdsaAlgs, alg):
		key, err = jwt.ParseECPublicKeyFromPEM(pem)
		if err == nil {
			return key, ecdsaAlgs, nil
		}
	case slices.Contains(edAlgs, alg):
		key, err = jwt.ParseEdPublicKeyFromPEM(pem)
		if err == nil {
			return key, edAlgs, nil
		}
	default:
		return nil, nil, fmt.Errorf("unsupported algorithm %q", alg)
	}
	return nil, nil, fmt.Errorf("the key is not a valid PEM public key for %s: %v", alg, err)
}

// Detect recognises three Base64URL segments whose header is JSON with "alg".
func (JWT) Detect(sample string) float64 {
	token, _ := cleanToken(sample)
	parts := strings.Split(token, ".")
	if len(parts) != 3 || strings.ContainsAny(token, " \n") {
		return 0
	}
	header, err := decodeSegment(parts[0])
	if err != nil {
		return 0
	}
	var hdr map[string]any
	if json.Unmarshal(header, &hdr) != nil {
		return 0
	}
	if _, ok := hdr["alg"].(string); !ok {
		return 0
	}
	return 0.98
}
