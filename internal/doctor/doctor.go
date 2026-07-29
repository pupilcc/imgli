// Package doctor implements `imgli doctor` self-host diagnostics.
package doctor

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/config"
	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/storagesvc"
)

// Level is the severity of a check result.
type Level string

const (
	OK   Level = "ok"
	Warn Level = "warn"
	Fail Level = "fail"
)

// Check is one diagnostic line.
type Check struct {
	Name    string
	Level   Level
	Message string
}

// Report is the full doctor run.
type Report struct {
	Checks   []Check
	HardFail bool // true if any Fail
}

func (r *Report) add(name string, level Level, msg string) {
	r.Checks = append(r.Checks, Check{Name: name, Level: level, Message: msg})
	if level == Fail {
		r.HardFail = true
	}
}

// Run loads-independent checks against cfg; opens DB and probes local storage when possible.
func Run(cfg *config.Config) Report {
	var r Report
	if cfg == nil {
		r.add("config", Fail, "配置为空")
		return r
	}

	checkDataDir(cfg, &r)
	checkBaseURL(cfg, &r)
	checkTrustProxy(cfg, &r)
	checkListen(cfg, &r)

	db, err := model.Open(cfg)
	if err != nil {
		r.add("database", Fail, fmt.Sprintf("打开失败: %v", err))
		return r
	}
	defer func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	}()
	checkDatabase(db, cfg, &r)
	checkLocalPolicies(cfg, db, &r)
	checkCDNMetering(db, &r)
	return r
}

// checkCDNMetering warns when any enabled policy has cdn_domain set: admin
// traffic/referer stats are origin-only and under-count edge cache hits.
func checkCDNMetering(db *gorm.DB, r *Report) {
	var policies []model.StoragePolicy
	if err := db.Where("enabled = ?", true).Find(&policies).Error; err != nil {
		r.add("cdn_metering", Warn, fmt.Sprintf("列举存储策略失败: %v（可先 imgli migrate）", err))
		return
	}
	var named []string
	for _, p := range policies {
		cdn := strings.TrimSpace(p.CDNDomain)
		if cdn == "" {
			continue
		}
		named = append(named, fmt.Sprintf("%q→%s", p.Name, cdn))
	}
	if len(named) == 0 {
		r.add("cdn_metering", OK, "无启用策略配置 cdn_domain；仪表盘流量≈源站可见访问")
		return
	}
	r.add("cdn_metering", Warn,
		"已配置 CDN 回源前缀 ("+strings.Join(named, "; ")+
			")：管理端流量/Referer 仅统计源站 /i 门禁命中，边缘缓存未计入；成本请看 CDN/桶账单。见 deploy/ops/admin-stats-metering.md")
}

func checkDataDir(cfg *config.Config, r *Report) {
	dir := strings.TrimSpace(cfg.DataDir)
	if dir == "" {
		r.add("data_dir", Fail, "data_dir 为空")
		return
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		r.add("data_dir", Fail, fmt.Sprintf("解析路径失败: %v", err))
		return
	}
	if st, err := os.Stat(abs); err != nil {
		if os.IsNotExist(err) {
			if mkErr := os.MkdirAll(abs, 0o755); mkErr != nil {
				r.add("data_dir", Fail, fmt.Sprintf("不存在且无法创建 %s: %v", abs, mkErr))
				return
			}
			r.add("data_dir", Warn, fmt.Sprintf("已创建目录 %s", abs))
		} else {
			r.add("data_dir", Fail, fmt.Sprintf("无法访问 %s: %v", abs, err))
			return
		}
	} else if !st.IsDir() {
		r.add("data_dir", Fail, fmt.Sprintf("%s 不是目录", abs))
		return
	}
	// write probe
	probe := filepath.Join(abs, ".imgli-doctor-write")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		r.add("data_dir", Fail, fmt.Sprintf("%s 不可写: %v", abs, err))
		return
	}
	_ = os.Remove(probe)
	r.add("data_dir", OK, fmt.Sprintf("可写: %s", abs))
}

// CheckBaseURL validates public base URL shape (exported for unit tests).
func CheckBaseURL(raw string) (Level, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Fail, "base_url 为空（设置 IMGLI_BASE_URL 或配置 base_url）"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return Fail, fmt.Sprintf("base_url 无效: %v", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return Fail, "base_url 需为 http 或 https"
	}
	if u.Host == "" {
		return Fail, "base_url 缺少 host"
	}
	host := u.Hostname()
	msg := fmt.Sprintf("%s (生成外链用)", strings.TrimRight(raw, "/"))
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return Warn, msg + " — 生产请改为公网域名，否则复制出的链接仅本机可访问"
	}
	if u.Scheme == "http" && host != "localhost" && host != "127.0.0.1" {
		return Warn, msg + " — 非本机建议 https"
	}
	return OK, msg
}

func checkBaseURL(cfg *config.Config, r *Report) {
	lv, msg := CheckBaseURL(cfg.BaseURL)
	r.add("base_url", lv, msg)
}

func checkTrustProxy(cfg *config.Config, r *Report) {
	if cfg.TrustProxy {
		r.add("trust_proxy", Warn,
			"trust_proxy=true：仅在可信反代后开启；否则客户端可伪造 X-Forwarded-For 影响限速与审计 IP")
		return
	}
	// listen on all interfaces without trust_proxy is fine for direct access
	r.add("trust_proxy", OK, "false（直连或未声明反代；若前有 Nginx/Caddy 且需真实 IP，请设 true）")
}

func checkListen(cfg *config.Config, r *Report) {
	listen := strings.TrimSpace(cfg.Listen)
	if listen == "" {
		r.add("listen", Fail, "listen 为空")
		return
	}
	// Try brief bind probe on ephemeral? Don't bind production port if in use.
	// Just validate format.
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		// ":8686" is valid for SplitHostPort
		r.add("listen", Fail, fmt.Sprintf("listen 格式无效 %q: %v", listen, err))
		return
	}
	msg := fmt.Sprintf("%s (host=%q port=%s)", listen, host, port)
	if host == "" || host == "0.0.0.0" {
		msg += " — 监听全部网卡"
	}
	if host == "127.0.0.1" || host == "::1" {
		msg += " — 仅本机；公网需反代到该端口"
	}
	r.add("listen", OK, msg)
}

func checkDatabase(db *gorm.DB, cfg *config.Config, r *Report) {
	sqlDB, err := db.DB()
	if err != nil {
		r.add("database", Fail, fmt.Sprintf("底层连接: %v", err))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		r.add("database", Fail, fmt.Sprintf("ping 失败: %v", err))
		return
	}
	driver := strings.ToLower(strings.TrimSpace(cfg.Database.Driver))
	if driver == "" {
		driver = "sqlite"
	}
	dsn := cfg.Database.DSN
	if driver == "sqlite" && dsn == "" {
		dsn = cfg.SQLiteDefaultDSN()
	}
	// soft schema probe
	if err := db.Exec("SELECT 1").Error; err != nil {
		r.add("database", Fail, fmt.Sprintf("查询失败: %v", err))
		return
	}
	r.add("database", OK, fmt.Sprintf("driver=%s 连通", driver))
	if driver == "sqlite" {
		r.add("database_dsn", OK, fmt.Sprintf("sqlite file ≈ %s", dsn))
	}
}

func checkLocalPolicies(cfg *config.Config, db *gorm.DB, r *Report) {
	var policies []model.StoragePolicy
	if err := db.Where("enabled = ? AND driver = ?", true, "local").Find(&policies).Error; err != nil {
		r.add("storage_local", Warn, fmt.Sprintf("列举策略失败: %v（可先 imgli migrate）", err))
		return
	}
	if len(policies) == 0 {
		r.add("storage_local", OK, "无启用中的 local 策略（可能全用 S3/WebDAV）")
		return
	}
	res := storagesvc.New(cfg, db)
	for i := range policies {
		p := &policies[i]
		root := p.Config["root"]
		if root == "" {
			root = "uploads"
		}
		abs := filepath.Join(cfg.DataDir, root)
		d, err := res.Driver(p)
		if err != nil {
			r.add(fmt.Sprintf("storage_local#%d", p.ID), Fail, fmt.Sprintf("策略 %q 驱动: %v", p.Name, err))
			continue
		}
		// write/read/delete tiny object
		key := fmt.Sprintf(".imgli-doctor/%d-%d", p.ID, time.Now().UnixNano())
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err = d.Put(ctx, key, strings.NewReader("doctor"))
		if err != nil {
			cancel()
			r.add(fmt.Sprintf("storage_local#%d", p.ID), Fail, fmt.Sprintf("策略 %q Put 失败 (%s): %v", p.Name, abs, err))
			continue
		}
		rc, err := d.Open(ctx, key)
		if err != nil {
			cancel()
			r.add(fmt.Sprintf("storage_local#%d", p.ID), Fail, fmt.Sprintf("策略 %q Open 失败: %v", p.Name, err))
			_ = d.Delete(ctx, key)
			continue
		}
		_ = rc.Close()
		_ = d.Delete(ctx, key)
		cancel()
		r.add(fmt.Sprintf("storage_local#%d", p.ID), OK, fmt.Sprintf("策略 %q 读写删 OK (root=%s)", p.Name, abs))
	}
}

// Format prints a human-readable report to a string.
func Format(rep Report) string {
	var b strings.Builder
	b.WriteString("imgli doctor\n")
	for _, c := range rep.Checks {
		mark := "·"
		switch c.Level {
		case OK:
			mark = "ok  "
		case Warn:
			mark = "WARN"
		case Fail:
			mark = "FAIL"
		}
		fmt.Fprintf(&b, "  [%s] %s: %s\n", mark, c.Name, c.Message)
	}
	if rep.HardFail {
		b.WriteString("结果: 存在失败项（exit 1）\n")
	} else {
		b.WriteString("结果: 通过（可有 WARN）\n")
	}
	return b.String()
}
