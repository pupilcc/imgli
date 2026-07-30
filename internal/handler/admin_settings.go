package handler

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/yixian-huang/imgli/internal/apperr"
	"github.com/yixian-huang/imgli/internal/model"
)

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
	if err == nil {
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
		return
	}
	// adminsvc/moderation/upload 校验错误均实现 apperr.Client → 400
	if apperr.IsClient(err) {
		Fail(w, http.StatusBadRequest, CodeInvalidRequest, err.Error())
		return
	}
	Fail(w, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
}
