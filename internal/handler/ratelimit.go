package handler

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
)

// window 固定窗口计数：到期重置。
type window struct {
	count int
	reset time.Time
}

// sweepThreshold 超过此键数时在写路径顺带清扫过期窗口，防 IP/用户桶无限膨胀。
const sweepThreshold = 512

type Limiter struct {
	mu   sync.Mutex
	hits map[string]*window // key = id|bucket|dur
	now  func() time.Time   // 测试可替换
	mult float64            // 固定命名桶 perMin 倍率(默认 1;e2e 等场景放宽)
}

func NewLimiter() *Limiter {
	return &Limiter{hits: map[string]*window{}, now: time.Now, mult: 1}
}

// Sweep 删除已过期窗口（测试/运维可主动调用）。
func (l *Limiter) Sweep() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweepExpired(l.now())
}

// Len 当前窗口键数（测试用）。
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.hits)
}

// sweepExpired 调用方须持锁。
func (l *Limiter) sweepExpired(now time.Time) {
	for k, w := range l.hits {
		if now.After(w.reset) {
			delete(l.hits, k)
		}
	}
}

// maybeSweep 键数超阈时清扫；调用方须持锁。
func (l *Limiter) maybeSweep(now time.Time) {
	if len(l.hits) >= sweepThreshold {
		l.sweepExpired(now)
	}
}

// NewLimiterMult 同 NewLimiter 但把 Middleware/IPMiddleware 的每分钟限额乘以 mult
// (mult<=0 视为 1)。GroupMiddleware 的 DB 驱动上传限额不受影响。
func NewLimiterMult(mult float64) *Limiter {
	l := NewLimiter()
	if mult > 0 {
		l.mult = mult
	}
	return l
}

// scale 按倍率放大 perMin(向上取整,至少 1);perMin<=0(不限)原样返回。
func (l *Limiter) scale(perMin int) int {
	if perMin <= 0 || l.mult <= 1 {
		return perMin
	}
	scaled := int(float64(perMin)*l.mult + 0.999)
	if scaled < perMin {
		scaled = perMin
	}
	return scaled
}

// allow 对单个 (key, dur, limit) 固定窗口计数；limit<=0 表示不限。调用方须持锁。
func (l *Limiter) allow(key string, dur time.Duration, limit int) bool {
	if limit <= 0 {
		return true
	}
	now := l.now()
	l.maybeSweep(now)
	w, ok := l.hits[key]
	if !ok || now.After(w.reset) {
		// 过期键直接覆盖，避免残留
		l.hits[key] = &window{count: 1, reset: now.Add(dur)}
		return true
	}
	if w.count >= limit {
		return false
	}
	w.count++
	return true
}

// AllowN 三档窗口校验（分/时/日），任一超限即拒绝，并返回触发窗口名与建议重试等待。
// 为避免一次调用把三档都 +1 却因后档拒绝而"白扣前档"，此处按顺序检查、全部通过才逐一计数。
func (l *Limiter) AllowN(id string, perMin, perHour, perDay int) (string, time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.maybeSweep(now)
	type w struct {
		name  string
		dur   time.Duration
		limit int
	}
	windows := []w{{"minute", time.Minute, perMin}, {"hour", time.Hour, perHour}, {"day", 24 * time.Hour, perDay}}
	// 预检：不计数，仅看是否会超
	for _, x := range windows {
		if x.limit <= 0 {
			continue
		}
		key := id + "|" + x.name
		if cur, ok := l.hits[key]; ok && !now.After(cur.reset) && cur.count >= x.limit {
			return x.name, cur.reset.Sub(now), false
		}
	}
	// 全通过 → 计数
	for _, x := range windows {
		if x.limit <= 0 {
			continue
		}
		l.allow(id+"|"+x.name, x.dur, x.limit)
	}
	return "", 0, true
}

// windowLabel 触发窗口名 → 展示文案。
func windowLabel(name string) string {
	switch name {
	case "minute":
		return "每分钟"
	case "hour":
		return "每小时"
	case "day":
		return "每日"
	}
	return ""
}

// Middleware 单一每分钟限额，按身份（登录用户 ID 优先，否则 ClientIP）计数。
func (l *Limiter) Middleware(bucket string, perMin int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := "ip:" + ClientIP(r)
			if p := PrincipalFrom(r); p != nil {
				id = "u:" + itoa(p.User.ID)
			}
			l.mu.Lock()
			ok := l.allow(id+"|"+bucket, time.Minute, l.scale(perMin))
			l.mu.Unlock()
			if !ok {
				w.Header().Set("Retry-After", "60")
				Fail(w, http.StatusTooManyRequests, CodeRateLimited, "请求过于频繁，请稍后再试")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// IPMiddleware 恒按 ClientIP 分桶的每分钟限额(不看登录态——用于匿名敏感端点如 forgot-password,
// 防带会话轮换账号绕过 per-IP 限制)。
func (l *Limiter) IPMiddleware(bucket string, perMin int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			l.mu.Lock()
			ok := l.allow("ip:"+ClientIP(r)+"|"+bucket, time.Minute, l.scale(perMin))
			l.mu.Unlock()
			if !ok {
				w.Header().Set("Retry-After", "60")
				Fail(w, http.StatusTooManyRequests, CodeRateLimited, "请求过于频繁，请稍后再试")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GroupMiddleware 按上传者所在组三档限速（登录=其组键 u:id；匿名=游客组键 guest:ip）。
// 查组失败（库错/组不存在）时放行给下游，避免限速中间件自身的故障吞掉请求
// （下游 Save 仍会按各自逻辑判定，如游客开关关闭）。
func (l *Limiter) GroupMiddleware(db *gorm.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var g model.UserGroup
			var key string
			if p := PrincipalFrom(r); p != nil {
				if err := db.First(&g, p.User.GroupID).Error; err != nil {
					next.ServeHTTP(w, r)
					return
				}
				key = "u:" + itoa(p.User.ID)
			} else {
				if err := db.Where("is_guest = ?", true).First(&g).Error; err != nil {
					next.ServeHTTP(w, r)
					return
				}
				key = "guest:ip:" + ClientIP(r)
			}
			if win, retry, ok := l.AllowN(key, g.RatePerMinute, g.RatePerHour, g.RatePerDay); !ok {
				secs := int((retry + time.Second - 1) / time.Second)
				if secs < 1 {
					secs = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(secs))
				msg := "请求过于频繁，请稍后再试"
				if lbl := windowLabel(win); lbl != "" {
					msg = "已达" + lbl + "上传上限，请稍后再试"
				}
				Fail(w, http.StatusTooManyRequests, CodeRateLimited, msg)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func itoa(u uint64) string {
	if u == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for u > 0 {
		i--
		b[i] = byte('0' + u%10)
		u /= 10
	}
	return string(b[i:])
}
