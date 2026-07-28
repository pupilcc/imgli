//go:build vips

package imaging

// New(vips 构建)返回 libvips 处理器:缩略图 WebP q80、解码显著更快。
func New() Processor { return NewVips() }
