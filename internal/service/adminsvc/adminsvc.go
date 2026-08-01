// Package adminsvc 管理员域服务：统计、用户/图片/审核/组/策略/设置管理与审计落库。
package adminsvc

import (
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/moderation"
	"github.com/yixian-huang/imgli/internal/service/settings"
)

type Service struct {
	db      *gorm.DB
	st      *settings.Service
	dataDir string // 与 storagesvc 一致：local 策略相对 root 拼到此目录下
}

// New 构造管理端服务。可选传入进程内共享的 settings.Service，
// 使 PutSettings 的 Invalidate 能立刻作用于广场/游客上传等热路径缓存。
func New(db *gorm.DB, shared ...*settings.Service) *Service {
	var st *settings.Service
	if len(shared) > 0 && shared[0] != nil {
		st = shared[0]
	} else {
		st = settings.New(db)
	}
	return &Service{db: db, st: st}
}

// UseDataDir 设置应用 data_dir，供 local 存储探针解析 root（与 storagesvc 相同语义）。
// 相对 root 拼到 dataDir 下；绝对 root 经 filepath.Join 后仍为自身。可链式调用。
func (s *Service) UseDataDir(dir string) *Service {
	if s != nil {
		s.dataDir = dir
	}
	return s
}

// settings 返回进程内 settings 服务（与 Discover/upload 共享时 Invalidate 才有效）。
func (s *Service) settings() *settings.Service {
	if s.st != nil {
		return s.st
	}
	return settings.New(s.db)
}

// DB 暴露底层连接（admin 设置读写等）。
func (s *Service) DB() *gorm.DB { return s.db }

// ModerationConfig 返回明文机审配置(服务端内部用,不经 GetSettings 打码路径)。
func (s *Service) ModerationConfig() (moderation.Config, error) {
	cfg := moderation.DefaultConfig()
	if err := s.settings().Get(model.SettingModeration, &cfg); err != nil && !errors.Is(err, settings.ErrNotFound) {
		return moderation.Config{}, err
	}
	return cfg, nil
}

// Audit 落审计日志；序列化/写库失败仅告警，不阻断业务。
func (s *Service) Audit(actorID *uint64, actorType, action string, detail any, ip string) {
	b, err := json.Marshal(detail)
	if err != nil {
		slog.Warn("audit detail 序列化失败", "action", action, "err", err)
		b = []byte("{}")
	}
	if err := s.db.Create(&model.AuditLog{
		ActorID: actorID, ActorType: actorType, Action: action, Detail: string(b), IP: ip,
	}).Error; err != nil {
		slog.Warn("audit 落库失败", "action", action, "err", err)
	}
}

type DailyCount struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

type DayCount struct {
	Date  string `json:"date"`
	Views int64  `json:"views"`
}

type HostCount struct {
	Host    string `json:"host"`
	Count   int64  `json:"count"`
	Suspect bool   `json:"suspect,omitempty"`
}

type ChannelCount struct {
	Channel string `json:"channel"`
	Count   int64  `json:"count"`
}

type BandwidthUser struct {
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	Used     int64  `json:"used"`
}

type Stats struct {
	Users              int64           `json:"users"`
	Images             int64           `json:"images"`
	Storage            int64           `json:"storage"`
	TodayUploads       int64           `json:"today_uploads"`
	PendingImages      int64           `json:"pending_images"` // 待审
	RejectedImages     int64           `json:"rejected_images"`
	TasksPending       int64           `json:"tasks_pending"`
	TasksRunning       int64           `json:"tasks_running"`
	Daily              []DailyCount    `json:"daily"`
	Traffic7d          []DayCount      `json:"traffic_7d"`
	Traffic30d         []DayCount      `json:"traffic_30d"`
	TopReferers        []HostCount     `json:"top_referers"` // 7d (back-compat)
	TopReferers30d     []HostCount     `json:"top_referers_30d"`
	Signups30d         []DailyCount    `json:"signups_30d"`
	SignupChannels30d  []ChannelCount  `json:"signup_channels_30d"`
	BandwidthUsedMonth int64           `json:"bandwidth_used_month"`
	BandwidthTopUsers  []BandwidthUser `json:"bandwidth_top_users"`
	OriginMeteringOnly bool            `json:"origin_metering_only"` // always true; UI caveat
	StatsRetentionDays int             `json:"stats_retention_days"`
}

// Stats 仪表盘统计（裁决 13 口径 + 运营轻量扩展）。
func (s *Service) Stats() (*Stats, error) {
	return s.StatsWithOwnHost("")
}

// StatsWithOwnHost same as Stats; ownHost marks non-allowlisted external referers suspect.
func (s *Service) StatsWithOwnHost(ownHost string) (*Stats, error) {
	var st Stats
	st.OriginMeteringOnly = true
	st.StatsRetentionDays = 90
	if err := s.db.Model(&model.User{}).Count(&st.Users).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.Image{}).Count(&st.Images).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.User{}).Select("COALESCE(SUM(used_storage),0)").Scan(&st.Storage).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.User{}).Select("COALESCE(SUM(bandwidth_used_month),0)").Scan(&st.BandwidthUsedMonth).Error; err != nil {
		return nil, err
	}
	today := time.Now().Truncate(24 * time.Hour)
	if err := s.db.Model(&model.Image{}).Where("created_at >= ?", today).Count(&st.TodayUploads).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.Image{}).Where("status = ?", "pending").Count(&st.PendingImages).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.Image{}).Where("status = ?", "rejected").Count(&st.RejectedImages).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.Task{}).Where("status = ?", "pending").Count(&st.TasksPending).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.Task{}).Where("status = ?", "running").Count(&st.TasksRunning).Error; err != nil {
		return nil, err
	}
	since := today.AddDate(0, 0, -29)
	rows, err := s.db.Model(&model.Image{}).Unscoped().
		Select("DATE(created_at) AS d, COUNT(*) AS c").
		Where("created_at >= ?", since).
		Group("DATE(created_at)").Order("d").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d string
		var c int64
		if err := rows.Scan(&d, &c); err != nil {
			return nil, err
		}
		if len(d) > 10 {
			d = d[:10] // PG DATE 扫成 time 字符串时裁剪
		}
		st.Daily = append(st.Daily, DailyCount{Date: d, Count: c})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Signups 30d (users.created_at), pad missing days
	smap, err := s.dailyUserCounts(since)
	if err != nil {
		return nil, err
	}
	st.Signups30d = padDailyCounts(smap, 30)

	// Signup channels 30d
	chRows, err := s.db.Model(&model.User{}).
		Select("CASE WHEN signup_channel = '' OR signup_channel IS NULL THEN 'unknown' ELSE signup_channel END AS ch, COUNT(*) AS c").
		Where("created_at >= ?", since).
		Group("ch").Order("c DESC").Rows()
	if err != nil {
		return nil, err
	}
	for chRows.Next() {
		var ch string
		var c int64
		if err := chRows.Scan(&ch, &c); err != nil {
			chRows.Close()
			return nil, err
		}
		st.SignupChannels30d = append(st.SignupChannels30d, ChannelCount{Channel: ch, Count: c})
	}
	if err := chRows.Err(); err != nil {
		chRows.Close()
		return nil, err
	}
	chRows.Close()
	if st.SignupChannels30d == nil {
		st.SignupChannels30d = []ChannelCount{}
	}

	// Traffic 7d + 30d
	start7 := time.Now().AddDate(0, 0, -6).Format("2006-01-02")
	start30 := time.Now().AddDate(0, 0, -29).Format("2006-01-02")
	tmap, err := s.trafficByDate(start30)
	if err != nil {
		return nil, err
	}
	st.Traffic7d = padTraffic(tmap, 7)
	st.Traffic30d = padTraffic(tmap, 30)

	// Hotlink allowlist for suspect flags
	allow := map[string]bool{}
	var hot statsHotlink
	_ = s.settings().Get(model.SettingHotlink, &hot)
	for _, d := range hot.AllowedDomains {
		allow[strings.ToLower(strings.TrimSpace(d))] = true
	}
	own := strings.ToLower(strings.TrimSpace(ownHost))

	st.TopReferers, err = s.topReferers(start7, 10, own, allow)
	if err != nil {
		return nil, err
	}
	st.TopReferers30d, err = s.topReferers(start30, 10, own, allow)
	if err != nil {
		return nil, err
	}

	// Bandwidth top users (current counters; period may differ per user but still useful)
	var bwUsers []model.User
	if err := s.db.Select("id, username, bandwidth_used_month").
		Where("bandwidth_used_month > 0").
		Order("bandwidth_used_month DESC").Limit(5).Find(&bwUsers).Error; err != nil {
		return nil, err
	}
	st.BandwidthTopUsers = make([]BandwidthUser, 0, len(bwUsers))
	for _, u := range bwUsers {
		st.BandwidthTopUsers = append(st.BandwidthTopUsers, BandwidthUser{
			UserID: u.ID, Username: u.Username, Used: u.BandwidthUsedMonth,
		})
	}

	return &st, nil
}

// statsHotlink mirrors stats.HotlinkConfig JSON without importing cycle risk for fields we need.
type statsHotlink struct {
	AllowedDomains []string `json:"allowed_domains"`
}

func (s *Service) dailyUserCounts(since time.Time) (map[string]int64, error) {
	rows, err := s.db.Model(&model.User{}).
		Select("DATE(created_at) AS d, COUNT(*) AS c").
		Where("created_at >= ?", since).
		Group("DATE(created_at)").Order("d").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var d string
		var c int64
		if err := rows.Scan(&d, &c); err != nil {
			return nil, err
		}
		if len(d) > 10 {
			d = d[:10]
		}
		out[d] = c
	}
	return out, rows.Err()
}

func padDailyCounts(byDate map[string]int64, days int) []DailyCount {
	out := make([]DailyCount, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		out = append(out, DailyCount{Date: d, Count: byDate[d]})
	}
	return out
}

func (s *Service) trafficByDate(start string) (map[string]int64, error) {
	trows, err := s.db.Model(&model.AccessStat{}).
		Select("date AS d, COALESCE(SUM(views),0) AS c").
		Where("date >= ?", start).
		Group("date").Order("d").Rows()
	if err != nil {
		return nil, err
	}
	defer trows.Close()
	byDate := map[string]int64{}
	for trows.Next() {
		var d string
		var c int64
		if err := trows.Scan(&d, &c); err != nil {
			return nil, err
		}
		if len(d) > 10 {
			d = d[:10]
		}
		byDate[d] = c
	}
	return byDate, trows.Err()
}

func padTraffic(byDate map[string]int64, days int) []DayCount {
	out := make([]DayCount, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		out = append(out, DayCount{Date: d, Views: byDate[d]})
	}
	return out
}

func (s *Service) topReferers(start string, limit int, ownHost string, allow map[string]bool) ([]HostCount, error) {
	rrows, err := s.db.Model(&model.RefererStat{}).
		Select("host, SUM(count) AS c").
		Where("date >= ?", start).
		Group("host").Order("c DESC").Limit(limit).Rows()
	if err != nil {
		return nil, err
	}
	defer rrows.Close()
	var out []HostCount
	for rrows.Next() {
		var host string
		var c int64
		if err := rrows.Scan(&host, &c); err != nil {
			return nil, err
		}
		h := HostCount{Host: host, Count: c}
		lh := strings.ToLower(host)
		if lh != "(direct)" && lh != ownHost && !allow[lh] && !strings.HasPrefix(lh, "*.") {
			// Suspect: external host not on allowlist (operators review for hotlink).
			h.Suspect = true
		}
		// If allow has exact match, not suspect; wildcards in allow are approximate only.
		for d := range allow {
			if strings.HasPrefix(d, "*.") {
				suf := d[2:]
				if lh == suf || (strings.HasSuffix(lh, "."+suf)) {
					h.Suspect = false
				}
			}
		}
		out = append(out, h)
	}
	if out == nil {
		out = []HostCount{}
	}
	return out, rrows.Err()
}

// RefererImageRow is one image hit under a referer host.
type RefererImageRow struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// TopImagesByRefererHost returns top images for host in the last `days` (default 30).
func (s *Service) TopImagesByRefererHost(host string, days, limit int) ([]RefererImageRow, error) {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" {
		return nil, errors.New("host required")
	}
	if days <= 0 {
		days = 30
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	start := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	rows, err := s.db.Table("referer_image_stats AS r").
		Select("i.key AS key, i.name AS name, SUM(r.count) AS c").
		Joins("JOIN images i ON i.id = r.image_id AND i.deleted_at IS NULL").
		Where("r.host = ? AND r.date >= ?", host, start).
		Group("i.id, i.key, i.name").
		Order("c DESC").
		Limit(limit).
		Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RefererImageRow
	for rows.Next() {
		var key, name string
		var c int64
		if err := rows.Scan(&key, &name, &c); err != nil {
			return nil, err
		}
		out = append(out, RefererImageRow{Key: key, Name: name, Count: c})
	}
	if out == nil {
		out = []RefererImageRow{}
	}
	return out, rows.Err()
}
