package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/adminsvc"
)

// adminGroupDTO 用户组基础字段（不含 user_count，供 create/update 响应复用）。
func adminGroupDTO(g *model.UserGroup) map[string]any {
	return map[string]any{
		"id": g.ID, "name": g.Name, "is_default": g.IsDefault, "is_guest": g.IsGuest,
		"storage_quota": g.StorageQuota, "max_file_size": g.MaxFileSize,
		"bandwidth_quota_month": g.BandwidthQuotaMonth,
		"rate_per_minute":       g.RatePerMinute, "rate_per_hour": g.RatePerHour, "rate_per_day": g.RatePerDay,
		"allowed_exts": g.AllowedExts, "allowed_policy_ids": g.AllowedPolicyIDs,
		"created_at": g.CreatedAt.Format(time.RFC3339),
	}
}

func groupRowDTO(row *adminsvc.GroupRow) map[string]any {
	m := adminGroupDTO(&row.Group)
	m["user_count"] = row.UserCount
	return m
}

// Groups GET /api/v1/admin/groups —— 无分页，组数量少。
func (h *AdminHandlers) Groups(w http.ResponseWriter, r *http.Request) {
	rows, err := h.D.Adm.ListGroups()
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for i := range rows {
		items = append(items, groupRowDTO(&rows[i]))
	}
	OK(w, map[string]any{"items": items})
}

// groupWriteRequest 是 CreateGroup/UpdateGroup 共享的请求体形状（PATCH 全字段可选）。
type groupWriteRequest struct {
	Name                *string   `json:"name"`
	StorageQuota        *int64    `json:"storage_quota"`
	MaxFileSize         *int64    `json:"max_file_size"`
	BandwidthQuotaMonth *int64    `json:"bandwidth_quota_month"`
	RatePerMinute       *int      `json:"rate_per_minute"`
	RatePerHour         *int      `json:"rate_per_hour"`
	RatePerDay          *int      `json:"rate_per_day"`
	AllowedExts         *[]string `json:"allowed_exts"`
	AllowedPolicyIDs    *[]uint64 `json:"allowed_policy_ids"`
}

// CreateGroup POST /api/v1/admin/groups {name,storage_quota,max_file_size,rate_per_minute,
// rate_per_hour,rate_per_day,allowed_exts,allowed_policy_ids}——不含 is_default/is_guest
// （内置标志不可经 API 设置）。成功统一 200 OK 返回创建后对象。
func (h *AdminHandlers) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var req groupWriteRequest
	if err := DecodeJSON(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体无效")
		return
	}
	g := &model.UserGroup{}
	if req.Name != nil {
		g.Name = *req.Name
	}
	if req.StorageQuota != nil {
		g.StorageQuota = *req.StorageQuota
	}
	if req.MaxFileSize != nil {
		g.MaxFileSize = *req.MaxFileSize
	}
	if req.BandwidthQuotaMonth != nil {
		g.BandwidthQuotaMonth = *req.BandwidthQuotaMonth
	}
	if req.RatePerMinute != nil {
		g.RatePerMinute = *req.RatePerMinute
	}
	if req.RatePerHour != nil {
		g.RatePerHour = *req.RatePerHour
	}
	if req.RatePerDay != nil {
		g.RatePerDay = *req.RatePerDay
	}
	if req.AllowedExts != nil {
		g.AllowedExts = *req.AllowedExts
	}
	if req.AllowedPolicyIDs != nil {
		g.AllowedPolicyIDs = *req.AllowedPolicyIDs
	}
	err := h.D.Adm.CreateGroup(g)
	switch {
	case err == nil:
		actor := PrincipalFrom(r).User
		h.D.Adm.Audit(&actor.ID, "admin", "group_create", map[string]any{"name": g.Name}, ClientIP(r))
		OK(w, adminGroupDTO(g))
	case errors.Is(err, adminsvc.ErrPolicyNotFound):
		Fail(w, http.StatusNotFound, CodeNotFound, err.Error())
	case errors.Is(err, adminsvc.ErrExtsEmpty), errors.Is(err, adminsvc.ErrGroupNameInvalid),
		errors.Is(err, adminsvc.ErrQuotaInvalid), errors.Is(err, adminsvc.ErrBandwidthQuotaInvalid):
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, err.Error())
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}

// UpdateGroup PATCH /api/v1/admin/groups/{id} —— 同 groupWriteRequest，均可选，nil 字段保持不变。
func (h *AdminHandlers) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "组 id 无效")
		return
	}
	var req groupWriteRequest
	if err := DecodeJSON(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体无效")
		return
	}
	patch := adminsvc.GroupPatch{
		Name: req.Name, StorageQuota: req.StorageQuota, MaxFileSize: req.MaxFileSize,
		BandwidthQuotaMonth: req.BandwidthQuotaMonth,
		RatePerMinute:       req.RatePerMinute, RatePerHour: req.RatePerHour, RatePerDay: req.RatePerDay,
		AllowedExts: req.AllowedExts, AllowedPolicyIDs: req.AllowedPolicyIDs,
	}
	g, err := h.D.Adm.UpdateGroup(id, patch)
	switch {
	case err == nil:
		var fields []string
		if req.Name != nil {
			fields = append(fields, "name")
		}
		if req.StorageQuota != nil {
			fields = append(fields, "storage_quota")
		}
		if req.MaxFileSize != nil {
			fields = append(fields, "max_file_size")
		}
		if req.BandwidthQuotaMonth != nil {
			fields = append(fields, "bandwidth_quota_month")
		}
		if req.RatePerMinute != nil {
			fields = append(fields, "rate_per_minute")
		}
		if req.RatePerHour != nil {
			fields = append(fields, "rate_per_hour")
		}
		if req.RatePerDay != nil {
			fields = append(fields, "rate_per_day")
		}
		if req.AllowedExts != nil {
			fields = append(fields, "allowed_exts")
		}
		if req.AllowedPolicyIDs != nil {
			fields = append(fields, "allowed_policy_ids")
		}
		actor := PrincipalFrom(r).User
		h.D.Adm.Audit(&actor.ID, "admin", "group_update", map[string]any{"id": id, "fields": fields}, ClientIP(r))
		OK(w, adminGroupDTO(g))
	case errors.Is(err, adminsvc.ErrGroupNotFound), errors.Is(err, adminsvc.ErrPolicyNotFound):
		Fail(w, http.StatusNotFound, CodeNotFound, err.Error())
	case errors.Is(err, adminsvc.ErrBuiltinGroup), errors.Is(err, adminsvc.ErrGroupInUse),
		errors.Is(err, adminsvc.ErrExtsEmpty), errors.Is(err, adminsvc.ErrGroupNameInvalid),
		errors.Is(err, adminsvc.ErrQuotaInvalid), errors.Is(err, adminsvc.ErrBandwidthQuotaInvalid):
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, err.Error())
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}

// DeleteGroup DELETE /api/v1/admin/groups/{id}——内置组/在用组不可删。
func (h *AdminHandlers) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "组 id 无效")
		return
	}
	// 先取名字供成功时 audit 用；不存在时下面 DeleteGroup 同样会报 404，此处忽略取值失败。
	existing, _ := h.D.Adm.GroupByID(id)
	err = h.D.Adm.DeleteGroup(id)
	switch {
	case err == nil:
		name := ""
		if existing != nil {
			name = existing.Name
		}
		actor := PrincipalFrom(r).User
		h.D.Adm.Audit(&actor.ID, "admin", "group_delete", map[string]any{"id": id, "name": name}, ClientIP(r))
		OK(w, map[string]any{"id": id, "deleted": true})
	case errors.Is(err, adminsvc.ErrGroupNotFound):
		Fail(w, http.StatusNotFound, CodeNotFound, err.Error())
	case errors.Is(err, adminsvc.ErrBuiltinGroup), errors.Is(err, adminsvc.ErrGroupInUse):
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, err.Error())
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}
