package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/model"
)

// TestServeExpiredImageGone 造 expires_at 过去的图 → /i 与 /t 返 410。
func TestServeExpiredImageGone(t *testing.T) {
	fx := newServeFixture(t)
	past := time.Now().Add(-time.Hour)
	if err := fx.db.Model(fx.img).Update("expires_at", past).Error; err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/i/" + fx.name, "/t/" + fx.name} {
		// Accept JSON → 410 信封(CodeGone,与移除图共用 placeholder 的 gone 语义)
		rec := fx.get(path, map[string]string{"Accept": "application/json"})
		if rec.Code != http.StatusGone {
			t.Errorf("%s: status=%d want 410 body=%s", path, rec.Code, rec.Body.String())
			continue
		}
		var env struct {
			Message string `json:"message"`
			Data    struct {
				Code string `json:"code"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Errorf("%s: decode: %v", path, err)
			continue
		}
		if env.Data.Code != CodeGone {
			t.Errorf("%s: code=%q want %q", path, env.Data.Code, CodeGone)
		}
		// 浏览器/热链(无 Accept JSON)→ 410 + SVG 占位图(IMAGE EXPIRED),非破图
		rec2 := fx.get(path, nil)
		if rec2.Code != http.StatusGone {
			t.Errorf("%s: img status=%d want 410", path, rec2.Code)
		}
		if ct := rec2.Header().Get("Content-Type"); !strings.Contains(ct, "image/svg") {
			t.Errorf("%s: content-type=%q want svg placeholder", path, ct)
		}
		if !strings.Contains(rec2.Body.String(), "IMAGE EXPIRED") {
			t.Errorf("%s: placeholder 未含 IMAGE EXPIRED", path)
		}
	}
}

// TestServeFutureAndNilExpiresOK expires_at 未来或 nil → 200。
func TestServeFutureAndNilExpiresOK(t *testing.T) {
	fx := newServeFixture(t)

	// nil（默认）→ 200
	rec := fx.get("/i/"+fx.name, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("nil expires: status=%d want 200 body=%s", rec.Code, rec.Body.String())
	}

	future := time.Now().Add(24 * time.Hour)
	if err := fx.db.Model(fx.img).Update("expires_at", future).Error; err != nil {
		t.Fatal(err)
	}
	// 刷新后重新读路由仍用同一 key
	rec = fx.get("/i/"+fx.name, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("future expires: status=%d want 200 body=%s", rec.Code, rec.Body.String())
	}
	rec = fx.get("/t/"+fx.name, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("future /t: status=%d want 200 body=%s", rec.Code, rec.Body.String())
	}
}

// TestServeExpiredDoesNotAffectUnrelated 确认过期检查不破坏其它门禁前的非过期路径。
func TestServeExpiredDoesNotAffectUnrelated(t *testing.T) {
	fx := newServeFixture(t)
	// 另建一张未过期图直接 200
	f := &model.File{
		Hash: "unrelatedhash01", StoragePolicyID: 1, Path: "2024/01/01/unrelated.png",
		Size: 4, MIME: "image/png", Width: 1, Height: 1, RefCount: 1,
	}
	if err := fx.db.Create(f).Error; err != nil {
		t.Fatal(err)
	}
	img := &model.Image{
		Key: "unrelatedkey01", UserID: fx.img.UserID, FileID: f.ID,
		Name: "u", Ext: "png", Visibility: "public", Status: "normal",
	}
	if err := fx.db.Create(img).Error; err != nil {
		t.Fatal(err)
	}
	// 物理文件复用 fixture 已有目录——不落盘则 stream 404，但 lookup 应通过。
	// 用 Accept:json 看门禁：若 404 说明过了 expiry/visibility；若 410 说明误伤。
	// 更稳：只断言不是 410 resource_gone 过期文案——实际上没文件会 404。
	rec := fx.get("/i/"+img.Key+".png", map[string]string{"Accept": "application/json"})
	if rec.Code == http.StatusGone {
		var env struct {
			Message string `json:"message"`
		}
		json.Unmarshal(rec.Body.Bytes(), &env)
		if env.Message == "图片已过期" {
			t.Fatalf("未过期图不应 410 过期: %s", rec.Body.String())
		}
	}
}
