package handler

import (
	"net/http"
	"time"

	"github.com/yixian-huang/imgli/internal/model"
)

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
	page, limit := ParsePage(r, logsDefaultPage, logsDefaultLimit, logsMaxLimit)
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
