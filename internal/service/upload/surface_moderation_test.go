package upload

import (
	"context"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
)

// TestCrossSurfaceInheritsRejected 回归 M-A:私密重传已被拒审的公开图字节,
// scoped-dedup 下会新建独立私密 File,若不跨 surface 继承审核结论,已拒内容会
// 以新 surface 复活为 normal(绕过 commitInstant 的同 file 继承)。
func TestCrossSurfaceInheritsRejected(t *testing.T) {
	svc, u, _ := setup(t)
	dir := t.TempDir()

	res1, err := svc.Save(context.Background(), pngFile(t, dir, 300, 200), "a.png", u, Opts{Visibility: "public"}, "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	// 把该 File 上的 image 置为 rejected,模拟机审已拒
	if err := svc.db.Model(&model.Image{}).Where("file_id = ?", res1.File.ID).Update("status", "rejected").Error; err != nil {
		t.Fatal(err)
	}

	res2, err := svc.Save(context.Background(), pngFile(t, dir, 300, 200), "b.png", u, Opts{Visibility: "private"}, "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}

	if res2.File.ID == res1.File.ID {
		t.Fatal("私密图应指向独立 File,不与公开图共享")
	}
	if res2.File.Surface != model.SurfacePrivate {
		t.Errorf("res2.File.Surface=%q, want private", res2.File.Surface)
	}

	var reloaded model.Image
	if err := svc.db.First(&reloaded, res2.Image.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reloaded.Status != "rejected" {
		t.Errorf("跨 surface 应继承 rejected, got status=%q", reloaded.Status)
	}
}

// TestCrossSurfaceMidFlightStillEnqueues 机审中途窗口(status=normal 但已写 score)时,
// 跨 surface 重传不能因"有分"跳过入队——否则原图随后变 rejected,新图却永久停 normal。
func TestCrossSurfaceMidFlightStillEnqueues(t *testing.T) {
	svc, u, _ := setup(t)
	dir := t.TempDir()
	res1, err := svc.Save(context.Background(), pngFile(t, dir, 300, 200), "a.png", u, Opts{Visibility: "public"}, "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	// 模拟机审中途:已写 score,status 仍 normal(尚未 applyStatusDecision)
	score := 0.91
	if err := svc.db.Model(&model.Image{}).Where("id = ?", res1.Image.ID).
		Update("nsfw_score", score).Error; err != nil {
		t.Fatal(err)
	}
	// 清掉首次上传入队的任务,便于断言本次是否入队
	svc.db.Where("type = ?", "moderate_image").Delete(&model.Task{})

	// 跨 surface 重传相同字节(私密)
	res2, err := svc.Save(context.Background(), pngFile(t, dir, 300, 200), "b.png", u, Opts{Visibility: "private"}, "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	if res2.Image.Status != "normal" {
		t.Fatalf("中途窗口继承应为 normal, got %q", res2.Image.Status)
	}
	var pending int64
	svc.db.Model(&model.Task{}).Where("type = ? AND status = ?", "moderate_image", "pending").Count(&pending)
	if pending == 0 {
		t.Error("中途窗口跨 surface 重传应自行入队机审,不能因继承到分数而跳过")
	}
}
