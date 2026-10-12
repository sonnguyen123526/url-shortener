package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sonnguyen/url-shortener/internal/store"
)

func newTestServer() http.Handler {
	return New(store.NewMemory(), "http://short.test").Routes()
}

func TestShortenAndRedirect(t *testing.T) {
	srv := newTestServer()

	// 1. Shorten a URL.
	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"https://go.dev/doc"}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("shorten: got status %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body)
	}
	var resp shortenResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Code) != codeLength {
		t.Errorf("code length = %d, want %d", len(resp.Code), codeLength)
	}
	if want := "http://short.test/" + resp.Code; resp.ShortURL != want {
		t.Errorf("short_url = %q, want %q", resp.ShortURL, want)
	}

	// 2. Follow the short code.
	req = httptest.NewRequest(http.MethodGet, "/"+resp.Code, nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("redirect: got status %d, want %d", rec.Code, http.StatusFound)
	}
	if loc := rec.Header().Get("Location"); loc != "https://go.dev/doc" {
		t.Errorf("Location = %q, want %q", loc, "https://go.dev/doc")
	}
}

func TestShortenRejectsBadInput(t *testing.T) {
	srv := newTestServer()
	tests := map[string]string{
		"not json":       `nope`,
		"missing url":    `{}`,
		"relative url":   `{"url":"/just/a/path"}`,
		"non-http":       `{"url":"ftp://example.com/file"}`,
		"javascript url": `{"url":"javascript:alert(1)"}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(body))
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("got status %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestRedirectUnknownCode(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/doesnotexist", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHealthEndpoints(t *testing.T) {
	srv := newTestServer()
	for _, path := range []string{"/healthz", "/readyz"} {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: got status %d, want %d", path, rec.Code, http.StatusOK)
		}
	}
}
