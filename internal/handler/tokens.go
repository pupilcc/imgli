package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/apitoken"
)

type TokenHandlers struct{ Svc *apitoken.Service }

func tokenDTO(t *model.APIToken) map[string]any {
	var lastUsed *string
	if t.LastUsedAt != nil {
		s := t.LastUsedAt.Format(time.RFC3339)
		lastUsed = &s
	}
	return map[string]any{
		"id": t.ID, "name": t.Name, "scope": t.Scope,
		"created_at": t.CreatedAt.Format(time.RFC3339), "last_used_at": lastUsed,
	}
}

// List GET /api/v1/user/tokens
func (h *TokenHandlers) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.Svc.List(PrincipalFrom(r).User.ID)
	if err != nil {
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	out := make([]map[string]any, 0, len(list))
	for i := range list {
		out = append(out, tokenDTO(&list[i]))
	}
	OK(w, out)
}

// Create POST /api/v1/user/tokens —— 响应含 token 明文，仅此一次。
func (h *TokenHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Scope string `json:"scope"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "需要 name 与 scope")
		return
	}
	plain, t, err := h.Svc.Create(PrincipalFrom(r).User.ID, req.Name, req.Scope)
	switch {
	case errors.Is(err, apitoken.ErrInvalidScope):
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "scope 仅支持 upload 或 full")
		return
	case errors.Is(err, apitoken.ErrInvalidName):
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "名称需 1-64 个字符")
		return
	case err != nil:
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	dto := tokenDTO(t)
	dto["token"] = plain
	OK(w, dto)
}

// Delete DELETE /api/v1/user/tokens/{id}
func (h *TokenHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, "非法 id")
		return
	}
	if err := h.Svc.Revoke(PrincipalFrom(r).User.ID, id); err != nil {
		if errors.Is(err, apitoken.ErrNotFound) {
			Fail(w, http.StatusNotFound, CodeNotFound, "Token 不存在")
			return
		}
		Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		return
	}
	OK(w, nil)
}
