package adminsvc

import (
	"errors"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
)

func TestListReviewOnlyPending(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)

	u := &model.User{Username: "alice", Email: "alice@x.li", GroupID: 1}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	seedImage(t, db, "k1", &u.ID, "normal", 1)
	seedImage(t, db, "k2", &u.ID, "pending", 1)
	seedImage(t, db, "k3", &u.ID, "rejected", 1)
	seedImage(t, db, "k4", &u.ID, "pending", 1)

	rows, total, err := svc.ListReview(1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(rows) != 2 {
		t.Fatalf("total=%d len=%d, want 2/2", total, len(rows))
	}
	// id 倒序：最新的 k4 排前
	if rows[0].Img.Key != "k4" || rows[1].Img.Key != "k2" {
		t.Errorf("顺序 = [%s,%s], want [k4,k2]", rows[0].Img.Key, rows[1].Img.Key)
	}
	for _, r := range rows {
		if r.Img.Status != "pending" {
			t.Errorf("行 status=%s, want pending", r.Img.Status)
		}
	}
}

func TestDecideApproveAndReject(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	u := &model.User{Username: "alice", Email: "alice@x.li", GroupID: 1}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	seedImage(t, db, "k1", &u.ID, "pending", 1)
	seedImage(t, db, "k2", &u.ID, "pending", 1)

	img, err := svc.Decide("k1", "approve")
	if err != nil {
		t.Fatal(err)
	}
	if img.Status != "normal" {
		t.Errorf("approve 后 status=%s, want normal", img.Status)
	}
	var got model.Image
	db.First(&got, "key = ?", "k1")
	if got.Status != "normal" {
		t.Errorf("db status=%s, want normal", got.Status)
	}

	img2, err := svc.Decide("k2", "reject")
	if err != nil {
		t.Fatal(err)
	}
	if img2.Status != "rejected" {
		t.Errorf("reject 后 status=%s, want rejected", img2.Status)
	}
}

func TestDecideInvalidAction(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	u := &model.User{Username: "alice", Email: "alice@x.li", GroupID: 1}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	seedImage(t, db, "k1", &u.ID, "pending", 1)

	if _, err := svc.Decide("k1", "delete"); !errors.Is(err, ErrInvalidAction) {
		t.Errorf("非法 action err = %v, want ErrInvalidAction", err)
	}
	// 非法 action 不应改动 status（校验先于写）。
	var got model.Image
	db.First(&got, "key = ?", "k1")
	if got.Status != "pending" {
		t.Errorf("非法 action 后 status=%s, want 保持 pending", got.Status)
	}
}

// TestDecideNotFoundVsNotPending 验证 0 行受影响时的错因区分：图不存在 → ErrImageNotFound；
// 图存在但非 pending → ErrNotPending（400，先写后查补定错因，不改变写句本身的权威性）。
func TestDecideNotFoundVsNotPending(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	u := &model.User{Username: "alice", Email: "alice@x.li", GroupID: 1}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	seedImage(t, db, "k1", &u.ID, "normal", 1) // 已是 normal，非 pending

	if _, err := svc.Decide("k1", "approve"); !errors.Is(err, ErrNotPending) {
		t.Errorf("非 pending err = %v, want ErrNotPending", err)
	}
	if _, err := svc.Decide("nope", "approve"); !errors.Is(err, ErrImageNotFound) {
		t.Errorf("不存在 key err = %v, want ErrImageNotFound", err)
	}

	// 软删的图也应按不存在处理（默认 scope 排除）。
	seedImage(t, db, "k2", &u.ID, "pending", 1)
	var img model.Image
	db.First(&img, "key = ?", "k2")
	db.Delete(&img)
	if _, err := svc.Decide("k2", "approve"); !errors.Is(err, ErrImageNotFound) {
		t.Errorf("软删 key err = %v, want ErrImageNotFound", err)
	}
}

// TestDecideRowsAffectedGateReplayNoFakeSuccess 直接重放 Decide 内部所用的写语句：
// 已裁决过（非 pending）的行再次匹配该 WHERE 子句应得到 0 行受影响，验证写门禁本身
// （而非仅验证 Decide 的错误分支包装）拒绝了重复裁决——同 images_test.go 的重放测试风格。
func TestDecideRowsAffectedGateReplayNoFakeSuccess(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	u := &model.User{Username: "alice", Email: "alice@x.li", GroupID: 1}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	seedImage(t, db, "gatekey0001", &u.ID, "pending", 1)

	if _, err := svc.Decide("gatekey0001", "approve"); err != nil {
		t.Fatal(err)
	}
	// 重放 Decide 内部所用的同一条语句：对已非 pending 的行必须 0 行受影响。
	res := db.Model(&model.Image{}).
		Where("key = ? AND status = ? AND deleted_at IS NULL", "gatekey0001", "pending").
		Update("status", "normal")
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	if res.RowsAffected != 0 {
		t.Errorf("重放 RowsAffected = %d, want 0（防止双重裁决覆盖）", res.RowsAffected)
	}
}

// TestDecideRefetchSucceedsWhenConcurrentlySoftDeletedAfterUpdate 模拟 Decide 内部时序：
// UPDATE 已提交（RowsAffected>=1，裁决已生效）之后、Decide 尚未返回之前，该图被
// 另一并发请求软删（如 AdminSoftDelete）。回归验证直接调用生产函数
// (*Service).decideRefetch（而非重放一份平行 SQL）：它必须 Unscoped 取回该行，
// 而不是被默认 scope（排除 deleted_at 非空）挡住——否则一次已经生效的裁决会被
// 误报成内部错误、且漏掉 audit（handler 只在 err==nil 分支落审计）。
// 把 decideRefetch 内部的 Unscoped() 退回普通 Where 时，本测试必须变红——已手工
// 验证（临时改回 s.db.Where(...) 跑本测试，得到 "want status=normal, got: record
// not found"；改回 Unscoped 后复测转绿），过程见 task-8-report.md。
func TestDecideRefetchSucceedsWhenConcurrentlySoftDeletedAfterUpdate(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	u := &model.User{Username: "alice", Email: "alice@x.li", GroupID: 1}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	seedImage(t, db, "racekey0001", &u.ID, "pending", 1)

	// 手动重放 Decide 的第一条语句（状态更新），模拟"更新已提交"这一时刻。
	res := db.Model(&model.Image{}).
		Where("key = ? AND status = ? AND deleted_at IS NULL", "racekey0001", "pending").
		Update("status", "normal")
	if res.Error != nil || res.RowsAffected != 1 {
		t.Fatalf("模拟更新失败: err=%v rows=%d", res.Error, res.RowsAffected)
	}
	// 模拟并发软删发生在"更新已提交"与"Decide 内部补查"之间。
	var img model.Image
	if err := db.Where("key = ?", "racekey0001").First(&img).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&img).Error; err != nil {
		t.Fatal(err)
	}

	// 默认 scope 此时确实查不到该行（证明若 decideRefetch 不加 Unscoped 会误报失败）。
	var scoped model.Image
	if err := db.Where("key = ?", "racekey0001").First(&scoped).Error; err == nil {
		t.Fatalf("默认 scope 不应查到已软删行")
	}

	// 咬合生产函数本身：decideRefetch 必须仍能取回该行且状态已生效为 normal。
	got, err := svc.decideRefetch("racekey0001")
	if err != nil {
		t.Fatalf("decideRefetch 应能取回该行: %v", err)
	}
	if got.Status != "normal" {
		t.Errorf("status = %s, want normal（更新已生效，不应因后续软删而丢失）", got.Status)
	}
}

func TestNSFWScoresAndImagesByKeys(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	f := &model.File{Hash: "hs", StoragePolicyID: 1, Path: "ps", Size: 1, RefCount: 1}
	db.Create(f)
	sc := 0.42
	db.Create(&model.Image{Key: "s1", FileID: f.ID, Name: "a.png", Ext: "png", Status: "pending", NSFWScore: &sc})
	db.Create(&model.Image{Key: "s2", FileID: f.ID, Name: "b.png", Ext: "png", Status: "pending"})
	scores, err := svc.NSFWScoresByKeys([]string{"s1", "s2", "nope"})
	if err != nil {
		t.Fatal(err)
	}
	if scores["s1"] == nil || *scores["s1"] != 0.42 {
		t.Fatalf("s1 score=%v", scores["s1"])
	}
	if _, ok := scores["s2"]; !ok {
		t.Fatal("s2 should be present")
	}
	if _, ok := scores["nope"]; ok {
		t.Fatal("nope should be absent")
	}
	imgs, err := svc.ImagesByKeys([]string{"s1", "s2"})
	if err != nil || len(imgs) != 2 {
		t.Fatalf("imgs=%d err=%v", len(imgs), err)
	}
}

func TestDecideBatchPartialSuccess(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	u := &model.User{Username: "alice", Email: "alice@x.li", GroupID: 1}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	seedImage(t, db, "k1", &u.ID, "pending", 1)
	seedImage(t, db, "k2", &u.ID, "pending", 1)
	seedImage(t, db, "k3", &u.ID, "normal", 1) // 非 pending，会失败

	results, err := svc.DecideBatch([]string{"k1", "k2", "k3", "nope"}, "approve")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 {
		t.Fatalf("results 长度 = %d, want 4", len(results))
	}
	want := map[string]bool{"k1": true, "k2": true, "k3": false, "nope": false}
	for _, r := range results {
		if r.OK != want[r.Key] {
			t.Errorf("key=%s ok=%v, want %v", r.Key, r.OK, want[r.Key])
		}
		if !r.OK && r.Error == "" {
			t.Errorf("key=%s 失败但 Error 为空", r.Key)
		}
		if r.OK && r.Error != "" {
			t.Errorf("key=%s 成功但 Error 非空: %s", r.Key, r.Error)
		}
	}
	var k1, k2 model.Image
	db.First(&k1, "key = ?", "k1")
	db.First(&k2, "key = ?", "k2")
	if k1.Status != "normal" || k2.Status != "normal" {
		t.Errorf("k1/k2 status = %s/%s, want normal/normal", k1.Status, k2.Status)
	}
}

func TestDecideBatchTooManyKeys(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	keys := make([]string, 101)
	for i := range keys {
		keys[i] = "k"
	}
	if _, err := svc.DecideBatch(keys, "approve"); !errors.Is(err, ErrTooManyKeys) {
		t.Errorf("err = %v, want ErrTooManyKeys", err)
	}
}

func TestDecideBatchInvalidAction(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	if _, err := svc.DecideBatch([]string{"k1"}, "delete"); !errors.Is(err, ErrInvalidAction) {
		t.Errorf("err = %v, want ErrInvalidAction", err)
	}
}

func TestModerationTriggersByKeysNewAndLegacy(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	// 新格式 results
	db.Create(&model.AuditLog{
		ActorType: "system", Action: "moderation_flag",
		Detail: `{"image_id":1,"key":"ka","action":"pending","score":0.82,"results":[{"plugin":"nsfwjs","severity":"review","score":0.82}]}`,
	})
	// 旧格式仅 score
	db.Create(&model.AuditLog{
		ActorType: "system", Action: "moderation_flag",
		Detail: `{"image_id":2,"key":"kb","action":"pending","score":0.9}`,
	})
	// 更新的一条应覆盖 ka 的旧条
	db.Create(&model.AuditLog{
		ActorType: "system", Action: "moderation_flag",
		Detail: `{"image_id":1,"key":"ka","action":"pending","score":0.91,"results":[{"plugin":"nsfwjs","severity":"review","score":0.91},{"plugin":"ocr","severity":"review","hits":["x"]}]}`,
	})

	m, err := svc.ModerationTriggersByKeys([]string{"ka", "kb", "kc", "ka"})
	if err != nil {
		t.Fatal(err)
	}
	if len(m["ka"]) != 2 {
		t.Fatalf("ka triggers=%+v, want 2 (latest audit)", m["ka"])
	}
	if m["ka"][0].Plugin != "nsfwjs" || m["ka"][0].Score == nil || *m["ka"][0].Score != 0.91 {
		t.Errorf("ka[0]=%+v", m["ka"][0])
	}
	if m["ka"][1].Plugin != "ocr" || len(m["ka"][1].Hits) != 1 {
		t.Errorf("ka[1]=%+v", m["ka"][1])
	}
	if len(m["kb"]) != 1 || m["kb"][0].Plugin != "legacy" || m["kb"][0].Score == nil || *m["kb"][0].Score != 0.9 {
		t.Errorf("kb=%+v", m["kb"])
	}
	if _, ok := m["kc"]; ok {
		t.Errorf("kc should be absent")
	}
}

func TestParseModerationFlagTriggersEmpty(t *testing.T) {
	if parseModerationFlagTriggers("") != nil {
		t.Fatal("empty")
	}
	if parseModerationFlagTriggers(`{`) != nil {
		t.Fatal("invalid json")
	}
}
