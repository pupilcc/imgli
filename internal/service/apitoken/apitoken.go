// Package apitoken API Token 签发与 Bearer 解析（PicGo/Typora/CLI 通道）。
package apitoken

import (
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/token"
)

const Prefix = "bl_"

var (
	ErrNotFound     = errors.New("apitoken: not found")
	ErrInvalidScope = errors.New("apitoken: scope 仅支持 upload 或 full")
	ErrInvalidName  = errors.New("apitoken: 名称需 1-64 个字符")
)

type Service struct{ db *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db: db} }

// Create 签发 token，返回明文（仅此一次）。
func (s *Service) Create(userID uint64, name, scope string) (string, *model.APIToken, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 64 {
		return "", nil, ErrInvalidName
	}
	if scope != "upload" && scope != "full" {
		return "", nil, ErrInvalidScope
	}
	raw, err := token.Random()
	if err != nil {
		return "", nil, err
	}
	plaintext := Prefix + raw
	t := &model.APIToken{UserID: userID, Name: name, TokenHash: token.Hash(plaintext), Scope: scope}
	if err := s.db.Create(t).Error; err != nil {
		return "", nil, err
	}
	return plaintext, t, nil
}

func (s *Service) List(userID uint64) ([]model.APIToken, error) {
	var list []model.APIToken
	err := s.db.Where("user_id = ?", userID).Order("id").Find(&list).Error
	return list, err
}

func (s *Service) Revoke(userID, id uint64) error {
	res := s.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.APIToken{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// UserByToken 解析 Bearer 明文。无效/封禁 → (nil,"",nil)；命中时节流更新 last_used_at。
func (s *Service) UserByToken(plaintext string) (*model.User, string, error) {
	if !strings.HasPrefix(plaintext, Prefix) {
		return nil, "", nil
	}
	var t model.APIToken
	err := s.db.First(&t, "token_hash = ?", token.Hash(plaintext)).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	var u model.User
	if err := s.db.First(&u, t.UserID).Error; err != nil {
		return nil, "", err
	}
	if u.Status == "banned" {
		return nil, "", nil
	}
	if t.LastUsedAt == nil || time.Since(*t.LastUsedAt) > time.Minute {
		now := time.Now()
		if err := s.db.Model(&t).Update("last_used_at", &now).Error; err != nil {
			slog.Warn("更新 api_token.last_used_at 失败", "token_id", t.ID, "err", err)
		}
	}
	return &u, t.Scope, nil
}
