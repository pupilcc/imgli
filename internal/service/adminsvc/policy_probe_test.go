package adminsvc

import (
	"errors"
	"strings"
	"testing"

	"github.com/yixian-huang/imgli/internal/storage"
)

func TestProbeHint(t *testing.T) {
	if h := probeHint(storage.ErrNotFound); h == "" {
		t.Fatal("not_found should hint")
	}
	if h := probeHint(errors.New("permission denied")); !strings.Contains(h, "可写") {
		t.Fatalf("permission hint: %q", h)
	}
	if h := probeHint(errors.New("something weird from vendor")); h != "" {
		t.Fatalf("unknown should be empty, got %q", h)
	}
}

func TestFormatLocalProbeErr(t *testing.T) {
	err := formatLocalProbeErr("root 不可写", "/data/uploads", "uploads", "/data",
		errors.New("permission denied"))
	s := err.Error()
	for _, want := range []string{"root 不可写", "/data/uploads", "uploads", "data_dir=/data", "permission denied", "可写"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %q", want, s)
		}
	}
	// absolute-only: no config parenthetical when same
	err2 := formatLocalProbeErr("root 不可写", "/tmp/x", "/tmp/x", "/data", errors.New("permission denied"))
	if strings.Contains(err2.Error(), "config.root") {
		t.Errorf("should omit redundant config when abs==cfg: %s", err2)
	}
}

func TestFormatRemoteProbeErr(t *testing.T) {
	err := formatRemoteProbeErr("写入探针失败", "webdav", "http://127.0.0.1:1",
		errors.New("dial tcp 127.0.0.1:1: connect: connection refused"))
	s := err.Error()
	for _, want := range []string{"写入探针失败", "webdav", "http://127.0.0.1:1", "connection refused", "监听"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %q", want, s)
		}
	}
	cause := errors.New("dial tcp: connection refused")
	err = formatRemoteProbeErr("写入探针失败", "s3", "127.0.0.1:1", cause)
	if !errors.Is(err, cause) {
		t.Fatalf("unwrap: %v", err)
	}
}

func TestFormatWebDAVWriteProbeErrVirtualRootHint(t *testing.T) {
	// 不可达 endpoint：无挂载建议，但 404/not found 应带虚根说明
	err := formatWebDAVWriteProbeErr("https://example.invalid/dav", map[string]string{
		"endpoint": "http://127.0.0.1:1",
		"username": "u",
		"password": "p",
	}, storage.ErrNotFound)
	s := err.Error()
	if !strings.Contains(s, "写入探针失败") {
		t.Fatalf("msg=%s", s)
	}
	// Discover 失败时仍应有 OpenList 虚根提示
	if !strings.Contains(s, "OpenList") && !strings.Contains(s, "虚根") && !strings.Contains(s, "挂载") {
		t.Fatalf("want virtual-root hint, got %s", s)
	}
}
