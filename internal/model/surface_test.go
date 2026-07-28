package model_test

import (
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
)

// TestFilesSurfaceUniqueness 全新库:同 hash 跨 surface 两行可共存,同 hash 同 surface 拒绝。
func TestFilesSurfaceUniqueness(t *testing.T) {
	db := model.TestDB(t)

	pub := &model.File{Hash: "h1", Surface: model.SurfacePublic, StoragePolicyID: 1, Path: "public/a.png", Size: 1, RefCount: 1}
	if err := db.Create(pub).Error; err != nil {
		t.Fatalf("建公开 File 失败: %v", err)
	}
	// 同 hash 不同 surface —— 应允许(这是拆秒传的根基)
	priv := &model.File{Hash: "h1", Surface: model.SurfacePrivate, StoragePolicyID: 1, Path: "private/a.png", Size: 1, RefCount: 1}
	if err := db.Create(priv).Error; err != nil {
		t.Fatalf("同 hash 跨 surface 应允许, got: %v", err)
	}
	// 同 hash 同 surface —— 应被唯一索引拒绝
	dup := &model.File{Hash: "h1", Surface: model.SurfacePublic, StoragePolicyID: 1, Path: "public/b.png", Size: 1, RefCount: 1}
	if err := db.Create(dup).Error; err == nil {
		t.Error("同 (hash,surface) 应被唯一索引拒绝")
	}
}

// TestMigrateSurfaceDropsOldHashIndex 模拟存量库:手工重建旧单列唯一索引后,
// 再跑 Migrate(幂等)应删掉它——否则 unique(hash) 阻止同 hash 跨 surface。
func TestMigrateSurfaceDropsOldHashIndex(t *testing.T) {
	db := model.TestDB(t)
	if err := db.Exec("CREATE UNIQUE INDEX idx_files_hash ON files(hash)").Error; err != nil {
		t.Fatalf("重建旧索引失败: %v", err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatalf("重跑 Migrate 失败: %v", err)
	}
	if db.Migrator().HasIndex(&model.File{}, "idx_files_hash") {
		t.Error("旧 idx_files_hash 应被迁移删除")
	}
	// 删掉旧索引后,同 hash 跨 surface 应可插
	if err := db.Create(&model.File{Hash: "h2", Surface: model.SurfacePublic, StoragePolicyID: 1, Path: "public/x", Size: 1, RefCount: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.File{Hash: "h2", Surface: model.SurfacePrivate, StoragePolicyID: 1, Path: "private/x", Size: 1, RefCount: 1}).Error; err != nil {
		t.Errorf("删旧索引后同 hash 跨 surface 应可插: %v", err)
	}
}

// TestMigrateSurfaceBackfill 迁移应把空 surface 补成 public。
func TestMigrateSurfaceBackfill(t *testing.T) {
	db := model.TestDB(t)
	if err := db.Create(&model.File{Hash: "h3", Surface: model.SurfacePublic, StoragePolicyID: 1, Path: "p", Size: 1, RefCount: 1}).Error; err != nil {
		t.Fatal(err)
	}
	// 制造一行空 surface(绕过默认值)
	if err := db.Exec("UPDATE files SET surface = '' WHERE hash = 'h3'").Error; err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	var f model.File
	db.First(&f, "hash = 'h3'")
	if f.Surface != model.SurfacePublic {
		t.Errorf("空 surface 应回填 public, got %q", f.Surface)
	}
}
