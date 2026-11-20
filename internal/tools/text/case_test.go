package text_test

import (
	"context"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/text"
)

func runCase(t *testing.T, in string, opts tool.Values) string {
	t.Helper()
	res, err := text.Case{}.Run(context.Background(), tool.Request{Inputs: map[string]string{"input": in}, Opts: opts})
	if err != nil {
		t.Fatal(err)
	}
	return res.Text
}

func TestCaseGolden(t *testing.T) {
	in := testutil.ReadInput(t, "case", "identifiers")
	for _, to := range []string{"camel", "snake", "kebab", "title"} {
		t.Run(to, func(t *testing.T) {
			testutil.Golden(t, "case", "identifiers-"+to, runCase(t, in, tool.Values{"to": to}))
		})
	}
}

func TestCase(t *testing.T) {
	tests := []struct {
		in, to string
		digits bool
		want   string
	}{
		{"HTTPServer2", "snake", true, "http_server_2"},
		{"HTTPServer2", "snake", false, "http_server2"},
		{"HTTPServer2", "camel", true, "httpServer2"},
		{"getUserByID", "snake", true, "get_user_by_id"},
		{"getUserByID", "pascal", true, "GetUserById"},
		{"user_id", "camel", true, "userId"},
		{"user_id", "pascal", true, "UserId"},
		{"parse-xml-document", "constant", true, "PARSE_XML_DOCUMENT"},
		{"SOME_CONSTANT", "kebab", true, "some-constant"},
		{"some value", "dot", true, "some.value"},
		{"v2ApiClient", "snake", true, "v_2_api_client"},
		{"v2ApiClient", "snake", false, "v2_api_client"},
		{"base64Encode", "kebab", false, "base64-encode"},
		{"École élémentaire", "camel", true, "écoleÉlémentaire"},
		{"  --  ", "snake", true, ""},
		{"hello WORLD, again", "title", true, "Hello World, Again"},
		{"don't STOP", "title", true, "Don't Stop"},
		{"rock’n’roll", "title", true, "Rock’n’roll"},
		{"hello WORLD. Again", "sentence", true, "Hello world. again"},
		{"getUserByID", "title", true, "Get User By Id"},
		{"parse-xml-document", "sentence", true, "Parse xml document"},
		{"Mixed Case", "lower", true, "mixed case"},
		{"Mixed Case", "upper", true, "MIXED CASE"},
	}
	for _, tt := range tests {
		t.Run(tt.in+"→"+tt.to, func(t *testing.T) {
			if got := runCase(t, tt.in, tool.Values{"to": tt.to, "split_digits": tt.digits}); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCaseKeepsLines(t *testing.T) {
	got := runCase(t, "fooBar\r\n\n    bazQux\n", tool.Values{"to": "snake"})
	if want := "foo_bar\r\n\n    baz_qux\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
