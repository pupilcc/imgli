package settings

import (
	"errors"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
)

func TestGetSeededAndRoundTrip(t *testing.T) {
	svc := New(model.TestDB(t))

	var name string
	if err := svc.Get(model.SettingSiteName, &name); err != nil || name != "img.li" {
		t.Errorf("site_name = %q, err=%v", name, err)
	}

	if err := svc.Set("announcement", "维护公告"); err != nil {
		t.Fatal(err)
	}
	var ann string
	if err := svc.Get("announcement", &ann); err != nil || ann != "维护公告" {
		t.Errorf("roundtrip = %q, err=%v", ann, err)
	}
	// 覆盖写
	svc.Set("announcement", "新公告")
	svc.Get("announcement", &ann)
	if ann != "新公告" {
		t.Errorf("覆盖写失败: %q", ann)
	}
	// 非字符串值往返（guest_upload_enabled 是 bool）
	if err := svc.Set("bool_key", true); err != nil {
		t.Fatal(err)
	}
	var b bool
	if err := svc.Get("bool_key", &b); err != nil || !b {
		t.Errorf("bool 往返失败: %v err=%v", b, err)
	}
}

func TestGetMissingKey(t *testing.T) {
	svc := New(model.TestDB(t))
	var v string
	if err := svc.Get("no-such-key", &v); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRegistrationMode(t *testing.T) {
	svc := New(model.TestDB(t))
	if m := svc.RegistrationMode(); m != "open" {
		t.Errorf("播种默认 = %q, want open", m)
	}
	svc.Set(model.SettingRegistrationMode, "closed")
	if m := svc.RegistrationMode(); m != "closed" {
		t.Errorf("改后 = %q, want closed", m)
	}
}
