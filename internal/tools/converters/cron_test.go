package converters_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/converters"
)

// cronNow is a Friday.
var cronNow = func() time.Time { return time.Date(2026, 11, 20, 8, 15, 0, 0, time.UTC) }

func runCron(t *testing.T, in string, opts tool.Values) (tool.Result, error) {
	t.Helper()
	return converters.Cron{Now: cronNow}.Run(context.Background(), tool.Request{Inputs: map[string]string{"input": in}, Opts: opts})
}

func TestCronGolden(t *testing.T) {
	for _, tt := range []struct {
		name, in string
		opts     tool.Values
	}{
		{"weekdays-0930", "30 9 * * 1-5", nil},
		{"every-15-bogota", "*/15 * * * *", tool.Values{"tz": "America/Bogota", "count": 5}},
		{"seconds", "*/10 0 12 * * MON", tool.Values{"seconds": true, "count": 3}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runCron(t, tt.in, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "cron", tt.name, res.Text)
		})
	}
}

func TestCronDescribe(t *testing.T) {
	tests := []struct {
		in   string
		secs bool
		want string
	}{
		{"* * * * *", false, "Every minute"},
		{"*/5 * * * *", false, "Every 5 minutes"},
		{"30 9 * * 1-5", false, "At 09:30, Monday through Friday"},
		{"0 0 1 * *", false, "At 00:00, on day 1 of the month"},
		{"0 12 * 1,6 *", false, "At 12:00, in January and June"},
		{"0 12 * JAN-MAR *", false, "At 12:00, January through March"},
		{"15 */2 * * *", false, "At minute 15, past every 2nd hour"},
		{"0,30 9-17 * * *", false, "At minutes 0 and 30, from 09:00 through 17:59"},
		{"0 9,17 * * *", false, "At minute 0, past hours 9 and 17"},
		{"0-29 * * * *", false, "Every minute from 0 through 29"},
		{"10-50/10 * * * *", false, "Every 10 minutes from 10 through 50"},
		{"0 8 1,15 * MON,WED,FRI", false, "At 08:00, on days 1 and 15 of the month or on Monday, Wednesday and Friday"},
		{"0 8 */2 * *", false, "At 08:00, every 2nd day of the month"},
		{"0 0 * */3 *", false, "At 00:00, every 3rd month"},
		{"0 22 * * 0", false, "At 22:00, on Sunday"},
		{"@daily", false, "At 00:00"},
		{"@every 1h30m", false, "Every 1h30m"},
		{"5 30 9 * * *", true, "At 09:30:05"},
		{"*/10 * * * * *", true, "Every 10 seconds, every minute"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			res, err := runCron(t, tt.in, tool.Values{"seconds": tt.secs})
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.SplitN(res.Text, "\n", 2)[0]; got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCronNextRuns(t *testing.T) {
	res, err := runCron(t, "30 9 * * 1-5", tool.Values{"count": 2})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Next runs (UTC):", "  Fri 2026-11-20 09:30  (in 1 h)", "  Mon 2026-11-23 09:30  (in 3 d)"} {
		if !strings.Contains(res.Text+"\n", want+"\n") {
			t.Errorf("missing %q in:\n%s", want, res.Text)
		}
	}
	if strings.Count(res.Text, "\n  ") != 2 {
		t.Errorf("want 2 runs:\n%s", res.Text)
	}
	if res, _ := runCron(t, "", nil); res.Text != "" {
		t.Errorf("empty input gave %q", res.Text)
	}
}

func TestCronErrors(t *testing.T) {
	tests := []struct {
		in    string
		opts  tool.Values
		input string
		col   int
		msg   string
	}{
		{"60 * * * *", nil, "input", 1, "minute field"},
		{"0 25 * * *", nil, "input", 3, "hour field"},
		{"  0 0 * FOO *", nil, "input", 9, "month field"},
		{"0 0 * * 9", nil, "input", 9, "day of week field"},
		{"0 0 * *", nil, "input", 1, "expected 5 fields, got 4"},
		{"* * * * *", tool.Values{"seconds": true}, "input", 1, "expected 6 fields, got 5"},
		{"@sometimes", nil, "input", 1, "descriptor"},
		{"* * * * *", tool.Values{"tz": "Mars/Base"}, "tz", 1, "unknown zone"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			_, err := runCron(t, tt.in, tt.opts)
			ie, ok := tool.AsInputError(err)
			if !ok {
				t.Fatalf("error = %v, want *InputError", err)
			}
			if ie.Input != tt.input || ie.Line != 1 || ie.Col != tt.col || !strings.Contains(ie.Msg, tt.msg) {
				t.Errorf("error = %s %d:%d %q, want %s 1:%d %q", ie.Input, ie.Line, ie.Col, ie.Msg, tt.input, tt.col, tt.msg)
			}
		})
	}
}

func TestCronDetect(t *testing.T) {
	tests := []struct {
		sample string
		want   float64
	}{
		{"*/5 * * * *", 0.6},
		{"30 9 * * 1-5", 0.6},
		{"@daily", 0.6},
		{"1 2 3 4", 0},
		{"60 * * * *", 0},
		{"hello world how are you", 0},
		{"* * * * *\n* * * * *", 0},
	}
	for _, tt := range tests {
		if got := (converters.Cron{}).Detect(tt.sample); got != tt.want {
			t.Errorf("Detect(%q) = %v, want %v", tt.sample, got, tt.want)
		}
	}
}

func FuzzCron(f *testing.F) {
	for _, s := range []string{"* * * * *", "*/5 1-3 * JAN-MAR MON,FRI", "@every 5m", "0 0 31 2 *"} {
		f.Add(s, false)
	}
	f.Fuzz(func(t *testing.T, s string, secs bool) {
		_, err := runCron(t, s, tool.Values{"seconds": secs, "count": 3})
		if err != nil {
			if _, ok := tool.AsInputError(err); !ok {
				t.Fatalf("error that is not *InputError: %v", err)
			}
		}
	})
}
