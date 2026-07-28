package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/yixian-huang/imgli/internal/mail"
)

// TestSMTP POST /api/v1/admin/settings/smtp/test {to} —— 用当前已保存配置即时发一封测试邮件。
func (h *AdminHandlers) TestSMTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To string `json:"to"`
	}
	if err := DecodeJSON(r, &req); err != nil || !strings.Contains(req.To, "@") {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "收件人邮箱无效")
		return
	}
	err := h.D.Mail.Send(req.To, "SMTP 测试邮件", "<p>看到这封邮件说明 SMTP 配置可用。</p>")
	switch {
	case err == nil:
		actor := PrincipalFrom(r).User
		h.D.Adm.Audit(&actor.ID, "admin", "smtp_test", map[string]any{"to": req.To}, ClientIP(r))
		OK(w, map[string]any{})
	case errors.Is(err, mail.ErrNotConfigured):
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "SMTP 未配置,请先保存邮件设置")
	default:
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "发送失败:"+err.Error())
	}
}
