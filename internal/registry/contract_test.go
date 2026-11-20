package registry_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/registry"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools"
)

// TestContract checks the contract on every registered tool.
func TestContract(t *testing.T) {
	r, err := registry.New(tools.All()...)
	if err != nil {
		t.Fatalf("invalid registry: %v", err)
	}
	for _, tl := range r.All() {
		spec := tl.Spec()
		t.Run(spec.ID, func(t *testing.T) {
			empty := tool.Request{Inputs: map[string]string{}, Opts: tool.Values{}.WithDefaults(spec)}
			for _, in := range spec.Inputs {
				empty.Inputs[in.Key] = ""
			}

			t.Run("empty input", func(t *testing.T) {
				_, err := tl.Run(context.Background(), empty)
				if err != nil {
					if _, ok := tool.AsInputError(err); !ok {
						t.Errorf("error that is not *InputError: %v", err)
					}
				}
			})

			t.Run("cancelled ctx", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				_, err := tl.Run(ctx, empty)
				if !errors.Is(err, context.Canceled) {
					t.Errorf("Run with cancelled ctx = %v, want context.Canceled", err)
				}
			})

			t.Run("every choice", func(t *testing.T) {
				for _, o := range spec.Options {
					for _, c := range o.Choices {
						req := empty
						req.Opts = tool.Values{o.Key: c}.WithDefaults(spec)
						if _, err := tl.Run(context.Background(), req); err != nil {
							if _, ok := tool.AsInputError(err); !ok {
								t.Errorf("%s=%s: error that is not *InputError: %v", o.Key, c, err)
							}
						}
					}
				}
			})

			if d, ok := tl.(tool.Detector); ok {
				t.Run("detect in range", func(t *testing.T) {
					for _, s := range []string{"", " ", "\x00\xff", "{}", "a.b.c"} {
						if score := d.Detect(s); score < 0 || score > 1 {
							t.Errorf("Detect(%q) = %v outside [0,1]", s, score)
						}
					}
				})
			}
		})
	}
}
