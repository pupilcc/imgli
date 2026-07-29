// Package stats 提供访问统计缓冲刷盘、防盗链配置快照与属主统计查询（D-①）。
package stats

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/settings"
)

// HotlinkConfig 防盗链配置(settings "hotlink" 键 JSON 契约,前后端逐字一致)。
type HotlinkConfig struct {
	Enabled           bool     `json:"enabled"`
	AllowedDomains    []string `json:"allowed_domains"` // 精确域或 *.example.com 前缀一级通配
	AllowEmptyReferer bool     `json:"allow_empty_referer"`
}

// DefaultHotlink 返回防盗链出厂默认：关闭、空白名单、允许空 Referer。
func DefaultHotlink() HotlinkConfig {
	return HotlinkConfig{
		Enabled:           false,
		AllowedDomains:    []string{},
		AllowEmptyReferer: true,
	}
}

// ErrNotFound 图片不存在或非属主。
var ErrNotFound = errors.New("stats: 记录不存在")

// HotlinkAllowed 纯函数:未启用恒 true;refHost 空看 AllowEmptyReferer;
// 等于 ownHost 恒 true;精确或 *. 通配命中 true(*.wild.example 同时命中
// wild.example 与 a.wild.example);其余 false。全小写比较。
func HotlinkAllowed(cfg HotlinkConfig, refHost, ownHost string) bool {
	if !cfg.Enabled {
		return true
	}
	ref := strings.ToLower(refHost)
	own := strings.ToLower(ownHost)
	if ref == "" {
		return cfg.AllowEmptyReferer
	}
	if ref == own {
		return true
	}
	for _, d := range cfg.AllowedDomains {
		d = strings.ToLower(d)
		if strings.HasPrefix(d, "*.") {
			suffix := d[2:]
			if ref == suffix {
				return true
			}
			// 一级通配：恰好一个标签前缀
			if i := strings.IndexByte(ref, '.'); i > 0 && ref[i+1:] == suffix {
				return true
			}
		} else if ref == d {
			return true
		}
	}
	return false
}

// DayViews 属主统计的单日视图。
type DayViews struct {
	Date  string `json:"date"`
	Views int64  `json:"views"`
}

type accessKey struct {
	imageID uint64
	date    string
}

type refKey struct {
	host string
	date string
}

type refImgKey struct {
	imageID uint64
	host    string
	date    string
}

// StatsRetentionDays is the default rolling retention for access/referer tables.
const StatsRetentionDays = 90

// Service 内存缓冲计数 + 定时 upsert 刷盘 + hotlink 快照。
type Service struct {
	db            *gorm.DB
	flushInterval time.Duration
	settings      *settings.Service

	mu         sync.Mutex
	access     map[accessKey]int64
	referer    map[refKey]int64
	refererImg map[refImgKey]int64

	hotMu    sync.Mutex
	hotCfg   HotlinkConfig
	hotAt    time.Time
	hotValid bool
	hotGen   uint64 // Invalidate 递增;刷新只在读库前后代次一致时发布,防旧值复活(codex 终审)

	lastPurge time.Time
}

// New 构造 Service；flushInterval 供 Start 使用。
func New(db *gorm.DB, flushInterval time.Duration) *Service {
	return &Service{
		db:            db,
		flushInterval: flushInterval,
		settings:      settings.New(db),
		access:        make(map[accessKey]int64),
		referer:       make(map[refKey]int64),
		refererImg:    make(map[refImgKey]int64),
	}
}

// Record 并发安全计数;refHost 空串在此归一为 "(direct)"(归一单点)。
func (s *Service) Record(imageID uint64, refHost string) {
	if refHost == "" {
		refHost = "(direct)"
	}
	date := time.Now().Format("2006-01-02")
	s.mu.Lock()
	s.access[accessKey{imageID: imageID, date: date}]++
	s.referer[refKey{host: refHost, date: date}]++
	s.refererImg[refImgKey{imageID: imageID, host: refHost, date: date}]++
	s.mu.Unlock()
}

// Flush 把缓冲搬出锁外,事务内对统计表 clause.OnConflict upsert 累加。
func (s *Service) Flush() error {
	s.mu.Lock()
	access := s.access
	referer := s.referer
	refererImg := s.refererImg
	s.access = make(map[accessKey]int64)
	s.referer = make(map[refKey]int64)
	s.refererImg = make(map[refImgKey]int64)
	s.mu.Unlock()

	accessRows := make([]model.AccessStat, 0, len(access))
	for k, v := range access {
		accessRows = append(accessRows, model.AccessStat{
			ImageID: k.imageID,
			Date:    k.date,
			Views:   v,
		})
	}
	refRows := make([]model.RefererStat, 0, len(referer))
	for k, v := range referer {
		refRows = append(refRows, model.RefererStat{
			Host:  k.host,
			Date:  k.date,
			Count: v,
		})
	}
	refImgRows := make([]model.RefererImageStat, 0, len(refererImg))
	for k, v := range refererImg {
		refImgRows = append(refImgRows, model.RefererImageStat{
			ImageID: k.imageID,
			Host:    k.host,
			Date:    k.date,
			Count:   v,
		})
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if len(accessRows) > 0 {
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "image_id"}, {Name: "date"}},
				DoUpdates: clause.Assignments(map[string]any{
					"views": gorm.Expr("access_stats.views + excluded.views"),
				}),
			}).Create(&accessRows).Error; err != nil {
				return err
			}
		}
		if len(refRows) > 0 {
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "host"}, {Name: "date"}},
				DoUpdates: clause.Assignments(map[string]any{
					"count": gorm.Expr("referer_stats.count + excluded.count"),
				}),
			}).Create(&refRows).Error; err != nil {
				return err
			}
		}
		if len(refImgRows) > 0 {
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "image_id"}, {Name: "host"}, {Name: "date"}},
				DoUpdates: clause.Assignments(map[string]any{
					"count": gorm.Expr("referer_image_stats.count + excluded.count"),
				}),
			}).Create(&refImgRows).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		// 刷盘失败把计数合并回活动缓冲,下个周期重试——短暂 DB 故障不再永久丢一窗数据
		// (codex 终审 D-①)。
		s.mu.Lock()
		for k, v := range access {
			s.access[k] += v
		}
		for k, v := range referer {
			s.referer[k] += v
		}
		for k, v := range refererImg {
			s.refererImg[k] += v
		}
		s.mu.Unlock()
	}
	return err
}

// PurgeOlderThan deletes access/referer rows with date older than cutoff days.
func (s *Service) PurgeOlderThan(days int) error {
	if days <= 0 {
		days = StatsRetentionDays
	}
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	if err := s.db.Where("date < ?", cutoff).Delete(&model.AccessStat{}).Error; err != nil {
		return err
	}
	if err := s.db.Where("date < ?", cutoff).Delete(&model.RefererStat{}).Error; err != nil {
		return err
	}
	return s.db.Where("date < ?", cutoff).Delete(&model.RefererImageStat{}).Error
}

// Start ticker 周期 Flush; 每日一次滚动删除; ctx.Done 时终刷一次后返回。
func (s *Service) Start(ctx context.Context) {
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()
	_ = s.PurgeOlderThan(StatsRetentionDays)
	s.lastPurge = time.Now()
	for {
		select {
		case <-ctx.Done():
			_ = s.Flush()
			return
		case <-ticker.C:
			_ = s.Flush()
			if time.Since(s.lastPurge) >= 24*time.Hour {
				_ = s.PurgeOlderThan(StatsRetentionDays)
				s.lastPurge = time.Now()
			}
		}
	}
}

const hotlinkTTL = 30 * time.Second

// Hotlink 30s TTL 快照读 settings(读失败回退上次快照,首次失败回退 DefaultHotlink)。
// 过期时乐观续期:抢到刷新权的单个请求锁外查库,其余并发请求直接拿旧值不排队
// (serve 热路径不因 TTL 到期串行阻塞;查库失败沿用旧值,下个 TTL 再试;codex 评审 Task2)。
func (s *Service) Hotlink() HotlinkConfig {
	s.hotMu.Lock()
	if s.hotValid && time.Since(s.hotAt) < hotlinkTTL {
		cfg := s.hotCfg
		s.hotMu.Unlock()
		return cfg
	}
	// 抢刷新权:先把 hotAt 推到现在——已有旧值的并发者走上面快路径拿旧值;
	// 冷启动/Invalidate 后的短暂并发重读无害(都读到新值)。
	stale := s.hotCfg
	staleValid := s.hotValid
	gen := s.hotGen
	s.hotAt = time.Now()
	s.hotMu.Unlock()

	var cfg HotlinkConfig
	if err := s.settings.Get(model.SettingHotlink, &cfg); err != nil {
		if staleValid {
			return stale
		}
		return DefaultHotlink()
	}
	if cfg.AllowedDomains == nil {
		cfg.AllowedDomains = []string{}
	}
	s.hotMu.Lock()
	// 读库期间若被 Invalidate(代次变化),放弃发布——本次读到的可能是保存前的旧行,
	// 发布会让旧规则再存活一个 TTL(codex 终审)。本请求仍返回读到的值,下个请求重读。
	if s.hotGen == gen {
		s.hotCfg = cfg
		s.hotAt = time.Now()
		s.hotValid = true
	}
	s.hotMu.Unlock()
	return cfg
}

// InvalidateHotlink 使 hotlink 快照立即失效,并递增代次废弃在途刷新的发布权。
func (s *Service) InvalidateHotlink() {
	s.hotMu.Lock()
	s.hotValid = false
	s.hotGen++
	s.hotMu.Unlock()
}

// ImageStats 属主统计:按 key 查 image 且 user_id 匹配(不存在/非属主 → ErrNotFound;
// 软删也算不存在,用默认 scope 查询即可),返回 total(全部日期 SUM)与近 30 天逐日
// (缺日补 0,升序,含今天,恒 30 项)。
func (s *Service) ImageStats(userID uint64, key string) (int64, []DayViews, error) {
	var img model.Image
	err := s.db.Where("key = ? AND user_id = ?", key, userID).First(&img).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil, ErrNotFound
	}
	if err != nil {
		return 0, nil, err
	}

	// 单查询取全量逐日行,total 与近 30 天明细同一时点派生——两次独立查询会跨 Flush
	// 撕裂(明细之和可能大于 total;codex 终审)。行数上界=有数据的天数,可控。
	var rows []model.AccessStat
	if err := s.db.Where("image_id = ?", img.ID).Find(&rows).Error; err != nil {
		return 0, nil, err
	}
	var total int64
	byDate := make(map[string]int64, len(rows))
	for _, r := range rows {
		total += r.Views
		byDate[r.Date] = r.Views
	}

	now := time.Now()
	start := now.AddDate(0, 0, -29)

	daily := make([]DayViews, 0, 30)
	for i := 0; i < 30; i++ {
		d := start.AddDate(0, 0, i).Format("2006-01-02")
		daily = append(daily, DayViews{Date: d, Views: byDate[d]})
	}
	return total, daily, nil
}
