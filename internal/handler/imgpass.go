package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/auth"
)

// 图片访问口令：cookie 为 HMAC(hash, "v1|"+key)，换口令即失效；不落明文。
const (
	imgPassCookiePrefix = "imgli_ipw_"
	imgPassHeader       = "X-Image-Password"
	imgPassMaxLen       = 128
	imgPassCookieMaxAge = 7 * 24 * 3600 // 7d
)

func imgPassCookieName(key string) string {
	// cookie name: ASCII only; key is base62
	return imgPassCookiePrefix + key
}

func imgPassToken(hash, key string) string {
	mac := hmac.New(sha256.New, []byte(hash))
	_, _ = mac.Write([]byte("v1|" + key))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func imgHasPassword(img *model.Image) bool {
	return img != nil && strings.TrimSpace(img.AccessPasswordHash) != ""
}

// imgPasswordOK 属主恒 true；无口令 true；cookie/header 校验通过 true。
func imgPasswordOK(r *http.Request, img *model.Image) bool {
	if img == nil || !imgHasPassword(img) {
		return true
	}
	// isOwner 在 serve 包内；此处用 Principal 避免循环
	if p := PrincipalFrom(r); p != nil && img.UserID != nil && p.User != nil && *img.UserID == p.User.ID {
		return true
	}
	hash := img.AccessPasswordHash
	want := imgPassToken(hash, img.Key)
	if c, err := r.Cookie(imgPassCookieName(img.Key)); err == nil && c.Value != "" {
		if hmac.Equal([]byte(c.Value), []byte(want)) {
			return true
		}
	}
	// Header unlock (API clients / curl)
	if pw := strings.TrimSpace(r.Header.Get(imgPassHeader)); pw != "" {
		if auth.VerifyPassword(hash, pw) {
			return true
		}
	}
	return false
}

func setImgPassCookie(w http.ResponseWriter, r *http.Request, key, hash string) {
	http.SetCookie(w, &http.Cookie{
		Name:     imgPassCookieName(key),
		Value:    imgPassToken(hash, key),
		Path:     "/",
		MaxAge:   imgPassCookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
		Expires:  time.Now().Add(time.Duration(imgPassCookieMaxAge) * time.Second),
	})
}

func clearImgPassCookie(w http.ResponseWriter, key string) {
	http.SetCookie(w, &http.Cookie{
		Name:     imgPassCookieName(key),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// HashAccessPassword 校验长度后 argon2 哈希；空串表示清除（返回 ""）。
func HashAccessPassword(pw string) (string, error) {
	pw = strings.TrimSpace(pw)
	if pw == "" {
		return "", nil
	}
	if len(pw) > imgPassMaxLen {
		return "", errAccessPasswordTooLong
	}
	if len(pw) < 1 {
		return "", nil
	}
	return auth.HashPassword(pw)
}

var errAccessPasswordTooLong = errString("access_password 过长")

type errString string

func (e errString) Error() string { return string(e) }
