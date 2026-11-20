package encoders_test

import (
	"context"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/encoders"
)

func runHTML(t *testing.T, in string, opts tool.Values) tool.Result {
	t.Helper()
	res, err := encoders.HTML{}.Run(context.Background(), tool.Request{
		Inputs: map[string]string{"input": in},
		Opts:   opts,
	})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestHTMLGolden(t *testing.T) {
	tests := []struct {
		name string
		opts tool.Values
	}{
		{"encode-tags", nil},
		{"encode-ascii", tool.Values{"ascii": true}},
		{"decode-mixed", tool.Values{"mode": "decode"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := runHTML(t, testutil.ReadInput(t, "html", tt.name), tt.opts)
			testutil.Golden(t, "html", tt.name, res.Text)
		})
	}
}

func TestHTML(t *testing.T) {
	tests := []struct {
		name string
		in   string
		opts tool.Values
		want string
	}{
		{"encode empty", "", nil, ""},
		{"encode specials", `<a href="x">'&'</a>`, nil, "&lt;a href=&#34;x&#34;&gt;&#39;&amp;&#39;&lt;/a&gt;"},
		{"encode keeps unicode", "café 🍕", nil, "café 🍕"},
		{"encode ascii", "café 🍕", tool.Values{"ascii": true}, "caf&#233; &#127829;"},
		{"encode ascii keeps C1 controls", "\u0085", tool.Values{"ascii": true}, "\u0085"},
		{"encode ascii keeps invalid UTF-8", "a\xffb", tool.Values{"ascii": true}, "a\xffb"},
		{"decode named", "&lt;b&gt; &amp;amp; &eacute;", tool.Values{"mode": "decode"}, "<b> &amp; é"},
		{"decode numeric", "&#65;&#x42;&#X43;", tool.Values{"mode": "decode"}, "ABC"},
		{"decode keeps unknown", "&nope; & &", tool.Values{"mode": "decode"}, "&nope; & &"},
		{"decode without semicolon", "&amp &lt", tool.Values{"mode": "decode"}, "& <"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runHTML(t, tt.in, tt.opts).Text; got != tt.want {
				t.Errorf("Text = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHTMLMeta(t *testing.T) {
	res := runHTML(t, "&lt;&nope;&#65;", tool.Values{"mode": "decode"})
	if got := res.Meta[2]; got.Key != "Entities" || got.Value != "2" {
		t.Errorf("Meta = %v, want Entities: 2", res.Meta)
	}
}

func TestHTMLDetect(t *testing.T) {
	tests := []struct {
		name   string
		sample string
		want   float64
	}{
		{"named", "Fish &amp; chips", 0.55},
		{"numeric", "&#128512;", 0.55},
		{"unknown only", "&nope; &foo;", 0},
		{"bare ampersand", "Fish & chips", 0},
		{"empty", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (encoders.HTML{}).Detect(tt.sample); got != tt.want {
				t.Errorf("Detect(%q) = %v, want %v", tt.sample, got, tt.want)
			}
		})
	}
}

func FuzzHTMLRoundTrip(f *testing.F) {
	for _, s := range []string{"", "<a>&amp;</a>", "café 🍕", "\x85\xff", "&#0;"} {
		f.Add(s, false)
		f.Add(s, true)
	}
	f.Fuzz(func(t *testing.T, s string, ascii bool) {
		enc := runHTML(t, s, tool.Values{"ascii": ascii})
		dec := runHTML(t, enc.Text, tool.Values{"mode": "decode"})
		if dec.Text != s {
			t.Errorf("round trip: %q → %q → %q", s, enc.Text, dec.Text)
		}
	})
}
