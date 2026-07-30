// Package bandwidth 实现第一期月流量硬顶：按门禁放行计量、属主记账、自然月重置。
// 时区固定 Asia/Shanghai；配额 0=不限制。详见 omni 产品裁决页。
package bandwidth

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
)

// Loc 账期所用时区（产品裁决：Asia/Shanghai）。
var Loc = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}()

// ErrExceeded 本月流量已达用户组硬顶。
var ErrExceeded = errors.New("bandwidth: monthly quota exceeded")

// Period 返回当前自然月账期键 "2006-01"（Asia/Shanghai）。
func Period(now time.Time) string {
	return now.In(Loc).Format("2006-01")
}

// CurrentPeriod 当前时刻账期。
func CurrentPeriod() string { return Period(time.Now()) }

// EffectiveUsed 若用户账期与当前不一致则视为 0（跨月未写库时）。
func EffectiveUsed(u *model.User, period string) int64 {
	if u == nil || u.BandwidthPeriod != period {
		return 0
	}
	if u.BandwidthUsedMonth < 0 {
		return 0
	}
	return u.BandwidthUsedMonth
}

// QuotaForUser 读取用户所属组的月流量配额；用户/组不存在返回 0,err。
func QuotaForUser(db *gorm.DB, userID uint64) (quota int64, err error) {
	var row struct {
		Quota int64 `gorm:"column:bandwidth_quota_month"`
	}
	err = db.Table("users").
		Select("user_groups.bandwidth_quota_month").
		Joins("JOIN user_groups ON user_groups.id = users.group_id").
		Where("users.id = ?", userID).
		Take(&row).Error
	return row.Quota, err
}

// checkRow 是 Check 单次 JOIN 的投影。
type checkRow struct {
	BandwidthUsedMonth  int64  `gorm:"column:bandwidth_used_month"`
	BandwidthPeriod     string `gorm:"column:bandwidth_period"`
	BandwidthQuotaMonth int64  `gorm:"column:bandwidth_quota_month"`
}

// Check 若用户本月已用尽硬顶返回 ErrExceeded；quota<=0 或 userID==0 不限制。
// 单次 JOIN users↔user_groups（原 3 次查询），减轻 /i /t 门禁在 SQLite 单连接下的往返。
func Check(db *gorm.DB, userID uint64) error {
	if userID == 0 {
		return nil
	}
	var row checkRow
	err := db.Table("users").
		Select("users.bandwidth_used_month, users.bandwidth_period, user_groups.bandwidth_quota_month").
		Joins("JOIN user_groups ON user_groups.id = users.group_id").
		Where("users.id = ?", userID).
		Take(&row).Error
	if err != nil {
		return err
	}
	if row.BandwidthQuotaMonth <= 0 {
		return nil
	}
	u := model.User{
		BandwidthUsedMonth: row.BandwidthUsedMonth,
		BandwidthPeriod:    row.BandwidthPeriod,
	}
	if EffectiveUsed(&u, CurrentPeriod()) >= row.BandwidthQuotaMonth {
		return ErrExceeded
	}
	return nil
}

// Add 将 n 字节计入属主本月用量；账期切换时重置为 n。n<=0 或 userID==0 为 no-op。
func Add(db *gorm.DB, userID uint64, n int64) error {
	if userID == 0 || n <= 0 {
		return nil
	}
	period := CurrentPeriod()
	// CASE：同账期累加；跨月从 n 起算并改 period。
	res := db.Exec(`
UPDATE users SET
  bandwidth_used_month = CASE WHEN bandwidth_period = ? THEN bandwidth_used_month + ? ELSE ? END,
  bandwidth_period = ?
WHERE id = ?`, period, n, n, period, userID)
	return res.Error
}

// Snapshot 用户当前账期用量与组配额（供 profile/API）。
type Snapshot struct {
	Period string `json:"period"`
	Used   int64  `json:"used"`
	Quota  int64  `json:"quota"` // 0=不限
}

// SnapshotFor 组装展示数据（单次 JOIN）。
func SnapshotFor(db *gorm.DB, userID uint64) (Snapshot, error) {
	period := CurrentPeriod()
	out := Snapshot{Period: period}
	if userID == 0 {
		return out, nil
	}
	var row checkRow
	err := db.Table("users").
		Select("users.bandwidth_used_month, users.bandwidth_period, user_groups.bandwidth_quota_month").
		Joins("JOIN user_groups ON user_groups.id = users.group_id").
		Where("users.id = ?", userID).
		Take(&row).Error
	if err != nil {
		return out, err
	}
	out.Used = EffectiveUsed(&model.User{
		BandwidthUsedMonth: row.BandwidthUsedMonth,
		BandwidthPeriod:    row.BandwidthPeriod,
	}, period)
	out.Quota = row.BandwidthQuotaMonth
	return out, nil
}
