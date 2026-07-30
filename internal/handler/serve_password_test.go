package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/auth"
)

func TestServeAccessPasswordRequired(t *testing.T) {
	fx := newServeFixture(t)
	h, err := auth.HashPassword("s3cret")
	if err != nil {
		t.Fatal(err)
	}
	if err := fx.db.Model(fx.img).Update("access_password_hash", h).Error; err != nil {
		t.Fatal(err)
	}

	// no credential → 401
	rec := fx.get("/i/"+fx.name, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anon no pw: %d want 401 body=%s", rec.Code, rec.Body.String())
	}

	// wrong header → 401
	recW := fx.get("/i/"+fx.name, map[string]string{imgPassHeader: "wrong"})
	if recW.Code != http.StatusUnauthorized {
		t.Fatalf("wrong pw: %d", recW.Code)
	}

	// correct header → 200
	recOK := fx.get("/i/"+fx.name, map[string]string{imgPassHeader: "s3cret"})
	if recOK.Code != http.StatusOK {
		t.Fatalf("good pw: %d body=%s", recOK.Code, recOK.Body.String())
	}

	// owner exempt without password
	var u model.User
	if err := fx.db.First(&u, *fx.img.UserID).Error; err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/i/"+fx.name, nil)
	req = req.WithContext(context.WithValue(req.Context(), principalKey{}, &Principal{User: &u, Scope: "full"}))
	recO := httptest.NewRecorder()
	fx.mux.ServeHTTP(recO, req)
	if recO.Code != http.StatusOK {
		t.Fatalf("owner: %d", recO.Code)
	}
}

func TestServeAccessPasswordCookie(t *testing.T) {
	fx := newServeFixture(t)
	h, err := auth.HashPassword("cook1e")
	if err != nil {
		t.Fatal(err)
	}
	if err := fx.db.Model(fx.img).Update("access_password_hash", h).Error; err != nil {
		t.Fatal(err)
	}
	tok := imgPassToken(h, fx.img.Key)
	req := httptest.NewRequest(http.MethodGet, "/i/"+fx.name, nil)
	req.AddCookie(&http.Cookie{Name: imgPassCookieName(fx.img.Key), Value: tok})
	rec := httptest.NewRecorder()
	fx.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("cookie unlock: %d body=%s", rec.Code, rec.Body.String())
	}
}
