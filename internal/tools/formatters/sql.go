package formatters

import (
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// SQLFormat formats SQL with its own tokenizer, without a dialect: each
// clause starts a line and its body is indented below; subqueries, CTEs
// and column lists open an indented block. Strings, quoted identifiers and
// comments are kept as written.
type SQLFormat struct{}

func (SQLFormat) Spec() tool.Spec {
	return tool.Spec{
		ID:          "sql-format",
		Name:        "SQL",
		Category:    tool.Formatters,
		Description: "Format SELECT, INSERT, UPDATE, DELETE, CTEs and DDL",
		Keywords:    []string{"sql", "query", "postgres", "mysql", "sqlite", "beautify", "pretty"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Input", Lang: "sql", Multiline: true}},
		Options: []tool.Option{
			{Key: "indent", Label: "Indent", Kind: tool.Number, Min: 1, Max: 8, Default: 2},
			{Key: "uppercase", Label: "Uppercase keywords", Kind: tool.Toggle, Default: true},
		},
		Output: tool.Highlighted,
		Live:   true,
	}
}

func (f SQLFormat) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(f.Spec())
	in := req.Inputs["input"]
	if strings.TrimSpace(in) == "" {
		return tool.Result{Lang: "sql"}, nil
	}
	toks, err := tokenizeSQL(in)
	if err != nil {
		return tool.Result{}, err
	}
	if err := checkParens(in, toks); err != nil {
		return tool.Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	w := &sqlWriter{indent: strings.Repeat(" ", min(max(opts.Int("indent"), 1), 8)), upper: opts.Bool("uppercase")}
	out, statements := w.format(toks)
	return tool.Result{
		Text: out,
		Lang: "sql",
		Meta: []tool.KV{{Key: "Statements", Value: fmt.Sprint(statements)}},
	}, nil
}

type sqlKind int

const (
	sqlWord   sqlKind = iota // keyword or identifier
	sqlQuoted                // "ident", `ident`, [ident]
	sqlString                // 'text', E'text', N'text', $$text$$
	sqlNumber                //
	sqlLineComment
	sqlBlockComment
	sqlPunct // ( ) , ; .
	sqlOp    // = <> <= ::  * + - / % || …
	sqlParam // ? $1 :name @var
)

type sqlToken struct {
	kind sqlKind
	text string
	off  int
	nl   bool // a line break came before it (to keep blank-line-free comments in place)
}

// tokenizeSQL splits the input; unterminated strings, quoted identifiers
// and block comments are InputErrors at their start.
func tokenizeSQL(in string) ([]sqlToken, error) {
	var toks []sqlToken
	nl := false
	unterminated := func(off int, what string) error {
		line, col := tool.Position(in, off)
		return &tool.InputError{Input: "input", Line: line, Col: col, Msg: "unterminated " + what}
	}
	for i := 0; i < len(in); {
		r, size := utf8.DecodeRuneInString(in[i:])
		start := i
		switch {
		case r == '\n':
			nl = true
			i += size
			continue
		case unicode.IsSpace(r):
			i += size
			continue
		case strings.HasPrefix(in[i:], "--"):
			end := strings.IndexByte(in[i:], '\n')
			if end < 0 {
				end = len(in) - i
			}
			toks = append(toks, sqlToken{sqlLineComment, strings.TrimRight(in[i:i+end], " \t\r"), start, nl})
			i += end
		case strings.HasPrefix(in[i:], "/*"):
			end := strings.Index(in[i+2:], "*/")
			if end < 0 {
				return nil, unterminated(start, "comment")
			}
			i += 2 + end + 2
			toks = append(toks, sqlToken{sqlBlockComment, in[start:i], start, nl})
		case r == '\'' || ((r == 'E' || r == 'e' || r == 'N' || r == 'n' || r == 'X' || r == 'x' || r == 'B' || r == 'b') && i+1 < len(in) && in[i+1] == '\''):
			j := i
			if r != '\'' {
				j++
			}
			end, ok := closeQuote(in, j, '\'', r == 'E' || r == 'e')
			if !ok {
				return nil, unterminated(start, "string")
			}
			i = end
			toks = append(toks, sqlToken{sqlString, in[start:i], start, nl})
		case r == '$' && dollarTag(in[i:]) != "":
			tag := dollarTag(in[i:])
			end := strings.Index(in[i+len(tag):], tag)
			if end < 0 {
				return nil, unterminated(start, "dollar-quoted string")
			}
			i += len(tag) + end + len(tag)
			toks = append(toks, sqlToken{sqlString, in[start:i], start, nl})
		case r == '"' || r == '`':
			end, ok := closeQuote(in, i, byte(r), false)
			if !ok {
				return nil, unterminated(start, "quoted identifier")
			}
			i = end
			toks = append(toks, sqlToken{sqlQuoted, in[start:i], start, nl})
		case r == '[' && i+1 < len(in) && (unicode.IsLetter(rune(in[i+1])) || in[i+1] == '_'):
			end := strings.IndexByte(in[i:], ']')
			if end < 0 {
				return nil, unterminated(start, "bracketed identifier")
			}
			i += end + 1
			toks = append(toks, sqlToken{sqlQuoted, in[start:i], start, nl})
		case unicode.IsDigit(r) || (r == '.' && i+1 < len(in) && in[i+1] >= '0' && in[i+1] <= '9'):
			j := i
			for j < len(in) && (isWordByte(in[j]) || in[j] == '.' ||
				((in[j] == '+' || in[j] == '-') && j > i && (in[j-1] == 'e' || in[j-1] == 'E'))) {
				j++
			}
			i = j
			toks = append(toks, sqlToken{sqlNumber, in[start:i], start, nl})
		case unicode.IsLetter(r) || r == '_':
			j := i
			for j < len(in) {
				rr, sz := utf8.DecodeRuneInString(in[j:])
				if !unicode.IsLetter(rr) && !unicode.IsDigit(rr) && rr != '_' && rr != '$' {
					break
				}
				j += sz
			}
			i = j
			toks = append(toks, sqlToken{sqlWord, in[start:i], start, nl})
		case r == '?' || ((r == '$' || r == ':' || r == '@') && i+1 < len(in) && isWordByte(in[i+1]) && !strings.HasPrefix(in[i:], "::")):
			j := i + 1
			for j < len(in) && isWordByte(in[j]) {
				j++
			}
			if in[i] == '@' && j < len(in) && in[j] == '@' { // @@global
				j++
				for j < len(in) && isWordByte(in[j]) {
					j++
				}
			}
			i = j
			toks = append(toks, sqlToken{sqlParam, in[start:i], start, nl})
		case strings.ContainsRune("(),;.", r):
			i += size
			toks = append(toks, sqlToken{sqlPunct, in[start:i], start, nl})
		default:
			op := string(r)
			for _, two := range []string{"<>", "!=", "<=", ">=", "::", "||", "->>", "->", "=>", "<<", ">>", "#>>", "#>", "@>", "<@", "~*", "!~*", "!~"} {
				if strings.HasPrefix(in[i:], two) && len(two) > len(op) {
					op = two
				}
			}
			i += len(op)
			toks = append(toks, sqlToken{sqlOp, op, start, nl})
		}
		nl = false
	}
	return toks, nil
}

// closeQuote returns the offset after the quote that closes the one at
// in[i]: a doubled quote is an escaped one, and so is \' with backslash.
func closeQuote(in string, i int, q byte, backslash bool) (int, bool) {
	for j := i + 1; j < len(in); j++ {
		switch {
		case backslash && in[j] == '\\':
			j++
		case in[j] == q && j+1 < len(in) && in[j+1] == q:
			j++
		case in[j] == q:
			return j + 1, true
		}
	}
	return 0, false
}

// dollarTag returns "$$" or "$tag$" at the start of s, or "".
func dollarTag(s string) string {
	if strings.HasPrefix(s, "$$") {
		return "$$"
	}
	for j := 1; j < len(s); j++ {
		switch {
		case s[j] == '$':
			if j > 1 && (s[1] < '0' || s[1] > '9') { // $1 is a parameter, not a tag
				return s[:j+1]
			}
			return ""
		case !isWordByte(s[j]):
			return ""
		}
	}
	return ""
}

func isWordByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

// checkParens reports the first ")" without "(" or the last "(" left open.
func checkParens(in string, toks []sqlToken) error {
	var open []int
	for _, t := range toks {
		if t.kind != sqlPunct {
			continue
		}
		switch t.text {
		case "(":
			open = append(open, t.off)
		case ")":
			if len(open) == 0 {
				line, col := tool.Position(in, t.off)
				return &tool.InputError{Input: "input", Line: line, Col: col, Msg: "\")\" without a matching \"(\""}
			}
			open = open[:len(open)-1]
		}
	}
	if len(open) > 0 {
		line, col := tool.Position(in, open[len(open)-1])
		return &tool.InputError{Input: "input", Line: line, Col: col, Msg: "\"(\" is never closed"}
	}
	return nil
}

var sqlKeywords = func() map[string]bool {
	m := map[string]bool{}
	for _, k := range strings.Fields(`ADD ALL ALTER AND ANY AS ASC BEGIN BETWEEN BY CASCADE CASE CAST CHECK COLLATE
		COLUMN COMMIT CONFLICT CONSTRAINT CREATE CROSS CURRENT_DATE CURRENT_TIMESTAMP DATABASE DEFAULT DELETE
		DESC DISTINCT DO DROP ELSE END EXCEPT EXISTS EXPLAIN FALSE FETCH FILTER FIRST FOLLOWING FOR FOREIGN FROM
		FULL GRANT GROUP HAVING IF ILIKE IN INDEX INNER INSERT INTERSECT INTERVAL INTO IS JOIN KEY LAST LATERAL
		LEFT LIKE LIMIT MATERIALIZED NATURAL NEXT NOT NOTHING NULL NULLS OFFSET ON ONLY OR ORDER OUTER OVER
		PARTITION PRECEDING PRIMARY RANGE RECURSIVE REFERENCES REPLACE RETURNING REVOKE RIGHT ROLLBACK ROW ROWS
		SELECT SET TABLE TEMP TEMPORARY THEN TO TOP TRANSACTION TRIGGER TRUE TRUNCATE UNBOUNDED UNION UNIQUE
		UPDATE USING VALUES VIEW WHEN WHERE WINDOW WITH`) {
		m[k] = true
	}
	return m
}()

// sqlClauses start a line at the frame's indent; their body goes below,
// one level in. The value is how many words make up the clause.
var sqlClauses = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true, "GROUP BY": true, "ORDER BY": true, "HAVING": true,
	"LIMIT": true, "OFFSET": true, "FETCH": true, "UNION": true, "UNION ALL": true, "INTERSECT": true, "EXCEPT": true,
	"INSERT INTO": true, "VALUES": true, "UPDATE": true, "SET": true, "DELETE FROM": true, "RETURNING": true,
	"WITH": true, "WITH RECURSIVE": true, "ON CONFLICT": true, "WINDOW": true, "CREATE": true, "ALTER": true, "DROP": true,
}

// inlineClauses keep their body on the same line (LIMIT 10).
var inlineClauses = map[string]bool{"LIMIT": true, "OFFSET": true, "FETCH": true, "UNION": true, "UNION ALL": true,
	"INTERSECT": true, "EXCEPT": true, "ON CONFLICT": true, "CREATE": true, "ALTER": true, "DROP": true,
	"WITH": true, "WITH RECURSIVE": true, "INSERT INTO": true, "UPDATE": true, "DELETE FROM": true}

// listClauses break their body at top-level commas.
var listClauses = map[string]bool{"SELECT": true, "FROM": true, "GROUP BY": true, "ORDER BY": true, "SET": true,
	"VALUES": true, "RETURNING": true, "WINDOW": true}

var joinWords = map[string]bool{"JOIN": true, "LEFT": true, "RIGHT": true, "INNER": true, "FULL": true, "CROSS": true, "NATURAL": true, "OUTER": true}

// frame is one indentation context: the statement, a subquery or a CTE body.
type frame struct {
	level   int    // indent level of its clauses
	parent  int    // indent level of the line that opened it, for ")"
	clause  string // current clause
	inline  int    // nesting of inline parentheses (calls, IN lists)
	block   bool   // opened by "(" (subquery, CTE, column list)
	list    bool   // a column list: commas break lines
	between bool   // inside BETWEEN … AND
}

type sqlWriter struct {
	indent    string
	upper     bool
	b         strings.Builder
	frames    []*frame
	lineStart bool
	level     int // indent level of the current line
	prev      sqlToken
	hasPrev   bool
	space1    bool // the next token gets a space even after a name: users (id)
}

func (w *sqlWriter) top() *frame { return w.frames[len(w.frames)-1] }

// newline starts a line at level; on a line still empty it only changes
// the indentation, so clauses never leave blank lines.
func (w *sqlWriter) newline(level int) {
	out := strings.TrimRight(w.b.String(), " ")
	w.b.Reset()
	w.b.WriteString(out)
	if w.b.Len() > 0 && !strings.HasSuffix(out, "\n") {
		w.b.WriteString("\n")
	}
	w.b.WriteString(strings.Repeat(w.indent, max(level, 0)))
	w.lineStart = true
	w.level = level
}

func (w *sqlWriter) word(t sqlToken) string {
	if t.kind == sqlWord && sqlKeywords[strings.ToUpper(t.text)] {
		if w.upper {
			return strings.ToUpper(t.text)
		}
		return strings.ToLower(t.text)
	}
	return t.text
}

// space writes the separator before t, unless the line just started or
// the layout glues them (count(*), t.col, a::int) and gluing keeps them
// two tokens.
func (w *sqlWriter) space(t sqlToken) {
	if w.lineStart || !w.hasPrev {
		return
	}
	if w.space1 {
		w.space1 = false
		w.b.WriteString(" ")
		return
	}
	if !w.glue(w.prev, t) || merges(w.prev.text, t.text) {
		w.b.WriteString(" ")
	}
}

func (w *sqlWriter) glue(p, t sqlToken) bool {
	name := func(k sqlToken) bool { return k.kind == sqlWord || k.kind == sqlQuoted }
	switch {
	case p.kind == sqlPunct && p.text == "(":
	case p.kind == sqlPunct && p.text == "." && (name(t) || t.text == "*"): // t.col, t.*
	case t.kind == sqlPunct && t.text == "." && (name(p) || p.text == ")"):
	case t.kind == sqlPunct && (t.text == ")" || t.text == "," || t.text == ";"):
	case t.kind == sqlOp && t.text == "::", p.kind == sqlOp && p.text == "::":
	case t.kind == sqlPunct && t.text == "(" && name(p) && !sqlKeywords[strings.ToUpper(p.text)]:
		// a function call: count(*)
	default:
		return false
	}
	return true
}

// merges reports whether a and b written together read as other tokens
// (":" + "::" is ":::", "1" + ".x" is a number).
func merges(a, b string) bool {
	toks, err := tokenizeSQL(a + b)
	return err != nil || len(toks) != 2 || toks[0].text != a || toks[1].text != b
}

func (w *sqlWriter) write(t sqlToken, text string) {
	w.space(t)
	w.b.WriteString(text)
	w.lineStart = false
	w.prev, w.hasPrev = t, true
}

// clauseAt joins the words of a clause starting at toks[i] (GROUP BY,
// LEFT OUTER JOIN…) and returns it with the number of tokens it uses.
func clauseAt(toks []sqlToken, i int) (string, int) {
	up := func(k int) string {
		if k < len(toks) && toks[k].kind == sqlWord {
			return strings.ToUpper(toks[k].text)
		}
		return ""
	}
	first := up(i)
	switch first {
	case "GROUP", "ORDER", "PARTITION":
		if up(i+1) == "BY" {
			return first + " BY", 2
		}
	case "INSERT", "DELETE":
		if next := up(i + 1); next == "INTO" || next == "FROM" {
			return first + " " + next, 2
		}
	case "UNION":
		if up(i+1) == "ALL" {
			return "UNION ALL", 2
		}
	case "WITH":
		if up(i+1) == "RECURSIVE" {
			return "WITH RECURSIVE", 2
		}
	case "ON":
		if up(i+1) == "CONFLICT" {
			return "ON CONFLICT", 2
		}
	}
	if joinWords[first] {
		n := 0
		var words []string
		for joinWords[up(i+n)] {
			words = append(words, up(i+n))
			n++
			if words[len(words)-1] == "JOIN" {
				return strings.Join(words, " "), n
			}
		}
	}
	return first, 1
}

func (w *sqlWriter) format(toks []sqlToken) (string, int) {
	w.frames = []*frame{{}}
	statements := 0
	pendingStatement := false
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		f := w.top()
		if !pendingStatement && t.kind != sqlLineComment && t.kind != sqlBlockComment {
			pendingStatement = true
			statements++
		}
		switch t.kind {
		case sqlLineComment:
			if t.nl && !w.lineStart && w.hasPrev {
				w.newline(w.bodyLevel())
			}
			w.write(t, t.text)
			w.newline(w.bodyLevel())
			w.prev = t
			continue
		case sqlBlockComment:
			if t.nl && w.hasPrev && !w.lineStart {
				w.newline(w.bodyLevel())
			}
			w.write(t, t.text)
			continue
		case sqlPunct:
			switch t.text {
			case ";":
				w.write(t, ";")
				w.frames = []*frame{{}}
				pendingStatement = false
				if i+1 < len(toks) {
					w.b.WriteString("\n\n")
					w.lineStart, w.level = true, 0
				}
				continue
			case "(":
				next := ""
				if i+1 < len(toks) && toks[i+1].kind == sqlWord {
					next = strings.ToUpper(toks[i+1].text)
				}
				column := f.inline == 0 && (f.clause == "CREATE" || f.clause == "INSERT INTO") && w.prev.kind != sqlWord ||
					f.inline == 0 && f.clause == "CREATE" && (w.prev.kind == sqlWord || w.prev.kind == sqlQuoted) && !sqlKeywords[strings.ToUpper(w.prev.text)]
				table := f.inline == 0 && (f.clause == "INSERT INTO" || f.clause == "CREATE") &&
					(w.prev.kind == sqlWord || w.prev.kind == sqlQuoted) && !sqlKeywords[strings.ToUpper(w.prev.text)]
				if table {
					w.space1 = true // INSERT INTO users (id), CREATE TABLE t (
				}
				if next == "SELECT" || next == "WITH" || column {
					w.write(t, "(")
					list := column && next != "SELECT" && next != "WITH"
					nf := &frame{level: w.level + 1, parent: w.level, block: true, list: list}
					if list {
						nf.clause, nf.level = "LIST", w.level
					}
					w.frames = append(w.frames, nf)
					w.newline(nf.level + boolInt(list))
					continue
				}
				f.inline++
				w.write(t, "(")
				continue
			case ")":
				if f.inline > 0 {
					f.inline--
					w.write(t, ")")
					continue
				}
				if len(w.frames) > 1 {
					w.frames = w.frames[:len(w.frames)-1]
					w.newline(f.parent)
					w.write(t, ")")
					continue
				}
				w.write(t, ")")
				continue
			case ",":
				w.write(t, ",")
				if f.inline == 0 && (listClauses[f.clause] || f.list || f.clause == "WITH" || f.clause == "WITH RECURSIVE") {
					if f.clause == "WITH" || f.clause == "WITH RECURSIVE" {
						w.newline(f.level)
					} else {
						w.newline(w.bodyLevel())
					}
				}
				continue
			}
		case sqlWord:
			up := strings.ToUpper(t.text)
			if f.inline == 0 && !f.list {
				clause, n := clauseAt(toks, i)
				switch {
				case sqlClauses[clause] && (clause != "UPDATE" || !strings.EqualFold(w.prev.text, "DO")): // DO UPDATE stays inline
					if w.hasPrev && (w.prev.kind != sqlPunct || w.prev.text != "(") {
						w.newline(f.level)
					}
					f.clause, f.between = clause, false
					w.write(t, w.words(toks[i:i+n]))
					i += n - 1
					if !inlineClauses[clause] {
						w.newline(f.level + 1)
					}
					continue
				case strings.HasSuffix(clause, "JOIN"):
					w.newline(f.level)
					f.clause = "JOIN"
					w.write(t, w.words(toks[i:i+n]))
					i += n - 1
					continue
				case up == "ON" && f.clause == "JOIN":
					w.newline(f.level + 1)
					f.clause = "ON"
					w.write(t, w.word(t))
					continue
				case up == "BETWEEN":
					f.between = true
				case (up == "AND" || up == "OR") && (f.clause == "WHERE" || f.clause == "HAVING" || f.clause == "ON"):
					if up == "AND" && f.between {
						f.between = false
						break
					}
					w.newline(w.bodyLevel())
					w.write(t, w.word(t))
					continue
				}
			}
			w.write(t, w.word(t))
			continue
		}
		w.write(t, t.text)
	}
	return strings.TrimRight(w.b.String(), " \n"), statements
}

// bodyLevel is where the current clause's body goes.
func (w *sqlWriter) bodyLevel() int {
	f := w.top()
	switch {
	case f.list:
		return f.level + 1
	case f.clause == "" || inlineClauses[f.clause] && f.clause != "UPDATE":
		return f.level
	}
	return f.level + 1
}

func (w *sqlWriter) words(ts []sqlToken) string {
	parts := make([]string, len(ts))
	for i, t := range ts {
		parts[i] = w.word(t)
	}
	return strings.Join(parts, " ")
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Detect recognises text that starts with a common SQL statement.
func (SQLFormat) Detect(sample string) float64 {
	s := strings.TrimSpace(sample)
	for strings.HasPrefix(s, "--") { // leading comments
		_, rest, _ := strings.Cut(s, "\n")
		s = strings.TrimSpace(rest)
	}
	first, _, _ := strings.Cut(s, " ")
	switch strings.ToUpper(strings.TrimSpace(first)) {
	case "SELECT", "INSERT", "UPDATE", "DELETE", "WITH", "CREATE", "ALTER", "DROP":
		if strings.ContainsAny(s, " \n\t") {
			return 0.7
		}
	}
	return 0
}
