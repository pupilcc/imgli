package imagesvc

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
)

// LifecycleApplyPreview 组级「补写有效期」干跑结果。
// 仅处理 live 图：永久（expires_at NULL）或 expires_at 超过 now+cap 的记录。
// 不回溯改组前的历史策略；force/retention 超龄清理走小时任务 / cleanup kinds。
type LifecycleApplyPreview struct {
	GroupID       uint64   `json:"group_id"`
	CapSec        int      `json:"cap_sec"` // 0=组无 cap，无需补写
	Permanent     int      `json:"permanent_count"`
	OverCap       int      `json:"over_cap_count"`
	Total         int      `json:"total"` // permanent+over_cap（去重后即候选数）
	Samples       []string `json:"samples,omitempty"`
	Note          string   `json:"note,omitempty"`
}

// LifecycleApplyResult 执行结果。
type LifecycleApplyResult struct {
	GroupID  uint64 `json:"group_id"`
	Updated  int    `json:"updated"`
	Skipped  int    `json:"skipped"`
	CapSec   int    `json:"cap_sec"`
}

// PreviewApplyLifecycle 统计将把有效期钳到 now+cap 的 live 图数量。
func (s *Service) PreviewApplyLifecycle(groupID uint64, sampleLimit int) (*LifecycleApplyPreview, error) {
	if sampleLimit <= 0 {
		sampleLimit = 10
	}
	g, err := s.loadGroup(groupID)
	if err != nil {
		return nil, err
	}
	capSec := groupCapSec(g)
	out := &LifecycleApplyPreview{GroupID: groupID, CapSec: capSec}
	if capSec <= 0 {
		out.Note = "group has no max_expires_in / force_max_age_days; nothing to clamp"
		return out, nil
	}
	now := time.Now()
	maxAt := now.Add(time.Duration(capSec) * time.Second)
	var perm, over int64
	if err := s.groupLiveImagesQuery(g).Where("expires_at IS NULL").Count(&perm).Error; err != nil {
		return nil, err
	}
	if err := s.groupLiveImagesQuery(g).Where("expires_at IS NOT NULL AND expires_at > ?", maxAt).Count(&over).Error; err != nil {
		return nil, err
	}
	out.Permanent = int(perm)
	out.OverCap = int(over)
	out.Total = out.Permanent + out.OverCap
	var keys []string
	_ = s.groupLiveImagesQuery(g).
		Where("expires_at IS NULL OR expires_at > ?", maxAt).
		Order("id").Limit(sampleLimit).Pluck("key", &keys)
	out.Samples = keys
	out.Note = "new uploads already enforced; this only clamps existing live images"
	return out, nil
}

// ApplyLifecycle 将组内 live 图有效期钳制到 now+cap（永久→设 cap；超 cap→截断）。
// confirm 必须 true；limit 为最多更新条数，0=默认 500。
func (s *Service) ApplyLifecycle(groupID uint64, confirm bool, limit int) (*LifecycleApplyResult, error) {
	if !confirm {
		return nil, errors.New("imagesvc: confirm=true required")
	}
	g, err := s.loadGroup(groupID)
	if err != nil {
		return nil, err
	}
	capSec := groupCapSec(g)
	out := &LifecycleApplyResult{GroupID: groupID, CapSec: capSec}
	if capSec <= 0 {
		return out, nil
	}
	if limit <= 0 {
		limit = 500
	}
	now := time.Now()
	maxAt := now.Add(time.Duration(capSec) * time.Second)
	var imgs []model.Image
	if err := s.groupLiveImagesQuery(g).
		Where("expires_at IS NULL OR expires_at > ?", maxAt).
		Order("id").Limit(limit).Find(&imgs).Error; err != nil {
		return nil, err
	}
	for i := range imgs {
		res := s.db.Model(&model.Image{}).
			Where("id = ? AND deleted_at IS NULL AND (expires_at IS NULL OR expires_at > ?)", imgs[i].ID, maxAt).
			Update("expires_at", maxAt)
		if res.Error != nil {
			return out, res.Error
		}
		if res.RowsAffected > 0 {
			out.Updated++
		} else {
			out.Skipped++
		}
	}
	return out, nil
}

func (s *Service) loadGroup(id uint64) (*model.UserGroup, error) {
	var g model.UserGroup
	if err := s.db.First(&g, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &g, nil
}

func groupCapSec(g *model.UserGroup) int {
	if g == nil {
		return 0
	}
	capSec := g.MaxExpiresIn
	if g.ForceMaxAgeDays > 0 {
		force := g.ForceMaxAgeDays * 86400
		if capSec <= 0 || force < capSec {
			capSec = force
		}
	}
	if capSec > 366*24*60*60 {
		capSec = 366 * 24 * 60 * 60
	}
	if capSec < 0 {
		return 0
	}
	return capSec
}

func (s *Service) groupLiveImagesQuery(g *model.UserGroup) *gorm.DB {
	q := s.db.Model(&model.Image{}).Where("deleted_at IS NULL")
	if g.IsGuest {
		return q.Where("user_id IS NULL")
	}
	return q.Where("user_id IN (?)", s.db.Model(&model.User{}).Select("id").Where("group_id = ?", g.ID))
}
