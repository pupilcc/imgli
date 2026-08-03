package imagesvc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
)

// CleanupKind 管理员清理类别。
const (
	CleanupExpired        = "expired"          // expires_at 已过且未软删
	CleanupTrash          = "trash"            // 回收站超过 TrashMaxAge
	CleanupGroupRetention = "group_retention"  // 组 RetentionDays 超龄 live → 软删
	CleanupGroupForceAge  = "group_force_age"  // 组 ForceMaxAgeDays 超龄 live → 硬清
)

// TrashMaxAge 与 PurgeExpiredTrash 一致。
const TrashMaxAge = 30 * 24 * time.Hour

// CleanupPreview 干跑结果（不含对象密钥）。
type CleanupPreview struct {
	Kind    string   `json:"kind"`
	Count   int      `json:"count"`
	Samples []string `json:"samples,omitempty"` // image keys，最多 10
}

// CleanupRunResult 执行结果。
type CleanupRunResult struct {
	Kind    string `json:"kind"`
	Deleted int    `json:"deleted"`
	Errors  int    `json:"errors"`
}

// PreviewCleanup 统计将清理的条数与样例 key。
func (s *Service) PreviewCleanup(kinds []string, sampleLimit int) ([]CleanupPreview, error) {
	if sampleLimit <= 0 {
		sampleLimit = 10
	}
	if len(kinds) == 0 {
		kinds = []string{CleanupExpired, CleanupTrash, CleanupGroupRetention, CleanupGroupForceAge}
	}
	out := make([]CleanupPreview, 0, len(kinds))
	for _, k := range kinds {
		switch k {
		case CleanupExpired:
			p, err := s.previewExpired(sampleLimit)
			if err != nil {
				return nil, err
			}
			out = append(out, p)
		case CleanupTrash:
			p, err := s.previewTrash(sampleLimit)
			if err != nil {
				return nil, err
			}
			out = append(out, p)
		case CleanupGroupRetention:
			p, err := s.previewGroupRetention(sampleLimit)
			if err != nil {
				return nil, err
			}
			out = append(out, p)
		case CleanupGroupForceAge:
			p, err := s.previewGroupForceAge(sampleLimit)
			if err != nil {
				return nil, err
			}
			out = append(out, p)
		default:
			return nil, fmt.Errorf("imagesvc: unknown cleanup kind %q", k)
		}
	}
	return out, nil
}

// RunCleanup 执行清理；confirm 必须 true。limit 为每类最多删除数，0=不限（仍建议运营用小批）。
func (s *Service) RunCleanup(ctx context.Context, kinds []string, limit int, confirm bool) ([]CleanupRunResult, error) {
	if !confirm {
		return nil, errors.New("imagesvc: confirm=true required")
	}
	if len(kinds) == 0 {
		kinds = []string{CleanupExpired, CleanupTrash, CleanupGroupRetention, CleanupGroupForceAge}
	}
	out := make([]CleanupRunResult, 0, len(kinds))
	for _, k := range kinds {
		switch k {
		case CleanupExpired:
			n, errs, err := s.runExpired(ctx, limit)
			if err != nil {
				return out, err
			}
			out = append(out, CleanupRunResult{Kind: k, Deleted: n, Errors: errs})
		case CleanupTrash:
			n, errs, err := s.runTrash(ctx, limit)
			if err != nil {
				return out, err
			}
			out = append(out, CleanupRunResult{Kind: k, Deleted: n, Errors: errs})
		case CleanupGroupRetention:
			n, err := s.SoftDeleteByGroupRetention(ctx)
			if err != nil {
				return out, err
			}
			if limit > 0 && n > limit {
				n = limit // SoftDelete 内部按 500/组；limit 仅作展示上限提示
			}
			out = append(out, CleanupRunResult{Kind: k, Deleted: n})
		case CleanupGroupForceAge:
			n, err := s.PurgeByGroupForceMaxAge(ctx)
			if err != nil {
				return out, err
			}
			out = append(out, CleanupRunResult{Kind: k, Deleted: n})
		default:
			return nil, fmt.Errorf("imagesvc: unknown cleanup kind %q", k)
		}
	}
	return out, nil
}

func (s *Service) previewExpired(sampleLimit int) (CleanupPreview, error) {
	var count int64
	q := s.db.Model(&model.Image{}).
		Where("expires_at IS NOT NULL AND expires_at < ? AND deleted_at IS NULL", time.Now())
	if err := q.Count(&count).Error; err != nil {
		return CleanupPreview{}, err
	}
	var keys []string
	_ = s.db.Model(&model.Image{}).
		Where("expires_at IS NOT NULL AND expires_at < ? AND deleted_at IS NULL", time.Now()).
		Order("id").Limit(sampleLimit).Pluck("key", &keys)
	return CleanupPreview{Kind: CleanupExpired, Count: int(count), Samples: keys}, nil
}

func (s *Service) previewTrash(sampleLimit int) (CleanupPreview, error) {
	cutoff := time.Now().Add(-TrashMaxAge)
	var count int64
	if err := s.db.Unscoped().Model(&model.Image{}).
		Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff).Count(&count).Error; err != nil {
		return CleanupPreview{}, err
	}
	var keys []string
	_ = s.db.Unscoped().Model(&model.Image{}).
		Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff).
		Order("id").Limit(sampleLimit).Pluck("key", &keys)
	return CleanupPreview{Kind: CleanupTrash, Count: int(count), Samples: keys}, nil
}

func (s *Service) runExpired(ctx context.Context, limit int) (deleted, errs int, err error) {
	var imgs []model.Image
	q := s.db.Where("expires_at IS NOT NULL AND expires_at < ? AND deleted_at IS NULL", time.Now()).Order("id")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&imgs).Error; err != nil {
		return 0, 0, err
	}
	for i := range imgs {
		if err := ctx.Err(); err != nil {
			return deleted, errs, err
		}
		var pd *physicalDelete
		did := false
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			res := tx.Where("id = ? AND expires_at IS NOT NULL AND expires_at < ? AND deleted_at IS NULL",
				imgs[i].ID, time.Now()).Delete(&model.Image{})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return nil
			}
			did = true
			var e error
			pd, e = s.purgeOne(tx, &imgs[i])
			return e
		}); err != nil {
			slog.Error("admin cleanup expired failed", "image_id", imgs[i].ID, "err", err)
			errs++
			continue
		}
		if !did {
			continue
		}
		s.enqueuePhysical(pd)
		deleted++
	}
	return deleted, errs, nil
}

func (s *Service) previewGroupRetention(sampleLimit int) (CleanupPreview, error) {
	keys, count, err := s.listGroupLifecycleCandidates(true, sampleLimit)
	if err != nil {
		return CleanupPreview{}, err
	}
	return CleanupPreview{Kind: CleanupGroupRetention, Count: count, Samples: keys}, nil
}

func (s *Service) previewGroupForceAge(sampleLimit int) (CleanupPreview, error) {
	keys, count, err := s.listGroupLifecycleCandidates(false, sampleLimit)
	if err != nil {
		return CleanupPreview{}, err
	}
	return CleanupPreview{Kind: CleanupGroupForceAge, Count: count, Samples: keys}, nil
}

// listGroupLifecycleCandidates retention=true 用 RetentionDays；false 用 ForceMaxAgeDays。
func (s *Service) listGroupLifecycleCandidates(retention bool, sampleLimit int) (samples []string, total int, err error) {
	var groups []model.UserGroup
	q := s.db
	if retention {
		q = q.Where("retention_days > 0")
	} else {
		q = q.Where("force_max_age_days > 0")
	}
	if err := q.Find(&groups).Error; err != nil {
		return nil, 0, err
	}
	now := time.Now()
	seen := map[string]bool{}
	for _, g := range groups {
		days := g.ForceMaxAgeDays
		if retention {
			days = g.RetentionDays
		}
		cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
		var n int64
		cq := s.db.Model(&model.Image{}).Where("deleted_at IS NULL AND created_at < ?", cutoff)
		if g.IsGuest {
			cq = cq.Where("user_id IS NULL")
		} else {
			cq = cq.Where("user_id IN (?)", s.db.Model(&model.User{}).Select("id").Where("group_id = ?", g.ID))
		}
		if err := cq.Count(&n).Error; err != nil {
			return nil, 0, err
		}
		total += int(n)
		if sampleLimit > 0 && len(samples) < sampleLimit {
			var keys []string
			sq := s.db.Model(&model.Image{}).Where("deleted_at IS NULL AND created_at < ?", cutoff)
			if g.IsGuest {
				sq = sq.Where("user_id IS NULL")
			} else {
				sq = sq.Where("user_id IN (?)", s.db.Model(&model.User{}).Select("id").Where("group_id = ?", g.ID))
			}
			_ = sq.Order("id").Limit(sampleLimit - len(samples)).Pluck("key", &keys)
			for _, k := range keys {
				if !seen[k] {
					seen[k] = true
					samples = append(samples, k)
				}
			}
		}
	}
	return samples, total, nil
}

func (s *Service) runTrash(ctx context.Context, limit int) (deleted, errs int, err error) {
	cutoff := time.Now().Add(-TrashMaxAge)
	var imgs []model.Image
	q := s.db.Unscoped().Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff).Order("id")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&imgs).Error; err != nil {
		return 0, 0, err
	}
	for i := range imgs {
		if err := ctx.Err(); err != nil {
			return deleted, errs, err
		}
		var pd *physicalDelete
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			var e error
			pd, e = s.purgeOne(tx, &imgs[i])
			return e
		}); err != nil {
			slog.Error("admin cleanup trash failed", "image_id", imgs[i].ID, "err", err)
			errs++
			continue
		}
		s.enqueuePhysical(pd)
		deleted++
	}
	return deleted, errs, nil
}
