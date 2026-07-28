package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
)

func TestRequireAdmin(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	run := func(u *model.User) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/admin/stats", nil)
		req = req.WithContext(context.WithValue(req.Context(), principalKey{}, &Principal{User: u}))
		RequireAdmin(next).ServeHTTP(rec, req)
		return rec
	}
	if rec := run(&model.User{IsAdmin: true}); rec.Code != 200 {
		t.Errorf("admin = %d, want 200", rec.Code)
	}
	if rec := run(&model.User{IsAdmin: false}); rec.Code != http.StatusForbidden {
		t.Errorf("non-admin = %d, want 403", rec.Code)
	}
}
