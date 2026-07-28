package upload

import (
	"context"
	"strings"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
)

// TestSaveDedupSameSurfaceReuses 同 surface 同字节 → 秒传复用同 File。
func TestSaveDedupSameSurfaceReuses(t *testing.T) {
	svc, u, _ := setup(t)
	dir := t.TempDir()
	r1, err := svc.Save(context.Background(), pngFile(t, dir, 300, 200), "a.png", u, Opts{Visibility: "public"}, "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	r2, err := svc.Save(context.Background(), pngFile(t, dir, 300, 200), "b.png", u, Opts{Visibility: "public"}, "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	if !r2.Instant || r2.File.ID != r1.File.ID {
		t.Error("同 surface 同字节应秒传复用同 File")
	}
}

// TestSaveDedupCrossSurfaceSeparateObject 公开图与私密图字节相同 → 不共享,
// 各自独立 File、独立 surface、独立前缀。这是「真隐私」的核心行为。
func TestSaveDedupCrossSurfaceSeparateObject(t *testing.T) {
	svc, u, _ := setup(t)
	dir := t.TempDir()
	pub, err := svc.Save(context.Background(), pngFile(t, dir, 300, 200), "a.png", u, Opts{Visibility: "public"}, "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	priv, err := svc.Save(context.Background(), pngFile(t, dir, 300, 200), "b.png", u, Opts{Visibility: "private"}, "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	if priv.Instant {
		t.Error("私密图撞公开字节不应秒传共享,应复制独立私密对象")
	}
	if priv.File.ID == pub.File.ID {
		t.Error("私密图应指向独立 File,不与公开图共享")
	}
	if pub.File.Surface != model.SurfacePublic || priv.File.Surface != model.SurfacePrivate {
		t.Errorf("surface 错: pub=%q priv=%q", pub.File.Surface, priv.File.Surface)
	}
	if !strings.HasPrefix(pub.File.Path, "public/") {
		t.Errorf("公开图对象键应带 public/ 前缀, got %q", pub.File.Path)
	}
	if !strings.HasPrefix(priv.File.Path, "private/") {
		t.Errorf("私密图对象键应带 private/ 前缀, got %q", priv.File.Path)
	}
}
