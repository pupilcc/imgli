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
)

func TestOpenAIScorerScoreSexualMax(t *testing.T) {
	var gotAuth string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []any{
				map[string]any{
					"category_scores": map[string]any{
						"sexual":         0.9,
						"sexual/minors":  0.1,
						"violence":       0.3,
					},
				},
			},
		})
	}))
	defer srv.Close()

	// 注入 Client 把请求打到桩（OpenAIScorer 固定打 api.openai.com；用 Transport 劫持 host）。
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = strings.TrimPrefix(srv.URL, "http://")
			req.Host = req.URL.Host
			return http.DefaultTransport.RoundTrip(req)
		}),
	}
	sc := &OpenAIScorer{APIKey: "k", Client: client}
	score, err := sc.Score(context.Background(), bytes.NewReader([]byte("png-bytes")), "image/png", "")
	if err != nil {
		t.Fatal(err)
	}
	if score != 0.9 {
		t.Errorf("score = %v, want 0.9", score)
	}
	if gotAuth != "Bearer k" {
		t.Errorf("Authorization = %q, want Bearer k", gotAuth)
	}
	if !bytes.Contains(gotBody, []byte(`"data:image/png;base64,`)) && !bytes.Contains(gotBody, []byte("data:image/png;base64,")) {
		t.Errorf("body 应含 data:image/png;base64, got %s", gotBody)
	}
}

func TestOpenAIScorerNon200Errors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
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
	sc := &OpenAIScorer{APIKey: "k", Client: client}
	if _, err := sc.Score(context.Background(), bytes.NewReader([]byte("x")), "image/png", ""); err == nil {
		t.Error("非 200 应报错")
	}
}

func TestOpenAIScorerOversizeReturnsZeroNoError(t *testing.T) {
	called := false
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			called = true
			return nil, io.EOF
		}),
	}
	sc := &OpenAIScorer{APIKey: "k", Client: client}
	// 20MB+1：超限应 (0, nil) 且不发起请求
	score, err := sc.Score(context.Background(), bytes.NewReader(make([]byte, 20<<20+1)), "image/png", "")
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

// roundTripFunc 便于注入桩 Transport。
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
