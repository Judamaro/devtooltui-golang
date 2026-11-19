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
		{0, "now"},
		{500 * time.Millisecond, "now"},
		{45 * time.Second, "in 45 s"},
		{-90 * time.Second, "1 min ago"},
		{3 * time.Hour, "in 3 h"},
		{-47 * time.Hour, "47 h ago"},
		{-50 * time.Hour, "2 d ago"},
		{400 * 24 * time.Hour, "in 400 d"},
		{-3 * 366 * 24 * time.Hour, "3 years ago"},
	}
	for _, tt := range tests {
		t.Run(tt.d.String(), func(t *testing.T) {
			if got := converters.Relative(tt.d); got != tt.want {
				t.Errorf("Relative(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}
