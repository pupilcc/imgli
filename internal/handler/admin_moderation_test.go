package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/adminsvc"
	"github.com/yixian-huang/imgli/internal/service/moderation"
)

func setupModerationTestHandler(t *testing.T) (*adminsvc.Service, *chi.Mux) {
	t.Helper()
	db := model.TestDB(t)
	adm := adminsvc.New(db)
	admin := &model.User{Username: "modadmin", Email: "modadmin@img.li", GroupID: 1, IsAdmin: true}
	if err := db.Create(admin).Error; err != nil {
		t.Fatal(err)
	}
	ah := &AdminHandlers{D: AdminDeps{Adm: adm}}
	mux := chi.NewRouter()
	mux.With(withPrincipal(admin)).Post("/api/v1/admin/settings/moderation/test", ah.TestModeration)
	return adm, mux
}

func TestModerationTestDisabled(t *testing.T) {
	_, mux := setupModerationTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/settings/moderation/test", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "请先启用") {
		t.Errorf("body 应含「请先启用」, got %s", rec.Body.String())
	}
}

func TestModerationTestWebhookOK(t *testing.T) {
	adm, mux := setupModerationTestHandler(t)

	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"score": 0.42})
	}))
	defer webhook.Close()

	raw, err := json.Marshal(moderation.Config{
		Enabled: true, Provider: "webhook", Endpoint: webhook.URL,
		Threshold: 0.8, Action: "pending",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := adm.PutSettings(map[string]json.RawMessage{"moderation": raw}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/settings/moderation/test", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Status bool `json:"status"`
		Data   struct {
			Score float64 `json:"score"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.Status || env.Data.Score != 0.42 {
		t.Errorf("envelope = %+v, want status true score 0.42", env)
	}
}

// codex 终审 F3:aliyun 无公网 URL 无法测试,应明确拒绝而非给虚假成功。
func TestModerationTestAliyunUnsupported(t *testing.T) {
	adm, mux := setupModerationTestHandler(t)
	raw, err := json.Marshal(moderation.Config{
		Enabled: true, Provider: "aliyun", AccessKeyID: "ak", AccessKeySecret: "sk", Region: "cn-shanghai",
		Threshold: 0.8, Action: "pending",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := adm.PutSettings(map[string]json.RawMessage{"moderation": raw}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/settings/moderation/test", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "阿里云") {
		t.Errorf("aliyun 测试应 400 含「阿里云」, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestModerationTestWebhookErr(t *testing.T) {
	adm, mux := setupModerationTestHandler(t)

	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer webhook.Close()

	raw, err := json.Marshal(moderation.Config{
		Enabled: true, Provider: "webhook", Endpoint: webhook.URL,
		Threshold: 0.8, Action: "pending",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := adm.PutSettings(map[string]json.RawMessage{"moderation": raw}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/settings/moderation/test", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "测试失败") {
		t.Errorf("body 应含「测试失败」, got %s", rec.Body.String())
	}
}
