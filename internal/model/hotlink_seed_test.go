// 独立的外部测试包（package model_test）：db.go 的 settingHotlinkDefaultJSON 字面量
// 需要与 stats.DefaultHotlink() 逐字段核对一致，但 stats 包 import 了 model 包。
// 若这个断言放在 package model 内部测试文件里，会与 stats 反向 import model 形成循环依赖。
// 放到外部测试包 model_test 则不构成环。
package model_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/stats"
)

// TestSettingHotlinkSeedMatchesDefaultHotlink 断言 db.go 手写的 hotlink 播种
// JSON 字面量与 stats.DefaultHotlink() 逐字段一致，防止两处漂移。
func TestSettingHotlinkSeedMatchesDefaultHotlink(t *testing.T) {
	db := model.TestDB(t)
	var row model.Setting
	if err := db.First(&row, "key = ?", model.SettingHotlink).Error; err != nil {
		t.Fatal(err)
	}
	var seeded stats.HotlinkConfig
	if err := json.Unmarshal([]byte(row.Value), &seeded); err != nil {
		t.Fatalf("播种的 hotlink JSON 解析失败: %v", err)
	}
	if !reflect.DeepEqual(seeded, stats.DefaultHotlink()) {
		t.Errorf("播种值 = %+v, DefaultHotlink() = %+v", seeded, stats.DefaultHotlink())
	}
}
