// Package registry holds the available tools and offers fuzzy search and
// smart detection over them.
package registry

import (
	"cmp"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/sahilm/fuzzy"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// DetectTimeout is the most time each Detector may take.
const DetectTimeout = 20 * time.Millisecond

var idPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Registry is immutable after New and safe for concurrent use.
type Registry struct {
	tools []tool.Tool
	byID  map[string]tool.Tool
}

// Group groups the tools of one category.
type Group struct {
	Category tool.Category
	Tools    []tool.Tool
}

// Match is a Search result.
type Match struct {
	Tool  tool.Tool
	Score int
}

// Scored is a Detect result.
type Scored struct {
	Tool  tool.Tool
	Score float64
}

// New builds a registry, validating every Spec. Registration order is kept.
func New(tools ...tool.Tool) (*Registry, error) {
	r := &Registry{byID: make(map[string]tool.Tool, len(tools))}
	var errs []error
	for _, t := range tools {
		s := t.Spec()
		if err := validateSpec(s); err != nil {
			errs = append(errs, err)
			continue
		}
		if _, dup := r.byID[s.ID]; dup {
			errs = append(errs, fmt.Errorf("%s: duplicate ID", s.ID))
			continue
		}
		r.byID[s.ID] = t
		r.tools = append(r.tools, t)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return r, nil
}

// Get returns the tool with that ID.
func (r *Registry) Get(id string) (tool.Tool, bool) {
	t, ok := r.byID[id]
	return t, ok
}

// All returns the tools in registration order.
func (r *Registry) All() []tool.Tool {
	return slices.Clone(r.tools)
}

// ByCategory groups tools by category in tool.Categories() order,
// skipping empty categories.
func (r *Registry) ByCategory() []Group {
	var groups []Group
	for _, c := range tool.Categories() {
		var ts []tool.Tool
		for _, t := range r.tools {
			if t.Spec().Category == c {
				ts = append(ts, t)
			}
		}
		if len(ts) > 0 {
			groups = append(groups, Group{Category: c, Tools: ts})
		}
	}
	return groups
}

// Search fuzzy-searches Name, ID and Keywords. An empty q returns every
// tool in registration order.
func (r *Registry) Search(q string) []Match {
	q = strings.TrimSpace(q)
	if q == "" {
		out := make([]Match, len(r.tools))
		for i, t := range r.tools {
			out[i] = Match{Tool: t}
		}
		return out
	}
	best := make(map[int]int) // tool index → best score
	for i, t := range r.tools {
		s := t.Spec()
		cands := append([]string{s.Name, s.ID}, s.Keywords...)
		for _, m := range fuzzy.Find(q, cands) {
			if prev, ok := best[i]; !ok || m.Score > prev {
				best[i] = m.Score
			}
		}
	}
	out := make([]Match, 0, len(best))
	for i, score := range best {
		out = append(out, Match{Tool: r.tools[i], Score: score})
	}
	slices.SortStableFunc(out, func(a, b Match) int {
		if c := cmp.Compare(b.Score, a.Score); c != 0 {
			return c
		}
		return cmp.Compare(r.index(a.Tool), r.index(b.Tool))
	})
	return out
}

// Detect runs the Detectors on sample and returns the scores above zero,
// highest first. A detector that panics or exceeds DetectTimeout counts
// as 0.
func (r *Registry) Detect(sample string) []Scored {
	var out []Scored
	for _, t := range r.tools {
		d, ok := t.(tool.Detector)
		if !ok {
			continue
		}
		if score := runDetector(d, sample); score > 0 {
			out = append(out, Scored{Tool: t, Score: min(score, 1)})
		}
	}
	slices.SortStableFunc(out, func(a, b Scored) int { return cmp.Compare(b.Score, a.Score) })
	return out
}

// runDetector gives each Detect DetectTimeout from the moment it starts
// running: waiting for the goroutine to be scheduled does not count (on
// slow runners with -race, that latency alone used up the 20 ms).
func runDetector(d tool.Detector, sample string) float64 {
	ch := make(chan float64, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		defer func() {
			if recover() != nil {
				ch <- 0
			}
		}()
		ch <- d.Detect(sample)
	}()
	<-started
	timer := time.NewTimer(DetectTimeout)
	defer timer.Stop()
	select {
	case s := <-ch:
		return s
	case <-timer.C:
		return 0
	}
}

func (r *Registry) index(t tool.Tool) int {
	return slices.Index(r.tools, t)
}

func validateSpec(s tool.Spec) error {
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	if !idPattern.MatchString(s.ID) {
		add("invalid ID %q: use lowercase kebab-case", s.ID)
	}
	if s.Name == "" {
		add("empty Name")
	}
	if !slices.Contains(tool.Categories(), s.Category) {
		add("unknown category %q", s.Category)
	}
	if len(s.Inputs) > 2 {
		add("%d inputs; the maximum is 2", len(s.Inputs))
	}
	inputKeys := map[string]bool{}
	for _, in := range s.Inputs {
		if in.Key == "" || inputKeys[in.Key] {
			add("input with empty or repeated key %q", in.Key)
		}
		inputKeys[in.Key] = true
	}
	optKeys := map[string]bool{}
	for _, o := range s.Options {
		if o.Key == "" || optKeys[o.Key] {
			add("option with empty or repeated key %q", o.Key)
		}
		optKeys[o.Key] = true
		if err := validateOption(o); err != nil {
			add("option %q: %w", o.Key, err)
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("%s: %w", s.ID, err)
	}
	return nil
}

func validateOption(o tool.Option) error {
	switch o.Kind {
	case tool.Toggle:
		if _, ok := o.Default.(bool); !ok {
			return fmt.Errorf("default %v is not a bool", o.Default)
		}
	case tool.Select:
		d, ok := o.Default.(string)
		if !ok || !slices.Contains(o.Choices, d) {
			return fmt.Errorf("default %v is not in %v", o.Default, o.Choices)
		}
	case tool.Number:
		d, ok := o.Default.(int)
		if !ok {
			return fmt.Errorf("default %v is not an int", o.Default)
		}
		if o.Min > o.Max || d < o.Min || d > o.Max {
			return fmt.Errorf("default %d outside [%d, %d]", d, o.Min, o.Max)
		}
	case tool.String:
		if _, ok := o.Default.(string); !ok {
			return fmt.Errorf("default %v is not a string", o.Default)
		}
	default:
		return fmt.Errorf("unknown kind %d", o.Kind)
	}
	return nil
}
