package registry_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

type fake struct {
	spec   tool.Spec
	detect func(string) float64
}

func (f fake) Spec() tool.Spec { return f.spec }
func (f fake) Run(context.Context, tool.Request) (tool.Result, error) {
	return tool.Result{}, nil
}

type fakeDetector struct{ fake }

func (f fakeDetector) Detect(s string) float64 { return f.detect(s) }

func spec(id string, mut ...func(*tool.Spec)) tool.Spec {
	s := tool.Spec{ID: id, Name: strings.ToUpper(id), Category: tool.Encoders}
	for _, m := range mut {
		m(&s)
	}
	return s
}

func withOpt(o tool.Option) func(*tool.Spec) {
	return func(s *tool.Spec) { s.Options = append(s.Options, o) }
}

func TestNewValidation(t *testing.T) {
	tests := []struct {
		name    string
		tools   []tool.Tool
		wantErr string
	}{
		{"valid", []tool.Tool{fake{spec: spec("ok")}}, ""},
		{"duplicate id", []tool.Tool{fake{spec: spec("a")}, fake{spec: spec("a")}}, "duplicate ID"},
		{"empty id", []tool.Tool{fake{spec: spec("")}}, "invalid ID"},
		{"uppercase id", []tool.Tool{fake{spec: spec("Base64")}}, "invalid ID"},
		{"no name", []tool.Tool{fake{spec: spec("a", func(s *tool.Spec) { s.Name = "" })}}, "empty Name"},
		{"unknown category", []tool.Tool{fake{spec: spec("a", func(s *tool.Spec) { s.Category = "X" })}}, "unknown category"},
		{"three inputs", []tool.Tool{fake{spec: spec("a", func(s *tool.Spec) {
			s.Inputs = []tool.InputSpec{{Key: "a"}, {Key: "b"}, {Key: "c"}}
		})}}, "maximum is 2"},
		{"repeated input", []tool.Tool{fake{spec: spec("a", func(s *tool.Spec) {
			s.Inputs = []tool.InputSpec{{Key: "a"}, {Key: "a"}}
		})}}, "repeated"},
		{"repeated option", []tool.Tool{fake{spec: spec("a",
			withOpt(tool.Option{Key: "x", Kind: tool.Toggle, Default: true}),
			withOpt(tool.Option{Key: "x", Kind: tool.Toggle, Default: true}))}}, "repeated"},
		{"toggle without bool", []tool.Tool{fake{spec: spec("a", withOpt(tool.Option{Key: "x", Kind: tool.Toggle, Default: "yes"}))}}, "is not a bool"},
		{"select outside choices", []tool.Tool{fake{spec: spec("a", withOpt(tool.Option{
			Key: "mode", Kind: tool.Select, Choices: []string{"encode", "decode"}, Default: "parse",
		}))}}, "is not in"},
		{"number out of range", []tool.Tool{fake{spec: spec("a", withOpt(tool.Option{
			Key: "n", Kind: tool.Number, Min: 1, Max: 10, Default: 11,
		}))}}, "outside"},
		{"number no int", []tool.Tool{fake{spec: spec("a", withOpt(tool.Option{
			Key: "n", Kind: tool.Number, Min: 1, Max: 10, Default: 2.5,
		}))}}, "is not an int"},
		{"string without string", []tool.Tool{fake{spec: spec("a", withOpt(tool.Option{Key: "s", Kind: tool.String, Default: 1}))}}, "is not a string"},
		{"unknown kind", []tool.Tool{fake{spec: spec("a", withOpt(tool.Option{Key: "s", Kind: 99}))}}, "unknown kind"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := registry.New(tt.tools...)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("New() error = %v", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Fatalf("New() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestGetAllByCategory(t *testing.T) {
	r, err := registry.New(
		fake{spec: spec("b64")},
		fake{spec: spec("json", func(s *tool.Spec) { s.Category = tool.Formatters })},
		fake{spec: spec("url")},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Get("json"); !ok {
		t.Error("Get(json) not found")
	}
	if _, ok := r.Get("nope"); ok {
		t.Error("Get(nope) found")
	}
	if got := ids(r.All()); got != "b64,json,url" {
		t.Errorf("All() = %s", got)
	}
	groups := r.ByCategory()
	if len(groups) != 2 || groups[0].Category != tool.Formatters || ids(groups[1].Tools) != "b64,url" {
		t.Errorf("ByCategory() = %+v", groups)
	}
}

func TestSearch(t *testing.T) {
	r, err := registry.New(
		fake{spec: spec("base64", func(s *tool.Spec) { s.Name = "Base64" })},
		fake{spec: spec("json-format", func(s *tool.Spec) { s.Name = "Formateador JSON"; s.Keywords = []string{"pretty"} })},
		fake{spec: spec("url", func(s *tool.Spec) { s.Name = "URL" })},
	)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		q     string
		first string
		n     int
	}{
		{"", "base64", 3},
		{"b64", "base64", 1},
		{"json", "json-format", 1},
		{"pretty", "json-format", 1},
		{"zzz", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.q, func(t *testing.T) {
			got := r.Search(tt.q)
			if len(got) != tt.n {
				t.Fatalf("Search(%q) = %d results, want %d", tt.q, len(got), tt.n)
			}
			if tt.n > 0 && got[0].Tool.Spec().ID != tt.first {
				t.Errorf("Search(%q)[0] = %s, want %s", tt.q, got[0].Tool.Spec().ID, tt.first)
			}
		})
	}
}

func TestDetect(t *testing.T) {
	det := func(id string, f func(string) float64) tool.Tool {
		return fakeDetector{fake{spec: spec(id), detect: f}}
	}
	r, err := registry.New(
		det("low", func(string) float64 { return 0.3 }),
		det("high", func(string) float64 { return 0.9 }),
		det("zero", func(string) float64 { return 0 }),
		det("panics", func(string) float64 { panic("boom") }),
		det("slow", func(string) float64 { time.Sleep(200 * time.Millisecond); return 1 }),
		det("over", func(string) float64 { return 7 }),
		fake{spec: spec("plain")},
	)
	if err != nil {
		t.Fatal(err)
	}
	got := r.Detect("x")
	var parts []string
	for _, s := range got {
		parts = append(parts, s.Tool.Spec().ID)
	}
	if strings.Join(parts, ",") != "over,high,low" {
		t.Errorf("Detect() = %v", parts)
	}
	if got[0].Score != 1 {
		t.Errorf("score not capped at 1: %v", got[0].Score)
	}
}

func ids(ts []tool.Tool) string {
	var s []string
	for _, t := range ts {
		s = append(s, t.Spec().ID)
	}
	return strings.Join(s, ",")
}
