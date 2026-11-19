// Package encoders groups the encoding tools.
package encoders

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Base64 encodes and decodes standard or URL-safe Base64.
type Base64 struct{}

func (Base64) Spec() tool.Spec {
	return tool.Spec{
		ID:          "base64",
		Name:        "Base64",
		Category:    tool.Encoders,
		Description: "Encode and decode standard or URL-safe Base64",
		Keywords:    []string{"b64", "base64url", "encode", "decode"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Input", Multiline: true}},
		Options: []tool.Option{
			{Key: "mode", Label: "Mode", Kind: tool.Select, Choices: []string{"encode", "decode"}, Default: "encode"},
			{Key: "variant", Label: "Alphabet", Kind: tool.Select, Choices: []string{"standard", "url"}, Default: "standard"},
			{Key: "padding", Label: "Padding", Help: "Add '=' when encoding", Kind: tool.Toggle, Default: true},
			{Key: "output", Label: "Output", Help: "When decoding: auto shows hex if the result is binary", Kind: tool.Select, Choices: []string{"auto", "text", "hex"}, Default: "auto"},
		},
		Output: tool.PlainText,
		Live:   true,
	}
}

func (b Base64) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(b.Spec())
	in := req.Inputs["input"]
	urlSafe := opts.String("variant") == "url"

	var out string
	var meta []tool.KV
	switch opts.String("mode") {
	case "decode":
		dec, err := decodeBase64(in, urlSafe)
		if err != nil {
			return tool.Result{}, err
		}
		out, meta = renderDecoded(dec, opts.String("output"))
	default:
		enc := base64.StdEncoding
		if urlSafe {
			enc = base64.URLEncoding
		}
		if !opts.Bool("padding") {
			enc = enc.WithPadding(base64.NoPadding)
		}
		out = enc.EncodeToString([]byte(in))
	}
	return tool.Result{
		Text: out,
		Meta: append([]tool.KV{
			{Key: "Input", Value: fmt.Sprintf("%d B", len(in))},
			{Key: "Output", Value: fmt.Sprintf("%d B", len(out))},
		}, meta...),
	}, nil
}

// renderDecoded shows the bytes as text or, when they are binary (or hex
// was requested), as hex.
func renderDecoded(dec []byte, output string) (string, []tool.KV) {
	binary := len(dec) > 0 && !isText(dec)
	if output == "hex" || (output == "auto" && binary) {
		meta := []tool.KV{{Key: "Format", Value: "hex"}}
		if binary {
			meta = append(meta, tool.KV{Key: "Binary", Value: fmt.Sprintf("%d bytes", len(dec))})
		}
		return hex.EncodeToString(dec), meta
	}
	return string(dec), nil
}

// isText requires valid UTF-8, no NUL and at least 90% printable runes.
func isText(b []byte) bool {
	for _, c := range b {
		if c == 0 {
			return false
		}
	}
	return mostlyPrintable(b)
}

// decodeBase64 ignores spaces and line breaks, accepts input with or without
// padding and returns a *tool.InputError with the original position.
func decodeBase64(in string, urlSafe bool) ([]byte, error) {
	clean := make([]byte, 0, len(in))
	offsets := make([]int, 0, len(in)) // position in clean → position in in
	for i := 0; i < len(in); i++ {
		switch in[i] {
		case ' ', '\t', '\r', '\n':
			continue
		}
		clean = append(clean, in[i])
		offsets = append(offsets, i)
	}
	enc := base64.RawStdEncoding
	if urlSafe {
		enc = base64.RawURLEncoding
	}
	trimmed := strings.TrimRight(string(clean), "=")
	dec, err := enc.DecodeString(trimmed)
	if err == nil {
		return dec, nil
	}
	var corrupt base64.CorruptInputError
	if !errors.As(err, &corrupt) {
		return nil, err
	}
	pos := len(in)
	if int(corrupt) < len(offsets) {
		pos = offsets[corrupt]
	}
	line, col := tool.Position(in, pos)
	return nil, &tool.InputError{Input: "input", Line: line, Col: col, Msg: "invalid Base64 character or wrong length"}
}

// Detect recognises valid Base64 whose decoding is mostly text.
func (Base64) Detect(sample string) float64 {
	s := strings.TrimSpace(sample)
	if len(s) < 8 || len(s)%4 != 0 {
		return 0
	}
	for _, urlSafe := range []bool{false, true} {
		dec, err := decodeBase64(s, urlSafe)
		if err == nil && mostlyPrintable(dec) {
			return 0.6
		}
	}
	return 0
}

func mostlyPrintable(b []byte) bool {
	if len(b) == 0 || !utf8.Valid(b) {
		return false
	}
	total, ok := 0, 0
	for _, r := range string(b) {
		total++
		if unicode.IsPrint(r) || unicode.IsSpace(r) {
			ok++
		}
	}
	return ok*10 >= total*9
}
