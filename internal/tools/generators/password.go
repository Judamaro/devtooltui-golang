package generators

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math"
	"math/big"
	"strings"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Password generates random passwords. Rand pins the randomness in tests;
// when nil, crypto/rand is used.
type Password struct {
	Rand io.Reader
}

// Character sets. Symbols leave out quotes, backslash, backtick and space,
// so passwords paste safely into shells and config files.
const (
	lowerSet  = "abcdefghijklmnopqrstuvwxyz"
	upperSet  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitSet  = "0123456789"
	symbolSet = "!#$%&()*+,-./:;<=>?@[]^_{|}~"
	ambiguous = "0Oo1lI|"
)

func (Password) Spec() tool.Spec {
	return tool.Spec{
		ID:          "password",
		Name:        "Password",
		Category:    tool.Generators,
		Description: "Generate random passwords and show their entropy",
		Keywords:    []string{"passphrase", "random", "secret", "pwgen"},
		Options: []tool.Option{
			{Key: "length", Label: "Length", Kind: tool.Number, Min: 4, Max: 256, Default: 20},
			{Key: "count", Label: "Count", Kind: tool.Number, Min: 1, Max: 100, Default: 1},
			{Key: "lower", Label: "a-z", Kind: tool.Toggle, Default: true},
			{Key: "upper", Label: "A-Z", Kind: tool.Toggle, Default: true},
			{Key: "digits", Label: "0-9", Kind: tool.Toggle, Default: true},
			{Key: "symbols", Label: "Symbols", Kind: tool.Toggle, Default: true},
			{Key: "exclude_ambiguous", Label: "No ambiguous", Help: "Leave out " + ambiguous, Kind: tool.Toggle, Default: false},
		},
		Output: tool.PlainText,
		Live:   true,
	}
}

func (p Password) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(p.Spec())
	var sets []string
	for _, s := range []struct{ key, chars string }{
		{"lower", lowerSet}, {"upper", upperSet}, {"digits", digitSet}, {"symbols", symbolSet},
	} {
		if !opts.Bool(s.key) {
			continue
		}
		chars := s.chars
		if opts.Bool("exclude_ambiguous") {
			chars = strings.Map(func(r rune) rune {
				if strings.ContainsRune(ambiguous, r) {
					return -1
				}
				return r
			}, chars)
		}
		sets = append(sets, chars)
	}
	if len(sets) == 0 {
		return tool.Result{}, &tool.InputError{Input: "lower", Line: 1, Col: 1, Msg: "enable at least one character set"}
	}
	pool := strings.Join(sets, "")

	r := p.Rand
	if r == nil {
		r = rand.Reader
	}
	length := min(max(opts.Int("length"), 4), 256)
	count := min(max(opts.Int("count"), 1), 100)
	out := make([]string, 0, count)
	for range count {
		if err := ctx.Err(); err != nil {
			return tool.Result{}, err
		}
		pw, err := password(r, sets, pool, length)
		if err != nil {
			return tool.Result{}, err
		}
		out = append(out, pw)
	}
	bits := float64(length) * math.Log2(float64(len(pool)))
	return tool.Result{
		Text: strings.Join(out, "\n"),
		Meta: []tool.KV{
			{Key: "Entropy", Value: fmt.Sprintf("%.0f bits (%s)", bits, strength(bits))},
			{Key: "Pool", Value: fmt.Sprintf("%d characters", len(pool))},
		},
	}, nil
}

// password takes one character from each set, so every enabled set appears,
// fills the rest from the whole pool and shuffles.
func password(r io.Reader, sets []string, pool string, length int) (string, error) {
	b := make([]byte, 0, length)
	for _, s := range sets {
		c, err := pick(r, s)
		if err != nil {
			return "", err
		}
		b = append(b, c)
	}
	for len(b) < length {
		c, err := pick(r, pool)
		if err != nil {
			return "", err
		}
		b = append(b, c)
	}
	for i := len(b) - 1; i > 0; i-- { // Fisher–Yates
		j, err := randInt(r, i+1)
		if err != nil {
			return "", err
		}
		b[i], b[j] = b[j], b[i]
	}
	return string(b), nil
}

func pick(r io.Reader, chars string) (byte, error) {
	i, err := randInt(r, len(chars))
	if err != nil {
		return 0, err
	}
	return chars[i], nil
}

// randInt returns a uniform int in [0, n) without modulo bias.
func randInt(r io.Reader, n int) (int, error) {
	v, err := rand.Int(r, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}

func strength(bits float64) string {
	switch {
	case bits < 50:
		return "weak"
	case bits < 80:
		return "fair"
	case bits < 110:
		return "strong"
	default:
		return "very strong"
	}
}
