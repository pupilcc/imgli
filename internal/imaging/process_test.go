package imaging

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

func encodeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.White)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func solidPNG(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func solidJPEG(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestScale(t *testing.T) {
	in := encodePNG(t, 800, 200)

	out, err := Scale(in, 400)
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if format != "png" {
		t.Errorf("format = %s, want png", format)
	}
	if cfg.Width != 400 || cfg.Height != 100 {
		t.Errorf("size = %d×%d, want 400×100", cfg.Width, cfg.Height)
	}

	same, err := Scale(in, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(same, in) {
		t.Error("maxEdge 未超出时应原样返回输入")
	}

	jpgIn := encodeJPEG(t, 800, 200)
	jpgOut, err := Scale(jpgIn, 400)
	if err != nil {
		t.Fatal(err)
	}
	_, jpgFormat, err := image.DecodeConfig(bytes.NewReader(jpgOut))
	if err != nil {
		t.Fatal(err)
	}
	if jpgFormat != "jpeg" {
		t.Errorf("JPEG 输入应输出 jpeg, got %s", jpgFormat)
	}
}

func TestWatermarkImageCorner(t *testing.T) {
	base := solidJPEG(t, 400, 400, color.White)
	mark := solidPNG(t, 100, 100, color.RGBA{R: 255, A: 255})

	// br + margin 10 + 100×100 mark → 覆盖 [290,390)×[290,390)
	// 采 mark 内部点 (340,340)；JPEG 块边缘采样不稳定，故不用开区间端点 (390,390)
	const brX, brY = 340, 340

	out, err := WatermarkImage(base, mark, "br", 1.0, 10)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := img.At(brX, brY).RGBA()
	if r>>8 <= 150 {
		t.Errorf("(%d,%d) R=%d, want >150", brX, brY, r>>8)
	}
	r10, g10, b10, _ := img.At(10, 10).RGBA()
	if r10>>8 < 240 || g10>>8 < 240 || b10>>8 < 240 {
		t.Errorf("(10,10) 应近白, got rgb=(%d,%d,%d)", r10>>8, g10>>8, b10>>8)
	}
	gFull := g >> 8
	if gFull >= 100 {
		t.Errorf("opacity 1.0 时 (%d,%d) G=%d, want <100", brX, brY, gFull)
	}
	_ = b

	out02, err := WatermarkImage(base, mark, "br", 0.2, 10)
	if err != nil {
		t.Fatal(err)
	}
	img02, err := jpeg.Decode(bytes.NewReader(out02))
	if err != nil {
		t.Fatal(err)
	}
	_, g02, _, _ := img02.At(brX, brY).RGBA()
	if g02>>8 <= 180 {
		t.Errorf("opacity 0.2 时 (%d,%d) G=%d, want >180", brX, brY, g02>>8)
	}

	outTL, err := WatermarkImage(base, mark, "tl", 1.0, 10)
	if err != nil {
		t.Fatal(err)
	}
	imgTL, err := jpeg.Decode(bytes.NewReader(outTL))
	if err != nil {
		t.Fatal(err)
	}
	// tl + margin 10 → 覆盖 [10,110)；采 (60,60) 避开 JPEG 边缘
	rTL, gTL, _, _ := imgTL.At(60, 60).RGBA()
	if rTL>>8 <= 150 || gTL>>8 >= 100 {
		t.Errorf("position tl 时 (60,60) 应偏红, got R=%d G=%d", rTL>>8, gTL>>8)
	}
	rTL10, gTL10, _, _ := imgTL.At(10, 10).RGBA()
	if rTL10>>8 <= 150 {
		t.Errorf("position tl 时 (10,10) 应偏红, got R=%d G=%d", rTL10>>8, gTL10>>8)
	}
}

func TestWatermarkImageOversizeMark(t *testing.T) {
	base := solidJPEG(t, 400, 400, color.White)
	mark := solidPNG(t, 500, 500, color.RGBA{R: 255, A: 255})
	out, err := WatermarkImage(base, mark, "br", 1.0, 0)
	if err != nil {
		t.Fatalf("超大 mark 不应报错: %v", err)
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	// 缩至 90% 后贴 br，右下角应有红色
	r, g, _, _ := img.At(390, 390).RGBA()
	if r>>8 < 100 || g>>8 > 200 {
		t.Errorf("角落应有色, got R=%d G=%d", r>>8, g>>8)
	}
}

func TestWatermarkText(t *testing.T) {
	in := solidPNG(t, 600, 300, color.White)
	out, err := WatermarkText(in, "白栗©2026", "bc", 0.9, 0.08)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(out, in) {
		t.Error("输出应不同于输入")
	}
	img, format, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if format != "png" {
		t.Errorf("format = %s, want png", format)
	}
	b := img.Bounds()
	h := b.Dy()
	topEnd := h / 3
	botStart := h * 2 / 3

	nonWhiteBot := 0
	for y := botStart; y < h; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bb, _ := img.At(x, y).RGBA()
			if r>>8 < 250 || g>>8 < 250 || bb>>8 < 250 {
				nonWhiteBot++
			}
		}
	}
	if nonWhiteBot <= 500 {
		t.Errorf("底部 1/3 非白像素 = %d, want >500", nonWhiteBot)
	}

	for y := b.Min.Y; y < topEnd; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bb, _ := img.At(x, y).RGBA()
			if r>>8 < 250 || g>>8 < 250 || bb>>8 < 250 {
				t.Fatalf("顶部 1/3 应全白, 于 (%d,%d) rgb=(%d,%d,%d)", x, y, r>>8, g>>8, bb>>8)
			}
		}
	}
}

// TestAnchorClampsWithinCanvas 超大 margin 不得把水印推出画布(codex 终审 F2)。
func TestAnchorClampsWithinCanvas(t *testing.T) {
	for _, pos := range []string{"tl", "tc", "tr", "ml", "mc", "mr", "bl", "bc", "br"} {
		x, y := anchor(pos, 100, 100, 40, 30, 1000) // margin 远大于画布
		if x < 0 || y < 0 || x > 100-40 || y > 100-30 {
			t.Errorf("pos=%s 越界: x=%d y=%d(应 0..60 / 0..70)", pos, x, y)
		}
	}
	// 水印大于画布:钳到 0
	if x, y := anchor("br", 50, 50, 80, 80, 4); x != 0 || y != 0 {
		t.Errorf("mark 大于画布应钳到 (0,0),得 (%d,%d)", x, y)
	}
}

// TestWatermarkTextExtremeAspectBounded 极端长宽比+大 size_ratio 不得触发巨额分配,
// 应正常返回且尺寸不变(codex 终审 F1:中间层被钳到底图边界内)。
func TestWatermarkTextExtremeAspectBounded(t *testing.T) {
	// 2000×16 白底:长边 2000,size_ratio 0.2 → 原始字号 400,中间层若按文本全尺寸
	// 分配将远超底图;修复后字号收缩+层钳制,分配 ≤ 2000×16。
	src := image.NewRGBA(image.Rect(0, 0, 2000, 16))
	for i := range src.Pix {
		src.Pix[i] = 255
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	out, err := WatermarkText(buf.Bytes(), "白栗测试水印一段较长的文字内容", "br", 0.6, 0.2)
	if err != nil {
		t.Fatalf("极端长宽比应正常返回: %v", err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil || format != "png" || cfg.Width != 2000 || cfg.Height != 16 {
		t.Errorf("输出应为 2000×16 png, got %dx%d %s %v", cfg.Width, cfg.Height, format, err)
	}
}

func TestProcessRejects(t *testing.T) {
	// 最小手造 GIF
	gifImg := image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.White})
	var gifBuf bytes.Buffer
	if err := gif.Encode(&gifBuf, gifImg, nil); err != nil {
		t.Fatal(err)
	}
	gifBytes := gifBuf.Bytes()

	if _, err := Scale(gifBytes, 100); err != ErrUnsupported {
		t.Errorf("Scale GIF: got %v, want ErrUnsupported", err)
	}
	if _, err := WatermarkText(gifBytes, "x", "br", 1.0, 0.1); err != ErrUnsupported {
		t.Errorf("WatermarkText GIF: got %v, want ErrUnsupported", err)
	}

	base := solidJPEG(t, 100, 100, color.White)
	jpegMark := solidJPEG(t, 20, 20, color.RGBA{R: 255, A: 255})
	if _, err := WatermarkImage(base, jpegMark, "br", 1.0, 0); err != ErrUnsupported {
		t.Errorf("非 PNG mark: got %v, want ErrUnsupported", err)
	}
}
