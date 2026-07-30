package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/adminsvc"
)

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
	page, limit := ParsePage(r, imagesDefaultPage, imagesDefaultLimit, imagesMaxLimit)
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
// 成功项的 score/拒审通知用批量查询，避免 per-key N+1。
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
		okKeys := make([]string, 0, len(results))
		for _, res := range results {
			if res.OK {
				okKeys = append(okKeys, res.Key)
			}
		}
		scores, _ := h.D.Adm.NSFWScoresByKeys(okKeys) // best-effort；失败则 score=nil
		var imgs map[string]model.Image
		if req.Action == "reject" && h.D.Mod != nil && len(okKeys) > 0 {
			imgs, _ = h.D.Adm.ImagesByKeys(okKeys)
		}
		for _, res := range results {
			if !res.OK {
				continue
			}
			var score *float64
			if scores != nil {
				score = scores[res.Key]
			}
			h.D.Adm.Audit(&actor.ID, "admin", auditAction,
				map[string]any{"key": res.Key, "score": score}, ClientIP(r))
			if req.Action == "reject" && h.D.Mod != nil && imgs != nil {
				if img, ok := imgs[res.Key]; ok {
					h.D.Mod.NotifyRejectIfConfigured(img)
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
