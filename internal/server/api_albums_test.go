package server

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestAlbumCRUD(t *testing.T) {
	s := newUploadTestServer(t)
	sess := register(t, s)
	// 建
	rec, e := doJSON(t, s, "POST", "/api/v1/albums", `{"name":"工作","visibility":"private"}`, []*http.Cookie{sess})
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created struct{ ID uint64 `json:"id"` }
	json.Unmarshal(e.Data, &created)
	if created.ID == 0 {
		t.Fatalf("create 应返回 id: %s", rec.Body.String())
	}
	// 列
	_, e = doJSON(t, s, "GET", "/api/v1/albums", "", []*http.Cookie{sess})
	var list struct {
		Items []struct {
			ID    uint64 `json:"id"`
			Count int64  `json:"image_count"`
		} `json:"items"`
	}
	json.Unmarshal(e.Data, &list)
	if len(list.Items) != 1 || list.Items[0].Count != 0 {
		t.Fatalf("列表/计数异常: %s", string(e.Data))
	}
	// 改名
	rec, _ = doJSON(t, s, "PATCH", "/api/v1/albums/1", `{"name":"设计稿"}`, []*http.Cookie{sess})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body.String())
	}
	// 删
	rec, _ = doJSON(t, s, "DELETE", "/api/v1/albums/1", "", []*http.Cookie{sess})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAlbumForeign404(t *testing.T) {
	s := newUploadTestServer(t)
	sess := register(t, s)
	rec, e := doJSON(t, s, "GET", "/api/v1/albums/999", "", []*http.Cookie{sess})
	if rec.Code != http.StatusNotFound || code(t, e) != "not_found" {
		t.Errorf("未知相册应 404, got %d %s", rec.Code, rec.Body.String())
	}
}
