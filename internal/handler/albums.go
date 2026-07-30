package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/imgli/internal/service/albumsvc"
)

type AlbumDeps struct{ Alb *albumsvc.Service }

type AlbumHandlers struct{ D AlbumDeps }

func albumViewDTO(v *albumsvc.AlbumView) map[string]any {
	return map[string]any{
		"id":          v.Album.ID,
		"name":        v.Album.Name,
		"visibility":  v.Album.Visibility,
		"image_count": v.Count,
		"cover_key":   v.CoverKey,
		"created_at":  v.Album.CreatedAt.Format(time.RFC3339),
	}
}

func (h *AlbumHandlers) List(w http.ResponseWriter, r *http.Request) {
	views, err := h.D.Alb.List(PrincipalFrom(r).User.ID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	items := make([]map[string]any, 0, len(views))
	for i := range views {
		items = append(items, albumViewDTO(&views[i]))
	}
	OK(w, map[string]any{"items": items})
}

func (h *AlbumHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		Visibility string `json:"visibility"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体无效")
		return
	}
	alb, err := h.D.Alb.Create(PrincipalFrom(r).User.ID, req.Name, req.Visibility)
	switch {
	case err == nil:
		OK(w, map[string]any{"id": alb.ID, "name": alb.Name, "visibility": alb.Visibility})
	case errors.Is(err, albumsvc.ErrInvalidName), errors.Is(err, albumsvc.ErrInvalidVisibility):
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, err.Error())
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}

func albumIDParam(r *http.Request) (uint64, bool) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	return id, err == nil
}

func (h *AlbumHandlers) Detail(w http.ResponseWriter, r *http.Request) {
	id, ok := albumIDParam(r)
	if !ok {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "相册 id 无效")
		return
	}
	v, err := h.D.Alb.Get(PrincipalFrom(r).User.ID, id)
	if errors.Is(err, albumsvc.ErrNotFound) {
		Fail(w, http.StatusNotFound, CodeNotFound, "相册不存在")
		return
	}
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	OK(w, albumViewDTO(v))
}

// PublicGet GET /api/v1/a/{id} —— 公开相册元数据（无需登录）。
func (h *AlbumHandlers) PublicGet(w http.ResponseWriter, r *http.Request) {
	id, ok := albumIDParam(r)
	if !ok {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "相册 id 无效")
		return
	}
	v, err := h.D.Alb.GetPublic(id)
	if errors.Is(err, albumsvc.ErrNotFound) {
		Fail(w, http.StatusNotFound, CodeNotFound, "相册不存在")
		return
	}
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	OK(w, albumViewDTO(v))
}

// PublicImages GET /api/v1/a/{id}/images
func (h *AlbumHandlers) PublicImages(w http.ResponseWriter, r *http.Request) {
	id, ok := albumIDParam(r)
	if !ok {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "相册 id 无效")
		return
	}
	limit := 24
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	items, next, err := h.D.Alb.ListPublicImages(id, r.URL.Query().Get("cursor"), limit)
	if errors.Is(err, albumsvc.ErrNotFound) {
		Fail(w, http.StatusNotFound, CodeNotFound, "相册不存在")
		return
	}
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{
			"key": it.Key, "name": it.Name, "ext": it.Ext,
			"width": it.Width, "height": it.Height, "size": it.Size,
			"thumbnail_url": "/t/" + it.Key + ".jpg",
			"url":           "/i/" + it.Key + "." + it.Ext,
		})
	}
	OK(w, map[string]any{"items": out, "next_cursor": next})
}

func (h *AlbumHandlers) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := albumIDParam(r)
	if !ok {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "相册 id 无效")
		return
	}
	var req struct {
		Name       *string `json:"name"`
		Visibility *string `json:"visibility"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "请求体无效")
		return
	}
	alb, err := h.D.Alb.Update(PrincipalFrom(r).User.ID, id, req.Name, req.Visibility)
	switch {
	case err == nil:
		OK(w, map[string]any{"id": alb.ID, "name": alb.Name, "visibility": alb.Visibility})
	case errors.Is(err, albumsvc.ErrNotFound):
		Fail(w, http.StatusNotFound, CodeNotFound, "相册不存在")
	case errors.Is(err, albumsvc.ErrInvalidName), errors.Is(err, albumsvc.ErrInvalidVisibility):
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, err.Error())
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}

func (h *AlbumHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := albumIDParam(r)
	if !ok {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "相册 id 无效")
		return
	}
	withImages := r.URL.Query().Get("with_images") == "true"
	err := h.D.Alb.Delete(PrincipalFrom(r).User.ID, id, withImages)
	switch {
	case err == nil:
		OK(w, map[string]any{"id": id, "deleted": true, "with_images": withImages})
	case errors.Is(err, albumsvc.ErrNotFound):
		Fail(w, http.StatusNotFound, CodeNotFound, "相册不存在")
	default:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	}
}
