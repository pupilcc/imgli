package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
)

// TestServeMaxViewsExhausted 匿名第 2 次 /i 在 max_views=1 时 410；/t 不消耗。
func TestServeMaxViewsExhausted(t *testing.T) {
	fx := newServeFixture(t)
	if err := fx.db.Model(fx.img).Updates(map[string]any{"max_views": 1, "views_served": 0}).Error; err != nil {
		t.Fatal(err)
	}

	// thumb 不消耗
	recT := fx.get("/t/"+fx.name, nil)
	if recT.Code != http.StatusOK {
		t.Fatalf("/t first: %d", recT.Code)
	}
	var img model.Image
	fx.db.First(&img, fx.img.ID)
	if img.ViewsServed != 0 {
		t.Fatalf("thumb should not claim views, got %d", img.ViewsServed)
	}

	// first /i OK and claims
	rec1 := fx.get("/i/"+fx.name, nil)
	if rec1.Code != http.StatusOK {
		t.Fatalf("/i first: %d body=%s", rec1.Code, rec1.Body.String())
	}
	fx.db.First(&img, fx.img.ID)
	if img.ViewsServed != 1 {
		t.Fatalf("views_served=%d want 1", img.ViewsServed)
	}

	// second /i exhausted
	rec2 := fx.get("/i/"+fx.name, nil)
	if rec2.Code != http.StatusGone {
		t.Fatalf("/i second: %d want 410 body=%s", rec2.Code, rec2.Body.String())
	}
}

// TestServeMaxViewsOwnerExempt 属主 principal 在 views 已满时仍可 /i 且不 claim。
func TestServeMaxViewsOwnerExempt(t *testing.T) {
	fx := newServeFixture(t)
	if err := fx.db.Model(fx.img).Updates(map[string]any{"max_views": 1, "views_served": 1}).Error; err != nil {
		t.Fatal(err)
	}
	// 无 principal → 410
	rec := fx.get("/i/"+fx.name, nil)
	if rec.Code != http.StatusGone {
		t.Fatalf("anon exhausted: %d", rec.Code)
	}
	// 属主 context
	var u model.User
	if err := fx.db.First(&u, *fx.img.UserID).Error; err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/i/"+fx.name, nil)
	req = req.WithContext(context.WithValue(req.Context(), principalKey{}, &Principal{User: &u, Scope: "full"}))
	recO := httptest.NewRecorder()
	fx.mux.ServeHTTP(recO, req)
	if recO.Code != http.StatusOK {
		t.Fatalf("owner exhausted still 200: %d body=%s", recO.Code, recO.Body.String())
	}
	var img model.Image
	fx.db.First(&img, fx.img.ID)
	if img.ViewsServed != 1 {
		t.Fatalf("owner must not claim, views_served=%d", img.ViewsServed)
	}
}
