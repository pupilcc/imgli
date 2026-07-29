package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/imgli/internal/config"
	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/stats"
	"github.com/yixian-huang/imgli/internal/service/storagesvc"
)

// newCDNServeEnv 构造带 CDNDomain 策略的 /i /t 路由与公开/私密图。
// file.Path 固定 2026/07/x.png；CDNDomain 由调用方传入(空串=无 CDN)。
func newCDNServeEnv(t *testing.T, cdnDomain, visibility string) (mux *chi.Mux, img *model.Image, owner *model.User, name string) {
	t.Helper()
	db := model.TestDB(t)
	cfg := &config.Config{DataDir: t.TempDir(), BaseURL: "http://img.li:8686"}

	owner = &model.User{Username: "cdnowner", Email: "cdn@img.li", GroupID: 1}
	if err := db.Create(owner).Error; err != nil {
		t.Fatal(err)
	}

	pol := &model.StoragePolicy{
		Name: "cdn-pol", Driver: "local",
		CDNDomain: cdnDomain,
		Config:    map[string]string{"root": "uploads", "prefix": ""},
		Enabled:   true,
	}
	if err := db.Create(pol).Error; err != nil {
		t.Fatal(err)
	}

	path := "2026/07/x.png"
	f := &model.File{
		Hash: "cdn302hash00001", StoragePolicyID: pol.ID, Path: path,
		Size: 4, MIME: "image/png", Width: 1, Height: 1, RefCount: 1,
	}
	if err := db.Create(f).Error; err != nil {
		t.Fatal(err)
	}
	img = &model.Image{
		Key: "cdn302key0001", UserID: &owner.ID, FileID: f.ID,
		Name: "x", Ext: "png", Visibility: visibility, Status: "normal",
	}
	if err := db.Create(img).Error; err != nil {
		t.Fatal(err)
	}

	st := stats.New(db, time.Hour)
	sh := &ServeHandlers{D: ServeDeps{
		DB: db, Res: storagesvc.New(cfg, db),
		Stats: st, OwnHost: "img.li",
	}}
	mux = chi.NewRouter()
	mux.Get("/i/{name}", sh.Original)
	mux.Get("/t/{name}", sh.Thumbnail)
	// 私密图属主访问：在同一 mux 上挂带 principal 的别名路径供测试用
	mux.With(withPrincipal(owner)).Get("/auth/i/{name}", sh.Original)
	return mux, img, owner, img.Key + ".png"
}

func get302(mux *chi.Mux, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestServe302PublicWithCDN(t *testing.T) {
	mux, _, _, name := newCDNServeEnv(t, "https://cdn.example", "public")
	rec := get302(mux, "/i/"+name)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d want 302 body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if loc != "https://cdn.example/2026/07/x.png" {
		t.Errorf("Location=%q want https://cdn.example/2026/07/x.png", loc)
	}
	cc := rec.Header().Get("Cache-Control")
	if !strings.Contains(cc, "max-age=300") {
		t.Errorf("Cache-Control=%q want contain max-age=300", cc)
	}
}

// TestServe302PrivateLocalNoRedirect local 驱动不实现 Presigner → 私密图恒流式。
// (原 TestServe302PrivateNoRedirect:裁决 8 后「私密图恒不 302」不再成立,
//  但「驱动不支持预签名时不 302」仍然成立,断言改指后者。)
func TestServe302PrivateLocalNoRedirect(t *testing.T) {
	mux, _, _, name := newCDNServeEnv(t, "https://cdn.example", "private")
	rec := get302(mux, "/auth/i/"+name)
	if rec.Code == http.StatusFound {
		t.Errorf("local 驱动不应 302, Location=%s", rec.Header().Get("Location"))
	}
}

// TestServe302PrivatePathNeverCDN S4：即使 visibility 被误写成 public，private/ 对象键
// 也不得 302 到 CDNDomain（ObjectURL fail-closed）。
func TestServe302PrivatePathNeverCDN(t *testing.T) {
	db := model.TestDB(t)
	cfg := &config.Config{DataDir: t.TempDir(), BaseURL: "http://img.li:8686"}
	owner := &model.User{Username: "s4owner", Email: "s4@img.li", GroupID: 1}
	if err := db.Create(owner).Error; err != nil {
		t.Fatal(err)
	}
	pol := &model.StoragePolicy{
		Name: "s4-cdn", Driver: "local",
		CDNDomain: "https://cdn.evil.example",
		Config:    map[string]string{"root": "uploads"},
		Enabled:   true,
	}
	if err := db.Create(pol).Error; err != nil {
		t.Fatal(err)
	}
	// 磁盘上不需要真实文件：若误 302 会先返回 Location；不 302 则流式失败可接受。
	f := &model.File{
		Hash: "s4privhash000001", StoragePolicyID: pol.ID,
		Path: "private/2026/07/secret.png", Surface: model.SurfacePrivate,
		Size: 4, MIME: "image/png", Width: 1, Height: 1, RefCount: 1,
	}
	if err := db.Create(f).Error; err != nil {
		t.Fatal(err)
	}
	// 模拟数据不一致：visibility=public 但 surface/path 为 private
	img := &model.Image{
		Key: "s4privkey0001", UserID: &owner.ID, FileID: f.ID,
		Name: "secret", Ext: "png", Visibility: "public", Status: "normal",
	}
	if err := db.Create(img).Error; err != nil {
		t.Fatal(err)
	}
	sh := &ServeHandlers{D: ServeDeps{
		DB: db, Res: storagesvc.New(cfg, db), OwnHost: "img.li",
	}}
	mux := chi.NewRouter()
	mux.Get("/i/{name}", sh.Original)
	name := img.Key + ".png"
	rec := get302(mux, "/i/"+name)
	if rec.Code == http.StatusFound {
		loc := rec.Header().Get("Location")
		if strings.Contains(loc, "cdn.evil") || strings.Contains(loc, "private/") {
			t.Fatalf("S4: 不得 302 到 CDN/private 键, Location=%s", loc)
		}
	}
}

func TestServe302PublicNoCDN(t *testing.T) {
	mux, _, _, name := newCDNServeEnv(t, "", "public")
	rec := get302(mux, "/i/"+name)
	if rec.Code == http.StatusFound {
		t.Errorf("无 CDN 不应 302, Location=%s", rec.Header().Get("Location"))
	}
}

func TestServeThumbnailNo302(t *testing.T) {
	mux, _, _, name := newCDNServeEnv(t, "https://cdn.example", "public")
	rec := get302(mux, "/t/"+name)
	if rec.Code == http.StatusFound {
		t.Errorf("/t 不应 302, Location=%s", rec.Header().Get("Location"))
	}
}

// fakeS3Object 假 S3 端点返回的固定内容。streamFile → driver.Open 会 HEAD 取
// Content-Length 再 Range GET;http.ServeContent + bytes.Reader 自动处理两者。
var fakeS3Object = []byte("fake-s3-object-body")

// newPresignServeEnv 构造 s3 驱动策略(可选 presign_domain)与私密图。
// PresignGet 是纯计算不发网络;回落 streamFile 会真实 Open/GET,故起本地
// httptest.Server 作假 S3 端点,避免打公网(endpoint 曾写死真实生产地址)。
func newPresignServeEnv(t *testing.T, presignDomain string) (mux *chi.Mux, owner *model.User, name string) {
	t.Helper()
	// 任意路径返回同一内容:原图键与缩略图键(.thumbs/...)都能命中。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(fakeS3Object))
	}))
	t.Cleanup(srv.Close)

	db := model.TestDB(t)
	cfg := &config.Config{DataDir: t.TempDir(), BaseURL: "http://img.li:8686"}

	owner = &model.User{Username: "psowner", Email: "ps@img.li", GroupID: 1}
	if err := db.Create(owner).Error; err != nil {
		t.Fatal(err)
	}
	pol := &model.StoragePolicy{
		Name: "ps-pol", Driver: "s3",
		// endpoint 带 http:// 前缀 → s3.New 走明文 scheme;srv.URL 形如 http://127.0.0.1:PORT
		Config: map[string]string{
			"endpoint": srv.URL, "region": "us-east-1", "bucket": "imgli",
			"access_key_id": "AK", "secret_access_key": "SK",
			"path_style": "true", "presign_domain": presignDomain,
		},
		Enabled: true,
	}
	if err := db.Create(pol).Error; err != nil {
		t.Fatal(err)
	}
	f := &model.File{
		Hash: "pshash000000001", StoragePolicyID: pol.ID, Path: "2026/07/p.png",
		Size: int64(len(fakeS3Object)), MIME: "image/png", Width: 1, Height: 1, RefCount: 1,
	}
	if err := db.Create(f).Error; err != nil {
		t.Fatal(err)
	}
	img := &model.Image{
		Key: "pskey00000001", UserID: &owner.ID, FileID: f.ID,
		Name: "p", Ext: "png", Visibility: "private", Status: "normal",
	}
	if err := db.Create(img).Error; err != nil {
		t.Fatal(err)
	}
	sh := &ServeHandlers{D: ServeDeps{
		DB: db, Res: storagesvc.New(cfg, db),
		Stats: stats.New(db, time.Hour), OwnHost: "img.li",
	}}
	mux = chi.NewRouter()
	mux.With(withPrincipal(owner)).Get("/auth/i/{name}", sh.Original)
	mux.With(withPrincipal(owner)).Get("/auth/t/{name}", sh.Thumbnail)
	return mux, owner, img.Key + ".png"
}

// TestServe302PrivatePresigned 私密图 + 驱动支持预签名 + 配了 presign_domain
// → 302 到签名 URL,且缓存头必须是 private,no-store。
func TestServe302PrivatePresigned(t *testing.T) {
	mux, _, name := newPresignServeEnv(t, "https://s3.img.li")
	rec := get302(mux, "/auth/i/"+name)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d want 302 body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://s3.img.li/imgli/2026/07/p.png?") {
		t.Errorf("Location=%q 未指向 presign_domain 上的对象", loc)
	}
	if !strings.Contains(loc, "X-Amz-Signature=") || !strings.Contains(loc, "X-Amz-Expires=60") {
		t.Errorf("Location 缺签名或 TTL: %q", loc)
	}
	// 签名 60s 后失效:绝不能沿用公开图的 public,max-age=300
	if cc := rec.Header().Get("Cache-Control"); cc != "private, no-store" {
		t.Errorf("Cache-Control=%q want private, no-store", cc)
	}
}

// TestServe302PrivateNoPresignDomain 未配 presign_domain → 回落流式成功(200+非空体)。
func TestServe302PrivateNoPresignDomain(t *testing.T) {
	mux, _, name := newPresignServeEnv(t, "")
	rec := get302(mux, "/auth/i/"+name)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200 body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() == 0 {
		t.Error("回落流式响应体为空")
	}
}

// TestServeThumbnailNoPresign 缩略图恒不预签名(裁决 4)→ 回落流式成功(200+非空体)。
func TestServeThumbnailNoPresign(t *testing.T) {
	mux, _, name := newPresignServeEnv(t, "https://s3.img.li")
	rec := get302(mux, "/auth/t/"+name)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200 body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() == 0 {
		t.Error("回落流式响应体为空")
	}
}
