package text_test

import (
	"context"
	"strings"
	"testing"

	"charm.land/glamour/v2/styles"
	"github.com/charmbracelet/x/ansi"

	"github.com/Judamaro/devtooltui-golang/internal/render"
	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/text"
)

func runMarkdown(t *testing.T, in string) tool.Result {
	t.Helper()
	res, err := text.Markdown{}.Run(context.Background(), tool.Request{Inputs: map[string]string{"input": in}})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestMarkdown(t *testing.T) {
	in := testutil.ReadInput(t, "markdown", "readme")
	res := runMarkdown(t, in)
	if res.Text != in || res.Lang != "markdown" {
		t.Errorf("Run must pass the source through with Lang markdown, got Lang %q", res.Lang)
	}
	meta := map[string]string{}
	for _, kv := range res.Meta {
		meta[kv.Key] = kv.Value
	}
	if meta["Headings"] != "2" { // the # in the bash block is not a heading
		t.Errorf("Meta = %v", res.Meta)
	}
}

// TestMarkdownRendered keeps the Glamour view, without ANSI, as a golden:
// it is what the TUI shows.
func TestMarkdownRendered(t *testing.T) {
	in := testutil.ReadInput(t, "markdown", "readme")
	style := styles.DarkStyleConfig
	for _, width := range []int{60, 100} {
		out, noColor := render.Result(runMarkdown(t, in), tool.Highlighted, nil, width, render.Styles{Markdown: &style})
		if noColor || out == "" {
			t.Fatalf("width %d: not rendered", width)
		}
		lines := strings.Split(ansi.Strip(out), "\n")
		for i, l := range lines {
			lines[i] = strings.TrimRight(l, " ")
			if w := ansi.StringWidth(l); w > width {
				t.Errorf("width %d: line %d is %d columns", width, i+1, w)
			}
		}
		plain := strings.Join(lines, "\n")
		for _, want := range []string{"devtooltui", "go install", "• Works offline", "Ctrl+P", "Inspired by"} {
			if !strings.Contains(plain, want) {
				t.Errorf("width %d: missing %q", width, want)
			}
		}
		if width == 60 {
			testutil.Golden(t, "markdown", "readme-60", plain)
		}
	}
}
