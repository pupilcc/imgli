package adminsvc

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/model"
)

func TestValidateAnnouncement(t *testing.T) {
	ok := DefaultAnnouncement()
	ok.Enabled = true
	ok.Text = "hello"
	if err := ValidateAnnouncement(ok); err != nil {
		t.Fatal(err)
	}
	bad := ok
	bad.Text = ""
	if err := ValidateAnnouncement(bad); !errors.Is(err, ErrAnnouncementInvalid) {
		t.Fatalf("empty text when enabled: %v", err)
	}
	bad = ok
	bad.LinkURL = "javascript:alert(1)"
	if err := ValidateAnnouncement(bad); !errors.Is(err, ErrAnnouncementInvalid) {
		t.Fatalf("js url: %v", err)
	}
	ok.LinkURL = "https://img.li"
	ok.LinkLabel = "demo"
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
	a := Announcement{Enabled: true, Text: "hi", StartsAt: "2026-07-01T00:00:00Z", EndsAt: "2026-08-01T00:00:00Z"}
	if !AnnouncementActive(a, now) {
		t.Fatal("should be active")
	}
	a.EndsAt = "2026-07-01T00:00:00Z"
	if AnnouncementActive(a, now) {
		t.Fatal("should be expired")
	}
}

func TestValidateFooterAndHTML(t *testing.T) {
	f := Footer{Groups: []FooterGroup{{Title: "A", Links: []FooterLink{{Label: "GitHub", URL: "https://github.com"}}}}}
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

func TestPutSettingsSlotsRoundTrip(t *testing.T) {
	svc := New(model.TestDB(t))
	ann := Announcement{Enabled: true, Text: "公告", LinkURL: "/explore", LinkLabel: "广场", Dismissible: true}
	raw, _ := json.Marshal(ann)
	if err := svc.PutSettings(map[string]json.RawMessage{"announcement": raw}); err != nil {
		t.Fatal(err)
	}
	foot := Footer{Groups: []FooterGroup{{Title: "链接", Links: []FooterLink{{Label: "GitHub", URL: "https://github.com/yixian-huang/imgli"}}}}}
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
	if !gotAnn.Enabled || gotAnn.Text != "公告" {
		t.Fatalf("announcement=%+v", gotAnn)
	}
	gotFoot, _ := m["footer"].(Footer)
	if len(gotFoot.Groups) != 1 || gotFoot.Groups[0].Links[0].Label != "GitHub" {
		t.Fatalf("footer=%+v", gotFoot)
	}
}
