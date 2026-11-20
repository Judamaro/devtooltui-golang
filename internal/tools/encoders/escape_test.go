package encoders_test

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/encoders"
)

func runEscape(t testing.TB, in, mode, lang string) (tool.Result, error) {
	t.Helper()
	return encoders.Escape{}.Run(context.Background(), tool.Request{
		Inputs: map[string]string{"input": in},
		Opts:   tool.Values{"mode": mode, "lang": lang},
	})
}

var escapeLangs = []string{"json", "go", "csharp", "sql", "shell"}

func TestEscapeGolden(t *testing.T) {
	in := testutil.ReadInput(t, "escape", "mixed")
	for _, lang := range escapeLangs {
		t.Run(lang, func(t *testing.T) {
			res, err := runEscape(t, in, "encode", lang)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "escape", "mixed-"+lang, res.Text)
			back, err := runEscape(t, res.Text, "decode", lang)
			if err != nil || back.Text != in {
				t.Errorf("decode = %q, %v; want the input back", back.Text, err)
			}
		})
	}
}

func TestEscape(t *testing.T) {
	tests := []struct {
		lang, mode, in, want string
	}{
		{"json", "encode", "a\"b\\c\nd\x01\u2028", `a\"b\\c\nd\u0001\u2028`},
		{"json", "encode", "a\xffb", "a\ufffdb"},
		{"json", "decode", `a\"b\\c\/d\n\u00e9`, "a\"b\\c/d\né"},
		{"json", "decode", `"quoted\tvalue"`, "quoted\tvalue"},
		{"json", "decode", `\ud83c\udf55`, "🍕"},
		{"json", "decode", `\ud83c x`, "\ufffd x"},
		{"json", "decode", `say "hi"`, `say "hi"`},
		{"json", "decode", `"ends with \"`, `"ends with "`},
		{"go", "encode", "tab\there \"q\" \x00\xff é", `tab\there \"q\" \x00\xff é`},
		{"go", "decode", `\x41\101\u00e9\U0001F355\a`, "AAé🍕\a"},
		{"go", "decode", `"quoted"`, "quoted"},
		{"go", "decode", "`not raw`", "`not raw`"},
		{"go", "decode", `\xff`, "\xff"},
		{"csharp", "encode", "a\x00\a\v\"\\\x1f", `a\0\a\v\"\\\u001f`},
		{"csharp", "decode", `\x41\x4142\U0001F355\'`, "A䅂🍕'"},
		{"csharp", "decode", `@"C:\path ""x"""`, `C:\path "x"`},
		{"sql", "encode", "O'Brien's", "O''Brien''s"},
		{"sql", "decode", "O''Brien", "O'Brien"},
		{"sql", "decode", "'O''Brien'", "O'Brien"},
		{"sql", "decode", "''", "'"},
		{"shell", "encode", "", "''"},
		{"shell", "encode", "safe/path-1.txt", "safe/path-1.txt"},
		{"shell", "encode", "it's a $HOME", `'it'\''s a $HOME'`},
		{"shell", "decode", `'it'\''s'`, "it's"},
		{"shell", "decode", `"a \$b \"c\" \x"`, `a $b "c" \x`},
		{"shell", "decode", `a\ b`, "a b"},
		{"shell", "decode", "a\\\nb", "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.lang+" "+tt.mode+" "+tt.in, func(t *testing.T) {
			res, err := runEscape(t, tt.in, tt.mode, tt.lang)
			if err != nil {
				t.Fatal(err)
			}
			if res.Text != tt.want {
				t.Errorf("got %q, want %q", res.Text, tt.want)
			}
		})
	}
}

func TestEscapeErrors(t *testing.T) {
	tests := []struct {
		lang, in  string
		line, col int
		msg       string
	}{
		{"json", `ok\q`, 1, 3, `invalid escape "\\q"`},
		{"json", "line\n  \\u12", 2, 3, "invalid escape"},
		{"json", `"bad \x"`, 1, 6, "invalid escape"},
		{"json", `end\`, 1, 4, "unfinished escape"},
		{"go", `\z`, 1, 1, "invalid escape"},
		{"go", `\'`, 1, 1, "invalid escape"},
		{"csharp", `\xg`, 1, 1, "invalid escape"},
		{"csharp", `\U0011FFFF`, 1, 1, "invalid escape"},
		{"sql", "O'Brien", 1, 2, "unescaped quote"},
		{"shell", "a 'b", 1, 3, "unterminated single quote"},
		{"shell", "x\n\"y", 2, 1, "unterminated double quote"},
		{"shell", `a\`, 1, 2, "backslash at the end"},
	}
	for _, tt := range tests {
		t.Run(tt.lang+" "+tt.in, func(t *testing.T) {
			_, err := runEscape(t, tt.in, "decode", tt.lang)
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

func FuzzEscapeRoundTrip(f *testing.F) {
	for _, s := range []string{"", `a"b\c`, "it's", "\x00\n\t", "🍕\u2028", "\xff"} {
		for i := range escapeLangs {
			f.Add(s, uint8(i))
		}
	}
	f.Fuzz(func(t *testing.T, s string, li uint8) {
		lang := escapeLangs[int(li)%len(escapeLangs)]
		if (lang == "json" || lang == "csharp") && !utf8.ValidString(s) {
			s = strings.ToValidUTF8(s, "\ufffd") // neither can hold raw bytes
		}
		enc, err := runEscape(t, s, "encode", lang)
		if err != nil {
			t.Fatal(err)
		}
		dec, err := runEscape(t, enc.Text, "decode", lang)
		if err != nil {
			t.Fatalf("%s: decode(%q): %v", lang, enc.Text, err)
		}
		if dec.Text != s {
			t.Errorf("%s round trip: %q → %q → %q", lang, s, enc.Text, dec.Text)
		}
	})
}
