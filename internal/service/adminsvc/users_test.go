package adminsvc

import (
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/auth"
)

func TestUpdateUserGuards(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	admin := &model.User{Username: "boss", Email: "b@x", GroupID: 1, IsAdmin: true}
	db.Create(admin)
	banned := "banned"
	if _, err := svc.UpdateUser(admin.ID, admin.ID, nil, &banned); err != ErrSelfBan {
		t.Errorf("自封禁 err = %v, want ErrSelfBan", err)
	}
	pleb := &model.User{Username: "p", Email: "p@x", GroupID: 1}
	db.Create(pleb)
	if _, err := svc.UpdateUser(admin.ID, pleb.ID, nil, &banned); err != nil {
		t.Fatal(err)
	}
	var got model.User
	db.First(&got, pleb.ID)
	if got.Status != "banned" {
		t.Errorf("status = %s", got.Status)
	}
	bogus := "flying"
	if _, err := svc.UpdateUser(admin.ID, pleb.ID, nil, &bogus); err != ErrInvalidStatus {
		t.Errorf("非法 status err = %v", err)
	}
}

func TestUpdateUserNotFound(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	admin := &model.User{Username: "boss", Email: "b@x", GroupID: 1, IsAdmin: true}
	db.Create(admin)

	if _, err := svc.UpdateUser(admin.ID, 999999, nil, nil); err != ErrUserNotFound {
		t.Errorf("不存在用户 err = %v, want ErrUserNotFound", err)
	}

	pleb := &model.User{Username: "p", Email: "p@x", GroupID: 1}
	db.Create(pleb)
	bogusGroup := uint64(999999)
	if _, err := svc.UpdateUser(admin.ID, pleb.ID, &bogusGroup, nil); err != ErrGroupNotFound {
		t.Errorf("不存在组 err = %v, want ErrGroupNotFound", err)
	}
}

func TestUpdateUserChangeGroup(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	admin := &model.User{Username: "boss", Email: "b@x", GroupID: 1, IsAdmin: true}
	db.Create(admin)
	pleb := &model.User{Username: "p", Email: "p@x", GroupID: 1}
	db.Create(pleb)
	newGroup := &model.UserGroup{Name: "vip", StorageQuota: 1 << 30}
	db.Create(newGroup)

	u, err := svc.UpdateUser(admin.ID, pleb.ID, &newGroup.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if u.GroupID != newGroup.ID {
		t.Errorf("GroupID = %d, want %d", u.GroupID, newGroup.ID)
	}
	var got model.User
	db.First(&got, pleb.ID)
	if got.GroupID != newGroup.ID {
		t.Errorf("db GroupID = %d, want %d", got.GroupID, newGroup.ID)
	}
}

func TestListUsersFilterAndPaginate(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	vip := &model.UserGroup{Name: "vip", StorageQuota: 1 << 30}
	db.Create(vip)

	alice := &model.User{Username: "alice", Email: "alice@x.li", GroupID: 1, Status: "active"}
	bob := &model.User{Username: "bob", Email: "bob@x.li", GroupID: 1, Status: "banned"}
	carol := &model.User{Username: "carol", Email: "carol@vip.li", GroupID: vip.ID, Status: "active"}
	for _, u := range []*model.User{alice, bob, carol} {
		if err := db.Create(u).Error; err != nil {
			t.Fatal(err)
		}
	}
	f := &model.File{Hash: "h1", StoragePolicyID: 1, Path: "p", Size: 10, RefCount: 1}
	db.Create(f)
	db.Create(&model.Image{Key: "k1", UserID: &alice.ID, FileID: f.ID, Name: "a.png", Ext: "png"})
	db.Create(&model.Image{Key: "k2", UserID: &alice.ID, FileID: f.ID, Name: "b.png", Ext: "png"})
	deleted := &model.Image{Key: "k3", UserID: &alice.ID, FileID: f.ID, Name: "c.png", Ext: "png"}
	db.Create(deleted)
	db.Delete(deleted) // 软删，不应计入 image_count

	// q 搜索 username
	rows, total, err := svc.ListUsers("ali", 0, "", "", "", 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 || rows[0].User.Username != "alice" {
		t.Errorf("q=ali: total=%d rows=%+v", total, rows)
	}
	if rows[0].ImageCount != 2 {
		t.Errorf("alice image_count = %d, want 2（软删不计）", rows[0].ImageCount)
	}

	// q 搜索 email
	_, total, err = svc.ListUsers("vip.li", 0, "", "", "", 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("q=vip.li total=%d, want 1", total)
	}

	// group 筛选
	rows, total, err = svc.ListUsers("", vip.ID, "", "", "", 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || rows[0].User.Username != "carol" {
		t.Errorf("group filter: total=%d rows=%+v", total, rows)
	}

	// status 筛选
	rows, total, err = svc.ListUsers("", 0, "banned", "", "", 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || rows[0].User.Username != "bob" {
		t.Errorf("status filter: total=%d rows=%+v", total, rows)
	}

	// 分页
	rows, total, err = svc.ListUsers("", 0, "", "", "", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(rows) != 2 {
		t.Errorf("page1 limit2: total=%d len=%d", total, len(rows))
	}
	rows, total, err = svc.ListUsers("", 0, "", "", "", 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(rows) != 1 {
		t.Errorf("page2 limit2: total=%d len=%d", total, len(rows))
	}

	// bandwidth 排序 + last_seen
	if err := db.Model(alice).Updates(map[string]any{
		"bandwidth_used_month": int64(1000), "bandwidth_period": "2026-08",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(bob).Updates(map[string]any{
		"bandwidth_used_month": int64(5000), "bandwidth_period": "2026-08",
	}).Error; err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	if err := db.Create(&model.Session{ID: "s-alice", UserID: alice.ID, CreatedAt: old, ExpiresAt: time.Now().Add(24 * time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Session{ID: "s-bob", UserID: bob.ID, CreatedAt: newer, ExpiresAt: time.Now().Add(24 * time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	rows, _, err = svc.ListUsers("", 0, "", "", "bandwidth", 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 || rows[0].User.Username != "bob" {
		t.Errorf("bandwidth sort first = %+v, want bob", rows)
	}
	var bobRow *UserRow
	for i := range rows {
		if rows[i].User.Username == "bob" {
			bobRow = &rows[i]
			break
		}
	}
	if bobRow == nil || bobRow.LastSeenAt == nil {
		t.Fatalf("bob last_seen missing: %+v", bobRow)
	}
	if bobRow.LastSeenAt.Before(newer.Add(-time.Minute)) {
		t.Errorf("bob last_seen = %v, want ~%v", bobRow.LastSeenAt, newer)
	}
}

func TestResetPassword(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	u := &model.User{Username: "alice", Email: "alice@x.li", GroupID: 1, Status: "active"}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	oldHash := u.PasswordHash
	if err := db.Create(&model.Session{ID: "sess1", UserID: u.ID}).Error; err != nil {
		t.Fatal(err)
	}

	plain, err := svc.ResetPassword(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(plain) < 16 {
		t.Errorf("明文密码长度 = %d, want >=16", len(plain))
	}

	var got model.User
	db.First(&got, u.ID)
	if got.PasswordHash == oldHash || got.PasswordHash == "" {
		t.Errorf("密码哈希未更新")
	}
	if !auth.VerifyPassword(got.PasswordHash, plain) {
		t.Errorf("新密码哈希校验失败")
	}

	var n int64
	db.Model(&model.Session{}).Where("user_id = ?", u.ID).Count(&n)
	if n != 0 {
		t.Errorf("重置密码应删除全部 session，剩余 %d", n)
	}
}

func TestResetPasswordUserNotFound(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	if _, err := svc.ResetPassword(999999); err != ErrUserNotFound {
		t.Errorf("err = %v, want ErrUserNotFound", err)
	}
}
