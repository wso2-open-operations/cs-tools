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

package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDecodeRequestWithLimit_OversizedTrailingData reproduces a case where the
// first JSON object fits within the size cap, but trailing data after it pushes
// the body over the limit. The trailing-data check must report this as a
// size-limit error (the caller-supplied tooLargeMsg), not the generic "must
// contain a single JSON object" message — the two errors mean different things
// to the caller.
func TestDecodeRequestWithLimit_OversizedTrailingData(t *testing.T) {
	const limit = 1024

	body := `{"x":"ok"}` + strings.Repeat(" ", limit*2)
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	rec := httptest.NewRecorder()

	var dst struct {
		X string `json:"x"`
	}
	ok := decodeRequestWithLimit(rec, req, &dst, limit, attachmentTooLargeMsg)

	if ok {
		t.Fatal("expected decodeRequestWithLimit to return false for oversized trailing data")
	}
	// Prove this actually reached the trailing-data check (the second Decode)
	// rather than failing on the first one — dst must reflect a fully-decoded
	// first object.
	if dst.X != "ok" {
		t.Fatalf("dst.X = %q, want %q — the first Decode should have succeeded before the trailing-data check ran", dst.X, "ok")
	}
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), attachmentTooLargeMsg) {
		t.Errorf("response body = %q, want it to contain the size-limit message %q", rec.Body.String(), attachmentTooLargeMsg)
	}
	if strings.Contains(rec.Body.String(), "must contain a single JSON object") {
		t.Errorf("response body = %q, should not fall back to the generic trailing-data message for a size-limit case", rec.Body.String())
	}
}

// TestWriteServiceError_DeadlineIsAGatewayTimeout: a downstream call or query
// that ran out of time is this service's timeout, reported as 504 — never
// 408, which tells the client it was too slow and invites a blind retry.
func TestWriteServiceError_DeadlineIsAGatewayTimeout(t *testing.T) {
	rec := httptest.NewRecorder()
	writeServiceError(rec, httptest.NewRequest(http.MethodPost, "/cases", nil),
		fmt.Errorf("query: %w", context.DeadlineExceeded))
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusGatewayTimeout)
	}
}
