package moderation

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/config"
	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/settings"
	"github.com/yixian-huang/imgli/internal/service/storagesvc"
)

// ---- WebhookScorer ----

func TestWebhookScorerScoresAndSendsExpectedRequest(t *testing.T) {
	var gotMethod, gotCT, gotAuth string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotCT = r.Header.Get("Content-Type")
		gotAuth = r.Header.Get("Authorization")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"score": 0.87})
	}))
	defer srv.Close()

	sc := &WebhookScorer{Endpoint: srv.URL, APIKey: "secret123"}
	score, err := sc.Score(context.Background(), bytes.NewReader([]byte("fake-image-bytes")), "image/png", "")
	if err != nil {
		t.Fatal(err)
	}
	if score != 0.87 {
		t.Errorf("score = %v, want 0.87", score)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotCT != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", gotCT)
	}
	if gotAuth != "Bearer secret123" {
		t.Errorf("Authorization = %q, want Bearer secret123", gotAuth)
	}
	if string(gotBody) != "fake-image-bytes" {
		t.Errorf("body = %q, want fake-image-bytes", gotBody)
	}
}

func TestWebhookScorerNoAuthHeaderWhenAPIKeyEmpty(t *testing.T) {
	sawAuth := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization") != ""
		json.NewEncoder(w).Encode(map[string]any{"score": 0.1})
	}))
	defer srv.Close()

	sc := &WebhookScorer{Endpoint: srv.URL}
	if _, err := sc.Score(context.Background(), bytes.NewReader([]byte("x")), "image/png", ""); err != nil {
		t.Fatal(err)
	}
	if sawAuth {
		t.Error("api_key 为空不应带 Authorization 头")
	}
}

func TestWebhookScorerNon200Errors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	sc := &WebhookScorer{Endpoint: srv.URL}
	if _, err := sc.Score(context.Background(), bytes.NewReader([]byte("x")), "image/png", ""); err == nil {
		t.Error("非 200 应报错")
	}
}

func TestWebhookScorerMissingScoreFieldErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	sc := &WebhookScorer{Endpoint: srv.URL}
	if _, err := sc.Score(context.Background(), bytes.NewReader([]byte("x")), "image/png", ""); err == nil {
		t.Error("缺 score 字段应报错")
	}
}

// TestWebhookScorerLimitsResponseBodySize 防一个被攻破/配错的审核端点用超大或无限响应体
// 把 worker 拖入 OOM——Score 必须把响应体限流在 64KB（respond.go DecodeJSON 同款惯例）。
//
// 用一个从字节 0 就非法的 body 测不出限流是否生效：json.Decoder 遇到语法错误会立即
// 失败，不管背后限没限流，测试对"是否真的限流"没有区分力。这里构造的是一段完整合法的
// JSON——一个 100KB 的 padding 字符串字段排在 score 字段之前——若不限流，Decode 会顺利
// 读完整个 body 并成功解出 score；一旦限流在 64KB，reader 会在 padding 字符串字面量中途
// 被切断（永远读不到 score 字段与收尾的 `}`），Decode 必然因 JSON 不完整而报错。
// 报错即证明确实发生了截断限流，而不是走到了别的失败分支。
func TestWebhookScorerLimitsResponseBodySize(t *testing.T) {
	padding := bytes.Repeat([]byte("x"), 100<<10) // 100KB，超过 64KB 限流线
	body := append(append([]byte(`{"padding":"`), padding...), []byte(`","score":0.5}`)...)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer srv.Close()

	sc := &WebhookScorer{Endpoint: srv.URL}
	if _, err := sc.Score(context.Background(), bytes.NewReader([]byte("x")), "image/png", ""); err == nil {
		t.Error("响应体应在 64KB 处被截断，本可合法解出的 score 字段读不到，应报错")
	}
}

func TestWebhookScorerDefaultClientHasFifteenSecondTimeout(t *testing.T) {
	sc := &WebhookScorer{Endpoint: "http://example.invalid"}
	if got := sc.httpClient().Timeout; got != 15*time.Second {
		t.Errorf("默认超时 = %v, want 15s", got)
	}
	custom := &http.Client{Timeout: time.Second}
	sc2 := &WebhookScorer{Endpoint: "http://example.invalid", Client: custom}
	if sc2.httpClient() != custom {
		t.Error("显式设置的 Client 应原样使用")
	}
}

// ---- ModerateTask ----

// setupService 建 TestDB + 临时本地存储根，返回可直接跑任务的 Service 及其协作对象
// （seedImage 需要复用同一个 res 才能把测试图片写到 Service 实际会读取的路径下）。
func setupService(t *testing.T) (svc *Service, db *gorm.DB, res *storagesvc.Resolver, st *settings.Service) {
	t.Helper()
	db = model.TestDB(t)
	cfg := &config.Config{DataDir: t.TempDir(), BaseURL: "https://img.li"}
	res = storagesvc.New(cfg, db)
	st = settings.New(db)
	svc = New(db, st, res)
	return
}

// seedImage 手写 file+image 行并把字节写入种子的本地策略(ID=1)根下，模拟一张已上传的图。
func seedImage(t *testing.T, db *gorm.DB, res *storagesvc.Resolver, key, hash, status string, whitelisted bool) *model.Image {
	t.Helper()
	var policy model.StoragePolicy
	if err := db.First(&policy, 1).Error; err != nil {
		t.Fatal(err)
	}
	driver, err := res.Driver(&policy)
	if err != nil {
		t.Fatal(err)
	}
	path := key + ".png"
	if err := driver.Put(context.Background(), path, bytes.NewReader([]byte("pixel-bytes-"+key))); err != nil {
		t.Fatal(err)
	}
	file := &model.File{Hash: hash, StoragePolicyID: policy.ID, Path: path, Size: 20, MIME: "image/png", RefCount: 1}
	if err := db.Create(file).Error; err != nil {
		t.Fatal(err)
	}
	img := &model.Image{
		Key: key, FileID: file.ID, Name: key + ".png", Ext: "png",
		Status: status, IsWhitelisted: whitelisted, Visibility: "public",
	}
	if err := db.Create(img).Error; err != nil {
		t.Fatal(err)
	}
	return img
}

func taskPayload(t *testing.T, imageID uint64) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"image_id": imageID})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestModerateTaskFlagsAndRejectsOverThreshold(t *testing.T) {
	svc, db, res, st := setupService(t)
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"score": 0.9})
	}))
	defer webhook.Close()
	if err := st.Set(model.SettingModeration, Config{
		Enabled: true, Provider: "webhook", Endpoint: webhook.URL, Threshold: 0.5, Action: "rejected",
	}); err != nil {
		t.Fatal(err)
	}
	img := seedImage(t, db, res, "k1rejected", "h1rejected", "normal", false)

	if err := svc.ModerateTask(context.Background(), taskPayload(t, img.ID)); err != nil {
		t.Fatal(err)
	}

	var got model.Image
	if err := db.First(&got, img.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != "rejected" {
		t.Errorf("status = %q, want rejected", got.Status)
	}
	if got.NSFWScore == nil || *got.NSFWScore != 0.9 {
		t.Errorf("nsfw_score = %v, want 0.9", got.NSFWScore)
	}

	var logs []model.AuditLog
	db.Where("action = ?", "moderation_flag").Find(&logs)
	if len(logs) != 1 {
		t.Fatalf("audit 行数 = %d, want 1", len(logs))
	}
	if logs[0].ActorType != "system" {
		t.Errorf("actor_type = %q, want system", logs[0].ActorType)
	}
	var detail map[string]any
	json.Unmarshal([]byte(logs[0].Detail), &detail)
	if detail["key"] != img.Key || detail["action"] != "rejected" {
		t.Errorf("audit detail = %+v", detail)
	}
}

func TestModerateTaskPendingAction(t *testing.T) {
	svc, db, res, st := setupService(t)
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"score": 0.75})
	}))
	defer webhook.Close()
	if err := st.Set(model.SettingModeration, Config{
		Enabled: true, Provider: "webhook", Endpoint: webhook.URL, Threshold: 0.5, Action: "pending",
	}); err != nil {
		t.Fatal(err)
	}
	img := seedImage(t, db, res, "k2pending", "h2pending", "normal", false)

	if err := svc.ModerateTask(context.Background(), taskPayload(t, img.ID)); err != nil {
		t.Fatal(err)
	}

	var got model.Image
	db.First(&got, img.ID)
	if got.Status != "pending" {
		t.Errorf("status = %q, want pending", got.Status)
	}
	if got.NSFWScore == nil || *got.NSFWScore != 0.75 {
		t.Errorf("nsfw_score = %v, want 0.75", got.NSFWScore)
	}
}

func TestModerateTaskBelowThresholdOnlyRecordsScore(t *testing.T) {
	svc, db, res, st := setupService(t)
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"score": 0.2})
	}))
	defer webhook.Close()
	if err := st.Set(model.SettingModeration, Config{
		Enabled: true, Provider: "webhook", Endpoint: webhook.URL, Threshold: 0.5, Action: "rejected",
	}); err != nil {
		t.Fatal(err)
	}
	img := seedImage(t, db, res, "k3below", "h3below", "normal", false)

	if err := svc.ModerateTask(context.Background(), taskPayload(t, img.ID)); err != nil {
		t.Fatal(err)
	}

	var got model.Image
	db.First(&got, img.ID)
	if got.Status != "normal" {
		t.Errorf("status = %q, want normal (未过阈值不应改状态)", got.Status)
	}
	if got.NSFWScore == nil || *got.NSFWScore != 0.2 {
		t.Errorf("nsfw_score = %v, want 0.2（无论是否过阈值都应记分）", got.NSFWScore)
	}
	var n int64
	db.Model(&model.AuditLog{}).Where("action = ?", "moderation_flag").Count(&n)
	if n != 0 {
		t.Errorf("未过阈值不应落 audit, got %d", n)
	}
}

func TestModerateTaskSkipsWhitelisted(t *testing.T) {
	svc, db, res, st := setupService(t)
	called := false
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		json.NewEncoder(w).Encode(map[string]any{"score": 0.99})
	}))
	defer webhook.Close()
	if err := st.Set(model.SettingModeration, Config{
		Enabled: true, Provider: "webhook", Endpoint: webhook.URL, Threshold: 0.1, Action: "rejected",
	}); err != nil {
		t.Fatal(err)
	}
	img := seedImage(t, db, res, "k4white", "h4white", "normal", true)

	if err := svc.ModerateTask(context.Background(), taskPayload(t, img.ID)); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("加白图不应调用 webhook 打分")
	}
	var got model.Image
	db.First(&got, img.ID)
	if got.Status != "normal" {
		t.Errorf("status = %q, want normal（加白不应被机审改状态）", got.Status)
	}
	if got.NSFWScore != nil {
		t.Errorf("nsfw_score = %v, 加白图不应被打分", got.NSFWScore)
	}
}

// TestApplyStatusDecisionGuardsAgainstConcurrentWhitelist 回归测试：ModerateTask 打分是
// 异步的，存在一个窗口——任务已读到 img.IsWhitelisted=false（入口检查通过）之后、状态决定
// 写句执行之前，管理员并发把这张图加白（SetWhitelist 不改 status，图仍是 status=normal）。
// 若最终状态写只守 status='normal' AND deleted_at IS NULL，这个窗口里机审仍会把刚被管理员
// 显式信任的图打成 pending/rejected，覆盖人工决定。
//
// 复现方式：图先落库为 normal+is_whitelisted=true（模拟"加白发生在状态决定写之前"这一读后
// 状态），直接重放 applyStatusDecision（状态决定写句本身，ModerateTask 生产代码路径上
// 唯一做状态覆盖判定的地方）断言 0 行受影响、status 原样保持 normal。
func TestApplyStatusDecisionGuardsAgainstConcurrentWhitelist(t *testing.T) {
	svc, db, res, _ := setupService(t)
	img := seedImage(t, db, res, "k9race", "h9race", "normal", true)

	rows, err := svc.applyStatusDecision(img.ID, "rejected")
	if err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Errorf("rows = %d, want 0（is_whitelisted=true 应挡住状态覆盖）", rows)
	}

	var got model.Image
	db.First(&got, img.ID)
	if got.Status != "normal" {
		t.Errorf("status = %q, want normal（加白图的状态不应被机审决定覆盖）", got.Status)
	}
}

func TestModerateTaskSkipsAlreadyDecidedImage(t *testing.T) {
	svc, db, res, st := setupService(t)
	called := false
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		json.NewEncoder(w).Encode(map[string]any{"score": 0.99})
	}))
	defer webhook.Close()
	if err := st.Set(model.SettingModeration, Config{
		Enabled: true, Provider: "webhook", Endpoint: webhook.URL, Threshold: 0.1, Action: "rejected",
	}); err != nil {
		t.Fatal(err)
	}
	img := seedImage(t, db, res, "k5rej", "h5rej", "rejected", false)

	if err := svc.ModerateTask(context.Background(), taskPayload(t, img.ID)); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("已是 rejected（人工处理过）的图不应再打分")
	}
}

func TestModerateTaskSkipsWhenDisabled(t *testing.T) {
	svc, db, res, _ := setupService(t)
	called := false
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		json.NewEncoder(w).Encode(map[string]any{"score": 0.99})
	}))
	defer webhook.Close()
	// 播种默认值就是 disabled——不显式 Set moderation 配置。
	img := seedImage(t, db, res, "k6off", "h6off", "normal", false)

	if err := svc.ModerateTask(context.Background(), taskPayload(t, img.ID)); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("disabled 时不应调用 webhook")
	}
	var got model.Image
	db.First(&got, img.ID)
	if got.NSFWScore != nil {
		t.Error("disabled 时不应写 nsfw_score")
	}
}

func TestModerateTaskSkipsMissingImage(t *testing.T) {
	svc, _, _, st := setupService(t)
	if err := st.Set(model.SettingModeration, Config{
		Enabled: true, Provider: "webhook", Endpoint: "http://example.invalid", Threshold: 0.5, Action: "rejected",
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.ModerateTask(context.Background(), taskPayload(t, 999999)); err != nil {
		t.Errorf("图不存在应静默跳过, got err=%v", err)
	}
}

func TestModerateTaskSkipsSoftDeletedImage(t *testing.T) {
	svc, db, res, st := setupService(t)
	called := false
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		json.NewEncoder(w).Encode(map[string]any{"score": 0.99})
	}))
	defer webhook.Close()
	if err := st.Set(model.SettingModeration, Config{
		Enabled: true, Provider: "webhook", Endpoint: webhook.URL, Threshold: 0.1, Action: "rejected",
	}); err != nil {
		t.Fatal(err)
	}
	img := seedImage(t, db, res, "k7del", "h7del", "normal", false)
	if err := db.Delete(&model.Image{}, img.ID).Error; err != nil {
		t.Fatal(err)
	}

	if err := svc.ModerateTask(context.Background(), taskPayload(t, img.ID)); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("已软删的图不应再打分")
	}
}

func TestModerateTaskScoringFailureReturnsErrorForRetry(t *testing.T) {
	svc, db, res, st := setupService(t)
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer webhook.Close()
	if err := st.Set(model.SettingModeration, Config{
		Enabled: true, Provider: "webhook", Endpoint: webhook.URL, Threshold: 0.5, Action: "rejected",
	}); err != nil {
		t.Fatal(err)
	}
	img := seedImage(t, db, res, "k8fail", "h8fail", "normal", false)

	if err := svc.ModerateTask(context.Background(), taskPayload(t, img.ID)); err == nil {
		t.Error("打分失败应返回 err 供任务系统重试")
	}
	var got model.Image
	db.First(&got, img.ID)
	if got.Status != "normal" {
		t.Errorf("打分失败不应拦图, status = %q", got.Status)
	}
}

func TestModerateTaskMalformedPayloadErrors(t *testing.T) {
	svc, _, _, _ := setupService(t)
	if err := svc.ModerateTask(context.Background(), "not-json"); err == nil {
		t.Error("非法 payload 应报错")
	}
}
