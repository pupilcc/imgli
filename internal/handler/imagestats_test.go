package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/adminsvc"
	"github.com/yixian-huang/imgli/internal/service/stats"
)

func TestImageStatsOwnerAndOther(t *testing.T) {
	db := model.TestDB(t)
	st := stats.New(db, time.Hour)

	owner := &model.User{Username: "own", Email: "own@img.li", GroupID: 1}
	other := &model.User{Username: "oth", Email: "oth@img.li", GroupID: 1}
	if err := db.Create(owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(other).Error; err != nil {
		t.Fatal(err)
	}
	f := &model.File{Hash: "isthash", StoragePolicyID: 1, Path: "p/ist", Size: 1, MIME: "image/png", RefCount: 1}
	if err := db.Create(f).Error; err != nil {
		t.Fatal(err)
	}
	img := &model.Image{
		Key: "istatskey0001", UserID: &owner.ID, FileID: f.ID,
		Name: "n", Ext: "png", Visibility: "public", Status: "normal",
	}
	if err := db.Create(img).Error; err != nil {
		t.Fatal(err)
	}
	today := time.Now().Format("2006-01-02")
	if err := db.Create(&model.AccessStat{ImageID: img.ID, Date: today, Views: 4}).Error; err != nil {
		t.Fatal(err)
	}

	h := &ImageHandlers{D: ImageDeps{Stats: st}}
	mux := chi.NewRouter()
	mux.With(withPrincipal(owner)).Get("/api/v1/images/{key}/stats", h.Stats)
	mux.With(withPrincipal(other)).Get("/other/images/{key}/stats", h.Stats)

	// 属主 200
	req := httptest.NewRequest(http.MethodGet, "/api/v1/images/istatskey0001/stats", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("owner status=%d body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Status bool `json:"status"`
		Data   struct {
			Total int64 `json:"total"`
			Daily []struct {
				Date  string `json:"date"`
				Views int64  `json:"views"`
			} `json:"daily"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.Total != 4 {
		t.Errorf("total=%d want 4", env.Data.Total)
	}
	if len(env.Data.Daily) != 30 {
		t.Errorf("daily len=%d want 30", len(env.Data.Daily))
	}

	// 他人 404
	req = httptest.NewRequest(http.MethodGet, "/other/images/istatskey0001/stats", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Errorf("other status=%d want 404 body=%s", rec.Code, rec.Body.String())
	}
}

func TestHotlinkPutSettingsInvalidatesSnapshot(t *testing.T) {
	db := model.TestDB(t)
	st := stats.New(db, time.Hour)
	adm := adminsvc.New(db)

	// 预热快照：默认 Enabled=false
	if st.Hotlink().Enabled {
		t.Fatal("播种默认 Enabled 应为 false")
	}

	admin := &model.User{Username: "adm", Email: "adm@img.li", GroupID: 1, IsAdmin: true}
	if err := db.Create(admin).Error; err != nil {
		t.Fatal(err)
	}

	ah := &AdminHandlers{D: AdminDeps{Adm: adm, Stats: st}}
	mux := chi.NewRouter()
	mux.With(withPrincipal(admin)).Put("/api/v1/admin/settings", ah.PutSettings)

	body := []byte(`{"hotlink":{"enabled":true,"allowed_domains":["ok.example"],"allow_empty_referer":true}}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("PutSettings status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !st.Hotlink().Enabled {
		t.Error("Invalidate 后 Hotlink().Enabled 应为 true")
	}
	cfg := st.Hotlink()
	if len(cfg.AllowedDomains) != 1 || cfg.AllowedDomains[0] != "ok.example" {
		t.Errorf("AllowedDomains=%v want [ok.example]", cfg.AllowedDomains)
	}
}
