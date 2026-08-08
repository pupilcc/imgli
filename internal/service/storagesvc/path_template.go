package storagesvc

import (
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Path template placeholders (storage object key relative path, after surface prefix).
//
// Fixed time (from `now`):
//
//	{Y} {m} {d} {H} {M} {S} {ms}
//
// Random (required at least one):
//
//	{uniqid}           12-char base62 mixed
//	{rand}             alias of {uniqid}
//	{rand:N}           N-char base62 mixed, N∈[8,32]
//	{hex:N}            N-char lowercase hex, N∈[12,32]
//	{HEX:N}            N-char uppercase hex, N∈[12,32]
//	{digits:N}         N-char digits, N∈[16,32]
//
// Other:
//
//	{ext}              file extension without dot
//
// Surface prefix (public/|private/) is applied by upload code, not this template.

var (
	// ErrPathTemplateInvalid is returned when a path_template fails validation.
	ErrPathTemplateInvalid = errors.New("path template invalid")

	rePathToken = regexp.MustCompile(`\{([^{}]+)\}`)
	reRandToken = regexp.MustCompile(`\{(uniqid|rand|hex|HEX|digits)(?::(\d+))?\}`)
)

const (
	pathTemplateMaxLen = 128
	defaultUniqLen     = 12
)

// ValidatePathTemplate checks operator-facing path_template rules.
// Empty is allowed (caller applies default). Non-empty must include a random token.
func ValidatePathTemplate(tmpl string) error {
	tmpl = strings.TrimSpace(tmpl)
	if tmpl == "" {
		return nil
	}
	if len(tmpl) > pathTemplateMaxLen {
		return fmt.Errorf("%w: 长度不得超过 %d", ErrPathTemplateInvalid, pathTemplateMaxLen)
	}
	if strings.Contains(tmpl, `\`) {
		return fmt.Errorf("%w: 不得包含反斜杠", ErrPathTemplateInvalid)
	}
	if strings.Contains(tmpl, "..") {
		return fmt.Errorf("%w: 不得包含 ..", ErrPathTemplateInvalid)
	}
	if strings.HasPrefix(tmpl, "/") {
		return fmt.Errorf("%w: 不得以 / 开头", ErrPathTemplateInvalid)
	}

	hasRandom := false
	for _, m := range rePathToken.FindAllStringSubmatch(tmpl, -1) {
		inner := m[1]
		ok, isRand, err := validatePathToken(inner)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: 未知占位符 {%s}", ErrPathTemplateInvalid, inner)
		}
		if isRand {
			hasRandom = true
		}
	}
	if !hasRandom {
		return fmt.Errorf("%w: 须包含随机占位符（如 {uniqid} 或 {rand:12}），避免并发撞键", ErrPathTemplateInvalid)
	}
	return nil
}

func validatePathToken(inner string) (known bool, isRandom bool, err error) {
	switch inner {
	case "Y", "m", "d", "H", "M", "S", "ms", "ext":
		return true, false, nil
	case "uniqid", "rand":
		return true, true, nil
	}
	if name, nStr, ok := strings.Cut(inner, ":"); ok {
		n, err := strconv.Atoi(nStr)
		if err != nil || nStr == "" || strings.TrimSpace(nStr) != nStr {
			return false, false, fmt.Errorf("%w: {%s} 长度无效", ErrPathTemplateInvalid, inner)
		}
		switch name {
		case "rand":
			if n < 8 || n > 32 {
				return false, false, fmt.Errorf("%w: {rand:N} 要求 N∈[8,32]", ErrPathTemplateInvalid)
			}
			return true, true, nil
		case "hex", "HEX":
			if n < 12 || n > 32 {
				return false, false, fmt.Errorf("%w: {%s:N} 要求 N∈[12,32]", ErrPathTemplateInvalid, name)
			}
			return true, true, nil
		case "digits":
			if n < 16 || n > 32 {
				return false, false, fmt.Errorf("%w: {digits:N} 要求 N∈[16,32]", ErrPathTemplateInvalid)
			}
			return true, true, nil
		case "uniqid":
			return false, false, fmt.Errorf("%w: {uniqid} 不支持长度参数，请用 {rand:N}", ErrPathTemplateInvalid)
		}
	}
	return false, false, nil
}

// RenderPath renders path_template with time and random tokens.
// Does not prepend surface prefix or policy prefix.
// Empty tmpl falls back to the product default date + uniqid layout.
func (r *Resolver) RenderPath(tmpl, ext string, now time.Time) (string, error) {
	tmpl = strings.TrimSpace(tmpl)
	if tmpl == "" {
		tmpl = "{Y}/{m}/{d}/{uniqid}.{ext}"
	}
	if err := ValidatePathTemplate(tmpl); err != nil {
		return "", err
	}
	ext = strings.TrimPrefix(strings.TrimSpace(ext), ".")
	out := tmpl
	rep := strings.NewReplacer(
		"{Y}", now.Format("2006"),
		"{m}", now.Format("01"),
		"{d}", now.Format("02"),
		"{H}", now.Format("15"),
		"{M}", now.Format("04"),
		"{S}", now.Format("05"),
		"{ms}", fmt.Sprintf("%03d", now.Nanosecond()/1e6),
		"{ext}", ext,
	)
	out = rep.Replace(out)

	// Expand random tokens left-to-right so repeated tokens get distinct values.
	var b strings.Builder
	last := 0
	matches := reRandToken.FindAllStringSubmatchIndex(out, -1)
	for _, loc := range matches {
		b.WriteString(out[last:loc[0]])
		name := out[loc[2]:loc[3]]
		n := defaultUniqLen
		if loc[4] >= 0 {
			parsed, err := strconv.Atoi(out[loc[4]:loc[5]])
			if err != nil {
				return "", fmt.Errorf("storagesvc: bad rand length: %w", err)
			}
			n = parsed
		}
		var alphabet string
		switch name {
		case "uniqid", "rand":
			alphabet = base62
			if loc[4] < 0 {
				n = defaultUniqLen
			}
		case "hex":
			alphabet = "0123456789abcdef"
		case "HEX":
			alphabet = "0123456789ABCDEF"
		case "digits":
			alphabet = "0123456789"
		default:
			return "", fmt.Errorf("storagesvc: unexpected rand token %q", name)
		}
		s, err := randFromAlphabet(n, alphabet)
		if err != nil {
			return "", err
		}
		b.WriteString(s)
		last = loc[1]
	}
	b.WriteString(out[last:])
	return b.String(), nil
}

func randFromAlphabet(n int, alphabet string) (string, error) {
	if n <= 0 || alphabet == "" {
		return "", fmt.Errorf("storagesvc: bad rand alphabet")
	}
	// Rejection sampling: only use full sets of len(alphabet) to avoid modulo bias.
	limit := 256 - (256 % len(alphabet))
	out := make([]byte, n)
	buf := make([]byte, n)
	for i := 0; i < n; {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, c := range buf {
			if int(c) >= limit {
				continue
			}
			out[i] = alphabet[int(c)%len(alphabet)]
			i++
			if i >= n {
				break
			}
		}
	}
	return string(out), nil
}
