package formatters

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// XMLFormat re-indents XML keeping each token's original text (attributes,
// comments, CDATA, processing instructions and DOCTYPE).
type XMLFormat struct{}

func (XMLFormat) Spec() tool.Spec {
	return tool.Spec{
		ID:          "xml-format",
		Name:        "XML",
		Category:    tool.Formatters,
		Description: "Indent or minify XML, keeping comments and CDATA",
		Keywords:    []string{"xml", "pretty", "minify", "beautify", "svg", "html"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Input", Lang: "xml", Multiline: true}},
		Options: []tool.Option{
			{Key: "indent", Label: "Indent", Help: "Spaces per level; 0 = minify", Kind: tool.Number, Min: 0, Max: 8, Default: 2},
		},
		Output: tool.Highlighted,
		Live:   true,
	}
}

func (x XMLFormat) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(x.Spec())
	in := req.Inputs["input"]
	if strings.TrimSpace(in) == "" {
		return tool.Result{Lang: "xml"}, nil
	}
	if err := ValidateXML(in); err != nil {
		return tool.Result{}, err
	}
	toks := tokenizeXML(in)
	var out string
	if n := opts.Int("indent"); n > 0 {
		out = indentXML(ctx, toks, strings.Repeat(" ", n))
	} else {
		out = minifyXML(toks)
	}
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	return tool.Result{
		Text: out,
		Lang: "xml",
		Meta: []tool.KV{
			{Key: "XML", Value: "well-formed"},
			{Key: "Size", Value: fmt.Sprintf("%d B → %d B", len(in), len(out))},
		},
	}, nil
}

// ValidateXML checks that in is well-formed XML with at least one element.
// Errors are InputErrors with the line and, as far as encoding/xml tells,
// the column where the decoder stopped. Other XML tools reuse it.
func ValidateXML(in string) error {
	dec := xml.NewDecoder(strings.NewReader(in))
	dec.Strict = true
	dec.Entity = dtdEntities(in)
	roots, depth := 0, 0
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			ie := &tool.InputError{Input: "input", Line: 1, Col: 1, Msg: err.Error()}
			line, col := dec.InputPos()
			var syn *xml.SyntaxError
			if errors.As(err, &syn) {
				ie.Line, ie.Msg = syn.Line, syn.Msg
			}
			if line == ie.Line {
				ie.Col = max(col, 1) // bytes into the line: close enough for the caret
			} else {
				ie.Line, ie.Col = line, max(col, 1)
			}
			return ie
		}
		switch tok.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
			}
			depth++
		case xml.EndElement:
			depth--
		}
	}
	if roots == 0 {
		return &tool.InputError{Input: "input", Line: 1, Col: 1, Msg: "there are no elements"}
	}
	return nil
}

var entityRe = regexp.MustCompile(`<!ENTITY\s+([\w.:-]+)\s+("([^"]*)"|'([^']*)')\s*>`)

// dtdEntities reads the internal entities declared in the DOCTYPE, which
// encoding/xml does not resolve on its own.
func dtdEntities(in string) map[string]string {
	ents := map[string]string{}
	for _, m := range entityRe.FindAllStringSubmatch(in, -1) {
		ents[m[1]] = m[3] + m[4]
	}
	return ents
}

type xmlKind int

const (
	xText xmlKind = iota
	xStart
	xEnd
	xEmpty
	xOther // comment, CDATA, PI, DOCTYPE
)

type xmlToken struct {
	kind xmlKind
	raw  string
}

// tokenizeXML splits the (already validated) input into tokens with their raw text.
func tokenizeXML(s string) []xmlToken {
	var toks []xmlToken
	for i := 0; i < len(s); {
		if s[i] != '<' {
			j := strings.IndexByte(s[i:], '<')
			if j < 0 {
				j = len(s) - i
			}
			toks = append(toks, xmlToken{xText, s[i : i+j]})
			i += j
			continue
		}
		var end int
		kind := xOther
		switch rest := s[i:]; {
		case strings.HasPrefix(rest, "<!--"):
			end = indexAfter(rest, "-->")
		case strings.HasPrefix(rest, "<![CDATA["):
			end = indexAfter(rest, "]]>")
		case strings.HasPrefix(rest, "<?"):
			end = indexAfter(rest, "?>")
		case strings.HasPrefix(rest, "<!"):
			end = declEnd(rest)
		default:
			end = tagEnd(rest)
			switch {
			case strings.HasPrefix(rest, "</"):
				kind = xEnd
			case strings.HasSuffix(rest[:end], "/>"):
				kind = xEmpty
			default:
				kind = xStart
			}
		}
		toks = append(toks, xmlToken{kind, s[i : i+end]})
		i += end
	}
	return toks
}

func indexAfter(s, sep string) int {
	if j := strings.Index(s, sep); j >= 0 {
		return j + len(sep)
	}
	return len(s)
}

// tagEnd finds the '>' that closes the tag, skipping quotes.
func tagEnd(s string) int {
	var quote byte
	for i := 1; i < len(s); i++ {
		switch c := s[i]; {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '>':
			return i + 1
		}
	}
	return len(s)
}

// declEnd closes a <!DOCTYPE …>, which may carry an internal subset [ … ].
func declEnd(s string) int {
	var quote byte
	brackets := 0
	for i := 2; i < len(s); i++ {
		switch c := s[i]; {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '[':
			brackets++
		case c == ']':
			brackets--
		case c == '>' && brackets <= 0:
			return i + 1
		}
	}
	return len(s)
}

func indentXML(ctx context.Context, toks []xmlToken, unit string) string {
	var b strings.Builder
	depth := 0
	line := func(s string) {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(strings.Repeat(unit, depth))
		b.WriteString(s)
	}
	for i := 0; i < len(toks); i++ {
		if i%4096 == 0 && ctx.Err() != nil {
			return ""
		}
		t := toks[i]
		switch t.kind {
		case xText:
			if txt := strings.TrimSpace(t.raw); txt != "" {
				line(txt)
			}
		case xStart:
			// <a>text</a>, <a><![CDATA[…]]></a> and <a></a> stay on one line.
			if i+2 < len(toks) && toks[i+2].kind == xEnd {
				next := toks[i+1]
				if next.kind == xText && !strings.Contains(strings.TrimSpace(next.raw), "\n") {
					line(t.raw + strings.TrimSpace(next.raw) + toks[i+2].raw)
					i += 2
					continue
				}
				if next.kind == xOther && strings.HasPrefix(next.raw, "<![CDATA[") {
					line(t.raw + next.raw + toks[i+2].raw)
					i += 2
					continue
				}
			}
			if i+1 < len(toks) && toks[i+1].kind == xEnd {
				line(t.raw + toks[i+1].raw)
				i++
				continue
			}
			line(t.raw)
			depth++
		case xEnd:
			depth = max(depth-1, 0)
			line(t.raw)
		default:
			line(t.raw)
		}
	}
	return b.String()
}

func minifyXML(toks []xmlToken) string {
	var b strings.Builder
	for _, t := range toks {
		if t.kind == xText && strings.TrimSpace(t.raw) == "" {
			continue
		}
		b.WriteString(t.raw)
	}
	return b.String()
}

// Detect recognises XML that tokenises without errors in its first 50 tokens
// and has at least one element.
func (XMLFormat) Detect(sample string) float64 {
	s := strings.TrimSpace(sample)
	if !strings.HasPrefix(s, "<") {
		return 0
	}
	dec := xml.NewDecoder(strings.NewReader(s))
	dec.Entity = dtdEntities(s)
	elements := 0
	for range 50 {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		var syn *xml.SyntaxError
		if errors.As(err, &syn) && syn.Msg == "unexpected EOF" && elements > 0 {
			break // the sample (up to 64 KB) may be truncated
		}
		if err != nil {
			return 0
		}
		if _, ok := tok.(xml.StartElement); ok {
			elements++
		}
	}
	if elements == 0 {
		return 0
	}
	return 0.85
}
