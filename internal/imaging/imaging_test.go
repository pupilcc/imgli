package imaging

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// 生成 300x200 带透明像素的 PNG
func alphaPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 300, 200))
	for x := 0; x < 300; x++ {
		for y := 0; y < 200; y++ {
			img.Set(x, y, color.NRGBA{R: 200, A: 0}) // 全透明红
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func TestProbePNG(t *testing.T) {
	m, err := NewGo().Probe(bytes.NewReader(alphaPNG(t)))
	if err != nil {
		t.Fatal(err)
	}
	if m.Width != 300 || m.Height != 200 || m.MIME != "image/png" || m.Ext != "png" {
		t.Errorf("Meta = %+v", m)
	}
}

func TestProbeRejectsNonImage(t *testing.T) {
	_, err := NewGo().Probe(bytes.NewReader([]byte("MZ not an image at all........")))
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("err = %v, want ErrUnsupported", err)
	}
}

func TestThumbnailIsJPEGResizedFlattened(t *testing.T) {
	out, err := NewGo().Thumbnail(bytes.NewReader(alphaPNG(t)), 150)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(out)) // 必须能按 JPEG 解出
	if err != nil {
		t.Fatal("缩略图不是 JPEG:", err)
	}
	b := img.Bounds()
	if b.Dx() != 150 || b.Dy() != 100 { // 300x200 按长边 150 等比
		t.Errorf("thumb = %dx%d, want 150x100", b.Dx(), b.Dy())
	}
	r, g, bb, _ := img.At(75, 50).RGBA()
	if r>>8 < 240 || g>>8 < 240 || bb>>8 < 240 { // 全透明像素应平铺为白
		t.Errorf("透明区未平铺白底: rgb=(%d,%d,%d)", r>>8, g>>8, bb>>8)
	}
}

func TestThumbnailNoUpscale(t *testing.T) {
	out, err := NewGo().Thumbnail(bytes.NewReader(alphaPNG(t)), 800)
	if err != nil {
		t.Fatal(err)
	}
	img, _ := jpeg.Decode(bytes.NewReader(out))
	if img.Bounds().Dx() != 300 { // 小于 maxEdge 不放大
		t.Errorf("被放大了: %v", img.Bounds())
	}
}

func TestThumbnailRejectsTooManyPixels(t *testing.T) {
	// 用小图 + 压低像素预算，避免测试里分配巨型位图。
	old := MaxDecodePixels
	MaxDecodePixels = 100 // 300×200 alphaPNG = 60k 像素 >> 100
	t.Cleanup(func() { MaxDecodePixels = old })
	_, err := NewGo().Thumbnail(bytes.NewReader(alphaPNG(t)), 400)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err=%v, want ErrTooLarge", err)
	}
}

func TestThumbnailRejectsOversizedBytes(t *testing.T) {
	// 伪造超大输入：LimitReader 在读满 MaxThumbSourceBytes+1 后判 TooLarge
	old := MaxThumbSourceBytes
	MaxThumbSourceBytes = 1024
	t.Cleanup(func() { MaxThumbSourceBytes = old })
	blob := bytes.Repeat([]byte{0x00}, 2048)
	_, err := NewGo().Thumbnail(bytes.NewReader(blob), 400)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err=%v, want ErrTooLarge", err)
	}
}
