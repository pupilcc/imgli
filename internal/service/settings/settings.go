// Package settings 读写 settings 表（业务参数唯一来源，后台改、即时生效）。
package settings

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
)

var ErrNotFound = errors.New("settings: key not found")

const boolTTL = 30 * time.Second

type boolSnap struct {
	v     bool
	at    time.Time
	valid bool
	gen   uint64
}

type Service struct {
	db *gorm.DB

	// bool 键短 TTL 缓存（guest_upload / plaza 等热路径），Set/Invalidate 立即失效。
	boolMu    sync.Mutex
	boolCache map[string]boolSnap
}

func New(db *gorm.DB) *Service {
	return &Service{db: db, boolCache: make(map[string]boolSnap)}
}

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
	if err := s.db.Save(&model.Setting{Key: key, Value: string(b)}).Error; err != nil {
		return err
	}
	s.Invalidate(key)
	return nil
}

// Invalidate 使某键的 bool 缓存立即失效（多写路径或测试用）。
func (s *Service) Invalidate(key string) {
	s.boolMu.Lock()
	snap := s.boolCache[key]
	snap.valid = false
	snap.gen++
	s.boolCache[key] = snap
	s.boolMu.Unlock()
}

// GetBool 读 bool 设置，带 30s TTL 缓存；缺失或错误返回 def（fail-closed 由调用方选 def）。
// 过期时乐观续期：并发请求拿旧值，单次回源；Set 后 Invalidate 立即生效。
func (s *Service) GetBool(key string, def bool) bool {
	s.boolMu.Lock()
	snap := s.boolCache[key]
	if snap.valid && time.Since(snap.at) < boolTTL {
		v := snap.v
		s.boolMu.Unlock()
		return v
	}
	stale := snap
	gen := snap.gen
	// 乐观续期：推 at，让并发者走快路径拿旧值
	snap.at = time.Now()
	s.boolCache[key] = snap
	s.boolMu.Unlock()

	var v bool
	err := s.Get(key, &v)
	if err != nil {
		if stale.valid {
			return stale.v
		}
		return def
	}

	s.boolMu.Lock()
	if s.boolCache[key].gen == gen {
		s.boolCache[key] = boolSnap{v: v, at: time.Now(), valid: true, gen: gen}
	}
	s.boolMu.Unlock()
	return v
}

// GetBoolStrict 读 bool：缺失视为 false；其它 DB/JSON 错误返回 err（广场开关等需 500）。
// 成功结果同样走 30s 缓存。
func (s *Service) GetBoolStrict(key string) (bool, error) {
	s.boolMu.Lock()
	snap := s.boolCache[key]
	if snap.valid && time.Since(snap.at) < boolTTL {
		v := snap.v
		s.boolMu.Unlock()
		return v, nil
	}
	stale := snap
	gen := snap.gen
	snap.at = time.Now()
	s.boolCache[key] = snap
	s.boolMu.Unlock()

	var v bool
	err := s.Get(key, &v)
	if errors.Is(err, ErrNotFound) {
		v = false
		err = nil
	}
	if err != nil {
		if stale.valid {
			return stale.v, nil
		}
		return false, err
	}

	s.boolMu.Lock()
	if s.boolCache[key].gen == gen {
		s.boolCache[key] = boolSnap{v: v, at: time.Now(), valid: true, gen: gen}
	}
	s.boolMu.Unlock()
	return v, nil
}

// GuestUploadEnabled guest_upload_enabled；缺失/错误按 false（fail closed）。
func (s *Service) GuestUploadEnabled() bool {
	return s.GetBool(model.SettingGuestUpload, false)
}

// PlazaEnabled plaza_enabled；缺失 false；真错误返回 err。
func (s *Service) PlazaEnabled() (bool, error) {
	return s.GetBoolStrict(model.SettingPlazaEnabled)
}

// RegistrationMode 返回注册模式 open|invite|closed；缺失或异常时按 open 处理。
func (s *Service) RegistrationMode() string {
	var mode string
	if err := s.Get(model.SettingRegistrationMode, &mode); err != nil || mode == "" {
		return "open"
	}
	return mode
}
