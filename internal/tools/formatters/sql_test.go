package formatters_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/formatters"
)

func runSQL(t testing.TB, in string, opts tool.Values) (tool.Result, error) {
	t.Helper()
	return formatters.SQLFormat{}.Run(context.Background(), tool.Request{Inputs: map[string]string{"input": in}, Opts: opts})
}

func TestSQLFormatGolden(t *testing.T) {
	for _, name := range []string{"select", "cte", "dml"} {
		t.Run(name, func(t *testing.T) {
			res, err := runSQL(t, testutil.ReadInput(t, "sql-format", name), nil)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "sql-format", name, res.Text)
		})
	}
}

func TestSQLFormat(t *testing.T) {
	tests := []struct {
		name, in string
		opts     tool.Values
		want     string
	}{
		{"simple", "select a,b from t", nil, "SELECT\n  a,\n  b\nFROM\n  t"},
		{"lowercase", "SELECT A FROM T WHERE X IS NOT NULL", tool.Values{"uppercase": false}, "select\n  A\nfrom\n  T\nwhere\n  X is not null"},
		{"indent 4", "select a from t", tool.Values{"indent": 4}, "SELECT\n    a\nFROM\n    t"},
		{"strings and quoted names kept", `select "Order", [Group], 'it''s select', e'a\'b' from "my table"`, nil,
			"SELECT\n  \"Order\",\n  [Group],\n  'it''s select',\n  e'a\\'b'\nFROM\n  \"my table\""},
		{"dollar quoting", "select $$a; b$$, $fn$x$fn$", nil, "SELECT\n  $$a; b$$,\n  $fn$x$fn$"},
		{"params", "select * from t where a = ? and b = $1 and c = :name and d = @v", nil,
			"SELECT\n  *\nFROM\n  t\nWHERE\n  a = ?\n  AND b = $1\n  AND c = :name\n  AND d = @v"},
		{"casts and json", "select a::int, b->>'k', c #> '{x}' from t", nil, "SELECT\n  a::int,\n  b ->> 'k',\n  c #> '{x}'\nFROM\n  t"},
		{"case stays inline", "select case when a > 1 then 'x' else 'y' end as c from t", nil,
			"SELECT\n  CASE WHEN a > 1 THEN 'x' ELSE 'y' END AS c\nFROM\n  t"},
		{"union", "select a from t union all select a from u", nil, "SELECT\n  a\nFROM\n  t\nUNION ALL\nSELECT\n  a\nFROM\n  u"},
		{"window", "select row_number() over (partition by a order by b) from t", nil,
			"SELECT\n  row_number() OVER (PARTITION BY a ORDER BY b)\nFROM\n  t"},
		{"in list stays inline", "select * from t where id in (1,2,3)", nil, "SELECT\n  *\nFROM\n  t\nWHERE\n  id IN (1, 2, 3)"},
		{"exists subquery", "select 1 where exists (select 1 from t)", nil, "SELECT\n  1\nWHERE\n  EXISTS (\n    SELECT\n      1\n    FROM\n      t\n  )"},
		{"line comment at the end", "select a -- the a\nfrom t", nil, "SELECT\n  a -- the a\nFROM\n  t"},
		{"numbers", "select 1.5e-3, .5, 0x1F from t", nil, "SELECT\n  1.5e-3,\n  .5,\n  0x1F\nFROM\n  t"},
		{"statements", "select 1; select 2;", nil, "SELECT\n  1;\n\nSELECT\n  2;"},
		{"unicode identifiers", "select año, ñandú from países", nil, "SELECT\n  año,\n  ñandú\nFROM\n  países"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runSQL(t, tt.in, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if res.Text != tt.want {
				t.Errorf("got\n%s\nwant\n%s", res.Text, tt.want)
			}
		})
	}
}

func TestSQLFormatErrors(t *testing.T) {
	tests := []struct {
		name, in  string
		line, col int
		msg       string
	}{
		{"unterminated string", "select 'abc\nfrom t", 1, 8, "unterminated string"},
		{"unterminated comment", "select 1 /* note", 1, 10, "unterminated comment"},
		{"unterminated identifier", "select \"abc", 1, 8, "unterminated quoted identifier"},
		{"unterminated dollar", "select $$abc", 1, 8, "unterminated dollar-quoted string"},
		{"extra paren", "select (1))", 1, 11, "without a matching"},
		{"open paren", "select count(\nfrom t", 1, 13, "never closed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runSQL(t, tt.in, nil)
			ie, ok := tool.AsInputError(err)
			if !ok {
				t.Fatalf("error = %v, want *InputError", err)
			}
			if ie.Line != tt.line || ie.Col != tt.col || !strings.Contains(ie.Msg, tt.msg) {
				t.Errorf("error = %d:%d %q, want %d:%d %q", ie.Line, ie.Col, ie.Msg, tt.line, tt.col, tt.msg)
			}
		})
	}
}

func TestSQLFormatDetect(t *testing.T) {
	tests := []struct {
		sample string
		want   float64
	}{
		{"select * from t", 0.7},
		{"-- comment\nINSERT INTO t VALUES (1)", 0.7},
		{"WITH x AS (select 1) select * from x", 0.7},
		{"selection of items", 0},
		{"select", 0},
		{"hello world", 0},
	}
	for _, tt := range tests {
		if got := (formatters.SQLFormat{}).Detect(tt.sample); got != tt.want {
			t.Errorf("Detect(%q) = %v, want %v", tt.sample, got, tt.want)
		}
	}
}

// FuzzSQLFormat: formatting is idempotent and keeps every token.
func FuzzSQLFormat(f *testing.F) {
	for _, s := range []string{"select a, b from t where x = 1 and y between 1 and 2",
		"with x as (select 1) select * from x; insert into t (a) values (1)",
		"select 'a''b', \"q\", $$d$$ -- c\nfrom t /* b */", "select (select 1), count(*) over (partition by a)"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		once, err := runSQL(t, in, nil)
		if err != nil {
			if _, ok := tool.AsInputError(err); !ok {
				t.Fatalf("error that is not *InputError: %v", err)
			}
			return
		}
		twice, err := runSQL(t, once.Text, nil)
		if err != nil {
			t.Fatalf("formatting %q gave %q, which fails: %v", in, once.Text, err)
		}
		if twice.Text != once.Text {
			t.Fatalf("not idempotent:\n%q\n→ %q\n→ %q", in, once.Text, twice.Text)
		}
	})
}
