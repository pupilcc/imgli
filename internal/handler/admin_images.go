package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/imgli/internal/service/adminsvc"
	"github.com/yixian-huang/imgli/internal/service/imagesvc"
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

// Images GET /api/v1/admin/images?user=&status=&policy=&deleted=&page=&limit=
// deleted: 空|live=在线；trash=回收站；all=全部。
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
	deleted := query.Get("deleted")
	switch deleted {
	case "", "live", "trash", "all":
	default:
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "deleted 仅支持 live|trash|all")
		return
	}
	rows, total, err := h.D.Adm.ListImages(userID, status, policyID, deleted, page, limit)
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

// ImagesBatch POST /api/v1/admin/images/batch {keys, action: trash|purge|restore} → {results}。
// 上限 100；逐项处理部分成功。trash=软删（游客无回收站时升格 purge）；purge=彻底删除；restore=从回收站恢复。
func (h *AdminHandlers) ImagesBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Keys   []string `json:"keys"`
		Action string   `json:"action"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体无效")
		return
	}
	if req.Action != "trash" && req.Action != "purge" && req.Action != "restore" {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "action 仅支持 trash|purge|restore")
		return
	}
	if len(req.Keys) == 0 {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "keys 不能为空")
		return
	}
	if len(req.Keys) > 100 {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, adminsvc.ErrTooManyKeys.Error())
		return
	}
	actor := PrincipalFrom(r).User
	type item struct {
		Key            string `json:"key"`
		OK             bool   `json:"ok"`
		Error          string `json:"error,omitempty"`
		Permanent      bool   `json:"permanent,omitempty"`
		PhysicalQueued bool   `json:"physical_queued,omitempty"`
		ObjectRetained bool   `json:"object_retained,omitempty"`
	}
	out := make([]item, 0, len(req.Keys))
	for _, key := range req.Keys {
		key = strings.TrimSpace(key)
		if key == "" {
			out = append(out, item{Key: key, OK: false, Error: "empty key"})
			continue
		}
		if req.Action == "restore" {
			img, err := h.D.Adm.AdminRestore(key)
			if err != nil {
				msg := "not found"
				if !errors.Is(err, adminsvc.ErrImageNotFound) {
					msg = "error"
				}
				out = append(out, item{Key: key, OK: false, Error: msg})
				continue
			}
			h.D.Adm.Audit(&actor.ID, "admin", "image_admin_restore",
				map[string]any{"key": key, "owner_id": img.UserID, "batch": true}, ClientIP(r))
			out = append(out, item{Key: key, OK: true})
			continue
		}
		permanent := req.Action == "purge"
		if !permanent && h.D.Img != nil {
			if row, err := h.D.Adm.GetImageRow(key); err == nil && row.Img.UserID == nil {
				permanent = true
			}
		}
		if permanent {
			if h.D.Img == nil {
				out = append(out, item{Key: key, OK: false, Error: "internal"})
				continue
			}
			res, err := h.D.Img.AdminPurge(key)
			if err != nil {
				msg := "not found"
				if !errors.Is(err, imagesvc.ErrNotFound) {
					msg = "error"
				}
				out = append(out, item{Key: key, OK: false, Error: msg})
				continue
			}
			h.D.Adm.Audit(&actor.ID, "admin", "image_admin_purge",
				map[string]any{
					"key": key, "permanent": true, "batch": true,
					"owner_id": res.OwnerID, "physical_queued": res.PhysicalQueued,
					"object_retained": res.ObjectRetained,
				}, ClientIP(r))
			out = append(out, item{
				Key: key, OK: true, Permanent: true,
				PhysicalQueued: res.PhysicalQueued, ObjectRetained: res.ObjectRetained,
			})
			continue
		}
		img, err := h.D.Adm.AdminSoftDelete(key)
		if err != nil {
			msg := "not found"
			if !errors.Is(err, adminsvc.ErrImageNotFound) {
				msg = "error"
			}
			out = append(out, item{Key: key, OK: false, Error: msg})
			continue
		}
		h.D.Adm.Audit(&actor.ID, "admin", "image_admin_delete",
			map[string]any{"key": key, "owner_id": img.UserID, "permanent": false, "batch": true}, ClientIP(r))
		out = append(out, item{Key: key, OK: true, Permanent: false})
	}
	OK(w, map[string]any{"results": out})
}

// RestoreImage POST /api/v1/admin/images/{key}/restore
// 从回收站恢复（清 deleted_at）；未软删/不存在 → 404。
func (h *AdminHandlers) RestoreImage(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	actor := PrincipalFrom(r).User
	img, err := h.D.Adm.AdminRestore(key)
	switch {
	case err == nil:
		h.D.Adm.Audit(&actor.ID, "admin", "image_admin_restore",
			map[string]any{"key": key, "owner_id": img.UserID}, ClientIP(r))
		OK(w, map[string]any{"key": key, "restored": true})
	case errors.Is(err, adminsvc.ErrImageNotFound):
		Fail(w, http.StatusNotFound, CodeNotFound, "图片不存在或未在回收站")
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}

// DeleteImage DELETE /api/v1/admin/images/{key}[?permanent=1]
//
// 默认软删进属主回收站（原直链 410，物理对象仍保留，30 天内可恢复）。
// 游客图（无属主）没有回收站，默认即彻底删除。
// permanent=1|true：彻底删除（硬删 DB + 引用归零后投递 delete_file 清存储）。
func (h *AdminHandlers) DeleteImage(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	permanent := r.URL.Query().Get("permanent") == "1" || r.URL.Query().Get("permanent") == "true"
	actor := PrincipalFrom(r).User

	// 游客 live 图：无属主回收站，强制彻底删除（即使未传 permanent）。
	if !permanent && h.D.Img != nil {
		if row, err := h.D.Adm.GetImageRow(key); err == nil && row.Img.UserID == nil {
			permanent = true
		}
	}

	if permanent {
		if h.D.Img == nil {
			Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
			return
		}
		res, err := h.D.Img.AdminPurge(key)
		switch {
		case err == nil:
			h.D.Adm.Audit(&actor.ID, "admin", "image_admin_purge",
				map[string]any{
					"key":             key,
					"permanent":       true,
					"owner_id":        res.OwnerID,
					"policy_id":       res.PolicyID,
					"path":            res.Path,
					"physical_queued": res.PhysicalQueued,
					"object_retained": res.ObjectRetained,
				}, ClientIP(r))
			OK(w, map[string]any{
				"key":             key,
				"deleted":         true,
				"permanent":       true,
				"physical_queued": res.PhysicalQueued,
				"object_retained": res.ObjectRetained,
				"policy_id":       res.PolicyID,
				"path":            res.Path,
			})
		case errors.Is(err, imagesvc.ErrNotFound):
			Fail(w, http.StatusNotFound, CodeNotFound, "图片不存在")
		default:
			Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		}
		return
	}

	img, err := h.D.Adm.AdminSoftDelete(key)
	switch {
	case err == nil:
		h.D.Adm.Audit(&actor.ID, "admin", "image_admin_delete",
			map[string]any{"key": key, "owner_id": img.UserID, "permanent": false}, ClientIP(r))
		OK(w, map[string]any{"key": key, "deleted": true, "permanent": false})
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
