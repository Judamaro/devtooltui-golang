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
		{"empty", " \n", nil, ""},
		{"text on one line", "<a><b>hello</b></a>", nil, "<a>\n  <b>hello</b>\n</a>"},
		{"empty element on one line", "<a><b></b></a>", nil, "<a>\n  <b></b>\n</a>"},
		{"self-closing", "<a><b/><c x='1' /></a>", nil, "<a>\n  <b/>\n  <c x='1' />\n</a>"},
		{"cdata only on one line", "<a><![CDATA[ <x> & ]]></a>", nil, "<a><![CDATA[ <x> & ]]></a>"},
		{"cdata among children", "<a><b/><![CDATA[x]]></a>", nil, "<a>\n  <b/>\n  <![CDATA[x]]>\n</a>"},
		{"quotes with >", `<a t="x>y"><b/></a>`, nil, "<a t=\"x>y\">\n  <b/>\n</a>"},
		{"minifies", "<a>\n  <b> x </b>\n</a>", tool.Values{"indent": 0}, "<a><b> x </b></a>"},
		{"mixed content", "<p>hello <b>world</b> end</p>", nil, "<p>\n  hello\n  <b>world</b>\n  end\n</p>"},
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
		name, in  string
		line, col int // col: where encoding/xml stopped (the end of the bad token)
	}{
		{"mismatched close", "<a>\n<b></c>\n</a>", 2, 8},
		{"unclosed", "<a>\n<b>", 2, 4},
		{"unquoted attribute", "<a x=1/>", 1, 7},
		{"text only", "hello", 1, 1},
		{"unknown entity", "<a>&nope;</a>", 1, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runXML(t, tt.in, nil)
			ie, ok := tool.AsInputError(err)
			if !ok {
				t.Fatalf("error = %v, want *InputError", err)
			}
			if ie.Line != tt.line || ie.Col != tt.col {
				t.Errorf("position = %d:%d, want %d:%d (%s)", ie.Line, ie.Col, tt.line, tt.col, ie.Msg)
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
		{"truncated sample", "<a><b>one</b><b>two", 0.85},
		{"no xml", "a < b", 0},
		{"comment only", "<!-- x -->", 0},
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

// FuzzXMLFormatIdempotent: formatting twice gives the same and the result
// stays well-formed.
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
					t.Fatalf("the output is not well-formed: %v\n%s", err, once.Text)
				}
				break
			}
		}
		twice, err := runXML(t, once.Text, opts)
		if err != nil {
			t.Fatalf("the output cannot be reformatted: %v", err)
		}
		if once.Text != twice.Text {
			t.Errorf("not idempotent:\n%s\n---\n%s", once.Text, twice.Text)
		}
	})
}
