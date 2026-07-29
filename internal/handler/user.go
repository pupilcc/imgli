package handler

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/adminsvc"
	"github.com/yixian-huang/imgli/internal/service/auth"
	"github.com/yixian-huang/imgli/internal/service/imagesvc"
)

type UserHandlers struct {
	Svc          *auth.Service
	Img          *imagesvc.Service // C-③ 注销级联
	Adm          *adminsvc.Service // C-③ 注销审计
	AvatarDir    string            // C-③ 头像目录 <data_dir>/avatars
	WatermarkDir string            // D-② 水印图目录 <data_dir>/watermarks
	Secure       bool              // C-③ 注销清 cookie 的 Secure 位
}

// Profile GET /api/v1/user/profile
func (h *UserHandlers) Profile(w http.ResponseWriter, r *http.Request) {
	OK(w, userDTO(PrincipalFrom(r).User))
}

// UpdateProfile PATCH /api/v1/user/profile {nickname?, public_profile?}
// 字段均为可选指针：旧前端只发 nickname 仍生效；可只改 public_profile。
func (h *UserHandlers) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Nickname      *string `json:"nickname"`
		PublicProfile *bool   `json:"public_profile"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体不是合法 JSON")
		return
	}
	uid := PrincipalFrom(r).User.ID
	if req.Nickname != nil {
		if err := h.Svc.UpdateNickname(uid, *req.Nickname); err != nil {
			if errors.Is(err, auth.ErrInvalidInput) {
				Fail(w, http.StatusBadRequest, CodeInvalidRequest, "昵称需 1-64 个字符")
				return
			}
			Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
			return
		}
	}
	if req.PublicProfile != nil {
		if err := h.Svc.SetPublicProfile(uid, *req.PublicProfile); err != nil {
			Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
			return
		}
	}
	OK(w, nil)
}

// UpdatePreferences PATCH /api/v1/user/preferences(全量替换)
func (h *UserHandlers) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	var req model.Preferences
	if err := DecodeJSON(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体不是合法 JSON")
		return
	}
	if err := h.Svc.UpdatePreferences(PrincipalFrom(r).User.ID, req); err != nil {
		if errors.Is(err, auth.ErrInvalidInput) {
			Fail(w, http.StatusBadRequest, CodeInvalidRequest, "偏好设置不合法")
			return
		}
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	OK(w, nil)
}

// ChangeEmail POST /api/v1/user/email/change {password, new_email}
func (h *UserHandlers) ChangeEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
		NewEmail string `json:"new_email"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体无效")
		return
	}
	err := h.Svc.RequestChangeEmail(PrincipalFrom(r).User.ID, req.Password, req.NewEmail)
	switch {
	case err == nil:
		OK(w, map[string]any{"sent": true})
	case errors.Is(err, auth.ErrWrongPassword):
		Fail(w, http.StatusUnauthorized, CodeUnauthorized, "密码错误")
	case errors.Is(err, auth.ErrEmailTaken):
		Fail(w, http.StatusConflict, CodeInvalidRequest, "邮箱已被占用")
	case errors.Is(err, auth.ErrInvalidInput):
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "邮箱不合法")
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}

// ChangePassword PATCH /api/v1/user/password {old_password,new_password}
func (h *UserHandlers) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体不是合法 JSON")
		return
	}
	err := h.Svc.ChangePassword(PrincipalFrom(r).User.ID, req.OldPassword, req.NewPassword)
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		Fail(w, http.StatusUnauthorized, "invalid_credentials", "旧密码错误")
	case errors.Is(err, auth.ErrWeakPassword):
		Fail(w, http.StatusBadRequest, "weak_password", "密码至少 8 位且包含字母和数字")
	case err != nil:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	default:
		OK(w, nil) // 全设备登出，前端应跳转登录页
	}
}

// Quota GET /api/v1/user/quota
func (h *UserHandlers) Quota(w http.ResponseWriter, r *http.Request) {
	qi, err := h.Svc.QuotaInfo(PrincipalFrom(r).User.ID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	OK(w, map[string]any{
		"used": qi.Used, "total": qi.Total,
		"max_file_size": qi.MaxFileSize, "allowed_exts": qi.AllowedExts,
		"bandwidth_used_month":  qi.BandwidthUsed,
		"bandwidth_quota_month": qi.BandwidthQuota,
		"bandwidth_period":      qi.BandwidthPeriod,
	})
}

// Policies GET /api/v1/user/policies
func (h *UserHandlers) Policies(w http.ResponseWriter, r *http.Request) {
	list, err := h.Svc.UserPolicies(PrincipalFrom(r).User)
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	OK(w, list)
}

// DeleteAccount DELETE /api/v1/user {password}——自助注销,全部硬删(spec 裁决 2)。
func (h *UserHandlers) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体不是合法 JSON")
		return
	}
	u := PrincipalFrom(r).User
	if u.IsAdmin {
		Fail(w, http.StatusForbidden, "admin_cannot_self_delete", "管理员账号不能自助注销,请先由其他管理员移除管理员身份")
		return
	}
	if !auth.VerifyPassword(u.PasswordHash, req.Password) {
		Fail(w, http.StatusUnauthorized, "invalid_credentials", "密码错误")
		return
	}
	// 头像文件先删(不存在视为已删):必须在 DB 级联之前——/avatar/{id} 是公开路径,
	// 账号删除后残留的头像文件将无人能清;此刻失败则中止,账号未动可重试。
	if err := os.Remove(filepath.Join(h.AvatarDir, fmt.Sprintf("%d.jpg", u.ID))); err != nil && !os.IsNotExist(err) {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	if err := os.Remove(filepath.Join(h.WatermarkDir, fmt.Sprintf("%d.png", u.ID))); err != nil && !os.IsNotExist(err) {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	if err := h.Img.DeleteUserData(u.ID); err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	h.Adm.Audit(nil, "user", "user_self_delete",
		map[string]any{"user_id": u.ID, "username": u.Username, "email": u.Email}, ClientIP(r))
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookie, Value: "", Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: h.Secure, MaxAge: -1,
	})
	OK(w, nil)
}
