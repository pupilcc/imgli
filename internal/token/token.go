// Package token 生成与哈希不透明令牌（session、API token 共用）。
// 明文只交给用户；库中一律存 Hash 值。
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// Random 返回 base64url 编码的 32 字节随机串（43 字符，无填充）。
func Random() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Hash 返回 raw 的 SHA-256 十六进制（64 字符），用作库中存储键。
func Hash(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
