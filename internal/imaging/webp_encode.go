//go:build !vips

package imaging

import "image"

// WebPEncodeAvailable 纯 Go 构建无有损 WebP 编码器。
func WebPEncodeAvailable() bool { return false }

// EncodeWebP 纯 Go 构建不支持；返回 ErrUnsupported。
func EncodeWebP(_ image.Image, _ int) ([]byte, error) {
	return nil, ErrUnsupported
}
