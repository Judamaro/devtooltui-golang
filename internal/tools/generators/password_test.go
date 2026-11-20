package generators_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/generators"
)

func runPassword(t *testing.T, opts tool.Values) (tool.Result, error) {
	t.Helper()
	p := generators.Password{Rand: rand.NewChaCha8([32]byte{4, 5, 6})}
	return p.Run(context.Background(), tool.Request{Opts: opts})
}

func TestPasswordGolden(t *testing.T) {
	tests := []struct {
		name string
		opts tool.Values
	}{
		{"default-x5", tool.Values{"count": 5}},
		{"pin-digits-x3", tool.Values{"count": 3, "length": 6, "lower": false, "upper": false, "symbols": false}},
		{"no-ambiguous-x3", tool.Values{"count": 3, "length": 32, "exclude_ambiguous": true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runPassword(t, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "password", tt.name, res.Text)
		})
	}
}

func TestPassword(t *testing.T) {
	tests := []struct {
		name     string
		opts     tool.Values
		length   int
		count    int
		allowed  string
		required []string // each set must appear at least once
		entropy  string
	}{
		{"defaults", nil, 20, 1, "", []string{"abcdefghijklmnopqrstuvwxyz", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "0123456789"}, "130 bits (very strong)"},
		{"digits only", tool.Values{"lower": false, "upper": false, "symbols": false, "length": 4}, 4, 1, "0123456789", nil, "13 bits (weak)"},
		{"lower only, count", tool.Values{"upper": false, "digits": false, "symbols": false, "count": 7, "length": 12}, 12, 7, "abcdefghijklmnopqrstuvwxyz", nil, "56 bits (fair)"},
		{"no ambiguous", tool.Values{"exclude_ambiguous": true, "length": 256}, 256, 1, "", nil, ""},
		{"length clamped", tool.Values{"length": 1000}, 256, 1, "", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runPassword(t, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(res.Text, "\n")
			if len(lines) != tt.count {
				t.Fatalf("%d passwords, want %d", len(lines), tt.count)
			}
			for _, pw := range lines {
				if len(pw) != tt.length {
					t.Errorf("length %d, want %d: %q", len(pw), tt.length, pw)
				}
				if tt.allowed != "" && strings.Trim(pw, tt.allowed) != "" {
					t.Errorf("%q has characters outside %q", pw, tt.allowed)
				}
				for _, set := range tt.required {
					if !strings.ContainsAny(pw, set) {
						t.Errorf("%q has nothing from %q", pw, set)
					}
				}
				if tt.opts["exclude_ambiguous"] == true && strings.ContainsAny(pw, "0Oo1lI|") {
					t.Errorf("%q has ambiguous characters", pw)
				}
				if strings.ContainsAny(pw, "\"'`\\ ") {
					t.Errorf("%q has quotes, backslash or space", pw)
				}
			}
			if tt.entropy != "" && res.Meta[0].Value != tt.entropy {
				t.Errorf("Entropy = %q, want %q", res.Meta[0].Value, tt.entropy)
			}
		})
	}
}

func TestPasswordEverySetAppears(t *testing.T) {
	// With length 4 and 4 sets, each set fills exactly one character.
	for range 200 {
		res, err := runPassword(t, tool.Values{"length": 4, "count": 100})
		if err != nil {
			t.Fatal(err)
		}
		for _, pw := range strings.Split(res.Text, "\n") {
			for _, set := range []string{"abcdefghijklmnopqrstuvwxyz", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "0123456789", "!#$%&()*+,-./:;<=>?@[]^_{|}~"} {
				if !strings.ContainsAny(pw, set) {
					t.Fatalf("%q has nothing from %q", pw, set)
				}
			}
		}
	}
}

func TestPasswordNoSets(t *testing.T) {
	_, err := runPassword(t, tool.Values{"lower": false, "upper": false, "digits": false, "symbols": false})
	if _, ok := tool.AsInputError(err); !ok {
		t.Errorf("error = %v, want *InputError", err)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("no entropy") }

func TestPasswordRandError(t *testing.T) {
	_, err := generators.Password{Rand: failingReader{}}.Run(context.Background(), tool.Request{})
	if err == nil {
		t.Error("expected the reader error")
	}
}
