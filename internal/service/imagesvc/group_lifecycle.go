package imagesvc

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
)

// SoftDeleteByGroupRetention 按用户组 RetentionDays 将超龄 live 图软删进回收站。
// 游客图（user_id IS NULL）用 is_guest 组策略。返回软删张数。
func (s *Service) SoftDeleteByGroupRetention(ctx context.Context) (int, error) {
	var groups []model.UserGroup
	if err := s.db.Where("retention_days > 0").Find(&groups).Error; err != nil {
		return 0, err
	}
	n := 0
	now := time.Now()
	for _, g := range groups {
		if err := ctx.Err(); err != nil {
			return n, err
		}
		cutoff := now.Add(-time.Duration(g.RetentionDays) * 24 * time.Hour)
		var imgs []model.Image
		q := s.db.Where("deleted_at IS NULL AND created_at < ?", cutoff)
		if g.IsGuest {
			q = q.Where("user_id IS NULL")
		} else {
			q = q.Where("user_id IN (?)", s.db.Model(&model.User{}).Select("id").Where("group_id = ?", g.ID))
		}
		if err := q.Order("id").Limit(500).Find(&imgs).Error; err != nil {
			return n, err
		}
		for i := range imgs {
			if err := ctx.Err(); err != nil {
				return n, err
			}
			res := s.db.Where("id = ? AND deleted_at IS NULL", imgs[i].ID).Delete(&model.Image{})
			if res.Error != nil {
				slog.Error("组保留期软删失败", "image_id", imgs[i].ID, "err", res.Error)
				continue
			}
			if res.RowsAffected > 0 {
				n++
			}
		}
	}
	return n, nil
}

// PurgeByGroupForceMaxAge 按用户组 ForceMaxAgeDays 永久清理超龄 live 图（不进回收站）。
// 与 expires_at 过期清理同路径（soft-then-purgeOne）。每组每轮最多 500 张。
func (s *Service) PurgeByGroupForceMaxAge(ctx context.Context) (int, error) {
	var groups []model.UserGroup
	if err := s.db.Where("force_max_age_days > 0").Find(&groups).Error; err != nil {
		return 0, err
	}
	n := 0
	now := time.Now()
	for _, g := range groups {
		if err := ctx.Err(); err != nil {
			return n, err
		}
		cutoff := now.Add(-time.Duration(g.ForceMaxAgeDays) * 24 * time.Hour)
		var imgs []model.Image
		q := s.db.Where("deleted_at IS NULL AND created_at < ?", cutoff)
		if g.IsGuest {
			q = q.Where("user_id IS NULL")
		} else {
			q = q.Where("user_id IN (?)", s.db.Model(&model.User{}).Select("id").Where("group_id = ?", g.ID))
		}
		if err := q.Order("id").Limit(500).Find(&imgs).Error; err != nil {
			return n, err
		}
		for i := range imgs {
			if err := ctx.Err(); err != nil {
				return n, err
			}
			var pd *physicalDelete
			did := false
			if err := s.db.Transaction(func(tx *gorm.DB) error {
				res := tx.Where("id = ? AND deleted_at IS NULL", imgs[i].ID).Delete(&model.Image{})
				if res.Error != nil {
					return res.Error
				}
				if res.RowsAffected == 0 {
					return nil
				}
				did = true
				// 刷新软删状态供 purgeOne
				if e := tx.Unscoped().Where("id = ?", imgs[i].ID).First(&imgs[i]).Error; e != nil {
					return e
				}
				var e error
				pd, e = s.purgeOne(tx, &imgs[i])
				return e
			}); err != nil {
				slog.Error("组强制最大存活清理失败", "image_id", imgs[i].ID, "err", err)
				continue
			}
			if !did {
				continue
			}
			s.enqueuePhysical(pd)
			n++
		}
	}
	return n, nil
}
