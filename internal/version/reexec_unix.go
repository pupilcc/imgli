//go:build !windows

package version

import (
	"log/slog"
	"os"
	"syscall"
	"time"
)

// scheduleReexec 在升级成功后延迟用新二进制覆盖当前进程（无需 systemd restart）。
// 给 HTTP 响应留出刷写时间。
func scheduleReexec(exe string) {
	go func() {
		time.Sleep(400 * time.Millisecond)
		args := os.Args
		if len(args) == 0 {
			args = []string{exe}
		} else {
			// argv0 指向新路径；其余参数不变（含 serve -config …）
			args = append([]string{exe}, args[1:]...)
		}
		if err := syscall.Exec(exe, args, os.Environ()); err != nil {
			slog.Error("upgrade reexec failed; restart the process manually", "exe", exe, "err", err)
		}
	}()
}
