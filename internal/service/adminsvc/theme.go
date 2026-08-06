package adminsvc

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/yixian-huang/imgli/internal/apperr"
)

var (
	// ErrThemeAccentInvalid theme_accent 须为空或 #RGB / #RRGGBB。
	ErrThemeAccentInvalid = apperr.New("theme_accent 须为空或 #RGB/#RRGGBB")
	// ErrThemeBgImageURLInvalid theme_bg_image_url 须为空或 http(s) / 站内路径。
	ErrThemeBgImageURLInvalid = apperr.New("theme_bg_image_url 须为空或 http(s) URL / 站内路径")
	// ErrThemeBgDimInvalid theme_bg_dim 须在 0–1。
	ErrThemeBgDimInvalid = apperr.New("theme_bg_dim 须在 0–1")
)

// DefaultThemeBgDim is the scrim strength when background image is set (readable text).
const DefaultThemeBgDim = 0.72

var themeAccentRe = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// NormalizeThemeAccent trims; empty stays empty; expands #RGB → #RRGGBB; lowercases hex.
func NormalizeThemeAccent(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if !themeAccentRe.MatchString(s) {
		return s // leave as-is; Validate will reject
	}
	s = strings.ToLower(s)
	if len(s) == 4 {
		// #rgb → #rrggbb
		return fmt.Sprintf("#%c%c%c%c%c%c", s[1], s[1], s[2], s[2], s[3], s[3])
	}
	return s
}

// ValidateThemeAccent empty OK; else #RGB or #RRGGBB.
func ValidateThemeAccent(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if !themeAccentRe.MatchString(s) {
		return ErrThemeAccentInvalid
	}
	return nil
}

// NormalizeThemeBgDim clamps missing/NaN-like to default; values outside 0–1 stay for Validate.
func NormalizeThemeBgDim(v float64) float64 {
	if v != v { // NaN
		return DefaultThemeBgDim
	}
	return v
}

// ValidateThemeBgDim requires 0 <= v <= 1.
func ValidateThemeBgDim(v float64) error {
	if v != v || v < 0 || v > 1 {
		return ErrThemeBgDimInvalid
	}
	return nil
}

// ContrastOnAccent returns near-white or near-black for button text on the given #RRGGBB accent.
func ContrastOnAccent(hex string) string {
	hex = NormalizeThemeAccent(hex)
	if len(hex) != 7 {
		return "#ffffff"
	}
	r, _ := strconv.ParseInt(hex[1:3], 16, 0)
	g, _ := strconv.ParseInt(hex[3:5], 16, 0)
	b, _ := strconv.ParseInt(hex[5:7], 16, 0)
	// relative luminance (sRGB approx)
	luma := (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 255
	if luma > 0.55 {
		return "#17171a"
	}
	return "#ffffff"
}
