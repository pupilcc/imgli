package handler

import (
	"errors"
	"net/http"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/settings"
)

// ConfigHandler GET /api/v1/config（公开，无需鉴权）——前端/游客上传页据此渲染品牌、
// 注册开关与游客上传限额。绝不含密钥/私密配置（如机审 api_key）。
type ConfigHandler struct{ DB *gorm.DB }

// Config GET /api/v1/config → {site_name, registration_mode, guest_upload_enabled,
// guest:{max_file_size, allowed_exts, per_day}}。settings 三键缺省时按 site_name=""/
// registration_mode="open"/guest_upload_enabled=false 处理（ErrNotFound 静默吞掉，与
// adminsvc.GetSettings 同一约定）。guest 取游客组（is_guest=true）的限额；查不到游客组
// （不应发生——已播种）时 guest 降级为 nil（JSON 里是 null），不因此报错整个请求。
func (h *ConfigHandler) Config(w http.ResponseWriter, r *http.Request) {
	st := settings.New(h.DB)

	var siteName string
	if err := st.Get(model.SettingSiteName, &siteName); err != nil && !errors.Is(err, settings.ErrNotFound) {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	regMode := "open"
	if err := st.Get(model.SettingRegistrationMode, &regMode); err != nil && !errors.Is(err, settings.ErrNotFound) {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	var guestUpload bool
	if err := st.Get(model.SettingGuestUpload, &guestUpload); err != nil && !errors.Is(err, settings.ErrNotFound) {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	var plazaEnabled bool
	if err := st.Get(model.SettingPlazaEnabled, &plazaEnabled); err != nil && !errors.Is(err, settings.ErrNotFound) {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}

	var guest any
	var g model.UserGroup
	if err := h.DB.Where("is_guest = ?", true).First(&g).Error; err == nil {
		guest = map[string]any{
			"max_file_size": g.MaxFileSize,
			"allowed_exts":  g.AllowedExts,
			"per_day":       g.RatePerDay,
		}
	}

	OK(w, map[string]any{
		"site_name":            siteName,
		"registration_mode":    regMode,
		"guest_upload_enabled": guestUpload,
		"plaza_enabled":        plazaEnabled,
		"guest":                guest,
	})
}
