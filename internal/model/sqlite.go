package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/config"
)

// sqliteDefaultPragmas 默认随空 DSN 拼接的连接 pragma。
// mmap_size(0)：关闭内存映射，降低 Docker 绑定挂载 + 低内存 ARM 上 SQLITE_NOMEM / OOM 概率。
// cache_size(-8000)：约 8MiB 页缓存（负数单位 KiB）。
// temp_store(FILE)：临时表落盘，避免低内存主机把大排序塞进 RAM。
const sqliteDefaultPragmas = "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=mmap_size(0)&_pragma=cache_size(-8000)&_pragma=temp_store(FILE)"

// SQLiteFileDSN 返回 dataDir 下 imgli.db 的 glebarez DSN（含默认 pragma）。
func SQLiteFileDSN(dataDir string) string {
	return filepath.Join(dataDir, "imgli.db") + "?" + sqliteDefaultPragmas
}

// ensureDataDirWritable 创建 data_dir 并探测可写；失败时给出 Docker 绑定挂载常见提示。
func ensureDataDirWritable(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return fmt.Errorf("data_dir 为空")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建 data_dir %s: %w%s", dir, err, dataDirHint(dir, err))
	}
	probe := filepath.Join(dir, ".imgli-write-probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		return fmt.Errorf("data_dir 不可写 %s: %w%s", dir, err, dataDirHint(dir, err))
	}
	_ = os.Remove(probe)
	return nil
}

func dataDirHint(dir string, err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "permission") && !strings.Contains(msg, "denied") && !os.IsPermission(err) {
		return ""
	}
	return fmt.Sprintf(
		"（提示：Docker 官方镜像以 uid 1000 运行。绑定挂载时请 chown 1000:1000 %s，或改用命名卷；也可依赖镜像 entrypoint 在 root 启动时修正目录属主）",
		dir,
	)
}

// openSQLite 打开 SQLite：保证 data_dir 可写、默认 pragma，并在 WAL 不可用时回退 DELETE。
func openSQLite(cfg *config.Config, gc *gorm.Config) (*gorm.DB, error) {
	if err := ensureDataDirWritable(cfg.DataDir); err != nil {
		return nil, err
	}

	dsn := strings.TrimSpace(cfg.Database.DSN)
	if dsn == "" {
		dsn = SQLiteFileDSN(cfg.DataDir)
	}

	db, err := gorm.Open(sqlite.Open(dsn), gc)
	if err != nil && strings.TrimSpace(cfg.Database.DSN) == "" {
		// 部分文件系统（网络盘/特殊绑定）拒绝 WAL：回退 DELETE 再试一次。
		fallback := filepath.Join(cfg.DataDir, "imgli.db") +
			"?_pragma=journal_mode(DELETE)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=mmap_size(0)&_pragma=cache_size(-8000)&_pragma=temp_store(FILE)"
		db, err = gorm.Open(sqlite.Open(fallback), gc)
	}
	if err != nil {
		return nil, fmt.Errorf("打开 sqlite: %w%s", err, dataDirHint(cfg.DataDir, err))
	}

	if sqlDB, e := db.DB(); e == nil {
		// 单写者：消除 shared-cache 锁竞争
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
	}

	if err := applySQLiteRuntimePragmas(db); err != nil {
		_ = closeDB(db)
		return nil, err
	}
	return db, nil
}

// applySQLiteRuntimePragmas 在连接上强制安全默认（含用户自定义 DSN 未写 pragma 的情况）。
func applySQLiteRuntimePragmas(db *gorm.DB) error {
	// foreign_keys / busy_timeout / mmap / cache / temp_store：自定义 DSN 也兜底。
	for _, p := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA mmap_size = 0",
		"PRAGMA cache_size = -8000",
		"PRAGMA temp_store = FILE",
	} {
		if err := db.Exec(p).Error; err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
	}
	// 优先 WAL；失败则保持驱动/文件当前模式（多为 DELETE）。
	var mode string
	if err := db.Raw("PRAGMA journal_mode = WAL").Scan(&mode).Error; err != nil {
		// 忽略：部分 FS 报错但仍可用默认 journal
		_ = db.Exec("PRAGMA journal_mode = DELETE")
	} else if mode != "" && !strings.EqualFold(mode, "wal") {
		// 已是 delete 等模式，可接受
	}
	return nil
}

func closeDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
