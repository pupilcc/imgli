package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/imgli/internal/config"
	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/imagesvc"
	"github.com/yixian-huang/imgli/internal/service/storagesvc"
)

// TestPatchExpiresIn tri-state：设 / 清除(0) / 不传不改。
func TestPatchExpiresIn(t *testing.T) {
	db := model.TestDB(t)
	cfg := &config.Config{DataDir: t.TempDir(), BaseURL: "https://img.li"}
	u := &model.User{Username: "px", Email: "px@img.li", GroupID: 1}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	res := storagesvc.New(cfg, db)
	// 该图后面会被 PATCH 到 private，触发 surface 重挂(S2)：File 需带 Surface/前缀路径，
	// 且源对象须真实存在，否则 resolveFileForSurface 复制时 Open 会失败。
	f := &model.File{Hash: "pxh", Surface: model.SurfacePublic, StoragePolicyID: 1,
		Path: "public/p/px", Size: 10, MIME: "image/png", Width: 1, Height: 1, RefCount: 1}
	db.Create(f)
	var pol model.StoragePolicy
	db.First(&pol, 1)
	if d, err := res.Driver(&pol); err == nil {
		_ = d.Put(context.Background(), f.Path, bytes.NewReader([]byte("x")))
	}
	img := &model.Image{
		Key: "pxexpkey00001", UserID: &u.ID, FileID: f.ID,
		Name: "px", Ext: "png", Visibility: "public", Status: "normal",
	}
	db.Create(img)
	h := &ImageHandlers{D: ImageDeps{
		Img: imagesvc.New(db, res, nil),
		Res: res,
	}}
	mux := chi.NewRouter()
	mux.With(withPrincipal(u)).Patch("/api/v1/images/{key}", h.Update)

	patch := func(body string) map[string]any {
		t.Helper()
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/images/"+img.Key, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH status=%d body=%s", rec.Code, rec.Body.String())
		}
		var env struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		return env.Data
	}

	// 设 expires_in=86400
	before := time.Now()
	data := patch(`{"expires_in":86400}`)
	got, _ := data["expires_at"].(string)
	if got == "" {
		t.Fatal("设过期后 expires_at 应有值")
	}
	parsed, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("expires_at 非法: %v", err)
	}
	// 约 now+1d（±2min 容差）
	want := before.Add(24 * time.Hour)
	if d := parsed.Sub(want); d < -2*time.Minute || d > 2*time.Minute {
		t.Errorf("expires_at=%v want≈%v", parsed, want)
	}

	// 不传 expires_in：不改（改 name 验证端点仍通）
	data = patch(`{"name":"still-exp"}`)
	if data["name"] != "still-exp" {
		t.Errorf("name 应更新, got %v", data["name"])
	}
	if data["expires_at"] != got {
		t.Errorf("不传 expires_in 不应改 expires_at: got %v want %v", data["expires_at"], got)
	}

	// expires_in=0：清除
	data = patch(`{"expires_in":0}`)
	if data["expires_at"] != nil {
		t.Errorf("expires_in=0 应清除 expires_at, got %v", data["expires_at"])
	}
	var re model.Image
	if err := db.Where("key = ?", img.Key).First(&re).Error; err != nil {
		t.Fatal(err)
	}
	if re.ExpiresAt != nil {
		t.Fatalf("DB expires_at 应为 NULL, got %v", re.ExpiresAt)
	}

	// 再设一次，然后不传字段确认仍保持
	data = patch(`{"expires_in":3600}`)
	if data["expires_at"] == nil {
		t.Fatal("再设后 expires_at 应有值")
	}
	kept := data["expires_at"]
	data = patch(`{"visibility":"private"}`)
	if data["expires_at"] != kept {
		t.Errorf("不传 expires_in 且改 visibility 不应清过期: got %v want %v", data["expires_at"], kept)
	}
}

// TestPatchExpiresInOverflowRejected expires_in 超上限(防 time.Duration 溢出)→ 400（codex 评审 F2）。
func TestPatchExpiresInOverflowRejected(t *testing.T) {
	db := model.TestDB(t)
	cfg := &config.Config{DataDir: t.TempDir(), BaseURL: "https://img.li"}
	u := &model.User{Username: "ov", Email: "ov@img.li", GroupID: 1}
	db.Create(u)
	f := &model.File{Hash: "ovh", StoragePolicyID: 1, Path: "p/ov", Size: 10, MIME: "image/png", Width: 1, Height: 1, RefCount: 1}
	db.Create(f)
	img := &model.Image{Key: "ovexpkey00001", UserID: &u.ID, FileID: f.ID, Name: "ov", Ext: "png", Visibility: "public", Status: "normal"}
	db.Create(img)
	res := storagesvc.New(cfg, db)
	h := &ImageHandlers{D: ImageDeps{Img: imagesvc.New(db, res, nil), Res: res}}
	mux := chi.NewRouter()
	mux.With(withPrincipal(u)).Patch("/api/v1/images/{key}", h.Update)

	// 超上限(1 年 +1 秒)与巨值均应 400,且不改 expires_at
	for _, sec := range []int64{int64(MaxExpiresInSec) + 1, 1 << 40} {
		body := `{"expires_in":` + strconv.FormatInt(sec, 10) + `}`
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/images/"+img.Key, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expires_in=%d status=%d want 400 body=%s", sec, rec.Code, rec.Body.String())
		}
	}
	var re model.Image
	db.Where("key = ?", img.Key).First(&re)
	if re.ExpiresAt != nil {
		t.Errorf("超上限请求不应设 expires_at, got %v", re.ExpiresAt)
	}
}
