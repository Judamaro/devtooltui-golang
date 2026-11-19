// Package generators groups the tools that generate values.
package generators

import (
	"context"
	"crypto/hmac"
	"crypto/md5"  //nolint:gosec // checksum on the user's request, not security
	"crypto/sha1" //nolint:gosec // checksum on the user's request, not security
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// Hash computes hashes and HMACs of the input.
type Hash struct{}

var hashes = map[string]func() hash.Hash{
	"md5":    md5.New,  //nolint:gosec // see import
	"sha1":   sha1.New, //nolint:gosec // see import
	"sha256": sha256.New,
	"sha512": sha512.New,
}

func (Hash) Spec() tool.Spec {
	return tool.Spec{
		ID:          "hash",
		Name:        "Hash",
		Category:    tool.Generators,
		Description: "MD5, SHA-1, SHA-256, SHA-512 and HMAC",
		Keywords:    []string{"md5", "sha", "sha256", "checksum", "hmac", "digest"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Input", Multiline: true}},
		Options: []tool.Option{
			{Key: "algo", Label: "Algorithm", Kind: tool.Select, Choices: []string{"md5", "sha1", "sha256", "sha512"}, Default: "sha256"},
			{Key: "key", Label: "HMAC key", Help: "When not empty, an HMAC is computed", Kind: tool.String, Default: ""},
			{Key: "encoding", Label: "Format", Kind: tool.Select, Choices: []string{"hex", "base64"}, Default: "hex"},
			{Key: "uppercase", Label: "Uppercase", Help: "Hex only", Kind: tool.Toggle, Default: false},
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
		return tool.Result{}, fmt.Errorf("unknown algorithm %q", algo)
	}

	kind := strings.ToUpper(algo)
	var hh hash.Hash
	if key := opts.String("key"); key != "" {
		hh = hmac.New(newHash, []byte(key))
		kind = "HMAC-" + kind
	} else {
		hh = newHash()
	}
	hh.Write([]byte(req.Inputs["input"])) // hash.Hash.Write never fails
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
			{Key: "Algorithm", Value: kind},
			{Key: "Bits", Value: fmt.Sprint(len(sum) * 8)},
			{Key: "Input", Value: fmt.Sprintf("%d B", len(req.Inputs["input"]))},
		},
	}, nil
}
