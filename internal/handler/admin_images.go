package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/imgli/internal/service/adminsvc"
)

const (
	imagesDefaultPage  = 1
	imagesDefaultLimit = 50
	imagesMaxLimit     = 200
)

// adminImageItemDTO 全站图片列表/详情项（类型化 DTO）。
func (h *AdminHandlers) adminImageItemDTO(row *adminsvc.ImageRow) AdminImageItemDTO {
	return adminImageItemDTOFrom(row, h.D.Res.LinkBase(&row.Policy))
}

// Images GET /api/v1/admin/images?user=&status=&policy=&page=&limit=
func (h *AdminHandlers) Images(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page, limit := ParsePage(r, imagesDefaultPage, imagesDefaultLimit, imagesMaxLimit)
	var userID uint64
	if v := query.Get("user"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			userID = n
		}
	}
	status := query.Get("status")
	switch status {
	case "", "normal", "pending", "rejected":
	default:
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "status 仅支持 normal|pending|rejected")
		return
	}
	var policyID uint64
	if v := query.Get("policy"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			policyID = n
		}
	}
	rows, total, err := h.D.Adm.ListImages(userID, status, policyID, page, limit)
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	items := make([]AdminImageItemDTO, 0, len(rows))
	for i := range rows {
		items = append(items, h.adminImageItemDTO(&rows[i]))
	}
	OK(w, map[string]any{"items": items, "total": total, "page": page, "limit": limit})
}

// DeleteImage DELETE /api/v1/admin/images/{key} —— 软删进属主回收站，原直链转 410。
func (h *AdminHandlers) DeleteImage(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	img, err := h.D.Adm.AdminSoftDelete(key)
	switch {
	case err == nil:
		actor := PrincipalFrom(r).User
		h.D.Adm.Audit(&actor.ID, "admin", "image_admin_delete",
			map[string]any{"key": key, "owner_id": img.UserID}, ClientIP(r))
		OK(w, map[string]any{"key": key, "deleted": true})
	case errors.Is(err, adminsvc.ErrImageNotFound):
		Fail(w, http.StatusNotFound, CodeNotFound, "图片不存在")
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}

// UpdateImageWhitelist PATCH /api/v1/admin/images/{key} {is_whitelisted:bool}
func (h *AdminHandlers) UpdateImageWhitelist(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	var req struct {
		IsWhitelisted *bool `json:"is_whitelisted"`
	}
	if err := DecodeJSON(r, &req); err != nil || req.IsWhitelisted == nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体无效")
		return
	}
	img, err := h.D.Adm.SetWhitelist(key, *req.IsWhitelisted)
	switch {
	case err == nil:
		actor := PrincipalFrom(r).User
		h.D.Adm.Audit(&actor.ID, "admin", "image_whitelist",
			map[string]any{"key": key, "on": *req.IsWhitelisted}, ClientIP(r))
		row, rerr := h.D.Adm.GetImageRow(key)
		if rerr != nil {
			// 加白已提交且已审计；图在这次补查的窗口内被并发软删（GetImageRow 默认
			// scope 查不到）——同 ReviewDecide：不得因这次读失败把已经成功、已经
			// 审计的操作报成 500，降级返回最小成功体（img 是 SetWhitelist 已返回的
			// 加白后状态）。
			OK(w, map[string]any{"key": img.Key, "status": img.Status, "is_whitelisted": img.IsWhitelisted})
			return
		}
		OK(w, h.adminImageItemDTO(row))
	case errors.Is(err, adminsvc.ErrImageNotFound):
		Fail(w, http.StatusNotFound, CodeNotFound, "图片不存在")
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}
