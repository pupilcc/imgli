package adminsvc

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// LocaleString is operator copy in zh/en.
// JSON accepts a legacy plain string (→ zh) or {"zh":"...","en":"..."}.
type LocaleString struct {
	ZH string `json:"zh,omitempty"`
	EN string `json:"en,omitempty"`
}

// UnmarshalJSON accepts string or object.
func (l *LocaleString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*l = LocaleString{}
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*l = LocaleString{ZH: strings.TrimSpace(s)}
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	*l = LocaleString{
		ZH: strings.TrimSpace(m["zh"]),
		EN: strings.TrimSpace(m["en"]),
	}
	return nil
}

// MarshalJSON always emits an object (stable API for admin SPA).
func (l LocaleString) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ZH string `json:"zh"`
		EN string `json:"en"`
	}{ZH: l.ZH, EN: l.EN})
}

// Normalize trims both sides.
func (l LocaleString) Normalize() LocaleString {
	return LocaleString{
		ZH: strings.TrimSpace(l.ZH),
		EN: strings.TrimSpace(l.EN),
	}
}

// Any returns the first non-empty locale (zh preferred).
func (l LocaleString) Any() string {
	l = l.Normalize()
	if l.ZH != "" {
		return l.ZH
	}
	return l.EN
}

// MaxRunes is the longer of zh/en rune counts.
func (l LocaleString) MaxRunes() int {
	l = l.Normalize()
	a, b := utf8.RuneCountInString(l.ZH), utf8.RuneCountInString(l.EN)
	if a > b {
		return a
	}
	return b
}
