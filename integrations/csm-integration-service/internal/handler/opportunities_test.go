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

func TestSearchOpportunities(t *testing.T) {
	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewOpportunityHandler(&mockEntityOpportunityClient{})
		r := httptest.NewRequest(http.MethodPost, "/opportunities/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))
		w := httptest.NewRecorder()
		h.SearchOpportunities(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewOpportunityHandler(&mockEntityOpportunityClient{})
		r := httptest.NewRequest(http.MethodPost, "/opportunities/search", strings.NewReader(`not-json`))
		w := httptest.NewRecorder()
		h.SearchOpportunities(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("allows an empty body", func(t *testing.T) {
		client := &mockEntityOpportunityClient{
			searchOpportunitiesFn: func(_ context.Context, body []byte) ([]byte, error) {
				if len(body) != 0 {
					t.Errorf("upstream body = %q, want empty", body)
				}
				return []byte(`{"opportunities":[],"total":0}`), nil
			},
		}
		h := NewOpportunityHandler(client)
		r := httptest.NewRequest(http.MethodPost, "/opportunities/search", nil)
		w := httptest.NewRecorder()
		h.SearchOpportunities(w, r)
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("forwards body to upstream and returns 200 with response", func(t *testing.T) {
		const reqPayload = `{"accountId":"11111111-1111-1111-1111-111111111111","pagination":{"limit":10}}`
		var capturedBody []byte
		client := &mockEntityOpportunityClient{
			searchOpportunitiesFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"opportunities":[{"id":"22222222-2222-2222-2222-222222222222","name":"Renewal"}],"total":1}`), nil
			},
		}
		h := NewOpportunityHandler(client)
		r := httptest.NewRequest(http.MethodPost, "/opportunities/search", strings.NewReader(reqPayload))
		w := httptest.NewRecorder()
		h.SearchOpportunities(w, r)

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
		for _, tc := range upstreamErrors("Failed to search opportunities.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityOpportunityClient{
					searchOpportunitiesFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewOpportunityHandler(client)
				r := httptest.NewRequest(http.MethodPost, "/opportunities/search", strings.NewReader(`{}`))
				w := httptest.NewRecorder()
				h.SearchOpportunities(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestGetOpportunity(t *testing.T) {
	t.Run("rejects empty opportunity ID", func(t *testing.T) {
		h := NewOpportunityHandler(&mockEntityOpportunityClient{})
		r := httptest.NewRequest(http.MethodGet, "/opportunities/", nil)
		w := httptest.NewRecorder()
		h.GetOpportunity(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID opportunity ID", func(t *testing.T) {
		h := NewOpportunityHandler(&mockEntityOpportunityClient{})
		r := httptest.NewRequest(http.MethodGet, "/opportunities/opp-42", nil)
		r.SetPathValue("id", "opp-42")
		w := httptest.NewRecorder()
		h.GetOpportunity(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("passes ID to upstream and returns 200 with response", func(t *testing.T) {
		const opportunityID = "11111111-1111-1111-1111-111111111111"
		var capturedID string
		client := &mockEntityOpportunityClient{
			getOpportunityFn: func(_ context.Context, id string) ([]byte, error) {
				capturedID = id
				return []byte(`{"id":"` + opportunityID + `","name":"Renewal"}`), nil
			},
		}
		h := NewOpportunityHandler(client)
		r := httptest.NewRequest(http.MethodGet, "/opportunities/"+opportunityID, nil)
		r.SetPathValue("id", opportunityID)
		w := httptest.NewRecorder()
		h.GetOpportunity(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedID != opportunityID {
			t.Errorf("upstream received id %q, want %q", capturedID, opportunityID)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["id"] != opportunityID {
			t.Errorf("response id = %v, want %s", resp["id"], opportunityID)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		const opportunityID = "11111111-1111-1111-1111-111111111111"
		for _, tc := range upstreamErrors("Failed to retrieve opportunity.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityOpportunityClient{
					getOpportunityFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewOpportunityHandler(client)
				r := httptest.NewRequest(http.MethodGet, "/opportunities/"+opportunityID, nil)
				r.SetPathValue("id", opportunityID)
				w := httptest.NewRecorder()
				h.GetOpportunity(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
