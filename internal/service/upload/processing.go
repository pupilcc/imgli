// 配置类型前置于 upload 包,adminsvc 与烧录管线共用,依赖方向 adminsvc→upload 无环。
package upload

import (
	"strings"
	"unicode/utf8"

	"github.com/yixian-huang/imgli/internal/apperr"
	"github.com/yixian-huang/imgli/internal/imaging"
)

// Positions 兼容别名——真源在 imaging.Positions，避免 auth 等包 import upload。
var Positions = imaging.Positions

// ErrProcessingInvalid 图片处理配置无效。
var ErrProcessingInvalid = apperr.New("图片处理配置无效:position 需九宫格、opacity 需 (0,1]、size_ratio 需 0.01-0.2、启用时 text 非空且 ≤64 字符、max_edge 需 0 或 256-16384")

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
type Processing struct {
	TextWatermark TextWatermark `json:"text_watermark"`
	MaxEdge       int           `json:"max_edge"` // 0=不限;否则 [256,16384],上传超限等比缩
	// StripExif nil=默认开；false=保留源文件元数据；true=上传时剥离 EXIF/GPS 等。
	StripExif *bool `json:"strip_exif"`
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
		MaxEdge:   0,
		StripExif: BoolPtr(true),
	}
}

// ValidateProcessing 校验(全部违规返回 ErrProcessingInvalid):
//
//	position ∈ 九宫格;opacity ∈ (0,1];size_ratio ∈ [0.01,0.2];
//	enabled 时 TrimSpace(text) 非空且 utf8 长度 ≤64;max_edge == 0 || 256 ≤ max_edge ≤ 16384。
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
	}
	if p.MaxEdge != 0 && (p.MaxEdge < 256 || p.MaxEdge > 16384) {
		return ErrProcessingInvalid
	}
	return nil
}
