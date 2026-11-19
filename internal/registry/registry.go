// Package registry guarda las herramientas disponibles y ofrece búsqueda
// difusa y detección inteligente sobre ellas.
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

// DetectTimeout es el tiempo máximo que puede tardar cada Detector.
const DetectTimeout = 20 * time.Millisecond

var idPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Registry es inmutable después de New y seguro para uso concurrente.
type Registry struct {
	tools []tool.Tool
	byID  map[string]tool.Tool
}

// Group agrupa las herramientas de una categoría.
type Group struct {
	Category tool.Category
	Tools    []tool.Tool
}

// Match es un resultado de Search.
type Match struct {
	Tool  tool.Tool
	Score int
}

// Scored es un resultado de Detect.
type Scored struct {
	Tool  tool.Tool
	Score float64
}

// New crea un registro validando cada Spec. El orden de registro se conserva.
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
			errs = append(errs, fmt.Errorf("%s: ID duplicado", s.ID))
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

// Get devuelve la herramienta con ese ID.
func (r *Registry) Get(id string) (tool.Tool, bool) {
	t, ok := r.byID[id]
	return t, ok
}

// All devuelve las herramientas en orden de registro.
func (r *Registry) All() []tool.Tool {
	return slices.Clone(r.tools)
}

// ByCategory agrupa por categoría en el orden de tool.Categories(),
// omitiendo las categorías vacías.
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

// Search hace búsqueda difusa sobre Name, ID y Keywords. Con q vacío
// devuelve todas las herramientas en orden de registro.
func (r *Registry) Search(q string) []Match {
	q = strings.TrimSpace(q)
	if q == "" {
		out := make([]Match, len(r.tools))
		for i, t := range r.tools {
			out[i] = Match{Tool: t}
		}
		return out
	}
	best := make(map[int]int) // índice de tool → mejor puntaje
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

// Detect ejecuta los Detector sobre sample y devuelve los puntajes mayores
// que cero, ordenados de mayor a menor. Un detector que entra en pánico o
// supera DetectTimeout cuenta como 0.
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

func runDetector(d tool.Detector, sample string) float64 {
	timer := time.NewTimer(DetectTimeout)
	defer timer.Stop()
	ch := make(chan float64, 1)
	go func() {
		defer func() {
			if recover() != nil {
				ch <- 0
			}
		}()
		ch <- d.Detect(sample)
	}()
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
		add("ID %q inválido: usa kebab-case en minúsculas", s.ID)
	}
	if s.Name == "" {
		add("Name vacío")
	}
	if !slices.Contains(tool.Categories(), s.Category) {
		add("categoría %q desconocida", s.Category)
	}
	if len(s.Inputs) > 2 {
		add("%d entradas; el máximo es 2", len(s.Inputs))
	}
	inputKeys := map[string]bool{}
	for _, in := range s.Inputs {
		if in.Key == "" || inputKeys[in.Key] {
			add("entrada con clave vacía o repetida %q", in.Key)
		}
		inputKeys[in.Key] = true
	}
	optKeys := map[string]bool{}
	for _, o := range s.Options {
		if o.Key == "" || optKeys[o.Key] {
			add("opción con clave vacía o repetida %q", o.Key)
		}
		optKeys[o.Key] = true
		if err := validateOption(o); err != nil {
			add("opción %q: %w", o.Key, err)
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
			return fmt.Errorf("default %v no es bool", o.Default)
		}
	case tool.Select:
		d, ok := o.Default.(string)
		if !ok || !slices.Contains(o.Choices, d) {
			return fmt.Errorf("default %v no está en %v", o.Default, o.Choices)
		}
	case tool.Number:
		d, ok := o.Default.(int)
		if !ok {
			return fmt.Errorf("default %v no es int", o.Default)
		}
		if o.Min > o.Max || d < o.Min || d > o.Max {
			return fmt.Errorf("default %d fuera de [%d, %d]", d, o.Min, o.Max)
		}
	case tool.String:
		if _, ok := o.Default.(string); !ok {
			return fmt.Errorf("default %v no es string", o.Default)
		}
	default:
		return fmt.Errorf("kind %d desconocido", o.Kind)
	}
	return nil
}
