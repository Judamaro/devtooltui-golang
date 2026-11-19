// Package generators agrupa las herramientas que generan valores.
package generators

import (
	"context"
	"crypto/hmac"
	"crypto/md5"  //nolint:gosec // checksum a pedido del usuario, no seguridad
	"crypto/sha1" //nolint:gosec // checksum a pedido del usuario, no seguridad
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Hash calcula hashes y HMAC de la entrada.
type Hash struct{}

var hashes = map[string]func() hash.Hash{
	"md5":    md5.New,  //nolint:gosec // ver import
	"sha1":   sha1.New, //nolint:gosec // ver import
	"sha256": sha256.New,
	"sha512": sha512.New,
}

func (Hash) Spec() tool.Spec {
	return tool.Spec{
		ID:          "hash",
		Name:        "Hash",
		Category:    tool.Generators,
		Description: "MD5, SHA-1, SHA-256, SHA-512 y HMAC",
		Keywords:    []string{"md5", "sha", "sha256", "checksum", "hmac", "digest"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Entrada", Multiline: true}},
		Options: []tool.Option{
			{Key: "algo", Label: "Algoritmo", Kind: tool.Select, Choices: []string{"md5", "sha1", "sha256", "sha512"}, Default: "sha256"},
			{Key: "key", Label: "Clave HMAC", Help: "Si no está vacía se calcula un HMAC", Kind: tool.String, Default: ""},
			{Key: "encoding", Label: "Formato", Kind: tool.Select, Choices: []string{"hex", "base64"}, Default: "hex"},
			{Key: "uppercase", Label: "Mayúsculas", Help: "Solo para hex", Kind: tool.Toggle, Default: false},
		},
		Output: tool.PlainText,
		Live:   true,
	}
}

func (h Hash) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(h.Spec())
	algo := opts.String("algo")
	newHash, ok := hashes[algo]
	if !ok {
		return tool.Result{}, fmt.Errorf("algoritmo desconocido %q", algo)
	}

	kind := strings.ToUpper(algo)
	var hh hash.Hash
	if key := opts.String("key"); key != "" {
		hh = hmac.New(newHash, []byte(key))
		kind = "HMAC-" + kind
	} else {
		hh = newHash()
	}
	hh.Write([]byte(req.Inputs["input"])) // hash.Hash.Write nunca falla
	sum := hh.Sum(nil)

	var out string
	if opts.String("encoding") == "base64" {
		out = base64.StdEncoding.EncodeToString(sum)
	} else {
		out = hex.EncodeToString(sum)
		if opts.Bool("uppercase") {
			out = strings.ToUpper(out)
		}
	}
	return tool.Result{
		Text: out,
		Meta: []tool.KV{
			{Key: "Algoritmo", Value: kind},
			{Key: "Bits", Value: fmt.Sprint(len(sum) * 8)},
			{Key: "Entrada", Value: fmt.Sprintf("%d B", len(req.Inputs["input"]))},
		},
	}, nil
}
