//go:build windows

package version

import "log/slog"

// Windows 上不自动 re-exec：请手动重启服务/进程。
func scheduleReexec(exe string) {
	slog.Info("upgrade: binary replaced; restart imgli manually on Windows", "exe", exe)
}
