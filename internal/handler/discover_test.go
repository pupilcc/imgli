package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/service/settings"
)

func discoverMux(h *DiscoverHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/api/v1/plaza", h.Plaza)
	r.Get("/api/v1/u/{username}", h.UserProfile)
	r.Get("/api/v1/u/{username}/images", h.UserImages)
	return r
}

func doGET(mux http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func decodeEnv(t *testing.T, rec *httptest.ResponseRecorder) (status bool, code int, data json.RawMessage, msg string) {
	t.Helper()
	var e struct {
		Status  bool            `json:"status"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("decode envelope: %v body=%s", err, rec.Body.String())
	}
	return e.Status, rec.Code, e.Data, e.Message
}

func TestDiscover_Disabled404(t *testing.T) {
	db := model.TestDB(t)
	// Seed 默认 plaza_enabled=false
	h := &DiscoverHandler{DB: db}
	mux := discoverMux(h)

	for _, path := range []string{
		"/api/v1/plaza",
		"/api/v1/u/anyone",
		"/api/v1/u/anyone/images",
	} {
		rec := doGET(mux, path)
		ok, code, _, _ := decodeEnv(t, rec)
		if code != 404 || ok {
			t.Errorf("%s: want 404 status=false, got code=%d status=%v body=%s",
				path, code, ok, rec.Body.String())
		}
	}
}

func TestDiscover_PlazaAndProfile(t *testing.T) {
	db := model.TestDB(t)
	if err := settings.New(db).Set(model.SettingPlazaEnabled, true); err != nil {
		t.Fatal(err)
	}

	u := model.User{
		Username: "pubuser", Email: "pubuser@test.img.li", GroupID: 1,
		Status: "active", PublicProfile: true, Nickname: "Pub",
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	f := model.File{
		Hash: "dhash1", StoragePolicyID: 1, Path: "p/d1", Size: 10,
		MIME: "image/png", Width: 1, Height: 1, RefCount: 1,
	}
	if err := db.Create(&f).Error; err != nil {
		t.Fatal(err)
	}
	img := model.Image{
		Key: "plazaKey0001", UserID: &u.ID, FileID: f.ID,
		Name: "shot", Ext: "png", Visibility: "public", Status: "normal",
	}
	if err := db.Create(&img).Error; err != nil {
		t.Fatal(err)
	}

	// hidden profile user
	hid := model.User{
		Username: "hiddenu", Email: "hiddenu@test.img.li", GroupID: 1,
		Status: "active", PublicProfile: false,
	}
	if err := db.Create(&hid).Error; err != nil {
		t.Fatal(err)
	}

	h := &DiscoverHandler{DB: db}
	mux := discoverMux(h)

	// plaza
	rec := doGET(mux, "/api/v1/plaza")
	ok, code, data, _ := decodeEnv(t, rec)
	if code != 200 || !ok {
		t.Fatalf("plaza: code=%d ok=%v body=%s", code, ok, rec.Body.String())
	}
	var plaza struct {
		Items []struct {
			Key string `json:"key"`
		} `json:"items"`
		NextCursor string `json:"next_cursor"`
	}
	if err := json.Unmarshal(data, &plaza); err != nil {
		t.Fatal(err)
	}
	if len(plaza.Items) != 1 || plaza.Items[0].Key != "plazaKey0001" {
		t.Fatalf("plaza items=%+v want 1 with plazaKey0001", plaza.Items)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=30" {
		t.Errorf("Cache-Control=%q", cc)
	}

	// profile ok
	rec = doGET(mux, "/api/v1/u/pubuser")
	ok, code, data, msg := decodeEnv(t, rec)
	if code != 200 || !ok {
		t.Fatalf("profile: code=%d ok=%v msg=%s", code, ok, msg)
	}
	var prof struct {
		User struct {
			Username         string `json:"username"`
			PublicImageCount int64  `json:"public_image_count"`
		} `json:"user"`
	}
	if err := json.Unmarshal(data, &prof); err != nil {
		t.Fatal(err)
	}
	if prof.User.Username != "pubuser" || prof.User.PublicImageCount != 1 {
		t.Errorf("profile=%+v", prof.User)
	}

	// ghost / hidden → 404 same message
	for _, name := range []string{"ghost", "hiddenu"} {
		rec = doGET(mux, "/api/v1/u/"+name)
		ok, code, _, msg = decodeEnv(t, rec)
		if code != 404 || ok {
			t.Errorf("u/%s: want 404, got %d ok=%v", name, code, ok)
		}
		if msg != "主页不存在或未公开" {
			t.Errorf("u/%s message=%q", name, msg)
		}
	}

	// user images
	rec = doGET(mux, "/api/v1/u/pubuser/images")
	ok, code, data, _ = decodeEnv(t, rec)
	if code != 200 || !ok {
		t.Fatalf("user images: code=%d body=%s", code, rec.Body.String())
	}
	var imgs struct {
		Items []struct {
			Key string `json:"key"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &imgs); err != nil {
		t.Fatal(err)
	}
	if len(imgs.Items) != 1 || imgs.Items[0].Key != "plazaKey0001" {
		t.Fatalf("user images=%+v", imgs.Items)
	}
}

func TestDiscover_LimitClampAndBadCursor(t *testing.T) {
	db := model.TestDB(t)
	if err := settings.New(db).Set(model.SettingPlazaEnabled, true); err != nil {
		t.Fatal(err)
	}
	u := model.User{
		Username: "limu", Email: "limu@test.img.li", GroupID: 1,
		Status: "active", PublicProfile: true,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	// 61 张合规图，验证 limit=999 被钳到 60
	for i := 0; i < 61; i++ {
		key := fmt.Sprintf("lim%04d", i)
		f := model.File{
			Hash: key + "h", StoragePolicyID: 1, Path: "p/" + key, Size: 1,
			MIME: "image/png", RefCount: 1,
		}
		if err := db.Create(&f).Error; err != nil {
			t.Fatal(err)
		}
		img := model.Image{
			Key: key, UserID: &u.ID, FileID: f.ID, Name: key, Ext: "png",
			Visibility: "public", Status: "normal",
		}
		if err := db.Create(&img).Error; err != nil {
			t.Fatal(err)
		}
	}

	h := &DiscoverHandler{DB: db}
	mux := discoverMux(h)

	rec := doGET(mux, "/api/v1/plaza?limit=999")
	ok, code, data, _ := decodeEnv(t, rec)
	if code != 200 || !ok {
		t.Fatalf("limit clamp: code=%d body=%s", code, rec.Body.String())
	}
	var plaza struct {
		Items      []json.RawMessage `json:"items"`
		NextCursor string            `json:"next_cursor"`
	}
	if err := json.Unmarshal(data, &plaza); err != nil {
		t.Fatal(err)
	}
	if len(plaza.Items) > 60 {
		t.Fatalf("items len=%d want ≤60", len(plaza.Items))
	}
	if len(plaza.Items) != 60 {
		t.Errorf("items len=%d want 60 (clamped)", len(plaza.Items))
	}
	if plaza.NextCursor == "" {
		t.Error("61 images with limit 60 should yield next_cursor")
	}

	rec = doGET(mux, "/api/v1/plaza?cursor=@@@")
	ok, code, _, msg := decodeEnv(t, rec)
	if code != 400 || ok {
		t.Fatalf("bad cursor: code=%d ok=%v body=%s", code, ok, rec.Body.String())
	}
	if msg != "游标格式错误" {
		t.Errorf("message=%q", msg)
	}
}
