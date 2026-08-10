package imaging

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

// TestTinyPNGWatermarkChangesHash 回归 e2e 3×2 PNG：水印必须改像素/哈希，
// 否则开文字水印后同字节仍命中秒传（processing.spec「开水印后非秒传」）。
func TestTinyPNGWatermarkChangesHash(t *testing.T) {
	raw, err := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAMAAAACCAYAAACddGYaAAAAEUlEQVR4nGPgEpH7D8MMyBwAV1kHYzmDy0UAAAAASUVORK5CYII=",
	)
	if err != nil {
		t.Fatal(err)
	}
	h := func(b []byte) string {
		s := sha256.Sum256(b)
		return hex.EncodeToString(s[:])
	}
	strip, err := ProcessPipeline(raw, PipelineOpts{ForceReencode: true})
	if err != nil {
		t.Fatal(err)
	}
	wm, err := ProcessPipeline(raw, PipelineOpts{
		ForceReencode: true,
		TextEnabled:   true,
		Text:          "imgli-e2e",
		TextPosition:  "br",
		TextOpacity:   0.35,
		TextSizeRatio: 0.04,
	})
	if err != nil {
		t.Fatal(err)
	}
	if h(strip.Data) == h(wm.Data) {
		t.Fatalf("watermark must change hash vs strip-only on 3×2 (strip=%s wm=%s)", h(strip.Data)[:16], h(wm.Data)[:16])
	}
}
