//go:build !vips

package imaging

// Backend 返回当前构建的图像后端标识（纯 Go）。
func Backend() string { return "pure-go" }
