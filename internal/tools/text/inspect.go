package text

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Inspect counts characters, words and lines, and sorts, deduplicates or
// reverses lines.
type Inspect struct{}

func (Inspect) Spec() tool.Spec {
	return tool.Spec{
		ID:          "inspect",
		Name:        "Text inspector",
		Category:    tool.Text,
		Description: "Count characters, words and lines; sort, deduplicate or reverse lines",
		Keywords:    []string{"count", "stats", "wc", "sort", "uniq", "dedupe", "lines", "words"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Input", Multiline: true}},
		Options: []tool.Option{
			{Key: "action", Label: "Action", Kind: tool.Select, Choices: []string{"stats", "sort", "dedupe", "reverse"}, Default: "stats"},
			{Key: "ignore_case", Label: "Ignore case", Help: "When sorting and deduplicating", Kind: tool.Toggle, Default: false},
		},
		Output: tool.PlainText,
		Live:   true,
	}
}

func (i Inspect) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(i.Spec())
	in := req.Inputs["input"]
	if opts.String("action") == "stats" {
		return stats(in), nil
	}

	lines, eol, trailing := textLines(in)
	before := len(lines)
	key := func(s string) string { return s }
	if opts.Bool("ignore_case") {
		key = strings.ToLower
	}
	var meta []tool.KV
	switch opts.String("action") {
	case "sort":
		slices.SortStableFunc(lines, func(a, b string) int {
			return cmp.Or(strings.Compare(key(a), key(b)), strings.Compare(a, b))
		})
	case "dedupe":
		seen := make(map[string]bool, len(lines))
		lines = slices.DeleteFunc(lines, func(l string) bool {
			k := key(l)
			dup := seen[k]
			seen[k] = true
			return dup
		})
		meta = append(meta, tool.KV{Key: "Removed", Value: fmt.Sprint(before - len(lines))})
	case "reverse":
		slices.Reverse(lines)
	}
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	out := strings.Join(lines, eol)
	if trailing && len(lines) > 0 {
		out += eol
	}
	return tool.Result{
		Text: out,
		Meta: append([]tool.KV{{Key: "Lines", Value: fmt.Sprintf("%d → %d", before, len(lines))}}, meta...),
	}, nil
}

// textLines splits on LF or CRLF (CRLF when every line break is CRLF;
// with mixed endings the CRs stay in the lines) and reports whether the
// text ends with a line break. The empty text has no lines.
func textLines(s string) (lines []string, eol string, trailing bool) {
	eol = "\n"
	if n := strings.Count(s, "\n"); n > 0 && strings.Count(s, "\r\n") == n {
		eol = "\r\n"
	}
	if s == "" {
		return nil, eol, false
	}
	trailing = strings.HasSuffix(s, "\n")
	return strings.Split(strings.TrimSuffix(s, eol), eol), eol, trailing
}

func stats(s string) tool.Result {
	lines, _, _ := textLines(s)
	blank, longest, longestAt := 0, 0, 0
	distinct := make(map[string]bool, len(lines))
	for n, l := range lines {
		l = strings.TrimSuffix(l, "\r")
		distinct[l] = true
		if strings.TrimSpace(l) == "" {
			blank++
		}
		if w := utf8.RuneCountInString(l); w > longest {
			longest, longestAt = w, n+1
		}
	}
	chars := utf8.RuneCountInString(s)
	words := len(strings.Fields(s))
	rows := []string{
		statRow("Characters", fmt.Sprint(chars)),
		statRow("Bytes", fmt.Sprint(len(s))),
		statRow("Words", fmt.Sprint(words)),
		statRow("Lines", fmt.Sprint(len(lines))),
		statRow("Blank lines", fmt.Sprint(blank)),
		statRow("Unique lines", fmt.Sprint(len(distinct))),
	}
	if longestAt > 0 {
		rows = append(rows, statRow("Longest line", fmt.Sprintf("%d characters (line %d)", longest, longestAt)))
	}
	rows = append(rows, statRow("Line endings", lineEndings(s)), statRow("Encoding", encoding(s)))
	return tool.Result{
		Text: strings.Join(rows, "\n"),
		Meta: []tool.KV{
			{Key: "Characters", Value: fmt.Sprint(chars)},
			{Key: "Words", Value: fmt.Sprint(words)},
			{Key: "Lines", Value: fmt.Sprint(len(lines))},
		},
	}
}

func statRow(label, value string) string { return fmt.Sprintf("%-13s %s", label+":", value) }

func lineEndings(s string) string {
	crlf := strings.Count(s, "\r\n")
	lf := strings.Count(s, "\n") - crlf
	cr := strings.Count(s, "\r") - crlf
	var kinds []string
	for _, k := range []struct {
		name string
		n    int
	}{{"LF", lf}, {"CRLF", crlf}, {"CR", cr}} {
		if k.n > 0 {
			kinds = append(kinds, fmt.Sprintf("%s (%d)", k.name, k.n))
		}
	}
	switch len(kinds) {
	case 0:
		return "none"
	case 1:
		return kinds[0]
	}
	return "mixed: " + strings.Join(kinds, ", ")
}

func encoding(s string) string {
	bom := strings.HasPrefix(s, "\uFEFF")
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			line, col := tool.Position(s, i)
			return fmt.Sprintf("not valid UTF-8 (first bad byte at %d:%d)", line, col)
		}
		i += size
	}
	ascii := true
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	switch {
	case bom:
		return "UTF-8 with BOM"
	case ascii:
		return "ASCII"
	}
	return "UTF-8"
}
