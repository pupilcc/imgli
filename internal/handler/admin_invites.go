package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/imgli/internal/service/adminsvc"
)

// adminInviteDTO 列表项;status 由服务层派生并随 InviteRow 下发。
func adminInviteDTO(row adminsvc.InviteRow) map[string]any {
	ic := row.Invite
	return map[string]any{
		"id": ic.ID, "code": ic.Code, "status": row.Status,
		"created_by_name": row.CreatedByName, "used_by_name": row.UsedByName,
		"created_at": ic.CreatedAt, "expires_at": ic.ExpiresAt, "used_at": ic.UsedAt,
	}
}

// Invites GET /api/v1/admin/invites?status=&page=&limit=
func (h *AdminHandlers) Invites(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	switch status {
	case "", "unused", "used", "expired":
	default:
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "status 仅支持 unused|used|expired")
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	// 与 adminsvc defaultListLimit/maxListLimit 保持同步
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, total, err := h.D.Adm.ListInvites(status, page, limit)
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	items := make([]map[string]any, len(rows))
	for i, row := range rows {
		items[i] = adminInviteDTO(row)
	}
	OK(w, map[string]any{"items": items, "total": total, "page": page, "limit": limit})
}

// CreateInvites POST /api/v1/admin/invites {count, expires_in_days?} → {codes:[...]}
func (h *AdminHandlers) CreateInvites(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Count         int `json:"count"`
		ExpiresInDays int `json:"expires_in_days"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体无效")
		return
	}
	actor := PrincipalFrom(r).User
	codes, err := h.D.Adm.CreateInvites(actor.ID, req.Count, req.ExpiresInDays)
	switch {
	case err == nil:
		h.D.Adm.Audit(&actor.ID, "admin", "invite_create",
			map[string]any{"count": req.Count, "expires_in_days": req.ExpiresInDays}, ClientIP(r))
		OK(w, map[string]any{"codes": codes})
	case errors.Is(err, adminsvc.ErrInviteCountInvalid):
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, err.Error())
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}

// RevokeInvite DELETE /api/v1/admin/invites/{id} —— 仅未用码。
func (h *AdminHandlers) RevokeInvite(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "邀请码 id 无效")
		return
	}
	code, rerr := h.D.Adm.RevokeInvite(id)
	switch {
	case rerr == nil:
		actor := PrincipalFrom(r).User
		h.D.Adm.Audit(&actor.ID, "admin", "invite_revoke", map[string]any{"id": id, "code": code}, ClientIP(r))
		OK(w, map[string]any{})
	case errors.Is(rerr, adminsvc.ErrInviteNotFound):
		Fail(w, http.StatusNotFound, CodeNotFound, rerr.Error())
	case errors.Is(rerr, adminsvc.ErrInviteUsed):
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, rerr.Error())
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}
