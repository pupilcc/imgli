package imagesvc

import (
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/config"
	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/storagesvc"
)

func TestPreviewAndApplyLifecycle(t *testing.T) {
	db := model.TestDB(t)
	s := New(db, storagesvc.New(&config.Config{DataDir: t.TempDir()}, db), nil)

	var g model.UserGroup
	if err := db.Where("is_default = ?", true).First(&g).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&g).Updates(map[string]any{
		"max_expires_in": 86400, "force_max_age_days": 0,
	}).Error; err != nil {
		t.Fatal(err)
	}

	u := model.User{Username: "lcuser", Email: "lc@test", GroupID: g.ID, PasswordHash: "x"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	f := model.File{Hash: "lchash", Surface: "public", StoragePolicyID: 1, Path: "p", Size: 1, MIME: "image/png", RefCount: 2}
	if err := db.Create(&f).Error; err != nil {
		t.Fatal(err)
	}
	perm := model.Image{Key: "lckeyperm0001", UserID: &u.ID, FileID: f.ID, Name: "p.png", Ext: "png", Visibility: "public", Status: "normal"}
	far := time.Now().Add(30 * 24 * time.Hour)
	over := model.Image{Key: "lckeyover0001", UserID: &u.ID, FileID: f.ID, Name: "o.png", Ext: "png", Visibility: "public", Status: "normal", ExpiresAt: &far}
	if err := db.Create(&perm).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&over).Error; err != nil {
		t.Fatal(err)
	}

	prev, err := s.PreviewApplyLifecycle(g.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if prev.CapSec != 86400 {
		t.Fatalf("cap=%d", prev.CapSec)
	}
	if prev.Permanent < 1 || prev.OverCap < 1 || prev.Total < 2 {
		t.Fatalf("preview %+v", prev)
	}

	res, err := s.ApplyLifecycle(g.ID, true, 100)
	if err != nil {
		t.Fatal(err)
	}
	if res.Updated < 2 {
		t.Fatalf("updated=%d want >=2", res.Updated)
	}
	var got model.Image
	if err := db.Where("key = ?", "lckeyperm0001").First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.ExpiresAt == nil {
		t.Fatal("permanent should be clamped")
	}
}
