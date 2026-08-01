// Package storage 定义存储驱动接口——全产品最重要的抽象。
// Phase 1 只有 local；Phase 2 起新增 s3/oss/cos/webdav 实现同一接口。
package storage

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"
)

var ErrNotFound = errors.New("storage: object not found")

// LocalRoot 解析 local 策略 root（storagesvc / admin 探针 / doctor 共用）：
// 空串默认 "uploads"；绝对路径原样 Clean；相对路径拼到 dataDir 下。
func LocalRoot(dataDir, root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "uploads"
	}
	if filepath.IsAbs(root) {
		return filepath.Clean(root)
	}
	return filepath.Join(dataDir, root)
}

type Driver interface {
	Put(ctx context.Context, key string, r io.Reader) error
	Open(ctx context.Context, key string) (io.ReadSeekCloser, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

// PresignTTL 是预签名直连 URL 的有效期。故意做成常量而非 admin 可配字段——
// TTL 是安全语义的一部分,旋钮化意味着管理员可以把泄露窗口调到任意大而不自知。
const PresignTTL = 60 * time.Second

// Presigner 是可选能力:驱动能签出短时效的直连 URL,让客户端绕过 app 直取对象。
// local/webdav 等无签名概念的驱动不实现;调用方靠类型断言探测:
//
//	if p, ok := d.(storage.Presigner); ok { ... }
type Presigner interface {
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}
