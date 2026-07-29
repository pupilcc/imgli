package imaging

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// StripMetadata 解码后按原格式重编码，去掉 JPEG EXIF/GPS 等附属元数据（及 PNG 文本块等）。
// 仅 jpeg/png；其它格式返回 ErrUnsupported。无元数据的干净图也可能因重编码改变字节。
func StripMetadata(data []byte) ([]byte, error) {
	img, format, err := decodeJP(data)
	if err != nil {
		return nil, err
	}
	return encodeAs(format, img)
}

// Scale 长边超过 maxEdge 时等比缩(CatmullRom);未超出时原样返回输入(不解码重编)。
func Scale(data []byte, maxEdge int) ([]byte, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, ErrUnsupported
	}
	if format != "jpeg" && format != "png" {
		return nil, ErrUnsupported
	}
	w, h := cfg.Width, cfg.Height
	long := w
	if h > long {
		long = h
	}
	if long <= maxEdge {
		return data, nil
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrUnsupported
	}
	sb := src.Bounds()
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
	return encodeAs(format, dst)
}

// WatermarkImage 把 PNG 水印图按九宫格 position+margin 叠加到底图,opacity∈(0,1] 经
// alpha mask 生效。mark 任一边超过底图对应边 90% 时先等比缩至 90%。mark 非 PNG → ErrUnsupported。
func WatermarkImage(data, mark []byte, position string, opacity float64, margin int) ([]byte, error) {
	base, format, err := decodeJP(data)
	if err != nil {
		return nil, err
	}
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
	return encodeAs(format, dst)
}

// WatermarkText 站点文字水印:字号 max(12, sizeRatio×长边),白字+四向 1px 黑描边
// (任意底色可读),margin=字号/2,按九宫格 position 定位(按测量宽高贴边不出界)。
func WatermarkText(data []byte, text, position string, opacity, sizeRatio float64) ([]byte, error) {
	base, format, err := decodeJP(data)
	if err != nil {
		return nil, err
	}
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
	// 字号上限:单字不超过底图短边——防极端长宽比(如 30000×20 允许的解压上限图)
	// 下 size=0.2×长边 使中间层远大于底图 → 数 GB 级分配耗尽内存(codex 终审)。
	shortEdge := w
	if h < shortEdge {
		shortEdge = h
	}
	if shortEdge >= 1 && size > float64(shortEdge) {
		size = float64(shortEdge)
	}

	face, err := textFace(size)
	if err != nil {
		return nil, err
	}

	d := &font.Drawer{Face: face}
	textW := d.MeasureString(text).Ceil()
	// 文本比底图宽则按比例缩小字号并重量一次:保证中间层不超过底图尺寸(codex 终审)。
	if textW > w && w >= 1 {
		size = size * float64(w) / float64(textW)
		if size < 1 {
			size = 1
		}
		if face, err = textFace(size); err != nil {
			return nil, err
		}
		d = &font.Drawer{Face: face}
		textW = d.MeasureString(text).Ceil()
	}
	m := face.Metrics()
	ascent := m.Ascent.Ceil()
	descent := m.Descent.Ceil()
	textH := ascent + descent
	if textW < 1 {
		textW = 1
	}
	if textH < 1 {
		textH = 1
	}

	// 描边 ±1，四周留 1px
	const pad = 1
	lw := textW + 2*pad
	lh := textH + 2*pad
	// 硬上限:中间层不超过底图边界——文本仍过大则裁剪,保证分配 ≤ 底图 RGBA 量级
	// (与已接受的底图解码同阶,不引入新 OOM 面;codex 终审)。
	if lw > w {
		lw = w
	}
	if lh > h {
		lh = h
	}
	layer := image.NewRGBA(image.Rect(0, 0, lw, lh))

	black := image.NewUniform(color.RGBA{A: 255})
	white := image.NewUniform(color.RGBA{R: 255, G: 255, B: 255, A: 255})
	baseDotX := pad
	baseDotY := pad + ascent

	for _, off := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		drawer := &font.Drawer{
			Dst:  layer,
			Src:  black,
			Face: face,
			Dot:  fixed.P(baseDotX+off[0], baseDotY+off[1]),
		}
		drawer.DrawString(text)
	}
	drawer := &font.Drawer{
		Dst:  layer,
		Src:  white,
		Face: face,
		Dot:  fixed.P(baseDotX, baseDotY),
	}
	drawer.DrawString(text)

	margin := int(size) / 2
	x, y := anchor(position, w, h, lw, lh, margin)

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), base, bb.Min, draw.Src)

	r := image.Rect(x, y, x+lw, y+lh)
	mask := image.NewUniform(color.Alpha{A: uint8(opacity*255 + 0.5)})
	draw.DrawMask(dst, r, layer, image.Point{}, mask, image.Point{}, draw.Over)
	return encodeAs(format, dst)
}

// anchor 九宫格定位:tl|tc|tr|ml|mc|mr|bl|bc|br,返回 mark 左上角坐标。
func anchor(position string, w, h, mw, mh, margin int) (x, y int) {
	switch position {
	case "tl":
		x, y = margin, margin
	case "tc":
		x, y = (w-mw)/2, margin
	case "tr":
		x, y = w-mw-margin, margin
	case "ml":
		x, y = margin, (h-mh)/2
	case "mc":
		x, y = (w-mw)/2, (h-mh)/2
	case "mr":
		x, y = w-mw-margin, (h-mh)/2
	case "bl":
		x, y = margin, h-mh-margin
	case "bc":
		x, y = (w-mw)/2, h-mh-margin
	case "br":
		x, y = w-mw-margin, h-mh-margin
	default:
		// 非法值按 br
		x, y = w-mw-margin, h-mh-margin
	}
	// 双向钳制在画布内:大 margin 不得把水印推出画布(否则处理成功却不可见;codex 终审)。
	// 先钳上界(mw>w 时 w-mw<0,随后下界拉回 0),再钳下界。
	if maxX := w - mw; x > maxX {
		x = maxX
	}
	if maxY := h - mh; y > maxY {
		y = maxY
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x, y
}

// decodeJP 仅认 jpeg/png。
func decodeJP(data []byte) (image.Image, string, error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", ErrUnsupported
	}
	if format != "jpeg" && format != "png" {
		return nil, "", ErrUnsupported
	}
	return img, format, nil
}

// encodeAs 按 format 重编码。
func encodeAs(format string, img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	switch format {
	case "jpeg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
			return nil, err
		}
	case "png":
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
	default:
		return nil, ErrUnsupported
	}
	return buf.Bytes(), nil
}
