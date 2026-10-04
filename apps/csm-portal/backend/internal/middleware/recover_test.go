// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecover(t *testing.T) {
	t.Run("passes a normal response through untouched", func(t *testing.T) {
		h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
		if w.Code != http.StatusCreated || w.Body.String() != `{"ok":true}` {
			t.Fatalf("got %d %q", w.Code, w.Body.String())
		}
	})

	t.Run("turns a panic into the standard 500 envelope and logs the correlation id", func(t *testing.T) {
		var logBuf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, nil)))
		t.Cleanup(func() { slog.SetDefault(prev) })

		// CorrelationID sits inside Recover in the real chain; the same order here
		// proves Recover can still report the id it set on the response header.
		h := Recover(CorrelationID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("assignment to entry in nil map")
		})))
		r := httptest.NewRequest(http.MethodPost, "/cases/x/comments/search", strings.NewReader("null"))
		r.Header.Set(correlationIDHeader, "11111111-2222-4333-8444-555555555555")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("Content-Type = %q", ct)
		}
		var body struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("body %q is not JSON: %v", w.Body.String(), err)
		}
		if body.Message != "An internal server error occurred. Please try again later." {
			t.Fatalf("message = %q", body.Message)
		}
		logged := logBuf.String()
		if !strings.Contains(logged, "handler panic recovered") || !strings.Contains(logged, "11111111-2222-4333-8444-555555555555") {
			t.Fatalf("log line missing panic/correlation id: %s", logged)
		}
		if !strings.Contains(logged, "nil map") {
			t.Fatalf("log line missing panic value: %s", logged)
		}
	})

	t.Run("does not write a second status when the handler already committed one", func(t *testing.T) {
		var logBuf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, nil)))
		t.Cleanup(func() { slog.SetDefault(prev) })

		h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte("partial"))
			panic("late")
		}))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
		if w.Code != http.StatusAccepted || w.Body.String() != "partial" {
			t.Fatalf("response was rewritten: %d %q", w.Code, w.Body.String())
		}
		if !strings.Contains(logBuf.String(), "late") {
			t.Fatalf("panic not logged: %s", logBuf.String())
		}
	})

	t.Run("re-panics http.ErrAbortHandler", func(t *testing.T) {
		h := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic(http.ErrAbortHandler)
		}))
		defer func() {
			if rec := recover(); rec != http.ErrAbortHandler {
				t.Fatalf("recovered %v, want http.ErrAbortHandler", rec)
			}
		}()
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
		t.Fatal("expected the abort sentinel to propagate")
	})
}
