// Package detect decide a qué herramienta saltar a partir de un texto: toma
// una muestra, ejecuta los Detector del registro y aplica el umbral y la
// regla de empate. Las heurísticas viven en el Detect de cada herramienta.
package detect

import (
	"unicode/utf8"

	"github.com/Judamaro/devtooltui-golang/internal/registry"
)

const (
	SampleBytes = 64 << 10 // tamaño de la muestra
	Threshold   = 0.5      // puntaje mínimo para saltar
	TieGap      = 0.1      // diferencia por debajo de la cual se pregunta
)

// Outcome es el resultado de la detección.
type Outcome struct {
	// Best es la herramienta elegida; nil si nadie supera Threshold o si
	// hay empate.
	Best *registry.Scored
	// Candidates son las que superan Threshold, de mayor a menor. Con
	// empate, solo las que están a menos de TieGap de la primera.
	Candidates []registry.Scored
	Ambiguous  bool
}

// Sample recorta s a SampleBytes sin partir un carácter UTF-8.
func Sample(s string) string {
	if len(s) <= SampleBytes {
		return s
	}
	cut := SampleBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// Run detecta sobre una muestra de text.
func Run(reg *registry.Registry, text string) Outcome {
	var above []registry.Scored
	for _, sc := range reg.Detect(Sample(text)) {
		if sc.Score > Threshold {
			above = append(above, sc)
		}
	}
	if len(above) == 0 {
		return Outcome{}
	}
	if len(above) > 1 && above[0].Score-above[1].Score < TieGap {
		var tied []registry.Scored
		for _, sc := range above {
			if above[0].Score-sc.Score < TieGap {
				tied = append(tied, sc)
			}
		}
		return Outcome{Candidates: tied, Ambiguous: true}
	}
	best := above[0]
	return Outcome{Best: &best, Candidates: above}
}
