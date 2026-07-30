package ftp

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
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
}

func TestCapabilities(t *testing.T) {
	d, _ := New(map[string]string{"host": "x", "allow_insecure": "true"})
	c := d.Capabilities()
	if c.Tier != "compat" || c.HotPathOK || c.PrivatePresignCapable {
		t.Fatalf("%+v", c)
	}
}

// Live probe: IMGLI_TEST_FTP=1 plus host/user/pass env (defaults 127.0.0.1:2121 imgli/imgli).
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
	ctx := context.Background()
	payload := []byte("imgli-ftp-selftest-ok")
	key := "a/b/selftest.txt"
	if err := d.Put(ctx, key, bytes.NewReader(payload)); err != nil {
		t.Fatalf("put: %v", err)
	}
	ok, err := d.Exists(ctx, key)
	if err != nil || !ok {
		t.Fatalf("exists: %v %v", ok, err)
	}
	rc, err := d.Open(ctx, key)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("mismatch %q", got)
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
