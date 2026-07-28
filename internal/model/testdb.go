package model

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestDB 返回已 Migrate 的测试库。默认 sqlite in-memory；
// 设 IMGLI_TEST_PG_DSN 时在 Postgres 上建随机 schema（测试结束 DROP），
// CI 双方言矩阵靠它复用同一套测试。
func TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gc := &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		TranslateError: true,
		// 与生产 Open() 保持一致:AutoMigrate 不自动建外键,FK 由 Migrate→applyForeignKeys
		// 显式、有条件地创建(否则本 helper 建的全新库会被 AutoMigrate 自动建 FK,
		// 与生产路径分叉、且绕过 applyForeignKeys 的方言/新旧库判断逻辑)。
		DisableForeignKeyConstraintWhenMigrating: true,
	}

	var db *gorm.DB
	var err error
	if dsn := os.Getenv("IMGLI_TEST_PG_DSN"); dsn != "" {
		schema := "t_" + randHex(6)
		admin, aerr := gorm.Open(postgres.Open(dsn), gc)
		if aerr != nil {
			t.Fatal(aerr)
		}
		if sqlDB, derr := admin.DB(); derr == nil {
			t.Cleanup(func() { sqlDB.Close() })
		}
		if aerr = admin.Exec("CREATE SCHEMA " + schema).Error; aerr != nil {
			t.Fatal(aerr)
		}
		t.Cleanup(func() { admin.Exec("DROP SCHEMA " + schema + " CASCADE") })
		db, err = gorm.Open(postgres.Open(dsn+" search_path="+schema), gc)
	} else {
		// 每个测试独立命名的共享内存库，避免连接池拿到不同的 :memory: 实例；
		// _pragma=foreign_keys(1) 与生产 Open() 一致，令 TestDB 强制执行 FK 约束。
		dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=foreign_keys(1)", randHex(8))
		db, err = gorm.Open(sqlite.Open(dsn), gc)
		if err == nil {
			// SQLite 单写者：单连接池消除 shared-cache 锁竞争（database is locked）
			if sqlDB, e := db.DB(); e == nil {
				sqlDB.SetMaxOpenConns(1)
			}
			// 兜底：确保 pragma 在实际使用的连接上生效（与 Open() 的兜底逻辑一致）。
			if err = db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := Seed(db); err != nil {
		t.Fatal(err)
	}
	if sqlDB, derr := db.DB(); derr == nil {
		t.Cleanup(func() { sqlDB.Close() })
	}
	return db
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
