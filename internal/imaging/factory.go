//go:build !vips

package imaging

// New 返回当前构建的图片处理器:缺省纯 Go;-tags vips 构建换 libvips(见 factory_vips.go)。
func New() Processor { return NewGo() }
