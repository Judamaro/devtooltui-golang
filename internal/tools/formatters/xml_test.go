package formatters_test

import (
	"context"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/formatters"
)

func runXML(t testing.TB, in string, opts tool.Values) (tool.Result, error) {
	t.Helper()
	return formatters.XMLFormat{}.Run(context.Background(), tool.Request{
		Inputs: map[string]string{"input": in},
		Opts:   opts,
	})
}

func TestXMLFormatGolden(t *testing.T) {
	tests := []struct {
		name string
		opts tool.Values
	}{
		{"catalog-indent2", nil},
		{"catalog-indent4", tool.Values{"indent": 4}},
		{"catalog-minify", tool.Values{"indent": 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runXML(t, testutil.ReadInput(t, "xml-format", "catalog"), tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "xml-format", tt.name, res.Text)
		})
	}
}

func TestXMLFormat(t *testing.T) {
	tests := []struct {
		name, in string
		opts     tool.Values
		want     string
	}{
		{"vacio", " \n", nil, ""},
		{"texto en una linea", "<a><b>hola</b></a>", nil, "<a>\n  <b>hola</b>\n</a>"},
		{"elemento vacio en una linea", "<a><b></b></a>", nil, "<a>\n  <b></b>\n</a>"},
		{"autocerrado", "<a><b/><c x='1' /></a>", nil, "<a>\n  <b/>\n  <c x='1' />\n</a>"},
		{"cdata solo en una linea", "<a><![CDATA[ <x> & ]]></a>", nil, "<a><![CDATA[ <x> & ]]></a>"},
		{"cdata entre hijos", "<a><b/><![CDATA[x]]></a>", nil, "<a>\n  <b/>\n  <![CDATA[x]]>\n</a>"},
		{"comillas con >", `<a t="x>y"><b/></a>`, nil, "<a t=\"x>y\">\n  <b/>\n</a>"},
		{"minifica", "<a>\n  <b> x </b>\n</a>", tool.Values{"indent": 0}, "<a><b> x </b></a>"},
		{"contenido mixto", "<p>hola <b>mundo</b> fin</p>", nil, "<p>\n  hola\n  <b>mundo</b>\n  fin\n</p>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runXML(t, tt.in, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if res.Text != tt.want {
				t.Errorf("Text =\n%s\nwant\n%s", res.Text, tt.want)
			}
		})
	}
}

func TestXMLFormatErrors(t *testing.T) {
	tests := []struct {
		name, in string
		line     int
	}{
		{"cierre distinto", "<a>\n<b></c>\n</a>", 2},
		{"sin cerrar", "<a>\n<b>", 2},
		{"atributo sin comillas", "<a x=1/>", 1},
		{"solo texto", "hola", 1},
		{"entidad desconocida", "<a>&nope;</a>", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runXML(t, tt.in, nil)
			ie, ok := tool.AsInputError(err)
			if !ok {
				t.Fatalf("error = %v, want *InputError", err)
			}
			if ie.Line != tt.line {
				t.Errorf("línea = %d, want %d (%s)", ie.Line, tt.line, ie.Msg)
			}
		})
	}
}

func TestXMLFormatDetect(t *testing.T) {
	tests := []struct {
		name, sample string
		want         float64
	}{
		{"xml", `<?xml version="1.0"?><a/>`, 0.85},
		{"svg", `<svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`, 0.85},
		{"muestra cortada", "<a><b>uno</b><b>dos", 0.85},
		{"no xml", "a < b", 0},
		{"solo comentario", "<!-- x -->", 0},
		{"json", `{"a":1}`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (formatters.XMLFormat{}).Detect(tt.sample); got != tt.want {
				t.Errorf("Detect = %v, want %v", got, tt.want)
			}
		})
	}
}

// FuzzXMLFormatIdempotent: formatear dos veces da lo mismo y el resultado
// sigue bien formado.
func FuzzXMLFormatIdempotent(f *testing.F) {
	for _, s := range []string{"<a><b>x</b><!--c--><![CDATA[d]]></a>", `<a x="1>"/>`, "<?xml version=\"1.0\"?><r>\n t <i/> </r>"} {
		f.Add(s, 2)
	}
	f.Fuzz(func(t *testing.T, in string, indent int) {
		opts := tool.Values{"indent": ((indent % 9) + 9) % 9}
		once, err := runXML(t, in, opts)
		if err != nil {
			return
		}
		if strings.TrimSpace(in) == "" {
			return
		}
		dec := xml.NewDecoder(strings.NewReader(once.Text))
		dec.Strict = true
		for {
			if _, err := dec.Token(); err != nil {
				if err.Error() != "EOF" {
					t.Fatalf("la salida no está bien formada: %v\n%s", err, once.Text)
				}
				break
			}
		}
		twice, err := runXML(t, once.Text, opts)
		if err != nil {
			t.Fatalf("la salida no se puede reformatear: %v", err)
		}
		if once.Text != twice.Text {
			t.Errorf("no es idempotente:\n%s\n---\n%s", once.Text, twice.Text)
		}
	})
}
