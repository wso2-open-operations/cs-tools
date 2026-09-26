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

func TestSearchITServices(t *testing.T) {
	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewITServiceHandler(&mockEntityITServiceClient{})
		r := httptest.NewRequest(http.MethodPost, "/services/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))
		w := httptest.NewRecorder()
		h.SearchITServices(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewITServiceHandler(&mockEntityITServiceClient{})
		r := httptest.NewRequest(http.MethodPost, "/services/search", strings.NewReader(`not-json`))
		w := httptest.NewRecorder()
		h.SearchITServices(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty body", func(t *testing.T) {
		h := NewITServiceHandler(&mockEntityITServiceClient{})
		r := httptest.NewRequest(http.MethodPost, "/services/search", nil)
		w := httptest.NewRecorder()
		h.SearchITServices(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body verbatim and returns upstream response", func(t *testing.T) {
		var capturedBody []byte
		reqBody := `{"filters":{"searchQuery":"Azure Monitoring"},"pagination":{"limit":1,"offset":0}}`
		client := &mockEntityITServiceClient{
			searchITServicesFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"services":[{"id":"33333333-3333-3333-3333-333333333333","name":"Azure Monitoring"}],"total":1,"limit":1,"offset":0}`), nil
			},
		}
		h := NewITServiceHandler(client)
		r := httptest.NewRequest(http.MethodPost, "/services/search", strings.NewReader(reqBody))
		w := httptest.NewRecorder()
		h.SearchITServices(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if string(capturedBody) != reqBody {
			t.Errorf("upstream body = %q, want verbatim %q", string(capturedBody), reqBody)
		}

		resp := decodeJSON[map[string]any](t, w)
		if resp["total"] != float64(1) {
			t.Errorf("total = %v, want %v", resp["total"], 1)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to search services.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityITServiceClient{
					searchITServicesFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewITServiceHandler(client)
				r := httptest.NewRequest(http.MethodPost, "/services/search", strings.NewReader(`{}`))
				w := httptest.NewRecorder()
				h.SearchITServices(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
