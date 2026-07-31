// Package mail SMTP 邮件发送。配置存 settings(每次发送时读,改配置免重启);
// host 空视为未配置(ErrNotConfigured)。发送经 sender 函数,测试可注入桩。
package mail

import (
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/settings"
)

// Config settings `smtp` 键形状(与 admin API/前端 JSON 逐字一致)。
type Config struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	From       string `json:"from"`
	Encryption string `json:"encryption"` // none | starttls | ssl
}

// DefaultConfig 播种/缺省值(host 空=未配置)。
func DefaultConfig() Config { return Config{Port: 587, Encryption: "starttls"} }

var ErrNotConfigured = errors.New("mail: SMTP 未配置")

const dialTimeout = 10 * time.Second

type Service struct {
	db     *gorm.DB
	sender func(cfg Config, to string, msg []byte) error // 测试注入点
}

func New(db *gorm.DB) *Service { return &Service{db: db, sender: smtpSend} }

func (s *Service) config() (Config, error) {
	cfg := DefaultConfig()
	if err := settings.New(s.db).Get(model.SettingSMTP, &cfg); err != nil && !errors.Is(err, settings.ErrNotFound) {
		return cfg, err
	}
	return cfg, nil
}

func (s *Service) siteName() string {
	name := "img.li"
	var v string
	if err := settings.New(s.db).Get(model.SettingSiteName, &v); err == nil && v != "" {
		name = v
	}
	return name
}

// Send 发送一封 HTML 邮件;host 未配置返回 ErrNotConfigured。
func (s *Service) Send(to, subject, htmlBody string) error {
	cfg, err := s.config()
	if err != nil {
		return err
	}
	if cfg.Host == "" {
		return ErrNotConfigured
	}
	return s.sender(cfg, to, buildMessage(cfg.From, to, subject, htmlBody))
}

// SendResetPassword 渲染重置密码模板并发送(auth.Mailer 实现)。lang 透传模板。
func (s *Service) SendResetPassword(to, link, lang string) error {
	sub, html := RenderResetPassword(s.siteName(), link, lang)
	return s.Send(to, sub, html)
}

// SendVerifyEmail 渲染邮箱验证模板并发送(auth.Mailer 实现)。lang 透传模板。
func (s *Service) SendVerifyEmail(to, link, lang string) error {
	sub, html := RenderVerifyEmail(s.siteName(), link, lang)
	return s.Send(to, sub, html)
}

// SendChangeEmail 换绑邮箱确认(auth.Mailer 实现)。
func (s *Service) SendChangeEmail(to, link, lang string) error {
	sub, html := RenderChangeEmail(s.siteName(), link, lang)
	return s.Send(to, sub, html)
}

// SendWelcome 注册欢迎信。baseURL 用于拼设置页链接；SMTP 未配返回 ErrNotConfigured。
func (s *Service) SendWelcome(to, baseURL, lang string) error {
	sub, html := RenderWelcome(s.siteName(), baseURL, lang)
	return s.Send(to, sub, html)
}

// buildMessage 组 RFC5322 信封:中文主题 RFC2047 Q 编码;HTML utf-8 正文 8bit 直发
// (现代 SMTP 普遍 8BITMIME,不做 quoted-printable——取舍见 spec §5)。
func buildMessage(from, to, subject, htmlBody string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=utf-8\r\n\r\n")
	b.WriteString(htmlBody)
	return []byte(b.String())
}

// smtpSend 真实发送:ssl=TLS 直连;starttls=明文拨号后升级;none=明文(仅内网)。
func smtpSend(cfg Config, to string, msg []byte) error {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	var conn net.Conn
	var err error
	if cfg.Encryption == "ssl" {
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: dialTimeout}, "tcp", addr, &tls.Config{ServerName: cfg.Host})
	} else {
		conn, err = net.DialTimeout("tcp", addr, dialTimeout)
	}
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Now().Add(dialTimeout))
	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()
	if cfg.Encryption == "starttls" {
		if err := c.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
			return err
		}
	}
	if cfg.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
			return err
		}
	}
	if err := c.Mail(cfg.From); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
