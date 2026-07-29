package bandwidth

import (
	"errors"
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/model"
)

func TestPeriodShanghai(t *testing.T) {
	// 固定 UTC 时刻：2026-07-31 18:00 UTC = 2026-08-01 02:00 CST → 账期 2026-08
	utc := time.Date(2026, 7, 31, 18, 0, 0, 0, time.UTC)
	if got := Period(utc); got != "2026-08" {
		t.Fatalf("Period=%q want 2026-08", got)
	}
	// 同日上午 UTC 仍 7 月
	utc2 := time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
	if got := Period(utc2); got != "2026-07" {
		t.Fatalf("Period=%q want 2026-07", got)
	}
}

func TestEffectiveUsed(t *testing.T) {
	u := &model.User{BandwidthUsedMonth: 100, BandwidthPeriod: "2026-07"}
	if EffectiveUsed(u, "2026-07") != 100 {
		t.Fatal("same period")
	}
	if EffectiveUsed(u, "2026-08") != 0 {
		t.Fatal("rollover should zero")
	}
	if EffectiveUsed(nil, "2026-07") != 0 {
		t.Fatal("nil")
	}
}

func TestCheckAndAdd(t *testing.T) {
	db := model.TestDB(t)
	g := model.UserGroup{
		Name: "bw", StorageQuota: 1 << 30, MaxFileSize: 1 << 20,
		BandwidthQuotaMonth: 1000, AllowedExts: []string{"png"},
	}
	if err := db.Create(&g).Error; err != nil {
		t.Fatal(err)
	}
	u := model.User{Username: "bw1", Email: "bw1@img.li", GroupID: g.ID, Status: "active"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}

	if err := Check(db, u.ID); err != nil {
		t.Fatalf("empty should pass: %v", err)
	}
	if err := Add(db, u.ID, 600); err != nil {
		t.Fatal(err)
	}
	if err := Check(db, u.ID); err != nil {
		t.Fatalf("600/1000 should pass: %v", err)
	}
	if err := Add(db, u.ID, 400); err != nil {
		t.Fatal(err)
	}
	if err := Check(db, u.ID); !errors.Is(err, ErrExceeded) {
		t.Fatalf("want ErrExceeded got %v", err)
	}
	snap, err := SnapshotFor(db, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Used != 1000 || snap.Quota != 1000 {
		t.Fatalf("snap=%+v", snap)
	}

	// unlimited group
	g2 := model.UserGroup{
		Name: "unlim", StorageQuota: 1, MaxFileSize: 1,
		BandwidthQuotaMonth: 0, AllowedExts: []string{"png"},
	}
	db.Create(&g2)
	u2 := model.User{Username: "bw2", Email: "bw2@img.li", GroupID: g2.ID, Status: "active"}
	db.Create(&u2)
	_ = Add(db, u2.ID, 1<<40)
	if err := Check(db, u2.ID); err != nil {
		t.Fatalf("quota 0 unlimited: %v", err)
	}
}

func TestAddPeriodRollover(t *testing.T) {
	db := model.TestDB(t)
	g := model.UserGroup{
		Name: "r", StorageQuota: 1, MaxFileSize: 1,
		BandwidthQuotaMonth: 5000, AllowedExts: []string{"png"},
	}
	db.Create(&g)
	u := model.User{
		Username: "roll", Email: "roll@img.li", GroupID: g.ID, Status: "active",
		BandwidthUsedMonth: 4000, BandwidthPeriod: "1999-01",
	}
	db.Create(&u)
	if err := Add(db, u.ID, 100); err != nil {
		t.Fatal(err)
	}
	var got model.User
	db.First(&got, u.ID)
	if got.BandwidthPeriod != CurrentPeriod() {
		t.Fatalf("period=%q want %q", got.BandwidthPeriod, CurrentPeriod())
	}
	if got.BandwidthUsedMonth != 100 {
		t.Fatalf("used=%d want 100 after rollover", got.BandwidthUsedMonth)
	}
}
