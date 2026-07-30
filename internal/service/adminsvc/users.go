package adminsvc

import (
	"crypto/rand"
	"encoding/base64"
	"errors"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/auth"
)

var (
	ErrUserNotFound  = errors.New("用户不存在")
	ErrGroupNotFound = errors.New("用户组不存在")
	ErrSelfBan       = errors.New("不能封禁自己")
	ErrInvalidStatus = errors.New("status 仅支持 active|banned")
)

// UserRow 是列表项：用户 + 实时算出的图片计数。
type UserRow struct {
	User       model.User
	ImageCount int64
}

const (
	defaultListLimit = 50
	maxListLimit     = 200
)

// ListUsers 按 q（username/email LIKE）、group_id、status、signup channel 筛选；
// sort=bandwidth 时按本月出站降序，否则 id。
func (s *Service) ListUsers(q string, groupID uint64, status, channel, sort string, page, limit int) ([]UserRow, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	tx := s.db.Model(&model.User{})
	if q != "" {
		like := "%" + q + "%"
		tx = tx.Where("username LIKE ? OR email LIKE ?", like, like)
	}
	if groupID > 0 {
		tx = tx.Where("group_id = ?", groupID)
	}
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	if channel != "" {
		tx = tx.Where("signup_channel = ?", channel)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	order := "id"
	if sort == "bandwidth" {
		order = "bandwidth_used_month DESC, id DESC"
	}
	var users []model.User
	if err := tx.Order(order).Offset((page - 1) * limit).Limit(limit).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]UserRow, 0, len(users))
	for i := range users {
		var n int64
		if err := s.db.Model(&model.Image{}).Where("user_id = ?", users[i].ID).Count(&n).Error; err != nil {
			return nil, 0, err
		}
		rows = append(rows, UserRow{User: users[i], ImageCount: n})
	}
	return rows, total, nil
}

// UpdateUser 改组/改状态。禁止对自己置 banned；校验 status 取值、目标用户与目标组存在。
func (s *Service) UpdateUser(actorID, targetID uint64, groupID *uint64, status *string) (*model.User, error) {
	if status != nil {
		if *status != "active" && *status != "banned" {
			return nil, ErrInvalidStatus
		}
		if targetID == actorID && *status == "banned" {
			return nil, ErrSelfBan
		}
	}
	var u model.User
	if err := s.db.First(&u, targetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if groupID != nil {
		var g model.UserGroup
		if err := s.db.First(&g, *groupID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrGroupNotFound
			}
			return nil, err
		}
	}
	updates := map[string]any{}
	if groupID != nil {
		updates["group_id"] = *groupID
		u.GroupID = *groupID
	}
	if status != nil {
		updates["status"] = *status
		u.Status = *status
	}
	if len(updates) == 0 {
		return &u, nil
	}
	if err := s.db.Model(&model.User{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

const resetPasswordLen = 16

// ResetPassword 生成 16 字符一次性明文密码，argon2id 入库，并删除该用户全部 session（强制登出）。
func (s *Service) ResetPassword(targetID uint64) (string, error) {
	var u model.User
	if err := s.db.First(&u, targetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrUserNotFound
		}
		return "", err
	}
	raw := make([]byte, 12) // base64url 无填充：12 字节 → 16 字符
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	plain := base64.RawURLEncoding.EncodeToString(raw)
	hash, err := auth.HashPassword(plain)
	if err != nil {
		return "", err
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("id = ?", u.ID).Update("password_hash", hash).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Session{}, "user_id = ?", u.ID).Error
	})
	if err != nil {
		return "", err
	}
	return plain, nil
}
