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
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// userIDSeen runs the UserID middleware on a request with the given header
// (none when empty) and returns the id the next handler sees.
func userIDSeen(t *testing.T, header string) string {
	t.Helper()
	var got string
	h := UserID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = UserIDFromContext(r.Context())
	}))
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	if header != "" {
		r.Header.Set("X-CSM-User-ID", header)
	}
	h.ServeHTTP(httptest.NewRecorder(), r)
	return got
}

// TestUserID_StoresTheHeader: the header's id is what the next handler sees.
func TestUserID_StoresTheHeader(t *testing.T) {
	t.Parallel()
	if got := userIDSeen(t, "user-42"); got != "user-42" {
		t.Errorf("UserIDFromContext() = %q, want user-42", got)
	}
}

// TestUserID_AbsentOrTooLongIsEmpty: no header, or one over maxUserIDLength,
// leaves the context without a user id.
func TestUserID_AbsentOrTooLongIsEmpty(t *testing.T) {
	t.Parallel()
	if got := userIDSeen(t, ""); got != "" {
		t.Errorf("no header: UserIDFromContext() = %q, want empty", got)
	}
	if got := userIDSeen(t, strings.Repeat("x", maxUserIDLength+1)); got != "" {
		t.Errorf("over-long header: UserIDFromContext() = %q, want empty", got)
	}
}

// TestCtxHandler_AddsTheCorrelationAndUserIDs: a record logged with a request's
// context gets both ids, under the portal backend's keys; one logged without
// gets neither.
func TestCtxHandler_AddsTheCorrelationAndUserIDs(t *testing.T) {
	t.Parallel()
	var out strings.Builder
	log := slog.New(ctxHandler{inner: slog.NewJSONHandler(&out, nil)})

	ctx := context.WithValue(context.WithValue(context.Background(), correlationIDKey{}, "cai-1"), userIDKey{}, "user-42")
	log.InfoContext(ctx, "with")
	log.InfoContext(context.Background(), "without")

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("log = %q, want two lines", out.String())
	}
	var with, without map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &with)
	_ = json.Unmarshal([]byte(lines[1]), &without)
	if with["correlationID"] != "cai-1" || with["userID"] != "user-42" {
		t.Errorf("with a request context = %v, want both ids", with)
	}
	if _, ok := without["userID"]; ok {
		t.Errorf("without a request context = %v, want no userID", without)
	}
}
