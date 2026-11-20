// Package testers groups the tools for testing expressions.
package testers

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"strconv"
	"strings"
	"time"

	"github.com/dlclark/regexp2/v2"
	syntax2 "github.com/dlclark/regexp2/v2/syntax"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// MaxMatches caps the listed matches.
const MaxMatches = 10000

// Regex tests RE2 regular expressions against a text.
type Regex struct{}

func (Regex) Spec() tool.Spec {
	return tool.Spec{
		ID:          "regex",
		Name:        "RegEx",
		Category:    tool.Testers,
		Description: "Test regular expressions with RE2 or regexp2 (.NET: lookaround, backreferences)",
		Keywords:    []string{"regexp", "regular", "expression", "match", "re2", "grep", "pcre", "dotnet", "lookahead"},
		Inputs:      []tool.InputSpec{{Key: "text", Label: "Text", Multiline: true}},
		Options: []tool.Option{
			{Key: "pattern", Label: "Pattern", Kind: tool.String, Default: ""},
			{Key: "engine", Label: "Engine", Help: "re2: Go's linear-time engine; regexp2: .NET syntax with lookaround and backreferences (1 s limit per match)", Kind: tool.Select, Choices: []string{"re2", "regexp2"}, Default: "re2"},
			{Key: "ignore_case", Label: "i", Help: "Ignore case", Kind: tool.Toggle, Default: false},
			{Key: "multiline", Label: "m", Help: "^ and $ per line", Kind: tool.Toggle, Default: false},
			{Key: "dotall", Label: "s", Help: ". matches \\n", Kind: tool.Toggle, Default: false},
		},
		Output: tool.Matches,
		Live:   true,
	}
}

func (r Regex) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(r.Spec())
	pattern := opts.String("pattern")
	if pattern == "" {
		return tool.Result{Text: "Type a pattern in the options."}, nil
	}
	ignoreCase, multiline, dotall := opts.Bool("ignore_case"), opts.Bool("multiline"), opts.Bool("dotall")
	text := req.Inputs["text"]
	var found matchSet
	var err error
	if opts.String("engine") == "regexp2" {
		found, err = matchRegexp2(ctx, pattern, text, ignoreCase, multiline, dotall)
	} else {
		found, err = matchRE2(pattern, text, ignoreCase, multiline, dotall)
	}
	if err != nil {
		return tool.Result{}, err
	}
	matches, names := found.matches, found.names
	pos := newPosTracker(text)

	var b strings.Builder
	var spans []tool.Span
	fmt.Fprintf(&b, "%d match", len(matches))
	if len(matches) != 1 {
		b.WriteString("es")
	}
	if len(matches) == MaxMatches {
		fmt.Fprintf(&b, " (limit of %d)", MaxMatches)
	}
	for i, m := range matches {
		if i%1024 == 0 && ctx.Err() != nil {
			return tool.Result{}, ctx.Err()
		}
		sl, sc := pos.at(m[0])
		el, ec := pos.at(m[1])
		fmt.Fprintf(&b, "\n#%d  %d:%d–%d:%d  %s", i+1, sl, sc, el, ec, quote(text[m[0]:m[1]]))
		spans = append(spans, tool.Span{Input: "text", Start: m[0], End: m[1]})
		for g := 1; g < len(m)/2; g++ {
			label := strconv.Itoa(g)
			if names[g] != "" {
				label += " (" + names[g] + ")"
			}
			if m[2*g] < 0 {
				fmt.Fprintf(&b, "\n    %s: —", label)
				continue
			}
			fmt.Fprintf(&b, "\n    %s: %s", label, quote(text[m[2*g]:m[2*g+1]]))
			spans = append(spans, tool.Span{Input: "text", Start: m[2*g], End: m[2*g+1], Group: g})
		}
	}
	return tool.Result{
		Text:  b.String(),
		Spans: spans,
		Meta: []tool.KV{
			{Key: "Matches", Value: strconv.Itoa(len(matches))},
			{Key: "Groups", Value: strconv.Itoa(len(names) - 1)},
		},
	}, nil
}

// matchSet is what both engines give: up to MaxMatches matches, each with
// [start, end] byte pairs for the whole match and every group (-1 when a
// group did not take part), and the group names (index 0 is the match).
type matchSet struct {
	matches [][]int
	names   []string
}

func matchRE2(pattern, text string, ignoreCase, multiline, dotall bool) (matchSet, error) {
	var flags string
	for _, f := range []struct {
		on   bool
		flag string
	}{{ignoreCase, "i"}, {multiline, "m"}, {dotall, "s"}} {
		if f.on {
			flags += f.flag
		}
	}
	full := pattern
	if flags != "" {
		full = "(?" + flags + ")" + pattern
	}
	re, err := regexp.Compile(full)
	if err != nil {
		return matchSet{}, patternError(pattern, err)
	}
	return matchSet{matches: re.FindAllStringSubmatchIndex(text, MaxMatches), names: re.SubexpNames()}, nil
}

// Regexp2Timeout bounds each regexp2 match: backtracking can be exponential.
const Regexp2Timeout = time.Second

func matchRegexp2(ctx context.Context, pattern, text string, ignoreCase, multiline, dotall bool) (matchSet, error) {
	var o regexp2.RegexOptions
	if ignoreCase {
		o |= regexp2.IgnoreCase
	}
	if multiline {
		o |= regexp2.Multiline
	}
	if dotall {
		o |= regexp2.Singleline
	}
	re, err := regexp2.Compile(pattern, o)
	if err != nil {
		ie := &tool.InputError{Input: "pattern", Line: 1, Col: 1, Msg: err.Error()}
		var pe *syntax2.Error
		if errors.As(err, &pe) {
			ie.Msg = pe.Code.String()
			if len(pe.Args) > 0 {
				ie.Msg = fmt.Sprintf(ie.Msg, pe.Args...)
			}
		}
		return matchSet{}, ie
	}
	re.MatchTimeout = Regexp2Timeout
	nums := re.GetGroupNumbers()
	names := make([]string, len(nums))
	for i, n := range nums {
		if name := re.GroupNameFromNumber(n); name != strconv.Itoa(n) {
			names[i] = name
		}
	}
	var set matchSet
	set.names = names
	m, err := re.FindStringMatch(text)
	for m != nil && err == nil && len(set.matches) < MaxMatches {
		if len(set.matches)%256 == 0 {
			if cerr := ctx.Err(); cerr != nil {
				return matchSet{}, cerr
			}
		}
		groups := m.Groups()
		pairs := make([]int, 2*len(groups))
		for g, grp := range groups {
			if len(grp.Captures) == 0 {
				pairs[2*g], pairs[2*g+1] = -1, -1
				continue
			}
			start, length := grp.ByteRange()
			pairs[2*g], pairs[2*g+1] = start, start+length
		}
		set.matches = append(set.matches, pairs)
		m, err = re.FindNextMatch(m)
	}
	if errors.Is(err, regexp2.ErrMatchTimeout) {
		return matchSet{}, &tool.InputError{Input: "pattern", Line: 1, Col: 1,
			Msg: fmt.Sprintf("a match took more than %s (catastrophic backtracking?)", Regexp2Timeout)}
	}
	if err != nil {
		return matchSet{}, &tool.InputError{Input: "pattern", Line: 1, Col: 1, Msg: err.Error()}
	}
	return set, nil
}

// quote shows the match in quotes, trimmed when long.
func quote(s string) string {
	const maxRunes = 60
	if r := []rune(s); len(r) > maxRunes {
		return strconv.Quote(string(r[:maxRunes])) + "…"
	}
	return strconv.Quote(s)
}

// patternError locates the error in the pattern: RE2 gives no offsets, but
// it does give the offending fragment.
func patternError(pattern string, err error) error {
	ie := &tool.InputError{Input: "pattern", Line: 1, Msg: err.Error()}
	var se *syntax.Error
	if !errors.As(err, &se) {
		return ie
	}
	ie.Msg = string(se.Code)
	if se.Expr != "" {
		ie.Msg += ": " + se.Expr
		if i := strings.Index(pattern, se.Expr); i >= 0 {
			_, ie.Col = tool.Position(pattern, i)
		}
	}
	switch {
	case strings.HasPrefix(se.Expr, "(?<=") || strings.HasPrefix(se.Expr, "(?<!"):
		// Since Go 1.22 "(?<" opens a named group: it arrives as
		// ErrInvalidNamedCapture.
		ie.Msg += " (RE2 does not support lookbehind)"
	case se.Code == syntax.ErrInvalidPerlOp && (strings.HasPrefix(se.Expr, "(?=") || strings.HasPrefix(se.Expr, "(?!")):
		ie.Msg += " (RE2 does not support lookahead)"
	case se.Code == syntax.ErrInvalidEscape && len(se.Expr) == 2 && se.Expr[1] >= '1' && se.Expr[1] <= '9':
		ie.Msg += " (RE2 does not support backreferences)"
	}
	return ie
}

// posTracker turns increasing offsets into line:column (in runes) in a
// single pass.
type posTracker struct {
	text      string
	off       int
	line, col int
}

func newPosTracker(text string) *posTracker { return &posTracker{text: text, line: 1, col: 1} }

func (p *posTracker) at(offset int) (int, int) {
	if offset < p.off { // cannot happen: offsets arrive sorted
		return tool.Position(p.text, offset)
	}
	for _, r := range p.text[p.off:offset] {
		if r == '\n' {
			p.line++
			p.col = 1
		} else {
			p.col++
		}
	}
	p.off = offset
	return p.line, p.col
}
