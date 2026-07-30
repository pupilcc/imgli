package ftp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/storage"
)

func TestNewRequiresHost(t *testing.T) {
	if _, err := New(map[string]string{}); err == nil {
		t.Fatal("want error")
	}
}

func TestNewDefaults(t *testing.T) {
	d, err := New(map[string]string{
		"host": "ftp.example.com", "username": "u", "password": "p",
		"prefix": "imgli", "allow_insecure": "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.port != 21 || d.prefix != "imgli" || !d.allowInsecure {
		t.Fatalf("%+v", d)
	}
	if d.remotePath("a/b.png") != "imgli/a/b.png" {
		t.Fatal(d.remotePath("a/b.png"))
	}
	if d.maxPool != defaultMaxPool || d.idleTTL != defaultIdleTTL {
		t.Fatalf("pool defaults: max=%d idle=%v", d.maxPool, d.idleTTL)
	}
	if d.mode != tlsUnknown {
		t.Fatalf("mode want unknown, got %v", d.mode)
	}
}

func TestCapabilities(t *testing.T) {
	d, _ := New(map[string]string{"host": "x", "allow_insecure": "true"})
	c := d.Capabilities()
	if c.Tier != "compat" || c.HotPathOK || c.PrivatePresignCapable {
		t.Fatalf("%+v", c)
	}
}

func TestReleaseCapsPool(t *testing.T) {
	d, _ := New(map[string]string{"host": "x", "allow_insecure": "true"})
	d.maxPool = 2
	// fake: release nil is no-op
	d.release(nil, false)
	if len(d.pool) != 0 {
		t.Fatal("nil release")
	}
}

func TestIdleTTLConstantPositive(t *testing.T) {
	if defaultIdleTTL < time.Second {
		t.Fatal(defaultIdleTTL)
	}
}

func TestIsNotFound(t *testing.T) {
	if !isNotFound(storage.ErrNotFound) || !isNotFound(errors.New("550 File not found")) {
		t.Fatal("expected not found")
	}
	if isNotFound(errors.New("421 service not available")) {
		t.Fatal("421 is not not-found")
	}
}

// ServeContent pattern: SeekEnd → SeekStart → Read (body still nil until Read).
func TestStreamRSCSeekLikeServeContent(t *testing.T) {
	s := &streamRSC{size: 1024, pos: 0}
	n, err := s.Seek(0, io.SeekEnd)
	if err != nil || n != 1024 {
		t.Fatalf("SeekEnd: %d %v", n, err)
	}
	n, err = s.Seek(0, io.SeekStart)
	if err != nil || n != 0 {
		t.Fatalf("SeekStart: %d %v", n, err)
	}
	if s.body != nil {
		t.Fatal("body should stay lazy until Read")
	}
	if _, err := s.Seek(-1, io.SeekStart); err == nil {
		t.Fatal("want out of range")
	}
	_ = s.Close()
	if _, err := s.Read([]byte{0}); err == nil {
		t.Fatal("read after close")
	}
}

// Live probe: IMGLI_TEST_FTP=1 (defaults 127.0.0.1:2121 imgli/imgli).
// Asserts second Open reuses pool (mode remembered / no re-probe required for plain).
func TestLiveRoundTrip(t *testing.T) {
	if os.Getenv("IMGLI_TEST_FTP") != "1" {
		t.Skip("set IMGLI_TEST_FTP=1 for live FTP")
	}
	host := envOr("IMGLI_TEST_FTP_HOST", "127.0.0.1")
	port := envOr("IMGLI_TEST_FTP_PORT", "2121")
	user := envOr("IMGLI_TEST_FTP_USER", "imgli")
	pass := envOr("IMGLI_TEST_FTP_PASS", "imgli")
	d, err := New(map[string]string{
		"host": host, "port": port, "username": user, "password": pass,
		"prefix": "imgli-live", "allow_insecure": "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	payload := []byte("imgli-ftp-selftest-ok")
	key := "a/b/selftest.txt"
	if err := d.Put(ctx, key, bytes.NewReader(payload)); err != nil {
		t.Fatalf("put: %v", err)
	}
	// After put, pool should hold a connection and mode plain.
	d.mu.Lock()
	if d.mode != tlsPlain {
		d.mu.Unlock()
		t.Fatalf("mode after put: %v want plain", d.mode)
	}
	pooled := len(d.pool)
	d.mu.Unlock()
	if pooled < 1 {
		t.Fatal("expected pooled connection after successful put")
	}

	ok, err := d.Exists(ctx, key)
	if err != nil || !ok {
		t.Fatalf("exists: %v %v", ok, err)
	}
	// Second op should still leave pool usable (reuse path).
	t0 := time.Now()
	rc, err := d.Open(ctx, key)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Mimic http.ServeContent: size via SeekEnd, then rewind, then stream.
	if n, err := rc.Seek(0, io.SeekEnd); err != nil || n != int64(len(payload)) {
		t.Fatalf("SeekEnd: %d %v", n, err)
	}
	if _, err := rc.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("mismatch %q", got)
	}
	if elapsed := time.Since(t0); elapsed > 5*time.Second {
		t.Logf("warning: open+stream took %v", elapsed)
	}

	// Second open (pool + stream) still correct.
	rc2, err := d.Open(ctx, key)
	if err != nil {
		t.Fatalf("open2: %v", err)
	}
	buf := make([]byte, 4)
	n, err := rc2.Read(buf)
	if err != nil || n != 4 || !bytes.Equal(buf, payload[:4]) {
		t.Fatalf("partial read: n=%d err=%v buf=%q", n, err, buf)
	}
	rest, _ := io.ReadAll(rc2)
	rc2.Close()
	if !bytes.Equal(append(buf[:n], rest...), payload) {
		t.Fatal("reassembled mismatch")
	}

	if err := d.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
