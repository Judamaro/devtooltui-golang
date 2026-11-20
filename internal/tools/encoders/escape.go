package encoders

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Escape escapes and unescapes string literals for JSON, Go, C#, SQL and
// POSIX shells.
type Escape struct{}

func (Escape) Spec() tool.Spec {
	return tool.Spec{
		ID:          "escape",
		Name:        "String escape",
		Category:    tool.Encoders,
		Description: "Escape and unescape string literals for JSON, Go, C#, SQL and shell",
		Keywords:    []string{"unescape", "quote", "literal", "string", "backslash"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Input", Multiline: true}},
		Options: []tool.Option{
			{Key: "mode", Label: "Mode", Kind: tool.Select, Choices: []string{"encode", "decode"}, Default: "encode"},
			{Key: "lang", Label: "Language", Help: "Encoding gives the inside of the literal (shell: a whole quoted word); decoding accepts it with or without quotes", Kind: tool.Select,
				Choices: []string{"json", "go", "csharp", "sql", "shell"}, Default: "json"},
		},
		Output: tool.PlainText,
		Live:   true,
	}
}

func (e Escape) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(e.Spec())
	in := req.Inputs["input"]
	lang := opts.String("lang")

	var out string
	if opts.String("mode") == "decode" {
		var off int
		var msg string
		out, off, msg = unescape(lang, in)
		if msg != "" {
			line, col := tool.Position(in, off)
			return tool.Result{}, &tool.InputError{Input: "input", Line: line, Col: col, Msg: msg}
		}
	} else {
		out = escapeString(lang, in)
	}
	return tool.Result{
		Text: out,
		Meta: []tool.KV{
			{Key: "Input", Value: fmt.Sprintf("%d B", len(in))},
			{Key: "Output", Value: fmt.Sprintf("%d B", len(out))},
		},
	}, nil
}

func escapeString(lang, s string) string {
	switch lang {
	case "go":
		q := strconv.Quote(s)
		return q[1 : len(q)-1]
	case "csharp":
		return escapeC(s, true)
	case "sql":
		return strings.ReplaceAll(s, "'", "''")
	case "shell":
		return shellQuote(s)
	}
	return escapeC(s, false)
}

// escapeC escapes for JSON (csharp=false) or C#. Invalid UTF-8 becomes
// U+FFFD: neither language can hold raw bytes in a string.
func escapeC(s string, csharp bool) string {
	var b strings.Builder
	b.Grow(len(s) + len(s)/8)
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			switch {
			case csharp && r == 0:
				b.WriteString(`\0`)
			case csharp && r == '\a':
				b.WriteString(`\a`)
			case csharp && r == '\v':
				b.WriteString(`\v`)
			case r < 0x20 || r == 0x7f || r == 0x2028 || r == 0x2029:
				fmt.Fprintf(&b, `\u%04x`, r)
			default:
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// shellQuote leaves safe words bare and single-quotes the rest ('\” for
// each quote), which every POSIX shell reads back literally.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	safe := !strings.ContainsFunc(s, func(r rune) bool { return !shellSafe(r) })
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func shellSafe(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("_@%+=:,./-", r)
}

// unescape decodes s. On error it returns the byte offset of the culprit in
// s and a message.
func unescape(lang, s string) (string, int, string) {
	switch lang {
	case "shell":
		return unescapeShell(s)
	case "sql":
		// As it is first: '' is an escaped quote, not an empty literal.
		out, off, msg := unescapeSQL(s)
		if msg != "" && len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
			if inner, _, imsg := unescapeSQL(s[1 : len(s)-1]); imsg == "" {
				return inner, 0, ""
			}
		}
		return out, off, msg
	}
	inner, shift := s, 0
	switch {
	case lang == "csharp" && len(s) >= 3 && strings.HasPrefix(s, `@"`) && s[len(s)-1] == '"':
		// Verbatim string; encoded content never starts with @" (it would be @\").
		return strings.ReplaceAll(s[2:len(s)-1], `""`, `"`), 0, ""
	case quoted(s):
		inner, shift = s[1:len(s)-1], 1
	}
	var out string
	var off int
	var msg string
	switch lang {
	case "go":
		out, off, msg = unescapeGo(inner)
	default:
		out, off, msg = unescapeC(inner, lang == "csharp")
	}
	return out, off + shift, msg
}

// quoted reports whether s is wrapped in double quotes whose closing quote
// is not escaped.
func quoted(s string) bool {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return false
	}
	backslashes := 0
	for i := len(s) - 2; i > 0 && s[i] == '\\'; i-- {
		backslashes++
	}
	return backslashes%2 == 0
}

func unescapeGo(s string) (string, int, string) {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] != '\\' {
			b.WriteByte(s[i]) // raw bytes, quotes included, pass through
			i++
			continue
		}
		v, multibyte, tail, err := strconv.UnquoteChar(s[i:], '"')
		if err != nil {
			return "", i, "invalid escape " + escapeAt(s, i)
		}
		if multibyte {
			b.WriteRune(v)
		} else {
			b.WriteByte(byte(v)) //nolint:gosec // \xFF and \377 are single bytes (v < 256)
		}
		i = len(s) - len(tail)
	}
	return b.String(), 0, ""
}

// unescapeC decodes JSON (csharp=false) or C# escapes. A \u pair that forms
// a surrogate pair becomes one character; a lone surrogate becomes U+FFFD.
func unescapeC(s string, csharp bool) (string, int, string) {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] != '\\' {
			b.WriteByte(s[i])
			i++
			continue
		}
		bad := func() (string, int, string) { return "", i, "invalid escape " + escapeAt(s, i) }
		if i+1 >= len(s) {
			return "", i, "unfinished escape at the end"
		}
		c := s[i+1]
		simple := jsonEscapes
		if csharp {
			simple = csharpEscapes
		}
		if r, ok := simple[c]; ok {
			b.WriteRune(r)
			i += 2
			continue
		}
		var digits int
		switch {
		case c == 'u':
			digits = 4
		case c == 'U' && csharp:
			digits = 8
		case c == 'x' && csharp:
			digits = 0 // 1 to 4 digits, greedy
		default:
			return bad()
		}
		hex := s[i+2:]
		n := digits
		if c == 'x' {
			for n < 4 && n < len(hex) && isHex(hex[n]) {
				n++
			}
			if n == 0 {
				return bad()
			}
		}
		if len(hex) < n {
			return bad()
		}
		v, err := strconv.ParseUint(hex[:n], 16, 32)
		if err != nil || v > utf8.MaxRune {
			return bad()
		}
		r := rune(v)
		i += 2 + n
		if utf16.IsSurrogate(r) {
			// A high surrogate followed by a low \u escape is one character.
			if r < 0xdc00 && i+6 <= len(s) && s[i] == '\\' && s[i+1] == 'u' {
				if lo, err := strconv.ParseUint(s[i+2:i+6], 16, 16); err == nil {
					if pair := utf16.DecodeRune(r, rune(lo)); pair != utf8.RuneError { //nolint:gosec // 16-bit value
						b.WriteRune(pair)
						i += 6
						continue
					}
				}
			}
			r = utf8.RuneError
		}
		b.WriteRune(r)
	}
	return b.String(), 0, ""
}

var (
	jsonEscapes   = map[byte]rune{'"': '"', '\\': '\\', '/': '/', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t'}
	csharpEscapes = map[byte]rune{'"': '"', '\\': '\\', '\'': '\'', '0': 0, 'a': '\a', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t', 'v': '\v'}
)

// escapeAt quotes the escape at s[i] (the backslash and the next
// character) for error messages.
func escapeAt(s string, i int) string {
	end := i + 1
	if end < len(s) {
		_, size := utf8.DecodeRuneInString(s[end:])
		end += size
	}
	return strconv.Quote(s[i:end])
}

func unescapeSQL(s string) (string, int, string) {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\'' {
			b.WriteByte(s[i])
			continue
		}
		if i+1 >= len(s) || s[i+1] != '\'' {
			return "", i, "unescaped quote: write it as ''"
		}
		b.WriteByte('\'')
		i++
	}
	return b.String(), 0, ""
}

// unescapeShell reads a POSIX shell word: '…' is literal, "…" honours
// \$ \` \" \\ and \newline, and outside quotes a backslash escapes the next
// character. Unquoted spaces are kept as they are.
func unescapeShell(s string) (string, int, string) {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\'':
			end := strings.IndexByte(s[i+1:], '\'')
			if end < 0 {
				return "", i, "unterminated single quote"
			}
			b.WriteString(s[i+1 : i+1+end])
			i += 1 + end
		case '"':
			start := i
			for i++; ; i++ {
				if i >= len(s) {
					return "", start, "unterminated double quote"
				}
				if s[i] == '"' {
					break
				}
				if s[i] == '\\' && i+1 < len(s) && strings.IndexByte("$`\"\\\n", s[i+1]) >= 0 {
					if s[i+1] != '\n' { // \newline is a line continuation
						b.WriteByte(s[i+1])
					}
					i++
					continue
				}
				b.WriteByte(s[i])
			}
		case '\\':
			if i+1 >= len(s) {
				return "", i, "backslash at the end"
			}
			if s[i+1] != '\n' {
				b.WriteByte(s[i+1])
			}
			i++
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String(), 0, ""
}
