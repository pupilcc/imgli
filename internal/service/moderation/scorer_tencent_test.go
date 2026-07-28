package moderation

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTencentScorerScoreFromResponse(t *testing.T) {
	var gotAuth, gotAction string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAction = r.Header.Get("X-TC-Action")
		json.NewEncoder(w).Encode(map[string]any{
			"Response": map[string]any{
				"Score": 85, "Suggestion": "Block",
			},
		})
	}))
	defer srv.Close()

	fixed := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = strings.TrimPrefix(srv.URL, "http://")
			req.Host = req.URL.Host
			return http.DefaultTransport.RoundTrip(req)
		}),
	}
	sc := &TencentScorer{
		SecretID: "AKIDzTEST", SecretKey: "sk-test", Region: "ap-guangzhou",
		Client: client, now: func() time.Time { return fixed },
	}
	score, err := sc.Score(context.Background(), bytes.NewReader([]byte("png")), "image/png", "https://x/i/k.png")
	if err != nil {
		t.Fatal(err)
	}
	if score != 0.85 {
		t.Errorf("score = %v, want 0.85", score)
	}
	if !strings.HasPrefix(gotAuth, "TC3-HMAC-SHA256") {
		t.Errorf("Authorization = %q, want TC3-HMAC-SHA256 前缀", gotAuth)
	}
	if gotAction != "ImageModeration" {
		t.Errorf("X-TC-Action = %q, want ImageModeration", gotAction)
	}
}

func TestTencentScorerErrorInResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"Response": map[string]any{
				"Error": map[string]any{"Code": "AuthFailure", "Message": "bad key"},
			},
		})
	}))
	defer srv.Close()
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = strings.TrimPrefix(srv.URL, "http://")
			req.Host = req.URL.Host
			return http.DefaultTransport.RoundTrip(req)
		}),
	}
	sc := &TencentScorer{
		SecretID: "id", SecretKey: "key", Region: "ap-guangzhou",
		Client: client, now: func() time.Time { return time.Unix(1735689600, 0) },
	}
	if _, err := sc.Score(context.Background(), bytes.NewReader([]byte("x")), "image/png", ""); err == nil {
		t.Error("含 Error 的响应应报错")
	}
}

func TestTencentScorerOversizeReturnsZeroNoError(t *testing.T) {
	called := false
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			called = true
			return nil, io.EOF
		}),
	}
	sc := &TencentScorer{
		SecretID: "id", SecretKey: "key", Region: "ap-guangzhou",
		Client: client,
	}
	score, err := sc.Score(context.Background(), bytes.NewReader(make([]byte, 10<<20+1)), "image/png", "")
	if err != nil {
		t.Fatalf("超限应降级不报错, got %v", err)
	}
	if score != 0 {
		t.Errorf("score = %v, want 0", score)
	}
	if called {
		t.Error("超限不应发起 HTTP 请求")
	}
}

func tencentRedirectClient(srvURL string) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = strings.TrimPrefix(srvURL, "http://")
		req.Host = req.URL.Host
		return http.DefaultTransport.RoundTrip(req)
	})}
}

func tencentTestScorer(client *http.Client) *TencentScorer {
	return &TencentScorer{SecretID: "id", SecretKey: "key", Region: "ap-guangzhou",
		Client: client, now: func() time.Time { return time.Unix(1, 0) }}
}

// codex 评审 F2:Label=Normal 时顶层 Score 是"确信正常"置信度,不得当风险分。
func TestTencentScorerNormalIsZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"Response": map[string]any{
			"Label": "Normal", "Suggestion": "Pass", "Score": 95,
		}})
	}))
	defer srv.Close()
	score, err := tencentTestScorer(tencentRedirectClient(srv.URL)).Score(context.Background(), bytes.NewReader([]byte("x")), "image/png", "")
	if err != nil || score != 0 {
		t.Errorf("score=%v err=%v, want 0(Normal/Pass)", score, err)
	}
}

// codex 评审 F4:成功响应无任何可用评分字段视为畸形报错,不静默判安全。
func TestTencentScorerMalformedErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"Response": map[string]any{"RequestId": "x"}})
	}))
	defer srv.Close()
	if _, err := tencentTestScorer(tencentRedirectClient(srv.URL)).Score(context.Background(), bytes.NewReader([]byte("x")), "image/png", ""); err == nil {
		t.Error("无评分字段的成功响应应报错")
	}
}

func TestTencentScorerLabelResultsFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"Response": map[string]any{
				"LabelResults": []any{
					map[string]any{"Label": "Porn", "Score": 70},
					map[string]any{"Label": "Sexy", "Score": 40},
				},
			},
		})
	}))
	defer srv.Close()
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = strings.TrimPrefix(srv.URL, "http://")
			req.Host = req.URL.Host
			return http.DefaultTransport.RoundTrip(req)
		}),
	}
	sc := &TencentScorer{
		SecretID: "id", SecretKey: "key", Region: "ap-guangzhou",
		Client: client, now: func() time.Time { return time.Unix(1, 0) },
	}
	score, err := sc.Score(context.Background(), bytes.NewReader([]byte("x")), "image/png", "")
	if err != nil {
		t.Fatal(err)
	}
	if score != 0.7 {
		t.Errorf("score = %v, want 0.7 (LabelResults max)", score)
	}
}
