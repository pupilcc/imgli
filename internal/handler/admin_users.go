package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/adminsvc"
)

// adminUserDTO 管理端视角的用户字段（不含 image_count，供 PATCH 响应复用）。
func adminUserDTO(u *model.User) map[string]any {
	return map[string]any{
		"id": u.ID, "username": u.Username, "email": u.Email,
		"nickname": u.Nickname, "group_id": u.GroupID, "status": u.Status,
		"is_admin": u.IsAdmin, "used_storage": u.UsedStorage,
		"email_verified": u.EmailVerifiedAt != nil,
		"signup_channel": u.SignupChannel,
		"created_at":     u.CreatedAt.Format(time.RFC3339),
	}
}

func userRowDTO(row *adminsvc.UserRow) map[string]any {
	m := adminUserDTO(&row.User)
	m["image_count"] = row.ImageCount
	return m
}

const (
	usersDefaultPage  = 1
	usersDefaultLimit = 50
	usersMaxLimit     = 200
)

// Users GET /api/v1/admin/users?q=&group=&status=&page=&limit=
func (h *AdminHandlers) Users(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page, limit := ParsePage(r, usersDefaultPage, usersDefaultLimit, usersMaxLimit)
	var groupID uint64
	if v := query.Get("group"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			groupID = n
		}
	}
	rows, total, err := h.D.Adm.ListUsers(query.Get("q"), groupID, query.Get("status"), query.Get("channel"), query.Get("sort"), page, limit)
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for i := range rows {
		items = append(items, userRowDTO(&rows[i]))
	}
	OK(w, map[string]any{"items": items, "total": total, "page": page, "limit": limit})
}

// ExportUsersCSV GET /api/v1/admin/export/users.csv — 用户摘要 CSV（含带宽与注册渠道）。
// 按页拉取全部匹配用户（不再静默截断为 200 行）。首页失败仍可回 JSON 错误信封。
func (h *AdminHandlers) ExportUsersCSV(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	var groupID uint64
	if v := query.Get("group"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			groupID = n
		}
	}
	q, status, channel, sort := query.Get("q"), query.Get("status"), query.Get("channel"), query.Get("sort")
	rows, total, err := h.D.Adm.ListUsers(q, groupID, status, channel, sort, 1, usersMaxLimit)
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="imgli-users.csv"`)
	_, _ = w.Write([]byte("id,username,email,status,group_id,bandwidth_used_month,bandwidth_period,signup_channel,image_count\n"))
	writeUserCSV := func(batch []adminsvc.UserRow) {
		for i := range batch {
			u := batch[i].User
			line := fmt.Sprintf("%d,%s,%s,%s,%d,%d,%s,%s,%d\n",
				u.ID, csvEsc(u.Username), csvEsc(u.Email), csvEsc(u.Status), u.GroupID,
				u.BandwidthUsedMonth, csvEsc(u.BandwidthPeriod), csvEsc(u.SignupChannel), batch[i].ImageCount)
			_, _ = w.Write([]byte(line))
		}
	}
	writeUserCSV(rows)
	for page := 2; int64((page-1)*usersMaxLimit) < total; page++ {
		more, _, err := h.D.Adm.ListUsers(q, groupID, status, channel, sort, page, usersMaxLimit)
		if err != nil || len(more) == 0 {
			return
		}
		writeUserCSV(more)
	}
}

func csvEsc(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

// UpdateUser PATCH /api/v1/admin/users/{id} {group_id?,status?}
func (h *AdminHandlers) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "用户 id 无效")
		return
	}
	var req struct {
		GroupID *uint64 `json:"group_id"`
		Status  *string `json:"status"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体无效")
		return
	}
	actor := PrincipalFrom(r).User
	u, err := h.D.Adm.UpdateUser(actor.ID, id, req.GroupID, req.Status)
	switch {
	case err == nil:
		var fields []string
		if req.GroupID != nil {
			fields = append(fields, "group_id")
		}
		if req.Status != nil {
			fields = append(fields, "status")
		}
		if len(fields) > 0 {
			h.D.Adm.Audit(&actor.ID, "admin", "user_update",
				map[string]any{"target_id": id, "fields": fields}, ClientIP(r))
		}
		OK(w, adminUserDTO(u))
	case errors.Is(err, adminsvc.ErrUserNotFound):
		Fail(w, http.StatusNotFound, CodeNotFound, "用户不存在")
	case errors.Is(err, adminsvc.ErrGroupNotFound):
		Fail(w, http.StatusNotFound, CodeNotFound, "用户组不存在")
	case errors.Is(err, adminsvc.ErrSelfBan):
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "不能封禁自己")
	case errors.Is(err, adminsvc.ErrInvalidStatus):
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "status 仅支持 active|banned")
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}

// ResetPassword POST /api/v1/admin/users/{id}/reset-password
func (h *AdminHandlers) ResetPassword(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "用户 id 无效")
		return
	}
	plain, err := h.D.Adm.ResetPassword(id)
	switch {
	case err == nil:
		actor := PrincipalFrom(r).User
		h.D.Adm.Audit(&actor.ID, "admin", "user_reset_password",
			map[string]any{"target_id": id}, ClientIP(r))
		OK(w, map[string]any{"password": plain})
	case errors.Is(err, adminsvc.ErrUserNotFound):
		Fail(w, http.StatusNotFound, CodeNotFound, "用户不存在")
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}
