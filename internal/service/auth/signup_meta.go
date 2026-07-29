package auth

import (
	"net/url"
	"strings"
	"unicode/utf8"
)

// Signup channel values persisted on User.SignupChannel.
const (
	ChannelDirect  = "direct"
	ChannelInvite  = "invite"
	ChannelUTM     = "utm"
	ChannelReferer = "referer"
	ChannelUnknown = "unknown"
)

const signupFieldMax = 64
const signupHostMax = 255

// SignupMeta is first-party attribution captured only at registration time.
type SignupMeta struct {
	UTMSource   string
	UTMMedium   string
	UTMCampaign string
	RefererHost string // host only; full URLs are rejected/normalized
}

// Sanitize truncates and normalizes meta fields (no full URLs stored).
func (m SignupMeta) Sanitize() SignupMeta {
	return SignupMeta{
		UTMSource:   truncateField(strings.TrimSpace(m.UTMSource), signupFieldMax),
		UTMMedium:   truncateField(strings.TrimSpace(m.UTMMedium), signupFieldMax),
		UTMCampaign: truncateField(strings.TrimSpace(m.UTMCampaign), signupFieldMax),
		RefererHost: normalizeSignupHost(m.RefererHost),
	}
}

// DeriveChannel picks a coarse channel (invite wins over UTM over referer).
func DeriveChannel(inviteUsed bool, m SignupMeta) string {
	m = m.Sanitize()
	if inviteUsed {
		return ChannelInvite
	}
	if m.UTMSource != "" {
		return ChannelUTM
	}
	if m.RefererHost != "" {
		return ChannelReferer
	}
	return ChannelDirect
}

func truncateField(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	// rune-safe truncate
	n := 0
	for i := range s {
		if n == max {
			return s[:i]
		}
		n++
	}
	return s
}

// normalizeSignupHost accepts a bare host or a URL; stores host only (lowercase, no port).
func normalizeSignupHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// Reject path-looking blobs without scheme if they contain /
	if strings.Contains(raw, "://") || strings.HasPrefix(raw, "//") {
		u, err := url.Parse(raw)
		if err != nil {
			return ""
		}
		raw = u.Hostname()
	} else if i := strings.IndexByte(raw, '/'); i >= 0 {
		raw = raw[:i]
	}
	// strip port (handle IPv6 [addr]:port lightly)
	if strings.HasPrefix(raw, "[") {
		if end := strings.IndexByte(raw, ']'); end > 0 {
			raw = raw[1:end]
		}
	} else if host, _, ok := strings.Cut(raw, ":"); ok && !strings.Contains(raw, "::") {
		// only strip :port when not IPv6-like without brackets
		if host != "" {
			raw = host
		}
	}
	raw = strings.ToLower(strings.TrimSpace(raw))
	return truncateField(raw, signupHostMax)
}
