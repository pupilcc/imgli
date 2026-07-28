package model_test

import (
	"testing"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
)

// TestMigrateCreatesAllDeclaredIndexes 首跑 Migrate 后,全部 schema 声明索引必须存在。
// 护栏:glebarez CreateConstraint 重建表丢二级索引(spec 2026-07-26-imgli-fresh-sqlite-
// indexes-design.md);按 AllModels() 动态枚举,将来加模型/加索引自动纳入,不写死清单。
func TestMigrateCreatesAllDeclaredIndexes(t *testing.T) {
	db := model.TestDB(t)
	m := db.Migrator()
	for _, mod := range model.AllModels() {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(mod); err != nil {
			t.Fatalf("解析模型 %T: %v", mod, err)
		}
		for _, idx := range stmt.Schema.ParseIndexes() {
			if !m.HasIndex(mod, idx.Name) {
				t.Errorf("缺失索引 %s.%s", stmt.Table, idx.Name)
			}
		}
	}
}

// TestFreshDBUniqueConstraintsEnforced 行为锁语义:唯一索引不仅存在,且真在拒重复
// (防「索引在但非 unique」变体)。取 users.username 与 images.key 两个代表。
func TestFreshDBUniqueConstraintsEnforced(t *testing.T) {
	db := model.TestDB(t)

	u1 := &model.User{Username: "dup", Email: "a@x.io", GroupID: 1}
	if err := db.Create(u1).Error; err != nil {
		t.Fatal(err)
	}
	// 对照:不同值可插
	if err := db.Create(&model.User{Username: "other", Email: "b@x.io", GroupID: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.User{Username: "dup", Email: "c@x.io", GroupID: 1}).Error; err == nil {
		t.Error("同 username 应被唯一索引拒绝")
	}

	f := &model.File{Hash: "hh", Surface: model.SurfacePublic, StoragePolicyID: 1, Path: "public/p", Size: 1, RefCount: 3}
	if err := db.Create(f).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Image{Key: "k123", UserID: &u1.ID, FileID: f.ID, Name: "a", Ext: "png"}).Error; err != nil {
		t.Fatal(err)
	}
	// 对照:不同 key 可插
	if err := db.Create(&model.Image{Key: "k456", UserID: &u1.ID, FileID: f.ID, Name: "b", Ext: "png"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Image{Key: "k123", UserID: &u1.ID, FileID: f.ID, Name: "c", Ext: "png"}).Error; err == nil {
		t.Error("同 images.key 应被唯一索引拒绝")
	}
}
