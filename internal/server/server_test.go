package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yixian-huang/imgli/internal/config"
)

func TestHealthz(t *testing.T) {
	cfg, _ := config.Load("")
	s := New(Options{Cfg: cfg})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Status || body.Message != "ok" {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestNotFoundUsesEnvelope(t *testing.T) {
	cfg, _ := config.Load("")
	s := New(Options{Cfg: cfg})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/no-such", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	var b struct {
		Status bool `json:"status"`
		Data   struct {
			Code string `json:"code"`
		} `json:"data"`
	}
	json.Unmarshal(rec.Body.Bytes(), &b)
	if b.Status || b.Data.Code != "not_found" {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestRunGracefulShutdown(t *testing.T) {
	cfg, _ := config.Load("")
	cfg.Listen = "127.0.0.1:0" // 随机端口，避免冲突
	s := New(Options{Cfg: cfg})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	time.Sleep(50 * time.Millisecond) // 等监听建立

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("优雅停机应返回 nil, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run 未在 ctx 取消后 2s 内退出")
	}
}

func TestParseFetchAllow(t *testing.T) {
	t.Run("有效 CIDR", func(t *testing.T) {
		nets := parseFetchAllow([]string{"10.0.0.0/8"})
		if len(nets) != 1 {
			t.Fatalf("len = %d, want 1", len(nets))
		}
		if !nets[0].Contains(net.ParseIP("10.1.2.3")) {
			t.Error("应包含 10.1.2.3")
		}
		if nets[0].Contains(net.ParseIP("11.0.0.1")) {
			t.Error("不应包含 11.0.0.1")
		}
	})

	t.Run("裸 IPv4 视为 /32", func(t *testing.T) {
		nets := parseFetchAllow([]string{"192.168.1.5"})
		if len(nets) != 1 {
			t.Fatalf("len = %d, want 1", len(nets))
		}
		if !nets[0].Contains(net.ParseIP("192.168.1.5")) {
			t.Error("应包含自身 192.168.1.5")
		}
		if nets[0].Contains(net.ParseIP("192.168.1.6")) {
			t.Error("不应包含 192.168.1.6")
		}
	})

	t.Run("裸 IPv6 视为 /128", func(t *testing.T) {
		nets := parseFetchAllow([]string{"fd00::1"})
		if len(nets) != 1 {
			t.Fatalf("len = %d, want 1", len(nets))
		}
		if !nets[0].Contains(net.ParseIP("fd00::1")) {
			t.Error("应包含自身 fd00::1")
		}
		if nets[0].Contains(net.ParseIP("fd00::2")) {
			t.Error("不应包含 fd00::2")
		}
	})

	t.Run("畸形条目被跳过", func(t *testing.T) {
		nets := parseFetchAllow([]string{"not-an-ip", ""})
		if len(nets) != 0 {
			t.Errorf("len = %d, want 0", len(nets))
		}
	})

	t.Run("nil 输入返回 nil", func(t *testing.T) {
		if nets := parseFetchAllow(nil); nets != nil {
			t.Errorf("nets = %v, want nil", nets)
		}
	})

	t.Run("空输入返回空", func(t *testing.T) {
		if nets := parseFetchAllow([]string{}); len(nets) != 0 {
			t.Errorf("len = %d, want 0", len(nets))
		}
	})
}
