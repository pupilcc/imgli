package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/settings"
)

// OIDCSettingKey settings 键。
const OIDCSettingKey = "oidc"

// OIDCConfig 管理端配置。
type OIDCConfig struct {
	Enabled      bool   `json:"enabled"`
	Issuer       string `json:"issuer"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// OIDCService 可选 OIDC 登录（单实例）。
type OIDCService struct {
	db      *gorm.DB
	auth    *Service
	baseURL string

	mu       sync.Mutex
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth    oauth2.Config
	cfgHash  string
}

func NewOIDC(db *gorm.DB, auth *Service, baseURL string) *OIDCService {
	return &OIDCService{db: db, auth: auth, baseURL: strings.TrimRight(baseURL, "/")}
}

func (o *OIDCService) LoadConfig() OIDCConfig {
	var c OIDCConfig
	_ = settings.New(o.db).Get(OIDCSettingKey, &c)
	return c
}

func (o *OIDCService) SaveConfig(c OIDCConfig) error {
	c.Issuer = strings.TrimRight(strings.TrimSpace(c.Issuer), "/")
	c.ClientID = strings.TrimSpace(c.ClientID)
	return settings.New(o.db).Set(OIDCSettingKey, c)
}

func (o *OIDCService) ensure(ctx context.Context) error {
	c := o.LoadConfig()
	if !c.Enabled || c.Issuer == "" || c.ClientID == "" {
		return errors.New("oidc 未启用或配置不完整")
	}
	h := c.Issuer + "|" + c.ClientID + "|" + c.ClientSecret
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.provider != nil && o.cfgHash == h {
		return nil
	}
	p, err := oidc.NewProvider(ctx, c.Issuer)
	if err != nil {
		return fmt.Errorf("oidc provider: %w", err)
	}
	redirect := o.baseURL + "/api/v1/auth/oidc/callback"
	o.provider = p
	o.verifier = p.Verifier(&oidc.Config{ClientID: c.ClientID})
	o.oauth = oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Endpoint:     p.Endpoint(),
		RedirectURL:  redirect,
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
	o.cfgHash = h
	return nil
}

// AuthCodeURL 生成授权 URL；state 调用方生成并 cookie 保存。
func (o *OIDCService) AuthCodeURL(ctx context.Context, state string) (string, error) {
	if err := o.ensure(ctx); err != nil {
		return "", err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.oauth.AuthCodeURL(state), nil
}

// Exchange 用 code 换 token 并确保本地用户，返回 session 用户。
func (o *OIDCService) Exchange(ctx context.Context, code string) (*model.User, error) {
	if err := o.ensure(ctx); err != nil {
		return nil, err
	}
	o.mu.Lock()
	oauthCfg := o.oauth
	verifier := o.verifier
	o.mu.Unlock()

	tok, err := oauthCfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	rawID, ok := tok.Extra("id_token").(string)
	if !ok || rawID == "" {
		return nil, errors.New("missing id_token")
	}
	idTok, err := verifier.Verify(ctx, rawID)
	if err != nil {
		return nil, fmt.Errorf("id_token: %w", err)
	}
	var claims struct {
		Email             string `json:"email"`
		EmailVerified     bool   `json:"email_verified"`
		PreferredUsername string `json:"preferred_username"`
		Name              string `json:"name"`
		Sub               string `json:"sub"`
	}
	if err := idTok.Claims(&claims); err != nil {
		return nil, err
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	if email == "" {
		return nil, errors.New("oidc 未提供 email")
	}
	// find or create user by email
	var u model.User
	err = o.db.Where("email = ?", email).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		uname := claims.PreferredUsername
		if uname == "" {
			uname = strings.Split(email, "@")[0]
		}
		uname = sanitizeUsername(uname)
		// ensure unique
		base := uname
		for i := 0; i < 20; i++ {
			cand := base
			if i > 0 {
				cand = fmt.Sprintf("%s%d", base, i)
			}
			var n int64
			_ = o.db.Model(&model.User{}).Where("username = ?", cand).Count(&n)
			if n == 0 {
				uname = cand
				break
			}
		}
		// random unusable password
		pw, _ := HashPassword(randomSecret(32))
		now := time.Now()
		u = model.User{
			Username: uname, Email: email, PasswordHash: pw,
			Nickname: claims.Name, GroupID: 1, Status: "active",
		}
		if claims.EmailVerified {
			u.EmailVerifiedAt = &now
		}
		// default group: non-guest
		var g model.UserGroup
		if err := o.db.Where("is_guest = ?", false).Order("id").First(&g).Error; err == nil {
			u.GroupID = g.ID
		}
		if err := o.db.Create(&u).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if u.Status == "banned" {
		return nil, errors.New("账号已封禁")
	}
	return &u, nil
}

func sanitizeUsername(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) < 3 {
		out = "user" + out
	}
	if len(out) > 32 {
		out = out[:32]
	}
	return out
}

func randomSecret(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// RandomState 生成 OAuth state。
func RandomState() string { return randomSecret(24) }

// EnabledPublic 公开配置是否展示 OIDC 入口。
func (o *OIDCService) EnabledPublic() bool {
	c := o.LoadConfig()
	return c.Enabled && c.Issuer != "" && c.ClientID != ""
}

// MarshalConfig 调试用
func MarshalConfig(c OIDCConfig) string {
	b, _ := json.Marshal(c)
	return string(b)
}
