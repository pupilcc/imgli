package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestDeleteAccountWrongPassword(t *testing.T) {
	db := model.TestDB(t)
	authSvc := auth.New(db, settings.New(db))
	// 首用户 admin，再注册一个普通用户
	if _, err := authSvc.Register("admin1", "admin1@img.li", "passw0rd1", ""); err != nil {
		t.Fatal(err)
	}
	u, err := authSvc.Register("user1", "user1@img.li", "passw0rd1", "")
	if err != nil {
		t.Fatal(err)
	}
	uh := &UserHandlers{
		Svc: authSvc, Img: imagesvc.New(db, storagesvc.New(&config.Config{DataDir: t.TempDir()}, db), nil), Adm: adminsvc.New(db),
		AvatarDir: t.TempDir(),
	}
	r := chi.NewRouter()
	r.With(withPrincipal(u)).Delete("/api/v1/user", uh.DeleteAccount)

	req := httptest.NewRequest("DELETE", "/api/v1/user", strings.NewReader(`{"password":"wrong"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid_credentials") {
		t.Errorf("body 应含 invalid_credentials: %s", rec.Body.String())
	}
	var n int64
	db.Model(&model.User{}).Where("id = ?", u.ID).Count(&n)
	if n != 1 {
		t.Errorf("错误密码后用户应仍在, got %d", n)
	}
}

func TestDeleteAccountAdminForbidden(t *testing.T) {
	db := model.TestDB(t)
	authSvc := auth.New(db, settings.New(db))
	u, err := authSvc.Register("adminonly", "adminonly@img.li", "passw0rd1", "")
	if err != nil {
		t.Fatal(err)
	}
	if !u.IsAdmin {
		t.Fatal("首用户应为 admin")
	}
	uh := &UserHandlers{
		Svc: authSvc, Img: imagesvc.New(db, storagesvc.New(&config.Config{DataDir: t.TempDir()}, db), nil), Adm: adminsvc.New(db),
		AvatarDir: t.TempDir(),
	}
	r := chi.NewRouter()
	r.With(withPrincipal(u)).Delete("/api/v1/user", uh.DeleteAccount)

	req := httptest.NewRequest("DELETE", "/api/v1/user", strings.NewReader(`{"password":"passw0rd1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "admin_cannot_self_delete") {
		t.Errorf("body 应含 admin_cannot_self_delete: %s", rec.Body.String())
	}
}

func TestDeleteAccountOK(t *testing.T) {
	db := model.TestDB(t)
	authSvc := auth.New(db, settings.New(db))
	if _, err := authSvc.Register("admin2", "admin2@img.li", "passw0rd1", ""); err != nil {
		t.Fatal(err)
	}
	u, err := authSvc.Register("goneuser", "gone@img.li", "passw0rd1", "")
	if err != nil {
		t.Fatal(err)
	}
	if u.IsAdmin {
		t.Fatal("第二用户不应为 admin")
	}
	uh := &UserHandlers{
		Svc: authSvc, Img: imagesvc.New(db, storagesvc.New(&config.Config{DataDir: t.TempDir()}, db), nil), Adm: adminsvc.New(db),
		AvatarDir: t.TempDir(),
	}
	r := chi.NewRouter()
	r.With(withPrincipal(u)).Delete("/api/v1/user", uh.DeleteAccount)

	req := httptest.NewRequest("DELETE", "/api/v1/user", strings.NewReader(`{"password":"passw0rd1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var n int64
	db.Model(&model.User{}).Where("id = ?", u.ID).Count(&n)
	if n != 0 {
		t.Errorf("用户行应删除")
	}

	var log model.AuditLog
	if err := db.Where("action = ?", "user_self_delete").First(&log).Error; err != nil {
		t.Fatalf("应有 user_self_delete 审计: %v", err)
	}
	if !strings.Contains(log.Detail, "goneuser") {
		t.Errorf("audit detail 应含 username, got %s", log.Detail)
	}

	var foundClear bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookie {
			foundClear = true
			if c.MaxAge >= 0 && c.Value != "" {
				t.Errorf("cookie 应为清除态 MaxAge=%d Value=%q", c.MaxAge, c.Value)
			}
		}
	}
	if !foundClear {
		sc := rec.Header().Get("Set-Cookie")
		if !strings.Contains(sc, SessionCookie) {
			t.Error("响应应清除 imgli_session cookie")
		}
	}

	// 同用户名可再注册
	if _, err := authSvc.Register("goneuser", "gone2@img.li", "passw0rd1", ""); err != nil {
		t.Errorf("注销后同名再注册应成功: %v", err)
	}
}
