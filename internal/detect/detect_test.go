package detect_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/detect"
	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools"
)

func allTools(t testing.TB) *registry.Registry {
	t.Helper()
	reg, err := registry.New(tools.All()...)
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

// TestRunRealTools recorre la tabla de detección del plan con las
// herramientas reales.
func TestRunRealTools(t *testing.T) {
	reg := allTools(t)
	tests := []struct {
		name, sample string
		want         string // "" = sin detección
	}{
		{"jwt", testutil.ReadInput(t, "jwt", "hs256"), "jwt"},
		{"uuid", "f47ac10b-58cc-4372-a567-0e02b2c3d479", "uuid"},
		{"json", `{"id": 1, "items": ["a", "b"]}`, "json-format"},
		{"xml", `<?xml version="1.0"?><a><b/></a>`, "xml-format"},
		{"timestamp", "1767225600", "timestamp"},
		{"base64", "SG9sYSwgbXVuZG8u", "base64"},
		{"url", "q%3Dhola%26x%3D1", "url"},
		{"texto plano", "hola mundo, esto no es nada en especial", ""},
		{"vacio", "", ""},
		{"yaml (0.40) no supera el umbral", "a: 1\nb:\n  c: 2", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := detect.Run(reg, tt.sample)
			got := ""
			if out.Best != nil {
				got = out.Best.Tool.Spec().ID
			}
			if got != tt.want || out.Ambiguous {
				t.Errorf("Best = %q (ambiguo %v), want %q; candidatas %v", got, out.Ambiguous, tt.want, ids(out.Candidates))
			}
		})
	}
}

type fixed struct {
	id    string
	score float64
}

func (f fixed) Spec() tool.Spec {
	return tool.Spec{ID: f.id, Name: f.id, Category: tool.Text}
}
func (fixed) Run(context.Context, tool.Request) (tool.Result, error) { return tool.Result{}, nil }
func (f fixed) Detect(string) float64                                { return f.score }

func TestRunRules(t *testing.T) {
	tests := []struct {
		name      string
		tools     []tool.Tool
		best      string
		ambiguous bool
		cands     string
	}{
		{"gana clara", []tool.Tool{fixed{"a", 0.9}, fixed{"b", 0.6}}, "a", false, "a,b"},
		{"empate pregunta", []tool.Tool{fixed{"a", 0.9}, fixed{"b", 0.85}, fixed{"c", 0.6}}, "", true, "a,b"},
		{"umbral exclusivo", []tool.Tool{fixed{"a", 0.5}}, "", false, ""},
		{"ninguna", []tool.Tool{fixed{"a", 0.2}}, "", false, ""},
		{"empate de tres", []tool.Tool{fixed{"a", 0.7}, fixed{"b", 0.65}, fixed{"c", 0.61}}, "", true, "a,b,c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, err := registry.New(tt.tools...)
			if err != nil {
				t.Fatal(err)
			}
			out := detect.Run(reg, "x")
			best := ""
			if out.Best != nil {
				best = out.Best.Tool.Spec().ID
			}
			if best != tt.best || out.Ambiguous != tt.ambiguous || ids(out.Candidates) != tt.cands {
				t.Errorf("= %q ambiguo=%v [%s], want %q ambiguo=%v [%s]", best, out.Ambiguous, ids(out.Candidates), tt.best, tt.ambiguous, tt.cands)
			}
		})
	}
}

func TestSample(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"corto", "abc", 3},
		{"exacto", strings.Repeat("a", detect.SampleBytes), detect.SampleBytes},
		{"largo", strings.Repeat("a", detect.SampleBytes+10), detect.SampleBytes},
		{"no parte runas", strings.Repeat("a", detect.SampleBytes-1) + "ñ", detect.SampleBytes - 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := len(detect.Sample(tt.in)); got != tt.want {
				t.Errorf("len = %d, want %d", got, tt.want)
			}
		})
	}
}

// BenchmarkRun64KB: el plan pide detectar 64 KB en menos de 20 ms.
func BenchmarkRun64KB(b *testing.B) {
	reg := allTools(b)
	sample := strings.Repeat(`{"id": 1, "name": "elemento", "ok": true},`, detect.SampleBytes/42)
	sample = "[" + sample[:len(sample)-1] + "]"
	for b.Loop() {
		detect.Run(reg, sample)
	}
}

// FuzzRun: ninguna entrada hace entrar en pánico a la detección.
func FuzzRun(f *testing.F) {
	reg := allTools(f)
	for _, s := range []string{"", "{", "<a", "a.b.c", "%zz", "0x", "ñ\x00\xff"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out := detect.Run(reg, s)
		for _, c := range out.Candidates {
			if c.Score <= detect.Threshold || c.Score > 1 {
				t.Fatalf("puntaje fuera de rango: %v", c.Score)
			}
		}
	})
}

func ids(sc []registry.Scored) string {
	var s []string
	for _, c := range sc {
		s = append(s, c.Tool.Spec().ID)
	}
	return strings.Join(s, ",")
}
