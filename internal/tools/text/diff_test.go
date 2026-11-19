package text_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/text"
)

func runDiff(t testing.TB, a, b string, opts tool.Values) (tool.Result, error) {
	t.Helper()
	return text.Diff{}.Run(context.Background(), tool.Request{
		Inputs: map[string]string{"a": a, "b": b},
		Opts:   opts,
	})
}

func TestDiffGolden(t *testing.T) {
	tests := []struct {
		name string
		opts tool.Values
	}{
		{"go-unified", nil},
		{"go-side-by-side", tool.Values{"view": "side-by-side"}},
		{"go-words", tool.Values{"by": "words"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runDiff(t, testutil.ReadInput(t, "diff", "a"), testutil.ReadInput(t, "diff", "b"), tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "diff", tt.name, res.Text)
		})
	}
}

func TestDiff(t *testing.T) {
	tests := []struct {
		name, a, b string
		opts       tool.Values
		want       string
		meta       string
	}{
		{"identical", "x\ny\n", "x\ny\n", nil, "No differences", "identical"},
		{"empty", "", "", nil, "No differences", "identical"},
		{"adds a line", "a\n", "a\nb\n", nil, "--- A\n+++ B\n@@ -1,1 +1,2 @@\n a\n+b", "+1 −0 lines"},
		{"from empty", "", "x", nil, "--- A\n+++ B\n@@ -0,0 +1,1 @@\n+x", "+1 −0 lines"},
		{"ignores spaces", "a  b\n", "a b\n", tool.Values{"ignore_space": true}, "No differences", "identical"},
		{"without ignoring spaces", "a  b\n", "a b\n", nil, "-a  b\n+a b", "+1 −1 lines"},
		{"words", "the black cat", "the black dog", tool.Values{"by": "words"}, "the black [-cat-]{+dog+}", "+1 −1 words"},
		{"words ignore spaces", "a  b", "a b", tool.Values{"by": "words", "ignore_space": true}, "a  b", "identical"},
		{"side by side", "a\nb\n", "a\nc\nd\n", tool.Values{"view": "side-by-side"}, "  a          │ a\n| b          │ c\n>            │ d", "+2 −1 lines"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runDiff(t, tt.a, tt.b, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(res.Text, tt.want) {
				t.Errorf("Text =\n%s\ndoes not contain\n%s", res.Text, tt.want)
			}
			if res.Meta[0].Value != tt.meta {
				t.Errorf("Meta = %q, want %q", res.Meta[0].Value, tt.meta)
			}
		})
	}
}

// The operations rebuild A (Equal+Delete) and B (Equal+Insert).
func FuzzDiffReconstructs(f *testing.F) {
	f.Add("a\nb\nc\n", "a\nx\nc", false, false)
	f.Add("the cat", "the  dog", true, true)
	f.Fuzz(func(t *testing.T, a, b string, words, ignore bool) {
		by := "lines"
		if words {
			by = "words"
		}
		res, err := runDiff(t, a, b, tool.Values{"by": by, "ignore_space": ignore})
		if err != nil {
			t.Fatal(err)
		}
		var ga, gb strings.Builder
		for _, op := range res.Diff {
			if op.Kind != tool.Insert {
				ga.WriteString(op.Text)
			}
			if op.Kind != tool.Delete {
				gb.WriteString(op.Text)
			}
		}
		gotA, gotB, wantA, wantB := ga.String(), gb.String(), a, b
		if !words { // by lines, the final "\n" does not count
			gotA, gotB = strings.TrimSuffix(gotA, "\n"), strings.TrimSuffix(gotB, "\n")
			wantA, wantB = strings.TrimSuffix(a, "\n"), strings.TrimSuffix(b, "\n")
		}
		if gotA != wantA || (!ignore && gotB != wantB) {
			t.Errorf("does not rebuild:\nA %q → %q\nB %q → %q", wantA, gotA, wantB, gotB)
		}
	})
}
