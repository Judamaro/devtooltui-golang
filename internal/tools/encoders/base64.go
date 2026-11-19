// Package encoders agrupa las herramientas de codificación.
package encoders

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Base64 codifica y decodifica Base64 estándar o URL-safe.
type Base64 struct{}

func (Base64) Spec() tool.Spec {
	return tool.Spec{
		ID:          "base64",
		Name:        "Base64",
		Category:    tool.Encoders,
		Description: "Codifica y decodifica Base64 estándar o URL-safe",
		Keywords:    []string{"b64", "base64url", "encode", "decode"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Entrada", Multiline: true}},
		Options: []tool.Option{
			{Key: "mode", Label: "Modo", Kind: tool.Select, Choices: []string{"encode", "decode"}, Default: "encode"},
			{Key: "variant", Label: "Alfabeto", Kind: tool.Select, Choices: []string{"standard", "url"}, Default: "standard"},
			{Key: "padding", Label: "Padding", Help: "Agregar '=' al codificar", Kind: tool.Toggle, Default: true},
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
	switch opts.String("mode") {
	case "decode":
		dec, err := decodeBase64(in, urlSafe)
		if err != nil {
			return tool.Result{}, err
		}
		out = string(dec)
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
		Meta: []tool.KV{
			{Key: "Entrada", Value: fmt.Sprintf("%d B", len(in))},
			{Key: "Salida", Value: fmt.Sprintf("%d B", len(out))},
		},
	}, nil
}

// decodeBase64 ignora los espacios y saltos de línea, acepta la entrada con
// o sin padding y devuelve un *tool.InputError con la posición original.
func decodeBase64(in string, urlSafe bool) ([]byte, error) {
	clean := make([]byte, 0, len(in))
	offsets := make([]int, 0, len(in)) // posición en clean → posición en in
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
	return nil, &tool.InputError{Input: "input", Line: line, Col: col, Msg: "carácter Base64 inválido o longitud incorrecta"}
}

// Detect reconoce Base64 válido cuya decodificación es mayormente texto.
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
