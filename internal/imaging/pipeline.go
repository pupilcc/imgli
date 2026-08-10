package imaging

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// clampWebPQuality 0/非法 → 80。
func clampWebPQuality(q int) int {
	if q < 1 || q > 100 {
		return 80
	}
	return q
}

// PipelineOpts 单次解码→变换→末次编码管线参数。
type PipelineOpts struct {
	// MaxEdge >0 时长边超限等比缩。
	MaxEdge int
	// JpegQuality keep 路径 JPEG 质量；0 → 90。
	JpegQuality int
	// OutputWebP 为 true 时尝试输出 WebP（gif/webp 输入不会进入本管线）。
	OutputWebP bool
	// WebPQuality 0 → 80。
	WebPQuality int
	// SkipIfLarger WebP 不小于回退基准时保留 jpeg/png（或原字节）。
	SkipIfLarger bool
	// ForceReencode 为 true 时即使无缩放/水印也解码重编码（如 strip_exif）。
	ForceReencode bool

	TextEnabled   bool
	Text          string
	TextPosition  string
	TextOpacity   float64
	TextSizeRatio float64

	// MarkPNG 用户 PNG 水印字节；nil 表示无图印。
	MarkPNG      []byte
	MarkPosition string
	MarkOpacity  float64
	MarkMargin   int
}

// PipelineResult 管线输出。
type PipelineResult struct {
	Data   []byte
	// Format 为 image 包格式名："jpeg" | "png" | "webp"
	Format string
	// Changed 相对输入是否写回。
	Changed bool
}

// ProcessPipeline 对 jpeg/png 字节：解码一次 → 缩放/水印 → 末次编码（keep 或 webp）。
// 内容不可解码 → ErrUnsupported（调用方整链放弃、原字节不动）。
func ProcessPipeline(data []byte, opts PipelineOpts) (PipelineResult, error) {
	img, format, err := decodeJP(data)
	if err != nil {
		return PipelineResult{}, err
	}

	transformed := false

	if opts.MaxEdge > 0 {
		if scaled, did := scaleImage(img, opts.MaxEdge); did {
			img = scaled
			transformed = true
		}
	}
	if opts.TextEnabled && opts.Text != "" {
		v, err := watermarkTextImage(img, opts.Text, opts.TextPosition, opts.TextOpacity, opts.TextSizeRatio)
		if err != nil {
			return PipelineResult{}, err
		}
		img = v
		transformed = true
	}
	if len(opts.MarkPNG) > 0 {
		v, err := watermarkImageImage(img, opts.MarkPNG, opts.MarkPosition, opts.MarkOpacity, opts.MarkMargin)
		if err == ErrUnsupported {
			// 水印图坏：仅跳过图印（与 burn 旧语义一致）
		} else if err != nil {
			return PipelineResult{}, err
		} else {
			img = v
			transformed = true
		}
	}

	need := transformed || opts.ForceReencode || opts.OutputWebP
	if !need {
		return PipelineResult{Data: data, Format: format, Changed: false}, nil
	}

	// keep 路径：原格式末次编码（jpeg 用 JpegQuality；png 无损）
	keepBytes, err := encodeAs(format, img, opts.JpegQuality)
	if err != nil {
		return PipelineResult{}, err
	}

	if !opts.OutputWebP {
		changed := !bytes.Equal(keepBytes, data)
		return PipelineResult{Data: keepBytes, Format: format, Changed: changed}, nil
	}

	// WebP 路径
	if !WebPEncodeAvailable() {
		// 运行时无编码器：回退 keep（配置层应已拒绝开启）
		changed := !bytes.Equal(keepBytes, data)
		return PipelineResult{Data: keepBytes, Format: format, Changed: changed}, nil
	}
	webpBytes, err := EncodeWebP(img, opts.WebPQuality)
	if err != nil {
		return PipelineResult{}, err
	}

	// 回退基准：有变换/强制重编码时用 keep 结果；仅转码时用原字节
	baseline := data
	if transformed || opts.ForceReencode {
		baseline = keepBytes
	}
	if opts.SkipIfLarger && len(webpBytes) >= len(baseline) {
		if transformed || opts.ForceReencode {
			return PipelineResult{Data: keepBytes, Format: format, Changed: !bytes.Equal(keepBytes, data)}, nil
		}
		return PipelineResult{Data: data, Format: format, Changed: false}, nil
	}
	return PipelineResult{Data: webpBytes, Format: "webp", Changed: true}, nil
}

// scaleImage 长边超 maxEdge 则等比缩；否则返回原图。
func scaleImage(src image.Image, maxEdge int) (image.Image, bool) {
	sb := src.Bounds()
	w, h := sb.Dx(), sb.Dy()
	long := w
	if h > long {
		long = h
	}
	if long <= maxEdge {
		return src, false
	}
	nw, nh := w, h
	if w >= h {
		nw = maxEdge
		nh = h * maxEdge / w
	} else {
		nh = maxEdge
		nw = w * maxEdge / h
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, sb, xdraw.Src, nil)
	return dst, true
}

func watermarkImageImage(base image.Image, mark []byte, position string, opacity float64, margin int) (image.Image, error) {
	markImg, markFmt, err := image.Decode(bytes.NewReader(mark))
	if err != nil || markFmt != "png" {
		return nil, ErrUnsupported
	}
	bb := base.Bounds()
	w, h := bb.Dx(), bb.Dy()
	mb := markImg.Bounds()
	mw, mh := mb.Dx(), mb.Dy()

	maxW := w * 9 / 10
	maxH := h * 9 / 10
	if mw > maxW || mh > maxH {
		scaleW := float64(maxW) / float64(mw)
		scaleH := float64(maxH) / float64(mh)
		s := scaleW
		if scaleH < s {
			s = scaleH
		}
		nw := int(float64(mw)*s + 0.5)
		nh := int(float64(mh)*s + 0.5)
		if nw < 1 {
			nw = 1
		}
		if nh < 1 {
			nh = 1
		}
		scaled := image.NewRGBA(image.Rect(0, 0, nw, nh))
		xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), markImg, mb, xdraw.Src, nil)
		markImg = scaled
		mw, mh = nw, nh
	}

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), base, bb.Min, draw.Src)
	x, y := anchor(position, w, h, mw, mh, margin)
	r := image.Rect(x, y, x+mw, y+mh)
	mask := image.NewUniform(color.Alpha{A: uint8(opacity*255 + 0.5)})
	draw.DrawMask(dst, r, markImg, markImg.Bounds().Min, mask, image.Point{}, draw.Over)
	return dst, nil
}

func watermarkTextImage(base image.Image, text, position string, opacity, sizeRatio float64) (image.Image, error) {
	bb := base.Bounds()
	w, h := bb.Dx(), bb.Dy()
	long := w
	if h > long {
		long = h
	}
	size := sizeRatio * float64(long)
	if size < 12 {
		size = 12
	}
	shortEdge := w
	if h < shortEdge {
		shortEdge = h
	}
	if shortEdge >= 1 && size > float64(shortEdge) {
		size = float64(shortEdge)
	}

	// 迭代收缩字号，直到描边+padding 后的层能完整放入画布。
	// 旧实现单次线性缩放后仍可能因取整导致 lw>w，再钳层宽会裁掉末尾字形（「字不全」）。
	// pad=1 同时覆盖四向 1px 描边外扩（与历史行为一致，避免把层预算撑到 2+2）。
	const pad = 1
	var face font.Face
	var textW, textH, ascent, descent int
	for attempt := 0; attempt < 12; attempt++ {
		if size < 1 {
			size = 1
		}
		var err error
		face, err = textFace(size)
		if err != nil {
			return nil, err
		}
		d := &font.Drawer{Face: face}
		textW = d.MeasureString(text).Ceil()
		m := face.Metrics()
		ascent = m.Ascent.Ceil()
		descent = m.Descent.Ceil()
		textH = ascent + descent
		if textW < 1 {
			textW = 1
		}
		if textH < 1 {
			textH = 1
		}
		needW := textW + 2*pad
		needH := textH + 2*pad
		fitsW := w < 1 || needW <= w
		fitsH := h < 1 || needH <= h
		if fitsW && fitsH {
			break
		}
		scale := 1.0
		if !fitsW && textW > 0 && w > 2*pad {
			scale = float64(w-2*pad) / float64(textW)
		}
		if !fitsH && textH > 0 && h > 2*pad {
			sh := float64(h-2*pad) / float64(textH)
			if sh < scale {
				scale = sh
			}
		}
		if scale >= 1 || scale <= 0 {
			// 画布极小（如 e2e 3×2）：落到最小字号后钳层，靠 Dot 钳制仍写出墨迹
			if size <= 1 {
				break
			}
			size = 1
			continue
		}
		next := size * scale
		// 至少缩一点，避免浮点卡死在同一 size
		if next >= size*0.999 {
			next = size * 0.9
		}
		if next < 1 {
			next = 1
		}
		size = next
	}

	lw := textW + 2*pad
	lh := textH + 2*pad
	if lw < 1 {
		lw = 1
	}
	if lh < 1 {
		lh = 1
	}
	// 极端画布仍装不下时钳层（条带/微缩略图）；字号已尽量缩小
	if w >= 1 && lw > w {
		lw = w
	}
	if h >= 1 && lh > h {
		lh = h
	}
	layer := image.NewRGBA(image.Rect(0, 0, lw, lh))
	black := image.NewUniform(color.RGBA{A: 255})
	white := image.NewUniform(color.RGBA{R: 255, G: 255, B: 255, A: 255})
	// Dot 必须落在层内：层被钳到极小画布时若仍用 pad+ascent，baseline 会在层外 → 零像素水印 → 哈希不变误秒传
	baseDotX := pad
	if baseDotX >= lw {
		baseDotX = 0
	}
	baseDotY := pad + ascent
	if baseDotY >= lh {
		baseDotY = lh - 1
		if baseDotY < 1 {
			baseDotY = 1
		}
	}
	if baseDotY < 1 {
		baseDotY = 1
	}
	for _, off := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		drawer := &font.Drawer{
			Dst: layer, Src: black, Face: face,
			Dot: fixed.P(baseDotX+off[0], baseDotY+off[1]),
		}
		drawer.DrawString(text)
	}
	drawer := &font.Drawer{
		Dst: layer, Src: white, Face: face,
		Dot: fixed.P(baseDotX, baseDotY),
	}
	drawer.DrawString(text)

	margin := int(size) / 2
	x, y := anchor(position, w, h, lw, lh, margin)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), base, bb.Min, draw.Src)
	r := image.Rect(x, y, x+lw, y+lh)
	mask := image.NewUniform(color.Alpha{A: uint8(opacity*255 + 0.5)})
	draw.DrawMask(dst, r, layer, image.Point{}, mask, image.Point{}, draw.Over)
	return dst, nil
}

// EncodePNG 供测试与 vips 中转。
func EncodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
