// Package moderation 定义机审配置的类型、默认值与校验规则。
//
// 本文件（Task 6）仅含类型 + 默认值 + 校验函数——Scorer 接口与 WebhookScorer 实现、
// 任务管线接入留 Task 7。之所以把 Config 单独前移到独立包（而不是内联在 adminsvc 里），
// 是为了让 adminsvc/settings.go 与（未来）上传管线都能直接依赖同一份配置类型，
// 避免 adminsvc ⇄ moderation 之间出现循环依赖。
package moderation

import (
	"net/url"

	"github.com/yixian-huang/imgli/internal/apperr"
)

var (
	// ErrThresholdRange threshold 必须落在 [0,1] 闭区间。
	ErrThresholdRange = apperr.New("threshold 必须在 0-1 之间")
	// ErrActionInvalid action 仅支持 pending|rejected。
	ErrActionInvalid = apperr.New("action 仅支持 pending|rejected")
	// ErrProviderInvalid provider 仅支持 webhook|aliyun|tencent|openai|nsfwjs。
	ErrProviderInvalid = apperr.New("provider 仅支持 webhook|aliyun|tencent|openai|nsfwjs")
	// ErrEndpointInvalid enabled=true 时 endpoint 必须是合法的 http(s) URL。
	ErrEndpointInvalid = apperr.New("enabled 时 endpoint 必须是 http(s) URL")
	// ErrCredentialMissing 所选机审服务商缺少必填凭据。
	ErrCredentialMissing = apperr.New("所选机审服务商缺少必填凭据")
	// ErrOCRKeywordsInvalid ocr_keywords 子配置非法（endpoint/on_hit）。
	ErrOCRKeywordsInvalid = apperr.New("ocr_keywords 配置无效")
	// ErrLoginSampleRate login_sample_rate 必须在 [0,1]。
	ErrLoginSampleRate = apperr.New("login_sample_rate 必须在 0-1 之间")
	// ErrOnPluginError on_plugin_error 仅支持 open|review。
	ErrOnPluginError = apperr.New("on_plugin_error 仅支持 open|review")
)

// OCRKeywordsConfig OCR+词表插件（PR3）。默认关闭；OCR 外置，词表在 imgli 匹配。
type OCRKeywordsConfig struct {
	Enabled  bool     `json:"enabled"`
	Endpoint string   `json:"endpoint"` // 外置 OCR HTTP，POST 原图 → {"text":"..."}
	APIKey   string   `json:"api_key"`  // 可选 Bearer
	Keywords []string `json:"keywords"` // 子串匹配（大小写不敏感）
	OnHit    string   `json:"on_hit"`   // review（默认）| block
}

// Config 机审配置。字段与 JSON 标签是 /admin/settings 的 moderation 子对象契约，
// 逐字段修改需同步 adminsvc/settings.go 的打码/保留语义与 handler 层。
type Config struct {
	Enabled         bool              `json:"enabled"`
	Provider        string            `json:"provider"`
	Endpoint        string            `json:"endpoint"`
	APIKey          string            `json:"api_key"`
	AccessKeyID     string            `json:"access_key_id"`
	AccessKeySecret string            `json:"access_key_secret"`
	Region          string            `json:"region"`
	Threshold       float64           `json:"threshold"`
	Action          string            `json:"action"`
	OCRKeywords     OCRKeywordsConfig `json:"ocr_keywords"`
	// LoginSampleRate 登录用户新图入队概率 [0,1]；游客恒 1（加严）。默认 1=全审。
	LoginSampleRate float64 `json:"login_sample_rate"`
	// OnPluginError 插件失败策略：open（默认 fail-open）| review（合成进待审）。
	OnPluginError string `json:"on_plugin_error"`
	// NotifyOnReject 人审/机审拒绝成功后异步邮件通知属主（默认关）。
	NotifyOnReject bool `json:"notify_on_reject"`
}

// DefaultConfig 返回机审的出厂默认值：禁用、webhook 供应商、阈值 0.8、命中动作 pending。
//
// 播种进 model.Seed 的 JSON 字面量（internal/model/db.go 的 SettingModeration 项）必须与
// 本函数返回值逐字段一致——db.go 出于避免 model⇄moderation 循环依赖，手写了等价的 JSON
// 字面量而非 import 本包；两者的一致性由 internal/model/moderation_seed_test.go
// （package model_test 外部测试包，可同时 import model 与 moderation 而不成环）断言。
func DefaultConfig() Config {
	return Config{
		Enabled:         false,
		Provider:        "webhook",
		Endpoint:        "",
		APIKey:          "",
		AccessKeyID:     "",
		AccessKeySecret: "",
		Region:          "",
		Threshold:       0.8,
		Action:          "pending",
		OCRKeywords:     OCRKeywordsConfig{}, // enabled=false 出厂
		LoginSampleRate: 1.0,
		OnPluginError:   "open",
		NotifyOnReject:  false,
	}
}

// ValidateConfig 校验机审配置：
//   - threshold ∈ [0,1]
//   - action ∈ {pending, rejected}
//   - provider ∈ {webhook, aliyun, tencent, openai, nsfwjs}
//   - enabled 时按 provider 分校验：webhook/nsfwjs 要求合法 http(s) endpoint；
//     openai 要求 api_key；aliyun/tencent 要求 access_key_id/secret 与 region
//   - ocr_keywords.enabled 时要求合法 endpoint；on_hit ∈ {""|review|block}
//   - login_sample_rate ∈ [0,1]（0 表示登录用户不入队机审；游客仍全审）
//   - on_plugin_error ∈ {""|open|review}（空视为 open）
//     （裁决 9：endpoint 是管理员配置的可信目标，不做 SSRF 限制）
func ValidateConfig(c Config) error {
	if c.Threshold < 0 || c.Threshold > 1 {
		return ErrThresholdRange
	}
	// 兼容旧 settings JSON 缺字段：0 且未显式配置时由 DefaultConfig 播种为 1；
	// 校验允许 0..1，缺省反序列化 0 时在 Get 侧用 NormalizeConfig 拉回 1。
	if c.LoginSampleRate < 0 || c.LoginSampleRate > 1 {
		return ErrLoginSampleRate
	}
	switch c.OnPluginError {
	case "", "open", "review":
	default:
		return ErrOnPluginError
	}
	switch c.Action {
	case "pending", "rejected":
	default:
		return ErrActionInvalid
	}
	switch c.Provider {
	case "webhook", "aliyun", "tencent", "openai", "nsfwjs":
	default:
		return ErrProviderInvalid
	}
	if err := validateOCRKeywords(c.OCRKeywords); err != nil {
		return err
	}
	if !c.Enabled {
		return nil
	}
	switch c.Provider {
	case "webhook", "nsfwjs":
		u, err := url.Parse(c.Endpoint)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return ErrEndpointInvalid
		}
	case "openai":
		if c.APIKey == "" {
			return ErrCredentialMissing
		}
	case "aliyun", "tencent":
		if c.AccessKeyID == "" || c.AccessKeySecret == "" || c.Region == "" {
			return ErrCredentialMissing
		}
	}
	return nil
}

func validateOCRKeywords(o OCRKeywordsConfig) error {
	switch o.OnHit {
	case "", "review", "block":
	default:
		return ErrOCRKeywordsInvalid
	}
	if !o.Enabled {
		return nil
	}
	u, err := url.Parse(o.Endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ErrOCRKeywordsInvalid
	}
	return nil
}

// NormalizeConfig 填缺省字段（旧 JSON 无 on_plugin_error 时）。
// login_sample_rate：load 时以 DefaultConfig 为基底 Unmarshal，缺字段保留 1.0；显式 0 合法。
func NormalizeConfig(c *Config) {
	if c.OnPluginError == "" {
		c.OnPluginError = "open"
	}
}

// ShouldEnqueueModerate 游客恒入队；登录用户按 login_sample_rate 确定性抽检（fnv(key)%1000）。
// rate>=1 或 key 空：登录也入队；rate<=0：登录不入队。
func ShouldEnqueueModerate(isGuest bool, rate float64, imageKey string) bool {
	if isGuest {
		return true
	}
	if rate >= 1 {
		return true
	}
	if rate <= 0 {
		return false
	}
	// 确定性：同 key 稳定
	h := uint32(2166136261)
	for i := 0; i < len(imageKey); i++ {
		h ^= uint32(imageKey[i])
		h *= 16777619
	}
	return float64(h%1000) < rate*1000
}
