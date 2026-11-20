package converters

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/robfig/cron/v3"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Cron explains a cron expression and lists its next runs. Now pins the
// clock in tests; when nil, time.Now is used.
type Cron struct {
	Now func() time.Time
}

func (Cron) Spec() tool.Spec {
	return tool.Spec{
		ID:          "cron",
		Name:        "Cron",
		Category:    tool.Converters,
		Description: "Explain a cron expression and list its next runs",
		Keywords:    []string{"crontab", "schedule", "job", "quartz", "@daily"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Expression (min hour day month weekday)"}},
		Options: []tool.Option{
			{Key: "seconds", Label: "Seconds field", Help: "6 fields: the first one is seconds", Kind: tool.Toggle, Default: false},
			{Key: "tz", Label: "Zone", Help: "Local, UTC or an IANA name (America/Bogota)", Kind: tool.String, Default: "Local"},
			{Key: "count", Label: "Next runs", Kind: tool.Number, Min: 1, Max: 50, Default: 10},
		},
		Output: tool.PlainText,
		Live:   true,
	}
}

const (
	stdFields = cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor
	secFields = cron.Second | stdFields
)

func (c Cron) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(c.Spec())
	in := req.Inputs["input"]
	expr := strings.TrimSpace(in)
	if expr == "" {
		return tool.Result{}, nil
	}
	loc, err := loadZone(opts.String("tz"))
	if err != nil {
		return tool.Result{}, &tool.InputError{Input: "tz", Line: 1, Col: 1, Msg: fmt.Sprintf("unknown zone %q", opts.String("tz"))}
	}
	withSeconds := opts.Bool("seconds")
	sched, err := parseCron(expr, withSeconds)
	if err != nil {
		return tool.Result{}, cronError(in, expr, withSeconds, err)
	}

	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	t0 := now().In(loc)
	layout := "Mon 2006-01-02 15:04"
	if withSeconds {
		layout += ":05"
	}
	rows := []string{describeCron(expr, withSeconds), "", fmt.Sprintf("Next runs (%s):", loc)}
	t := t0
	count := min(max(opts.Int("count"), 1), 50)
	for range count {
		t = sched.Next(t)
		if t.IsZero() {
			rows = append(rows, "  (no more runs)")
			break
		}
		rows = append(rows, fmt.Sprintf("  %s  (%s)", t.Format(layout), Relative(t.Sub(t0))))
	}
	return tool.Result{
		Text: strings.Join(rows, "\n"),
		Meta: []tool.KV{{Key: "Zone", Value: loc.String()}},
	}, nil
}

func parseCron(expr string, withSeconds bool) (cron.Schedule, error) {
	fields := stdFields
	if withSeconds {
		fields = secFields
	}
	return cron.NewParser(fields).Parse(expr)
}

// cronError points at the field that fails: robfig gives no position, so
// each field is replaced by "*" until the expression parses.
func cronError(in, expr string, withSeconds bool, err error) error {
	lead := strings.Index(in, expr)
	ie := &tool.InputError{Input: "input", Line: 1, Col: 1, Msg: err.Error()}
	fields, starts := splitFields(expr)
	want := 5
	if withSeconds {
		want = 6
	}
	if strings.HasPrefix(expr, "@") || len(fields) != want {
		if !strings.HasPrefix(expr, "@") {
			ie.Msg = fmt.Sprintf("expected %d fields, got %d", want, len(fields))
		}
		_, ie.Col = tool.Position(in, lead)
		return ie
	}
	for i := range fields {
		try := append([]string(nil), fields...)
		try[i] = "*"
		if _, perr := parseCron(strings.Join(try, " "), withSeconds); perr == nil {
			ie.Msg = fmt.Sprintf("%s field %q: %v", fieldNames(withSeconds)[i], fields[i], err)
			ie.Line, ie.Col = tool.Position(in, lead+starts[i])
			break
		}
	}
	return ie
}

// splitFields splits like strings.Fields (as robfig does) and keeps the
// byte offset of each field.
func splitFields(s string) (fields []string, starts []int) {
	start := -1
	for i, r := range s {
		if unicode.IsSpace(r) {
			if start >= 0 {
				fields = append(fields, s[start:i])
				starts = append(starts, start)
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		fields = append(fields, s[start:])
		starts = append(starts, start)
	}
	return fields, starts
}

func fieldNames(withSeconds bool) []string {
	n := []string{"minute", "hour", "day of month", "month", "day of week"}
	if withSeconds {
		return append([]string{"second"}, n...)
	}
	return n
}

var cronDescriptors = map[string]string{
	"@yearly":   "At 00:00 on January 1",
	"@annually": "At 00:00 on January 1",
	"@monthly":  "At 00:00 on day 1 of the month",
	"@weekly":   "At 00:00 on Sunday",
	"@daily":    "At 00:00",
	"@midnight": "At 00:00",
	"@hourly":   "At minute 0 of every hour",
}

var (
	monthNames = []string{"", "January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
	dayNames   = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
)

// describeCron explains an expression already validated by robfig.
func describeCron(expr string, withSeconds bool) string {
	if d, ok := cronDescriptors[strings.ToLower(expr)]; ok {
		return d
	}
	if rest, ok := strings.CutPrefix(expr, "@every "); ok {
		return "Every " + strings.TrimSpace(rest)
	}
	f, _ := splitFields(expr)
	sec := "0"
	if withSeconds {
		sec, f = f[0], f[1:]
	}
	minute, hour, dom, month, dow := f[0], f[1], f[2], f[3], f[4]

	var parts []string
	if s, okS := single(sec, nil); okS {
		m, okM := single(minute, nil)
		h, okH := single(hour, nil)
		if okM && okH {
			at := fmt.Sprintf("At %02d:%02d", h, m)
			if s != 0 {
				at += fmt.Sprintf(":%02d", s)
			}
			parts = append(parts, at)
		} else {
			if s != 0 {
				parts = append(parts, fmt.Sprintf("at second %d", s))
			}
			parts = append(parts, minutePhrase(minute))
			if hp := hourPhrase(hour); hp != "" {
				parts = append(parts, hp)
			}
		}
	} else {
		parts = append(parts, unitPhrase(sec, "second"), minutePhrase(minute))
		if hp := hourPhrase(hour); hp != "" {
			parts = append(parts, hp)
		}
	}

	var days []string
	if dom != "*" && dom != "?" {
		days = append(days, domPhrase(dom))
	}
	if dow != "*" && dow != "?" {
		days = append(days, namesPhrase(dow, dayNames, "on "))
	}
	if len(days) > 0 {
		parts = append(parts, strings.Join(days, " or ")) // cron runs when either matches
	}
	if month != "*" && month != "?" {
		parts = append(parts, namesPhrase(month, monthNames, "in "))
	}
	out := strings.Join(parts, ", ")
	return strings.ToUpper(out[:1]) + out[1:]
}

// single returns the value of a field with one plain number (or name).
func single(f string, names []string) (int, bool) {
	if v, ok := value(f, names); ok && !strings.ContainsAny(f, ",-/*?") {
		return v, true
	}
	return 0, false
}

func value(s string, names []string) (int, bool) {
	if n, err := strconv.Atoi(s); err == nil {
		return n, true
	}
	for i, name := range names {
		if name != "" && strings.EqualFold(s, name[:3]) {
			return i, true
		}
	}
	return 0, false
}

func minutePhrase(f string) string {
	if v, ok := single(f, nil); ok {
		return fmt.Sprintf("at minute %d", v)
	}
	return unitPhrase(f, "minute")
}

func hourPhrase(f string) string {
	switch {
	case f == "*":
		return ""
	case strings.HasPrefix(f, "*/"):
		return "past every " + ordinal(f[2:]) + " hour"
	}
	if lo, hi, ok := rangeOf(f); ok {
		return fmt.Sprintf("from %02d:00 through %02d:59", lo, hi)
	}
	return "past " + listPhrase(f, "hour", nil)
}

func domPhrase(f string) string {
	if strings.HasPrefix(f, "*/") {
		return "every " + ordinal(f[2:]) + " day of the month"
	}
	if lo, hi, ok := rangeOf(f); ok {
		return fmt.Sprintf("on days %d through %d of the month", lo, hi)
	}
	return "on " + listPhrase(f, "day", nil) + " of the month"
}

// unitPhrase covers *, */n, a-b, a-b/n and lists for seconds and minutes.
func unitPhrase(f, unit string) string {
	switch {
	case f == "*":
		return "every " + unit
	case strings.HasPrefix(f, "*/"):
		return "every " + f[2:] + " " + unit + "s"
	}
	if base, step, ok := strings.Cut(f, "/"); ok {
		if lo, hi, isRange := rangeOf(base); isRange {
			return fmt.Sprintf("every %s %ss from %d through %d", step, unit, lo, hi)
		}
		return fmt.Sprintf("every %s %ss starting at %s %s", step, unit, unit, base)
	}
	if lo, hi, ok := rangeOf(f); ok {
		return fmt.Sprintf("every %s from %d through %d", unit, lo, hi)
	}
	return "at " + listPhrase(f, unit, nil)
}

// namesPhrase describes months or weekdays: "in January and June",
// "Monday through Friday", "every 3rd month".
func namesPhrase(f string, names []string, prefix string) string {
	if strings.HasPrefix(f, "*/") {
		unit := "day"
		if len(names) > 8 {
			unit = "month"
		}
		return "every " + ordinal(f[2:]) + " " + unit
	}
	if lo, hi, ok := rangeNames(f, names); ok {
		return names[lo] + " through " + names[hi]
	}
	var items []string
	for _, p := range strings.Split(f, ",") {
		if lo, hi, ok := rangeNames(p, names); ok {
			items = append(items, names[lo]+" through "+names[hi])
		} else if v, ok := value(p, names); ok && v < len(names) {
			items = append(items, names[v])
		} else {
			items = append(items, p)
		}
	}
	return prefix + joinAnd(items)
}

// listPhrase: "minute 5", "minutes 0, 15 and 30", "hours 9 through 12 and 18".
func listPhrase(f, unit string, names []string) string {
	items := strings.Split(f, ",")
	for i, it := range items {
		if lo, hi, ok := rangeOf(it); ok {
			items[i] = fmt.Sprintf("%d through %d", lo, hi)
		}
	}
	if len(items) == 1 && !strings.Contains(items[0], "through") {
		return unit + " " + items[0]
	}
	return unit + "s " + joinAnd(items)
}

func rangeOf(f string) (lo, hi int, ok bool) {
	a, b, found := strings.Cut(f, "-")
	if !found || strings.ContainsAny(f, ",/") {
		return 0, 0, false
	}
	x, err1 := strconv.Atoi(a)
	y, err2 := strconv.Atoi(b)
	return x, y, err1 == nil && err2 == nil
}

func rangeNames(f string, names []string) (lo, hi int, ok bool) {
	a, b, found := strings.Cut(f, "-")
	if !found || strings.ContainsAny(f, ",/") {
		return 0, 0, false
	}
	x, ok1 := value(a, names)
	y, ok2 := value(b, names)
	return x, y, ok1 && ok2 && x < len(names) && y < len(names)
}

func ordinal(s string) string {
	n, err := strconv.Atoi(s)
	if err != nil {
		return s
	}
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s", n, suffix)
}

func joinAnd(items []string) string {
	if len(items) <= 1 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

// Detect recognises 5 cron fields that parse (and the @ descriptors).
func (Cron) Detect(sample string) float64 {
	s := strings.TrimSpace(sample)
	if strings.Contains(s, "\n") || len(s) > 100 {
		return 0
	}
	if f, _ := splitFields(s); len(f) != 5 && !strings.HasPrefix(s, "@") {
		return 0
	}
	if _, err := parseCron(s, false); err == nil {
		return 0.6
	}
	return 0
}
