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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestRecoveryLoggerUnwrap_SetWriteDeadline guards against a real regression:
// Recovery(Logger(handler)) (routes.go's actual composition) wraps a
// handler's http.ResponseWriter in recoveryWriter, then responseWriter, so a
// handler calling http.NewResponseController(w).SetWriteDeadline (as
// AutoPublishAnnouncementRequest does, to outlive the server's global
// WriteTimeout) only reaches the real, deadline-capable writer if every
// wrapper in that chain implements Unwrap. responseWriter briefly forwarded
// SetWriteDeadline itself instead -- which type-asserted the *next* layer
// directly (recoveryWriter, which implements neither SetWriteDeadline nor
// Unwrap) and so always failed, silently breaking every AutoPublish call.
// httptest.NewRecorder can't catch this: it isn't a real connection and
// doesn't implement SetWriteDeadline at all, so this needs a real listener.
func TestRecoveryLoggerUnwrap_SetWriteDeadline(t *testing.T) {
	var setDeadlineErr error
	handler := Recovery(Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setDeadlineErr = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(time.Minute))
		w.WriteHeader(http.StatusOK)
	})))

	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if setDeadlineErr != nil {
		t.Fatalf("SetWriteDeadline through Recovery(Logger(...)) failed: %v", setDeadlineErr)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}
