package upload

import (
	"errors"
	"strings"
	"testing"

	"github.com/yixian-huang/imgli/internal/imaging"
)

func TestValidateProcessingDefaultOK(t *testing.T) {
	if err := ValidateProcessing(DefaultProcessing()); err != nil {
		t.Fatalf("默认值应合法: %v", err)
	}
}

func TestStripExifEnabledDefaultsTrue(t *testing.T) {
	if !DefaultProcessing().StripExifEnabled() {
		t.Error("default strip on")
	}
	var p Processing // zero: StripExif nil → 存量配置视为开
	if !p.StripExifEnabled() {
		t.Error("missing field 应视为 strip on")
	}
	p.StripExif = BoolPtr(false)
	if p.StripExifEnabled() {
		t.Error("explicit false")
	}
}

func TestEffectiveJPEGQuality(t *testing.T) {
	if q := DefaultProcessing().EffectiveJPEGQuality(); q != 90 {
		t.Errorf("default effective = %d, want 90", q)
	}
	p := DefaultProcessing()
	p.JpegQuality = 75
	if q := p.EffectiveJPEGQuality(); q != 75 {
		t.Errorf("75 → %d", q)
	}
	p.JpegQuality = 0
	if q := p.EffectiveJPEGQuality(); q != 90 {
		t.Errorf("0 → %d want 90", q)
	}
}

func TestEffectiveOutputAndWebP(t *testing.T) {
	d := DefaultProcessing()
	if d.EffectiveOutputFormat() != OutputKeep {
		t.Errorf("default format = %q", d.EffectiveOutputFormat())
	}
	if d.EffectiveWebPQuality() != 80 {
		t.Errorf("default webp q = %d", d.EffectiveWebPQuality())
	}
	if !d.WebPSkipIfLargerEnabled() {
		t.Error("skip_if_larger default on")
	}
	d.OutputFormat = "webp"
	if d.EffectiveOutputFormat() != OutputWebP {
		t.Errorf("webp → %q", d.EffectiveOutputFormat())
	}
	d.WebPQuality = 72
	if d.EffectiveWebPQuality() != 72 {
		t.Errorf("72 → %d", d.EffectiveWebPQuality())
	}
	d.WebPSkipIfLarger = BoolPtr(false)
	if d.WebPSkipIfLargerEnabled() {
		t.Error("explicit false")
	}
}

func TestValidateProcessingMatrix(t *testing.T) {
	ok := DefaultProcessing()

	// 合法边界
	legals := []Processing{
		func() Processing {
			p := ok
			p.MaxEdge = 256
			return p
		}(),
		func() Processing {
			p := ok
			p.MaxEdge = 16384
			return p
		}(),
		func() Processing {
			p := ok
			p.TextWatermark.Enabled = true
			p.TextWatermark.Text = strings.Repeat("字", 64)
			return p
		}(),
		func() Processing {
			p := ok
			p.JpegQuality = 1
			return p
		}(),
		func() Processing {
			p := ok
			p.JpegQuality = 100
			return p
		}(),
		func() Processing {
			p := ok
			p.OutputFormat = OutputKeep
			p.WebPQuality = 100
			return p
		}(),
	}
	if imaging.WebPEncodeAvailable() {
		legals = append(legals, func() Processing {
			p := ok
			p.OutputFormat = OutputWebP
			p.WebPQuality = 80
			return p
		}())
	}
	for i, p := range legals {
		if err := ValidateProcessing(p); err != nil {
			t.Errorf("legal[%d]: %v", i, err)
		}
	}

	// 非法
	bads := []struct {
		name string
		mod  func(*Processing)
	}{
		{"position xx", func(p *Processing) { p.TextWatermark.Position = "xx" }},
		{"opacity 0", func(p *Processing) { p.TextWatermark.Opacity = 0 }},
		{"opacity 1.2", func(p *Processing) { p.TextWatermark.Opacity = 1.2 }},
		{"size_ratio 0.005", func(p *Processing) { p.TextWatermark.SizeRatio = 0.005 }},
		{"size_ratio 0.5", func(p *Processing) { p.TextWatermark.SizeRatio = 0.5 }},
		{"enabled empty text", func(p *Processing) {
			p.TextWatermark.Enabled = true
			p.TextWatermark.Text = "   "
		}},
		{"enabled 65 runes", func(p *Processing) {
			p.TextWatermark.Enabled = true
			p.TextWatermark.Text = strings.Repeat("字", 65)
		}},
		{"max_edge 100", func(p *Processing) { p.MaxEdge = 100 }},
		{"max_edge 20000", func(p *Processing) { p.MaxEdge = 20000 }},
		{"jpeg_quality -1", func(p *Processing) { p.JpegQuality = -1 }},
		{"jpeg_quality 101", func(p *Processing) { p.JpegQuality = 101 }},
		{"output_format avif", func(p *Processing) { p.OutputFormat = "avif" }},
		{"webp_quality 101", func(p *Processing) { p.WebPQuality = 101 }},
		// 无 WebP 编码器的构建上开启 webp 必须失败
		{"output_format webp without encoder", func(p *Processing) {
			if imaging.WebPEncodeAvailable() {
				p.OutputFormat = "avif" // vips 构建用另一非法值占位
			} else {
				p.OutputFormat = OutputWebP
			}
		}},
	}
	for _, tc := range bads {
		p := DefaultProcessing()
		tc.mod(&p)
		if err := ValidateProcessing(p); !errors.Is(err, ErrProcessingInvalid) {
			t.Errorf("%s: err=%v want ErrProcessingInvalid", tc.name, err)
		}
	}
}
