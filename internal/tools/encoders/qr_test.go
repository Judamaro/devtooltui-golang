package encoders_test

import (
	"bytes"
	"context"
	"image/png"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/encoders"
)

func qrReq(in string, opts tool.Values) tool.Request {
	return tool.Request{Inputs: map[string]string{"input": in}, Opts: opts}
}

func TestQRGolden(t *testing.T) {
	in := testutil.ReadInput(t, "qr", "url")
	for _, tt := range []struct {
		name string
		opts tool.Values
	}{
		{"url-m", nil},
		{"url-h-invert", tool.Values{"level": "H", "invert": true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := encoders.QR{}.Run(context.Background(), qrReq(in, tt.opts))
			if err != nil {
				t.Fatal(err)
			}
			testutil.Golden(t, "qr", tt.name, res.Text)
		})
	}
}

func TestQR(t *testing.T) {
	res, err := encoders.QR{}.Run(context.Background(), qrReq("hello", nil))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(res.Text, "\n")
	// Version 1 is 21 modules plus 4 of quiet zone per side: 29 columns and
	// 15 rows of half blocks.
	if len(lines) != 15 {
		t.Errorf("%d rows, want 15", len(lines))
	}
	for i, l := range lines {
		if n := len([]rune(l)); n != 29 {
			t.Errorf("row %d has %d columns, want 29", i, n)
		}
		if strings.Trim(l, " ▀▄█") != "" {
			t.Errorf("row %d has other characters: %q", i, l)
		}
	}
	meta := map[string]string{}
	for _, kv := range res.Meta {
		meta[kv.Key] = kv.Value
	}
	if meta["Version"] != "1" || meta["Modules"] != "21×21" || meta["Recovery"] != "M" {
		t.Errorf("Meta = %v", res.Meta)
	}
	// The quiet zone is light: blocks when not inverted, spaces when inverted.
	if !strings.HasPrefix(lines[0], "████") {
		t.Errorf("first row %q does not start with the light quiet zone", lines[0])
	}
	inv, _ := encoders.QR{}.Run(context.Background(), qrReq("hello", tool.Values{"invert": true}))
	if !strings.HasPrefix(inv.Text, "    ") {
		t.Errorf("inverted first row %q does not start with spaces", strings.SplitN(inv.Text, "\n", 2)[0])
	}
}

func TestQREmpty(t *testing.T) {
	res, err := encoders.QR{}.Run(context.Background(), qrReq("", nil))
	if err != nil || res.Text != "" {
		t.Errorf("Run(empty) = %q, %v", res.Text, err)
	}
	if _, _, err := (encoders.QR{}).Export(context.Background(), qrReq("", nil)); err == nil {
		t.Error("Export(empty) did not fail")
	} else if _, ok := tool.AsInputError(err); !ok {
		t.Errorf("Export(empty) error = %v, want *InputError", err)
	}
}

func TestQRTooLong(t *testing.T) {
	_, err := encoders.QR{}.Run(context.Background(), qrReq(strings.Repeat("x", 3000), tool.Values{"level": "L"}))
	ie, ok := tool.AsInputError(err)
	if !ok || ie.Line != 1 || ie.Col != 1 || !strings.Contains(ie.Msg, "too long") {
		t.Errorf("error = %v, want a too long InputError at 1:1", err)
	}
}

func TestQRExport(t *testing.T) {
	data, ext, err := encoders.QR{}.Export(context.Background(), qrReq("hello", tool.Values{"size": 256}))
	if err != nil {
		t.Fatal(err)
	}
	if ext != ".png" {
		t.Errorf("ext = %q, want .png", ext)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 256 || b.Dy() != 256 {
		t.Errorf("PNG is %dx%d, want 256x256", b.Dx(), b.Dy())
	}
	again, _, _ := encoders.QR{}.Export(context.Background(), qrReq("hello", tool.Values{"size": 256}))
	if !bytes.Equal(data, again) {
		t.Error("Export is not deterministic")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := (encoders.QR{}).Export(ctx, qrReq("hello", nil)); err != context.Canceled {
		t.Errorf("Export with cancelled ctx = %v", err)
	}
}
