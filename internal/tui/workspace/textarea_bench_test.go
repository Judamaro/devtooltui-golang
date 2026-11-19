package workspace_test

// Spike de la fase 0: ¿aguanta bubbles/textarea entradas de ~1 MB?
// Ejecutar con:  go test -run Spike -bench . -benchmem ./internal/tui/workspace/
// Resultados y decisión: docs/adr/0002-live-max-bytes.md

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

// genJSON genera un JSON de aproximadamente n bytes; pretty=true lo indenta
// (muchas líneas cortas) y pretty=false lo deja en una sola línea.
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
		it := item{ID: i, Name: fmt.Sprintf("elemento-%06d", i), Tags: []string{"a", "bb", "ccc"}, Valid: i%2 == 0}
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

// TestSpikeTruncation registra si el textarea conserva toda la entrada.
func TestSpikeTruncation(t *testing.T) {
	for _, sh := range shapes() {
		for _, n := range sizes {
			in := genJSON(n, sh.pretty)
			ta := newTextarea()
			ta.SetValue(in)
			got := len(ta.Value())
			t.Logf("%-8s %5d KB: %6d líneas, conserva %5.1f%%", sh.name, n>>10, strings.Count(in, "\n")+1, 100*float64(got)/float64(len(in)))
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

// BenchmarkTextareaKeystroke mide una tecla + View con la entrada ya cargada:
// es la latencia que percibe el usuario al escribir.
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

// BenchmarkTextareaValue mide Value(), que se llama en cada recálculo.
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

// BenchmarkViewportReadOnly es la alternativa de solo lectura.
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

// BenchmarkTextareaKeystrokeSmall busca el umbral en el que una tecla + View
// baja de 50 ms, con y sin números de línea.
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

// BenchmarkViewportScroll mide un frame (scroll + View) con el contenido ya
// cargado: es el costo por tecla en el modo de solo lectura.
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
