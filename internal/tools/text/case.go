package text

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Case converts each line between naming styles (camelCase, snake_case…)
// and text cases (Title, Sentence…).
type Case struct{}

func (Case) Spec() tool.Spec {
	return tool.Spec{
		ID:          "case",
		Name:        "Case converter",
		Category:    tool.Text,
		Description: "Convert each line to camelCase, snake_case, kebab-case, Title case…",
		Keywords:    []string{"camel", "pascal", "snake", "kebab", "constant", "title", "upper", "lower"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Input", Multiline: true}},
		Options: []tool.Option{
			{Key: "to", Label: "To", Kind: tool.Select,
				Choices: []string{"camel", "pascal", "snake", "kebab", "constant", "dot", "title", "sentence", "lower", "upper"}, Default: "camel"},
			{Key: "split_digits", Label: "Split digits", Help: "Treat digits as separate words: HTTPServer2 → http_server_2", Kind: tool.Toggle, Default: true},
		},
		Output: tool.PlainText,
		Live:   true,
	}
}

func (c Case) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(c.Spec())
	to, splitDigits := opts.String("to"), opts.Bool("split_digits")
	lines := strings.Split(req.Inputs["input"], "\n")
	for i, line := range lines {
		if i%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return tool.Result{}, err
			}
		}
		cr := strings.HasSuffix(line, "\r")
		line = strings.TrimSuffix(line, "\r")
		indent := line[:len(line)-len(strings.TrimLeftFunc(line, unicode.IsSpace))]
		out := convertCase(line[len(indent):], to, splitDigits)
		if out != "" {
			out = indent + out
		}
		if cr {
			out += "\r"
		}
		lines[i] = out
	}
	return tool.Result{
		Text: strings.Join(lines, "\n"),
		Meta: []tool.KV{{Key: "Lines", Value: fmt.Sprint(len(lines))}},
	}, nil
}

func convertCase(s, to string, splitDigits bool) string {
	switch to {
	case "lower":
		return strings.ToLower(s)
	case "upper":
		return strings.ToUpper(s)
	case "title", "sentence":
		if !strings.ContainsFunc(s, unicode.IsSpace) && !strings.ContainsAny(s, "'’") {
			// An identifier: split it into words first (userID → User Id).
			s = strings.ToLower(strings.Join(IdentWords(s, splitDigits), " "))
		}
		return titleText(s, to == "title")
	}
	words := IdentWords(s, splitDigits)
	for i, w := range words {
		switch to {
		case "camel":
			if i == 0 {
				words[i] = strings.ToLower(w)
			} else {
				words[i] = TitleWord(w)
			}
		case "pascal":
			words[i] = TitleWord(w)
		case "constant":
			words[i] = strings.ToUpper(w)
		default:
			words[i] = strings.ToLower(w)
		}
	}
	sep := map[string]string{"snake": "_", "constant": "_", "kebab": "-", "dot": "."}[to]
	return strings.Join(words, sep)
}

// IdentWords splits identifiers and phrases into words: at any character
// that is not a letter or a digit, at lower→upper (fooBar), before the last
// capital of an acronym (HTTPServer → HTTP Server) and, with splitDigits,
// between letters and digits (v2 → v 2).
func IdentWords(s string, splitDigits bool) []string {
	rs := []rune(s)
	var words []string
	start := -1
	flush := func(end int) {
		if start >= 0 && end > start {
			words = append(words, string(rs[start:end]))
		}
		start = -1
	}
	for i, r := range rs {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			flush(i)
			continue
		}
		if start < 0 {
			start = i
			continue
		}
		prev := rs[i-1]
		var next rune
		if i+1 < len(rs) {
			next = rs[i+1]
		}
		switch {
		case unicode.IsUpper(r) && (unicode.IsLower(prev) || unicode.IsDigit(prev)):
			flush(i) // fooBar, v2Api
		case unicode.IsUpper(r) && unicode.IsUpper(prev) && unicode.IsLower(next):
			flush(i) // HTTPServer
		case splitDigits && unicode.IsDigit(r) != unicode.IsDigit(prev):
			flush(i) // Server2, 2fa
		}
		if start < 0 {
			start = i
		}
	}
	flush(len(rs))
	return words
}

// TitleWord capitalises the first letter and lowercases the rest.
func TitleWord(w string) string {
	rs := []rune(strings.ToLower(w))
	if len(rs) > 0 {
		rs[0] = unicode.ToUpper(rs[0])
	}
	return string(rs)
}

// titleText keeps punctuation and spacing. every=true capitalises each
// word (Title Case); false only the first letter (Sentence case). An
// apostrophe between letters stays inside the word (don't → Don't).
func titleText(s string, every bool) string {
	rs := []rune(strings.ToLower(s))
	first := true
	for i, r := range rs {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			continue
		}
		inWord := i > 0 && (unicode.IsLetter(rs[i-1]) || unicode.IsDigit(rs[i-1]) ||
			(i > 1 && (rs[i-1] == '\'' || rs[i-1] == '’') && unicode.IsLetter(rs[i-2])))
		if (every && !inWord) || first {
			rs[i] = unicode.ToUpper(r)
		}
		first = false
	}
	return string(rs)
}
