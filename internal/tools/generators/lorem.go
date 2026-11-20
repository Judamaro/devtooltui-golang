package generators

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	mrand "math/rand/v2"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Lorem generates Lorem Ipsum placeholder text. Rand pins the randomness in
// tests; when nil, crypto/rand seeds the generator.
type Lorem struct {
	Rand io.Reader
}

const loremStart = "Lorem ipsum dolor sit amet, consectetur adipiscing elit"

var loremWords = strings.Fields(`a ac accumsan ad adipiscing aenean aliquam aliquet amet ante aptent arcu
at auctor augue bibendum blandit class commodo condimentum congue consectetur
consequat conubia convallis cras cubilia curabitur curae cursus dapibus diam
dictum dictumst dignissim dis dolor donec dui duis efficitur egestas eget
eleifend elementum elit enim erat eros est et etiam eu euismod ex facilisi
facilisis fames faucibus felis fermentum feugiat finibus fringilla fusce
gravida habitant habitasse hac hendrerit himenaeos iaculis id imperdiet in
inceptos integer interdum ipsum justo lacinia lacus laoreet lectus leo libero
ligula litora lobortis lorem luctus maecenas magna magnis malesuada massa
mattis mauris maximus metus mi molestie mollis montes morbi mus nam nascetur
natoque nec neque netus nibh nisi nisl non nostra nulla nullam nunc odio orci
ornare parturient pellentesque penatibus per pharetra phasellus placerat
platea porta porttitor posuere potenti praesent pretium primis proin pulvinar
purus quam quis quisque rhoncus ridiculus risus rutrum sagittis sapien
scelerisque sed sem semper senectus sit sociosqu sodales sollicitudin
suscipit suspendisse taciti tellus tempor tempus tincidunt torquent tortor
tristique turpis ullamcorper ultrices ultricies urna ut varius vehicula vel
velit venenatis vestibulum vitae vivamus viverra volutpat vulputate`)

func (Lorem) Spec() tool.Spec {
	return tool.Spec{
		ID:          "lorem",
		Name:        "Lorem Ipsum",
		Category:    tool.Generators,
		Description: "Generate Lorem Ipsum words, sentences or paragraphs",
		Keywords:    []string{"placeholder", "dummy", "filler", "text"},
		Options: []tool.Option{
			{Key: "unit", Label: "Unit", Kind: tool.Select, Choices: []string{"paragraphs", "sentences", "words"}, Default: "paragraphs"},
			{Key: "count", Label: "Count", Kind: tool.Number, Min: 1, Max: 100, Default: 3},
			{Key: "classic", Label: "Start with “Lorem ipsum”", Kind: tool.Toggle, Default: true},
		},
		Output: tool.PlainText,
		Live:   true,
	}
}

func (l Lorem) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(l.Spec())
	src := l.Rand
	if src == nil {
		src = rand.Reader
	}
	var seed [32]byte
	if _, err := io.ReadFull(src, seed[:]); err != nil {
		return tool.Result{}, err
	}
	// Placeholder text needs no secrecy; ChaCha8 keeps the output reproducible
	// from one seed.
	g := &loremGen{rng: mrand.New(mrand.NewChaCha8(seed)), classic: opts.Bool("classic")} //nolint:gosec // not security-sensitive
	count := min(max(opts.Int("count"), 1), 100)

	var out string
	switch opts.String("unit") {
	case "words":
		out = strings.Join(g.words(count), " ")
	case "sentences":
		s := make([]string, count)
		for i := range s {
			s[i] = g.sentence()
		}
		out = strings.Join(s, " ")
	default:
		p := make([]string, count)
		for i := range p {
			if err := ctx.Err(); err != nil {
				return tool.Result{}, err
			}
			p[i] = g.paragraph()
		}
		out = strings.Join(p, "\n\n")
	}
	return tool.Result{
		Text: out,
		Meta: []tool.KV{
			{Key: "Words", Value: fmt.Sprint(len(strings.Fields(out)))},
			{Key: "Characters", Value: fmt.Sprint(utf8.RuneCountInString(out))},
		},
	}, nil
}

type loremGen struct {
	rng     *mrand.Rand
	classic bool // the next words start with loremStart
}

// words returns n words; the first ones are the classic opening, "Lorem"
// capitalised, when it has not been used yet.
func (g *loremGen) words(n int) []string {
	out := make([]string, 0, n)
	if g.classic {
		g.classic = false
		start := strings.Fields(strings.ToLower(strings.ReplaceAll(loremStart, ",", "")))
		out = append(out, start[:min(n, len(start))]...)
		out[0] = capitalize(out[0])
	}
	for len(out) < n {
		out = append(out, loremWords[g.rng.IntN(len(loremWords))])
	}
	return out
}

// sentence has 6–14 words, maybe a comma, a capital and a full stop.
func (g *loremGen) sentence() string {
	if g.classic {
		g.classic = false
		return loremStart + "."
	}
	w := g.words(6 + g.rng.IntN(9))
	if len(w) > 7 && g.rng.IntN(2) == 0 {
		i := 2 + g.rng.IntN(len(w)-4)
		w[i] += ","
	}
	w[0] = capitalize(w[0])
	return strings.Join(w, " ") + "."
}

// paragraph has 3–6 sentences.
func (g *loremGen) paragraph() string {
	s := make([]string, 3+g.rng.IntN(4))
	for i := range s {
		s[i] = g.sentence()
	}
	return strings.Join(s, " ")
}

func capitalize(w string) string {
	r, size := utf8.DecodeRuneInString(w)
	return string(unicode.ToUpper(r)) + w[size:]
}
