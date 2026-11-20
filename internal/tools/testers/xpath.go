package testers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/antchfx/xpath"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/formatters"
)

// MaxXPathNodes caps the listed nodes.
const MaxXPathNodes = 10000

// XPath evaluates an XPath 1.0 expression (with antchfx extensions) on an
// XML document. Node sets come out one node per line; numbers, strings and
// booleans as the value.
type XPath struct{}

func (XPath) Spec() tool.Spec {
	return tool.Spec{
		ID:          "xpath",
		Name:        "XPath",
		Category:    tool.Testers,
		Description: "Query XML with XPath (//book[price < 10]/title, count(//item))",
		Keywords:    []string{"xml", "query", "xquery", "select", "nodes"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "XML", Lang: "xml", Multiline: true}},
		Options: []tool.Option{
			{Key: "expr", Label: "Expression", Help: "XPath 1.0; / is the document", Kind: tool.String, Default: "/"},
		},
		Output: tool.Highlighted,
		Live:   true,
	}
}

func (x XPath) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(x.Spec())
	src := opts.String("expr")
	expr := strings.TrimSpace(src)
	if expr == "" {
		expr = "/"
	}
	compiled, err := xpath.Compile(expr)
	if err != nil {
		lead := len(src) - len(strings.TrimLeft(src, " \t\r\n"))
		_, col := tool.Position(src, lead)
		return tool.Result{}, &tool.InputError{Input: "expr", Line: 1, Col: col, Msg: err.Error()}
	}
	in := req.Inputs["input"]
	if strings.TrimSpace(in) == "" {
		return tool.Result{Lang: "xml"}, nil
	}
	// encoding/xml validates with positions; xmlquery would only say "XML
	// syntax error".
	if err := formatters.ValidateXML(in); err != nil {
		return tool.Result{}, err
	}
	doc, err := xmlquery.Parse(strings.NewReader(in))
	if err != nil {
		return tool.Result{}, &tool.InputError{Input: "input", Line: 1, Col: 1, Msg: err.Error()}
	}
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}

	switch v := compiled.Evaluate(xmlquery.CreateXPathNavigator(doc)).(type) {
	case *xpath.NodeIterator:
		return nodeResult(ctx, v)
	case float64:
		return scalar(strconv.FormatFloat(v, 'f', -1, 64), "number"), nil
	case string:
		return scalar(v, "string"), nil
	case bool:
		return scalar(strconv.FormatBool(v), "boolean"), nil
	default:
		return scalar(fmt.Sprint(v), "value"), nil
	}
}

func scalar(text, kind string) tool.Result {
	return tool.Result{Text: text, Meta: []tool.KV{{Key: "Result", Value: kind}}}
}

// nodeResult writes each node on its own: elements as XML, attributes as
// name="value", text and comments as they are. The output is coloured as
// XML only when every node is an element.
func nodeResult(ctx context.Context, it *xpath.NodeIterator) (tool.Result, error) {
	var parts []string
	elements := true
	for it.MoveNext() {
		if len(parts)%256 == 0 {
			if err := ctx.Err(); err != nil {
				return tool.Result{}, err
			}
		}
		if len(parts) == MaxXPathNodes {
			parts = append(parts, fmt.Sprintf("… more than %d nodes", MaxXPathNodes))
			break
		}
		nav := it.Current()
		switch nav.NodeType() {
		case xpath.ElementNode:
			parts = append(parts, prettyXML(ctx, nav.(*xmlquery.NodeNavigator).Current().OutputXML(true)))
		case xpath.RootNode:
			parts = append(parts, prettyXML(ctx, nav.(*xmlquery.NodeNavigator).Current().OutputXML(false)))
		case xpath.AttributeNode:
			elements = false
			name := nav.LocalName()
			if nav.Prefix() != "" {
				name = nav.Prefix() + ":" + name
			}
			parts = append(parts, fmt.Sprintf("%s=%q", name, nav.Value()))
		default: // text, comments, processing instructions
			elements = false
			parts = append(parts, nav.Value())
		}
	}
	lang := ""
	if elements && len(parts) > 0 {
		lang = "xml"
	}
	count := len(parts)
	if count > MaxXPathNodes {
		count = MaxXPathNodes
	}
	return tool.Result{
		Text: strings.Join(parts, "\n"),
		Lang: lang,
		Meta: []tool.KV{{Key: "Nodes", Value: fmt.Sprint(count)}},
	}, nil
}

// prettyXML re-indents a matched element with xml-format, so it does not
// keep the indentation it had inside the document.
func prettyXML(ctx context.Context, s string) string {
	res, err := formatters.XMLFormat{}.Run(ctx, tool.Request{Inputs: map[string]string{"input": s}})
	if err != nil {
		return s
	}
	return res.Text
}
