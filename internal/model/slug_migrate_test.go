package model_test

import (
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
)

func TestSlugColumnMigrated(t *testing.T) {
	db := model.TestDB(t)
	var n int64
	if err := db.Raw("SELECT count(*) FROM pragma_table_info('images') WHERE name='slug'").Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("slug column count=%d", n)
	}
}
