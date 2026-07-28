package model

import (
	"log/slog"

	"gorm.io/gorm"
)

// fkRel 一个 belongs-to 关联(child model + 关联字段名),供显式建约束。
type fkRel struct {
	model any
	field string
}

// fkRelations 全部需 DB 级外键的关联(ON DELETE 语义在 model.go 的 constraint 标签里)。
func fkRelations() []fkRel {
	return []fkRel{
		{&User{}, "Group"},
		{&File{}, "Policy"},
		{&Image{}, "User"}, {&Image{}, "File"}, {&Image{}, "Album"},
		{&Album{}, "User"},
		{&APIToken{}, "User"}, {&Session{}, "User"}, {&AuthToken{}, "User"},
	}
}

// applyForeignKeys 显式建外键约束:
//   - 存量 SQLite(非 fresh):跳过——SQLite 无法原地 ALTER 加 FK,重建有风险(裁决:不重建),保留应用层引用完整性。
//   - 全新库(任意方言)/ 存量 Postgres:逐关联 HasConstraint 否则 CreateConstraint。
//   - 非 fresh 先清 SET NULL 关系的悬挂引用,避免 ADD CONSTRAINT 因历史脏数据失败。
func applyForeignKeys(db *gorm.DB, fresh bool) error {
	isSQLite := db.Dialector.Name() == "sqlite"
	if isSQLite && !fresh {
		slog.Info("存量 SQLite 库:跳过外键约束创建,保留应用层引用完整性")
		return nil
	}
	if !fresh {
		if err := cleanupOrphans(db); err != nil {
			return err
		}
	}
	m := db.Migrator()
	for _, r := range fkRelations() {
		if m.HasConstraint(r.model, r.field) {
			continue
		}
		if err := m.CreateConstraint(r.model, r.field); err != nil {
			return err
		}
	}
	return nil
}

// cleanupOrphans 清理会阻断 ADD CONSTRAINT 的历史悬挂引用。仅处理 SET NULL 关系
// (images.album_id 指向已不存在的相册)。RESTRICT 关系若有孤儿属数据异常,交由
// CreateConstraint 显式报错暴露,不静默删数据。
func cleanupOrphans(db *gorm.DB) error {
	return db.Exec(
		`UPDATE images SET album_id = NULL
		 WHERE album_id IS NOT NULL
		   AND album_id NOT IN (SELECT id FROM albums)`).Error
}
