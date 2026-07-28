package model_test

import (
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
)

func TestSlugColumnMigrated(t *testing.T) {
	db := model.TestDB(t)
	// Migrator 走各方言的 information_schema/pragma,双方言可跑
	// (裸 pragma_table_info 只有 SQLite 认识)。
	if !db.Migrator().HasColumn(&model.Image{}, "slug") {
		t.Fatal("images 表缺 slug 列")
	}
}
