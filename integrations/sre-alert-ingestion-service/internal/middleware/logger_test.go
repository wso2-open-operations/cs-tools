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
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/middleware"
)

// loggedStatus runs next through Logger, capturing slog's default logger
// output for the duration, and returns the "status" attribute the access
// log line recorded -- this is the only way to observe rw.status, since
// responseWriter is unexported and net/http's own ResponseRecorder already
// only honors the first WriteHeader on the wire regardless of this
// middleware's own (previously buggy) internal tracking.
func loggedStatus(t *testing.T, next http.Handler) int64 {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	middleware.Logger(next).ServeHTTP(w, r)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log line %q: %v", buf.String(), err)
	}
	status, ok := entry["status"].(float64)
	if !ok {
		t.Fatalf("log line has no numeric status field: %v", entry)
	}
	return int64(status)
}

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

// TestLogger_DoubleWriteHeaderLogsOnlyTheFirstStatus is the regression test
// for the actual bug: net/http commits only the first WriteHeader call, but
// the previous responseWriter.WriteHeader overwrote rw.status on every
// call, so a handler calling WriteHeader twice made the access log report a
// status the client never received.
// Deliberately not t.Parallel(): loggedStatus mutates the process-global
// slog default logger, which two parallel subtests would stomp on each
// other's captured output.
func TestLogger_DoubleWriteHeaderLogsOnlyTheFirstStatus(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.WriteHeader(http.StatusInternalServerError) // no-op on the wire; must not change the log either
	})

	if got, want := loggedStatus(t, next), int64(http.StatusAccepted); got != want {
		t.Errorf("logged status = %d, want %d (the second WriteHeader must be ignored)", got, want)
	}
}

// TestLogger_ImplicitOKViaWriteIsLogged covers the other half of the fix:
// a handler that writes a body without ever calling WriteHeader commits an
// implicit 200 -- the log must reflect that, not an unset zero value.
// Deliberately not t.Parallel() -- see TestLogger_DoubleWriteHeaderLogsOnlyTheFirstStatus.
func TestLogger_ImplicitOKViaWriteIsLogged(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("no explicit WriteHeader"))
	})

	if got, want := loggedStatus(t, next), int64(http.StatusOK); got != want {
		t.Errorf("logged status = %d, want %d", got, want)
	}
}
