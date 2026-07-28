package model

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func freshSQLite(t *testing.T) *gorm.DB {
	dsn := "file:" + t.TempDir() + "/b.db?_pragma=foreign_keys(1)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, e := db.DB(); e == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	return db
}

// 全新 SQLite:Migrate 后 FK 生效——插引用不存在 file 的 image 被拒。
func TestFreshSQLiteEnforcesFK(t *testing.T) {
	db := freshSQLite(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// user_id 用真实 user 或 nil(游客);此处造一个 file 引用不存在的策略应被拒
	err := db.Create(&File{ID: 1, Hash: "h1", StoragePolicyID: 999, Path: "p"}).Error
	if err == nil {
		t.Fatal("引用不存在策略的 file 应被 FK 拒绝")
	}
	// 合法链路:建策略→file→image 成功
	if err := db.Create(&StoragePolicy{ID: 1, Name: "p", Driver: "local"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&File{ID: 2, Hash: "h2", StoragePolicyID: 1, Path: "p"}).Error; err != nil {
		t.Fatalf("合法 file 应成功: %v", err)
	}
	if err := db.Create(&Image{ID: 1, Key: "k", FileID: 2}).Error; err != nil {
		t.Fatalf("合法 image 应成功: %v", err)
	}
	// 在用策略/文件:删 file(有 image 引用)应被 RESTRICT 拒
	if err := db.Delete(&File{}, "id = ?", 2).Error; err == nil {
		t.Fatal("被 image 引用的 file 应被 RESTRICT 拒绝删除")
	}
}

// 存量 SQLite:老库(无 FK、有孤儿)升级 Migrate 不报错、不重建、数据不丢。
func TestExistingSQLiteUpgradeSafe(t *testing.T) {
	dsn := "file:" + t.TempDir() + "/b.db?_pragma=foreign_keys(1)"
	oldCfg := &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true}

	// 用不带约束禁用的老配置先建"老库",造孤儿数据
	old, err := gorm.Open(sqlite.Open(dsn), oldCfg)
	if err != nil {
		t.Fatal(err)
	}
	if e := old.AutoMigrate(AllModels()...); e != nil {
		t.Fatal(e)
	}
	old.Exec("INSERT INTO files(id,hash,storage_policy_id,path) VALUES (1,'h',999,'p')") // 孤儿:策略 999 不存在
	old.FirstOrCreate(&SchemaVersion{Version: 1})
	sqlDBOld, _ := old.DB()
	sqlDBOld.Close()

	// 重开并 Migrate(模拟升级)——不应报错(存量 SQLite 跳过建约束)
	db, err := gorm.Open(sqlite.Open(dsn), oldCfg)
	if err != nil {
		t.Fatal(err)
	}
	if e := Migrate(db); e != nil {
		t.Fatalf("存量 SQLite 升级 Migrate 不应报错,却=%v", e)
	}
	var cnt int64
	db.Raw("SELECT count(*) FROM files").Scan(&cnt)
	if cnt != 1 {
		t.Fatalf("升级后数据应完好, files=%d", cnt)
	}
	// 验证存量 SQLite 表未被加外键(应跳过重建)
	var ddl string
	db.Raw("SELECT sql FROM sqlite_master WHERE type='table' AND name='files'").Scan(&ddl)
	if strings.Contains(strings.ToUpper(ddl), "FOREIGN KEY") {
		t.Fatalf("存量 SQLite 不应被加外键(应跳过重建), DDL=%s", ddl)
	}
}
