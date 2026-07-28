package imagesvc

import (
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/config"
	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/storagesvc"
)

func TestDeleteUserDataCascade(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db, storagesvc.New(&config.Config{DataDir: t.TempDir()}, db), nil)

	a := &model.User{Username: "del-a", Email: "del-a@img.li", GroupID: 1, UsedStorage: 300}
	b := &model.User{Username: "del-b", Email: "del-b@img.li", GroupID: 1, UsedStorage: 100}
	if err := db.Create(a).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(b).Error; err != nil {
		t.Fatal(err)
	}

	shared := &model.File{Hash: "h-shared", StoragePolicyID: 1, Path: "s/shared", Size: 100, MIME: "image/png", RefCount: 2}
	own := &model.File{Hash: "h-own", StoragePolicyID: 1, Path: "s/own", Size: 200, MIME: "image/png", RefCount: 1}
	if err := db.Create(shared).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(own).Error; err != nil {
		t.Fatal(err)
	}

	imgA := &model.Image{Key: "delkeyaaaaaa", UserID: &a.ID, FileID: shared.ID, Name: "a", Ext: "png", Visibility: "public", Status: "normal"}
	imgB := &model.Image{Key: "delkeybbbbbb", UserID: &b.ID, FileID: shared.ID, Name: "b", Ext: "png", Visibility: "public", Status: "normal"}
	imgTrash := &model.Image{Key: "delkeytrash1", UserID: &a.ID, FileID: own.ID, Name: "t", Ext: "png", Visibility: "public", Status: "normal"}
	if err := db.Create(imgA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(imgB).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(imgTrash).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(imgTrash).Error; err != nil { // 软删
		t.Fatal(err)
	}

	if err := db.Create(&model.Album{UserID: a.ID, Name: "a-alb", Visibility: "private"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.APIToken{UserID: a.ID, Name: "tok", TokenHash: "hash-api-a", Scope: "upload"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.AuthToken{
		UserID: a.ID, Purpose: "verify_email", TokenHash: "hash-auth-a",
		ExpiresAt: time.Now().Add(time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Session{
		ID: "sess-a-hash", UserID: a.ID, ExpiresAt: time.Now().Add(time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteUserData(a.ID); err != nil {
		t.Fatalf("DeleteUserData: %v", err)
	}

	var n int64
	db.Unscoped().Model(&model.Image{}).Where("user_id = ?", a.ID).Count(&n)
	if n != 0 {
		t.Errorf("A 的 images 应 0, got %d", n)
	}
	db.Unscoped().Model(&model.Image{}).Where("user_id = ?", b.ID).Count(&n)
	if n != 1 {
		t.Errorf("B 的 image 应仍在, got %d", n)
	}

	var f model.File
	if err := db.First(&f, shared.ID).Error; err != nil {
		t.Fatalf("共享 file 应仍在: %v", err)
	}
	if f.RefCount != 1 {
		t.Errorf("共享 file ref_count 应 1, got %d", f.RefCount)
	}
	db.Model(&model.File{}).Where("id = ?", own.ID).Count(&n)
	if n != 0 {
		t.Errorf("独占 file 应已删")
	}

	db.Model(&model.Album{}).Where("user_id = ?", a.ID).Count(&n)
	if n != 0 {
		t.Errorf("albums 应 0, got %d", n)
	}
	db.Model(&model.APIToken{}).Where("user_id = ?", a.ID).Count(&n)
	if n != 0 {
		t.Errorf("api_tokens 应 0, got %d", n)
	}
	db.Model(&model.AuthToken{}).Where("user_id = ?", a.ID).Count(&n)
	if n != 0 {
		t.Errorf("auth_tokens 应 0, got %d", n)
	}
	db.Model(&model.Session{}).Where("user_id = ?", a.ID).Count(&n)
	if n != 0 {
		t.Errorf("sessions 应 0, got %d", n)
	}
	db.Model(&model.User{}).Where("id = ?", a.ID).Count(&n)
	if n != 0 {
		t.Errorf("users 应无 A, got %d", n)
	}
	db.Model(&model.User{}).Where("id = ?", b.ID).Count(&n)
	if n != 1 {
		t.Errorf("B 用户行应仍在, got %d", n)
	}
}

func TestDeleteUserDataEmpty(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db, storagesvc.New(&config.Config{DataDir: t.TempDir()}, db), nil)
	u := &model.User{Username: "empty-del", Email: "empty-del@img.li", GroupID: 1}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteUserData(u.ID); err != nil {
		t.Fatalf("空用户注销应成功: %v", err)
	}
	var n int64
	db.Model(&model.User{}).Where("id = ?", u.ID).Count(&n)
	if n != 0 {
		t.Errorf("用户行应删除, got %d", n)
	}
}
