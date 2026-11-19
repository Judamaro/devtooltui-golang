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

// TestRunRealTools walks the plan's detection table with the real
// tools.
func TestRunRealTools(t *testing.T) {
	reg := allTools(t)
	tests := []struct {
		name, sample string
		want         string // "" = no detection
	}{
		{"jwt", testutil.ReadInput(t, "jwt", "hs256"), "jwt"},
		{"uuid", "f47ac10b-58cc-4372-a567-0e02b2c3d479", "uuid"},
		{"json", `{"id": 1, "items": ["a", "b"]}`, "json-format"},
		{"xml", `<?xml version="1.0"?><a><b/></a>`, "xml-format"},
		{"timestamp", "1767225600", "timestamp"},
		{"base64", "SG9sYSwgbXVuZG8u", "base64"},
		{"url", "q%3Dhola%26x%3D1", "url"},
		{"plain text", "hello world, nothing special here", ""},
		{"empty", "", ""},
		{"yaml (0.40) stays below the threshold", "a: 1\nb:\n  c: 2", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := detect.Run(reg, tt.sample)
			got := ""
			if out.Best != nil {
				got = out.Best.Tool.Spec().ID
			}
			if got != tt.want || out.Ambiguous {
				t.Errorf("Best = %q (ambiguous %v), want %q; candidates %v", got, out.Ambiguous, tt.want, ids(out.Candidates))
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
		{"clear winner", []tool.Tool{fixed{"a", 0.9}, fixed{"b", 0.6}}, "a", false, "a,b"},
		{"tie asks", []tool.Tool{fixed{"a", 0.9}, fixed{"b", 0.85}, fixed{"c", 0.6}}, "", true, "a,b"},
		{"exclusive threshold", []tool.Tool{fixed{"a", 0.5}}, "", false, ""},
		{"none", []tool.Tool{fixed{"a", 0.2}}, "", false, ""},
		{"three-way tie", []tool.Tool{fixed{"a", 0.7}, fixed{"b", 0.65}, fixed{"c", 0.61}}, "", true, "a,b,c"},
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
				t.Errorf("= %q ambiguous=%v [%s], want %q ambiguous=%v [%s]", best, out.Ambiguous, ids(out.Candidates), tt.best, tt.ambiguous, tt.cands)
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
		{"short", "abc", 3},
		{"exact", strings.Repeat("a", detect.SampleBytes), detect.SampleBytes},
		{"long", strings.Repeat("a", detect.SampleBytes+10), detect.SampleBytes},
		{"does not split runes", strings.Repeat("a", detect.SampleBytes-1) + "ñ", detect.SampleBytes - 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := len(detect.Sample(tt.in)); got != tt.want {
				t.Errorf("len = %d, want %d", got, tt.want)
			}
		})
	}
}

// BenchmarkRun64KB: the plan asks for detecting 64 KB in under 20 ms.
func BenchmarkRun64KB(b *testing.B) {
	reg := allTools(b)
	sample := strings.Repeat(`{"id": 1, "name": "elemento", "ok": true},`, detect.SampleBytes/42)
	sample = "[" + sample[:len(sample)-1] + "]"
	for b.Loop() {
		detect.Run(reg, sample)
	}
}

// FuzzRun: no input makes detection panic.
func FuzzRun(f *testing.F) {
	reg := allTools(f)
	for _, s := range []string{"", "{", "<a", "a.b.c", "%zz", "0x", "ñ\x00\xff"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out := detect.Run(reg, s)
		for _, c := range out.Candidates {
			if c.Score <= detect.Threshold || c.Score > 1 {
				t.Fatalf("score out of range: %v", c.Score)
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
