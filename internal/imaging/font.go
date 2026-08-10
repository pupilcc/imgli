package imaging

import (
	_ "embed"
	"sync"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
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

func ensureFont() error {
	fontOnce.Do(func() {
		parsedFont, fontErr = opentype.Parse(notoSansSC)
	})
	return fontErr
}

// textFace 返回指定字号的 Face。*opentype.Font 经 sync.Once 懒解析复用;
// Face 非并发安全,每次调用新建。
func textFace(size float64) (font.Face, error) {
	if err := ensureFont(); err != nil {
		return nil, err
	}
	return opentype.NewFace(parsedFont, &opentype.FaceOptions{
		Size: size,
		DPI:  72,
	})
}

// MissingWatermarkRunes 返回文字水印字体未覆盖的字符（gid=0 → 渲染为口/豆腐块）。
// 空白符跳过；解析失败返回 nil 并由调用方走其它校验。
// 最多返回 16 个不重复 rune，便于管理端错误提示。
func MissingWatermarkRunes(text string) []rune {
	if err := ensureFont(); err != nil || parsedFont == nil {
		return nil
	}
	var buf sfnt.Buffer
	seen := make(map[rune]struct{})
	out := make([]rune, 0, 8)
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		if _, ok := seen[r]; ok {
			continue
		}
		gid, err := parsedFont.GlyphIndex(&buf, r)
		if err != nil || gid == 0 {
			seen[r] = struct{}{}
			out = append(out, r)
			if len(out) >= 16 {
				break
			}
		}
	}
	return out
}
