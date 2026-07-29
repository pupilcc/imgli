package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// uploadN 用会话 cookie 传 n 张不同内容的图，返回其 key（按上传序）。
// 关键：用会话上传（上传路由 RequireUser 接受会话；OriginCheck 允许缺省 Origin），
// 使图片归属与后续列表/管理所用的同一 sess 用户一致——不能用 uploadToken(它内部会另注册用户)。
func uploadN(t *testing.T, s *Server, sess *http.Cookie, n int) []string {
	t.Helper()
	keys := make([]string, 0, n)
	for i := 0; i < n; i++ {
		req, _ := uploadReq(t, "file", "img.png", pngBytes(20+i, 20+i)) // 不同尺寸→不同内容→不去重
		req.AddCookie(sess)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("upload %d: %d %s", i, rec.Code, rec.Body.String())
		}
		var e env
		json.Unmarshal(rec.Body.Bytes(), &e)
		var d struct {
			Key string `json:"key"`
		}
		json.Unmarshal(e.Data, &d)
		keys = append(keys, d.Key)
	}
	return keys
}

func TestImagesListReturnsOwnedWithLinks(t *testing.T) {
	s := newUploadTestServer(t)
	sess := register(t, s)
	uploadN(t, s, sess, 3)
	rec, e := doJSON(t, s, "GET", "/api/v1/images", "", []*http.Cookie{sess})
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	var d struct {
		Items []struct {
			Key   string `json:"key"`
			Links struct {
				URL          string `json:"url"`
				ThumbnailURL string `json:"thumbnail_url"`
			} `json:"links"`
		} `json:"items"`
		NextCursor string `json:"next_cursor"`
	}
	json.Unmarshal(e.Data, &d)
	if len(d.Items) != 3 {
		t.Fatalf("应列出 3 张自有图, got %d", len(d.Items))
	}
	if d.Items[0].Links.URL == "" || d.Items[0].Links.ThumbnailURL == "" {
		t.Errorf("列表项应含 links: %s", rec.Body.String())
	}
}

func TestImagesListPaginates(t *testing.T) {
	s := newUploadTestServer(t)
	sess := register(t, s)
	uploadN(t, s, sess, 3)
	rec, e := doJSON(t, s, "GET", "/api/v1/images?limit=2", "", []*http.Cookie{sess})
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	var d struct {
		Items      []json.RawMessage `json:"items"`
		NextCursor string            `json:"next_cursor"`
	}
	json.Unmarshal(e.Data, &d)
	if len(d.Items) != 2 || d.NextCursor == "" {
		t.Fatalf("首页应 2 项 + next, got %d items next=%q", len(d.Items), d.NextCursor)
	}
}

func TestImagesListPaginatesAcrossPagesHTTP(t *testing.T) {
	s := newUploadTestServer(t)
	sess := register(t, s)
	uploadN(t, s, sess, 3)

	type page struct {
		Items []struct {
			Key string `json:"key"`
		} `json:"items"`
		NextCursor string `json:"next_cursor"`
	}

	rec, e := doJSON(t, s, "GET", "/api/v1/images?limit=1", "", []*http.Cookie{sess})
	if rec.Code != http.StatusOK {
		t.Fatalf("page1: %d %s", rec.Code, rec.Body.String())
	}
	var p1 page
	json.Unmarshal(e.Data, &p1)
	if len(p1.Items) != 1 || p1.NextCursor == "" {
		t.Fatalf("首页应 1 项 + next, got %d items next=%q", len(p1.Items), p1.NextCursor)
	}

	rec2, e2 := doJSON(t, s, "GET", "/api/v1/images?limit=1&cursor="+p1.NextCursor, "", []*http.Cookie{sess})
	if rec2.Code != http.StatusOK {
		t.Fatalf("page2: %d %s", rec2.Code, rec2.Body.String())
	}
	var p2 page
	json.Unmarshal(e2.Data, &p2)
	if len(p2.Items) != 1 {
		t.Fatalf("第二页应 1 项 (修复前 page2 为空), got %d", len(p2.Items))
	}
	if p2.Items[0].Key == p1.Items[0].Key {
		t.Errorf("第二页应与首页不同, both got key=%q", p1.Items[0].Key)
	}
}

func TestImagesListRequiresAuth(t *testing.T) {
	s := newUploadTestServer(t)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/images", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("匿名列表应 401, got %d", rec.Code)
	}
}

func TestImageDetailOwnerSeesUploadIP(t *testing.T) {
	s := newUploadTestServer(t)
	sess := register(t, s)
	keys := uploadN(t, s, sess, 1)
	rec, e := doJSON(t, s, "GET", "/api/v1/images/"+keys[0], "", []*http.Cookie{sess})
	if rec.Code != http.StatusOK {
		t.Fatalf("detail: %d %s", rec.Code, rec.Body.String())
	}
	var d map[string]any
	json.Unmarshal(e.Data, &d)
	if d["mime"] == nil || d["upload_ip"] == nil {
		t.Errorf("属主详情应含 mime 与 upload_ip: %s", rec.Body.String())
	}
	if d["links"] == nil {
		t.Error("详情应含 links")
	}
}

func TestImageDetailUnknownKey404(t *testing.T) {
	s := newUploadTestServer(t)
	sess := register(t, s)
	rec, e := doJSON(t, s, "GET", "/api/v1/images/nope00000000", "", []*http.Cookie{sess})
	if rec.Code != http.StatusNotFound || code(t, e) != "not_found" {
		t.Errorf("未知图应 404 not_found, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestImagePatchRenameAndVisibility(t *testing.T) {
	s := newUploadTestServer(t)
	sess := register(t, s)
	keys := uploadN(t, s, sess, 1)
	rec, e := doJSON(t, s, "PATCH", "/api/v1/images/"+keys[0],
		`{"name":"新名字","visibility":"private"}`, []*http.Cookie{sess})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body.String())
	}
	var d map[string]any
	json.Unmarshal(e.Data, &d)
	if d["name"] != "新名字" || d["visibility"] != "private" {
		t.Errorf("patch 未生效: %s", rec.Body.String())
	}
}

func TestImagePatchBadVisibility400(t *testing.T) {
	s := newUploadTestServer(t)
	sess := register(t, s)
	keys := uploadN(t, s, sess, 1)
	rec, e := doJSON(t, s, "PATCH", "/api/v1/images/"+keys[0], `{"visibility":"x"}`, []*http.Cookie{sess})
	if rec.Code != http.StatusBadRequest || code(t, e) != "invalid_request" {
		t.Errorf("非法可见性应 400, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestBatchDeleteHTTP(t *testing.T) {
	s := newUploadTestServer(t)
	sess := register(t, s)
	keys := uploadN(t, s, sess, 2)
	body := `{"action":"delete","keys":["` + keys[0] + `","` + keys[1] + `"]}`
	rec, e := doJSON(t, s, "POST", "/api/v1/images/batch", body, []*http.Cookie{sess})
	if rec.Code != http.StatusOK {
		t.Fatalf("batch: %d %s", rec.Code, rec.Body.String())
	}
	var d struct {
		Results []struct {
			Key string `json:"key"`
			OK  bool   `json:"ok"`
		} `json:"results"`
	}
	json.Unmarshal(e.Data, &d)
	if len(d.Results) != 2 || !d.Results[0].OK {
		t.Fatalf("批量结果异常: %s", rec.Body.String())
	}
}

func TestBatchTooManyKeys400(t *testing.T) {
	s := newUploadTestServer(t)
	sess := register(t, s)
	// 造 101 个假 key
	keys := make([]string, 101)
	for i := range keys {
		keys[i] = "k00000000000"
	}
	b, _ := json.Marshal(map[string]any{"action": "delete", "keys": keys})
	rec, e := doJSON(t, s, "POST", "/api/v1/images/batch", string(b), []*http.Cookie{sess})
	if rec.Code != http.StatusBadRequest || code(t, e) != "invalid_request" {
		t.Errorf("超 100 键应 400, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestSharePublicNoAuth(t *testing.T) {
	s := newUploadTestServer(t)
	sess := register(t, s)
	keys := uploadN(t, s, sess, 1)
	// no cookie
	rec, e := doJSON(t, s, "GET", "/api/v1/s/"+keys[0], "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("share public: %d %s", rec.Code, rec.Body.String())
	}
	var d struct {
		Key      string `json:"key"`
		ShareURL string `json:"share_url"`
		Links    struct {
			URL      string `json:"url"`
			Markdown string `json:"markdown"`
			ShareURL string `json:"share_url"`
		} `json:"links"`
		Width  int `json:"width"`
		Height int `json:"height"`
	}
	if err := json.Unmarshal(e.Data, &d); err != nil {
		t.Fatal(err)
	}
	if d.Key != keys[0] || d.Links.URL == "" || d.Width == 0 {
		t.Fatalf("share dto incomplete: %s", rec.Body.String())
	}
	if d.ShareURL == "" && d.Links.ShareURL == "" {
		t.Errorf("expect share_url: %s", rec.Body.String())
	}
}

func TestSharePrivateReturns404(t *testing.T) {
	s := newUploadTestServer(t)
	sess := register(t, s)
	req, _ := uploadReq(t, "file", "priv.png", pngBytes(40, 41))
	req.AddCookie(sess)
	// multipart already built — need visibility private via form
	// re-upload with private using raw multipart
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "priv.png")
	fw.Write(pngBytes(40, 41))
	_ = mw.WriteField("visibility", "private")
	mw.Close()
	req = httptest.NewRequest("POST", "/api/v1/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(sess)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload private: %d %s", rec.Code, rec.Body.String())
	}
	var e env
	json.Unmarshal(rec.Body.Bytes(), &e)
	var d struct {
		Key string `json:"key"`
	}
	json.Unmarshal(e.Data, &d)
	rec2, _ := doJSON(t, s, "GET", "/api/v1/s/"+d.Key, "", nil)
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("private share want 404, got %d %s", rec2.Code, rec2.Body.String())
	}
}
