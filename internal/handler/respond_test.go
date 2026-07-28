package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOKEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	OK(rec, map[string]int{"n": 1})
	var b struct {
		Status  bool           `json:"status"`
		Message string         `json:"message"`
		Data    map[string]int `json:"data"`
	}
	json.Unmarshal(rec.Body.Bytes(), &b)
	if !b.Status || b.Message != "ok" || b.Data["n"] != 1 {
		t.Errorf("body = %s", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestFailEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	Fail(rec, http.StatusRequestEntityTooLarge, CodeFileTooLarge, "文件超过 20 MB 上限")
	if rec.Code != 413 {
		t.Errorf("status = %d", rec.Code)
	}
	var b struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Data    struct {
			Code string `json:"code"`
		} `json:"data"`
	}
	json.Unmarshal(rec.Body.Bytes(), &b)
	if b.Status || b.Data.Code != "file_too_large" || b.Message == "" {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestRecovererTurnsPanicInto500(t *testing.T) {
	// Suppress slog output during panic recovery test
	oldDefault := slog.Default()
	t.Cleanup(func() { slog.SetDefault(oldDefault) })
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	h := Recoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 500 {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	var b struct {
		Status bool `json:"status"`
		Data   struct {
			Code string `json:"code"`
		} `json:"data"`
	}
	json.Unmarshal(rec.Body.Bytes(), &b)
	if b.Status || b.Data.Code != "internal_error" {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestRealIP(t *testing.T) {
	var got string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = ClientIP(r) })
	newReq := func(xff string) *http.Request {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.0.0.9:1234"
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		return req
	}

	// 不信任代理：XFF 是攻击面，始终用 RemoteAddr
	RealIP(false)(inner).ServeHTTP(httptest.NewRecorder(), newReq("1.2.3.4, 172.16.0.7"))
	if got != "10.0.0.9" {
		t.Errorf("不信任代理 = %q, want 10.0.0.9", got)
	}
	// 信任单跳反代：取最右值（反代追加的那个）；最左值可被客户端伪造
	RealIP(true)(inner).ServeHTTP(httptest.NewRecorder(), newReq("8.8.8.8, 172.16.0.7"))
	if got != "172.16.0.7" {
		t.Errorf("信任代理 = %q, want 172.16.0.7（最右）", got)
	}
	// 畸形 XFF（非 IP）：退回 RemoteAddr
	RealIP(true)(inner).ServeHTTP(httptest.NewRecorder(), newReq("not-an-ip"))
	if got != "10.0.0.9" {
		t.Errorf("畸形 XFF = %q, want 10.0.0.9", got)
	}
	// 空尾 token（"1.2.3.4, "）：退回 RemoteAddr
	RealIP(true)(inner).ServeHTTP(httptest.NewRecorder(), newReq("1.2.3.4, "))
	if got != "10.0.0.9" {
		t.Errorf("空尾 token = %q, want 10.0.0.9", got)
	}
	// 无 XFF
	RealIP(true)(inner).ServeHTTP(httptest.NewRecorder(), newReq(""))
	if got != "10.0.0.9" {
		t.Errorf("无 XFF = %q, want 10.0.0.9", got)
	}
}
