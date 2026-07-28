package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/imgli/internal/config"
	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/adminsvc"
	"github.com/yixian-huang/imgli/internal/service/auth"
	"github.com/yixian-huang/imgli/internal/service/imagesvc"
	"github.com/yixian-huang/imgli/internal/service/settings"
	"github.com/yixian-huang/imgli/internal/service/storagesvc"
)

func encodeTestJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.White)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestWatermarkUploadDelete(t *testing.T) {
	db := model.TestDB(t)
	svc := auth.New(db, settings.New(db))
	u, err := svc.Register("wmuser", "wmuser@img.li", "passw0rd1", "")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	uh := &UserHandlers{Svc: svc, WatermarkDir: dir}

	r := chi.NewRouter()
	r.With(withPrincipal(u)).Post("/api/v1/user/watermark", uh.UploadWatermark)
	r.With(withPrincipal(u)).Delete("/api/v1/user/watermark", uh.DeleteWatermark)

	pngBytes := encodeTestPNG(t, 300, 200)
	body, ctype := multipartFileBody(t, "file", "mark.png", pngBytes)
	req := httptest.NewRequest("POST", "/api/v1/user/watermark", body)
	req.Header.Set("Content-Type", ctype)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("upload status = %d body=%s", rec.Code, rec.Body.String())
	}

	var fresh model.User
	if err := db.First(&fresh, u.ID).Error; err != nil {
		t.Fatal(err)
	}
	wantPath := fmt.Sprintf("watermarks/%d.png", u.ID)
	if fresh.WatermarkPath != wantPath {
		t.Errorf("watermark_path = %q, want %q", fresh.WatermarkPath, wantPath)
	}
	pngPath := filepath.Join(dir, fmt.Sprintf("%d.png", u.ID))
	stored, err := os.ReadFile(pngPath)
	if err != nil {
		t.Fatalf("水印文件应存在: %v", err)
	}
	if !bytes.Equal(stored, pngBytes) {
		t.Error("存储字节应与上传原样一致(未重编码)")
	}
	dto := userDTO(&fresh)
	if dto["watermark_set"] != true {
		t.Errorf("watermark_set = %v, want true", dto["watermark_set"])
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("DELETE", "/api/v1/user/watermark", nil))
	if rec.Code != 200 {
		t.Fatalf("delete status = %d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(pngPath); !os.IsNotExist(err) {
		t.Errorf("删除后文件应不存在, err=%v", err)
	}
	if err := db.First(&fresh, u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.WatermarkPath != "" {
		t.Errorf("删除后 watermark_path 应为空, got %q", fresh.WatermarkPath)
	}
	if userDTO(&fresh)["watermark_set"] != false {
		t.Errorf("watermark_set = %v, want false", userDTO(&fresh)["watermark_set"])
	}

	// 再 DELETE 幂等 200
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("DELETE", "/api/v1/user/watermark", nil))
	if rec.Code != 200 {
		t.Fatalf("idempotent delete status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWatermarkRejects(t *testing.T) {
	db := model.TestDB(t)
	svc := auth.New(db, settings.New(db))
	u, err := svc.Register("wmrej", "wmrej@img.li", "passw0rd1", "")
	if err != nil {
		t.Fatal(err)
	}
	uh := &UserHandlers{Svc: svc, WatermarkDir: t.TempDir()}
	r := chi.NewRouter()
	r.With(withPrincipal(u)).Post("/api/v1/user/watermark", uh.UploadWatermark)

	// JPEG → 400
	body, ctype := multipartFileBody(t, "file", "x.jpg", encodeTestJPEG(t, 100, 100))
	req := httptest.NewRequest("POST", "/api/v1/user/watermark", body)
	req.Header.Set("Content-Type", ctype)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("JPEG 应 400, got %d body=%s", rec.Code, rec.Body.String())
	}

	// 2049×10 PNG → 400
	body, ctype = multipartFileBody(t, "file", "big.png", encodeTestPNG(t, 2049, 10))
	req = httptest.NewRequest("POST", "/api/v1/user/watermark", body)
	req.Header.Set("Content-Type", ctype)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("2049×10 PNG 应 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("水印图需为 PNG")) {
		// 消息可为完整句;至少表明尺寸/格式拒绝
		var env struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &env)
		if env.Error.Message == "" {
			t.Logf("reject body=%s", rec.Body.String())
		}
	}
}

func TestDeleteAccountRemovesWatermark(t *testing.T) {
	db := model.TestDB(t)
	authSvc := auth.New(db, settings.New(db))
	if _, err := authSvc.Register("adminwm", "adminwm@img.li", "passw0rd1", ""); err != nil {
		t.Fatal(err)
	}
	u, err := authSvc.Register("gonewm", "gonewm@img.li", "passw0rd1", "")
	if err != nil {
		t.Fatal(err)
	}
	wmDir := t.TempDir()
	wmPath := filepath.Join(wmDir, fmt.Sprintf("%d.png", u.ID))
	if err := os.WriteFile(wmPath, encodeTestPNG(t, 50, 50), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := authSvc.SetWatermarkPath(u.ID, fmt.Sprintf("watermarks/%d.png", u.ID)); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{DataDir: t.TempDir()}
	uh := &UserHandlers{
		Svc: authSvc, Img: imagesvc.New(db, storagesvc.New(cfg, db), nil), Adm: adminsvc.New(db),
		AvatarDir: t.TempDir(), WatermarkDir: wmDir,
	}
	r := chi.NewRouter()
	r.With(withPrincipal(u)).Delete("/api/v1/user", uh.DeleteAccount)

	req := httptest.NewRequest("DELETE", "/api/v1/user", bytes.NewReader([]byte(`{"password":"passw0rd1"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(wmPath); !os.IsNotExist(err) {
		t.Errorf("注销后水印文件应消失, err=%v", err)
	}
}
