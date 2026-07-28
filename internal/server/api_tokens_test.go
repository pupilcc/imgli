package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// register 返回 alice 的 session cookie（复用 api_auth_test.go 的 doJSON/env/code 助手）。
func register(t *testing.T, s *Server) *http.Cookie {
	t.Helper()
	rec, _ := doJSON(t, s, "POST", "/api/v1/auth/register",
		`{"username":"alice","email":"alice@img.li","password":"passw0rd"}`, nil)
	for _, c := range rec.Result().Cookies() {
		if c.Name == "imgli_session" {
			return c
		}
	}
	t.Fatal("无 session cookie")
	return nil
}

func TestTokenCRUDAndBearerScope(t *testing.T) {
	s := newTestServer(t)
	sess := register(t, s)

	// 创建：明文仅此一次
	rec, e := doJSON(t, s, "POST", "/api/v1/user/tokens",
		`{"name":"picgo","scope":"upload"}`, []*http.Cookie{sess})
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID    uint64 `json:"id"`
		Token string `json:"token"`
		Scope string `json:"scope"`
	}
	json.Unmarshal(e.Data, &created)
	if !strings.HasPrefix(created.Token, "bl_") || created.Scope != "upload" {
		t.Fatalf("created = %+v", created)
	}

	// 列表不含明文
	rec, e = doJSON(t, s, "GET", "/api/v1/user/tokens", "", []*http.Cookie{sess})
	if strings.Contains(string(e.Data), created.Token) {
		t.Error("列表不得含明文 token")
	}

	// upload scope 的 Bearer 访问 full 路由 → 403 forbidden
	req := httptest.NewRequest("GET", "/api/v1/user/tokens", nil)
	req.Header.Set("Authorization", "Bearer "+created.Token)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	var ev env
	json.Unmarshal(rr.Body.Bytes(), &ev)
	if rr.Code != http.StatusForbidden || code(t, ev) != "forbidden" {
		t.Errorf("upload scope 应 403 forbidden: %d %s", rr.Code, rr.Body.String())
	}

	// full scope 的 Bearer 可访问
	rec, e = doJSON(t, s, "POST", "/api/v1/user/tokens",
		`{"name":"cli","scope":"full"}`, []*http.Cookie{sess})
	var full struct {
		Token string `json:"token"`
	}
	json.Unmarshal(e.Data, &full)
	req = httptest.NewRequest("GET", "/api/v1/auth/session", nil)
	req.Header.Set("Authorization", "Bearer "+full.Token)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("full scope Bearer 应可访问 session: %d", rr.Code)
	}

	// 吊销后 Bearer 失效
	rec, _ = doJSON(t, s, "DELETE", fmt.Sprintf("/api/v1/user/tokens/%d", created.ID), "", []*http.Cookie{sess})
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke: %d", rec.Code)
	}
	req = httptest.NewRequest("GET", "/api/v1/auth/session", nil)
	req.Header.Set("Authorization", "Bearer "+created.Token)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("吊销后应 401: %d", rr.Code)
	}
}

func TestCreateTokenInvalidInputs(t *testing.T) {
	s := newTestServer(t)
	sess := register(t, s)
	rec, e := doJSON(t, s, "POST", "/api/v1/user/tokens",
		`{"name":"x","scope":"root"}`, []*http.Cookie{sess})
	if rec.Code != http.StatusBadRequest || code(t, e) != "invalid_request" {
		t.Errorf("非法 scope: %d %s", rec.Code, rec.Body.String())
	}
	longName := strings.Repeat("长", 65)
	rec, e = doJSON(t, s, "POST", "/api/v1/user/tokens",
		`{"name":"`+longName+`","scope":"upload"}`, []*http.Cookie{sess})
	if rec.Code != http.StatusBadRequest || code(t, e) != "invalid_request" {
		t.Errorf("超长名称: %d %s", rec.Code, rec.Body.String())
	}
}
