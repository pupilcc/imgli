package adminsvc

import (
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// 引流/站点插槽校验错误（PutSettings 映射 400）。
var (
	ErrAnnouncementInvalid = errors.New("announcement 配置无效")
	ErrFooterInvalid       = errors.New("footer 配置无效")
	ErrHTMLInjectInvalid   = errors.New("html_inject 配置无效")
)

const (
	maxAnnouncementText  = 500
	maxLinkLabel         = 80
	maxFooterGroups      = 8
	maxFooterLinks       = 12
	maxHTMLInjectBytes   = 16 << 10 // 16 KiB per field
	maxFooterGroupTitle  = 80
)

// Announcement 顶栏公告插槽。
type Announcement struct {
	Enabled     bool   `json:"enabled"`
	Text        string `json:"text"`
	LinkURL     string `json:"link_url"`
	LinkLabel   string `json:"link_label"`
	Dismissible bool   `json:"dismissible"`
	// StartsAt / EndsAt：RFC3339 或空；空表示无界。
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
}

// FooterLink 页脚单链。
type FooterLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// FooterGroup 页脚链接组。
type FooterGroup struct {
	Title string       `json:"title"`
	Links []FooterLink `json:"links"`
}

// Footer 页脚配置。
type Footer struct {
	Groups []FooterGroup `json:"groups"`
}

// HTMLInject 自定义 HTML（仅 admin 可写；属自托管自伤面）。
type HTMLInject struct {
	Head    string `json:"head"`
	BodyEnd string `json:"body_end"`
}

// DefaultAnnouncement / DefaultFooter / DefaultHTMLInject 与 model 播种字面量对齐。
func DefaultAnnouncement() Announcement {
	return Announcement{Enabled: false, Dismissible: true}
}
func DefaultFooter() Footer { return Footer{Groups: []FooterGroup{}} }
func DefaultHTMLInject() HTMLInject {
	return HTMLInject{}
}

// NormalizeAnnouncement trims fields; does not validate.
func NormalizeAnnouncement(a Announcement) Announcement {
	a.Text = strings.TrimSpace(a.Text)
	a.LinkURL = strings.TrimSpace(a.LinkURL)
	a.LinkLabel = strings.TrimSpace(a.LinkLabel)
	a.StartsAt = strings.TrimSpace(a.StartsAt)
	a.EndsAt = strings.TrimSpace(a.EndsAt)
	return a
}

// ValidateAnnouncement 校验公告插槽。
func ValidateAnnouncement(a Announcement) error {
	a = NormalizeAnnouncement(a)
	if utf8.RuneCountInString(a.Text) > maxAnnouncementText {
		return ErrAnnouncementInvalid
	}
	if utf8.RuneCountInString(a.LinkLabel) > maxLinkLabel {
		return ErrAnnouncementInvalid
	}
	if err := validateSlotURL(a.LinkURL, true); err != nil {
		return ErrAnnouncementInvalid
	}
	if a.LinkURL != "" && a.LinkLabel == "" {
		return ErrAnnouncementInvalid
	}
	if a.StartsAt != "" {
		if _, err := time.Parse(time.RFC3339, a.StartsAt); err != nil {
			return ErrAnnouncementInvalid
		}
	}
	if a.EndsAt != "" {
		if _, err := time.Parse(time.RFC3339, a.EndsAt); err != nil {
			return ErrAnnouncementInvalid
		}
	}
	if a.StartsAt != "" && a.EndsAt != "" {
		s, _ := time.Parse(time.RFC3339, a.StartsAt)
		e, _ := time.Parse(time.RFC3339, a.EndsAt)
		if e.Before(s) {
			return ErrAnnouncementInvalid
		}
	}
	if a.Enabled && a.Text == "" {
		return ErrAnnouncementInvalid
	}
	return nil
}

// AnnouncementActive 公开面是否应展示该公告（enabled + 时间窗）。
func AnnouncementActive(a Announcement, now time.Time) bool {
	a = NormalizeAnnouncement(a)
	if !a.Enabled || a.Text == "" {
		return false
	}
	if a.StartsAt != "" {
		s, err := time.Parse(time.RFC3339, a.StartsAt)
		if err != nil || now.Before(s) {
			return false
		}
	}
	if a.EndsAt != "" {
		e, err := time.Parse(time.RFC3339, a.EndsAt)
		if err != nil || now.After(e) {
			return false
		}
	}
	return true
}

// ValidateFooter 校验页脚链接组。
func ValidateFooter(f Footer) error {
	if f.Groups == nil {
		f.Groups = []FooterGroup{}
	}
	if len(f.Groups) > maxFooterGroups {
		return ErrFooterInvalid
	}
	for i := range f.Groups {
		g := &f.Groups[i]
		g.Title = strings.TrimSpace(g.Title)
		if utf8.RuneCountInString(g.Title) > maxFooterGroupTitle {
			return ErrFooterInvalid
		}
		if len(g.Links) > maxFooterLinks {
			return ErrFooterInvalid
		}
		for j := range g.Links {
			l := &g.Links[j]
			l.Label = strings.TrimSpace(l.Label)
			l.URL = strings.TrimSpace(l.URL)
			if l.Label == "" || l.URL == "" {
				return ErrFooterInvalid
			}
			if utf8.RuneCountInString(l.Label) > maxLinkLabel {
				return ErrFooterInvalid
			}
			if err := validateSlotURL(l.URL, false); err != nil {
				return ErrFooterInvalid
			}
		}
	}
	return nil
}

// ValidateHTMLInject 校验自定义 HTML 注入。
func ValidateHTMLInject(h HTMLInject) error {
	if len(h.Head) > maxHTMLInjectBytes || len(h.BodyEnd) > maxHTMLInjectBytes {
		return ErrHTMLInjectInvalid
	}
	if strings.ContainsRune(h.Head, 0) || strings.ContainsRune(h.BodyEnd, 0) {
		return ErrHTMLInjectInvalid
	}
	return nil
}

// validateSlotURL emptyOK 时允许空串；非空须 http(s) 或站内相对路径 /...
func validateSlotURL(raw string, emptyOK bool) error {
	if raw == "" {
		if emptyOK {
			return nil
		}
		return errors.New("empty url")
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		if strings.ContainsAny(raw, " \t\r\n") {
			return errors.New("bad path")
		}
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("bad url")
	}
	if u.User != nil {
		return errors.New("userinfo")
	}
	return nil
}
