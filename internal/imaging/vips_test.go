//go:build vips

package imaging

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"testing"

	_ "golang.org/x/image/webp"
)

// grayAlphaPNG 手写一张 8-bit 灰度+alpha(PNG color type 4,2 通道)图。
// Go 标准库无法编码该类型,而正是它触发 vips_flatten 通道数不匹配(codex 终审 F3)。
func grayAlphaPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	chunk := func(typ string, data []byte) []byte {
		var b bytes.Buffer
		binary.Write(&b, binary.BigEndian, uint32(len(data)))
		b.WriteString(typ)
		b.Write(data)
		crc := crc32.NewIEEE()
		crc.Write([]byte(typ))
		crc.Write(data)
		binary.Write(&b, binary.BigEndian, crc.Sum32())
		return b.Bytes()
	}
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], uint32(w))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(h))
	ihdr[8] = 8 // bit depth
	ihdr[9] = 4 // color type 4 = grayscale + alpha
	// 10,11,12 = compression/filter/interlace = 0
	var raw bytes.Buffer
	for y := 0; y < h; y++ {
		raw.WriteByte(0) // filter: none
		for x := 0; x < w; x++ {
			raw.WriteByte(byte(100 + x))          // gray
			raw.WriteByte(byte((x * 255) / w))    // alpha 渐变(含半透明)
		}
	}
	var zbuf bytes.Buffer
	zw := zlib.NewWriter(&zbuf)
	zw.Write(raw.Bytes())
	zw.Close()

	var out bytes.Buffer
	out.Write([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})
	out.Write(chunk("IHDR", ihdr))
	out.Write(chunk("IDAT", zbuf.Bytes()))
	out.Write(chunk("IEND", nil))
	return out.Bytes()
}

// TestVipsThumbnailGrayAlpha 灰度+alpha PNG 经 vips flatten(白底)+webpsave 不应失败
// (单值背景广播到去 alpha 后的 1 通道;codex 终审 F3)。
func TestVipsThumbnailGrayAlpha(t *testing.T) {
	ga := grayAlphaPNG(t, 80, 40)
	// 先确认这确实是 2 通道 GA(标准库能解码 color type 4)
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(ga)); err != nil || cfg.Width != 80 {
		t.Fatalf("构造的 GA PNG 无法解码: %v", err)
	}
	out, err := NewVips().Thumbnail(bytes.NewReader(ga), 40)
	if err != nil {
		t.Fatalf("灰度+alpha flatten 应成功: %v", err)
	}
	if len(out) < 12 || string(out[0:4]) != "RIFF" || string(out[8:12]) != "WEBP" {
		t.Fatalf("非 WebP 输出")
	}
}

// 生成 800×200 不透明 PNG(等比缩 maxEdge 400 → 400×100)。
func widePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 800, 200))
	for x := 0; x < 800; x++ {
		for y := 0; y < 200; y++ {
			img.Set(x, y, color.RGBA{R: 40, G: 120, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestVipsThumbnailWebP(t *testing.T) {
	out, err := NewVips().Thumbnail(bytes.NewReader(widePNG(t)), 400)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 12 {
		t.Fatalf("输出过短: %d", len(out))
	}
	// RIFF....WEBP
	if string(out[0:4]) != "RIFF" || string(out[8:12]) != "WEBP" {
		t.Fatalf("非 WebP 魔数: %q %q", out[0:4], out[8:12])
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatal("webp 解码失败:", err)
	}
	if format != "webp" {
		t.Errorf("format=%q want webp", format)
	}
	if cfg.Width != 400 || cfg.Height != 100 {
		t.Errorf("尺寸=%dx%d want 400x100", cfg.Width, cfg.Height)
	}
}

func TestVipsThumbnailRejectsNonImage(t *testing.T) {
	_, err := NewVips().Thumbnail(bytes.NewReader([]byte("MZ not an image at all........")), 400)
	if err == nil {
		t.Fatal("非图应返回错误")
	}
}
