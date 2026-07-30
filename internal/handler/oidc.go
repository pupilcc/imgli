package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/yixian-huang/imgli/internal/service/auth"
)

// OIDCHandlers OIDC 登录与管理配置。
type OIDCHandlers struct {
	OIDC   *auth.OIDCService
	Auth   *auth.Service
	Secure bool
}

// PublicEnabled GET 片段：是否展示 OIDC 按钮（由 config 合并）。
func (h *OIDCHandlers) Enabled() bool {
	return h != nil && h.OIDC != nil && h.OIDC.EnabledPublic()
}

// Start GET /api/v1/auth/oidc/start → 302 to IdP
func (h *OIDCHandlers) Start(w http.ResponseWriter, r *http.Request) {
	if h.OIDC == nil || !h.OIDC.EnabledPublic() {
		Fail(w, http.StatusNotFound, CodeNotFound, "OIDC 未启用")
		return
	}
	state := auth.RandomState()
	http.SetCookie(w, &http.Cookie{
		Name:     "imgli_oidc_state",
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.Secure,
	})
	url, err := h.OIDC.AuthCodeURL(r.Context(), state)
	if err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, err.Error())
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

// Callback GET /api/v1/auth/oidc/callback
func (h *OIDCHandlers) Callback(w http.ResponseWriter, r *http.Request) {
	if h.OIDC == nil {
		Fail(w, http.StatusNotFound, CodeNotFound, "OIDC 未启用")
		return
	}
	q := r.URL.Query()
	if errMsg := q.Get("error"); errMsg != "" {
		http.Redirect(w, r, "/login?oidc_error=1", http.StatusFound)
		return
	}
	state := q.Get("state")
	c, err := r.Cookie("imgli_oidc_state")
	if err != nil || c.Value == "" || c.Value != state {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "state 无效")
		return
	}
	// clear state cookie
	http.SetCookie(w, &http.Cookie{Name: "imgli_oidc_state", Value: "", Path: "/", MaxAge: -1})

	code := q.Get("code")
	if code == "" {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "缺少 code")
		return
	}
	u, err := h.OIDC.Exchange(r.Context(), code)
	if err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, err.Error())
		return
	}
	raw, err := h.Auth.CreateSession(u, ClientIP(r), r.UserAgent())
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "会话创建失败")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    raw,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.Secure,
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

// GetOIDCAdmin GET /api/v1/admin/oidc
func (h *OIDCHandlers) GetOIDCAdmin(w http.ResponseWriter, r *http.Request) {
	c := h.OIDC.LoadConfig()
	// mask secret
	if c.ClientSecret != "" {
		c.ClientSecret = "********"
	}
	OK(w, c)
}

// PutOIDCAdmin PUT /api/v1/admin/oidc
func (h *OIDCHandlers) PutOIDCAdmin(w http.ResponseWriter, r *http.Request) {
	var c auth.OIDCConfig
	if err := DecodeJSON(r, &c); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体无效")
		return
	}
	// preserve secret if masked
	if c.ClientSecret == "********" || c.ClientSecret == "" {
		old := h.OIDC.LoadConfig()
		if c.ClientSecret == "********" {
			c.ClientSecret = old.ClientSecret
		}
	}
	c.Issuer = strings.TrimSpace(c.Issuer)
	c.ClientID = strings.TrimSpace(c.ClientID)
	if c.Enabled && (c.Issuer == "" || c.ClientID == "") {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "启用时需要 issuer 与 client_id")
		return
	}
	if err := h.OIDC.SaveConfig(c); err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "保存失败")
		return
	}
	out := c
	if out.ClientSecret != "" {
		out.ClientSecret = "********"
	}
	OK(w, out)
}
