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

func TestSearchProjectOpportunityLinks(t *testing.T) {
	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewProjectOpportunityLinkHandler(&mockEntityProjectOpportunityLinkClient{})
		r := httptest.NewRequest(http.MethodPost, "/project-opportunity-links/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))
		w := httptest.NewRecorder()
		h.SearchProjectOpportunityLinks(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewProjectOpportunityLinkHandler(&mockEntityProjectOpportunityLinkClient{})
		r := httptest.NewRequest(http.MethodPost, "/project-opportunity-links/search", strings.NewReader(`not-json`))
		w := httptest.NewRecorder()
		h.SearchProjectOpportunityLinks(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("allows an empty body", func(t *testing.T) {
		client := &mockEntityProjectOpportunityLinkClient{
			searchProjectOpportunityLinksFn: func(_ context.Context, body []byte) ([]byte, error) {
				if len(body) != 0 {
					t.Errorf("upstream body = %q, want empty", body)
				}
				return []byte(`{"links":[],"total":0}`), nil
			},
		}
		h := NewProjectOpportunityLinkHandler(client)
		r := httptest.NewRequest(http.MethodPost, "/project-opportunity-links/search", nil)
		w := httptest.NewRecorder()
		h.SearchProjectOpportunityLinks(w, r)
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("forwards body to upstream and returns 200 with response", func(t *testing.T) {
		const reqPayload = `{"projectId":"44444444-4444-4444-4444-444444444444","pagination":{"limit":10}}`
		var capturedBody []byte
		client := &mockEntityProjectOpportunityLinkClient{
			searchProjectOpportunityLinksFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"links":[{"id":"55555555-5555-5555-5555-555555555555"}],"total":1}`), nil
			},
		}
		h := NewProjectOpportunityLinkHandler(client)
		r := httptest.NewRequest(http.MethodPost, "/project-opportunity-links/search", strings.NewReader(reqPayload))
		w := httptest.NewRecorder()
		h.SearchProjectOpportunityLinks(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["total"] != float64(1) {
			t.Errorf("total = %v, want 1", resp["total"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to search project-opportunity links.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityProjectOpportunityLinkClient{
					searchProjectOpportunityLinksFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewProjectOpportunityLinkHandler(client)
				r := httptest.NewRequest(http.MethodPost, "/project-opportunity-links/search", strings.NewReader(`{}`))
				w := httptest.NewRecorder()
				h.SearchProjectOpportunityLinks(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
