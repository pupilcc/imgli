// Package settings 读写 settings 表（业务参数唯一来源，后台改、即时生效）。
package settings

import (
	"encoding/json"
	"errors"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
)

var ErrNotFound = errors.New("settings: key not found")

type Service struct{ db *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) Get(key string, dst any) error {
	var row model.Setting
	err := s.db.First(&row, "key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(row.Value), dst)
}

func (s *Service) Set(key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.db.Save(&model.Setting{Key: key, Value: string(b)}).Error
}

// RegistrationMode 返回注册模式 open|invite|closed；缺失或异常时按 open 处理。
func (s *Service) RegistrationMode() string {
	var mode string
	if err := s.Get(model.SettingRegistrationMode, &mode); err != nil || mode == "" {
		return "open"
	}
	return mode
}
