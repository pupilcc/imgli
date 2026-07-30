package upload

import (
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
)

func (s *Service) commitInstant(u *model.User, file *model.File, filename, ext, visibility, ip string, size int64, albumID *uint64, expiresAt *time.Time, maxViews int, accessPasswordHash string, storageQuota int64) (*model.Image, error) {
	// 内容安全 P1：秒传继承同 file 上已有 image 的审核态与最高 nsfw_score，
	// 防止 rejected/pending 脏 hash 以新 key 复活为 normal。
	status, score := inheritModerationFrom(s.db, file.ID)
	img := &model.Image{
		Name: filename, Ext: ext, Visibility: visibilityFor(u, visibility),
		Status: status, NSFWScore: score, UploadIP: ip, FileID: file.ID, AlbumID: albumID,
		ExpiresAt: expiresAt, MaxViews: maxViews, AccessPasswordHash: accessPasswordHash,
	}
	if u != nil {
		img.UserID = &u.ID
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		key, err := s.uniqueKey(tx)
		if err != nil {
			return err
		}
		img.Key = key
		if err := tx.Create(img).Error; err != nil {
			return err
		}
		// 条件 +1：file 若已被 purge 删行则 0 行，回滚秒传（防 purge-vs-秒传 竞态）
		res := tx.Model(&model.File{}).Where("id = ?", file.ID).
			UpdateColumn("ref_count", gorm.Expr("ref_count + 1"))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		file.RefCount++
		if u == nil {
			return nil // 游客秒传不计配额，不累加 used_storage
		}
		return addUsedStorage(tx, u.ID, size, storageQuota)
	})
	if err != nil {
		return nil, err
	}
	return img, nil
}

// addUsedStorage 事务内原子累加配额。quota>0 时用 used_storage+size<=quota 条件更新，
// 堵住双请求均通过预检后超配额的 TOCTOU；quota<=0 表示不限。

// addUsedStorage 事务内原子累加配额。quota>0 时用 used_storage+size<=quota 条件更新，
// 堵住双请求均通过预检后超配额的 TOCTOU；quota<=0 表示不限。
func addUsedStorage(tx *gorm.DB, userID uint64, size, quota int64) error {
	var res *gorm.DB
	if quota > 0 {
		res = tx.Model(&model.User{}).
			Where("id = ? AND used_storage + ? <= ?", userID, size, quota).
			UpdateColumn("used_storage", gorm.Expr("used_storage + ?", size))
	} else {
		res = tx.Model(&model.User{}).Where("id = ?", userID).
			UpdateColumn("used_storage", gorm.Expr("used_storage + ?", size))
	}
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		// 区分：用户已注销 vs 配额不足
		var n int64
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			return gorm.ErrRecordNotFound
		}
		return ErrQuotaExceeded
	}
	return nil
}

// inheritModerationFrom 汇总同 file_id 既有 image 的审核结论。
// 严重度 rejected > pending > normal；nsfw_score 取非空最大值。
// 软删图不参与（gorm 默认 scope）。无兄弟图时返回 normal / nil。

// inheritModerationFrom 汇总同 file_id 既有 image 的审核结论。
// 严重度 rejected > pending > normal；nsfw_score 取非空最大值。
// 软删图不参与（gorm 默认 scope）。无兄弟图时返回 normal / nil。
func inheritModerationFrom(db *gorm.DB, fileID uint64) (status string, score *float64) {
	status = "normal"
	var siblings []model.Image
	if err := db.Select("status", "nsfw_score").Where("file_id = ?", fileID).Find(&siblings).Error; err != nil {
		return status, nil
	}
	for _, sib := range siblings {
		switch sib.Status {
		case "rejected":
			status = "rejected"
		case "pending":
			if status != "rejected" {
				status = "pending"
			}
		}
		if sib.NSFWScore != nil {
			if score == nil || *sib.NSFWScore > *score {
				v := *sib.NSFWScore
				score = &v
			}
		}
	}
	return status, score
}

// inheritModerationByHash 跨 surface 汇总同 hash 所有 File 上已有 image 的审核态与最高分。
// 用于上传新建分支:scoped-dedup 下私密撞公开(或反之)字节会新建独立 File,若不继承已有
// rejected/pending 结论,已拒内容会以新 surface 复活为 normal(绕过 commitInstant 的同 file
// 继承)。归并逻辑与 inheritModerationFrom 一致,只是范围扩到同 hash 的全部 File。

// inheritModerationByHash 跨 surface 汇总同 hash 所有 File 上已有 image 的审核态与最高分。
// 用于上传新建分支:scoped-dedup 下私密撞公开(或反之)字节会新建独立 File,若不继承已有
// rejected/pending 结论,已拒内容会以新 surface 复活为 normal(绕过 commitInstant 的同 file
// 继承)。归并逻辑与 inheritModerationFrom 一致,只是范围扩到同 hash 的全部 File。
func inheritModerationByHash(db *gorm.DB, hash string) (status string, score *float64) {
	status = "normal"
	var fileIDs []uint64
	if err := db.Model(&model.File{}).Where("hash = ?", hash).Pluck("id", &fileIDs).Error; err != nil || len(fileIDs) == 0 {
		return status, nil
	}
	var siblings []model.Image
	if err := db.Select("status", "nsfw_score").Where("file_id IN ?", fileIDs).Find(&siblings).Error; err != nil {
		return status, nil
	}
	for _, sib := range siblings {
		switch sib.Status {
		case "rejected":
			status = "rejected"
		case "pending":
			if status != "rejected" {
				status = "pending"
			}
		}
		if sib.NSFWScore != nil {
			if score == nil || *sib.NSFWScore > *score {
				v := *sib.NSFWScore
				score = &v
			}
		}
	}
	return status, score
}

// resolveAlbum 解析相册三态。游客(u==nil)恒 nil。
