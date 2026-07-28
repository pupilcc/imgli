// Package local 本地磁盘存储驱动。
package local

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/yixian-huang/imgli/internal/storage"
)

type Driver struct{ root string }

func New(root string) (*Driver, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	return &Driver{root: abs}, nil
}

// resolve 把存储键映射为根内绝对路径，拒绝任何逃出 root 的键。
func (d *Driver) resolve(key string) (string, error) {
	if strings.HasPrefix(key, "/") {
		return "", fmt.Errorf("非法存储键 %q", key)
	}
	p := filepath.Join(d.root, filepath.FromSlash(key))
	if !strings.HasPrefix(p, d.root+string(filepath.Separator)) {
		return "", fmt.Errorf("非法存储键 %q", key)
	}
	return p, nil
}

func (d *Driver) Put(ctx context.Context, key string, r io.Reader) error {
	p, err := d.resolve(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".put-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p) // 写临时文件再改名，避免半截文件被直链读到
}

func (d *Driver) Open(ctx context.Context, key string) (io.ReadSeekCloser, error) {
	p, err := d.resolve(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if os.IsNotExist(err) {
		return nil, storage.ErrNotFound
	}
	return f, err
}

func (d *Driver) Delete(ctx context.Context, key string) error {
	p, err := d.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (d *Driver) Exists(ctx context.Context, key string) (bool, error) {
	p, err := d.resolve(key)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
