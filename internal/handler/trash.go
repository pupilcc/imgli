package handler

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/imgli/internal/service/imagesvc"
	"github.com/yixian-huang/imgli/internal/service/storagesvc"
)

// TrashRetentionDays 回收站保留天数（spec 明文 30 天）。
const TrashRetentionDays = 30

type TrashDeps struct {
	Img *imagesvc.Service
	Res *storagesvc.Resolver
}

type TrashHandlers struct{ D TrashDeps }

func trashItemDTO(res *storagesvc.Resolver, row *imagesvc.Row) map[string]any {
	// deleted_at 存在性由 TrashList 保证
	deletedAt := row.Img.DeletedAt.Time
	elapsed := time.Since(deletedAt).Hours() / 24
	daysLeft := int(math.Ceil(float64(TrashRetentionDays) - elapsed))
	if daysLeft < 0 {
		daysLeft = 0
	}
	return map[string]any{
		"key":        row.Img.Key,
		"name":       row.Img.Name,
		"ext":        row.Img.Ext,
		"size":       row.File.Size,
		"width":      row.File.Width,
		"height":     row.File.Height,
		"deleted_at": deletedAt.Format(time.RFC3339),
		"days_left":  daysLeft,
	}
}

// List GET /api/v1/trash
func (h *TrashHandlers) List(w http.ResponseWriter, r *http.Request) {
	limit := 24
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 100 {
		limit = 100
	}
	rows, next, err := h.D.Img.TrashList(PrincipalFrom(r).User.ID, r.URL.Query().Get("cursor"), limit)
	if err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "回收站参数无效")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for i := range rows {
		items = append(items, trashItemDTO(h.D.Res, &rows[i]))
	}
	OK(w, map[string]any{"items": items, "next_cursor": next})
}

// Restore POST /api/v1/trash/{key}/restore
func (h *TrashHandlers) Restore(w http.ResponseWriter, r *http.Request) {
	err := h.D.Img.Restore(PrincipalFrom(r).User.ID, chi.URLParam(r, "key"))
	switch {
	case err == nil:
		OK(w, map[string]any{"key": chi.URLParam(r, "key"), "restored": true})
	case errors.Is(err, imagesvc.ErrNotFound):
		Fail(w, http.StatusNotFound, CodeNotFound, "回收站中无此图片")
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}

// Purge DELETE /api/v1/trash/{key} —— 彻底删除。
func (h *TrashHandlers) Purge(w http.ResponseWriter, r *http.Request) {
	err := h.D.Img.PurgePermanent(PrincipalFrom(r).User.ID, chi.URLParam(r, "key"))
	switch {
	case err == nil:
		OK(w, map[string]any{"key": chi.URLParam(r, "key"), "purged": true})
	case errors.Is(err, imagesvc.ErrNotFound):
		Fail(w, http.StatusNotFound, CodeNotFound, "回收站中无此图片")
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}

// Empty DELETE /api/v1/trash —— 清空回收站。
func (h *TrashHandlers) Empty(w http.ResponseWriter, r *http.Request) {
	n, err := h.D.Img.EmptyTrash(PrincipalFrom(r).User.ID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	OK(w, map[string]any{"purged": n})
}
