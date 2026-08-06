package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yixian-huang/imgli/internal/config"
)

func TestMigrateAndSeedIdempotent(t *testing.T) {
	db := TestDB(t)
	// TestDB 内部已 Migrate；再跑一次 Seed 验证幂等
	if err := Seed(db); err != nil {
		t.Fatal(err)
	}
	if err := Seed(db); err != nil {
		t.Fatal("Seed 二次执行应幂等:", err)
	}

	var groups []UserGroup
	if err := db.Order("id").Find(&groups).Error; err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("用户组数 = %d, want 2（默认组+游客组）", len(groups))
	}
	if !groups[0].IsDefault || groups[0].StorageQuota != 10<<30 {
		t.Errorf("默认组配置不符: %+v", groups[0])
	}
	if !groups[1].IsGuest || groups[1].MaxFileSize != 5<<20 || groups[1].RatePerDay != 3 {
		t.Errorf("游客组配置不符: %+v", groups[1])
	}

	var policy StoragePolicy
	if err := db.First(&policy).Error; err != nil {
		t.Fatal(err)
	}
	if policy.Driver != "local" || policy.Config["root"] != "uploads" || !policy.Enabled {
		t.Errorf("本地策略不符: %+v", policy)
	}
	if policy.PathTemplate != "{Y}/{m}/{d}/{uniqid}.{ext}" {
		t.Errorf("路径模板 = %q", policy.PathTemplate)
	}

	var reg Setting
	if err := db.First(&reg, "key = ?", SettingRegistrationMode).Error; err != nil {
		t.Fatal(err)
	}
	if reg.Value != `"open"` {
		t.Errorf("registration_mode = %s, want \"open\"", reg.Value)
	}
}

func TestSQLiteForeignKeysPragmaOn(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{DataDir: dir, Database: config.Database{Driver: "sqlite"}}
	db, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var fk int
	if err := db.Raw("PRAGMA foreign_keys").Scan(&fk).Error; err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys pragma = %d, want 1", fk)
	}
	// 默认关闭 mmap，降低绑定挂载/低内存环境 OOM。
	var mmap int64
	if err := db.Raw("PRAGMA mmap_size").Scan(&mmap).Error; err != nil {
		t.Fatal(err)
	}
	if mmap != 0 {
		t.Fatalf("mmap_size = %d, want 0", mmap)
	}
}

func TestSQLiteFileDSNContainsDefaults(t *testing.T) {
	dsn := SQLiteFileDSN("/data")
	if !strings.Contains(dsn, "/data/imgli.db?") {
		t.Fatalf("path: %s", dsn)
	}
	for _, frag := range []string{"journal_mode(WAL)", "busy_timeout(5000)", "foreign_keys(1)", "mmap_size(0)", "cache_size(-8000)", "temp_store(FILE)"} {
		if !strings.Contains(dsn, frag) {
			t.Errorf("missing %s in %s", frag, dsn)
		}
	}
}

func TestEnsureDataDirWritable(t *testing.T) {
	dir := t.TempDir()
	if err := ensureDataDirWritable(dir); err != nil {
		t.Fatal(err)
	}
	// not a directory
	f := filepath.Join(dir, "file")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureDataDirWritable(f); err == nil {
		t.Fatal("expected error for file path")
	}
}

func TestJSONSerializerRoundTrip(t *testing.T) {
	db := TestDB(t)
	g := UserGroup{Name: "试验组", AllowedExts: []string{"png", "jpg"}, AllowedPolicyIDs: []uint64{1}}
	if err := db.Create(&g).Error; err != nil {
		t.Fatal(err)
	}
	var got UserGroup
	db.First(&got, g.ID)
	if len(got.AllowedExts) != 2 || got.AllowedExts[0] != "png" {
		t.Errorf("AllowedExts 序列化往返失败: %+v", got.AllowedExts)
	}
}
