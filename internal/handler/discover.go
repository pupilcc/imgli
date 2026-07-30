package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/discoversvc"
	"github.com/yixian-huang/imgli/internal/service/settings"
)

// DiscoverHandler 公开发现面（广场 + 用户公开主页），无需鉴权。
// 总开关 SettingPlazaEnabled；关闭时统一 404 防探测。
type DiscoverHandler struct {
	DB *gorm.DB
	St *settings.Service // 可选；nil 时按 DB 构造
	Svc *discoversvc.Service // 可选；nil 时按 DB 构造（避免每请求 New）
}

func (h *DiscoverHandler) settings() *settings.Service {
	if h.St != nil {
		return h.St
	}
	return settings.New(h.DB)
}

func (h *DiscoverHandler) discover() *discoversvc.Service {
	if h.Svc != nil {
		return h.Svc
	}
	return discoversvc.New(h.DB)
}

// enabled 读 plaza_enabled 开关；键缺失视为 false；其它错误返回给调用方当 500。
// 经 settings 30s 缓存；后台 PutSettings 后 Invalidate 即时生效。
func (h *DiscoverHandler) enabled() (bool, error) {
	return h.settings().PlazaEnabled()
}

func parseSort(r *http.Request) string {
	if r.URL.Query().Get("sort") == "hot" {
		return "hot"
	}
	return "new"
}

func parseLimit(r *http.Request) int {
	s := r.URL.Query().Get("limit")
	if s == "" {
		return 24
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 24
	}
	if n < 1 {
		return 24
	}
	if n > 60 {
		return 60
	}
	return n
}

// Plaza GET /api/v1/plaza
func (h *DiscoverHandler) Plaza(w http.ResponseWriter, r *http.Request) {
	ok, err := h.enabled()
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	if !ok {
		Fail(w, http.StatusNotFound, CodeNotFound, "资源不存在")
		return
	}

	rows, next, err := h.discover().PlazaFeed(parseSort(r), r.URL.Query().Get("cursor"), parseLimit(r))
	if errors.Is(err, discoversvc.ErrBadCursor) {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "游标格式错误")
		return
	}
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=30")
	OK(w, map[string]any{"items": rows, "next_cursor": next})
}

// UserProfile GET /api/v1/u/{username}
func (h *DiscoverHandler) UserProfile(w http.ResponseWriter, r *http.Request) {
	ok, err := h.enabled()
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	if !ok {
		Fail(w, http.StatusNotFound, CodeNotFound, "资源不存在")
		return
	}

	username := chi.URLParam(r, "username")
	p, err := h.discover().UserPublic(username)
	if errors.Is(err, discoversvc.ErrNotFound) {
		Fail(w, http.StatusNotFound, CodeNotFound, "主页不存在或未公开")
		return
	}
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	OK(w, map[string]any{"user": p})
}

// UserImages GET /api/v1/u/{username}/images
func (h *DiscoverHandler) UserImages(w http.ResponseWriter, r *http.Request) {
	ok, err := h.enabled()
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	if !ok {
		Fail(w, http.StatusNotFound, CodeNotFound, "资源不存在")
		return
	}

	username := chi.URLParam(r, "username")
	svc := h.discover()
	// 先判主页可公开，保证与 UserProfile 同一 404 文案（防枚举）
	if _, err := svc.UserPublic(username); errors.Is(err, discoversvc.ErrNotFound) {
		Fail(w, http.StatusNotFound, CodeNotFound, "主页不存在或未公开")
		return
	} else if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	// Profile 不含 UserID：用与 discoversvc 同过滤条件取 ID
	var u model.User
	if err := h.DB.Where("username = ? AND public_profile = ? AND status = ?", username, true, "active").
		First(&u).Error; err != nil {
		// ErrRecordNotFound=库竞态(两查询间用户转私密/被封)按 404;其余真实 DB 故障按 500,
		// 否则瞬时故障会被掩盖成「主页不存在」误导调用方(codex 后端评审 F1)。
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Fail(w, http.StatusNotFound, CodeNotFound, "主页不存在或未公开")
		} else {
			Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		}
		return
	}

	rows, next, err := svc.UserImages(u.ID, parseSort(r), r.URL.Query().Get("cursor"), parseLimit(r))
	if errors.Is(err, discoversvc.ErrBadCursor) {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "游标格式错误")
		return
	}
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=30")
	OK(w, map[string]any{"items": rows, "next_cursor": next})
}
