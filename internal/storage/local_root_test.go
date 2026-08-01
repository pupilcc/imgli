package storage

import (
	"path/filepath"
	"testing"
)

func TestLocalRoot(t *testing.T) {
	data := "/var/imgli-data"
	if got := LocalRoot(data, "uploads"); got != filepath.Join(data, "uploads") {
		t.Errorf("relative = %q", got)
	}
	if got := LocalRoot(data, ""); got != filepath.Join(data, "uploads") {
		t.Errorf("empty default = %q", got)
	}
	if got := LocalRoot(data, "  "); got != filepath.Join(data, "uploads") {
		t.Errorf("blank default = %q", got)
	}
	if got := LocalRoot(data, "/abs/store"); got != "/abs/store" {
		t.Errorf("absolute = %q", got)
	}
	if got := LocalRoot(data, "/abs/store/../x"); got != "/abs/x" {
		t.Errorf("absolute clean = %q", got)
	}
	// 无 dataDir 时相对路径仍是相对路径
	if got := LocalRoot("", "uploads"); got != "uploads" {
		t.Errorf("no dataDir relative = %q", got)
	}
}
