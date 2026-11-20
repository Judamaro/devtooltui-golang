package text

import (
	"context"
	"fmt"
	"strings"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Markdown previews Markdown. Run only passes the text through with Lang
// "markdown": render draws it with Glamour at the panel's width, in the run
// goroutine, like any other colouring. Without colour (pipes, NO_COLOR) the
// output is the source.
type Markdown struct{}

func (Markdown) Spec() tool.Spec {
	return tool.Spec{
		ID:          "markdown",
		Name:        "Markdown preview",
		Category:    tool.Text,
		Description: "Preview Markdown in the terminal (headings, lists, tables, code)",
		Keywords:    []string{"md", "readme", "preview", "glamour", "commonmark", "gfm"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Markdown", Lang: "markdown", Multiline: true}},
		Output:      tool.Highlighted,
		Live:        true,
	}
}

func (m Markdown) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	in := req.Inputs["input"]
	headings := 0
	fence := false
	for _, l := range strings.Split(in, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fence = !fence
		}
		if !fence && strings.HasPrefix(t, "#") {
			headings++
		}
	}
	return tool.Result{
		Text: in,
		Lang: "markdown",
		Meta: []tool.KV{
			{Key: "Words", Value: fmt.Sprint(len(strings.Fields(in)))},
			{Key: "Headings", Value: fmt.Sprint(headings)},
		},
	}, nil
}
