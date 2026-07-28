package adminsvc

import (
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
)

// seedImage 造一条 file+image 记录，policyID 默认取 1（TestDB 已 seed 的默认本地策略）。
func seedImage(t *testing.T, db *gorm.DB, key string, userID *uint64, status string, policyID uint64) *model.Image {
	t.Helper()
	if policyID == 0 {
		policyID = 1
	}
	f := &model.File{Hash: key + "hash", StoragePolicyID: policyID, Path: "p/" + key, Size: 100, MIME: "image/png", RefCount: 1}
	if err := db.Create(f).Error; err != nil {
		t.Fatal(err)
	}
	img := &model.Image{Key: key, UserID: userID, FileID: f.ID, Name: key, Ext: "png", Visibility: "public", Status: status}
	if status == "" {
		img.Status = "normal"
	}
	if err := db.Create(img).Error; err != nil {
		t.Fatal(err)
	}
	return img
}

func TestListImagesFiltersAndTotal(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)

	alice := &model.User{Username: "alice", Email: "alice@x.li", GroupID: 1}
	bob := &model.User{Username: "bob", Email: "bob@x.li", GroupID: 1}
	if err := db.Create(alice).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(bob).Error; err != nil {
		t.Fatal(err)
	}
	pol2 := &model.StoragePolicy{Name: "s3", Driver: "s3"}
	if err := db.Create(pol2).Error; err != nil {
		t.Fatal(err)
	}

	seedImage(t, db, "k1", &alice.ID, "normal", 1)
	seedImage(t, db, "k2", &alice.ID, "pending", 1)
	seedImage(t, db, "k3", &bob.ID, "normal", pol2.ID)
	guestImg := seedImage(t, db, "k4", nil, "normal", 1) // 游客图 user_id nil
	softDeleted := seedImage(t, db, "k5", &alice.ID, "normal", 1)
	if err := db.Delete(softDeleted).Error; err != nil { // 软删，默认 scope 应排除
		t.Fatal(err)
	}

	// 全量：4 条（软删的 k5 被排除）
	rows, total, err := svc.ListImages(0, "", 0, 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || len(rows) != 4 {
		t.Fatalf("全量 total=%d len=%d, want 4/4", total, len(rows))
	}
	// 倒序：最新 (k4) 应在最前
	if rows[0].Img.Key != guestImg.Key {
		t.Errorf("首条 key=%s, want %s（按 id 倒序）", rows[0].Img.Key, guestImg.Key)
	}
	// 游客图 username 应为空
	for _, r := range rows {
		if r.Img.Key == "k4" {
			if r.Username != "" {
				t.Errorf("游客图 username = %q, want 空", r.Username)
			}
			if r.Img.UserID != nil {
				t.Errorf("游客图 UserID 应为 nil")
			}
		}
		if r.Img.Key == "k1" && r.Username != "alice" {
			t.Errorf("k1 username = %q, want alice", r.Username)
		}
	}

	// user 筛选
	rows, total, err = svc.ListImages(alice.ID, "", 0, 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("user=alice total=%d, want 2", total)
	}

	// status 筛选
	rows, total, err = svc.ListImages(0, "pending", 0, 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || rows[0].Img.Key != "k2" {
		t.Errorf("status=pending total=%d rows=%+v", total, rows)
	}

	// policy 筛选
	rows, total, err = svc.ListImages(0, "", pol2.ID, 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || rows[0].Img.Key != "k3" {
		t.Errorf("policy 筛选 total=%d rows=%+v", total, rows)
	}
	if rows[0].Policy.ID != pol2.ID {
		t.Errorf("row.Policy.ID = %d, want %d", rows[0].Policy.ID, pol2.ID)
	}

	// 分页
	rows, total, err = svc.ListImages(0, "", 0, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || len(rows) != 2 {
		t.Errorf("page1 limit2: total=%d len=%d", total, len(rows))
	}
}

func TestAdminSoftDeleteIdempotentNotFound(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	u := &model.User{Username: "alice", Email: "alice@x.li", GroupID: 1}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	seedImage(t, db, "k1", &u.ID, "normal", 1)

	img, err := svc.AdminSoftDelete("k1")
	if err != nil {
		t.Fatal(err)
	}
	if img.Key != "k1" {
		t.Errorf("返回 image key = %s, want k1", img.Key)
	}
	var got model.Image
	if err := db.Unscoped().Where("key = ?", "k1").First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if !got.DeletedAt.Valid {
		t.Errorf("软删后 DeletedAt 应有效")
	}

	// 再次软删（已软删）→ ErrImageNotFound（管理面幂等裁决：404 可接受）
	if _, err := svc.AdminSoftDelete("k1"); !errors.Is(err, ErrImageNotFound) {
		t.Errorf("重复软删 err = %v, want ErrImageNotFound", err)
	}

	// 不存在的 key
	if _, err := svc.AdminSoftDelete("nope"); !errors.Is(err, ErrImageNotFound) {
		t.Errorf("不存在 key err = %v, want ErrImageNotFound", err)
	}
}

func TestSetWhitelistRepositionsStatus(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	u := &model.User{Username: "alice", Email: "alice@x.li", GroupID: 1}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	seedImage(t, db, "k1", &u.ID, "pending", 1)

	img, err := svc.SetWhitelist("k1", true)
	if err != nil {
		t.Fatal(err)
	}
	if !img.IsWhitelisted {
		t.Errorf("IsWhitelisted = false, want true")
	}
	if img.Status != "normal" {
		t.Errorf("status = %s, want normal（加白复位）", img.Status)
	}
	var got model.Image
	db.First(&got, "key = ?", "k1")
	if got.Status != "normal" || !got.IsWhitelisted {
		t.Errorf("db 中 status=%s is_whitelisted=%v", got.Status, got.IsWhitelisted)
	}

	// on=false 不应改变 status
	seedImage(t, db, "k2", &u.ID, "rejected", 1)
	img2, err := svc.SetWhitelist("k2", false)
	if err != nil {
		t.Fatal(err)
	}
	if img2.IsWhitelisted {
		t.Errorf("k2 IsWhitelisted = true, want false")
	}
	if img2.Status != "rejected" {
		t.Errorf("k2 status = %s, want 保持 rejected", img2.Status)
	}

	// 不存在
	if _, err := svc.SetWhitelist("nope", true); !errors.Is(err, ErrImageNotFound) {
		t.Errorf("不存在 key err = %v, want ErrImageNotFound", err)
	}
}

// TestAdminSoftDeleteRowsAffectedGateNoFakeSuccess 直接验证 AdminSoftDelete 的软删语句
// 对已软删行 0 行受影响：单条 `Where(...).Delete(...)` 语句天然消除了旧版
// First→Delete 两步之间的竞态窗口（并发双删场景下输者不会再误报成功）。
func TestAdminSoftDeleteRowsAffectedGateNoFakeSuccess(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	u := &model.User{Username: "alice", Email: "alice@x.li", GroupID: 1}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	seedImage(t, db, "gatekey00001", &u.ID, "normal", 1)

	if _, err := svc.AdminSoftDelete("gatekey00001"); err != nil {
		t.Fatal(err)
	}
	// 重放 AdminSoftDelete 内部所用的同一条语句：对已软删行必须 0 行受影响。
	res := db.Where("key = ?", "gatekey00001").Delete(&model.Image{})
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	if res.RowsAffected != 0 {
		t.Fatalf("门禁失效：对已软删行 Delete 应 0 行受影响, got %d", res.RowsAffected)
	}
	// 端到端：写 0 行必须报 ErrImageNotFound，不得假成功。
	if _, err := svc.AdminSoftDelete("gatekey00001"); !errors.Is(err, ErrImageNotFound) {
		t.Errorf("重复软删 err = %v, want ErrImageNotFound", err)
	}
}

// TestSetWhitelistUpdateStepRowsAffectedGate 验证 SetWhitelist 写步骤（Update 语句
// `id = ? AND deleted_at IS NULL`）对已软删行的门禁：0 行受影响。真实竞态窗口
// （First 读到活行之后、Update 写入之前该行被并发软删）无法在单元测试内确定性注入，
// 故退而验证门禁本身——写 0 行必须报 ErrImageNotFound，不得用 First 阶段读到的
// 陈旧数据冒充成功。
func TestSetWhitelistUpdateStepRowsAffectedGate(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	u := &model.User{Username: "alice", Email: "alice@x.li", GroupID: 1}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	img := seedImage(t, db, "gatekey00002", &u.ID, "pending", 1)

	// 模拟“First 读到活行之后、写入之前被并发软删”：直接软删该行，
	// 再重放 SetWhitelist 写步骤所用的同一 WHERE 子句，断言 0 行受影响。
	if err := db.Delete(&model.Image{}, "id = ?", img.ID).Error; err != nil {
		t.Fatal(err)
	}
	res := db.Model(&model.Image{}).Where("id = ? AND deleted_at IS NULL", img.ID).
		Updates(map[string]any{"is_whitelisted": true})
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	if res.RowsAffected != 0 {
		t.Fatalf("门禁失效：对已软删行 Update 应 0 行受影响, got %d", res.RowsAffected)
	}

	// 端到端：对已软删 key 调用 SetWhitelist 必须报 ErrImageNotFound，不得假成功
	// （无论是被 First 的默认 scope 挡下，还是被写步骤的 RowsAffected 门禁挡下）。
	if _, err := svc.SetWhitelist("gatekey00002", true); !errors.Is(err, ErrImageNotFound) {
		t.Errorf("已软删行加白 err = %v, want ErrImageNotFound", err)
	}
}
