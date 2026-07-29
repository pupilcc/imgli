package adminsvc

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/settings"
	"github.com/yixian-huang/imgli/internal/service/stats"
)

func TestHotlinkPutSettingsNormalize(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	cfg := stats.HotlinkConfig{
		Enabled:           true,
		AllowedDomains:    []string{" OK.Example ", "*.Wild.Example", "ok.example"},
		AllowEmptyReferer: false,
	}
	if err := svc.PutSettings(map[string]json.RawMessage{"hotlink": rawJSON(t, cfg)}); err != nil {
		t.Fatal(err)
	}
	var got stats.HotlinkConfig
	if err := settings.New(db).Get(model.SettingHotlink, &got); err != nil {
		t.Fatal(err)
	}
	want := []string{"ok.example", "*.wild.example"}
	if !reflect.DeepEqual(got.AllowedDomains, want) {
		t.Errorf("AllowedDomains=%v want %v", got.AllowedDomains, want)
	}
	if !got.Enabled || got.AllowEmptyReferer {
		t.Errorf("flags Enabled=%v AllowEmpty=%v", got.Enabled, got.AllowEmptyReferer)
	}
}

func TestHotlinkPutSettingsInvalid(t *testing.T) {
	svc := New(model.TestDB(t))
	bads := []string{"", "has space.com", "*.", "*.x", "*x.a", "http://a.b"}
	for _, d := range bads {
		cfg := stats.HotlinkConfig{AllowedDomains: []string{d}}
		err := svc.PutSettings(map[string]json.RawMessage{"hotlink": rawJSON(t, cfg)})
		if !errors.Is(err, ErrHotlinkDomainInvalid) {
			t.Errorf("domain %q: err=%v want ErrHotlinkDomainInvalid", d, err)
		}
	}
}

func TestHotlinkGetSettingsDefault(t *testing.T) {
	svc := New(model.TestDB(t))
	m, err := svc.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	hot, ok := m["hotlink"].(stats.HotlinkConfig)
	if !ok {
		t.Fatalf("hotlink 类型 = %T", m["hotlink"])
	}
	def := stats.DefaultHotlink()
	if hot.Enabled != def.Enabled || hot.AllowEmptyReferer != def.AllowEmptyReferer {
		t.Errorf("hotlink flags = %+v want %+v", hot, def)
	}
	if len(hot.AllowedDomains) != 0 {
		t.Errorf("AllowedDomains 默认应空, got %v", hot.AllowedDomains)
	}
}

func TestStatsTrafficAndReferers(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if err := db.Create(&model.AccessStat{ImageID: 1, Date: today, Views: 5}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.AccessStat{ImageID: 1, Date: yesterday, Views: 3}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.RefererStat{Host: "a.example", Date: today, Count: 7}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.RefererStat{Host: "b.example", Date: today, Count: 2}).Error; err != nil {
		t.Fatal(err)
	}

	st, err := svc.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Traffic7d) != 7 {
		t.Fatalf("traffic_7d len=%d want 7", len(st.Traffic7d))
	}
	for i := 1; i < len(st.Traffic7d); i++ {
		if st.Traffic7d[i].Date < st.Traffic7d[i-1].Date {
			t.Errorf("traffic_7d 未升序: %s < %s", st.Traffic7d[i].Date, st.Traffic7d[i-1].Date)
		}
	}
	if st.Traffic7d[6].Date != today || st.Traffic7d[6].Views != 5 {
		t.Errorf("今日 traffic=%+v want date=%s views=5", st.Traffic7d[6], today)
	}
	if st.Traffic7d[5].Date != yesterday || st.Traffic7d[5].Views != 3 {
		t.Errorf("昨日 traffic=%+v want date=%s views=3", st.Traffic7d[5], yesterday)
	}
	for i := 0; i < 5; i++ {
		if st.Traffic7d[i].Views != 0 {
			t.Errorf("traffic_7d[%d]=%+v want 0", i, st.Traffic7d[i])
		}
	}
	if len(st.Traffic30d) != 30 {
		t.Fatalf("traffic_30d len=%d want 30", len(st.Traffic30d))
	}
	if !st.OriginMeteringOnly {
		t.Error("origin_metering_only should be true")
	}
	if len(st.Signups30d) != 30 {
		t.Fatalf("signups_30d len=%d want 30", len(st.Signups30d))
	}
	if len(st.TopReferers) < 2 {
		t.Fatalf("top_referers=%+v want >=2", st.TopReferers)
	}
	if st.TopReferers[0].Host != "a.example" || st.TopReferers[0].Count != 7 {
		t.Errorf("top[0]=%+v want a.example/7", st.TopReferers[0])
	}
	if st.TopReferers[1].Host != "b.example" || st.TopReferers[1].Count != 2 {
		t.Errorf("top[1]=%+v want b.example/2", st.TopReferers[1])
	}
	if len(st.TopReferers30d) < 2 {
		t.Fatalf("top_referers_30d=%+v", st.TopReferers30d)
	}
	// external hosts should be marked suspect without allowlist
	if !st.TopReferers[0].Suspect {
		t.Error("a.example should be suspect")
	}
}
