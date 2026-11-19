// Package text groups the tools that work on plain text.
package text

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sergi/go-diff/diffmatchpatch"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Diff compares two texts by lines or by words.
type Diff struct{}

// Result.Lang values the TUI uses to colour the output.
const (
	LangUnified = "diff"
	LangSide    = "diff-side"
)

const contextLines = 3

func (Diff) Spec() tool.Spec {
	return tool.Spec{
		ID:          "diff",
		Name:        "Diff",
		Category:    tool.Text,
		Description: "Compare two texts by lines or by words",
		Keywords:    []string{"compare", "differences", "patch", "unified"},
		Inputs: []tool.InputSpec{
			{Key: "a", Label: "Text A (original)", Multiline: true},
			{Key: "b", Label: "Text B (new)", Multiline: true},
		},
		Options: []tool.Option{
			{Key: "by", Label: "By", Kind: tool.Select, Choices: []string{"lines", "words"}, Default: "lines"},
			{Key: "ignore_space", Label: "Ignore spaces", Kind: tool.Toggle, Default: false},
			{Key: "view", Label: "View", Help: "Lines only; words are always inline", Kind: tool.Select,
				Choices: []string{"unified", "side-by-side"}, Default: "unified"},
		},
		Output: tool.DiffView,
		Live:   true,
	}
}

func (d Diff) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(d.Spec())
	a, b := req.Inputs["a"], req.Inputs["b"]
	ignore := opts.Bool("ignore_space")

	if opts.String("by") == "words" {
		ops := diffTokens(ctx, splitWords(a), splitWords(b), wordKey(ignore))
		ins, del := countKinds(ops, func(s string) bool { return strings.TrimSpace(s) != "" })
		return tool.Result{Text: inlineWords(ops), Diff: ops, Meta: changeMeta(ins, del, "words")}, ctx.Err()
	}

	la, lb := splitLines(a), splitLines(b)
	ops := diffTokens(ctx, la, lb, lineKey(ignore))
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	rows := toRows(ops)
	ins, del := 0, 0
	for _, r := range rows {
		switch r.kind {
		case tool.Insert:
			ins++
		case tool.Delete:
			del++
		}
	}
	res := tool.Result{Diff: ops, Meta: changeMeta(ins, del, "lines")}
	switch {
	case ins+del == 0:
		res.Text = "No differences"
	case opts.String("view") == "side-by-side":
		res.Text, res.Lang = sideBySide(rows), LangSide
	default:
		res.Text, res.Lang = unified(rows), LangUnified
	}
	return res, nil
}

func changeMeta(ins, del int, unit string) []tool.KV {
	if ins+del == 0 {
		return []tool.KV{{Key: "Result", Value: "identical"}}
	}
	return []tool.KV{{Key: "Changes", Value: fmt.Sprintf("+%d −%d %s", ins, del, unit)}}
}

// splitLines returns the lines ending in "\n" (the last one too, with or
// without a final break), so A and B compare the same way.
func splitLines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] += "\n"
	}
	return lines
}

var wordRe = regexp.MustCompile(`\s+|[^\s]+`)

func splitWords(s string) []string { return wordRe.FindAllString(s, -1) }

func lineKey(ignore bool) func(string) string {
	return func(s string) string {
		s = strings.TrimSuffix(s, "\n")
		if ignore {
			return strings.Join(strings.Fields(s), " ")
		}
		return s
	}
}

func wordKey(ignore bool) func(string) string {
	return func(s string) string {
		if ignore && strings.TrimSpace(s) == "" {
			return " "
		}
		return s
	}
}

// diffTokens compares token sequences: each distinct key maps to a rune (like
// go-diff's DiffLinesToChars) and the output text comes from the original
// tokens, so ignoring spaces does not change what is shown.
func diffTokens(ctx context.Context, a, b []string, key func(string) string) []tool.DiffOp {
	ids := map[string]rune{}
	next := rune(1)
	encode := func(toks []string) []rune {
		out := make([]rune, len(toks))
		for i, t := range toks {
			k := key(t)
			id, ok := ids[k]
			if !ok {
				id = next
				next++
				ids[k] = id
			}
			out[i] = id
		}
		return out
	}
	ra, rb := encode(a), encode(b)
	if ctx.Err() != nil {
		return nil
	}
	dmp := diffmatchpatch.New()
	dmp.DiffTimeout = 2 * time.Second
	var ops []tool.DiffOp
	ai, bi := 0, 0
	add := func(kind tool.DiffKind, text string) {
		if n := len(ops); n > 0 && ops[n-1].Kind == kind {
			ops[n-1].Text += text
			return
		}
		ops = append(ops, tool.DiffOp{Kind: kind, Text: text})
	}
	for _, d := range dmp.DiffMainRunes(ra, rb, false) {
		n := utf8.RuneCountInString(d.Text)
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			add(tool.Equal, strings.Join(a[ai:ai+n], ""))
			ai, bi = ai+n, bi+n
		case diffmatchpatch.DiffDelete:
			add(tool.Delete, strings.Join(a[ai:ai+n], ""))
			ai += n
		case diffmatchpatch.DiffInsert:
			add(tool.Insert, strings.Join(b[bi:bi+n], ""))
			bi += n
		}
	}
	return ops
}

func countKinds(ops []tool.DiffOp, count func(string) bool) (ins, del int) {
	for _, op := range ops {
		if op.Kind == tool.Equal {
			continue
		}
		n := 0
		for _, w := range splitWords(op.Text) {
			if count(w) {
				n++
			}
		}
		if op.Kind == tool.Insert {
			ins += n
		} else {
			del += n
		}
	}
	return ins, del
}

func inlineWords(ops []tool.DiffOp) string {
	var b strings.Builder
	for _, op := range ops {
		switch op.Kind {
		case tool.Equal:
			b.WriteString(op.Text)
		case tool.Delete:
			b.WriteString("[-" + op.Text + "-]")
		case tool.Insert:
			b.WriteString("{+" + op.Text + "+}")
		}
	}
	return b.String()
}

// row is a diff line with its numbers in A and B (0 = not applicable).
type row struct {
	kind  tool.DiffKind
	text  string
	aLine int
	bLine int
}

func toRows(ops []tool.DiffOp) []row {
	var rows []row
	an, bn := 1, 1
	for _, op := range ops {
		for _, l := range splitLines(op.Text) {
			l = strings.TrimSuffix(l, "\n")
			r := row{kind: op.Kind, text: l}
			switch op.Kind {
			case tool.Equal:
				r.aLine, r.bLine = an, bn
				an, bn = an+1, bn+1
			case tool.Delete:
				r.aLine = an
				an++
			case tool.Insert:
				r.bLine = bn
				bn++
			}
			rows = append(rows, r)
		}
	}
	return rows
}

// unified builds the unified format with hunks and 3 lines of context.
func unified(rows []row) string {
	var b strings.Builder
	b.WriteString("--- A\n+++ B")
	for i := 0; i < len(rows); {
		if rows[i].kind == tool.Equal {
			i++
			continue
		}
		start := max(i-contextLines, 0)
		end := i
		// Extend the hunk while changes are less than 2×context apart.
		for j := i; j < len(rows); j++ {
			if rows[j].kind != tool.Equal {
				end = j
			} else if j-end > 2*contextLines {
				break
			}
		}
		end = min(end+contextLines+1, len(rows))
		writeHunk(&b, rows[start:end])
		i = end
	}
	return b.String()
}

func writeHunk(b *strings.Builder, rows []row) {
	aStart, bStart, aCount, bCount := 0, 0, 0, 0
	for _, r := range rows {
		if r.aLine > 0 {
			if aStart == 0 {
				aStart = r.aLine
			}
			aCount++
		}
		if r.bLine > 0 {
			if bStart == 0 {
				bStart = r.bLine
			}
			bCount++
		}
	}
	fmt.Fprintf(b, "\n@@ -%d,%d +%d,%d @@", aStart, aCount, bStart, bCount)
	for _, r := range rows {
		prefix := " "
		switch r.kind {
		case tool.Delete:
			prefix = "-"
		case tool.Insert:
			prefix = "+"
		}
		b.WriteString("\n" + prefix + r.text)
	}
}

const maxColumn = 60

// sideBySide pairs consecutive deletions and insertions as changes ("|");
// unpaired ones are marked "<" or ">". The marker goes at the start of the
// line so it is easy to colour and to filter with grep.
func sideBySide(rows []row) string {
	for i := range rows {
		rows[i].text = strings.ReplaceAll(rows[i].text, "\t", "    ")
	}
	width := 10
	for _, r := range rows {
		if r.kind != tool.Insert {
			width = max(width, utf8.RuneCountInString(r.text))
		}
	}
	width = min(width, maxColumn)
	var lines []string
	emit := func(left, mark, right string) {
		lines = append(lines, strings.TrimRight(fmt.Sprintf("%s %s │ %s", mark, pad(left, width), right), " "))
	}
	for i := 0; i < len(rows); {
		if rows[i].kind == tool.Equal {
			emit(rows[i].text, " ", rows[i].text)
			i++
			continue
		}
		var dels, ins []string
		for ; i < len(rows) && rows[i].kind == tool.Delete; i++ {
			dels = append(dels, rows[i].text)
		}
		for ; i < len(rows) && rows[i].kind == tool.Insert; i++ {
			ins = append(ins, rows[i].text)
		}
		for k := range max(len(dels), len(ins)) {
			switch {
			case k < len(dels) && k < len(ins):
				emit(dels[k], "|", ins[k])
			case k < len(dels):
				emit(dels[k], "<", "")
			default:
				emit("", ">", ins[k])
			}
		}
	}
	return strings.Join(lines, "\n")
}

func pad(s string, width int) string {
	if n := utf8.RuneCountInString(s); n > width {
		return string([]rune(s)[:width-1]) + "…"
	} else if n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}
