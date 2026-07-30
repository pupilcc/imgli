// Package webhook 出站事件（upload / moderated），异步 HTTP POST + HMAC 签名。
package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/settings"
)

// SettingKey settings 表键。
const SettingKey = "webhooks"

// Config 管理端可配。
type Config struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
	Secret  string `json:"secret"`
}

// Event 载荷。
type Event struct {
	Type      string         `json:"type"`
	Timestamp string         `json:"timestamp"`
	Data      map[string]any `json:"data"`
}

// Service 读配置并投递。
type Service struct {
	db     *gorm.DB
	client *http.Client
}

func New(db *gorm.DB) *Service {
	return &Service{
		db:     db,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *Service) load() Config {
	var c Config
	if err := settings.New(s.db).Get(SettingKey, &c); err != nil {
		return Config{}
	}
	return c
}

// Emit 异步发送；失败只记日志，不阻塞主路径。
func (s *Service) Emit(eventType string, data map[string]any) {
	if s == nil || s.db == nil {
		return
	}
	go s.emitSync(eventType, data)
}

func (s *Service) emitSync(eventType string, data map[string]any) {
	c := s.load()
	if !c.Enabled || strings.TrimSpace(c.URL) == "" {
		return
	}
	ev := Event{
		Type:      eventType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		slog.Warn("webhook build request", "err", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "imgli-webhook/1")
	if sec := strings.TrimSpace(c.Secret); sec != "" {
		mac := hmac.New(sha256.New, []byte(sec))
		_, _ = mac.Write(body)
		req.Header.Set("X-Imgli-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := s.client.Do(req)
	if err != nil {
		slog.Warn("webhook deliver failed", "type", eventType, "err", err)
		return
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 300 {
		slog.Warn("webhook non-2xx", "type", eventType, "status", resp.StatusCode)
	}
}

// Ensure settings model key exists for docs — also re-export for admin.
var _ = model.SettingSiteName
