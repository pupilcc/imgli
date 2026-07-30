package imagesvc

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
)

// Get 取属主单图（连 file/policy）。非属主或不存在返回 ErrNotFound。
func (s *Service) Get(userID uint64, key string) (*Row, error) {
	var img model.Image
	err := s.db.Where("key = ? AND user_id = ?", key, userID).First(&img).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var file model.File
	if err := s.db.First(&file, img.FileID).Error; err != nil {
		return nil, err
	}
	var pol model.StoragePolicy
	if err := s.db.First(&pol, file.StoragePolicyID).Error; err != nil {
		return nil, err
	}
	return &Row{Img: img, File: file, Policy: pol}, nil
}

// GetPublicShare 公开分享页：仅 visibility=public 且 status=normal 且未过期。
// 支持 key 或 slug；private / pending / rejected / 软删 / 过期 一律 ErrNotFound（不区分存在性）。

// GetPublicShare 公开分享页：仅 visibility=public 且 status=normal 且未过期。
// 支持 key 或 slug；private / pending / rejected / 软删 / 过期 一律 ErrNotFound（不区分存在性）。
func (s *Service) GetPublicShare(ref string) (*Row, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, ErrNotFound
	}
	var img model.Image
	err := s.db.Where("key = ?", ref).First(&img).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = s.db.Where("slug = ?", ref).First(&img).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if img.Visibility != "public" || img.Status != "normal" {
		return nil, ErrNotFound
	}
	if img.ExpiresAt != nil && !img.ExpiresAt.After(time.Now()) {
		return nil, ErrNotFound
	}
	if img.MaxViews > 0 && img.ViewsServed >= img.MaxViews {
		return nil, ErrNotFound
	}
	var file model.File
	if err := s.db.First(&file, img.FileID).Error; err != nil {
		return nil, err
	}
	var pol model.StoragePolicy
	if err := s.db.First(&pol, file.StoragePolicyID).Error; err != nil {
		return nil, err
	}
	return &Row{Img: img, File: file, Policy: pol}, nil
}
