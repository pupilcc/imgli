package imagesvc

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/config"
	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/storagesvc"
)

// setupPurge 造用户(已用量=file.Size)、一张软删图、其 file(ref_count=1)。
func setupPurge(t *testing.T) (*Service, uint64, string, uint64) {
	db := model.TestDB(t)
	u := &model.User{Username: "p", Email: "p@img.li", GroupID: 1, UsedStorage: 500}
	db.Create(u)
	f := &model.File{Hash: "h1", StoragePolicyID: 1, Path: "x/y", Size: 500, MIME: "image/png", RefCount: 1}
	db.Create(f)
	img := &model.Image{Key: "purgekey0001", UserID: &u.ID, FileID: f.ID, Name: "n", Ext: "png", Visibility: "public", Status: "normal"}
	db.Create(img)
	db.Delete(img) // 软删
	return New(db, storagesvc.New(&config.Config{DataDir: t.TempDir()}, db), nil), u.ID, img.Key, f.ID
}

func TestAdminPurgeLiveImage(t *testing.T) {
	db := model.TestDB(t)
	u := &model.User{Username: "ap", Email: "ap@img.li", GroupID: 1, UsedStorage: 300}
	db.Create(u)
	f := &model.File{Hash: "hadmin", StoragePolicyID: 1, Path: "adm/x", Size: 300, MIME: "image/png", RefCount: 1}
	db.Create(f)
	img := &model.Image{Key: "adminpurge001", UserID: &u.ID, FileID: f.ID, Name: "n", Ext: "png", Visibility: "public", Status: "normal"}
	db.Create(img)
	s := New(db, storagesvc.New(&config.Config{DataDir: t.TempDir()}, db), nil)

	if _, err := s.AdminPurge(img.Key); err != nil {
		t.Fatal(err)
	}
	var cnt int64
	db.Unscoped().Model(&model.Image{}).Where("key = ?", img.Key).Count(&cnt)
	if cnt != 0 {
		t.Errorf("AdminPurge 应硬删 image, still %d", cnt)
	}
	db.Model(&model.File{}).Where("id = ?", f.ID).Count(&cnt)
	if cnt != 0 {
		t.Errorf("AdminPurge 应删 file, still %d", cnt)
	}
	var user model.User
	db.First(&user, u.ID)
	if user.UsedStorage != 0 {
		t.Errorf("应退配额, used=%d", user.UsedStorage)
	}
}

func TestAdminPurgeNotFound(t *testing.T) {
	s, _, _, _ := setupPurge(t)
	if _, err := s.AdminPurge("nosuchkey0001"); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestPurgeReturnsQuotaAndDropsRefCount(t *testing.T) {
	s, uid, key, fileID := setupPurge(t)
	if err := s.PurgePermanent(uid, key); err != nil {
		t.Fatal(err)
	}
	var u model.User
	s.db.First(&u, uid)
	if u.UsedStorage != 0 {
		t.Errorf("彻底删除应退配额至 0, got %d", u.UsedStorage)
	}
	// ref_count 归零 → file 行应被删
	var cnt int64
	s.db.Model(&model.File{}).Where("id = ?", fileID).Count(&cnt)
	if cnt != 0 {
		t.Errorf("ref_count 归零应删 file 行, 仍存在")
	}
	// image 行硬删
	s.db.Unscoped().Model(&model.Image{}).Where("key = ?", key).Count(&cnt)
	if cnt != 0 {
		t.Errorf("彻底删除应硬删 image 行")
	}
}

func TestPurgeSharedFileKeepsPhysical(t *testing.T) {
	s, uid, key, fileID := setupPurge(t)
	// 再加一张引用同 file 的图，ref_count=2
	s.db.Model(&model.File{}).Where("id = ?", fileID).Update("ref_count", 2)
	if err := s.PurgePermanent(uid, key); err != nil {
		t.Fatal(err)
	}
	var f model.File
	if err := s.db.First(&f, fileID).Error; err != nil {
		t.Fatalf("ref_count 未归零, file 不应删: %v", err)
	}
	if f.RefCount != 1 {
		t.Errorf("ref_count 应降到 1, got %d", f.RefCount)
	}
}

func TestPurgeExpiredTrashSweeps(t *testing.T) {
	s, uid, key, _ := setupPurge(t)
	// 把 deleted_at 改到 40 天前
	old := time.Now().Add(-40 * 24 * time.Hour)
	s.db.Unscoped().Model(&model.Image{}).Where("key = ?", key).Update("deleted_at", old)
	n, err := s.PurgeExpiredTrash(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("应清理 1 张过期软删, got %d", n)
	}
	_ = uid
}

// TestPurgeOneIdempotentNoDoubleRefund 是 CRITICAL 回归测试：同一张已软删图被并发/重试
// 两次 purgeOne（例如手动 PurgePermanent 与 PurgeExpiredTrash 扫到同一张图赛跑），
// 第二次必须是 no-op —— 不能重复退配额，也不能重复减 ref_count 误删仍被其它 live 图
// 引用的共享物理文件。
// TestPurgeDoesNotGoNegativeUsedStorage 陈旧 used_storage 小于 file.Size 时退配额夹到 0。
func TestPurgeDoesNotGoNegativeUsedStorage(t *testing.T) {
	s, uid, key, _ := setupPurge(t)
	// 人为把 used 压到小于 file.Size
	s.db.Model(&model.User{}).Where("id = ?", uid).Update("used_storage", 100)
	if err := s.PurgePermanent(uid, key); err != nil {
		t.Fatal(err)
	}
	var u model.User
	s.db.First(&u, uid)
	if u.UsedStorage != 0 {
		t.Fatalf("used_storage 应夹到 0, got %d", u.UsedStorage)
	}
}

func TestPurgeOneIdempotentNoDoubleRefund(t *testing.T) {
	s, uid, key, fileID := setupPurge(t) // user UsedStorage=500, file Size=500 ref_count=1, one soft-deleted image
	// 模拟共享文件：ref_count=2（另有一张 live 图引用同 file）
	s.db.Model(&model.File{}).Where("id = ?", fileID).Update("ref_count", 2)
	var img model.Image
	s.db.Unscoped().Where("key = ?", key).First(&img)
	purge := func() *physicalDelete {
		var pd *physicalDelete
		s.db.Transaction(func(tx *gorm.DB) error { var e error; pd, e = s.purgeOne(tx, &img); return e })
		return pd
	}
	pd1 := purge() // 真正清理
	pd2 := purge() // 用同一(陈旧)img 再清理一次——模拟并发/重试
	var u model.User
	s.db.First(&u, uid)
	if u.UsedStorage != 0 {
		t.Fatalf("配额应只退一次到 0(而非双退成负数), got %d", u.UsedStorage)
	}
	var f model.File
	if err := s.db.First(&f, fileID).Error; err != nil {
		t.Fatal("共享文件不应被销毁(仍被 live 图引用)")
	}
	if f.RefCount != 1 {
		t.Fatalf("ref_count 应只减一次 2->1, got %d", f.RefCount)
	}
	_ = pd1
	if pd2 != nil {
		t.Error("第二次(幂等 no-op)不应返回物理删除")
	}
}

// TestPurgePermanentRejectsLiveAndForeign 覆盖 PurgePermanent 的 ErrNotFound 分支：
// 未在回收站(live)与非属主两种情况都不可彻底删除。
func TestPurgePermanentRejectsLiveAndForeign(t *testing.T) {
	s, uid, key, _ := setupPurge(t)
	// 先把这张图恢复成 live（非回收站）
	s.db.Unscoped().Model(&model.Image{}).Where("key = ?", key).Update("deleted_at", nil)
	if err := s.PurgePermanent(uid, key); !errors.Is(err, ErrNotFound) {
		t.Errorf("live(未在回收站)图不可彻底删除, want ErrNotFound, got %v", err)
	}
	// 再软删并用他人身份彻底删除
	s.db.Delete(&model.Image{}, "key = ?", key)
	if err := s.PurgePermanent(uid+999, key); !errors.Is(err, ErrNotFound) {
		t.Errorf("非属主不可彻底删除, want ErrNotFound, got %v", err)
	}
}

// TestPurgeExpiredImages 清理 expires_at 已过的 live 图：硬删、退配额、减 ref；
// 未过期不动；已软删（归 PurgeExpiredTrash）本函数不处理。
func TestPurgeExpiredImages(t *testing.T) {
	db := model.TestDB(t)
	u := &model.User{Username: "pe", Email: "pe@img.li", GroupID: 1, UsedStorage: 1500}
	db.Create(u)
	// 过期 live 图（独占 file）
	fExp := &model.File{Hash: "pe-exp", StoragePolicyID: 1, Path: "p/exp", Size: 500, MIME: "image/png", RefCount: 1}
	db.Create(fExp)
	past := time.Now().Add(-time.Hour)
	imgExp := &model.Image{
		Key: "peexpkey00001", UserID: &u.ID, FileID: fExp.ID,
		Name: "exp", Ext: "png", Visibility: "public", Status: "normal",
		ExpiresAt: &past,
	}
	db.Create(imgExp)
	// 未过期 live 图
	fOk := &model.File{Hash: "pe-ok", StoragePolicyID: 1, Path: "p/ok", Size: 500, MIME: "image/png", RefCount: 1}
	db.Create(fOk)
	future := time.Now().Add(24 * time.Hour)
	imgOk := &model.Image{
		Key: "peokkey000001", UserID: &u.ID, FileID: fOk.ID,
		Name: "ok", Ext: "png", Visibility: "public", Status: "normal",
		ExpiresAt: &future,
	}
	db.Create(imgOk)
	// 已软删过期图（deleted_at 非空，本函数应跳过）
	fTrash := &model.File{Hash: "pe-tr", StoragePolicyID: 1, Path: "p/tr", Size: 500, MIME: "image/png", RefCount: 1}
	db.Create(fTrash)
	imgTrash := &model.Image{
		Key: "petrashkey001", UserID: &u.ID, FileID: fTrash.ID,
		Name: "tr", Ext: "png", Visibility: "public", Status: "normal",
		ExpiresAt: &past,
	}
	db.Create(imgTrash)
	db.Delete(imgTrash)

	// 共享 file 的过期图：ref_count=2，清理后 file 应保留
	fShare := &model.File{Hash: "pe-sh", StoragePolicyID: 1, Path: "p/sh", Size: 500, MIME: "image/png", RefCount: 2}
	db.Create(fShare)
	imgShare := &model.Image{
		Key: "pesharekey001", UserID: &u.ID, FileID: fShare.ID,
		Name: "sh", Ext: "png", Visibility: "public", Status: "normal",
		ExpiresAt: &past,
	}
	db.Create(imgShare)
	// 另一张 live 共享引用（不过期），保证物理文件不删
	imgShareLive := &model.Image{
		Key: "pesharelive01", UserID: &u.ID, FileID: fShare.ID,
		Name: "sh2", Ext: "png", Visibility: "public", Status: "normal",
	}
	db.Create(imgShareLive)

	s := New(db, storagesvc.New(&config.Config{DataDir: t.TempDir()}, db), nil)
	n, err := s.PurgeExpiredImages(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// imgExp + imgShare = 2；imgOk 未到期、imgTrash 已软删
	if n != 2 {
		t.Errorf("应清理 2 张过期 live 图, got %d", n)
	}

	var cnt int64
	db.Unscoped().Model(&model.Image{}).Where("key = ?", imgExp.Key).Count(&cnt)
	if cnt != 0 {
		t.Error("过期 live 图应被硬删")
	}
	db.Model(&model.Image{}).Where("key = ?", imgOk.Key).Count(&cnt)
	if cnt != 1 {
		t.Error("未过期图应保留")
	}
	db.Unscoped().Model(&model.Image{}).Where("key = ? AND deleted_at IS NOT NULL", imgTrash.Key).Count(&cnt)
	if cnt != 1 {
		t.Error("已软删图应仍由 PurgeExpiredTrash 处理, 本函数不碰")
	}

	var user model.User
	db.First(&user, u.ID)
	// 退两张 500 配额: 1500-500-500=500（trash 未退、ok 未退、shareLive 未退）
	if user.UsedStorage != 500 {
		t.Errorf("used_storage 应退至 500, got %d", user.UsedStorage)
	}

	// 独占 file 归零被删
	db.Model(&model.File{}).Where("id = ?", fExp.ID).Count(&cnt)
	if cnt != 0 {
		t.Error("独占 file ref 归零应删 file 行")
	}
	// 共享 file 保留 ref=1
	var f model.File
	if err := db.First(&f, fShare.ID).Error; err != nil {
		t.Fatalf("共享 file 应保留: %v", err)
	}
	if f.RefCount != 1 {
		t.Errorf("共享 file ref_count 应为 1, got %d", f.RefCount)
	}
}
