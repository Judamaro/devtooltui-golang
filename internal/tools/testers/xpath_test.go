package testers_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/testers"
)

func runXPath(t testing.TB, in, expr string) (tool.Result, error) {
	t.Helper()
	return testers.XPath{}.Run(context.Background(), tool.Request{
		Inputs: map[string]string{"input": in},
		Opts:   tool.Values{"expr": expr},
	})
}

func TestXPathGolden(t *testing.T) {
	in := testutil.ReadInput(t, "xpath", "library")
	for _, tt := range []struct{ name, expr string }{
		{"library-cheap-books", "//book[price < 10]"},
		{"library-titles", "//title/text()"},
		{"library-ids", "//book/@id"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runXPath(t, in, tt.expr)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "xpath", tt.name, res.Text)
		})
	}
}

func TestXPath(t *testing.T) {
	in := testutil.ReadInput(t, "xpath", "library")
	tests := []struct {
		expr, want, lang, meta string
	}{
		{"count(//book)", "3", "", "number"},
		{"sum(//price)", "30.93", "", "number"},
		{"string(//book[@id='b2']/title)", "Cien años de soledad", "", "string"},
		{"boolean(//book[@lang='fr'])", "false", "", "boolean"},
		{"//book[@lang='es']/title", "<title>Cien años de soledad</title>", "xml", "1"},
		{"//nothing", "", "", "0"},
		{"//comment()", " prices in EUR ", "", "1"},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			res, err := runXPath(t, in, tt.expr)
			if err != nil {
				t.Fatal(err)
			}
			if res.Text != tt.want || res.Lang != tt.lang || res.Meta[0].Value != tt.meta {
				t.Errorf("got %q (lang %q, meta %v), want %q (lang %q, %s)", res.Text, res.Lang, res.Meta, tt.want, tt.lang, tt.meta)
			}
		})
	}
	if res, err := runXPath(t, "  ", "//a"); err != nil || res.Text != "" {
		t.Errorf("empty input = %q, %v", res.Text, err)
	}
}

func TestXPathErrors(t *testing.T) {
	tests := []struct {
		name, in, expr string
		input          string
		line, col      int
		msg            string
	}{
		{"bad expression", "<a/>", "  //a[", "expr", 1, 3, ""},
		{"unknown function", "<a/>", "nope(//a)", "expr", 1, 1, ""},
		{"bad xml", "<a>\n<b></c>\n</a>", "//b", "input", 2, 8, "closed by"},
		{"no elements", "just text", "//a", "input", 1, 1, "no elements"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runXPath(t, tt.in, tt.expr)
			ie, ok := tool.AsInputError(err)
			if !ok {
				t.Fatalf("error = %v, want *InputError", err)
			}
			if ie.Input != tt.input || ie.Line != tt.line || ie.Col != tt.col || !strings.Contains(ie.Msg, tt.msg) {
				t.Errorf("error = %s %d:%d %q, want %s %d:%d %q", ie.Input, ie.Line, ie.Col, ie.Msg, tt.input, tt.line, tt.col, tt.msg)
			}
		})
	}
}

// FuzzXPath checks that no document or expression makes Run panic or return
// an error that is not an InputError.
func FuzzXPath(f *testing.F) {
	f.Add("<a><b x='1'>t</b></a>", "//b/@x")
	f.Add("<a/>", "count(//*)")
	f.Add("<r><i>1</i><i>2</i></r>", "//i[. > 1]")
	f.Fuzz(func(t *testing.T, in, expr string) {
		_, err := runXPath(t, in, expr)
		if err != nil {
			if _, ok := tool.AsInputError(err); !ok {
				t.Fatalf("error that is not *InputError: %v", err)
			}
		}
	})
}
