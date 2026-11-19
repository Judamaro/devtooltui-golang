package workspace_test

// Phase 0 spike: can bubbles/textarea handle ~1 MB inputs?
// Run with:  go test -run Spike -bench . -benchmem ./internal/tui/workspace/
// Results and decision: docs/adr/0002-live-max-bytes.md

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

var sizes = []int{100 << 10, 500 << 10, 1 << 20, 2 << 20}

// genJSON generates JSON of about n bytes; pretty=true indents it (many
// short lines) and pretty=false keeps it on a single line.
func genJSON(n int, pretty bool) string {
	type item struct {
		ID    int      `json:"id"`
		Name  string   `json:"name"`
		Tags  []string `json:"tags"`
		Valid bool     `json:"valid"`
	}
	var items []item
	size := 0
	for i := 0; size < n; i++ {
		it := item{ID: i, Name: fmt.Sprintf("item-%06d", i), Tags: []string{"a", "bb", "ccc"}, Valid: i%2 == 0}
		items = append(items, it)
		size += 90
	}
	var b []byte
	if pretty {
		b, _ = json.MarshalIndent(items, "", "  ")
	} else {
		b, _ = json.Marshal(items)
	}
	return string(b[:min(len(b), n)])
}

func newTextarea() textarea.Model {
	ta := textarea.New()
	ta.MaxHeight = 0
	ta.MaxWidth = 0
	ta.CharLimit = 0
	ta.SetWidth(80)
	ta.SetHeight(20)
	ta.Focus()
	return ta
}

var keyX = tea.KeyPressMsg{Code: 'x', Text: "x"}

func shapes() []struct {
	name   string
	pretty bool
} {
	return []struct {
		name   string
		pretty bool
	}{{"pretty", true}, {"minified", false}}
}

// TestSpikeTruncation logs whether the textarea keeps the whole input.
func TestSpikeTruncation(t *testing.T) {
	for _, sh := range shapes() {
		for _, n := range sizes {
			in := genJSON(n, sh.pretty)
			ta := newTextarea()
			ta.SetValue(in)
			got := len(ta.Value())
			t.Logf("%-8s %5d KB: %6d lines, keeps %5.1f%%", sh.name, n>>10, strings.Count(in, "\n")+1, 100*float64(got)/float64(len(in)))
		}
	}
}

func BenchmarkTextareaSetValue(b *testing.B) {
	for _, sh := range shapes() {
		for _, n := range sizes {
			in := genJSON(n, sh.pretty)
			b.Run(fmt.Sprintf("%s/%dKB", sh.name, n>>10), func(b *testing.B) {
				for b.Loop() {
					ta := newTextarea()
					ta.SetValue(in)
				}
			})
		}
	}
}

func BenchmarkTextareaPaste(b *testing.B) {
	for _, sh := range shapes() {
		for _, n := range sizes {
			in := genJSON(n, sh.pretty)
			b.Run(fmt.Sprintf("%s/%dKB", sh.name, n>>10), func(b *testing.B) {
				for b.Loop() {
					ta := newTextarea()
					ta, _ = ta.Update(tea.PasteMsg{Content: in})
					_ = ta.View()
				}
			})
		}
	}
}

// BenchmarkTextareaKeystroke measures one key + View with the input already
// loaded: the latency the user perceives while typing.
func BenchmarkTextareaKeystroke(b *testing.B) {
	for _, sh := range shapes() {
		for _, n := range sizes {
			in := genJSON(n, sh.pretty)
			b.Run(fmt.Sprintf("%s/%dKB", sh.name, n>>10), func(b *testing.B) {
				ta := newTextarea()
				ta.SetValue(in)
				for b.Loop() {
					ta, _ = ta.Update(keyX)
					_ = ta.View()
				}
			})
		}
	}
}

// BenchmarkTextareaValue measures Value(), which is called on every rerun.
func BenchmarkTextareaValue(b *testing.B) {
	for _, sh := range shapes() {
		for _, n := range sizes {
			in := genJSON(n, sh.pretty)
			b.Run(fmt.Sprintf("%s/%dKB", sh.name, n>>10), func(b *testing.B) {
				ta := newTextarea()
				ta.SetValue(in)
				for b.Loop() {
					_ = ta.Value()
				}
			})
		}
	}
}

// BenchmarkViewportReadOnly is the read-only alternative.
func BenchmarkViewportReadOnly(b *testing.B) {
	for _, sh := range shapes() {
		for _, n := range sizes {
			in := genJSON(n, sh.pretty)
			b.Run(fmt.Sprintf("%s/%dKB", sh.name, n>>10), func(b *testing.B) {
				for b.Loop() {
					vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
					vp.SetContent(in)
					_ = vp.View()
				}
			})
		}
	}
}

// BenchmarkTextareaKeystrokeSmall looks for the size at which one key + View
// drops below 50 ms, with and without line numbers.
func BenchmarkTextareaKeystrokeSmall(b *testing.B) {
	for _, sh := range shapes() {
		for _, lineNumbers := range []bool{true, false} {
			for _, n := range []int{8 << 10, 16 << 10, 32 << 10, 64 << 10} {
				in := genJSON(n, sh.pretty)
				b.Run(fmt.Sprintf("%s/lines=%v/%dKB", sh.name, lineNumbers, n>>10), func(b *testing.B) {
					ta := newTextarea()
					ta.ShowLineNumbers = lineNumbers
					ta.SetValue(in)
					for b.Loop() {
						ta, _ = ta.Update(keyX)
						_ = ta.View()
					}
				})
			}
		}
	}
}

// BenchmarkViewportScroll measures one frame (scroll + View) with the content
// already loaded: the per-key cost in read-only mode.
func BenchmarkViewportScroll(b *testing.B) {
	for _, sh := range shapes() {
		for _, n := range sizes {
			in := genJSON(n, sh.pretty)
			b.Run(fmt.Sprintf("%s/%dKB", sh.name, n>>10), func(b *testing.B) {
				vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
				vp.SetContent(in)
				for b.Loop() {
					vp.ScrollDown(1)
					_ = vp.View()
				}
			})
		}
	}
}
