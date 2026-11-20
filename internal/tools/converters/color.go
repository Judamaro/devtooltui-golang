package converters

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Color converts between HEX, RGB, HSL and HSV, names CSS colours and
// reports WCAG contrast. Result.Lang is "color" and Meta "Hex" carries the
// value, so render draws a swatch.
type Color struct{}

// ColorLang is the Result.Lang that asks render for a swatch.
const ColorLang = "color"

func (Color) Spec() tool.Spec {
	return tool.Spec{
		ID:          "color",
		Name:        "Color",
		Category:    tool.Converters,
		Description: "Convert colours between HEX, RGB, HSL and HSV, with a swatch and WCAG contrast",
		Keywords:    []string{"colour", "hex", "rgb", "hsl", "hsv", "css", "contrast"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Colour (#hex, rgb(), hsl() or a CSS name)"}},
		Output:      tool.PlainText,
		Live:        true,
	}
}

type rgba struct {
	r, g, b float64 // 0..255
	a       float64 // 0..1
}

func (c Color) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	in := req.Inputs["input"]
	s := strings.TrimSpace(in)
	if s == "" {
		return tool.Result{}, nil
	}
	lead := strings.Index(in, s)
	col, off, msg := parseColor(s)
	if msg != "" {
		line, column := tool.Position(in, lead+off)
		return tool.Result{}, &tool.InputError{Input: "input", Line: line, Col: column, Msg: msg}
	}

	r8, g8, b8 := round8(col.r), round8(col.g), round8(col.b)
	hex := fmt.Sprintf("#%02x%02x%02x", r8, g8, b8)
	h, sl, l := toHSL(col)
	hv, sv, v := toHSV(col)
	alpha := col.a < 1
	rows := []string{}
	if alpha {
		rows = append(rows,
			colorRow("HEX", fmt.Sprintf("%s%02x", hex, round8(col.a*255))),
			colorRow("RGB", fmt.Sprintf("rgba(%d, %d, %d, %s)", r8, g8, b8, fAlpha(col.a))),
			colorRow("HSL", fmt.Sprintf("hsla(%s, %s%%, %s%%, %s)", fnum(h), fnum(sl), fnum(l), fAlpha(col.a))))
	} else {
		rows = append(rows,
			colorRow("HEX", hex),
			colorRow("RGB", fmt.Sprintf("rgb(%d, %d, %d)", r8, g8, b8)),
			colorRow("HSL", fmt.Sprintf("hsl(%s, %s%%, %s%%)", fnum(h), fnum(sl), fnum(l))))
	}
	rows = append(rows, colorRow("HSV", fmt.Sprintf("hsv(%s, %s%%, %s%%)", fnum(hv), fnum(sv), fnum(v))))
	if name := nameOf(hex); name != "" && !alpha {
		rows = append(rows, colorRow("Name", name))
	}
	lum := luminance(col)
	rows = append(rows, colorRow("Contrast", fmt.Sprintf("%.2f:1 on white, %.2f:1 on black", 1.05/(lum+0.05), (lum+0.05)/0.05)))

	return tool.Result{
		Text: strings.Join(rows, "\n"),
		Lang: ColorLang,
		Meta: []tool.KV{{Key: "Hex", Value: hex}},
	}, nil
}

func colorRow(label, value string) string { return fmt.Sprintf("%-9s %s", label+":", value) }

func round8(v float64) int { return int(math.Round(math.Max(0, math.Min(255, v)))) }

// fnum prints up to one decimal, without a trailing ".0".
func fnum(v float64) string {
	return strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64)
}

// fAlpha prints alpha with up to three decimals (0.25, 0.502).
func fAlpha(a float64) string {
	return strconv.FormatFloat(math.Round(a*1000)/1000, 'f', -1, 64)
}

var funcRe = regexp.MustCompile(`^(?i)(rgba?|hsla?)\s*\(`)

// parseColor returns the colour or the byte offset in s of the problem and
// a message.
func parseColor(s string) (rgba, int, string) {
	switch {
	case strings.HasPrefix(s, "#"):
		return parseHex(s[1:], 1)
	case funcRe.MatchString(s):
		return parseFunc(s)
	}
	if c, ok := named()[strings.ToLower(s)]; ok {
		return c, 0, ""
	}
	if len(s) == 3 || len(s) == 6 || len(s) == 8 {
		if c, _, msg := parseHex(s, 0); msg == "" {
			return c, 0, "" // hex without #
		}
	}
	return rgba{}, 0, "not a colour: use #hex, rgb(), hsl() or a CSS name"
}

func parseHex(h string, base int) (rgba, int, string) {
	for i := 0; i < len(h); i++ {
		if !isHexDigit(h[i]) {
			return rgba{}, base + i, fmt.Sprintf("%q is not a hex digit", h[i])
		}
	}
	if len(h) == 3 || len(h) == 4 {
		var b strings.Builder
		for i := range len(h) {
			b.WriteByte(h[i])
			b.WriteByte(h[i])
		}
		h = b.String()
	}
	if len(h) != 6 && len(h) != 8 {
		return rgba{}, 0, "hex colours have 3, 4, 6 or 8 digits"
	}
	v, _ := strconv.ParseUint(h, 16, 64)
	if len(h) == 6 {
		v = v<<8 | 0xff
	}
	return rgba{
		r: float64(v >> 24 & 0xff), g: float64(v >> 16 & 0xff), b: float64(v >> 8 & 0xff),
		a: float64(v&0xff) / 255,
	}, 0, ""
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

type colorArg struct {
	text string
	off  int
}

// parseFunc reads rgb()/rgba()/hsl()/hsla() with commas or the CSS 4 form
// "rgb(255 0 0 / 50%)".
func parseFunc(s string) (rgba, int, string) {
	open := strings.IndexByte(s, '(')
	name := strings.ToLower(strings.TrimSpace(s[:open]))
	if !strings.HasSuffix(s, ")") {
		return rgba{}, len(s), "missing )"
	}
	var args []colorArg
	start := -1
	for i := open + 1; i <= len(s)-1; i++ {
		c := s[i]
		sep := c == ',' || c == ' ' || c == '\t' || c == '/' || c == ')'
		if !sep && start < 0 {
			start = i
		}
		if sep && start >= 0 {
			args = append(args, colorArg{s[start:i], start})
			start = -1
		}
	}
	if len(args) != 3 && len(args) != 4 {
		return rgba{}, open, fmt.Sprintf("%s() takes 3 values and an optional alpha, got %d", name, len(args))
	}
	var v [3]float64
	for i := range 3 {
		a := args[i]
		var x float64
		var ok bool
		switch {
		case strings.HasPrefix(name, "rgb"):
			x, ok = number(a.text, 255, true)
			if ok && (x < 0 || x > 255) {
				return rgba{}, a.off, fmt.Sprintf("%s is out of range (0–255 or 0–100%%)", a.text)
			}
		case i == 0:
			x, ok = number(strings.TrimSuffix(strings.ToLower(a.text), "deg"), 0, false)
			x = math.Mod(math.Mod(x, 360)+360, 360)
		default:
			x, ok = number(strings.TrimSuffix(a.text, "%"), 0, false)
			if ok && (x < 0 || x > 100) {
				return rgba{}, a.off, fmt.Sprintf("%s is out of range (0–100%%)", a.text)
			}
		}
		if !ok {
			return rgba{}, a.off, fmt.Sprintf("%q is not a number", a.text)
		}
		v[i] = x
	}
	alpha := 1.0
	if len(args) == 4 {
		x, ok := number(args[3].text, 1, true)
		if !ok || x < 0 || x > 1 {
			return rgba{}, args[3].off, fmt.Sprintf("alpha %s must be 0–1 or 0–100%%", args[3].text)
		}
		alpha = x
	}
	if strings.HasPrefix(name, "rgb") {
		return rgba{v[0], v[1], v[2], alpha}, 0, ""
	}
	c := fromHSL(v[0], v[1]/100, v[2]/100)
	c.a = alpha
	return c, 0, ""
}

// number parses a float; with pct, "50%" becomes 50% of scale.
func number(s string, scale float64, pct bool) (float64, bool) {
	if pct && strings.HasSuffix(s, "%") {
		x, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		return x / 100 * scale, err == nil && !math.IsNaN(x) && !math.IsInf(x, 0)
	}
	x, err := strconv.ParseFloat(s, 64)
	return x, err == nil && !math.IsNaN(x) && !math.IsInf(x, 0)
}

func toHSL(c rgba) (h, s, l float64) {
	r, g, b := c.r/255, c.g/255, c.b/255
	hi, lo := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l = (hi + lo) / 2
	if d := hi - lo; d > 0 {
		s = d / (1 - math.Abs(2*l-1))
		h = hue(r, g, b, hi, d)
	}
	return h, s * 100, l * 100
}

func toHSV(c rgba) (h, s, v float64) {
	r, g, b := c.r/255, c.g/255, c.b/255
	hi, lo := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	if d := hi - lo; d > 0 {
		s = d / hi
		h = hue(r, g, b, hi, d)
	}
	return h, s * 100, hi * 100
}

func hue(r, g, b, hi, d float64) float64 {
	var h float64
	switch hi {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return math.Mod(h*60+360, 360)
}

func fromHSL(h, s, l float64) rgba {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g = c, x
	case h < 120:
		r, g = x, c
	case h < 180:
		g, b = c, x
	case h < 240:
		g, b = x, c
	case h < 300:
		r, b = x, c
	default:
		r, b = c, x
	}
	return rgba{(r + m) * 255, (g + m) * 255, (b + m) * 255, 1}
}

// luminance is the WCAG 2 relative luminance.
func luminance(c rgba) float64 {
	lin := func(v float64) float64 {
		v /= 255
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.r) + 0.7152*lin(c.g) + 0.0722*lin(c.b)
}

// nameOf returns the first CSS name with that value (gray before grey).
func nameOf(hex string) string {
	if ns := namedByHex()[hex]; len(ns) > 0 {
		return ns[0]
	}
	return ""
}

// cssColors are the CSS named colours (CSS Color Module Level 4).
const cssColors = `aliceblue f0f8ff antiquewhite faebd7 aqua 00ffff aquamarine 7fffd4
azure f0ffff beige f5f5dc bisque ffe4c4 black 000000 blanchedalmond ffebcd
blue 0000ff blueviolet 8a2be2 brown a52a2a burlywood deb887 cadetblue 5f9ea0
chartreuse 7fff00 chocolate d2691e coral ff7f50 cornflowerblue 6495ed
cornsilk fff8dc crimson dc143c cyan 00ffff darkblue 00008b darkcyan 008b8b
darkgoldenrod b8860b darkgray a9a9a9 darkgreen 006400 darkgrey a9a9a9
darkkhaki bdb76b darkmagenta 8b008b darkolivegreen 556b2f darkorange ff8c00
darkorchid 9932cc darkred 8b0000 darksalmon e9967a darkseagreen 8fbc8f
darkslateblue 483d8b darkslategray 2f4f4f darkslategrey 2f4f4f darkturquoise 00ced1
darkviolet 9400d3 deeppink ff1493 deepskyblue 00bfff dimgray 696969 dimgrey 696969
dodgerblue 1e90ff firebrick b22222 floralwhite fffaf0 forestgreen 228b22
fuchsia ff00ff gainsboro dcdcdc ghostwhite f8f8ff gold ffd700 goldenrod daa520
gray 808080 green 008000 greenyellow adff2f grey 808080 honeydew f0fff0
hotpink ff69b4 indianred cd5c5c indigo 4b0082 ivory fffff0 khaki f0e68c
lavender e6e6fa lavenderblush fff0f5 lawngreen 7cfc00 lemonchiffon fffacd
lightblue add8e6 lightcoral f08080 lightcyan e0ffff lightgoldenrodyellow fafad2
lightgray d3d3d3 lightgreen 90ee90 lightgrey d3d3d3 lightpink ffb6c1
lightsalmon ffa07a lightseagreen 20b2aa lightskyblue 87cefa lightslategray 778899
lightslategrey 778899 lightsteelblue b0c4de lightyellow ffffe0 lime 00ff00
limegreen 32cd32 linen faf0e6 magenta ff00ff maroon 800000 mediumaquamarine 66cdaa
mediumblue 0000cd mediumorchid ba55d3 mediumpurple 9370db mediumseagreen 3cb371
mediumslateblue 7b68ee mediumspringgreen 00fa9a mediumturquoise 48d1cc
mediumvioletred c71585 midnightblue 191970 mintcream f5fffa mistyrose ffe4e1
moccasin ffe4b5 navajowhite ffdead navy 000080 oldlace fdf5e6 olive 808000
olivedrab 6b8e23 orange ffa500 orangered ff4500 orchid da70d6 palegoldenrod eee8aa
palegreen 98fb98 paleturquoise afeeee palevioletred db7093 papayawhip ffefd5
peachpuff ffdab9 peru cd853f pink ffc0cb plum dda0dd powderblue b0e0e6
purple 800080 rebeccapurple 663399 red ff0000 rosybrown bc8f8f royalblue 4169e1
saddlebrown 8b4513 salmon fa8072 sandybrown f4a460 seagreen 2e8b57 seashell fff5ee
sienna a0522d silver c0c0c0 skyblue 87ceeb slateblue 6a5acd slategray 708090
slategrey 708090 snow fffafa springgreen 00ff7f steelblue 4682b4 tan d2b48c
teal 008080 thistle d8bfd8 tomato ff6347 turquoise 40e0d0 violet ee82ee
wheat f5deb3 white ffffff whitesmoke f5f5f5 yellow ffff00 yellowgreen 9acd32`

var (
	named = sync.OnceValue(func() map[string]rgba {
		f := strings.Fields(cssColors)
		m := make(map[string]rgba, len(f)/2)
		for i := 0; i+1 < len(f); i += 2 {
			c, _, _ := parseHex(f[i+1], 0)
			m[f[i]] = c
		}
		m["transparent"] = rgba{a: 0}
		return m
	})
	namedByHex = sync.OnceValue(func() map[string][]string {
		f := strings.Fields(cssColors)
		m := make(map[string][]string, len(f)/2)
		for i := 0; i+1 < len(f); i += 2 {
			hex := "#" + f[i+1]
			m[hex] = append(m[hex], f[i])
		}
		return m
	})
)

var colorDetectRe = regexp.MustCompile(`^(#([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})|(?i:(rgba?|hsla?)\s*\([^()]*\)))$`)

// Detect recognises #hex and rgb()/hsl() values.
func (Color) Detect(sample string) float64 {
	if colorDetectRe.MatchString(strings.TrimSpace(sample)) {
		return 0.8
	}
	return 0
}
