package imaging

import (
	_ "embed"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

// 子集 Noto Sans SC（GB2312 一级 + Latin/标点），完整字体约 8MB 压到 ~0.9MB。
// 再生：scripts/subset-watermark-font.sh，见 fonts/README.md。
//
//go:embed fonts/NotoSansSC-Regular.otf
var notoSansSC []byte

var (
	fontOnce   sync.Once
	parsedFont *opentype.Font
	fontErr    error
)

// textFace 返回指定字号的 Face。*opentype.Font 经 sync.Once 懒解析复用;
// Face 非并发安全,每次调用新建。
func textFace(size float64) (font.Face, error) {
	fontOnce.Do(func() {
		parsedFont, fontErr = opentype.Parse(notoSansSC)
	})
	if fontErr != nil {
		return nil, fontErr
	}
	return opentype.NewFace(parsedFont, &opentype.FaceOptions{
		Size: size,
		DPI:  72,
	})
}
