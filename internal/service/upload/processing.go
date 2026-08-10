// 配置类型前置于 upload 包,adminsvc 与烧录管线共用,依赖方向 adminsvc→upload 无环。
package upload

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/yixian-huang/imgli/internal/apperr"
	"github.com/yixian-huang/imgli/internal/imaging"
)

// Positions 兼容别名——真源在 imaging.Positions，避免 auth 等包 import upload。
var Positions = imaging.Positions

// OutputFormat 原图输出策略。
const (
	OutputKeep = "keep"
	OutputWebP = "webp"
)

// ErrProcessingInvalid 图片处理配置无效。
var ErrProcessingInvalid = apperr.New("图片处理配置无效:position 需九宫格、opacity 需 (0,1]、size_ratio 需 0.01-0.2、启用时 text 非空且 ≤64 字符、max_edge 需 0 或 256-16384、jpeg_quality 需 0 或 1-100、output_format 需 keep|webp、webp_quality 需 0 或 1-100、output_format=webp 需本构建支持 WebP 编码")

// TextWatermark 站点级文字水印配置(settings "processing".text_watermark)。
type TextWatermark struct {
	Enabled   bool    `json:"enabled"`
	Text      string  `json:"text"`
	Position  string  `json:"position"`   // 九宫格 tl|tc|tr|ml|mc|mr|bl|bc|br
	Opacity   float64 `json:"opacity"`    // (0,1]
	SizeRatio float64 `json:"size_ratio"` // [0.01,0.2] 字号相对长边比例
}

// Processing 图片处理规则(settings "processing" 键 JSON 契约,前后端逐字一致)。
// StripExif 用指针：JSON 缺字段(存量配置) → nil → 视为开启(隐私默认开)。
// WebPSkipIfLarger 用指针：缺字段 → nil → 视为开启(体积变大则回退)。
type Processing struct {
	TextWatermark TextWatermark `json:"text_watermark"`
	MaxEdge       int           `json:"max_edge"` // 0=不限;否则 [256,16384],上传超限等比缩
	// StripExif nil=默认开；false=保留源文件元数据；true=上传时剥离 EXIF/GPS 等。
	StripExif *bool `json:"strip_exif"`
	// JpegQuality JPEG 重编码质量：0=默认 90；否则 [1,100]。仅在 keep 路径重编码时生效。
	JpegQuality int `json:"jpeg_quality"`
	// OutputFormat keep（默认）| webp。仅 jpeg/png 输入可转；gif/webp 输入不转。
	OutputFormat string `json:"output_format"`
	// WebPQuality 转 WebP 时的质量：0=默认 80；否则 [1,100]。
	WebPQuality int `json:"webp_quality"`
	// WebPSkipIfLarger nil=默认开；转 WebP 后不小于回退基准则保留 jpeg/png（或原字节）。
	WebPSkipIfLarger *bool `json:"webp_skip_if_larger"`
}

// EffectiveJPEGQuality 返回重编码用的 JPEG 质量（0/非法 → 90）。
func (p Processing) EffectiveJPEGQuality() int {
	if p.JpegQuality < 1 || p.JpegQuality > 100 {
		return 90
	}
	return p.JpegQuality
}

// EffectiveWebPQuality 返回 WebP 质量（0/非法 → 80）。
func (p Processing) EffectiveWebPQuality() int {
	if p.WebPQuality < 1 || p.WebPQuality > 100 {
		return 80
	}
	return p.WebPQuality
}

// EffectiveOutputFormat keep|webp；非法/空 → keep。
func (p Processing) EffectiveOutputFormat() string {
	if strings.EqualFold(strings.TrimSpace(p.OutputFormat), OutputWebP) {
		return OutputWebP
	}
	return OutputKeep
}

// WebPSkipIfLargerEnabled 缺省(nil)为 true。
func (p Processing) WebPSkipIfLargerEnabled() bool {
	if p.WebPSkipIfLarger == nil {
		return true
	}
	return *p.WebPSkipIfLarger
}

// BoolPtr 供测试与播种构造 *bool。
func BoolPtr(v bool) *bool { return &v }

// StripExifEnabled 缺省(nil)为 true。
func (p Processing) StripExifEnabled() bool {
	if p.StripExif == nil {
		return true
	}
	return *p.StripExif
}

// DefaultProcessing 返回图片处理的出厂默认值。
//
// 播种进 model.Seed 的 JSON 字面量（internal/model/db.go 的 SettingProcessing 项）必须与
// 本函数返回值逐字段一致——db.go 出于避免 model⇄upload 循环依赖，手写了等价的 JSON
// 字面量而非 import 本包；两者的一致性由 internal/model/processing_seed_test.go
// （package model_test 外部测试包）断言。
func DefaultProcessing() Processing {
	return Processing{
		TextWatermark: TextWatermark{
			Enabled:   false,
			Text:      "",
			Position:  "br",
			Opacity:   0.35,
			SizeRatio: 0.04,
		},
		MaxEdge:          0,
		StripExif:        BoolPtr(true),
		JpegQuality:      0, // 0 → EffectiveJPEGQuality 90
		OutputFormat:     OutputKeep,
		WebPQuality:      0, // 0 → EffectiveWebPQuality 80
		WebPSkipIfLarger: BoolPtr(true),
	}
}

// ValidateProcessing 校验(全部违规返回 ErrProcessingInvalid)。
// output_format=webp 时还要求本构建 WebPEncodeAvailable（无 vips 的纯 Go 构建不可开启）。
// 启用文字水印时还会检查嵌入字体字形覆盖，缺字会显示为口/方框，拒绝保存。
func ValidateProcessing(p Processing) error {
	tw := p.TextWatermark
	if !imaging.Positions[tw.Position] {
		return ErrProcessingInvalid
	}
	if tw.Opacity <= 0 || tw.Opacity > 1 {
		return ErrProcessingInvalid
	}
	if tw.SizeRatio < 0.01 || tw.SizeRatio > 0.2 {
		return ErrProcessingInvalid
	}
	if tw.Enabled {
		text := strings.TrimSpace(tw.Text)
		if text == "" || utf8.RuneCountInString(text) > 64 {
			return ErrProcessingInvalid
		}
		if miss := imaging.MissingWatermarkRunes(text); len(miss) > 0 {
			// 动态文案便于管理端定位缺字；仍可 errors.Is → ErrProcessingInvalid
			return fmt.Errorf("%w: 文字水印含字体未覆盖字符（会显示为口）: %s", ErrProcessingInvalid, string(miss))
		}
	}
	if p.MaxEdge != 0 && (p.MaxEdge < 256 || p.MaxEdge > 16384) {
		return ErrProcessingInvalid
	}
	if p.JpegQuality != 0 && (p.JpegQuality < 1 || p.JpegQuality > 100) {
		return ErrProcessingInvalid
	}
	of := strings.TrimSpace(strings.ToLower(p.OutputFormat))
	if of != "" && of != OutputKeep && of != OutputWebP {
		return ErrProcessingInvalid
	}
	if p.WebPQuality != 0 && (p.WebPQuality < 1 || p.WebPQuality > 100) {
		return ErrProcessingInvalid
	}
	if of == OutputWebP && !imaging.WebPEncodeAvailable() {
		return ErrProcessingInvalid
	}
	return nil
}
