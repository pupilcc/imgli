package handler

import (
	"net/http"
	"strings"

	"github.com/yixian-huang/imgli/internal/service/settings"
	"github.com/yixian-huang/imgli/internal/service/webhook"
)

// GetWebhooks GET /api/v1/admin/webhooks
func (h *AdminHandlers) GetWebhooks(w http.ResponseWriter, r *http.Request) {
	var c webhook.Config
	_ = settings.New(h.D.Adm.DB()).Get(webhook.SettingKey, &c)
	OK(w, c)
}

// PutWebhooks PUT /api/v1/admin/webhooks
func (h *AdminHandlers) PutWebhooks(w http.ResponseWriter, r *http.Request) {
	var c webhook.Config
	if err := DecodeJSON(r, &c); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体无效")
		return
	}
	c.URL = strings.TrimSpace(c.URL)
	if c.Enabled && c.URL == "" {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "启用时需要 URL")
		return
	}
	if err := settings.New(h.D.Adm.DB()).Set(webhook.SettingKey, c); err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	actor := PrincipalFrom(r).User
	h.D.Adm.Audit(&actor.ID, "admin", "webhooks_update", map[string]any{"enabled": c.Enabled}, ClientIP(r))
	OK(w, c)
}
