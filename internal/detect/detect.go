// Package detect decides which tool to jump to for a piece of text: it takes
// a sample, runs the registry's Detectors and applies the threshold and the
// tie rule. The heuristics live in each tool's Detect.
package detect

import (
	"unicode/utf8"

	"github.com/Judamaro/devtooltui-golang/internal/registry"
)

const (
	SampleBytes = 64 << 10 // sample size
	Threshold   = 0.5      // minimum score to jump
	TieGap      = 0.1      // gap below which the user is asked
)

// Outcome is the detection result.
type Outcome struct {
	// Best is the chosen tool; nil when nothing beats Threshold or when
	// there is a tie.
	Best *registry.Scored
	// Candidates are the tools above Threshold, highest first. On a tie,
	// only those within TieGap of the first one.
	Candidates []registry.Scored
	Ambiguous  bool
}

// Sample cuts s to SampleBytes without splitting a UTF-8 character.
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

// Run detects on a sample of text.
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
