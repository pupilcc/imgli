package adminsvc

import (
	"github.com/yixian-huang/imgli/internal/model"
)

// ListLogs 按 action 与 actor_type 筛选审计日志，按 CreatedAt 倒序（二级排序用 id）分页返回。
// 空字符串表示不筛选该字段。page 为 1 表示第一页；limit 的默认值为 50，最大 200。
func (s *Service) ListLogs(action, actorType string, page, limit int) ([]model.AuditLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}

	tx := s.db.Model(&model.AuditLog{})

	// 按 action 筛选（空字符串不筛选）
	if action != "" {
		tx = tx.Where("action = ?", action)
	}

	// 按 actor_type 筛选（空字符串不筛选）
	if actorType != "" {
		tx = tx.Where("actor_type = ?", actorType)
	}

	// 计算总数
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 倒序（CreatedAt 倒序，id 倒序为二级排序）并分页查询
	var logs []model.AuditLog
	if err := tx.Order("created_at DESC, id DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
