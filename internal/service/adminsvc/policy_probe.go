package adminsvc

import (
	"errors"
	"strings"

	"github.com/yixian-huang/imgli/internal/storage"
)

// 探针错误只做「可读的一句话」：动作 + 关键路径/地址 + 底层原因 + 可选短建议。
// 不维护机器码枚举、不向 API 塞结构化 probe 树。

func remoteEndpoint(driver string, cfg map[string]string) string {
	if cfg == nil {
		return ""
	}
	switch driver {
	case "webdav", "s3":
		return strings.TrimSpace(cfg["endpoint"])
	case "ftp":
		if h := strings.TrimSpace(cfg["host"]); h != "" {
			return h
		}
		return strings.TrimSpace(cfg["endpoint"])
	default:
		return ""
	}
}

// formatLocalProbeErr 例：
// root 不可写: /data/uploads: permission denied（config.root=uploads, data_dir=/data）。请确认…
func formatLocalProbeErr(action, absRoot, cfgRoot, dataDir string, cause error) error {
	var b strings.Builder
	b.WriteString(action)
	b.WriteString(": ")
	b.WriteString(absRoot)
	if cause != nil {
		b.WriteString(": ")
		b.WriteString(cause.Error())
	}
	if cfgRoot != "" && cfgRoot != absRoot {
		b.WriteString("（config.root=")
		b.WriteString(cfgRoot)
		if dataDir != "" {
			b.WriteString(", data_dir=")
			b.WriteString(dataDir)
		}
		b.WriteString("）")
	}
	if h := probeHint(cause); h != "" {
		b.WriteString("。")
		b.WriteString(h)
	}
	return &probeMsgError{msg: b.String(), cause: cause}
}

// formatRemoteProbeErr 例：写入探针失败 (webdav https://dav.example/dav): connection refused。请检查…
func formatRemoteProbeErr(action, driver, endpoint string, cause error) error {
	var b strings.Builder
	b.WriteString(action)
	b.WriteString(" (")
	b.WriteString(driver)
	if endpoint != "" {
		b.WriteByte(' ')
		b.WriteString(endpoint)
	}
	b.WriteByte(')')
	if cause != nil {
		b.WriteString(": ")
		b.WriteString(cause.Error())
	}
	if h := probeHint(cause); h != "" {
		b.WriteString("。")
		b.WriteString(h)
	}
	return &probeMsgError{msg: b.String(), cause: cause}
}

func formatProbeMsg(action string, cause error) error {
	msg := action
	if cause != nil {
		msg += ": " + cause.Error()
	}
	if h := probeHint(cause); h != "" {
		msg += "。" + h
	}
	return &probeMsgError{msg: msg, cause: cause}
}

type probeMsgError struct {
	msg   string
	cause error
}

func (e *probeMsgError) Error() string {
	if e == nil {
		return "探针失败"
	}
	return e.msg
}

func (e *probeMsgError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// probeHint 仅在高频可识别场景给一句短建议；识别不了则空串。
func probeHint(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, storage.ErrNotFound) {
		return "请核对 endpoint/路径是否指向可写目录，并确认账号有写权限"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "permission denied"),
		strings.Contains(msg, "read-only"),
		strings.Contains(msg, "operation not permitted"):
		return "请确认进程用户对该路径可写（Docker 检查 /data volume 属主）"
	case strings.Contains(msg, "401"), strings.Contains(msg, "403"),
		strings.Contains(msg, "认证"), strings.Contains(msg, "权限失败"):
		return "请检查用户名/密码与写权限"
	case strings.Contains(msg, "connection refused"), strings.Contains(msg, "no such host"),
		strings.Contains(msg, "dial tcp"), strings.Contains(msg, "network is unreachable"):
		return "请检查地址/端口与服务是否在监听"
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline exceeded"):
		return "请检查网络连通性与防火墙"
	default:
		return ""
	}
}
