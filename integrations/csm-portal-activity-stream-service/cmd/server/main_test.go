package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	cases := []struct {
		name     string
		running  func() bool
		wantCode int
		wantBody string
	}{
		{"event hub not configured", nil, http.StatusOK, `{"status":"ok"}`},
		{"consumer running", func() bool { return true }, http.StatusOK, `{"status":"ok"}`},
		{"consumer not running", func() bool { return false }, http.StatusServiceUnavailable, `{"status":"unavailable"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			healthHandler(tc.running)(w, httptest.NewRequest(http.MethodGet, "/health", nil))
			if w.Code != tc.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tc.wantCode)
			}
			if w.Body.String() != tc.wantBody {
				t.Errorf("body = %q, want %q", w.Body.String(), tc.wantBody)
			}
			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
		})
	}
}
