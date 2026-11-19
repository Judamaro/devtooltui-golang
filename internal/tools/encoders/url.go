package encoders

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// URL encodes, decodes and breaks down URLs.
type URL struct{}

func (URL) Spec() tool.Spec {
	return tool.Spec{
		ID:          "url",
		Name:        "URL",
		Category:    tool.Encoders,
		Description: "Encode, decode and break down URLs (percent-encoding)",
		Keywords:    []string{"percent", "encode", "decode", "query", "parse", "uri"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Input", Multiline: true}},
		Options: []tool.Option{
			{Key: "mode", Label: "Mode", Kind: tool.Select, Choices: []string{"encode", "decode", "parse"}, Default: "encode"},
			{Key: "escape", Label: "Escape", Help: "query: space → '+'; path: space → '%20'", Kind: tool.Select, Choices: []string{"query", "path"}, Default: "query"},
		},
		Output: tool.PlainText,
		Live:   true,
	}
}

func (u URL) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(u.Spec())
	in := req.Inputs["input"]
	query := opts.String("escape") == "query"

	var out string
	switch opts.String("mode") {
	case "decode":
		if pos := invalidEscape(in); pos >= 0 {
			line, col := tool.Position(in, pos)
			return tool.Result{}, &tool.InputError{Input: "input", Line: line, Col: col,
				Msg: fmt.Sprintf("invalid escape %q", in[pos:min(pos+3, len(in))])}
		}
		var err error
		if query {
			out, err = url.QueryUnescape(in)
		} else {
			out, err = url.PathUnescape(in)
		}
		if err != nil { // cannot happen after invalidEscape
			return tool.Result{}, &tool.InputError{Input: "input", Msg: err.Error()}
		}
	case "parse":
		return parseURL(in)
	default:
		if query {
			out = url.QueryEscape(in)
		} else {
			out = url.PathEscape(in)
		}
	}
	return tool.Result{
		Text: out,
		Meta: []tool.KV{{Key: "Size", Value: fmt.Sprintf("%d B → %d B", len(in), len(out))}},
	}, nil
}

// invalidEscape returns the offset of the first '%' not followed by two hex
// digits, or -1.
func invalidEscape(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		if i+2 >= len(s) || !isHex(s[i+1]) || !isHex(s[i+2]) {
			return i
		}
		i += 2
	}
	return -1
}

func isHex(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

func parseURL(in string) (tool.Result, error) {
	raw := strings.TrimSpace(in)
	if raw == "" {
		return tool.Result{}, nil
	}
	if strings.ContainsAny(raw, "\n\r") {
		return tool.Result{}, &tool.InputError{Input: "input", Msg: "parse expects a single URL"}
	}
	u, err := url.Parse(raw)
	if err != nil {
		msg := err.Error()
		if ue, ok := err.(*url.Error); ok {
			msg = ue.Err.Error()
		}
		return tool.Result{}, &tool.InputError{Input: "input", Line: 1, Msg: msg}
	}

	var b strings.Builder
	field := func(label, value string) {
		if value != "" {
			fmt.Fprintf(&b, "%-11s %s\n", label+":", value)
		}
	}
	field("Scheme", u.Scheme)
	field("Opaque", u.Opaque)
	if u.User != nil {
		field("User", u.User.Username())
		if p, ok := u.User.Password(); ok {
			field("Password", p)
		}
	}
	field("Host", u.Hostname())
	field("Port", u.Port())
	field("Path", u.Path)
	field("Fragment", u.Fragment)

	params := queryParams(u.RawQuery)
	if len(params) > 0 {
		b.WriteString("Query:\n")
		for _, p := range params {
			fmt.Fprintf(&b, "  %s = %s\n", p[0], p[1])
		}
	}
	return tool.Result{
		Text: strings.TrimRight(b.String(), "\n"),
		Meta: []tool.KV{{Key: "Parameters", Value: fmt.Sprint(len(params))}},
	}, nil
}

// queryParams breaks the query down keeping order and repeated keys.
func queryParams(raw string) [][2]string {
	var out [][2]string
	for part := range strings.SplitSeq(raw, "&") {
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		out = append(out, [2]string{unescapeOrRaw(k), unescapeOrRaw(v)})
	}
	return out
}

func unescapeOrRaw(s string) string {
	if d, err := url.QueryUnescape(s); err == nil {
		return d
	}
	return s
}

var escapeRe = regexp.MustCompile(`%[0-9A-Fa-f]{2}`)

// Detect recognises text with valid %XX escapes and no spaces.
func (URL) Detect(sample string) float64 {
	s := strings.TrimSpace(sample)
	if s == "" || strings.ContainsAny(s, " \t\n\r") || !escapeRe.MatchString(s) || invalidEscape(s) >= 0 {
		return 0
	}
	return 0.55
}
