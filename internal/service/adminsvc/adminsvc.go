// Package adminsvc 管理员域服务：统计、用户/图片/审核/组/策略/设置管理与审计落库。
package adminsvc

import (
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/moderation"
	"github.com/yixian-huang/imgli/internal/service/settings"
)

type Service struct{ db *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db: db} }

// ModerationConfig 返回明文机审配置(服务端内部用,不经 GetSettings 打码路径)。
func (s *Service) ModerationConfig() (moderation.Config, error) {
	cfg := moderation.DefaultConfig()
	if err := settings.New(s.db).Get(model.SettingModeration, &cfg); err != nil && !errors.Is(err, settings.ErrNotFound) {
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
	Host  string `json:"host"`
	Count int64  `json:"count"`
}

type Stats struct {
	Users          int64        `json:"users"`
	Images         int64        `json:"images"`
	Storage        int64        `json:"storage"`
	TodayUploads   int64        `json:"today_uploads"`
	PendingImages  int64        `json:"pending_images"` // 待审
	RejectedImages int64        `json:"rejected_images"`
	TasksPending   int64        `json:"tasks_pending"`
	TasksRunning   int64        `json:"tasks_running"`
	Daily          []DailyCount `json:"daily"`
	Traffic7d      []DayCount   `json:"traffic_7d"`
	TopReferers    []HostCount  `json:"top_referers"`
}

// Stats 仪表盘统计（裁决 13 口径 + 可观测最小集）。
func (s *Service) Stats() (*Stats, error) {
	var st Stats
	if err := s.db.Model(&model.User{}).Count(&st.Users).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.Image{}).Count(&st.Images).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.User{}).Select("COALESCE(SUM(used_storage),0)").Scan(&st.Storage).Error; err != nil {
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

	// D-① 近 7 日流量（access_stats 按日 SUM，缺日补 0，恒 7 项升序）
	start7 := time.Now().AddDate(0, 0, -6).Format("2006-01-02")
	trows, err := s.db.Model(&model.AccessStat{}).
		Select("date AS d, COALESCE(SUM(views),0) AS c").
		Where("date >= ?", start7).
		Group("date").Order("d").Rows()
	if err != nil {
		return nil, err
	}
	byDate := map[string]int64{}
	for trows.Next() {
		var d string
		var c int64
		if err := trows.Scan(&d, &c); err != nil {
			trows.Close()
			return nil, err
		}
		if len(d) > 10 {
			d = d[:10]
		}
		byDate[d] = c
	}
	if err := trows.Err(); err != nil {
		trows.Close()
		return nil, err
	}
	trows.Close()
	st.Traffic7d = make([]DayCount, 0, 7)
	for i := 0; i < 7; i++ {
		d := time.Now().AddDate(0, 0, -6+i).Format("2006-01-02")
		st.Traffic7d = append(st.Traffic7d, DayCount{Date: d, Views: byDate[d]})
	}

	// D-① 近 7 日来源 Top10
	rrows, err := s.db.Model(&model.RefererStat{}).
		Select("host, SUM(count) AS c").
		Where("date >= ?", start7).
		Group("host").Order("c DESC").Limit(10).Rows()
	if err != nil {
		return nil, err
	}
	defer rrows.Close()
	for rrows.Next() {
		var host string
		var c int64
		if err := rrows.Scan(&host, &c); err != nil {
			return nil, err
		}
		st.TopReferers = append(st.TopReferers, HostCount{Host: host, Count: c})
	}
	if st.TopReferers == nil {
		st.TopReferers = []HostCount{}
	}
	return &st, rrows.Err()
}
