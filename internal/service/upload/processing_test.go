package upload

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateProcessingDefaultOK(t *testing.T) {
	if err := ValidateProcessing(DefaultProcessing()); err != nil {
		t.Fatalf("默认值应合法: %v", err)
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
	}
	for _, tc := range bads {
		p := DefaultProcessing()
		tc.mod(&p)
		if err := ValidateProcessing(p); !errors.Is(err, ErrProcessingInvalid) {
			t.Errorf("%s: err=%v want ErrProcessingInvalid", tc.name, err)
		}
	}
}
