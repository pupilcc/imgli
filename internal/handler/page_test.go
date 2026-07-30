package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParsePage(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x?page=3&limit=10", nil)
	page, limit := ParsePage(r, 1, 50, 200)
	if page != 3 || limit != 10 {
		t.Fatalf("got page=%d limit=%d", page, limit)
	}

	r = httptest.NewRequest(http.MethodGet, "/x?limit=9999", nil)
	page, limit = ParsePage(r, 1, 50, 200)
	if page != 1 || limit != 200 {
		t.Fatalf("clamp: page=%d limit=%d", page, limit)
	}

	r = httptest.NewRequest(http.MethodGet, "/x?page=0&limit=abc", nil)
	page, limit = ParsePage(r, 1, 50, 200)
	if page != 1 || limit != 50 {
		t.Fatalf("defaults: page=%d limit=%d", page, limit)
	}
}

func TestParseDaysLimit(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x?days=7&limit=50", nil)
	days, limit := ParseDaysLimit(r, 30, 20, 100)
	if days != 7 || limit != 50 {
		t.Fatalf("got days=%d limit=%d", days, limit)
	}
	r = httptest.NewRequest(http.MethodGet, "/x?limit=500", nil)
	days, limit = ParseDaysLimit(r, 30, 20, 100)
	if days != 30 || limit != 100 {
		t.Fatalf("clamp: days=%d limit=%d", days, limit)
	}
}
