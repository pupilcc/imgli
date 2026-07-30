package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/adminsvc"
)

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
