package imagesvc

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/storagesvc"
)

type physicalDelete struct {
	policyID uint64
	path     string
	// thumbs 当前世代 + 遗留缩略键,Delete 幂等(C2 ThumbGen dual-probe)。
	thumbs []string
}

func (s *Service) purgeOne(tx *gorm.DB, img *model.Image) (*physicalDelete, error) {
	// 幂等门禁：先硬删 image 行(仍在回收站者)，只有真正删掉(RowsAffected==1)才继续退配额/减引用。
	// 防并发/重试双删导致双退配额，以及误删仍被其它 live 图引用的共享物理文件。
	res := tx.Unscoped().Delete(&model.Image{}, "id = ? AND deleted_at IS NOT NULL", img.ID)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, nil // 已被并发/上一轮清理(或已恢复)，幂等 no-op
	}
	var file model.File
	if err := tx.First(&file, img.FileID).Error; err != nil {
		return nil, err
	}
	// 退还配额（软删未退，此刻退）——仅当有属主；CASE 夹到 ≥0，防陈旧计数双退变负
	if img.UserID != nil {
		if err := tx.Model(&model.User{}).Where("id = ?", *img.UserID).
			UpdateColumn("used_storage", gorm.Expr(
				"CASE WHEN used_storage >= ? THEN used_storage - ? ELSE 0 END",
				file.Size, file.Size,
			)).Error; err != nil {
			return nil, err
		}
	}
	// ref_count-1（原子）
	if err := tx.Model(&model.File{}).Where("id = ?", file.ID).
		UpdateColumn("ref_count", gorm.Expr("ref_count - ?", 1)).Error; err != nil {
		return nil, err
	}
	// 条件删：仅当减后 ≤0 才删行；并发秒传 +1 后本条 0 行，file 保留（purge-vs-秒传）
	del := tx.Where("id = ? AND ref_count <= 0", file.ID).Delete(&model.File{})
	if del.Error != nil {
		return nil, del.Error
	}
	if del.RowsAffected == 1 {
		return &physicalDelete{
			policyID: file.StoragePolicyID, path: file.Path,
			thumbs: storagesvc.ThumbKeyCandidates(file.Surface, file.Hash),
		}, nil
	}
	return nil, nil
}

func (s *Service) enqueuePhysical(pd *physicalDelete) {
	if pd == nil || s.run == nil {
		return
	}
	keys := append([]string{pd.path}, pd.thumbs...)
	for _, key := range keys {
		if key == "" {
			continue
		}
		payload, _ := json.Marshal(map[string]any{"policy_id": pd.policyID, "key": key})
		if err := s.run.Enqueue("delete_file", string(payload)); err != nil {
			slog.Error("投递物理删除失败", "policy_id", pd.policyID, "key", key, "err", err)
		}
	}
}

// DeleteUserData 注销级联:单事务硬删该用户全部图片(先把 live 图软删,复用
// purgeOne 的幂等门禁与引用计数)、相册、API token、auth token、session、用户行。
// 物理文件删除在事务提交后投递(归零 file 才删,秒传共享文件安全)。
// 不依赖 DB 级 CASCADE——存量 SQLite 库无外键约束(⑤a 裁决),关联表显式删。

// DeleteUserData 注销级联:单事务硬删该用户全部图片(先把 live 图软删,复用
// purgeOne 的幂等门禁与引用计数)、相册、API token、auth token、session、用户行。
// 物理文件删除在事务提交后投递(归零 file 才删,秒传共享文件安全)。
// 不依赖 DB 级 CASCADE——存量 SQLite 库无外键约束(⑤a 裁决),关联表显式删。
func (s *Service) DeleteUserData(userID uint64) error {
	var pds []*physicalDelete
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Model(&model.Image{}).
			Where("user_id = ? AND deleted_at IS NULL", userID).
			Update("deleted_at", time.Now()).Error; err != nil {
			return err
		}
		var imgs []model.Image
		if err := tx.Unscoped().Where("user_id = ?", userID).Find(&imgs).Error; err != nil {
			return err
		}
		for i := range imgs {
			pd, err := s.purgeOne(tx, &imgs[i])
			if err != nil {
				return err
			}
			if pd != nil {
				pds = append(pds, pd)
			}
		}
		if err := tx.Delete(&model.Album{}, "user_id = ?", userID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&model.APIToken{}, "user_id = ?", userID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&model.AuthToken{}, "user_id = ?", userID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&model.Session{}, "user_id = ?", userID).Error; err != nil {
			return err
		}
		return tx.Delete(&model.User{}, userID).Error
	})
	if err != nil {
		return err
	}
	for _, pd := range pds {
		s.enqueuePhysical(pd)
	}
	return nil
}

// PurgePermanent 彻底删除属主的一张软删图。非属主/不在回收站→ErrNotFound。

// PurgePermanent 彻底删除属主的一张软删图。非属主/不在回收站→ErrNotFound。
func (s *Service) PurgePermanent(userID uint64, key string) error {
	var img model.Image
	err := s.db.Unscoped().
		Where("key = ? AND user_id = ? AND deleted_at IS NOT NULL", key, userID).
		First(&img).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	var pd *physicalDelete
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var e error
		pd, e = s.purgeOne(tx, &img)
		return e
	}); err != nil {
		return err
	}
	s.enqueuePhysical(pd)
	return nil
}

// EmptyTrash 彻底删除属主回收站全部，返回清理张数。

// EmptyTrash 彻底删除属主回收站全部，返回清理张数。
func (s *Service) EmptyTrash(userID uint64) (int, error) {
	var imgs []model.Image
	if err := s.db.Unscoped().
		Where("user_id = ? AND deleted_at IS NOT NULL", userID).Find(&imgs).Error; err != nil {
		return 0, err
	}
	var pds []*physicalDelete
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		for i := range imgs {
			pd, e := s.purgeOne(tx, &imgs[i])
			if e != nil {
				return e
			}
			pds = append(pds, pd)
		}
		return nil
	}); err != nil {
		return 0, err
	}
	for _, pd := range pds {
		s.enqueuePhysical(pd)
	}
	return len(imgs), nil
}

// PurgeExpiredTrash 全站清理 deleted_at 超过 30 天的软删图（server 定时调用）。

// PurgeExpiredTrash 全站清理 deleted_at 超过 30 天的软删图（server 定时调用）。
func (s *Service) PurgeExpiredTrash(ctx context.Context) (int, error) {
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	var imgs []model.Image
	if err := s.db.Unscoped().
		Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff).Find(&imgs).Error; err != nil {
		return 0, err
	}
	n := 0
	for i := range imgs {
		var pd *physicalDelete
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			var e error
			pd, e = s.purgeOne(tx, &imgs[i])
			return e
		}); err != nil {
			slog.Error("清理过期软删失败", "image_id", imgs[i].ID, "err", err)
			continue
		}
		s.enqueuePhysical(pd)
		n++
	}
	return n, nil
}

// PurgeExpiredImages 物理清理到期(expires_at<now)的 live 图。过期即永久删除、回收配额,
// 不进回收站(否则延迟 30 天回收违背回收存储初衷)。server 每小时调用。

// PurgeExpiredImages 物理清理到期(expires_at<now)的 live 图。过期即永久删除、回收配额,
// 不进回收站(否则延迟 30 天回收违背回收存储初衷)。server 每小时调用。
func (s *Service) PurgeExpiredImages(ctx context.Context) (int, error) {
	var imgs []model.Image
	if err := s.db.Where("expires_at IS NOT NULL AND expires_at < ? AND deleted_at IS NULL", time.Now()).
		Find(&imgs).Error; err != nil {
		return 0, err
	}
	n := 0
	for i := range imgs {
		var pd *physicalDelete
		did := false
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			// 事务内条件软删:重新确认 DB 中仍过期(expires_at<now)、未被并发清除/延长/删除,
			// 防按 Find 陈旧快照永久误删已改期的图(codex 评审竞态)。RowsAffected==0 即跳过。
			// 软删置 deleted_at 以过 purgeOne 的 deleted_at NOT NULL 门禁,再复用其配额/引用/
			// 文件物理删逻辑——过期图直接永久清理,不经回收站。
			res := tx.Where("id = ? AND expires_at IS NOT NULL AND expires_at < ? AND deleted_at IS NULL",
				imgs[i].ID, time.Now()).Delete(&model.Image{})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return nil // 竞态:已改期/已删/已清,不 purge
			}
			did = true
			var e error
			pd, e = s.purgeOne(tx, &imgs[i])
			return e
		}); err != nil {
			slog.Error("清理过期图失败", "image_id", imgs[i].ID, "err", err)
			continue
		}
		if !did {
			continue
		}
		s.enqueuePhysical(pd)
		n++
	}
	return n, nil
}
