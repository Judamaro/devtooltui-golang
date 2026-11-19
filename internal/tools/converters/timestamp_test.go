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

// 2026-01-01 12:00 UTC.
var tsNow = func() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) }

func runTS(t *testing.T, in string, opts tool.Values) (tool.Result, error) {
	t.Helper()
	return converters.Timestamp{Now: tsNow}.Run(context.Background(), tool.Request{
		Inputs: map[string]string{"input": in},
		Opts:   opts,
	})
}

func TestTimestampGolden(t *testing.T) {
	tests := []struct {
		name, in string
		opts     tool.Values
	}{
		{"seconds-utc", "seconds", tool.Values{"tz": "UTC"}},
		{"seconds-bogota", "seconds", tool.Values{"tz": "America/Bogota"}},
		{"millis-madrid", "millis", tool.Values{"tz": "Europe/Madrid"}},
		{"iso-utc", "iso", tool.Values{"tz": "UTC"}},
		{"rfc1123-tokyo", "rfc1123", tool.Values{"tz": "Asia/Tokyo"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runTS(t, testutil.ReadInput(t, "timestamp", tt.in), tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "timestamp", tt.name, res.Text)
		})
	}
}

// line devuelve el valor de la fila label de la salida.
func line(text, label string) string {
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(l, label+":") {
			return strings.TrimSpace(strings.TrimPrefix(l, label+":"))
		}
	}
	return ""
}

func TestTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		opts      tool.Values
		label     string
		want      string
		wantMeta0 string
	}{
		{"vacio es ahora", "", tool.Values{"tz": "UTC"}, "Unix (s)", "1767268800", "ahora"},
		{"auto segundos", "0", tool.Values{"tz": "UTC"}, "ISO 8601", "1970-01-01T00:00:00Z", "Unix en segundos"},
		{"auto ms", "1767225600000", tool.Values{"tz": "UTC"}, "Unix (s)", "1767225600", "Unix en milisegundos"},
		{"auto us", "1767225600000000", tool.Values{"tz": "UTC"}, "Unix (s)", "1767225600", "Unix en microsegundos"},
		{"auto ns", "1767225600000000000", tool.Values{"tz": "UTC"}, "Unix (s)", "1767225600", "Unix en nanosegundos"},
		{"unidad forzada", "1767225600", tool.Values{"tz": "UTC", "unit": "ms"}, "ISO 8601", "1970-01-21T10:53:45.600Z", "Unix en milisegundos"},
		{"negativo", "-86400", tool.Values{"tz": "UTC"}, "ISO 8601", "1969-12-31T00:00:00Z", "Unix en segundos"},
		{"decimal", "1.5", tool.Values{"tz": "UTC"}, "Unix (ms)", "1500", "Unix en segundos"},
		{"fecha sin zona usa tz", "2026-01-01 00:00", tool.Values{"tz": "America/Bogota"}, "Unix (s)", "1767243600", "fecha"},
		{"fecha sola", "2026-01-01", tool.Values{"tz": "UTC"}, "Relativo", "hace 12 h", "fecha"},
		{"tz minusculas", "0", tool.Values{"tz": "utc"}, "ISO 8601", "1970-01-01T00:00:00Z", "Unix en segundos"},
		{"dia en espanol", "1767225600", tool.Values{"tz": "UTC"}, "Día", "jueves, semana 2026-W01, día 1 del año", "Unix en segundos"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runTS(t, tt.in, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if got := line(res.Text, tt.label); got != tt.want {
				t.Errorf("%s = %q, want %q\n%s", tt.label, got, tt.want, res.Text)
			}
			if res.Meta[0].Value != tt.wantMeta0 {
				t.Errorf("Entrada = %q, want %q", res.Meta[0].Value, tt.wantMeta0)
			}
		})
	}
}

func TestTimestampErrors(t *testing.T) {
	tests := []struct {
		name, in  string
		opts      tool.Values
		wantInput string
	}{
		{"texto", "mañana", nil, "input"},
		{"fuera de rango", "99999999999999999999999", tool.Values{"unit": "s"}, "input"},
		{"zona desconocida", "0", tool.Values{"tz": "Marte/Olympus"}, "tz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runTS(t, tt.in, tt.opts)
			ie, ok := tool.AsInputError(err)
			if !ok {
				t.Fatalf("error = %v, want *InputError", err)
			}
			if ie.Input != tt.wantInput {
				t.Errorf("Input = %q, want %q", ie.Input, tt.wantInput)
			}
		})
	}
}

func TestTimestampDetect(t *testing.T) {
	tests := []struct {
		name, sample string
		want         float64
	}{
		{"segundos", "1767225600", 0.70},
		{"milisegundos", " 1767225600123\n", 0.70},
		{"anio 1990", "0631152000", 0},
		{"anio 2200", "7258118400", 0},
		{"11 digitos", "17672256000", 0},
		{"negativo", "-176722560", 0},
		{"texto", "hola", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (converters.Timestamp{}).Detect(tt.sample); got != tt.want {
				t.Errorf("Detect(%q) = %v, want %v", tt.sample, got, tt.want)
			}
		})
	}
}

func FuzzTimestamp(f *testing.F) {
	for _, s := range []string{"0", "1767225600", "-1.5", "2026-01-01", "1e400", "99999999999999999999"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		_ = (converters.Timestamp{}).Detect(in)
		if _, err := runTS(t, in, tool.Values{"tz": "UTC"}); err != nil {
			if _, ok := tool.AsInputError(err); !ok {
				t.Fatalf("error que no es *InputError: %v", err)
			}
		}
	})
}
