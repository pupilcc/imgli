package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/imgli/internal/linkbuilder"
	"github.com/yixian-huang/imgli/internal/mail"
	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/adminsvc"
	"github.com/yixian-huang/imgli/internal/service/moderation"
	"github.com/yixian-huang/imgli/internal/service/settings"
	"github.com/yixian-huang/imgli/internal/service/stats"
	"github.com/yixian-huang/imgli/internal/service/storagesvc"
	"github.com/yixian-huang/imgli/internal/service/upload"
	"github.com/yixian-huang/imgli/internal/service/webhook"
)

// AdminDeps 管理端 handler 依赖。
type AdminDeps struct {
	Adm     *adminsvc.Service
	Res     *storagesvc.Resolver
	Mail    *mail.Service
	Stats   *stats.Service
	Mod     *moderation.Service // 可选；拒绝通知
	Hooks   *webhook.Service   // 可选；出站 webhook
	OwnHost string             // BaseURL host，用于 referer suspect 排除自站
}

type AdminHandlers struct{ D AdminDeps }

// Stats GET /api/v1/admin/stats
func (h *AdminHandlers) Stats(w http.ResponseWriter, r *http.Request) {
	st, err := h.D.Adm.StatsWithOwnHost(h.D.OwnHost)
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	OK(w, st)
}

// RefererImages GET /api/v1/admin/referers/images?host=&days=&limit=
func (h *AdminHandlers) RefererImages(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	if host == "" {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "host 必填")
		return
	}
	days := 30
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			days = n
		}
	}
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	rows, err := h.D.Adm.TopImagesByRefererHost(host, days, limit)
	if err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, err.Error())
		return
	}
	OK(w, map[string]any{"host": host, "items": rows})
}

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
	page := usersDefaultPage
	if v := query.Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	limit := usersDefaultLimit
	if v := query.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > usersMaxLimit {
		limit = usersMaxLimit
	}
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
func (h *AdminHandlers) ExportUsersCSV(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	var groupID uint64
	if v := query.Get("group"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			groupID = n
		}
	}
	rows, _, err := h.D.Adm.ListUsers(query.Get("q"), groupID, query.Get("status"), query.Get("channel"), query.Get("sort"), 1, usersMaxLimit)
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="imgli-users.csv"`)
	_, _ = w.Write([]byte("id,username,email,status,group_id,bandwidth_used_month,bandwidth_period,signup_channel,image_count\n"))
	for i := range rows {
		u := rows[i].User
		line := fmt.Sprintf("%d,%s,%s,%s,%d,%d,%s,%s,%d\n",
			u.ID, csvEsc(u.Username), csvEsc(u.Email), csvEsc(u.Status), u.GroupID,
			u.BandwidthUsedMonth, csvEsc(u.BandwidthPeriod), csvEsc(u.SignupChannel), rows[i].ImageCount)
		_, _ = w.Write([]byte(line))
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

const (
	imagesDefaultPage  = 1
	imagesDefaultLimit = 50
	imagesMaxLimit     = 200
)

// adminImageItemDTO 全站图片列表/详情项（含 links，同②c imageItemDTO 模式）。
func (h *AdminHandlers) adminImageItemDTO(row *adminsvc.ImageRow) map[string]any {
	base := h.D.Res.LinkBase(&row.Policy)
	links := linkbuilder.Build(base, row.Img.Key, row.Img.Ext, row.Img.Name)
	return map[string]any{
		"key":            row.Img.Key,
		"name":           row.Img.Name,
		"ext":            row.Img.Ext,
		"size":           row.File.Size,
		"visibility":     row.Img.Visibility,
		"status":         row.Img.Status,
		"is_whitelisted": row.Img.IsWhitelisted,
		"nsfw_score":     row.Img.NSFWScore,
		"username":       row.Username,
		"user_id":        row.Img.UserID,
		"created_at":     row.Img.CreatedAt.Format(time.RFC3339),
		"links":          links,
	}
}

// Images GET /api/v1/admin/images?user=&status=&policy=&page=&limit=
func (h *AdminHandlers) Images(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page := imagesDefaultPage
	if v := query.Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	limit := imagesDefaultLimit
	if v := query.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > imagesMaxLimit {
		limit = imagesMaxLimit
	}
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
	items := make([]map[string]any, 0, len(rows))
	for i := range rows {
		items = append(items, h.adminImageItemDTO(&rows[i]))
	}
	OK(w, map[string]any{"items": items, "total": total, "page": page, "limit": limit})
}

// reviewAuditAction 把审核 action 映射为 audit 落库的动词（approve→review_approve，
// reject→review_reject）。调用处均已确认 action 合法（Decide/DecideBatch 校验先于此），
// 故未识别值兜底落 review_reject 只是防御性写法，正常路径不会触达。
func reviewAuditAction(action string) string {
	if action == "approve" {
		return "review_approve"
	}
	return "review_reject"
}

// Review GET /api/v1/admin/review?page=&limit= —— 待审队列（status=pending）。
func (h *AdminHandlers) Review(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page := imagesDefaultPage
	if v := query.Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	limit := imagesDefaultLimit
	if v := query.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > imagesMaxLimit {
		limit = imagesMaxLimit
	}
	rows, total, err := h.D.Adm.ListReview(page, limit)
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	keys := make([]string, 0, len(rows))
	for i := range rows {
		keys = append(keys, rows[i].Img.Key)
	}
	// best-effort：触发原因查失败仍返回队列本身，不 500。
	triggers, _ := h.D.Adm.ModerationTriggersByKeys(keys)
	items := make([]map[string]any, 0, len(rows))
	for i := range rows {
		item := h.adminImageItemDTO(&rows[i])
		if trigs, ok := triggers[rows[i].Img.Key]; ok && len(trigs) > 0 {
			item["triggers"] = trigs
		}
		items = append(items, item)
	}
	OK(w, map[string]any{"items": items, "total": total, "page": page, "limit": limit})
}

// ReviewDecide POST /api/v1/admin/review/{key} {action:"approve"|"reject"} —— 更新后返回
// item DTO（同 /admin/images 的 DTO）。成功落 audit review_approve|review_reject，
// detail {key, score}（score 取裁决后 img.NSFWScore，可为 null）。
func (h *AdminHandlers) ReviewDecide(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	var req struct {
		Action string `json:"action"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体无效")
		return
	}
	img, err := h.D.Adm.Decide(key, req.Action)
	switch {
	case err == nil:
		actor := PrincipalFrom(r).User
		h.D.Adm.Audit(&actor.ID, "admin", reviewAuditAction(req.Action),
			map[string]any{"key": key, "score": img.NSFWScore}, ClientIP(r))
		if req.Action == "reject" && h.D.Mod != nil {
			h.D.Mod.NotifyRejectIfConfigured(*img)
		}
		if h.D.Hooks != nil {
			h.D.Hooks.Emit("image.moderated", map[string]any{
				"key": img.Key, "status": img.Status, "action": req.Action,
			})
		}
		row, rerr := h.D.Adm.GetImageRow(key)
		if rerr != nil {
			// 裁决已提交且已审计；图在这次补查的窗口内被并发软删（GetImageRow 默认
			// scope 查不到）——不得因这次读失败把已经成功、已经审计的操作报成 500，
			// 降级返回最小成功体（img 是 Decide 已返回的裁决后状态）。
			OK(w, map[string]any{"key": img.Key, "status": img.Status})
			return
		}
		OK(w, h.adminImageItemDTO(row))
	case errors.Is(err, adminsvc.ErrImageNotFound):
		Fail(w, http.StatusNotFound, CodeNotFound, "图片不存在")
	case errors.Is(err, adminsvc.ErrInvalidAction), errors.Is(err, adminsvc.ErrNotPending):
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, err.Error())
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}

// ReviewBatch POST /api/v1/admin/review/batch {keys,action} → {results}。上限 100
// （超限 400）；action 非法整体 400；否则逐项裁决部分成功，每个成功项各落一条 audit。
func (h *AdminHandlers) ReviewBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Keys   []string `json:"keys"`
		Action string   `json:"action"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体无效")
		return
	}
	results, err := h.D.Adm.DecideBatch(req.Keys, req.Action)
	switch {
	case err == nil:
		actor := PrincipalFrom(r).User
		auditAction := reviewAuditAction(req.Action)
		for _, res := range results {
			if !res.OK {
				continue
			}
			score, _ := h.D.Adm.NSFWScoreByKey(res.Key)
			h.D.Adm.Audit(&actor.ID, "admin", auditAction,
				map[string]any{"key": res.Key, "score": score}, ClientIP(r))
			if req.Action == "reject" && h.D.Mod != nil {
				if row, rerr := h.D.Adm.GetImageRow(res.Key); rerr == nil {
					h.D.Mod.NotifyRejectIfConfigured(row.Img)
				}
			}
		}
		OK(w, map[string]any{"results": results})
	case errors.Is(err, adminsvc.ErrInvalidAction), errors.Is(err, adminsvc.ErrTooManyKeys):
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, err.Error())
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}

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

// adminPolicyDTO 存储策略字段。config 为 JSON 编码字符串；s3 的 secret_access_key、
// webdav 的 password 已打码。
func adminPolicyDTO(p *model.StoragePolicy) map[string]any {
	cfg := p.Config
	if p.Driver == "s3" && cfg["secret_access_key"] != "" {
		masked := make(map[string]string, len(cfg))
		for k, v := range cfg {
			masked[k] = v
		}
		masked["secret_access_key"] = adminsvc.MaskSecret(cfg["secret_access_key"])
		cfg = masked
	}
	if p.Driver == "webdav" && cfg["password"] != "" {
		masked := make(map[string]string, len(cfg))
		for k, v := range cfg {
			masked[k] = v
		}
		masked["password"] = adminsvc.MaskSecret(cfg["password"])
		cfg = masked
	}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		cfgJSON = []byte("{}")
	}
	return map[string]any{
		"id": p.ID, "name": p.Name, "driver": p.Driver,
		"config": string(cfgJSON), "cdn_domain": p.CDNDomain,
		"path_template": p.PathTemplate, "enabled": p.Enabled,
		"created_at": p.CreatedAt.Format(time.RFC3339),
	}
}

func policyRowDTO(row *adminsvc.PolicyRow) map[string]any {
	m := adminPolicyDTO(&row.Policy)
	m["file_count"] = row.FileCount
	m["used_bytes"] = row.UsedBytes
	return m
}

// Policies GET /api/v1/admin/policies —— 无分页，策略数量少。
func (h *AdminHandlers) Policies(w http.ResponseWriter, r *http.Request) {
	rows, err := h.D.Adm.ListPolicies()
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for i := range rows {
		items = append(items, policyRowDTO(&rows[i]))
	}
	OK(w, map[string]any{"items": items})
}

// policyCreateRequest POST /admin/policies 请求体：config 为 JSON 编码的字符串
// （如 `"{\"root\":\"/data\"}"`），与 GET 响应的 config 字段编码方式一致。
type policyCreateRequest struct {
	Name         string `json:"name"`
	Driver       string `json:"driver"`
	Config       string `json:"config"`
	CDNDomain    string `json:"cdn_domain"`
	PathTemplate string `json:"path_template"`
	Enabled      *bool  `json:"enabled"`
}

// CreatePolicy POST /api/v1/admin/policies {name,driver,config,cdn_domain,path_template,enabled}。
func (h *AdminHandlers) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	var req policyCreateRequest
	if err := DecodeJSON(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体无效")
		return
	}
	var cfg map[string]string
	if req.Config != "" {
		if err := json.Unmarshal([]byte(req.Config), &cfg); err != nil {
			Fail(w, http.StatusBadRequest, CodeInvalidRequest, "config 不是合法 JSON")
			return
		}
	}
	p := &model.StoragePolicy{
		Name: req.Name, Driver: req.Driver, Config: cfg,
		CDNDomain: req.CDNDomain, PathTemplate: req.PathTemplate, Enabled: true,
	}
	if req.Enabled != nil {
		p.Enabled = *req.Enabled
	}
	err := h.D.Adm.CreatePolicy(p)
	switch {
	case err == nil:
		actor := PrincipalFrom(r).User
		h.D.Adm.Audit(&actor.ID, "admin", "policy_create",
			map[string]any{"name": p.Name, "driver": p.Driver}, ClientIP(r))
		OK(w, adminPolicyDTO(p))
	case errors.Is(err, adminsvc.ErrDriverUnsupported), errors.Is(err, adminsvc.ErrBadConfig), errors.Is(err, adminsvc.ErrPolicyNameInvalid):
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, err.Error())
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}

// policyPatchRequest PATCH /admin/policies/{id} 请求体，形状与 adminsvc.PolicyPatch 一致
// （不含 driver——换驱动=建新策略）。
type policyPatchRequest struct {
	Name         *string `json:"name"`
	Config       *string `json:"config"`
	CDNDomain    *string `json:"cdn_domain"`
	PathTemplate *string `json:"path_template"`
	Enabled      *bool   `json:"enabled"`
}

// UpdatePolicy PATCH /api/v1/admin/policies/{id}
func (h *AdminHandlers) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "策略 id 无效")
		return
	}
	var req policyPatchRequest
	if err := DecodeJSON(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体无效")
		return
	}
	patch := adminsvc.PolicyPatch{
		Name: req.Name, Config: req.Config, CDNDomain: req.CDNDomain,
		PathTemplate: req.PathTemplate, Enabled: req.Enabled,
	}
	p, err := h.D.Adm.UpdatePolicy(id, patch)
	switch {
	case err == nil:
		var fields []string
		if req.Name != nil {
			fields = append(fields, "name")
		}
		if req.Config != nil {
			fields = append(fields, "config")
		}
		if req.CDNDomain != nil {
			fields = append(fields, "cdn_domain")
		}
		if req.PathTemplate != nil {
			fields = append(fields, "path_template")
		}
		if req.Enabled != nil {
			fields = append(fields, "enabled")
		}
		actor := PrincipalFrom(r).User
		h.D.Adm.Audit(&actor.ID, "admin", "policy_update", map[string]any{"id": id, "fields": fields}, ClientIP(r))
		OK(w, adminPolicyDTO(p))
	case errors.Is(err, adminsvc.ErrPolicyNotFound):
		Fail(w, http.StatusNotFound, CodeNotFound, err.Error())
	case errors.Is(err, adminsvc.ErrBadConfig), errors.Is(err, adminsvc.ErrPolicyNameInvalid):
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, err.Error())
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}

// DeletePolicy DELETE /api/v1/admin/policies/{id} —— 仍被 files 引用的策略不可删除。
func (h *AdminHandlers) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "策略 id 无效")
		return
	}
	// 先取名字供成功时 audit 用；不存在时下面 DeletePolicy 同样会报 404，此处忽略取值失败。
	existing, _ := h.D.Adm.PolicyByID(id)
	err = h.D.Adm.DeletePolicy(id)
	switch {
	case err == nil:
		name := ""
		if existing != nil {
			name = existing.Name
		}
		actor := PrincipalFrom(r).User
		h.D.Adm.Audit(&actor.ID, "admin", "policy_delete", map[string]any{"id": id, "name": name}, ClientIP(r))
		OK(w, map[string]any{"id": id, "deleted": true})
	case errors.Is(err, adminsvc.ErrPolicyNotFound):
		Fail(w, http.StatusNotFound, CodeNotFound, err.Error())
	case errors.Is(err, adminsvc.ErrPolicyInUse), errors.Is(err, adminsvc.ErrPolicyInUseByGroup):
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, err.Error())
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}

// TestPolicyConn POST /api/v1/admin/policies/{id}/test —— local/s3 写/读/删探针，
// 返回耗时 ms。不存在策略 404；存在但探测失败（driver 不支持/config 坏/root 不可写等）400。
func (h *AdminHandlers) TestPolicyConn(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "策略 id 无效")
		return
	}
	latency, terr := h.D.Adm.TestPolicy(id)
	if errors.Is(terr, adminsvc.ErrPolicyNotFound) {
		Fail(w, http.StatusNotFound, CodeNotFound, terr.Error())
		return
	}
	ok := terr == nil
	actor := PrincipalFrom(r).User
	h.D.Adm.Audit(&actor.ID, "admin", "policy_test", map[string]any{"id": id, "ok": ok}, ClientIP(r))
	if !ok {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, terr.Error())
		return
	}
	OK(w, map[string]any{"ok": true, "latency_ms": latency})
}

// GetSettings GET /api/v1/admin/settings → {site_name, registration_mode, moderation:{...}}
// （moderation.api_key 打码，见 adminsvc.GetSettings）。
func (h *AdminHandlers) GetSettings(w http.ResponseWriter, r *http.Request) {
	m, err := h.D.Adm.GetSettings()
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	OK(w, m)
}

// PutSettings PATCH 语义的部分更新（HTTP 方法用 PUT，与路由契约一致）。请求体是
// {site_name?, registration_mode?, guest_upload_enabled?, plaza_enabled?, moderation?, smtp?, hotlink?, processing?} 的任意子集；
// 逐键校验，任一键失败整个请求 400 且不落库。成功后 audit `settings_update`——detail
// 只含变更键名列表，不含任何值（api_key/password 防泄露：即便是打码后的值也不落 audit）。
func (h *AdminHandlers) PutSettings(w http.ResponseWriter, r *http.Request) {
	var patch map[string]json.RawMessage
	if err := DecodeJSON(r, &patch); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体无效")
		return
	}
	err := h.D.Adm.PutSettings(patch)
	switch {
	case err == nil:
		fields := make([]string, 0, len(patch))
		for k := range patch {
			fields = append(fields, k)
		}
		sort.Strings(fields)
		actor := PrincipalFrom(r).User
		h.D.Adm.Audit(&actor.ID, "admin", "settings_update", map[string]any{"fields": fields}, ClientIP(r))
		if _, ok := patch[model.SettingHotlink]; ok && h.D.Stats != nil {
			h.D.Stats.InvalidateHotlink() // 保存后快照即时生效(单实例语义;多实例陈旧 ≤30s)
		}
		m, gerr := h.D.Adm.GetSettings()
		if gerr != nil {
			Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
			return
		}
		OK(w, m)
	case errors.Is(err, adminsvc.ErrUnknownSetting), errors.Is(err, adminsvc.ErrSiteNameInvalid),
		errors.Is(err, adminsvc.ErrRegistrationModeInvalid), errors.Is(err, adminsvc.ErrGuestUploadInvalid),
		errors.Is(err, adminsvc.ErrPlazaEnabledInvalid),
		errors.Is(err, adminsvc.ErrModerationInvalid), errors.Is(err, adminsvc.ErrSMTPInvalid),
		errors.Is(err, adminsvc.ErrHotlinkDomainInvalid),
		errors.Is(err, adminsvc.ErrAnnouncementInvalid),
		errors.Is(err, adminsvc.ErrFooterInvalid),
		errors.Is(err, adminsvc.ErrHTMLInjectInvalid),
		errors.Is(err, upload.ErrProcessingInvalid),
		errors.Is(err, moderation.ErrThresholdRange), errors.Is(err, moderation.ErrActionInvalid),
		errors.Is(err, moderation.ErrProviderInvalid), errors.Is(err, moderation.ErrEndpointInvalid),
		errors.Is(err, moderation.ErrCredentialMissing),
		errors.Is(err, moderation.ErrOCRKeywordsInvalid),
		errors.Is(err, moderation.ErrLoginSampleRate),
		errors.Is(err, moderation.ErrOnPluginError): // 客户端校验错→400 而非 500
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, err.Error())
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}

const (
	logsDefaultPage  = 1
	logsDefaultLimit = 50
	logsMaxLimit     = 200
)

// auditLogDTO 审计日志列表项。
func auditLogDTO(log *model.AuditLog) map[string]any {
	actorID := any(nil)
	if log.ActorID != nil {
		actorID = *log.ActorID
	}
	return map[string]any{
		"id":         log.ID,
		"actor_id":   actorID,
		"actor_type": log.ActorType,
		"action":     log.Action,
		"detail":     log.Detail, // 原样 JSON 字符串透传
		"ip":         log.IP,
		"created_at": log.CreatedAt.Format(time.RFC3339),
	}
}

// Logs GET /api/v1/admin/logs?action=&actor_type=&page=&limit=
func (h *AdminHandlers) Logs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page := logsDefaultPage
	if v := query.Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	limit := logsDefaultLimit
	if v := query.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > logsMaxLimit {
		limit = logsMaxLimit
	}

	action := query.Get("action")
	actorType := query.Get("actor_type")

	logs, total, err := h.D.Adm.ListLogs(action, actorType, page, limit)
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}

	items := make([]map[string]any, 0, len(logs))
	for i := range logs {
		items = append(items, auditLogDTO(&logs[i]))
	}
	OK(w, map[string]any{"items": items, "total": total, "page": page, "limit": limit})
}

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
