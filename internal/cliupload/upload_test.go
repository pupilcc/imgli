package cliupload

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeBaseURL(t *testing.T) {
	got, err := NormalizeBaseURL("https://img.li/")
	if err != nil || got != "https://img.li" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := NormalizeBaseURL(""); err == nil {
		t.Fatal("empty should fail")
	}
	if _, err := NormalizeBaseURL("ftp://x"); err == nil {
		t.Fatal("ftp should fail")
	}
	// Path stripped
	got, err = NormalizeBaseURL("http://localhost:8686/api/v1")
	if err != nil || got != "http://localhost:8686" {
		t.Fatalf("path strip: %q %v", got, err)
	}
}

func TestFormatOutput(t *testing.T) {
	res := &Result{
		Name: "a.png",
		Links: Links{
			URL:      "https://img.li/i/abc.png",
			Markdown: "![a.png](https://img.li/i/abc.png)",
		},
		RawJSON: json.RawMessage(`{"key":"k","links":{"url":"https://img.li/i/abc.png"}}`),
	}
	u, err := FormatOutput("url", res)
	if err != nil || u != res.Links.URL {
		t.Fatalf("url: %q %v", u, err)
	}
	md, err := FormatOutput("markdown", res)
	if err != nil || md != res.Links.Markdown {
		t.Fatalf("md: %q %v", md, err)
	}
	j, err := FormatOutput("json", res)
	if err != nil || !strings.Contains(j, `"key"`) {
		t.Fatalf("json: %q %v", j, err)
	}
	if _, err := FormatOutput("html", res); err == nil {
		t.Fatal("html format should fail")
	}
}

func TestUploadSuccessAndAuth(t *testing.T) {
	var sawAuth, sawCT, sawName string
	var sawVis string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/upload" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		sawAuth = r.Header.Get("Authorization")
		sawCT = r.Header.Get("Content-Type")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse: %v", err)
			http.Error(w, "bad", 400)
			return
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "no file", 400)
			return
		}
		defer f.Close()
		sawName = hdr.Filename
		b, _ := io.ReadAll(f)
		if string(b) != "PNGDATA" {
			t.Errorf("body=%q", b)
		}
		sawVis = r.FormValue("visibility")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":true,"message":"ok","data":{"key":"k1","name":"shot.png","size":7,"instant":false,"links":{"url":"https://img.li/i/k1.png","markdown":"![shot.png](https://img.li/i/k1.png)","html":"","bbcode":"","thumbnail_url":""},"expires_at":null}}`))
	}))
	defer srv.Close()

	res, err := Upload(context.Background(), Opts{
		BaseURL:    srv.URL,
		Token:      "tok123",
		Filename:   "shot.png",
		Visibility: "public",
		Client:     srv.Client(),
	}, strings.NewReader("PNGDATA"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Key != "k1" || res.Links.URL != "https://img.li/i/k1.png" {
		t.Fatalf("%+v", res)
	}
	if sawAuth != "Bearer tok123" {
		t.Errorf("auth=%q", sawAuth)
	}
	if !strings.HasPrefix(sawCT, "multipart/form-data") {
		t.Errorf("ct=%q", sawCT)
	}
	if sawName != "shot.png" || sawVis != "public" {
		t.Errorf("name=%q vis=%q", sawName, sawVis)
	}
}

func TestUploadAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status":false,"message":"未授权","data":{"code":"unauthorized"}}`))
	}))
	defer srv.Close()

	_, err := Upload(context.Background(), Opts{
		BaseURL:  srv.URL,
		Token:    "bad",
		Filename: "a.png",
		Client:   srv.Client(),
	}, strings.NewReader("x"))
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("err=%v", err)
	}
}

func TestUploadGuestOmitsAuth(t *testing.T) {
	var sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":true,"message":"ok","data":{"key":"g","name":"x.png","size":1,"instant":false,"links":{"url":"http://x/i/g.png","markdown":"","html":"","bbcode":"","thumbnail_url":""}}}`))
	}))
	defer srv.Close()

	_, err := Upload(context.Background(), Opts{
		BaseURL:  srv.URL,
		Filename: "x.png",
		Client:   srv.Client(),
	}, strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	if sawAuth != "" {
		t.Errorf("guest should omit Authorization, got %q", sawAuth)
	}
}
