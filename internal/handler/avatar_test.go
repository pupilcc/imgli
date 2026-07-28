package handler

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/auth"
	"github.com/yixian-huang/imgli/internal/service/settings"
)

func encodeTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func multipartFileBody(t *testing.T, field, filename string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, mw.FormDataContentType()
}

func withPrincipal(u *model.User) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), principalKey{}, &Principal{User: u, Scope: "full"}))
			next.ServeHTTP(w, r)
		})
	}
}

func TestAvatarUploadServeDelete(t *testing.T) {
	db := model.TestDB(t)
	svc := auth.New(db, settings.New(db))
	u, err := svc.Register("avataruser", "avatar@img.li", "passw0rd1", "")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	uh := &UserHandlers{Svc: svc, AvatarDir: dir}

	r := chi.NewRouter()
	r.With(withPrincipal(u)).Post("/api/v1/user/avatar", uh.UploadAvatar)
	r.With(withPrincipal(u)).Delete("/api/v1/user/avatar", uh.DeleteAvatar)
	r.Get("/avatar/{id}", ServeAvatar(dir))

	// POST 300×200 PNG
	body, ctype := multipartFileBody(t, "file", "face.png", encodeTestPNG(t, 300, 200))
	req := httptest.NewRequest("POST", "/api/v1/user/avatar", body)
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
	wantPath := fmt.Sprintf("avatars/%d.jpg", u.ID)
	if fresh.AvatarPath != wantPath {
		t.Errorf("avatar_path = %q, want %q", fresh.AvatarPath, wantPath)
	}
	jpgPath := filepath.Join(dir, fmt.Sprintf("%d.jpg", u.ID))
	if _, err := os.Stat(jpgPath); err != nil {
		t.Fatalf("头像文件应存在: %v", err)
	}

	// ServeAvatar
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", fmt.Sprintf("/avatar/%d", u.ID), nil))
	if rec.Code != 200 {
		t.Fatalf("serve status = %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "image/jpeg") {
		t.Errorf("Content-Type = %q, want image/jpeg", ct)
	}
	cc := rec.Header().Get("Cache-Control")
	if !strings.Contains(cc, "max-age=86400") {
		t.Errorf("Cache-Control = %q, want max-age=86400", cc)
	}

	// DELETE
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("DELETE", "/api/v1/user/avatar", nil))
	if rec.Code != 200 {
		t.Fatalf("delete status = %d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(jpgPath); !os.IsNotExist(err) {
		t.Errorf("删除后文件应不存在, err=%v", err)
	}
	if err := db.First(&fresh, u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.AvatarPath != "" {
		t.Errorf("删除后 avatar_path 应为空, got %q", fresh.AvatarPath)
	}
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", fmt.Sprintf("/avatar/%d", u.ID), nil))
	if rec.Code != 404 {
		t.Errorf("删除后 serve 应 404, got %d", rec.Code)
	}
}

func TestAvatarBadFile(t *testing.T) {
	db := model.TestDB(t)
	svc := auth.New(db, settings.New(db))
	u, err := svc.Register("badavatar", "badavatar@img.li", "passw0rd1", "")
	if err != nil {
		t.Fatal(err)
	}
	uh := &UserHandlers{Svc: svc, AvatarDir: t.TempDir()}
	r := chi.NewRouter()
	r.With(withPrincipal(u)).Post("/api/v1/user/avatar", uh.UploadAvatar)

	body, ctype := multipartFileBody(t, "file", "x.bin", []byte("not an image"))
	req := httptest.NewRequest("POST", "/api/v1/user/avatar", body)
	req.Header.Set("Content-Type", ctype)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("非图片应 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}
