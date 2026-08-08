package storage_test

import (
	"testing"

	"github.com/yixian-huang/imgli/internal/storage"
	"github.com/yixian-huang/imgli/internal/storage/s3"
)

func TestCapsForDriverKnown(t *testing.T) {
	for _, d := range []string{"local", "s3", "webdav", "ftp"} {
		c, err := storage.CapsForDriver(d)
		if err != nil {
			t.Fatalf("%s: %v", d, err)
		}
		if c.SummaryKey == "" {
			t.Fatalf("%s: empty summary", d)
		}
		if c.ListPrefix || c.MultipartUpload {
			t.Fatalf("%s: list/multipart must be false until Driver exposes them", d)
		}
	}
	if _, err := storage.CapsForDriver("oss"); err == nil {
		t.Fatal("unknown driver should error")
	}
}

func TestPrivatePresignCapableMatchesPresigner(t *testing.T) {
	s3c, _ := storage.CapsForDriver("s3")
	if !s3c.PrivatePresignCapable {
		t.Fatal("s3 should be presign capable")
	}
	d, err := s3.New(map[string]string{
		"endpoint": "http://127.0.0.1:9000", "region": "us-east-1", "bucket": "b",
		"access_key_id": "a", "secret_access_key": "s", "path_style": "true",
		"presign_domain": "https://s3.example",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := any(d).(storage.Presigner); !ok {
		t.Fatal("s3.Driver must implement Presigner")
	}
	for _, name := range []string{"local", "webdav", "ftp"} {
		c, _ := storage.CapsForDriver(name)
		if c.PrivatePresignCapable {
			t.Fatalf("%s must not claim private presign", name)
		}
	}
}

func TestEffectivePresignReady(t *testing.T) {
	eff, err := storage.EffectiveFor("s3", map[string]string{"presign_domain": ""}, "")
	if err != nil {
		t.Fatal(err)
	}
	if eff.PrivatePresignReady {
		t.Fatal("empty presign_domain → not ready")
	}
	eff, _ = storage.EffectiveFor("s3", map[string]string{"presign_domain": "https://x"}, "")
	if !eff.PrivatePresignReady {
		t.Fatal("want ready")
	}
}

func TestWarningsCDN(t *testing.T) {
	c, _ := storage.CapsForDriver("local")
	eff, _ := storage.EffectiveFor("local", map[string]string{"root": "u"}, "https://cdn.example")
	w := storage.WarningsFor("local", map[string]string{"root": "u"}, "https://cdn.example", true, c, eff)
	found := false
	for _, x := range w {
		if x.Code == "cdn_not_recommended" {
			found = true
		}
	}
	if !found {
		t.Fatalf("want cdn_not_recommended, got %#v", w)
	}
}

func TestValidateCDNDomain(t *testing.T) {
	if err := storage.ValidateCDNDomain(""); err != nil {
		t.Fatal(err)
	}
	if err := storage.ValidateCDNDomain("https://cdn.example/img/"); err != nil {
		t.Fatal(err)
	}
	if err := storage.ValidateCDNDomain("https://user:pass@cdn.example"); err == nil {
		t.Fatal("userinfo should fail")
	}
	if err := storage.ValidateCDNDomain("https://cdn.example?x=1"); err == nil {
		t.Fatal("query should fail")
	}
	if err := storage.ValidateCDNDomain("not-a-url"); err == nil {
		t.Fatal("want error")
	}
}

func TestWarningsPathStyleVendor(t *testing.T) {
	c, _ := storage.CapsForDriver("s3")
	cfg := map[string]string{
		"endpoint": "oss-cn-hangzhou.aliyuncs.com", "path_style": "true",
		"presign_domain": "https://s3.example",
	}
	eff, _ := storage.EffectiveFor("s3", cfg, "")
	w := storage.WarningsFor("s3", cfg, "", true, c, eff)
	found := false
	for _, x := range w {
		if x.Code == "path_style_vendor" {
			found = true
		}
	}
	if !found {
		t.Fatalf("want path_style_vendor, got %#v", w)
	}
	cfg["path_style"] = "false"
	w = storage.WarningsFor("s3", cfg, "", true, c, eff)
	for _, x := range w {
		if x.Code == "path_style_vendor" {
			t.Fatal("virtual-host should not warn path_style_vendor")
		}
	}
}
