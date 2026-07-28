package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/config"
	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/settings"
	"github.com/yixian-huang/imgli/internal/service/stats"
	"github.com/yixian-huang/imgli/internal/service/storagesvc"
)

// serveFixture 最小 /i /t 测试环境：公开图 + 物理文件 + 挂载路由。
type serveFixture struct {
	db    *gorm.DB
	stats *stats.Service
	mux   *chi.Mux
	img   *model.Image
	name  string // key.ext
}

func newServeFixture(t *testing.T) *serveFixture {
	t.Helper()
	db := model.TestDB(t)
	dataDir := t.TempDir()
	cfg := &config.Config{DataDir: dataDir, BaseURL: "http://img.li:8686"}

	u := &model.User{Username: "su", Email: "su@img.li", GroupID: 1}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	path := "2024/01/01/hotlinktest01.png"
	f := &model.File{
		Hash: "hotlinkhash0001", StoragePolicyID: 1, Path: path,
		Size: 12, MIME: "image/png", Width: 1, Height: 1, RefCount: 1,
	}
	if err := db.Create(f).Error; err != nil {
		t.Fatal(err)
	}
	img := &model.Image{
		Key: "hotlinkkey001", UserID: &u.ID, FileID: f.ID,
		Name: "hot", Ext: "png", Visibility: "public", Status: "normal",
	}
	if err := db.Create(img).Error; err != nil {
		t.Fatal(err)
	}

	// 物理文件落在 DataDir/uploads（默认策略 root）
	abs := filepath.Join(dataDir, "uploads", filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte("fakepngbytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 缩略图（/t 探测用，计数测试不依赖成功）
	thumbAbs := filepath.Join(dataDir, "uploads", filepath.FromSlash(storagesvc.ThumbKey(f.Surface, f.Hash)))
	if err := os.MkdirAll(filepath.Dir(thumbAbs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(thumbAbs, []byte("fakejpg"), 0o644); err != nil {
		t.Fatal(err)
	}

	st := stats.New(db, time.Hour)
	sh := &ServeHandlers{D: ServeDeps{
		DB: db, Res: storagesvc.New(cfg, db),
		Stats: st, OwnHost: "img.li",
	}}
	mux := chi.NewRouter()
	mux.Get("/i/{name}", sh.Original)
	mux.Get("/t/{name}", sh.Thumbnail)
	return &serveFixture{db: db, stats: st, mux: mux, img: img, name: img.Key + ".png"}
}

func (f *serveFixture) get(path string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, req)
	return rec
}

func TestHotlinkGate(t *testing.T) {
	fx := newServeFixture(t)
	iPath := "/i/" + fx.name

	if err := settings.New(fx.db).Set(model.SettingHotlink, stats.HotlinkConfig{
		Enabled: true, AllowedDomains: []string{"good.example"}, AllowEmptyReferer: false,
	}); err != nil {
		t.Fatal(err)
	}
	fx.stats.InvalidateHotlink()

	// 白名单域 → 200
	rec := fx.get(iPath, map[string]string{"Referer": "https://good.example/p"})
	if rec.Code != 200 {
		t.Errorf("good.example: status=%d want 200 body=%s", rec.Code, rec.Body.String())
	}

	// 恶意外站 → 403 SVG
	rec = fx.get(iPath, map[string]string{"Referer": "https://evil.example/"})
	if rec.Code != 403 {
		t.Errorf("evil: status=%d want 403", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("HOTLINK DENIED")) {
		t.Errorf("evil: body 缺 HOTLINK DENIED: %s", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "svg") {
		t.Errorf("evil: Content-Type=%q want svg", ct)
	}

	// 无 Referer → 403
	rec = fx.get(iPath, nil)
	if rec.Code != 403 {
		t.Errorf("no referer: status=%d want 403", rec.Code)
	}

	// 无 Referer + Accept JSON → 403 信封含 forbidden
	rec = fx.get(iPath, map[string]string{"Accept": "application/json"})
	if rec.Code != 403 {
		t.Errorf("json no referer: status=%d want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "forbidden") {
		t.Errorf("json no referer: body 缺 forbidden: %s", rec.Body.String())
	}

	// 自站带端口 → 200
	rec = fx.get(iPath, map[string]string{"Referer": "http://img.li:8686/x"})
	if rec.Code != 200 {
		t.Errorf("own host: status=%d want 200 body=%s", rec.Code, rec.Body.String())
	}

	// 关闭 enabled → evil 放行
	if err := settings.New(fx.db).Set(model.SettingHotlink, stats.HotlinkConfig{
		Enabled: false, AllowedDomains: []string{"good.example"}, AllowEmptyReferer: false,
	}); err != nil {
		t.Fatal(err)
	}
	fx.stats.InvalidateHotlink()
	rec = fx.get(iPath, map[string]string{"Referer": "https://evil.example/"})
	if rec.Code != 200 {
		t.Errorf("disabled evil: status=%d want 200 body=%s", rec.Code, rec.Body.String())
	}
}

func TestAccessCounting(t *testing.T) {
	fx := newServeFixture(t)
	iPath := "/i/" + fx.name
	tPath := "/t/" + fx.name

	// /i 带 Referer
	if rec := fx.get(iPath, map[string]string{"Referer": "https://from.example/a"}); rec.Code != 200 {
		t.Fatalf("/i with referer status=%d body=%s", rec.Code, rec.Body.String())
	}
	// /i 无 Referer
	if rec := fx.get(iPath, nil); rec.Code != 200 {
		t.Fatalf("/i no referer status=%d body=%s", rec.Code, rec.Body.String())
	}
	// /t 一次（不计）
	_ = fx.get(tPath, map[string]string{"Referer": "https://from.example/a"})
	// 坏 key /i 一次（不计）
	_ = fx.get("/i/nokeyhere0001.png", nil)

	if err := fx.stats.Flush(); err != nil {
		t.Fatal(err)
	}

	var as model.AccessStat
	if err := fx.db.First(&as, "image_id = ?", fx.img.ID).Error; err != nil || as.Views != 2 {
		t.Errorf("access views=%d err=%v want 2", as.Views, err)
	}

	var from model.RefererStat
	if err := fx.db.First(&from, "host = ?", "from.example").Error; err != nil || from.Count != 1 {
		t.Errorf("from.example count=%d err=%v want 1", from.Count, err)
	}
	var direct model.RefererStat
	if err := fx.db.First(&direct, "host = ?", "(direct)").Error; err != nil || direct.Count != 1 {
		t.Errorf("(direct) count=%d err=%v want 1", direct.Count, err)
	}

	var n int64
	fx.db.Model(&model.RefererStat{}).Count(&n)
	if n != 2 {
		t.Errorf("referer 行数=%d want 2（/t 与 404 不应产生行）", n)
	}
	var accessN int64
	fx.db.Model(&model.AccessStat{}).Count(&accessN)
	if accessN != 1 {
		t.Errorf("access 行数=%d want 1", accessN)
	}
}
