package imaging

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// VipsConcurrency 限制 libvips 并行度（Docker/小机器默认 cap 为 2，避免大图 OOM）。
// 环境变量 VIPS_CONCURRENCY 可覆盖（0=串行）。纯 Go 构建不调用 libvips，但共享同一策略文档。
func VipsConcurrency() int {
	if v := strings.TrimSpace(os.Getenv("VIPS_CONCURRENCY")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	n := runtime.GOMAXPROCS(0)
	if n > 2 {
		return 2
	}
	if n < 1 {
		return 1
	}
	return n
}
