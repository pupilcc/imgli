package adminsvc

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/yixian-huang/imgli/internal/apperr"
)

// 引流/站点插槽校验错误（PutSettings 映射 400）。
var (
	ErrAnnouncementInvalid   = apperr.New("announcement 配置无效")
	ErrFooterInvalid         = apperr.New("footer 配置无效")
	ErrHTMLInjectInvalid     = apperr.New("html_inject 配置无效")
	ErrHelpURLInvalid        = apperr.New("help_url 无效")
	ErrUpgradeURLInvalid     = apperr.New("upgrade_url 无效")
	ErrRegisterNoticeInvalid = apperr.New("register_notice 无效")
	ErrShareBrandingInvalid  = apperr.New("share_branding 仅支持 off|site|links")
)

// Share branding modes for public /s and /a footers.
const (
	ShareBrandingOff   = "off"
	ShareBrandingSite  = "site"
	ShareBrandingLinks = "links"
)

const maxRegisterNotice = 500

const (
	maxAnnouncementText  = 500
	maxLinkLabel         = 80
	maxFooterGroups      = 8
	maxFooterLinks       = 12
	maxHTMLInjectBytes   = 16 << 10 // 16 KiB per field
	maxFooterGroupTitle  = 80
)

// Announcement 顶栏公告插槽（text / link_label 为 zh|en locale map）。
type Announcement struct {
	Enabled     bool         `json:"enabled"`
	Text        LocaleString `json:"text"`
	LinkURL     string       `json:"link_url"`
	LinkLabel   LocaleString `json:"link_label"`
	Dismissible bool         `json:"dismissible"`
	// StartsAt / EndsAt：RFC3339 或空；空表示无界。
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
}

// FooterLink 页脚单链。
type FooterLink struct {
	Label LocaleString `json:"label"`
	URL   string       `json:"url"`
}

// FooterGroup 页脚链接组。
type FooterGroup struct {
	Title LocaleString `json:"title"`
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
	a.Text = a.Text.Normalize()
	a.LinkURL = strings.TrimSpace(a.LinkURL)
	a.LinkLabel = a.LinkLabel.Normalize()
	a.StartsAt = strings.TrimSpace(a.StartsAt)
	a.EndsAt = strings.TrimSpace(a.EndsAt)
	return a
}

// ValidateAnnouncement 校验公告插槽。
func ValidateAnnouncement(a Announcement) error {
	a = NormalizeAnnouncement(a)
	if a.Text.MaxRunes() > maxAnnouncementText {
		return ErrAnnouncementInvalid
	}
	if a.LinkLabel.MaxRunes() > maxLinkLabel {
		return ErrAnnouncementInvalid
	}
	if err := validateSlotURL(a.LinkURL, true); err != nil {
		return ErrAnnouncementInvalid
	}
	// URL 有值时至少一种语言要有 link_label
	if a.LinkURL != "" && a.LinkLabel.Any() == "" {
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
	if a.Enabled && a.Text.Any() == "" {
		return ErrAnnouncementInvalid
	}
	return nil
}

// AnnouncementActive 公开面是否应展示该公告（enabled + 任一语言有文案 + 时间窗）。
func AnnouncementActive(a Announcement, now time.Time) bool {
	a = NormalizeAnnouncement(a)
	if !a.Enabled || a.Text.Any() == "" {
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
		g.Title = g.Title.Normalize()
		if g.Title.MaxRunes() > maxFooterGroupTitle {
			return ErrFooterInvalid
		}
		if len(g.Links) > maxFooterLinks {
			return ErrFooterInvalid
		}
		for j := range g.Links {
			l := &g.Links[j]
			l.Label = l.Label.Normalize()
			l.URL = strings.TrimSpace(l.URL)
			if l.Label.Any() == "" || l.URL == "" {
				return ErrFooterInvalid
			}
			if l.Label.MaxRunes() > maxLinkLabel {
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

// NormalizeOptionalURL trims; empty stays empty.
func NormalizeOptionalURL(s string) string { return strings.TrimSpace(s) }

// ValidateOptionalURL empty OK; else http(s) or site-relative path.
func ValidateOptionalURL(raw string) error {
	return validateSlotURL(strings.TrimSpace(raw), true)
}

// NormalizeShareBranding returns off|site|links; empty/unknown → site.
func NormalizeShareBranding(s string) string {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case ShareBrandingOff:
		return ShareBrandingOff
	case ShareBrandingLinks:
		return ShareBrandingLinks
	case ShareBrandingSite:
		return ShareBrandingSite
	default:
		return ShareBrandingSite
	}
}

// ValidateShareBranding rejects values outside off|site|links (after trim/lower).
func ValidateShareBranding(s string) error {
	v := strings.TrimSpace(strings.ToLower(s))
	switch v {
	case "", ShareBrandingOff, ShareBrandingSite, ShareBrandingLinks:
		return nil
	default:
		return ErrShareBrandingInvalid
	}
}

// ValidateRegisterNotice max length after trim (either locale).
func ValidateRegisterNotice(n LocaleString) error {
	n = n.Normalize()
	if n.MaxRunes() > maxRegisterNotice {
		return ErrRegisterNoticeInvalid
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
