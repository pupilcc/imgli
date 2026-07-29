package stats

import (
	"errors"
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/settings"
)

func TestHotlinkAllowed(t *testing.T) {
	cfg := HotlinkConfig{Enabled: true, AllowedDomains: []string{"ok.example", "*.wild.example"}, AllowEmptyReferer: false}
	cases := []struct {
		ref  string
		want bool
	}{
		{"", false}, {"ok.example", true}, {"OK.Example", true}, {"evil.example", false},
		{"a.wild.example", true}, {"wild.example", true}, {"awild.example", false},
		{"img.li", true}, // ownHost 恒放行
	}
	for _, c := range cases {
		if got := HotlinkAllowed(cfg, c.ref, "img.li"); got != c.want {
			t.Errorf("ref=%q got %v want %v", c.ref, got, c.want)
		}
	}
	cfg.AllowEmptyReferer = true
	if !HotlinkAllowed(cfg, "", "img.li") {
		t.Error("allow_empty_referer 应放行空 Referer")
	}
	if !HotlinkAllowed(HotlinkConfig{Enabled: false}, "evil.example", "img.li") {
		t.Error("未启用应恒放行")
	}
}

func TestRecordFlushUpsert(t *testing.T) {
	db := model.TestDB(t)
	s := New(db, time.Hour)
	s.Record(1, "a.example")
	s.Record(1, "a.example")
	s.Record(2, "")
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	s.Record(1, "b.example")
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	var as model.AccessStat
	if err := db.First(&as, "image_id = ?", 1).Error; err != nil || as.Views != 3 {
		t.Errorf("image 1 views=%d err=%v, want 3", as.Views, err)
	}
	var direct model.RefererStat
	if err := db.First(&direct, "host = ?", "(direct)").Error; err != nil || direct.Count != 1 {
		t.Errorf("(direct) count=%d err=%v, want 1", direct.Count, err)
	}
	var n int64
	db.Model(&model.RefererStat{}).Count(&n)
	if n != 3 {
		t.Errorf("referer 行数=%d want 3", n)
	}
	var ri model.RefererImageStat
	if err := db.First(&ri, "image_id = ? AND host = ?", 1, "a.example").Error; err != nil || ri.Count != 2 {
		t.Errorf("refimg a.example count=%d err=%v want 2", ri.Count, err)
	}
}

func TestPurgeOlderThan(t *testing.T) {
	db := model.TestDB(t)
	s := New(db, time.Hour)
	old := time.Now().AddDate(0, 0, -100).Format("2006-01-02")
	if err := db.Create(&model.RefererStat{Host: "old.example", Date: old, Count: 9}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.PurgeOlderThan(90); err != nil {
		t.Fatal(err)
	}
	var n int64
	db.Model(&model.RefererStat{}).Where("host = ?", "old.example").Count(&n)
	if n != 0 {
		t.Fatalf("old rows remain: %d", n)
	}
}

// TestImageStatsOwnerOnly:造两用户(直接 Create model.User{Username,Email,GroupID:播种默认组ID})
// 各一图(Create model.File + model.Image{Key,UserID,FileID});给属主图写两日 AccessStat
// (昨天 3 次/今天 2 次)→ ImageStats(属主, key) total==5,daily 恒 30 项升序、
// 昨日项 Views==3、今日项 Views==2、其余 0;ImageStats(他人ID, key) 与
// ImageStats(属主, "nokey") 均返回 ErrNotFound(errors.Is)。
func TestImageStatsOwnerOnly(t *testing.T) {
	db := model.TestDB(t)
	s := New(db, time.Hour)

	owner := &model.User{Username: "owner", Email: "owner@img.li", GroupID: 1}
	other := &model.User{Username: "other", Email: "other@img.li", GroupID: 1}
	if err := db.Create(owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(other).Error; err != nil {
		t.Fatal(err)
	}

	f1 := &model.File{Hash: "ownerhash", StoragePolicyID: 1, Path: "p/owner", Size: 1, MIME: "image/png", RefCount: 1}
	f2 := &model.File{Hash: "otherhash", StoragePolicyID: 1, Path: "p/other", Size: 1, MIME: "image/png", RefCount: 1}
	if err := db.Create(f1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(f2).Error; err != nil {
		t.Fatal(err)
	}
	imgOwner := &model.Image{Key: "ownerkey00001", UserID: &owner.ID, FileID: f1.ID, Name: "o", Ext: "png", Visibility: "public", Status: "normal"}
	imgOther := &model.Image{Key: "otherkey00001", UserID: &other.ID, FileID: f2.ID, Name: "x", Ext: "png", Visibility: "public", Status: "normal"}
	if err := db.Create(imgOwner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(imgOther).Error; err != nil {
		t.Fatal(err)
	}

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if err := db.Create(&model.AccessStat{ImageID: imgOwner.ID, Date: yesterday, Views: 3}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.AccessStat{ImageID: imgOwner.ID, Date: today, Views: 2}).Error; err != nil {
		t.Fatal(err)
	}

	total, daily, err := s.ImageStats(owner.ID, "ownerkey00001")
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 {
		t.Errorf("total=%d want 5", total)
	}
	if len(daily) != 30 {
		t.Fatalf("daily len=%d want 30", len(daily))
	}
	for i := 1; i < len(daily); i++ {
		if daily[i].Date < daily[i-1].Date {
			t.Errorf("daily 未升序: %s < %s", daily[i].Date, daily[i-1].Date)
		}
	}
	if daily[len(daily)-1].Date != today || daily[len(daily)-1].Views != 2 {
		t.Errorf("今日项 date=%s views=%d want date=%s views=2", daily[len(daily)-1].Date, daily[len(daily)-1].Views, today)
	}
	if daily[len(daily)-2].Date != yesterday || daily[len(daily)-2].Views != 3 {
		t.Errorf("昨日项 date=%s views=%d want date=%s views=3", daily[len(daily)-2].Date, daily[len(daily)-2].Views, yesterday)
	}
	for i, d := range daily {
		if d.Date == today || d.Date == yesterday {
			continue
		}
		if d.Views != 0 {
			t.Errorf("daily[%d] date=%s views=%d want 0", i, d.Date, d.Views)
		}
	}

	if _, _, err := s.ImageStats(other.ID, "ownerkey00001"); !errors.Is(err, ErrNotFound) {
		t.Errorf("他人属主应 ErrNotFound, got %v", err)
	}
	if _, _, err := s.ImageStats(owner.ID, "nokey"); !errors.Is(err, ErrNotFound) {
		t.Errorf("nokey 应 ErrNotFound, got %v", err)
	}
}

// TestHotlinkSnapshotInvalidate:TestDB 播种默认 → s.Hotlink().Enabled==false;
// settings.New(db).Set(model.SettingHotlink, HotlinkConfig{Enabled:true, AllowEmptyReferer:true})
// 后立即 Hotlink() 仍 false(30s TTL 内);InvalidateHotlink() 后 Hotlink().Enabled==true。
func TestHotlinkSnapshotInvalidate(t *testing.T) {
	db := model.TestDB(t)
	s := New(db, time.Hour)

	if s.Hotlink().Enabled {
		t.Fatal("播种默认 Enabled 应为 false")
	}
	if err := settings.New(db).Set(model.SettingHotlink, HotlinkConfig{Enabled: true, AllowEmptyReferer: true}); err != nil {
		t.Fatal(err)
	}
	if s.Hotlink().Enabled {
		t.Error("30s TTL 内应仍为缓存 false")
	}
	s.InvalidateHotlink()
	if !s.Hotlink().Enabled {
		t.Error("Invalidate 后应读到 Enabled=true")
	}
}
