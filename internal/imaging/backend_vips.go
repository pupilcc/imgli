//go:build vips

package imaging

// Backend 返回当前构建的图像后端标识（libvips）。
func Backend() string { return "vips" }
