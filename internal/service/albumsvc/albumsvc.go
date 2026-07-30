// Package albumsvc 实现相册 CRUD；count/cover 实时查询不反规范化(spec §4)。
package albumsvc

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
)

var (
	ErrNotFound          = errors.New("albumsvc: 相册不存在")
	ErrInvalidName       = errors.New("albumsvc: 名称需 1-128 字节")
	ErrInvalidVisibility = errors.New("albumsvc: 可见性仅 public|private")
)

type Service struct{ db *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db: db} }

// AlbumView 相册 + 实时统计。
type AlbumView struct {
	Album    model.Album
	Count    int64
	CoverKey string
}

func normVis(v string) (string, error) {
	if v == "" {
		return "private", nil
	}
	if v != "public" && v != "private" {
		return "", ErrInvalidVisibility
	}
	return v, nil
}

func (s *Service) view(alb model.Album) (AlbumView, error) {
	var count int64
	if err := s.db.Model(&model.Image{}).
		Where("album_id = ? AND user_id = ? AND deleted_at IS NULL", alb.ID, alb.UserID).Count(&count).Error; err != nil {
		return AlbumView{}, err
	}
	var cover model.Image
	coverKey := ""
	err := s.db.Where("album_id = ? AND user_id = ? AND deleted_at IS NULL", alb.ID, alb.UserID).
		Order("created_at DESC, id DESC").First(&cover).Error
	if err == nil {
		coverKey = cover.Key
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return AlbumView{}, err
	}
	return AlbumView{Album: alb, Count: count, CoverKey: coverKey}, nil
}

func (s *Service) List(userID uint64) ([]AlbumView, error) {
	var albums []model.Album
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&albums).Error; err != nil {
		return nil, err
	}
	out := make([]AlbumView, 0, len(albums))
	for _, a := range albums {
		v, err := s.view(a)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *Service) Create(userID uint64, name, visibility string) (*model.Album, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 128 {
		return nil, ErrInvalidName
	}
	vis, err := normVis(visibility)
	if err != nil {
		return nil, err
	}
	alb := &model.Album{UserID: userID, Name: name, Visibility: vis}
	if err := s.db.Create(alb).Error; err != nil {
		return nil, err
	}
	return alb, nil
}

func (s *Service) Get(userID, id uint64) (*AlbumView, error) {
	var alb model.Album
	err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&alb).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	v, err := s.view(alb)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// GetPublic 公开相册访客：仅 visibility=public；否则 ErrNotFound（不区分存在性）。
func (s *Service) GetPublic(id uint64) (*AlbumView, error) {
	var alb model.Album
	err := s.db.Where("id = ?", id).First(&alb).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if alb.Visibility != "public" {
		return nil, ErrNotFound
	}
	v, err := s.viewPublic(alb)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// viewPublic 统计/封面仅 public+normal+未过期+非口令图（口令图不出现在访客封面网格）。
func (s *Service) viewPublic(alb model.Album) (AlbumView, error) {
	q := s.db.Model(&model.Image{}).
		Where("album_id = ? AND user_id = ? AND deleted_at IS NULL AND visibility = ? AND status = ?",
			alb.ID, alb.UserID, "public", "normal").
		Where("expires_at IS NULL OR expires_at > NOW()").
		Where("access_password_hash = '' OR access_password_hash IS NULL")
	// SQLite 兼容：NOW() 在 SQLite 可能不行 — 用 gorm 表达式 time.Now()
	// 改用 Where 与 imagesvc 一致
	_ = q
	var count int64
	nowQ := s.db.Model(&model.Image{}).
		Where("album_id = ? AND user_id = ? AND deleted_at IS NULL AND visibility = ? AND status = ?",
			alb.ID, alb.UserID, "public", "normal").
		Where("(expires_at IS NULL OR expires_at > ?)", timeNow()).
		Where("(access_password_hash = '' OR access_password_hash IS NULL)")
	if err := nowQ.Count(&count).Error; err != nil {
		return AlbumView{}, err
	}
	var cover model.Image
	coverKey := ""
	err := s.db.Where("album_id = ? AND user_id = ? AND deleted_at IS NULL AND visibility = ? AND status = ?",
		alb.ID, alb.UserID, "public", "normal").
		Where("(expires_at IS NULL OR expires_at > ?)", timeNow()).
		Where("(access_password_hash = '' OR access_password_hash IS NULL)").
		Order("created_at DESC, id DESC").First(&cover).Error
	if err == nil {
		coverKey = cover.Key
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return AlbumView{}, err
	}
	return AlbumView{Album: alb, Count: count, CoverKey: coverKey}, nil
}

// PublicImage 访客网格行。
type PublicImage struct {
	Key    string
	Name   string
	Ext    string
	Width  int
	Height int
	Size   int64
}

// ListPublicImages 公开相册内可展示图（cursor=id 降序）。
func (s *Service) ListPublicImages(albumID uint64, cursor string, limit int) ([]PublicImage, string, error) {
	if limit <= 0 {
		limit = 24
	}
	if limit > 100 {
		limit = 100
	}
	var alb model.Album
	if err := s.db.Where("id = ? AND visibility = ?", albumID, "public").First(&alb).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", ErrNotFound
		}
		return nil, "", err
	}
	q := s.db.Model(&model.Image{}).
		Select("images.key, images.name, images.ext, files.width, files.height, files.size, images.id").
		Joins("JOIN files ON files.id = images.file_id").
		Where("images.album_id = ? AND images.user_id = ? AND images.deleted_at IS NULL", alb.ID, alb.UserID).
		Where("images.visibility = ? AND images.status = ?", "public", "normal").
		Where("(images.expires_at IS NULL OR images.expires_at > ?)", timeNow()).
		Where("(images.access_password_hash = '' OR images.access_password_hash IS NULL)").
		Order("images.id DESC")
	if cursor != "" {
		var cid uint64
		if _, err := fmt.Sscanf(cursor, "%d", &cid); err == nil && cid > 0 {
			q = q.Where("images.id < ?", cid)
		}
	}
	type row struct {
		Key    string
		Name   string
		Ext    string
		Width  int
		Height int
		Size   int64
		ID     uint64
	}
	var rows []row
	if err := q.Limit(limit + 1).Scan(&rows).Error; err != nil {
		return nil, "", err
	}
	next := ""
	if len(rows) > limit {
		next = fmt.Sprintf("%d", rows[limit-1].ID)
		rows = rows[:limit]
	}
	out := make([]PublicImage, 0, len(rows))
	for _, r := range rows {
		out = append(out, PublicImage{Key: r.Key, Name: r.Name, Ext: r.Ext, Width: r.Width, Height: r.Height, Size: r.Size})
	}
	return out, next, nil
}

// timeNow 可测；生产为 time.Now。
var timeNow = func() time.Time { return time.Now() }

func (s *Service) Update(userID, id uint64, name, visibility *string) (*model.Album, error) {
	var alb model.Album
	err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&alb).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" || len(n) > 128 {
			return nil, ErrInvalidName
		}
		updates["name"] = n
	}
	if visibility != nil {
		v, err := normVis(*visibility)
		if err != nil {
			return nil, err
		}
		updates["visibility"] = v
	}
	if len(updates) > 0 {
		if err := s.db.Model(&alb).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return &alb, nil
}

// Delete 删除相册。withImages=false：图片移入未分类(album_id=NULL)；true：软删相册内图片。
func (s *Service) Delete(userID, id uint64, withImages bool) error {
	var alb model.Album
	err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&alb).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if withImages {
			// 软删相册内所有 live 图（进各自属主回收站，30 天后由清理任务退配额）
			if err := tx.Where("album_id = ? AND user_id = ?", id, userID).Delete(&model.Image{}).Error; err != nil {
				return err
			}
		}
		// 清空该相册所有图片(含已在回收站者)的 album_id，避免悬挂指向已删相册
		if err := tx.Unscoped().Model(&model.Image{}).
			Where("album_id = ? AND user_id = ?", id, userID).
			Update("album_id", nil).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Album{}, "id = ?", id).Error
	})
}
