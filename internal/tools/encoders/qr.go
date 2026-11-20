package encoders

import (
	"context"
	"fmt"
	"strings"

	"github.com/skip2/go-qrcode"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
)

// QR draws a QR code with Unicode half blocks and exports it as PNG.
type QR struct{}

var qrLevels = map[string]qrcode.RecoveryLevel{
	"L": qrcode.Low, "M": qrcode.Medium, "Q": qrcode.High, "H": qrcode.Highest,
}

func (QR) Spec() tool.Spec {
	return tool.Spec{
		ID:          "qr",
		Name:        "QR code",
		Category:    tool.Encoders,
		Description: "Draw a QR code in the terminal; save it as PNG (Ctrl+S, --out)",
		Keywords:    []string{"qrcode", "barcode", "png", "image"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "Text or URL", Multiline: true}},
		Options: []tool.Option{
			{Key: "level", Label: "Recovery", Help: "Error correction: L 7 %, M 15 %, Q 25 %, H 30 %", Kind: tool.Select, Choices: []string{"L", "M", "Q", "H"}, Default: "M"},
			{Key: "invert", Label: "Light terminal", Help: "Draw dark modules as blocks, for terminals with a light background", Kind: tool.Toggle, Default: false},
			{Key: "size", Label: "PNG size", Help: "Width and height of the PNG in pixels", Kind: tool.Number, Min: 64, Max: 4096, Default: 512},
		},
		Output: tool.Image,
		Live:   true,
	}
}

func (q QR) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	code, opts, err := q.encode(ctx, req)
	if err != nil || code == nil {
		return tool.Result{}, err
	}
	// The terminal shows light text on dark: blocks are the light modules,
	// so the code reads dark on light. "invert" swaps that.
	art := strings.TrimSuffix(code.ToSmallString(opts.Bool("invert")), "\n")
	n := len(code.Bitmap()) - 8 // without the 4-module quiet zone
	return tool.Result{
		Text: art,
		Meta: []tool.KV{
			{Key: "Version", Value: fmt.Sprint(code.VersionNumber)},
			{Key: "Modules", Value: fmt.Sprintf("%d×%d", n, n)},
			{Key: "Recovery", Value: opts.String("level")},
		},
	}, nil
}

// Export returns the code as a PNG (always dark on light).
func (q QR) Export(ctx context.Context, req tool.Request) ([]byte, string, error) {
	code, opts, err := q.encode(ctx, req)
	if err != nil {
		return nil, "", err
	}
	if code == nil {
		return nil, "", &tool.InputError{Input: "input", Line: 1, Col: 1, Msg: "nothing to encode"}
	}
	png, err := code.PNG(min(max(opts.Int("size"), 64), 4096))
	return png, ".png", err
}

// encode returns nil, without error, for an empty input.
func (q QR) encode(ctx context.Context, req tool.Request) (*qrcode.QRCode, tool.Values, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	opts := req.Opts.WithDefaults(q.Spec())
	in := req.Inputs["input"]
	if in == "" {
		return nil, opts, nil
	}
	level, ok := qrLevels[opts.String("level")]
	if !ok {
		level = qrcode.Medium
	}
	code, err := qrcode.New(in, level)
	if err != nil {
		return nil, nil, &tool.InputError{Input: "input", Line: 1, Col: 1,
			Msg: fmt.Sprintf("too long for a QR code (%d bytes; at most 2953 with recovery L)", len(in))}
	}
	return code, opts, nil
}
