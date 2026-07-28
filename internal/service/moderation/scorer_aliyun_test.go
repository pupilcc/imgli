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

func TestAliyunScorerEmptyImageURLSkips(t *testing.T) {
	called := false
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			called = true
			return nil, io.EOF
		}),
	}
	sc := &AliyunScorer{
		AccessKeyID: "ak", AccessKeySecret: "sk", Region: "cn-shanghai",
		Client: client,
	}
	score, err := sc.Score(context.Background(), bytes.NewReader([]byte("x")), "image/png", "")
	if err != nil {
		t.Fatalf("空 imageURL 应降级不报错, got %v", err)
	}
	if score != 0 {
		t.Errorf("score = %v, want 0", score)
	}
	if called {
		t.Error("空 imageURL 不应发起 HTTP 请求")
	}
}

func TestAliyunScorerScoreFromConfidence(t *testing.T) {
	var gotAuth, gotAction string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAction = r.Header.Get("x-acs-action")
		json.NewEncoder(w).Encode(map[string]any{
			"Code": 200,
			"Data": map[string]any{
				"Result": []any{
					map[string]any{"Confidence": 92.5},
					map[string]any{"Confidence": 10},
				},
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
	sc := &AliyunScorer{
		AccessKeyID: "ak", AccessKeySecret: "sk", Region: "cn-shanghai",
		Client: client,
		now:    func() time.Time { return fixed },
		nonce:  func() string { return "fixed-nonce-000000" },
	}
	score, err := sc.Score(context.Background(), bytes.NewReader([]byte("x")), "image/png", "https://img.example/i/k.png")
	if err != nil {
		t.Fatal(err)
	}
	if score != 0.925 {
		t.Errorf("score = %v, want 0.925", score)
	}
	if !strings.HasPrefix(gotAuth, "ACS3-HMAC-SHA256") {
		t.Errorf("Authorization = %q, want ACS3-HMAC-SHA256 前缀", gotAuth)
	}
	if gotAction != "ImageModeration" {
		t.Errorf("x-acs-action = %q, want ImageModeration", gotAction)
	}
}

// redirectClient 把请求改指向 httptest 服务器(host 由签名固定,不能真连阿里)。
func redirectClient(srvURL string) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = strings.TrimPrefix(srvURL, "http://")
		req.Host = req.URL.Host
		return http.DefaultTransport.RoundTrip(req)
	})}
}

func aliyunTestScorer(client *http.Client) *AliyunScorer {
	return &AliyunScorer{
		AccessKeyID: "ak", AccessKeySecret: "sk", Region: "cn-shanghai",
		Client: client,
		now:    func() time.Time { return time.Unix(1, 0) },
		nonce:  func() string { return "n" },
	}
}

// codex 评审 F1:nonLabel(无风险)高置信度必须排除,否则安全图误判高分。
func TestAliyunScorerExcludesNonLabel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"Code": 200, "Data": map[string]any{"Result": []any{
			map[string]any{"Label": "nonLabel", "Confidence": 99},
			map[string]any{"Label": "pornographic", "Confidence": 30},
		}}})
	}))
	defer srv.Close()
	score, err := aliyunTestScorer(redirectClient(srv.URL)).Score(context.Background(), bytes.NewReader([]byte("x")), "image/png", "https://x/i.png")
	if err != nil || score != 0.3 {
		t.Errorf("score=%v err=%v, want 0.3(排除 nonLabel 99)", score, err)
	}
}

func TestAliyunScorerAllNonLabelIsZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"Code": 200, "Data": map[string]any{"Result": []any{
			map[string]any{"Label": "nonLabel", "Confidence": 95},
		}}})
	}))
	defer srv.Close()
	score, err := aliyunTestScorer(redirectClient(srv.URL)).Score(context.Background(), bytes.NewReader([]byte("x")), "image/png", "https://x/i.png")
	if err != nil || score != 0 {
		t.Errorf("score=%v err=%v, want 0(全 nonLabel)", score, err)
	}
}

// codex 评审 F5:成功响应缺 Data.Result 视为畸形报错,不静默判安全。
func TestAliyunScorerEmptyResultErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"Code": 200, "Data": map[string]any{"Result": []any{}}})
	}))
	defer srv.Close()
	if _, err := aliyunTestScorer(redirectClient(srv.URL)).Score(context.Background(), bytes.NewReader([]byte("x")), "image/png", "https://x/i.png"); err == nil {
		t.Error("空 Result 应报错")
	}
}

func TestAliyunScorerCodeNot200Errors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"Code": 400, "Data": map[string]any{}})
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
	sc := &AliyunScorer{
		AccessKeyID: "ak", AccessKeySecret: "sk", Region: "cn-shanghai",
		Client: client,
		now:    func() time.Time { return time.Unix(1, 0) },
		nonce:  func() string { return "n" },
	}
	if _, err := sc.Score(context.Background(), bytes.NewReader([]byte("x")), "image/png", "https://x/i.png"); err == nil {
		t.Error("Code=400 应报错")
	}
}
