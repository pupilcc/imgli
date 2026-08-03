package upload

import (
	"errors"
	"time"

	"github.com/yixian-huang/imgli/internal/model"
)

// 与 handler.MaxExpiresInSec / imagesvc.MaxViewsMax 对齐的全局上限（避免 upload 反向依赖 handler）。
const (
	globalMaxExpiresInSec = 366 * 24 * 60 * 60
	globalMaxViews        = 10000
)

var (
	// ErrExpiresOverGroup 有效期超过用户组上限或组禁止永久。
	ErrExpiresOverGroup = errors.New("upload: 有效期超出用户组限制")
	// ErrMaxViewsOverGroup 访问次数超过用户组上限或组禁止不限次。
	ErrMaxViewsOverGroup = errors.New("upload: 访问次数超出用户组限制")
)

// ApplyGroupAccess 按用户组策略钳制/补全 opts 的 ExpiresAt 与 MaxViews。
//
// 规则摘要：
//   - MaxExpiresIn>0 或 ForceMaxAgeDays>0：禁止永久；nil ExpiresAt 时用 DefaultExpiresIn，
//     再回退到 cap 秒数。有 cap 时若 ExpiresAt 超过 now+cap 则拒绝（不静默截断）。
//   - MaxMaxViews>0：禁止 0（不限）；0 时套 DefaultMaxViews，再回退到 MaxMaxViews；超 cap 拒绝。
//   - 仅 Default* 而无 Max*/Force*：不强制改写（默认只给 UI；API 仍可传永久/不限）。
func ApplyGroupAccess(g *model.UserGroup, opts *Opts, now time.Time) error {
	if g == nil || opts == nil {
		return nil
	}
	capSec := effectiveExpiresCapSec(g)
	if capSec > 0 {
		if opts.ExpiresAt == nil {
			sec := g.DefaultExpiresIn
			if sec <= 0 || sec > capSec {
				sec = capSec
			}
			t := now.Add(time.Duration(sec) * time.Second)
			opts.ExpiresAt = &t
		} else {
			maxAt := now.Add(time.Duration(capSec) * time.Second)
			if opts.ExpiresAt.After(maxAt) {
				return ErrExpiresOverGroup
			}
		}
	}

	if g.MaxMaxViews > 0 {
		capV := g.MaxMaxViews
		if capV > globalMaxViews {
			capV = globalMaxViews
		}
		if opts.MaxViews <= 0 {
			v := g.DefaultMaxViews
			if v <= 0 || v > capV {
				v = capV
			}
			opts.MaxViews = v
		} else if opts.MaxViews > capV {
			return ErrMaxViewsOverGroup
		}
	} else if opts.MaxViews > globalMaxViews {
		return ErrMaxViewsOverGroup
	}
	return nil
}

// effectiveExpiresCapSec 返回组有效期 cap（秒）；0=无组级 cap（允许永久）。
func effectiveExpiresCapSec(g *model.UserGroup) int {
	if g == nil {
		return 0
	}
	capSec := g.MaxExpiresIn
	if g.ForceMaxAgeDays > 0 {
		force := g.ForceMaxAgeDays * 86400
		if capSec <= 0 || force < capSec {
			capSec = force
		}
	}
	if capSec > globalMaxExpiresInSec {
		capSec = globalMaxExpiresInSec
	}
	if capSec < 0 {
		return 0
	}
	return capSec
}
