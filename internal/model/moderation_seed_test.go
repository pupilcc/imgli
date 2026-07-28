// 独立的外部测试包（package model_test）：db.go 的 settingModerationDefaultJSON 字面量
// 需要与 moderation.DefaultConfig() 逐字段核对一致，但 moderation 包（Task 7 起）import
// 了 model 包（ModerateTask 需要 model.Image/File/StoragePolicy/AuditLog）。若这个断言放在
// package model 内部测试文件里，会与 moderation 反向 import model 形成循环依赖
// （model 内部测试 → moderation → model）。放到外部测试包 model_test 则不构成环：
// model_test 本身不是 model 包的一部分，moderation 也从不 import model_test。
package model_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/moderation"
)

// TestSettingModerationSeedMatchesDefaultConfig 断言 db.go 手写的 moderation 播种
// JSON 字面量与 moderation.DefaultConfig() 逐字段一致，防止两处漂移。
func TestSettingModerationSeedMatchesDefaultConfig(t *testing.T) {
	db := model.TestDB(t)
	var row model.Setting
	if err := db.First(&row, "key = ?", model.SettingModeration).Error; err != nil {
		t.Fatal(err)
	}
	var seeded moderation.Config
	if err := json.Unmarshal([]byte(row.Value), &seeded); err != nil {
		t.Fatalf("播种的 moderation JSON 解析失败: %v", err)
	}
	// Config 含 []string 不可用 ==；DeepEqual 比较
	if !reflect.DeepEqual(seeded, moderation.DefaultConfig()) {
		t.Errorf("播种值 = %+v, DefaultConfig() = %+v", seeded, moderation.DefaultConfig())
	}
}
