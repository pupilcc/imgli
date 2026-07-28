package local

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/yixian-huang/imgli/internal/storage"
)

func TestPutOpenDeleteRoundTrip(t *testing.T) {
	d, err := New(t.TempDir() + "/uploads")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	key := "2026/07/15/abc123.png"

	if err := d.Put(ctx, key, strings.NewReader("hello")); err != nil {
		t.Fatal(err)
	}
	ok, _ := d.Exists(ctx, key)
	if !ok {
		t.Fatal("Exists = false after Put")
	}
	rc, err := d.Open(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	rc.Close()
	if string(b) != "hello" {
		t.Errorf("content = %q", b)
	}
	if err := d.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Open(ctx, key); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Open after Delete err = %v, want ErrNotFound", err)
	}
}

func TestRejectsPathTraversal(t *testing.T) {
	d, _ := New(t.TempDir())
	ctx := context.Background()
	for _, bad := range []string{"../escape", "a/../../b", "/etc/passwd", "", ".", "./", "b/../"} {
		if err := d.Put(ctx, bad, strings.NewReader("x")); err == nil {
			t.Errorf("Put(%q) 应拒绝路径穿越", bad)
		}
	}
}
