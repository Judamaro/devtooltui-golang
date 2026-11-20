package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

func TestSwatch(t *testing.T) {
	res := tool.Result{Text: "HEX:      #663399", Lang: "color", Meta: []tool.KV{{Key: "Hex", Value: "#663399"}}}
	out, noColor := Result(res, tool.PlainText, nil, Styles{})
	if noColor {
		t.Fatal("noColor for a swatch")
	}
	if !strings.Contains(out, "48;2;102;51;153") {
		t.Errorf("no truecolor background for #663399: %q", out)
	}
	lines := strings.Split(ansi.Strip(out), "\n")
	if len(lines) != 3 || strings.TrimSpace(lines[0]) != "" || lines[2] != res.Text {
		t.Errorf("want two swatch rows and the text, got %q", lines)
	}
	// Without the Hex meta there is nothing to draw.
	if out, _ := Result(tool.Result{Text: "x", Lang: "color"}, tool.PlainText, nil, Styles{}); out != "" {
		t.Errorf("rendered %q without Hex", out)
	}
}
