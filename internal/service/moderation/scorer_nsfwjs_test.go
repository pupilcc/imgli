package moderation

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNSFWJSScorerScoreFormula(t *testing.T) {
	var gotCT, gotAuth string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		gotAuth = r.Header.Get("Authorization")
		gotBody, _ = io.ReadAll(r.Body)
		json.NewEncoder(w).Encode(map[string]any{
			"porn": 0.6, "hentai": 0.2, "sexy": 0.4,
		})
	}))
	defer srv.Close()

	sc := &NSFWJSScorer{Endpoint: srv.URL, APIKey: "tok"}
	// max(0.6,0.2)+0.5*0.4 = 0.6+0.2 = 0.8
	score, err := sc.Score(context.Background(), bytes.NewReader([]byte("img")), "image/jpeg", "")
	if err != nil {
		t.Fatal(err)
	}
	if score != 0.8 {
		t.Errorf("score = %v, want 0.8", score)
	}
	if gotCT != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", gotCT)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("Authorization = %q, want Bearer tok", gotAuth)
	}
	if string(gotBody) != "img" {
		t.Errorf("body = %q, want img", gotBody)
	}
}

func TestNSFWJSScorerMissingFieldsDefaultZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	sc := &NSFWJSScorer{Endpoint: srv.URL}
	score, err := sc.Score(context.Background(), bytes.NewReader([]byte("x")), "image/png", "")
	if err != nil {
		t.Fatal(err)
	}
	if score != 0 {
		t.Errorf("缺字段 score = %v, want 0", score)
	}
}

func TestNSFWJSScorerNon200Errors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	sc := &NSFWJSScorer{Endpoint: srv.URL}
	if _, err := sc.Score(context.Background(), bytes.NewReader([]byte("x")), "image/png", ""); err == nil {
		t.Error("非 200 应报错")
	}
}

func TestNSFWJSScorerClampAboveOne(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"porn": 0.9, "hentai": 0.95, "sexy": 0.8, // 0.95+0.4=1.35 → 1
		})
	}))
	defer srv.Close()

	sc := &NSFWJSScorer{Endpoint: srv.URL}
	score, err := sc.Score(context.Background(), bytes.NewReader([]byte("x")), "image/png", "")
	if err != nil {
		t.Fatal(err)
	}
	if score != 1 {
		t.Errorf("score = %v, want 1 (clamp)", score)
	}
}
