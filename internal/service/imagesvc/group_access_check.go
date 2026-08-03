package imagesvc

import (
	"time"

	"github.com/yixian-huang/imgli/internal/model"
)

// checkGroupExpires 改期时组策略：cap>0 禁止永久且不得超过 now+cap。
func checkGroupExpires(g *model.UserGroup, expiresAt *time.Time, now time.Time) error {
	if g == nil {
		return nil
	}
	capSec := g.MaxExpiresIn
	if g.ForceMaxAgeDays > 0 {
		force := g.ForceMaxAgeDays * 86400
		if capSec <= 0 || force < capSec {
			capSec = force
		}
	}
	if capSec <= 0 {
		return nil
	}
	if capSec > 366*24*60*60 {
		capSec = 366 * 24 * 60 * 60
	}
	if expiresAt == nil {
		return ErrExpiresOverGroup
	}
	maxAt := now.Add(time.Duration(capSec) * time.Second)
	if expiresAt.After(maxAt) {
		return ErrExpiresOverGroup
	}
	return nil
}

func checkGroupMaxViews(g *model.UserGroup, maxViews int) error {
	if g == nil || g.MaxMaxViews <= 0 {
		return nil
	}
	capV := g.MaxMaxViews
	if capV > MaxViewsMax {
		capV = MaxViewsMax
	}
	if maxViews <= 0 || maxViews > capV {
		return ErrMaxViewsOverGroup
	}
	return nil
}
