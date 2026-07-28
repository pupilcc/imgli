package imaging

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func encodePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, 0, color.RGBA{R: uint8(x % 256), A: 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestAvatarSquare(t *testing.T) {
	for _, dims := range [][2]int{{400, 200}, {100, 300}, {64, 64}} {
		out, err := Avatar(encodePNG(t, dims[0], dims[1]), 256)
		if err != nil {
			t.Fatalf("%v: %v", dims, err)
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
		if err != nil || format != "jpeg" {
			t.Fatalf("%v: 输出应为 jpeg, got %s %v", dims, format, err)
		}
		if cfg.Width != 256 || cfg.Height != 256 {
			t.Errorf("%v: 应 256×256, got %d×%d", dims, cfg.Width, cfg.Height)
		}
	}
}

func TestAvatarRejects(t *testing.T) {
	if _, err := Avatar([]byte("not an image"), 256); err != ErrUnsupported {
		t.Errorf("非图应 ErrUnsupported, got %v", err)
	}
}
