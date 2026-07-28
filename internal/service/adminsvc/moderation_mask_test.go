package adminsvc

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/moderation"
)

// TestGetSettingsMasksAccessKeySecret access_key_secret 打码为 ****+尾4；
// access_key_id / region 明文回显。
func TestGetSettingsMasksAccessKeySecret(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)

	cfg := moderation.Config{
		Enabled: false, Provider: "aliyun", Threshold: 0.8, Action: "pending",
		AccessKeyID: "LTAI5tExampleID", AccessKeySecret: "sk-verysecret12345", Region: "cn-hangzhou",
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Save(&model.Setting{Key: model.SettingModeration, Value: string(b)}).Error; err != nil {
		t.Fatal(err)
	}

	m, err := svc.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	mod := m["moderation"].(map[string]any)
	got := mod["access_key_secret"].(string)
	if got != "****2345" {
		t.Errorf("access_key_secret 打码 = %q, want ****2345", got)
	}
	if mod["access_key_id"] != "LTAI5tExampleID" {
		t.Errorf("access_key_id 应明文回显, got %v", mod["access_key_id"])
	}
	if mod["region"] != "cn-hangzhou" {
		t.Errorf("region 应明文回显, got %v", mod["region"])
	}
}

// TestPutSettingsModerationAccessKeySecretRetain 掩码 secret 且 provider/region/AKID 不变 → 保留明文；
// 改 region 仍传 **** → ErrModerationInvalid。
func TestPutSettingsModerationAccessKeySecretRetain(t *testing.T) {
	svc := New(model.TestDB(t))

	initial := moderation.Config{
		Enabled: false, Provider: "aliyun", Threshold: 0.8, Action: "pending",
		AccessKeyID: "akid-x", AccessKeySecret: "plaintext-secret-value", Region: "cn-hangzhou",
	}
	if err := svc.PutSettings(map[string]json.RawMessage{"moderation": rawJSON(t, initial)}); err != nil {
		t.Fatal(err)
	}

	// provider/region/AKID 不变，打码 secret 回传 → 落库仍是原明文
	patched := initial
	patched.AccessKeySecret = "****alue"
	patched.Threshold = 0.9
	if err := svc.PutSettings(map[string]json.RawMessage{"moderation": rawJSON(t, patched)}); err != nil {
		t.Fatalf("同身份掩码保留应成功: %v", err)
	}
	var row model.Setting
	if err := svc.db.First(&row, "key = ?", model.SettingModeration).Error; err != nil {
		t.Fatal(err)
	}
	var stored moderation.Config
	if err := json.Unmarshal([]byte(row.Value), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.AccessKeySecret != initial.AccessKeySecret {
		t.Errorf("access_key_secret 保留失败: stored=%q want %q", stored.AccessKeySecret, initial.AccessKeySecret)
	}
	if stored.Threshold != 0.9 {
		t.Errorf("threshold 未生效: %v", stored.Threshold)
	}

	// 改 region + 同 **** → ErrModerationInvalid
	changed := initial
	changed.AccessKeySecret = "****alue"
	changed.Region = "cn-shanghai"
	if err := svc.PutSettings(map[string]json.RawMessage{"moderation": rawJSON(t, changed)}); !errors.Is(err, ErrModerationInvalid) {
		t.Fatalf("改 region 仍传掩码 secret: err=%v want ErrModerationInvalid", err)
	}
}

// TestPutSettingsModerationAPIKeyRetainTightened api_key 掩码保留收紧：
// webhook 同 provider/endpoint 可保留；改 endpoint 传 **** → ErrModerationInvalid。
func TestPutSettingsModerationAPIKeyRetainTightened(t *testing.T) {
	svc := New(model.TestDB(t))

	initial := moderation.Config{
		Enabled: true, Provider: "webhook", Endpoint: "https://mod.example.com/score",
		APIKey: "sk-realsecretvalue", Threshold: 0.7, Action: "pending",
	}
	if err := svc.PutSettings(map[string]json.RawMessage{"moderation": rawJSON(t, initial)}); err != nil {
		t.Fatal(err)
	}

	// 同 provider/endpoint 掩码保留
	same := initial
	same.APIKey = "****alue"
	same.Threshold = 0.85
	if err := svc.PutSettings(map[string]json.RawMessage{"moderation": rawJSON(t, same)}); err != nil {
		t.Fatalf("同指向掩码 api_key 应保留: %v", err)
	}
	var row model.Setting
	if err := svc.db.First(&row, "key = ?", model.SettingModeration).Error; err != nil {
		t.Fatal(err)
	}
	var stored moderation.Config
	if err := json.Unmarshal([]byte(row.Value), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.APIKey != initial.APIKey {
		t.Errorf("api_key 保留失败: stored=%q want %q", stored.APIKey, initial.APIKey)
	}

	// 改 endpoint + **** → ErrModerationInvalid
	moved := initial
	moved.APIKey = "****alue"
	moved.Endpoint = "https://evil.example.com/score"
	if err := svc.PutSettings(map[string]json.RawMessage{"moderation": rawJSON(t, moved)}); !errors.Is(err, ErrModerationInvalid) {
		t.Fatalf("改 endpoint 仍传掩码 api_key: err=%v want ErrModerationInvalid", err)
	}
}
