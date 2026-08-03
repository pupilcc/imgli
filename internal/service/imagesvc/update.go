package imagesvc

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/auth"
	"github.com/yixian-huang/imgli/internal/service/storagesvc"
)

// UpdatePatch 是 Update 的可选字段集合；指针 nil 表示不改。
// AlbumID：nil=不改，0=移出，>0=移入(校验归属)。
// SetExpires=false 不改过期；true 时写入 ExpiresAt（nil 即清除为 NULL）。
// Slug：nil=不改；""=清除；否则校验 [a-z0-9-]{3,32} 并唯一。
// MaxViews：nil=不改；0=不限；1–MaxViewsMax=上限（不重置 views_served）。
// AccessPassword：nil=不改；""=清除；非空=argon2 哈希写入（明文不落库）。
type UpdatePatch struct {
	Name           *string
	Visibility     *string
	AlbumID        *int64
	ExpiresAt      *time.Time
	SetExpires     bool
	Slug           *string
	MaxViews       *int
	AccessPassword *string
}

var (
	// ErrExpiresOverGroup 改期超出用户组有效期限制。
	ErrExpiresOverGroup = errors.New("imagesvc: 有效期超出用户组限制")
	// ErrMaxViewsOverGroup 访问次数超出用户组限制。
	ErrMaxViewsOverGroup = errors.New("imagesvc: 访问次数超出用户组限制")
)

// Update 部分更新单图（见 UpdatePatch）。
func (s *Service) Update(userID uint64, key string, p UpdatePatch) (*Row, error) {
	var img model.Image
	err := s.db.Where("key = ? AND user_id = ?", key, userID).First(&img).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var group model.UserGroup
	if p.SetExpires || p.MaxViews != nil {
		var u model.User
		if err := s.db.First(&u, userID).Error; err != nil {
			return nil, err
		}
		if err := s.db.First(&group, u.GroupID).Error; err != nil {
			return nil, err
		}
	}
	updates := map[string]any{}
	if p.Name != nil {
		n := strings.TrimSpace(*p.Name)
		if n == "" || len(n) > 255 {
			return nil, ErrInvalidName
		}
		updates["name"] = n
	}
	if p.Visibility != nil {
		if *p.Visibility != "public" && *p.Visibility != "private" {
			return nil, ErrInvalidVisibility
		}
		updates["visibility"] = *p.Visibility
	}
	if p.AlbumID != nil {
		if *p.AlbumID == 0 {
			updates["album_id"] = nil
		} else {
			var cnt int64
			if err := s.db.Model(&model.Album{}).
				Where("id = ? AND user_id = ?", *p.AlbumID, userID).Count(&cnt).Error; err != nil {
				return nil, err
			}
			if cnt == 0 {
				return nil, ErrAlbumNotFound
			}
			updates["album_id"] = uint64(*p.AlbumID)
		}
	}
	if p.SetExpires {
		if err := checkGroupExpires(&group, p.ExpiresAt, time.Now()); err != nil {
			return nil, err
		}
		// map 中显式写 nil → GORM Updates 写 NULL（与 album_id 清出同模式）
		updates["expires_at"] = p.ExpiresAt
	}
	if p.MaxViews != nil {
		if *p.MaxViews < 0 || *p.MaxViews > MaxViewsMax {
			return nil, ErrInvalidMaxViews
		}
		if err := checkGroupMaxViews(&group, *p.MaxViews); err != nil {
			return nil, err
		}
		updates["max_views"] = *p.MaxViews
	}
	if p.AccessPassword != nil {
		pw := strings.TrimSpace(*p.AccessPassword)
		if pw == "" {
			updates["access_password_hash"] = ""
		} else {
			if len(pw) > 128 {
				return nil, ErrInvalidAccessPassword
			}
			h, err := auth.HashPassword(pw)
			if err != nil {
				return nil, err
			}
			updates["access_password_hash"] = h
		}
	}
	if p.Slug != nil {
		v := strings.ToLower(strings.TrimSpace(*p.Slug))
		if v == "" {
			updates["slug"] = nil
		} else {
			if !slugRe.MatchString(v) {
				return nil, ErrInvalidSlug
			}
			var n int64
			if err := s.db.Model(&model.Image{}).Where("slug = ? AND key <> ?", v, key).Count(&n).Error; err != nil {
				return nil, err
			}
			if n > 0 {
				return nil, ErrSlugTaken
			}
			// 勿与既有 key 冲突
			if err := s.db.Model(&model.Image{}).Where("key = ? AND key <> ?", v, key).Count(&n).Error; err != nil {
				return nil, err
			}
			if n > 0 {
				return nil, ErrSlugTaken
			}
			updates["slug"] = v
		}
	}
	// 可见性变更 → surface 重挂(私密图对象层防护 S2):把 img 重挂到目标 surface 的 File,
	// 复制对象(事务外),调 ref_count,旧 File 归零投递异步 purge。surface == visibility。
	if p.Visibility != nil && *p.Visibility != img.Visibility {
		var oldFile model.File
		if err := s.db.First(&oldFile, img.FileID).Error; err != nil {
			return nil, err
		}
		var policy model.StoragePolicy
		if err := s.db.First(&policy, oldFile.StoragePolicyID).Error; err != nil {
			return nil, err
		}
		newFile, err := s.resolveFileForSurface(&policy, &oldFile, *p.Visibility)
		if err != nil {
			return nil, err
		}
		updates["file_id"] = newFile.ID
		var pd *physicalDelete
		err = s.db.Transaction(func(tx *gorm.DB) error {
			// CAS 重挂:仅当 img 仍指向 oldFile 才重挂——防并发切换重复调 ref。
			// 先于 ref 调整与删旧 File:此后 img 不再引用 oldFile,删其行不违反
			// images.file_id 的 ON DELETE RESTRICT。
			res := tx.Model(&model.Image{}).Where("id = ? AND file_id = ?", img.ID, oldFile.ID).Updates(updates)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return errRehomeConflict // 并发已重挂,回滚(本事务未做 ref 变更)
			}
			// newFile ref++(条件:防并发 purge 删行,同 upload 秒传先例)
			r1 := tx.Model(&model.File{}).Where("id = ?", newFile.ID).
				UpdateColumn("ref_count", gorm.Expr("ref_count + 1"))
			if r1.Error != nil {
				return r1.Error
			}
			if r1.RowsAffected == 0 {
				return gorm.ErrRecordNotFound
			}
			// oldFile ref--
			if err := tx.Model(&model.File{}).Where("id = ?", oldFile.ID).
				UpdateColumn("ref_count", gorm.Expr("ref_count - ?", 1)).Error; err != nil {
				return err
			}
			// 条件删:仅当减后 ≤0 才删旧 File 行 → 备物理删除旧对象(此时 img 已重挂)
			del := tx.Where("id = ? AND ref_count <= 0", oldFile.ID).Delete(&model.File{})
			if del.Error != nil {
				return del.Error
			}
			if del.RowsAffected == 1 {
				pd = &physicalDelete{
					policyID: oldFile.StoragePolicyID, path: oldFile.Path,
					thumbs: storagesvc.ThumbKeyCandidates(oldFile.Surface, oldFile.Hash),
				}
			}
			return nil
		})
		if errors.Is(err, errRehomeConflict) {
			return s.Get(userID, key) // 并发已达目标态,返回当前
		}
		if err != nil {
			return nil, err
		}
		s.enqueuePhysical(pd)
		return s.Get(userID, key)
	}

	if len(updates) > 0 {
		if err := s.db.Model(&img).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.Get(userID, key)
}

// SoftDelete 软删（进回收站，保配额，直链转 410）。非属主→ErrNotFound。
