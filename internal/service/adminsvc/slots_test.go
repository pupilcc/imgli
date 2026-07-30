package adminsvc

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/settings"
)

func L(zh string, en ...string) LocaleString {
	s := LocaleString{ZH: zh}
	if len(en) > 0 {
		s.EN = en[0]
	}
	return s
}

func TestValidateAnnouncement(t *testing.T) {
	ok := DefaultAnnouncement()
	ok.Enabled = true
	ok.Text = L("hello")
	if err := ValidateAnnouncement(ok); err != nil {
		t.Fatal(err)
	}
	bad := ok
	bad.Text = L("")
	if err := ValidateAnnouncement(bad); !errors.Is(err, ErrAnnouncementInvalid) {
		t.Fatalf("empty text when enabled: %v", err)
	}
	bad = ok
	bad.LinkURL = "javascript:alert(1)"
	if err := ValidateAnnouncement(bad); !errors.Is(err, ErrAnnouncementInvalid) {
		t.Fatalf("js url: %v", err)
	}
	ok.LinkURL = "https://img.li"
	ok.LinkLabel = L("demo")
	if err := ValidateAnnouncement(ok); err != nil {
		t.Fatal(err)
	}
	ok.StartsAt = "2026-01-01T00:00:00Z"
	ok.EndsAt = "2025-01-01T00:00:00Z"
	if err := ValidateAnnouncement(ok); !errors.Is(err, ErrAnnouncementInvalid) {
		t.Fatalf("ends before starts: %v", err)
	}
}

func TestAnnouncementActive(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	a := Announcement{Enabled: true, Text: L("hi"), StartsAt: "2026-07-01T00:00:00Z", EndsAt: "2026-08-01T00:00:00Z"}
	if !AnnouncementActive(a, now) {
		t.Fatal("should be active")
	}
	a.EndsAt = "2026-07-01T00:00:00Z"
	if AnnouncementActive(a, now) {
		t.Fatal("should be expired")
	}
}

func TestValidateFooterAndHTML(t *testing.T) {
	f := Footer{Groups: []FooterGroup{{Title: L("A"), Links: []FooterLink{{Label: L("GitHub"), URL: "https://github.com"}}}}}
	if err := ValidateFooter(f); err != nil {
		t.Fatal(err)
	}
	f.Groups[0].Links[0].URL = "ftp://x"
	if err := ValidateFooter(f); !errors.Is(err, ErrFooterInvalid) {
		t.Fatalf("ftp: %v", err)
	}
	h := HTMLInject{Head: "<script>/*ok*/</script>"}
	if err := ValidateHTMLInject(h); err != nil {
		t.Fatal(err)
	}
	h.Head = string(make([]byte, maxHTMLInjectBytes+1))
	if err := ValidateHTMLInject(h); !errors.Is(err, ErrHTMLInjectInvalid) {
		t.Fatalf("oversize: %v", err)
	}
}

func TestSiteCopySettings(t *testing.T) {
	svc := New(model.TestDB(t))
	patch := map[string]json.RawMessage{
		"help_url":        json.RawMessage(`"https://docs.example.com"`),
		"upgrade_url":     json.RawMessage(`"/upgrade"`),
		"register_notice": json.RawMessage(`"Trial has quotas."`),
		"share_branding":  json.RawMessage(`"links"`),
	}
	if err := svc.PutSettings(patch); err != nil {
		t.Fatal(err)
	}
	m, err := svc.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if m["help_url"] != "https://docs.example.com" {
		t.Fatalf("help_url=%v", m["help_url"])
	}
	if m["share_branding"] != ShareBrandingLinks {
		t.Fatalf("share_branding=%v", m["share_branding"])
	}
	if err := svc.PutSettings(map[string]json.RawMessage{"share_branding": json.RawMessage(`"nope"`)}); !errors.Is(err, ErrShareBrandingInvalid) {
		t.Fatalf("want ErrShareBrandingInvalid got %v", err)
	}
	if err := svc.PutSettings(map[string]json.RawMessage{"help_url": json.RawMessage(`"javascript:x"`)}); !errors.Is(err, ErrHelpURLInvalid) {
		t.Fatalf("want ErrHelpURLInvalid got %v", err)
	}
}

func TestPutSettingsSlotsRoundTrip(t *testing.T) {
	svc := New(model.TestDB(t))
	ann := Announcement{Enabled: true, Text: L("公告"), LinkURL: "/explore", LinkLabel: L("广场"), Dismissible: true}
	raw, _ := json.Marshal(ann)
	if err := svc.PutSettings(map[string]json.RawMessage{"announcement": raw}); err != nil {
		t.Fatal(err)
	}
	foot := Footer{Groups: []FooterGroup{{Title: L("链接"), Links: []FooterLink{{Label: L("GitHub"), URL: "https://github.com/yixian-huang/imgli"}}}}}
	raw, _ = json.Marshal(foot)
	if err := svc.PutSettings(map[string]json.RawMessage{"footer": raw}); err != nil {
		t.Fatal(err)
	}
	html := HTMLInject{Head: "<!-- analytics -->", BodyEnd: ""}
	raw, _ = json.Marshal(html)
	if err := svc.PutSettings(map[string]json.RawMessage{"html_inject": raw}); err != nil {
		t.Fatal(err)
	}
	m, err := svc.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	gotAnn, _ := m["announcement"].(Announcement)
	if !gotAnn.Enabled || gotAnn.Text.Any() != "公告" {
		t.Fatalf("announcement=%+v", gotAnn)
	}
	gotFoot, _ := m["footer"].(Footer)
	if len(gotFoot.Groups) != 1 || gotFoot.Groups[0].Links[0].Label.Any() != "GitHub" {
		t.Fatalf("footer=%+v", gotFoot)
	}
}

// Shared settings instance must Invalidate plaza cache used by DiscoverHandler.
func TestPutSettingsInvalidatesSharedPlazaCache(t *testing.T) {
	db := model.TestDB(t)
	st := settings.New(db)
	// Warm cache: plaza on
	if err := st.Set(model.SettingPlazaEnabled, true); err != nil {
		t.Fatal(err)
	}
	on, err := st.PlazaEnabled()
	if err != nil || !on {
		t.Fatalf("warm: on=%v err=%v", on, err)
	}

	// Admin write via shared instance → must flip cached value immediately
	svc := New(db, st)
	if err := svc.PutSettings(map[string]json.RawMessage{"plaza_enabled": json.RawMessage(`false`)}); err != nil {
		t.Fatal(err)
	}
	on, err = st.PlazaEnabled()
	if err != nil {
		t.Fatal(err)
	}
	if on {
		t.Fatal("shared settings cache still true after PutSettings false")
	}
}
