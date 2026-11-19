package converters

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata" // zonas IANA embebidas: no depende del sistema (Windows, binario estático)

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Timestamp convierte entre timestamps Unix y fechas. Now permite fijar el
// reloj en los tests; si es nil se usa time.Now.
type Timestamp struct {
	Now func() time.Time
}

func (Timestamp) Spec() tool.Spec {
	return tool.Spec{
		ID:          "timestamp",
		Name:        "Timestamp",
		Category:    tool.Converters,
		Description: "Convierte timestamps Unix ↔ fechas (ISO 8601, RFC 1123)",
		Keywords:    []string{"unix", "epoch", "fecha", "date", "time", "iso8601", "rfc3339"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Timestamp o fecha (vacío = ahora)"}},
		Options: []tool.Option{
			{Key: "unit", Label: "Unidad", Help: "auto: según la cantidad de dígitos", Kind: tool.Select,
				Choices: []string{"auto", "s", "ms", "us", "ns"}, Default: "auto"},
			{Key: "tz", Label: "Zona", Help: "Local, UTC o un nombre IANA (America/Bogota)", Kind: tool.String, Default: "Local"},
		},
		Output: tool.PlainText,
		Live:   true,
	}
}

var numberRe = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

var dateLayouts = []string{
	time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04:05",
	"2006-01-02 15:04", "2006-01-02", time.RFC1123, time.RFC1123Z, time.RFC822, time.RFC822Z,
	time.RFC850, time.ANSIC, time.UnixDate,
}

var weekdays = [...]string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"}

var unitNames = map[string]string{"s": "segundos", "ms": "milisegundos", "us": "microsegundos", "ns": "nanosegundos"}

func (ts Timestamp) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(ts.Spec())
	loc, err := loadZone(opts.String("tz"))
	if err != nil {
		return tool.Result{}, &tool.InputError{Input: "tz", Msg: fmt.Sprintf("zona desconocida %q", opts.String("tz"))}
	}
	now := time.Now
	if ts.Now != nil {
		now = ts.Now
	}

	in := strings.TrimSpace(req.Inputs["input"])
	var t time.Time
	var source string
	switch {
	case in == "":
		t, source = now(), "ahora"
	case numberRe.MatchString(in):
		unit := opts.String("unit")
		if unit == "auto" {
			unit = guessUnit(in)
		}
		t, err = fromUnix(in, unit)
		if err != nil {
			return tool.Result{}, &tool.InputError{Input: "input", Line: 1, Col: 1, Msg: err.Error()}
		}
		source = "Unix en " + unitNames[unit]
	default:
		t, err = parseDate(in, loc)
		if err != nil {
			return tool.Result{}, &tool.InputError{Input: "input", Line: 1, Col: 1, Msg: "no es un número ni una fecha reconocida (ISO 8601, RFC 1123…)"}
		}
		source = "fecha"
	}

	lt := t.In(loc)
	isoYear, week := lt.ISOWeek()
	zone := loc.String()
	if abbr, _ := lt.Zone(); abbr != "" && abbr != zone && !strings.ContainsAny(abbr, "+-") {
		zone += ", " + abbr
	}
	iso := time.RFC3339
	if t.Nanosecond() != 0 {
		iso = "2006-01-02T15:04:05.000Z07:00"
	}
	rows := [][2]string{
		{"Unix (s)", fmt.Sprint(t.Unix())},
		{"Unix (ms)", fmt.Sprint(t.UnixMilli())},
		{"ISO 8601", lt.Format(iso)},
		{"RFC 1123", lt.Format(time.RFC1123)},
		{"UTC", t.UTC().Format("2006-01-02 15:04:05")},
		{"Hora local", lt.Format("2006-01-02 15:04:05 -07:00") + " (" + zone + ")"},
		{"Relativo", Relative(t.Sub(now()))},
		{"Día", fmt.Sprintf("%s, semana %d-W%02d, día %d del año", weekdays[lt.Weekday()], isoYear, week, lt.YearDay())},
	}
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%-11s %s\n", r[0]+":", r[1])
	}
	return tool.Result{
		Text: strings.TrimRight(b.String(), "\n"),
		Meta: []tool.KV{{Key: "Entrada", Value: source}, {Key: "Zona", Value: loc.String()}},
	}, nil
}

func loadZone(name string) (*time.Location, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "local":
		return time.Local, nil
	case "utc", "z":
		return time.UTC, nil
	}
	return time.LoadLocation(strings.TrimSpace(name))
}

// guessUnit elige la unidad por la cantidad de dígitos de la parte entera.
func guessUnit(s string) string {
	digits := len(strings.TrimPrefix(strings.SplitN(s, ".", 2)[0], "-"))
	switch {
	case digits <= 11:
		return "s"
	case digits <= 14:
		return "ms"
	case digits <= 17:
		return "us"
	default:
		return "ns"
	}
}

// fromUnix usa big.Float para no perder precisión ni desbordar.
func fromUnix(s, unit string) (time.Time, error) {
	f, _, err := big.ParseFloat(s, 10, 128, big.ToNearestEven)
	if err != nil {
		return time.Time{}, err
	}
	perSecond := map[string]float64{"s": 1, "ms": 1e3, "us": 1e6, "ns": 1e9}[unit]
	ns := new(big.Float).Mul(f, big.NewFloat(1e9/perSecond))
	n, _ := ns.Int(nil)
	if !n.IsInt64() {
		return time.Time{}, fmt.Errorf("fuera de rango para %s", unitNames[unit])
	}
	sec := n.Int64() / 1e9
	if math.Abs(float64(sec)) > 1e11 { // ±3000 años aprox.
		return time.Time{}, fmt.Errorf("fuera de rango para %s", unitNames[unit])
	}
	return time.Unix(sec, n.Int64()%1e9), nil
}

func parseDate(s string, loc *time.Location) (time.Time, error) {
	for _, layout := range dateLayouts {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("formato desconocido")
}

// Detect reconoce 10 o 13 dígitos que caen entre los años 2000 y 2100.
func (Timestamp) Detect(sample string) float64 {
	s := strings.TrimSpace(sample)
	if (len(s) != 10 && len(s) != 13) || !numberRe.MatchString(s) || strings.ContainsAny(s, "-.") {
		return 0
	}
	t, err := fromUnix(s, map[int]string{10: "s", 13: "ms"}[len(s)])
	// UTC: t.Year() on a Local time loads the system zone the first time,
	// which on Windows (with -race) alone exceeded the 20 ms detect budget.
	if err != nil || t.UTC().Year() < 2000 || t.UTC().Year() >= 2100 {
		return 0
	}
	return 0.70
}
