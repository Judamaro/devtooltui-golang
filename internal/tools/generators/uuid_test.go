package generators_test

import (
	"context"
	"math/rand/v2"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/generators"
)

var uuidNow = func() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) }

// newUUID uses a deterministic generator so the goldens stay stable.
func newUUID() generators.UUID {
	return generators.UUID{Now: uuidNow, Rand: rand.NewChaCha8([32]byte{1, 2, 3})}
}

func runUUID(t *testing.T, in string, opts tool.Values) (tool.Result, error) {
	t.Helper()
	return newUUID().Run(context.Background(), tool.Request{Inputs: map[string]string{"input": in}, Opts: opts})
}

func TestUUIDGenerateGolden(t *testing.T) {
	tests := []struct {
		name string
		opts tool.Values
	}{
		{"v4-x3", tool.Values{"count": 3}},
		{"v7-x5", tool.Values{"version": "v7", "count": 5}},
		{"ulid-x3-upper", tool.Values{"version": "ulid", "count": 3, "uppercase": true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runUUID(t, "", tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "uuid", tt.name, res.Text)
		})
	}
}

var (
	v4Re   = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	v7Re   = regexp.MustCompile(`^019b796d-d600-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`) // 2026-01-01 12:00 UTC
	ulidRe = regexp.MustCompile(`^01KDWPVNG0[0-9A-HJKMNP-TV-Z]{16}$`)
)

func TestUUIDGenerate(t *testing.T) {
	tests := []struct {
		name string
		opts tool.Values
		re   *regexp.Regexp
		n    int
	}{
		{"v4 format and version", tool.Values{"count": 50}, v4Re, 50},
		{"v7 timestamp and version", tool.Values{"version": "v7", "count": 50}, v7Re, 50},
		{"ulid timestamp", tool.Values{"version": "ulid", "count": 50, "uppercase": true}, ulidRe, 50},
		{"maximum 1000", tool.Values{"count": 1000}, v4Re, 1000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runUUID(t, "", tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			ids := strings.Split(res.Text, "\n")
			if len(ids) != tt.n {
				t.Fatalf("generated %d, want %d", len(ids), tt.n)
			}
			for _, id := range ids {
				if !tt.re.MatchString(id) {
					t.Fatalf("%q does not match %s", id, tt.re)
				}
			}
			if len(slices.Compact(slices.Sorted(slices.Values(ids)))) != len(ids) {
				t.Error("there are repeated IDs")
			}
		})
	}
}

// v7 and ULID values from the same batch (same millisecond) come out sorted.
func TestUUIDSortable(t *testing.T) {
	for _, version := range []string{"v7", "ulid"} {
		t.Run(version, func(t *testing.T) {
			res, err := runUUID(t, "", tool.Values{"version": version, "count": 200})
			if err != nil {
				t.Fatal(err)
			}
			ids := strings.Split(res.Text, "\n")
			if !slices.IsSorted(ids) {
				t.Error("the batch is not sorted")
			}
		})
	}
}

func TestUUIDInspect(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"v4", "f47ac10b-58cc-4372-a567-0e02b2c3d479", []string{"Version:   4 (random)", "Variant:   RFC4122"}},
		{"v7 with date", "019b796d-d600-7abc-8def-0123456789ab", []string{"Version:   7", "Timestamp: 2026-01-01 12:00:00.000 UTC (now)"}},
		{"v1 with date", "c232ab00-9414-11ec-b3c8-9f6bdeced846", []string{"Version:   1", "Timestamp: 2022-02-22 19:22:22.000 UTC"}},
		{"uppercase and braces", "{F47AC10B-58CC-4372-A567-0E02B2C3D479}", []string{"UUID:      f47ac10b-58cc-4372-a567-0e02b2c3d479"}},
		{"urn", "urn:uuid:f47ac10b-58cc-4372-a567-0e02b2c3d479", []string{"Version:   4"}},
		{"nil", "00000000-0000-0000-0000-000000000000", []string{"Nil UUID"}},
		{"max", "ffffffff-ffff-ffff-ffff-ffffffffffff", []string{"Max UUID"}},
		{"ulid", "01KDWPVNG0ABCDEFGHJKMNPQRS", []string{"Timestamp: 2026-01-01 12:00:00.000 UTC (now)", "As UUID:"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runUUID(t, "  "+tt.in+"\n", nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tt.want {
				if !strings.Contains(res.Text, w) {
					t.Errorf("missing %q in:\n%s", w, res.Text)
				}
			}
		})
	}
}

func TestUUIDInspectErrors(t *testing.T) {
	for _, in := range []string{"hello", "f47ac10b-58cc-4372-a567", "01KDWPVNG0ABCDEFGHJKMNPQRSU"} {
		_, err := runUUID(t, in, nil)
		if _, ok := tool.AsInputError(err); !ok {
			t.Errorf("%q: error = %v, want *InputError", in, err)
		}
	}
}

func TestUUIDDetect(t *testing.T) {
	tests := []struct {
		sample string
		want   float64
	}{
		{"f47ac10b-58cc-4372-a567-0e02b2c3d479", 0.90},
		{" F47AC10B-58CC-4372-A567-0E02B2C3D479\n", 0.90},
		{"f47ac10b58cc4372a5670e02b2c3d479", 0},
		{"01KDWPVNG0ABCDEFGHJKMNPQRS", 0},
		{"hello", 0},
	}
	for _, tt := range tests {
		if got := (generators.UUID{}).Detect(tt.sample); got != tt.want {
			t.Errorf("Detect(%q) = %v, want %v", tt.sample, got, tt.want)
		}
	}
}
