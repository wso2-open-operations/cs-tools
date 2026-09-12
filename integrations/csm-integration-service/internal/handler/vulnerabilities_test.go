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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSyncProductVulnerabilities(t *testing.T) {
	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewVulnerabilityHandler(&mockEntityVulnerabilityClient{})
		r := httptest.NewRequest(http.MethodPost, "/vulnerabilities/sync", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))
		w := httptest.NewRecorder()
		h.SyncProductVulnerabilities(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewVulnerabilityHandler(&mockEntityVulnerabilityClient{})
		r := httptest.NewRequest(http.MethodPost, "/vulnerabilities/sync", strings.NewReader(`not-json`))
		w := httptest.NewRecorder()
		h.SyncProductVulnerabilities(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 201 with response", func(t *testing.T) {
		const reqPayload = `[{"productId":"prod-1","vulnerabilityId":"CVE-2026-0001"}]`
		var capturedBody []byte
		client := &mockEntityVulnerabilityClient{
			syncProductVulnerabilitiesFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"insertedRecords":1,"updatedRecords":0,"deletedRecords":0}`), nil
			},
		}
		h := NewVulnerabilityHandler(client)
		r := httptest.NewRequest(http.MethodPost, "/vulnerabilities/sync", strings.NewReader(reqPayload))
		w := httptest.NewRecorder()
		h.SyncProductVulnerabilities(w, r)

		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["insertedRecords"] != float64(1) {
			t.Errorf("insertedRecords = %v, want 1", resp["insertedRecords"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to sync product vulnerabilities.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityVulnerabilityClient{
					syncProductVulnerabilitiesFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewVulnerabilityHandler(client)
				r := httptest.NewRequest(http.MethodPost, "/vulnerabilities/sync", strings.NewReader(`[]`))
				w := httptest.NewRecorder()
				h.SyncProductVulnerabilities(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
