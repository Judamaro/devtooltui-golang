package converters_test

import (
	"testing"
	"time"

	"github.com/Judamaro/devtooltui-golang/internal/tools/converters"
)

func TestRelative(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "ahora"},
		{500 * time.Millisecond, "ahora"},
		{45 * time.Second, "en 45 s"},
		{-90 * time.Second, "hace 1 min"},
		{3 * time.Hour, "en 3 h"},
		{-47 * time.Hour, "hace 47 h"},
		{-50 * time.Hour, "hace 2 d"},
		{400 * 24 * time.Hour, "en 400 d"},
		{-3 * 366 * 24 * time.Hour, "hace 3 años"},
	}
	for _, tt := range tests {
		t.Run(tt.d.String(), func(t *testing.T) {
			if got := converters.Relative(tt.d); got != tt.want {
				t.Errorf("Relative(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}
