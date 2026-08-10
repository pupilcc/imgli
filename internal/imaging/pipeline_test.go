package imaging

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func solidRGBABytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 20, G: 40, B: 60, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestProcessPipelineNoop(t *testing.T) {
	in := solidRGBABytes(t, 40, 30)
	res, err := ProcessPipeline(in, PipelineOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Error("noop should not change")
	}
	if !bytes.Equal(res.Data, in) {
		t.Error("bytes changed")
	}
}

func TestProcessPipelineMaxEdge(t *testing.T) {
	in := solidRGBABytes(t, 800, 600)
	res, err := ProcessPipeline(in, PipelineOpts{MaxEdge: 200, JpegQuality: 90})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Fatal("expected change")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatal(err)
	}
	if format != "png" {
		t.Errorf("format=%s", format)
	}
	if cfg.Width != 200 && cfg.Height != 200 {
		// 800x600 → long=800 → 200x150
		if cfg.Width != 200 || cfg.Height != 150 {
			t.Errorf("size %dx%d", cfg.Width, cfg.Height)
		}
	}
}

func TestProcessPipelineWebPWithoutEncoderFallsBack(t *testing.T) {
	if WebPEncodeAvailable() {
		t.Skip("this build encodes webp")
	}
	in := solidRGBABytes(t, 40, 30)
	res, err := ProcessPipeline(in, PipelineOpts{
		OutputWebP:   true,
		WebPQuality:  80,
		SkipIfLarger: true,
		// force reencode so pipeline runs
		ForceReencode: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	// no encoder → keep path after reencode (png)
	if res.Format != "png" {
		t.Errorf("format=%s want png fallback", res.Format)
	}
}

func TestWebPEncodeUnavailableOnPureGo(t *testing.T) {
	if WebPEncodeAvailable() {
		t.Skip("vips build")
	}
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	if _, err := EncodeWebP(img, 80); err != ErrUnsupported {
		t.Errorf("err=%v want ErrUnsupported", err)
	}
}

