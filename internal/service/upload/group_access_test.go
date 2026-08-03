package upload

import (
	"errors"
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/model"
)

func TestApplyGroupAccessPermanentForbiddenUsesDefault(t *testing.T) {
	g := &model.UserGroup{DefaultExpiresIn: 3600, MaxExpiresIn: 86400}
	opts := Opts{} // permanent
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	if err := ApplyGroupAccess(g, &opts, now); err != nil {
		t.Fatal(err)
	}
	if opts.ExpiresAt == nil {
		t.Fatal("应补默认有效期")
	}
	want := now.Add(time.Hour)
	if d := opts.ExpiresAt.Sub(want); d < -time.Second || d > time.Second {
		t.Fatalf("ExpiresAt=%v want ~%v", opts.ExpiresAt, want)
	}
}

func TestApplyGroupAccessOverCapRejected(t *testing.T) {
	g := &model.UserGroup{MaxExpiresIn: 3600}
	now := time.Now()
	far := now.Add(48 * time.Hour)
	opts := Opts{ExpiresAt: &far}
	if err := ApplyGroupAccess(g, &opts, now); !errors.Is(err, ErrExpiresOverGroup) {
		t.Fatalf("err=%v want ErrExpiresOverGroup", err)
	}
}

func TestApplyGroupAccessForceMaxAgeCaps(t *testing.T) {
	g := &model.UserGroup{ForceMaxAgeDays: 1, MaxExpiresIn: 30 * 86400}
	opts := Opts{}
	now := time.Now()
	if err := ApplyGroupAccess(g, &opts, now); err != nil {
		t.Fatal(err)
	}
	if opts.ExpiresAt == nil {
		t.Fatal("force max age 应禁止永久")
	}
	maxAt := now.Add(24 * time.Hour)
	if opts.ExpiresAt.After(maxAt.Add(time.Second)) {
		t.Fatalf("应受 ForceMaxAgeDays 约束: %v", opts.ExpiresAt)
	}
}

func TestApplyGroupAccessMaxViews(t *testing.T) {
	g := &model.UserGroup{DefaultMaxViews: 3, MaxMaxViews: 10}
	opts := Opts{MaxViews: 0}
	if err := ApplyGroupAccess(g, &opts, time.Now()); err != nil {
		t.Fatal(err)
	}
	if opts.MaxViews != 3 {
		t.Fatalf("MaxViews=%d want 3", opts.MaxViews)
	}
	opts.MaxViews = 20
	if err := ApplyGroupAccess(g, &opts, time.Now()); !errors.Is(err, ErrMaxViewsOverGroup) {
		t.Fatalf("err=%v want ErrMaxViewsOverGroup", err)
	}
}

func TestApplyGroupAccessDefaultOnlySoftWhenNoCap(t *testing.T) {
	// 仅默认、无 cap：后端不强制；永久仍允许（默认只给 UI）。
	g := &model.UserGroup{DefaultExpiresIn: 7200}
	opts := Opts{}
	if err := ApplyGroupAccess(g, &opts, time.Now()); err != nil {
		t.Fatal(err)
	}
	if opts.ExpiresAt != nil {
		t.Fatalf("无 cap 时不应强制默认, got %v", opts.ExpiresAt)
	}
}
