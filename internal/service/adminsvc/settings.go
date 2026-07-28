package adminsvc

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/yixian-huang/imgli/internal/mail"
	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/moderation"
	"github.com/yixian-huang/imgli/internal/service/settings"
	"github.com/yixian-huang/imgli/internal/service/stats"
	"github.com/yixian-huang/imgli/internal/service/upload"
)

// smtpFromRe from 字段：非空时需为形如 local@domain.tld 的邮箱(空值仍允许)。
var smtpFromRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// hotlinkHostRe 防盗链域名(剥掉可选 *. 前缀后的部分):小写字母/数字开头结尾的
// 点分标签,标签内允许连字符——域名字符白名单,兜住黑名单漏网(? 内嵌 * 等)。
var hotlinkHostRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`)

var (
	// ErrUnknownSetting PutSettings 只认 site_name/registration_mode/guest_upload_enabled/plaza_enabled/moderation/smtp/hotlink/processing 键。
	ErrUnknownSetting = errors.New("未知的设置键")
	// ErrSiteNameInvalid site_name 需 1-64 个字符（TrimSpace 后）。
	ErrSiteNameInvalid = errors.New("site_name 需 1-64 个字符")
	// ErrRegistrationModeInvalid registration_mode 仅支持 open|invite|closed。
	ErrRegistrationModeInvalid = errors.New("registration_mode 仅支持 open|invite|closed")
	// ErrModerationInvalid moderation 键的值不是合法 JSON 对象。
	ErrModerationInvalid = errors.New("moderation 配置格式错误")
	// ErrGuestUploadInvalid guest_upload_enabled 需为布尔值。
	ErrGuestUploadInvalid = errors.New("guest_upload_enabled 需为布尔值")
	// ErrPlazaEnabledInvalid plaza_enabled 需为布尔值。
	ErrPlazaEnabledInvalid = errors.New("plaza_enabled 需为布尔值")
	// ErrSMTPInvalid smtp 配置：port 需 1-65535、encryption 需 none|starttls|ssl、from 需为邮箱或留空。
	ErrSMTPInvalid = errors.New("smtp 配置无效:port 需 1-65535、encryption 需 none|starttls|ssl、from 需为邮箱或留空")
	// ErrHotlinkDomainInvalid 防盗链域名不合法（空/空白/scheme/路径/非法通配）。
	ErrHotlinkDomainInvalid = errors.New("防盗链域名不合法")
)

// maskAPIKey 打码 api_key：非空时返回 "****"+尾4字符（长度<=4 时全打码为 "****"），
// 空值原样返回空——GET 响应与 PUT 保留语义（settingWrite 里识别 "****" 前缀）共用此约定。
func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 4 {
		return "****"
	}
	return "****" + key[len(key)-4:]
}

// MaskSecret 供 handler 层给策略 config 里的密钥打码(复用 maskAPIKey:****+尾4)。
func MaskSecret(s string) string { return maskAPIKey(s) }

// GetSettings 返回管理端设置面：{site_name, registration_mode, guest_upload_enabled, plaza_enabled, moderation:{...}, smtp:{...}, hotlink:{...}, processing:{...}}。
// moderation.api_key / moderation.access_key_secret / smtp.password 按 maskAPIKey 打码——明文密钥永不通过本方法对外可见。
// access_key_id 与 region 明文回显。
func (s *Service) GetSettings() (map[string]any, error) {
	st := settings.New(s.db)

	var siteName string
	if err := st.Get(model.SettingSiteName, &siteName); err != nil && !errors.Is(err, settings.ErrNotFound) {
		return nil, err
	}
	var regMode string
	if err := st.Get(model.SettingRegistrationMode, &regMode); err != nil && !errors.Is(err, settings.ErrNotFound) {
		return nil, err
	}
	modCfg := moderation.DefaultConfig()
	if err := st.Get(model.SettingModeration, &modCfg); err != nil && !errors.Is(err, settings.ErrNotFound) {
		return nil, err
	}
	moderation.NormalizeConfig(&modCfg)
	var guestUpload bool
	if err := st.Get(model.SettingGuestUpload, &guestUpload); err != nil && !errors.Is(err, settings.ErrNotFound) {
		return nil, err
	}
	var plazaEnabled bool
	if err := st.Get(model.SettingPlazaEnabled, &plazaEnabled); err != nil && !errors.Is(err, settings.ErrNotFound) {
		return nil, err
	}
	smtpCfg := mail.DefaultConfig()
	if err := st.Get(model.SettingSMTP, &smtpCfg); err != nil && !errors.Is(err, settings.ErrNotFound) {
		return nil, err
	}
	smtpCfg.Password = maskAPIKey(smtpCfg.Password)
	hotCfg := stats.DefaultHotlink()
	if err := st.Get(model.SettingHotlink, &hotCfg); err != nil && !errors.Is(err, settings.ErrNotFound) {
		return nil, err
	}
	procCfg := upload.DefaultProcessing()
	if err := st.Get(model.SettingProcessing, &procCfg); err != nil && !errors.Is(err, settings.ErrNotFound) {
		return nil, err
	}

	return map[string]any{
		"site_name":            siteName,
		"registration_mode":    regMode,
		"guest_upload_enabled": guestUpload,
		"plaza_enabled":        plazaEnabled,
		"moderation": map[string]any{
			"enabled":            modCfg.Enabled,
			"provider":           modCfg.Provider,
			"endpoint":           modCfg.Endpoint,
			"api_key":            maskAPIKey(modCfg.APIKey),
			"threshold":          modCfg.Threshold,
			"action":             modCfg.Action,
			"access_key_id":      modCfg.AccessKeyID,
			"access_key_secret":  maskAPIKey(modCfg.AccessKeySecret),
			"region":             modCfg.Region,
			"login_sample_rate":  modCfg.LoginSampleRate,
			"on_plugin_error":    modCfg.OnPluginError,
			"notify_on_reject":   modCfg.NotifyOnReject,
			"ocr_keywords": map[string]any{
				"enabled":  modCfg.OCRKeywords.Enabled,
				"endpoint": modCfg.OCRKeywords.Endpoint,
				"api_key":  maskAPIKey(modCfg.OCRKeywords.APIKey),
				"keywords": modCfg.OCRKeywords.Keywords,
				"on_hit":   modCfg.OCRKeywords.OnHit,
			},
		},
		"smtp": map[string]any{
			"host":       smtpCfg.Host,
			"port":       smtpCfg.Port,
			"username":   smtpCfg.Username,
			"password":   smtpCfg.Password,
			"from":       smtpCfg.From,
			"encryption": smtpCfg.Encryption,
		},
		"hotlink":    hotCfg,
		"processing": procCfg,
	}, nil
}

// settingWrite 是校验通过、待落库的单个键值对；PutSettings 先把全部键校验完（收集到
// 一批 settingWrite），全部通过后才真正写库——任一键校验失败，整个请求不落任何键
// （契约「逐键校验，任一键失败整个请求 400 不落库」）。
type settingWrite struct {
	key   string
	value any
}

// PutSettings 部分更新设置面。patch 只认 site_name/registration_mode/guest_upload_enabled/plaza_enabled/moderation/smtp/hotlink/processing
// 键，未知键返回 ErrUnknownSetting。moderation 按整对象校验（moderation.ValidateConfig）；
// 其 api_key / access_key_secret 若以 "****" 开头，视为前端把打码后的展示值原样回传：
// api_key 仅当 provider 与 endpoint 均未变才沿用库中明文；access_key_secret 仅当
// provider/region/access_key_id 均未变才沿用——改指向即失效，返回 ErrModerationInvalid。
// smtp.password 同样支持 "****" 前缀保留语义（host+username 未变）。
// hotlink 经 normalizeHotlink 规整（小写/去重/域名形态校验）。
// processing 经 upload.ValidateProcessing 校验（坏 JSON 与校验失败均返 upload.ErrProcessingInvalid）。
func (s *Service) PutSettings(patch map[string]json.RawMessage) error {
	st := settings.New(s.db)
	writes := make([]settingWrite, 0, len(patch))

	for key, raw := range patch {
		switch key {
		case model.SettingSiteName:
			var name string
			if err := json.Unmarshal(raw, &name); err != nil {
				return ErrSiteNameInvalid
			}
			name = strings.TrimSpace(name)
			if name == "" || len(name) > 64 {
				return ErrSiteNameInvalid
			}
			writes = append(writes, settingWrite{model.SettingSiteName, name})

		case model.SettingRegistrationMode:
			var mode string
			if err := json.Unmarshal(raw, &mode); err != nil {
				return ErrRegistrationModeInvalid
			}
			switch mode {
			case "open", "invite", "closed":
			default:
				return ErrRegistrationModeInvalid
			}
			writes = append(writes, settingWrite{model.SettingRegistrationMode, mode})

		case model.SettingGuestUpload:
			var enabled bool
			if err := json.Unmarshal(raw, &enabled); err != nil {
				return ErrGuestUploadInvalid
			}
			writes = append(writes, settingWrite{model.SettingGuestUpload, enabled})

		case model.SettingPlazaEnabled:
			var enabled bool
			if err := json.Unmarshal(raw, &enabled); err != nil {
				return ErrPlazaEnabledInvalid
			}
			writes = append(writes, settingWrite{model.SettingPlazaEnabled, enabled})

		case model.SettingModeration:
			cfg := moderation.DefaultConfig()
			if err := json.Unmarshal(raw, &cfg); err != nil {
				return ErrModerationInvalid
			}
			moderation.NormalizeConfig(&cfg)
			if strings.HasPrefix(cfg.APIKey, "****") || strings.HasPrefix(cfg.AccessKeySecret, "****") ||
				strings.HasPrefix(cfg.OCRKeywords.APIKey, "****") {
				var cur moderation.Config
				if err := st.Get(model.SettingModeration, &cur); err != nil && !errors.Is(err, settings.ErrNotFound) {
					return err
				}
				if strings.HasPrefix(cfg.APIKey, "****") {
					// 改指向即失效:provider 或 endpoint 变了不得沿用旧 key(收 C-②b 债)
					if cfg.Provider != cur.Provider || cfg.Endpoint != cur.Endpoint {
						return ErrModerationInvalid
					}
					cfg.APIKey = cur.APIKey
				}
				if strings.HasPrefix(cfg.AccessKeySecret, "****") {
					if cfg.Provider != cur.Provider || cfg.Region != cur.Region || cfg.AccessKeyID != cur.AccessKeyID {
						return ErrModerationInvalid
					}
					cfg.AccessKeySecret = cur.AccessKeySecret
				}
				if strings.HasPrefix(cfg.OCRKeywords.APIKey, "****") {
					if cfg.OCRKeywords.Endpoint != cur.OCRKeywords.Endpoint {
						return ErrModerationInvalid
					}
					cfg.OCRKeywords.APIKey = cur.OCRKeywords.APIKey
				}
			}
			if err := moderation.ValidateConfig(cfg); err != nil {
				return err
			}
			writes = append(writes, settingWrite{model.SettingModeration, cfg})

		case model.SettingSMTP:
			var cfg mail.Config
			if err := json.Unmarshal(raw, &cfg); err != nil {
				return ErrSMTPInvalid
			}
			if cfg.Port < 1 || cfg.Port > 65535 {
				return ErrSMTPInvalid
			}
			switch cfg.Encryption {
			case "none", "starttls", "ssl":
			default:
				return ErrSMTPInvalid
			}
			// net/smtp PlainAuth 拒绝非 TLS 连接——none+认证必然发送失败,保存时拒绝。
			if cfg.Encryption == "none" && cfg.Username != "" {
				return ErrSMTPInvalid
			}
			if cfg.From != "" && !smtpFromRe.MatchString(cfg.From) {
				return ErrSMTPInvalid
			}
			if strings.HasPrefix(cfg.Password, "****") {
				var cur mail.Config
				if err := st.Get(model.SettingSMTP, &cur); err != nil && !errors.Is(err, settings.ErrNotFound) {
					return err
				}
				// 防 admin 改指向受控服务器窃取旧凭据:仅 host+username 均未变才保留密码。
				if cfg.Host != cur.Host || cfg.Username != cur.Username {
					return ErrSMTPInvalid
				}
				cfg.Password = cur.Password
			}
			writes = append(writes, settingWrite{model.SettingSMTP, cfg})

		case model.SettingHotlink:
			var cfg stats.HotlinkConfig
			if err := json.Unmarshal(raw, &cfg); err != nil {
				return ErrHotlinkDomainInvalid
			}
			norm, err := normalizeHotlink(cfg)
			if err != nil {
				return err
			}
			writes = append(writes, settingWrite{model.SettingHotlink, norm})

		case model.SettingProcessing:
			var cfg upload.Processing
			if err := json.Unmarshal(raw, &cfg); err != nil {
				return upload.ErrProcessingInvalid
			}
			if err := upload.ValidateProcessing(cfg); err != nil {
				return err
			}
			writes = append(writes, settingWrite{model.SettingProcessing, cfg})

		default:
			return ErrUnknownSetting
		}
	}

	for _, w := range writes {
		if err := st.Set(w.key, w.value); err != nil {
			return err
		}
	}
	return nil
}

// normalizeHotlink 校验并规整防盗链配置:域名逐项 TrimSpace 后须非空、无内部空白、
// 不含 scheme/路径字符(/:);通配仅允许 "*." 前缀且剥掉后仍含 "." 且非空;
// 全部小写化并按序去重。违规返回 ErrHotlinkDomainInvalid。
func normalizeHotlink(cfg stats.HotlinkConfig) (stats.HotlinkConfig, error) {
	out := make([]string, 0, len(cfg.AllowedDomains))
	seen := map[string]bool{}
	for _, d := range cfg.AllowedDomains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" {
			return cfg, ErrHotlinkDomainInvalid
		}
		host := d
		if strings.HasPrefix(d, "*.") {
			host = d[2:]
			if host == "" || !strings.Contains(host, ".") {
				return cfg, ErrHotlinkDomainInvalid
			}
		}
		// 剥掉合法 *. 前缀后不许再出现星号(拒 foo.*.example);字符集白名单拒
		// 空白/scheme/路径/问号等一切非域名字符(codex 终审:此前黑名单漏 ? 与内嵌 *)。
		if !hotlinkHostRe.MatchString(host) {
			return cfg, ErrHotlinkDomainInvalid
		}
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	cfg.AllowedDomains = out
	return cfg, nil
}
