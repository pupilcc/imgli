package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProfileQuotaPasswordFlow(t *testing.T) {
	s := newTestServer(t)
	sess := register(t, s)

	rec, e := doJSON(t, s, "GET", "/api/v1/user/quota", "", []*http.Cookie{sess})
	if rec.Code != http.StatusOK {
		t.Fatalf("quota: %d", rec.Code)
	}
	var data map[string]any
	json.Unmarshal(e.Data, &data)
	if v, ok := data["used"]; !ok {
		t.Errorf("quota 响应缺 used: %v", data)
	} else if used, okf := v.(float64); !okf {
		t.Errorf("used 不是数字: %v", v)
	} else if int64(used) != 0 {
		t.Errorf("used = %d, want 0", int64(used))
	}
	if v, ok := data["total"]; !ok {
		t.Errorf("quota 响应缺 total: %v", data)
	} else if total, okf := v.(float64); !okf {
		t.Errorf("total 不是数字: %v", v)
	} else if int64(total) != 10<<30 {
		t.Errorf("total = %d, want %d", int64(total), 10<<30)
	}
	if _, ok := data["max_file_size"]; !ok {
		t.Errorf("quota 响应缺 max_file_size: %v", data)
	}
	if _, ok := data["allowed_exts"]; !ok {
		t.Errorf("quota 响应缺 allowed_exts: %v", data)
	}

	rec, _ = doJSON(t, s, "PATCH", "/api/v1/user/profile", `{"nickname":"爱丽丝"}`, []*http.Cookie{sess})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch profile: %d", rec.Code)
	}
	rec, e = doJSON(t, s, "GET", "/api/v1/user/profile", "", []*http.Cookie{sess})
	var p struct{ Nickname string `json:"nickname"` }
	json.Unmarshal(e.Data, &p)
	if p.Nickname != "爱丽丝" {
		t.Errorf("nickname = %q", p.Nickname)
	}

	// 改密后当前 session 失效（全设备登出）
	rec, _ = doJSON(t, s, "PATCH", "/api/v1/user/password",
		`{"old_password":"passw0rd","new_password":"newpassw0rd"}`, []*http.Cookie{sess})
	if rec.Code != http.StatusOK {
		t.Fatalf("change pw: %d", rec.Code)
	}
	rec, _ = doJSON(t, s, "GET", "/api/v1/user/profile", "", []*http.Cookie{sess})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("改密后旧 session 应 401, got %d", rec.Code)
	}
}

func TestChangePasswordViaBearerNoSessionError(t *testing.T) {
	s := newTestServer(t)
	sess := register(t, s)

	// 签发 full token 并解析明文
	_, e := doJSON(t, s, "POST", "/api/v1/user/tokens", `{"name":"cli","scope":"full"}`, []*http.Cookie{sess})
	var tokData struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(e.Data, &tokData); err != nil {
		t.Fatal(err)
	}

	// 用 Bearer full token 改密：该主体无 session 行，删除全部 session 应无错、返回 200
	req := httptest.NewRequest("PATCH", "/api/v1/user/password",
		strings.NewReader(`{"old_password":"passw0rd","new_password":"newpassw0rd"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokData.Token)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Bearer 改密应 200, got %d %s", rr.Code, rr.Body.String())
	}
}
