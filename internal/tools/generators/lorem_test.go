package generators_test

import (
	"context"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/generators"
)

func runLorem(t *testing.T, opts tool.Values) tool.Result {
	t.Helper()
	l := generators.Lorem{Rand: rand.NewChaCha8([32]byte{7, 8, 9})}
	res, err := l.Run(context.Background(), tool.Request{Opts: opts})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestLoremGolden(t *testing.T) {
	tests := []struct {
		name string
		opts tool.Values
	}{
		{"paragraphs-x3", nil},
		{"sentences-x4-random", tool.Values{"unit": "sentences", "count": 4, "classic": false}},
		{"words-x12", tool.Values{"unit": "words", "count": 12}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.Golden(t, "lorem", tt.name, runLorem(t, tt.opts).Text)
		})
	}
}

func TestLorem(t *testing.T) {
	tests := []struct {
		name   string
		opts   tool.Values
		prefix string
		check  func(t *testing.T, out string)
	}{
		{"one word", tool.Values{"unit": "words", "count": 1}, "Lorem", func(t *testing.T, out string) {
			if out != "Lorem" {
				t.Errorf("got %q", out)
			}
		}},
		{"words count", tool.Values{"unit": "words", "count": 40}, "Lorem ipsum dolor sit amet consectetur adipiscing elit ", func(t *testing.T, out string) {
			if n := len(strings.Fields(out)); n != 40 {
				t.Errorf("%d words, want 40", n)
			}
		}},
		{"random words", tool.Values{"unit": "words", "count": 5, "classic": false}, "", func(t *testing.T, out string) {
			if strings.HasPrefix(out, "Lorem ipsum") {
				t.Errorf("classic opening without classic: %q", out)
			}
		}},
		{"sentences", tool.Values{"unit": "sentences", "count": 5}, "Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", func(t *testing.T, out string) {
			if n := strings.Count(out, "."); n != 5 {
				t.Errorf("%d sentences, want 5", n)
			}
		}},
		{"paragraphs", tool.Values{"count": 4}, "Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", func(t *testing.T, out string) {
			ps := strings.Split(out, "\n\n")
			if len(ps) != 4 {
				t.Fatalf("%d paragraphs, want 4", len(ps))
			}
			for _, p := range ps {
				if n := strings.Count(p, "."); n < 3 || n > 6 {
					t.Errorf("%d sentences in a paragraph, want 3–6", n)
				}
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := runLorem(t, tt.opts).Text
			if !strings.HasPrefix(out, tt.prefix) {
				t.Errorf("%q does not start with %q", out, tt.prefix)
			}
			tt.check(t, out)
		})
	}
}

func TestLoremSentencesAreCapitalised(t *testing.T) {
	out := runLorem(t, tool.Values{"unit": "sentences", "count": 50, "classic": false}).Text
	for _, s := range strings.SplitAfter(out, ". ") {
		if s == "" {
			continue
		}
		if c := s[0]; c < 'A' || c > 'Z' {
			t.Errorf("sentence without a capital: %q", s)
		}
	}
}

func TestLoremMeta(t *testing.T) {
	res := runLorem(t, tool.Values{"unit": "words", "count": 7})
	if res.Meta[0].Value != "7" {
		t.Errorf("Words = %q, want 7", res.Meta[0].Value)
	}
}

func TestLoremRandError(t *testing.T) {
	_, err := generators.Lorem{Rand: failingReader{}}.Run(context.Background(), tool.Request{})
	if err == nil {
		t.Error("expected the reader error")
	}
}
