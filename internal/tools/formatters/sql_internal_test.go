package formatters

import (
	"context"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// FuzzSQLKeepsTokens: the formatted SQL has the same tokens as the input,
// in the same order (keywords compared without case).
func FuzzSQLKeepsTokens(f *testing.F) {
	f.Add("select a, b from t where x = 1 and y between 1 and 2 order by a desc")
	f.Add("with x as (select 1) select * from x; insert into t (a) values (1) on conflict (a) do update set a = 2")
	f.Add("select 'a''b', \"q\", $$d$$ -- c\nfrom t /* b */ where a::int > ? and b ->> 'k' = $1")
	f.Fuzz(func(t *testing.T, in string) {
		before, err := tokenizeSQL(in)
		if err != nil || checkParens(in, before) != nil {
			return
		}
		res, err := SQLFormat{}.Run(context.Background(), tool.Request{Inputs: map[string]string{"input": in}})
		if err != nil {
			t.Fatalf("Run(%q): %v", in, err)
		}
		after, err := tokenizeSQL(res.Text)
		if err != nil {
			t.Fatalf("the output does not tokenize: %v\n%s", err, res.Text)
		}
		norm := func(ts []sqlToken) []string {
			out := make([]string, len(ts))
			for i, tk := range ts {
				out[i] = tk.text
				if tk.kind == sqlWord {
					out[i] = strings.ToUpper(tk.text)
				}
			}
			return out
		}
		a, b := norm(before), norm(after)
		if strings.Join(a, "\x00") != strings.Join(b, "\x00") {
			t.Fatalf("tokens changed:\n%q\n%q\ninput %q\noutput %q", a, b, in, res.Text)
		}
	})
}
