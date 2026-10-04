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

package middleware_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-integration-service/internal/middleware"
)

func TestLogger_CallsNextAndPreservesResponse(t *testing.T) {
	t.Parallel()

	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("hello"))
	})

	r := httptest.NewRequest(http.MethodGet, "/some/path", nil)
	w := httptest.NewRecorder()
	middleware.Logger(next).ServeHTTP(w, r)

	if !nextCalled {
		t.Fatal("Logger did not call the wrapped handler")
	}
	if w.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d", w.Code, http.StatusTeapot)
	}
	if body := w.Body.String(); body != "hello" {
		t.Errorf("body = %q, want %q", body, "hello")
	}
}

func TestLogger_DefaultsStatusToOKWhenUnset(t *testing.T) {
	t.Parallel()

	// A handler that never calls WriteHeader explicitly (net/http defaults this to 200).
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	middleware.Logger(next).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// captureLogs routes the default slog logger into a buffer for the duration of
// the test. Not parallel: it swaps process-wide state.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestLogger_LogsFirstStatusOnly(t *testing.T) {
	buf := captureLogs(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.WriteHeader(http.StatusInternalServerError) // ignored by net/http
	})
	w := httptest.NewRecorder()
	middleware.Logger(next).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/cases/search", nil))
	if w.Code != http.StatusAccepted {
		t.Errorf("sent status = %d, want 202", w.Code)
	}
	if !strings.Contains(buf.String(), "status=202") {
		t.Errorf("access log = %q, want status=202", buf.String())
	}
	if strings.Contains(buf.String(), "status=500") {
		t.Errorf("access log recorded the ignored second status: %q", buf.String())
	}
}

func TestLogger_WriteBeforeWriteHeaderLogs200(t *testing.T) {
	buf := captureLogs(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("x"))
		w.WriteHeader(http.StatusTeapot) // too late; already 200
	})
	w := httptest.NewRecorder()
	middleware.Logger(next).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/accounts/x", nil))
	if !strings.Contains(buf.String(), "status=200") || strings.Contains(buf.String(), "status=418") {
		t.Errorf("access log = %q, want status=200 only", buf.String())
	}
}

func TestLogger_SkipsHealthPolls(t *testing.T) {
	buf := captureLogs(t)
	var served int
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served++
		w.WriteHeader(http.StatusOK)
	})
	middleware.Logger(next).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))
	if served != 1 {
		t.Fatalf("health poll served %d times, want 1", served)
	}
	if buf.Len() != 0 {
		t.Errorf("health poll was logged: %q", buf.String())
	}

	// Anything else on /health (or any other path) is still logged.
	middleware.Logger(next).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/health", nil))
	if !strings.Contains(buf.String(), "request completed") {
		t.Errorf("POST /health was not logged: %q", buf.String())
	}
}
