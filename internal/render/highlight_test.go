package render

import (
	"fmt"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/x/ansi"
)

var testStyle = chroma.MustNewStyle("test", chroma.StyleEntries{
	chroma.NameTag:       "#ansiblue",
	chroma.LiteralString: "#ansigreen",
})

func TestHighlight(t *testing.T) {
	tests := []struct {
		name, text, lang string
		style            *chroma.Style
		wantOK           bool
	}{
		{"json", `{"a": "b"}`, "json", testStyle, true},
		{"yaml", "a: b\n", "yaml", testStyle, true},
		{"xml", "<a x=\"1\">b</a>", "xml", testStyle, true},
		{"no language", "x", "", testStyle, false},
		{"unknown language", "x", "does-not-exist", testStyle, false},
		{"no style", "x", "json", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, ok := Highlight(tt.text, tt.lang, tt.style)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if !strings.Contains(out, "\x1b[") {
				t.Error("the output has no colour codes")
			}
			if ansi.Strip(out) != tt.text {
				t.Errorf("stripping colours changes the text: %q", ansi.Strip(out))
			}
		})
	}
}

func jsonOfSize(n int) string {
	var b strings.Builder
	b.WriteString("[\n")
	for i := 0; b.Len() < n; i++ {
		fmt.Fprintf(&b, "  {\"id\": %d, \"name\": \"item-%06d\", \"ok\": true, \"tags\": [\"a\", \"b\"]},\n", i, i)
	}
	b.WriteString("  {}\n]")
	return b.String()
}

// BenchmarkHighlight backs MaxBytes: colouring must stay close to the
// 50 ms budget per update.
func BenchmarkHighlight(b *testing.B) {
	for _, n := range []int{64 << 10, 256 << 10, 1 << 20} {
		text := jsonOfSize(n)
		b.Run(fmt.Sprintf("%dKB", n>>10), func(b *testing.B) {
			b.SetBytes(int64(len(text)))
			for b.Loop() {
				if _, ok := Highlight(text, "json", testStyle); !ok {
					b.Fatal("did not colour")
				}
			}
		})
	}
}
