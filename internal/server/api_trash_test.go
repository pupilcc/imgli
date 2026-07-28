package server

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestDeleteMovesToTrashAndRestore(t *testing.T) {
	s := newUploadTestServer(t)
	sess := register(t, s)
	keys := uploadN(t, s, sess, 1)
	// 软删
	rec, _ := doJSON(t, s, "DELETE", "/api/v1/images/"+keys[0], "", []*http.Cookie{sess})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	// 列表不含
	_, e := doJSON(t, s, "GET", "/api/v1/images", "", []*http.Cookie{sess})
	var l struct{ Items []json.RawMessage `json:"items"` }
	json.Unmarshal(e.Data, &l)
	if len(l.Items) != 0 {
		t.Errorf("软删后列表应空, got %d", len(l.Items))
	}
	// 回收站含 + days_left
	_, e = doJSON(t, s, "GET", "/api/v1/trash", "", []*http.Cookie{sess})
	var tr struct {
		Items []struct {
			Key      string `json:"key"`
			DaysLeft int    `json:"days_left"`
		} `json:"items"`
	}
	json.Unmarshal(e.Data, &tr)
	if len(tr.Items) != 1 || tr.Items[0].DaysLeft > 30 || tr.Items[0].DaysLeft < 29 {
		t.Fatalf("回收站项/days_left 异常: %s", string(e.Data))
	}
	// 恢复
	rec, _ = doJSON(t, s, "POST", "/api/v1/trash/"+keys[0]+"/restore", "", []*http.Cookie{sess})
	if rec.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body.String())
	}
	_, e = doJSON(t, s, "GET", "/api/v1/images", "", []*http.Cookie{sess})
	json.Unmarshal(e.Data, &l)
	if len(l.Items) != 1 {
		t.Errorf("恢复后列表应有 1 张, got %d", len(l.Items))
	}
}

func TestPurgeAndEmptyTrash(t *testing.T) {
	s := newUploadTestServer(t)
	sess := register(t, s)
	keys := uploadN(t, s, sess, 2)
	for _, k := range keys {
		doJSON(t, s, "DELETE", "/api/v1/images/"+k, "", []*http.Cookie{sess})
	}
	// 彻底删除第一张
	rec, _ := doJSON(t, s, "DELETE", "/api/v1/trash/"+keys[0], "", []*http.Cookie{sess})
	if rec.Code != http.StatusOK {
		t.Fatalf("purge: %d %s", rec.Code, rec.Body.String())
	}
	// 清空回收站
	rec, e := doJSON(t, s, "DELETE", "/api/v1/trash", "", []*http.Cookie{sess})
	if rec.Code != http.StatusOK {
		t.Fatalf("empty: %d %s", rec.Code, rec.Body.String())
	}
	var d struct{ Purged int `json:"purged"` }
	json.Unmarshal(e.Data, &d)
	if d.Purged != 1 {
		t.Errorf("清空应删剩余 1 张, got %d", d.Purged)
	}
	// 回收站空
	_, e = doJSON(t, s, "GET", "/api/v1/trash", "", []*http.Cookie{sess})
	var tr struct{ Items []json.RawMessage `json:"items"` }
	json.Unmarshal(e.Data, &tr)
	if len(tr.Items) != 0 {
		t.Errorf("清空后回收站应空, got %d", len(tr.Items))
	}
}
