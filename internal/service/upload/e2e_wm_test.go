package upload

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/settings"
)

func TestE2EStyleWatermarkBreaksInstant(t *testing.T) {
	svc, u, _ := setup(t)
	dir := t.TempDir()
	raw, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAMAAAACCAYAAACddGYaAAAAEUlEQVR4nGPgEpH7D8MMyBwAV1kHYzmDy0UAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	write := func(name string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}

	// default processing (strip on)
	res1, err := svc.Save(context.Background(), write("a1.png"), "a.png", u, Opts{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if res1.Instant {
		t.Fatal("first should not be instant")
	}

	res2, err := svc.Save(context.Background(), write("a2.png"), "a.png", u, Opts{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !res2.Reused {
		t.Fatalf("second should reuse, got instant=%v reused=%v", res2.Instant, res2.Reused)
	}

	proc := Processing{
		TextWatermark: TextWatermark{
			Enabled: true, Text: "imgli-e2e", Position: "br", Opacity: 0.35, SizeRatio: 0.04,
		},
		MaxEdge: 0,
	}
	if err := ValidateProcessing(proc); err != nil {
		t.Fatal(err)
	}
	if err := settings.New(svc.db).Set(model.SettingProcessing, proc); err != nil {
		t.Fatal(err)
	}

	res3, err := svc.Save(context.Background(), write("a3.png"), "a-wm.png", u, Opts{}, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("res3 instant=%v reused=%v hash1=%s hash3=%s", res3.Instant, res3.Reused, res1.File.Hash, res3.File.Hash)
	if res3.Instant || res3.Reused {
		t.Fatalf("after watermark should be new upload, instant=%v reused=%v", res3.Instant, res3.Reused)
	}
	if res3.File.Hash == res1.File.Hash {
		t.Fatal("hash should change after watermark")
	}
}
