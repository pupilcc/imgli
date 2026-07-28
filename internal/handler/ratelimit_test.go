package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/model"
)

func TestLimiterSweepDropsExpiredWindows(t *testing.T) {
	l := NewLimiter()
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return base }
	// 种下一批「已过期」键与一条仍有效键
	l.mu.Lock()
	for i := 0; i < 10; i++ {
		l.hits["old|"+itoa(uint64(i))] = &window{count: 1, reset: base.Add(-time.Minute)}
	}
	l.hits["live"] = &window{count: 3, reset: base.Add(time.Minute)}
	l.mu.Unlock()
	if l.Len() != 11 {
		t.Fatalf("seed len=%d", l.Len())
	}
	l.Sweep()
	if l.Len() != 1 {
		t.Fatalf("sweep 后应只剩 live, got %d", l.Len())
	}
	l.mu.Lock()
	if _, ok := l.hits["live"]; !ok {
		t.Error("未过期键不应被清")
	}
	l.mu.Unlock()
}

func TestLimiterMaybeSweepAtThreshold(t *testing.T) {
	l := NewLimiter()
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return base }
	l.mu.Lock()
	for i := 0; i < sweepThreshold; i++ {
		l.hits["e|"+itoa(uint64(i))] = &window{count: 1, reset: base.Add(-time.Second)}
	}
	l.mu.Unlock()
	// 触发 allow → maybeSweep
	l.mu.Lock()
	_ = l.allow("new", time.Minute, 5)
	l.mu.Unlock()
	// 过期键清掉后只剩新窗口
	if n := l.Len(); n != 1 {
		t.Fatalf("阈值清扫后应仅 1 键, got %d", n)
	}
}

func TestLimiterMultScalesNamedBucket(t *testing.T) {
	// mult=3 → perMin 2 的桶实际放行 6 次,第 7 次才 429
	l := NewLimiterMult(3)
	h := l.Middleware("test", 2)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := func() int {
		rec := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/", nil)
		r.RemoteAddr = "9.9.9.9:5"
		h.ServeHTTP(rec, r)
		return rec.Code
	}
	for i := 0; i < 6; i++ {
		if code := req(); code != 200 {
			t.Fatalf("第 %d 次应放行(mult 后限额 6), got %d", i+1, code)
		}
	}
	if code := req(); code != http.StatusTooManyRequests {
		t.Errorf("第 7 次应 429, got %d", code)
	}
	// mult<=0 视为 1(不放大)
	if NewLimiterMult(0).scale(5) != 5 || NewLimiterMult(-2).scale(5) != 5 {
		t.Error("mult<=0 应等价 1 倍")
	}
	// 不限桶(perMin<=0)不受倍率影响
	if NewLimiterMult(10).scale(0) != 0 {
		t.Error("perMin<=0 应原样返回")
	}
}

func TestLimiterMiddlewareBlocksOverLimit(t *testing.T) {
	l := NewLimiter()
	h := l.Middleware("test", 2)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/", nil)
		r.RemoteAddr = "1.2.3.4:5"
		h.ServeHTTP(rec, r)
		return rec
	}
	if req().Code != 200 || req().Code != 200 {
		t.Fatal("前两次应放行")
	}
	rec := req()
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("第三次应 429, got %d", rec.Code)
	}
}

func TestLimiterPerIdentityIsolation(t *testing.T) {
	l := NewLimiter()
	if _, _, ok := l.AllowN("userA", 1, 10, 100); !ok {
		t.Fatal("A 首次应放行")
	}
	if _, _, ok := l.AllowN("userA", 1, 10, 100); ok {
		t.Error("A 第二次超每分钟限额应拒绝")
	}
	if _, _, ok := l.AllowN("userB", 1, 10, 100); !ok {
		t.Error("B 独立计数应放行")
	}
}

func TestAllowNReportsTriggeredWindow(t *testing.T) {
	l := NewLimiter()
	// 每分钟 5、每小时 5、每日 1：第一次放行，第二次触发 day 窗口
	l.AllowN("u", 5, 5, 1)
	win, _, ok := l.AllowN("u", 5, 5, 1)
	if ok || win != "day" {
		t.Errorf("应因 day 超限被拒, win=%q ok=%v", win, ok)
	}
}

func TestAllowNZeroMeansUnlimited(t *testing.T) {
	l := NewLimiter()
	for i := 0; i < 50; i++ {
		if _, _, ok := l.AllowN("u", 0, 0, 0); !ok {
			t.Fatal("0 表示不限，不应拒绝")
		}
	}
}

func TestLimiterMiddlewareKeysByUserNotIP(t *testing.T) {
	l := NewLimiter()
	h := l.Middleware("b", 1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 同一 IP、两个不同登录用户：各自独立计数，都应放行首次
	withUser := func(uid uint64) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/", nil)
		r.RemoteAddr = "9.9.9.9:1" // 同一 IP
		ctx := context.WithValue(r.Context(), principalKey{}, &Principal{User: &model.User{ID: uid}, Scope: "full"})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r.WithContext(ctx))
		return rec
	}
	if withUser(1).Code != http.StatusOK {
		t.Fatal("用户1首次应放行")
	}
	if withUser(2).Code != http.StatusOK {
		t.Error("用户2共享 IP 但身份不同，应独立计数放行（证明按用户ID而非IP）")
	}
	if withUser(1).Code != http.StatusTooManyRequests {
		t.Error("用户1第二次应 429（每分钟限1）")
	}
}

// TestIPMiddlewareSharesBucketAcrossUsers 同 IP 两个登录身份共享 forgot 桶(恒按 IP)。
func TestIPMiddlewareSharesBucketAcrossUsers(t *testing.T) {
	l := NewLimiter()
	h := l.IPMiddleware("forgot", 1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	withUser := func(uid uint64) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/auth/forgot-password", nil)
		r.RemoteAddr = "9.9.9.9:1"
		ctx := context.WithValue(r.Context(), principalKey{}, &Principal{User: &model.User{ID: uid}, Scope: "full"})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r.WithContext(ctx))
		return rec
	}
	if withUser(1).Code != http.StatusOK {
		t.Fatal("用户1首次应放行")
	}
	if withUser(2).Code != http.StatusTooManyRequests {
		t.Error("用户2同 IP 应共享 forgot 桶被 429")
	}
}

// TestGroupMiddlewareGuestExceedsRatePerDay 匿名请求按游客组三档限速（种子
// rate_per_day=3），第 4 次应 429。
func TestGroupMiddlewareGuestExceedsRatePerDay(t *testing.T) {
	db := model.TestDB(t)
	l := NewLimiter()
	h := l.GroupMiddleware(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := func() int {
		r := httptest.NewRequest("POST", "/upload", nil)
		r.RemoteAddr = "5.6.7.8:1"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Code
	}
	for i := 0; i < 3; i++ {
		if code := req(); code != http.StatusOK {
			t.Fatalf("第 %d 次应放行, got %d", i+1, code)
		}
	}
	if code := req(); code != http.StatusTooManyRequests {
		t.Errorf("第 4 次应 429（游客组 rate_per_day=3）, got %d", code)
	}
}

// TestGroupMiddlewareGuestIsolatesByIP 不同匿名 IP 各自独立计数。
func TestGroupMiddlewareGuestIsolatesByIP(t *testing.T) {
	db := model.TestDB(t)
	l := NewLimiter()
	h := l.GroupMiddleware(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	reqFrom := func(ip string) int {
		r := httptest.NewRequest("POST", "/upload", nil)
		r.RemoteAddr = ip + ":1"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Code
	}
	for i := 0; i < 3; i++ {
		reqFrom("1.1.1.1")
	}
	if code := reqFrom("1.1.1.1"); code != http.StatusTooManyRequests {
		t.Fatalf("IP1 第 4 次应 429, got %d", code)
	}
	if code := reqFrom("2.2.2.2"); code != http.StatusOK {
		t.Errorf("IP2 独立计数应放行, got %d", code)
	}
}

// TestGroupMiddlewareLoggedInUserRateLimitsByGroup 登录用户按其所在组三档限速
// （非游客组），证明三档接线对登录用户同样生效（而不只是匿名）。
func TestGroupMiddlewareLoggedInUserRateLimitsByGroup(t *testing.T) {
	db := model.TestDB(t)
	tiny := model.UserGroup{Name: "tiny", RatePerMinute: 1, RatePerHour: 100, RatePerDay: 100}
	if err := db.Create(&tiny).Error; err != nil {
		t.Fatal(err)
	}
	u := model.User{Username: "ratelimu1", Email: "ratelimu1@example.com", GroupID: tiny.ID}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	l := NewLimiter()
	h := l.GroupMiddleware(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := func() int {
		r := httptest.NewRequest("POST", "/upload", nil)
		ctx := context.WithValue(r.Context(), principalKey{}, &Principal{User: &u, Scope: "full"})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r.WithContext(ctx))
		return rec.Code
	}
	if code := req(); code != http.StatusOK {
		t.Fatalf("首次应放行, got %d", code)
	}
	if code := req(); code != http.StatusTooManyRequests {
		t.Errorf("第二次应 429（组 rate_per_minute=1）, got %d", code)
	}
}

// TestGroupMiddlewareGroupLoadErrorFallsThrough 查组失败（GroupID 指向不存在的组）
// 时应放行给下游，而不是让限速中间件自己吞掉请求。
func TestGroupMiddlewareGroupLoadErrorFallsThrough(t *testing.T) {
	db := model.TestDB(t)
	// 不落库——中间件只按 p.User.GroupID 查组，不查 users 表，故构造一个
	// 指向不存在组的内存 User 即可触发查组失败分支，无需担心 FK 约束。
	u := model.User{ID: 123456, GroupID: 999999}
	l := NewLimiter()
	h := l.GroupMiddleware(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	r := httptest.NewRequest("POST", "/upload", nil)
	ctx := context.WithValue(r.Context(), principalKey{}, &Principal{User: &u, Scope: "full"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r.WithContext(ctx))
	if rec.Code != http.StatusOK {
		t.Errorf("查组失败应放行给下游, got %d", rec.Code)
	}
}

// TestAllowNReturnsRetryAfter 被拒时返回触发窗口的剩余等待时长。
func TestAllowNReturnsRetryAfter(t *testing.T) {
	l := NewLimiter()
	base := time.Now()
	l.now = func() time.Time { return base }
	l.AllowN("u", 0, 0, 1) // 占满 day 窗口
	base = base.Add(10 * time.Hour)
	win, retry, ok := l.AllowN("u", 0, 0, 1)
	if ok || win != "day" {
		t.Fatalf("应因 day 超限被拒, win=%q ok=%v", win, ok)
	}
	if retry != 14*time.Hour {
		t.Errorf("retry = %v, want 14h（day 窗口剩余）", retry)
	}
}

// TestGroupMiddleware429HasRetryAfterAndWindowMessage 三档 429 带 Retry-After 头与触发档名文案。
// 游客组种子 3/分·3/时·3/日,同一分钟内第 4 次先触发 minute 档。
func TestGroupMiddleware429HasRetryAfterAndWindowMessage(t *testing.T) {
	db := model.TestDB(t)
	l := NewLimiter()
	h := l.GroupMiddleware(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/upload", nil)
		r.RemoteAddr = "7.7.7.7:1"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec
	}
	for i := 0; i < 3; i++ {
		req()
	}
	rec := req()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("第 4 次应 429, got %d", rec.Code)
	}
	ra, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || ra < 1 || ra > 60 {
		t.Errorf("Retry-After = %q, 应为 1..60 的整数秒（minute 档）", rec.Header().Get("Retry-After"))
	}
	if !strings.Contains(rec.Body.String(), "每分钟") {
		t.Errorf("响应应含触发档名「每分钟」, body=%s", rec.Body.String())
	}
}
