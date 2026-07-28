// Package imaging 图像探测与缩略图。纯 Go 实现（零 cgo）；
// Phase 3 引 libvips 时以同一 Processor 接口经 build tag 替换。
package imaging

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"

	xdraw "golang.org/x/image/draw"

	_ "image/gif"  // 注册解码器；GIF 经 image.Decode 得首帧
	_ "image/png"
	_ "golang.org/x/image/webp" // 仅解码——纯 Go 无成熟有损 WebP 编码器
)

// ErrUnsupported is returned when image format is not recognized.
var ErrUnsupported = errors.New("imaging: unsupported image format")

// Meta contains image metadata: dimensions, MIME type, and file extension.
type Meta struct {
	Width, Height int
	MIME, Ext     string
}

// Processor detects image metadata and generates JPEG thumbnails.
// Both Probe and Thumbnail consume their input readers (partially or fully);
// callers needing multiple passes must supply independent readers, e.g. buffer once then bytes.NewReader per call.
type Processor interface {
	// Probe reads only the image header bytes and returns metadata.
	// After Probe returns, r is at an indeterminate offset and must not be reused.
	// 仅读取头部字节即返回；调用后 r 处于不确定偏移，不可复用。
	Probe(r io.Reader) (Meta, error)

	// Thumbnail fully consumes r, returning a JPEG-encoded thumbnail.
	// 完整消耗 r。
	Thumbnail(r io.Reader, maxEdge int) ([]byte, error)

	// ThumbExt 本处理器缩略图输出格式的扩展名(纯 Go "jpg"/vips "webp")——
	// 上传落盘键与 /t 双探测按此对齐(D-②)。
	ThumbExt() string
}

type goProcessor struct{}

// NewGo returns a pure-Go image Processor using the standard library.
func NewGo() Processor { return goProcessor{} }

var formatMeta = map[string]struct{ mime, ext string }{
	"jpeg": {"image/jpeg", "jpg"},
	"png":  {"image/png", "png"},
	"gif":  {"image/gif", "gif"},
	"webp": {"image/webp", "webp"},
}

func (goProcessor) ThumbExt() string { return "jpg" }

func (goProcessor) Probe(r io.Reader) (Meta, error) {
	cfg, format, err := image.DecodeConfig(r)
	if err != nil {
		return Meta{}, ErrUnsupported
	}
	fm, ok := formatMeta[format]
	if !ok {
		return Meta{}, ErrUnsupported
	}
	return Meta{Width: cfg.Width, Height: cfg.Height, MIME: fm.mime, Ext: fm.ext}, nil
}

func (goProcessor) Thumbnail(r io.Reader, maxEdge int) ([]byte, error) {
	src, _, err := image.Decode(r)
	if err != nil {
		return nil, ErrUnsupported
	}
	sb := src.Bounds()
	w, h := sb.Dx(), sb.Dy()
	if w > maxEdge || h > maxEdge { // 只缩不放
		if w >= h {
			h = h * maxEdge / w
			w = maxEdge
		} else {
			w = w * maxEdge / h
			h = maxEdge
		}
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
	}
	// 白底画布：平铺 alpha（JPEG 无透明通道）
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, sb, xdraw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 82}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Avatar 中心方裁并缩放到 edge×edge 的 JPEG 头像(白底平铺 alpha)。
// 非图片或单边超过 8192(头像场景防解压炸弹,阈值远低于上传管线)返回 ErrUnsupported。
func Avatar(data []byte, edge int) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width > 8192 || cfg.Height > 8192 {
		return nil, ErrUnsupported
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrUnsupported
	}
	sb := src.Bounds()
	side := sb.Dx()
	if sb.Dy() < side {
		side = sb.Dy()
	}
	x0 := sb.Min.X + (sb.Dx()-side)/2
	y0 := sb.Min.Y + (sb.Dy()-side)/2
	crop := image.Rect(x0, y0, x0+side, y0+side)

	dst := image.NewRGBA(image.Rect(0, 0, edge, edge))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, crop, xdraw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 82}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
