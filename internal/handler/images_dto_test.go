package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/imgli/internal/config"
	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/imagesvc"
	"github.com/yixian-huang/imgli/internal/service/storagesvc"
)

// TestImageDTOExpiresAt 列表/详情 DTO 含 expires_at：有值→RFC3339，永久→null。
func TestImageDTOExpiresAt(t *testing.T) {
	db := model.TestDB(t)
	cfg := &config.Config{DataDir: t.TempDir(), BaseURL: "https://img.li"}
	u := &model.User{Username: "dto", Email: "dto@img.li", GroupID: 1}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	f1 := &model.File{Hash: "dtoh1", StoragePolicyID: 1, Path: "p/d1", Size: 10, MIME: "image/png", Width: 1, Height: 1, RefCount: 1}
	f2 := &model.File{Hash: "dtoh2", StoragePolicyID: 1, Path: "p/d2", Size: 20, MIME: "image/png", Width: 2, Height: 2, RefCount: 1}
	db.Create(f1)
	db.Create(f2)
	exp := time.Date(2030, 6, 1, 12, 0, 0, 0, time.UTC)
	imgExp := &model.Image{
		Key: "dtoexpkey0001", UserID: &u.ID, FileID: f1.ID,
		Name: "with-exp", Ext: "png", Visibility: "public", Status: "normal",
		ExpiresAt: &exp,
	}
	imgPerm := &model.Image{
		Key: "dtopermkey001", UserID: &u.ID, FileID: f2.ID,
		Name: "perm", Ext: "png", Visibility: "public", Status: "normal",
	}
	db.Create(imgExp)
	db.Create(imgPerm)

	h := &ImageHandlers{D: ImageDeps{
		Img: imagesvc.New(db, storagesvc.New(cfg, db), nil),
		Res: storagesvc.New(cfg, db),
	}}
	mux := chi.NewRouter()
	mux.With(withPrincipal(u)).Get("/api/v1/images", h.List)
	mux.With(withPrincipal(u)).Get("/api/v1/images/{key}", h.Detail)

	// List
	req := httptest.NewRequest(http.MethodGet, "/api/v1/images", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}
	var listEnv struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listEnv); err != nil {
		t.Fatal(err)
	}
	byKey := map[string]map[string]any{}
	for _, it := range listEnv.Data.Items {
		k, _ := it["key"].(string)
		byKey[k] = it
	}
	if _, ok := byKey[imgExp.Key]["expires_at"]; !ok {
		t.Fatal("list DTO 缺 expires_at 字段")
	}
	if got, _ := byKey[imgExp.Key]["expires_at"].(string); got != exp.Format(time.RFC3339) {
		t.Errorf("list expires_at=%q want %q", got, exp.Format(time.RFC3339))
	}
	if byKey[imgPerm.Key]["expires_at"] != nil {
		t.Errorf("永久图 list expires_at 应 null, got %v", byKey[imgPerm.Key]["expires_at"])
	}

	// Detail 有过期
	req = httptest.NewRequest(http.MethodGet, "/api/v1/images/"+imgExp.Key, nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", rec.Code, rec.Body.String())
	}
	var detEnv struct {
		Data map[string]any `json:"data"`
	}
	json.Unmarshal(rec.Body.Bytes(), &detEnv)
	if got, _ := detEnv.Data["expires_at"].(string); got != exp.Format(time.RFC3339) {
		t.Errorf("detail expires_at=%q want %q", got, exp.Format(time.RFC3339))
	}

	// Detail 永久
	req = httptest.NewRequest(http.MethodGet, "/api/v1/images/"+imgPerm.Key, nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	json.Unmarshal(rec.Body.Bytes(), &detEnv)
	if detEnv.Data["expires_at"] != nil {
		t.Errorf("永久 detail expires_at 应 null, got %v", detEnv.Data["expires_at"])
	}
}
