// 独立的外部测试包（package model_test）：db.go 的 settingProcessingDefaultJSON 字面量
// 需要与 upload.DefaultProcessing() 逐字段核对一致，但 upload 包 import 了 model 包。
// 若这个断言放在 package model 内部测试文件里，会与 upload 反向 import model 形成循环依赖。
// 放到外部测试包 model_test 则不构成环。
package model_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/upload"
)

// TestSettingProcessingSeedMatchesDefaultProcessing 断言 db.go 手写的 processing 播种
// JSON 字面量与 upload.DefaultProcessing() 逐字段一致，防止两处漂移。
func TestSettingProcessingSeedMatchesDefaultProcessing(t *testing.T) {
	db := model.TestDB(t)
	var row model.Setting
	if err := db.First(&row, "key = ?", model.SettingProcessing).Error; err != nil {
		t.Fatal(err)
	}
	var seeded upload.Processing
	if err := json.Unmarshal([]byte(row.Value), &seeded); err != nil {
		t.Fatalf("播种的 processing JSON 解析失败: %v", err)
	}
	if !reflect.DeepEqual(seeded, upload.DefaultProcessing()) {
		t.Errorf("播种值 = %+v, DefaultProcessing() = %+v", seeded, upload.DefaultProcessing())
	}
}
