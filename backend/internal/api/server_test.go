package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

func TestRequireAPIKey(t *testing.T) {
	cases := []struct {
		name   string
		envKey string
		header string
		want   int
	}{
		{"key not configured", "", "anything", http.StatusInternalServerError},
		{"missing header", "secret", "", http.StatusUnauthorized},
		{"wrong key", "secret", "wrong", http.StatusUnauthorized},
		{"correct key", "secret", "secret", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("INFRAALIGN_API_KEY", tc.envKey)
			req := httptest.NewRequest(http.MethodPost, "/api/remediate/b", nil)
			if tc.header != "" {
				req.Header.Set("X-API-Key", tc.header)
			}
			rec := httptest.NewRecorder()
			requireAPIKey(okHandler)(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestCORS(t *testing.T) {
	h := withCORS(http.HandlerFunc(okHandler))

	// Preflight from the local dashboard is answered and allowed.
	req := httptest.NewRequest(http.MethodOptions, "/api/remediate/b", nil)
	req.Header.Set("Origin", "null")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "null" {
		t.Fatalf("preflight: code %d, allow-origin %q", rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
	}

	// localhost dev server origin is allowed.
	req = httptest.NewRequest(http.MethodGet, "/api/drift", nil)
	req.Header.Set("Origin", "http://localhost:5500")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5500" {
		t.Fatalf("localhost origin not allowed")
	}

	// A random website is not given CORS access.
	req = httptest.NewRequest(http.MethodGet, "/api/drift", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected allow-origin for foreign site: %q", got)
	}
}
