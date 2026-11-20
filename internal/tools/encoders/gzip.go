package encoders

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// GzipMaxOutput caps decompression, so a small bomb cannot use up memory.
const GzipMaxOutput = 64 << 20

// Gzip compresses text into Base64 and decompresses it back.
type Gzip struct{}

func (Gzip) Spec() tool.Spec {
	return tool.Spec{
		ID:          "gzip",
		Name:        "GZip",
		Category:    tool.Encoders,
		Description: "Compress text to Base64 GZip and decompress it back",
		Keywords:    []string{"gz", "compress", "decompress", "deflate", "zip"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Input", Multiline: true}},
		Options: []tool.Option{
			{Key: "mode", Label: "Mode", Kind: tool.Select, Choices: []string{"encode", "decode"}, Default: "encode"},
			{Key: "level", Label: "Level", Help: "Compression level: 1 fastest, 9 smallest", Kind: tool.Number, Min: 1, Max: 9, Default: 6},
			{Key: "output", Label: "Output", Help: "When decoding: auto shows hex if the result is binary", Kind: tool.Select, Choices: []string{"auto", "text", "hex"}, Default: "auto"},
		},
		Output: tool.PlainText,
		Live:   true,
	}
}

func (g Gzip) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(g.Spec())
	in := req.Inputs["input"]

	if opts.String("mode") == "decode" {
		if strings.TrimSpace(in) == "" {
			return tool.Result{}, nil
		}
		raw, err := decodeBase64(in, false)
		if err != nil {
			if urlRaw, uerr := decodeBase64(in, true); uerr == nil {
				raw, err = urlRaw, nil
			}
		}
		if err != nil {
			return tool.Result{}, err
		}
		plain, err := gunzip(ctx, raw)
		if err != nil {
			return tool.Result{}, err
		}
		out, meta := renderDecoded(plain, opts.String("output"))
		return tool.Result{Text: out, Meta: append(ratioMeta(len(plain), len(raw)), meta...)}, nil
	}

	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, min(max(opts.Int("level"), gzip.BestSpeed), gzip.BestCompression))
	if err != nil {
		return tool.Result{}, err
	}
	// The header has no name or time, so the same input gives the same output.
	if _, err := zw.Write([]byte(in)); err != nil {
		return tool.Result{}, err
	}
	if err := zw.Close(); err != nil {
		return tool.Result{}, err
	}
	return tool.Result{
		Text: base64.StdEncoding.EncodeToString(buf.Bytes()),
		Meta: ratioMeta(len(in), buf.Len()),
	}, nil
}

// gunzip decompresses, honouring ctx and GzipMaxOutput. Data errors are
// InputErrors at 1:1: the position inside the Base64 text means nothing.
func gunzip(ctx context.Context, raw []byte) ([]byte, error) {
	bad := func(msg string) error { return &tool.InputError{Input: "input", Line: 1, Col: 1, Msg: msg} }
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, bad("not GZip data: " + err.Error())
	}
	var out bytes.Buffer
	chunk := make([]byte, 64<<10)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, err := zr.Read(chunk)
		out.Write(chunk[:n])
		if out.Len() > GzipMaxOutput {
			return nil, bad(fmt.Sprintf("the decompressed data exceeds %d MB", GzipMaxOutput>>20))
		}
		if errors.Is(err, io.EOF) {
			return out.Bytes(), nil
		}
		if err != nil {
			return nil, bad("corrupt GZip data: " + err.Error())
		}
	}
}

func ratioMeta(plain, compressed int) []tool.KV {
	ratio := "–"
	if plain > 0 {
		ratio = fmt.Sprintf("%.1f %%", float64(compressed)*100/float64(plain))
	}
	return []tool.KV{
		{Key: "Original", Value: fmt.Sprintf("%d B", plain)},
		{Key: "Compressed", Value: fmt.Sprintf("%d B", compressed)},
		{Key: "Ratio", Value: ratio},
	}
}

// Detect recognises Base64 that starts with the GZip magic (1f 8b 08 →
// "H4sI") and a whole header. Only the start is decoded: the sample may be
// cut.
func (Gzip) Detect(sample string) float64 {
	s := strings.TrimSpace(sample)
	if !strings.HasPrefix(s, "H4sI") || len(s) < 24 {
		return 0
	}
	if _, err := decodeBase64(s[:24], false); err != nil {
		return 0
	}
	return 0.85
}
