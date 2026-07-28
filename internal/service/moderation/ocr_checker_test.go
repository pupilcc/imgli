package moderation

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMatchKeywords(t *testing.T) {
	hits := matchKeywords("Hello World 测试敏感词 here", []string{"敏感词", "HELLO", "nope", "  ", "world"})
	if len(hits) != 3 {
		t.Fatalf("hits=%v, want 3", hits)
	}
	// 保序：敏感词, HELLO, world
	if hits[0] != "敏感词" || strings.ToLower(hits[1]) != "hello" || strings.ToLower(hits[2]) != "world" {
		t.Fatalf("hits=%v", hits)
	}
}

func TestMatchKeywordsCompactSpace(t *testing.T) {
	// 文本内空白折叠后仍可命中连续关键词
	hits := matchKeywords("ab  c", []string{"ab c"})
	if len(hits) != 1 {
		t.Fatalf("hits=%v", hits)
	}
}

func TestMatchKeywordsOCRInsertedSpaces(t *testing.T) {
	// RapidOCR 常在拉丁字母间插空格；去空格后仍应命中
	hits := matchKeywords("IMGLIOC RSMOKE999 pipe line", []string{"IMGLIOCRSMOKE999"})
	if len(hits) != 1 || hits[0] != "IMGLIOCRSMOKE999" {
		t.Fatalf("hits=%v", hits)
	}
}

func TestOCRKeywordsCheckerHitReview(t *testing.T) {
	var gotCT, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{"text": "普通内容 违禁示例 结尾"})
	}))
	defer srv.Close()

	c := &OCRKeywordsChecker{
		Endpoint: srv.URL, APIKey: "tok",
		Keywords: []string{"违禁示例"}, OnHit: "review",
	}
	r, err := c.Check(context.Background(), ImageRef{
		MIME: "image/png",
		Open: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("img")), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Severity != SeverityReview || len(r.Hits) != 1 || r.Hits[0] != "违禁示例" {
		t.Fatalf("got %+v", r)
	}
	if gotCT != "image/png" || gotAuth != "Bearer tok" {
		t.Fatalf("ct=%q auth=%q", gotCT, gotAuth)
	}
}

func TestOCRKeywordsCheckerNoHit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"text": "hello clean"})
	}))
	defer srv.Close()
	c := &OCRKeywordsChecker{Endpoint: srv.URL, Keywords: []string{"bad"}}
	r, err := c.Check(context.Background(), ImageRef{
		Open: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("x")), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Severity != SeverityNone || len(r.Hits) != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestOCRKeywordsCheckerBlockOnHit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"text": "xxx banned xxx"})
	}))
	defer srv.Close()
	c := &OCRKeywordsChecker{Endpoint: srv.URL, Keywords: []string{"banned"}, OnHit: "block"}
	r, err := c.Check(context.Background(), ImageRef{
		Open: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("x")), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Severity != SeverityBlock {
		t.Fatalf("severity=%s", r.Severity)
	}
}

func TestOCRKeywordsCheckerNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	c := &OCRKeywordsChecker{Endpoint: srv.URL, Keywords: []string{"a"}}
	_, err := c.Check(context.Background(), ImageRef{
		Open: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("x")), nil
		},
	})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestBuildCheckersWithOCR(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Provider = "nsfwjs"
	cfg.Endpoint = "http://127.0.0.1:9/"
	cfg.OCRKeywords = OCRKeywordsConfig{
		Enabled: true, Endpoint: "http://127.0.0.1:8/ocr", Keywords: []string{"x"},
	}
	cs := BuildCheckersFromConfig(cfg)
	if len(cs) != 2 || cs[0].Name() != "nsfwjs" || cs[1].Name() != "ocr_keywords" {
		t.Fatalf("%v %v", len(cs), names(cs))
	}
}

func names(cs []Checker) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name()
	}
	return out
}

func TestValidateOCRKeywords(t *testing.T) {
	cfg := DefaultConfig()
	cfg.OCRKeywords.Enabled = true
	cfg.OCRKeywords.Endpoint = "not-url"
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("want invalid")
	}
	cfg.OCRKeywords.Endpoint = "http://ocr.local/v1"
	cfg.OCRKeywords.OnHit = "nope"
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("want on_hit invalid")
	}
	cfg.OCRKeywords.OnHit = "review"
	// 总开关关时仍校验 ocr 子配置（避免脏配置入库）
	if err := ValidateConfig(cfg); err != nil {
		t.Fatal(err)
	}
}
