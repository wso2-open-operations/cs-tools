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

func TestCreateAlertIncidentMapping(t *testing.T) {
	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewAlertIncidentMappingHandler(&mockEntityAlertIncidentMappingClient{})
		r := httptest.NewRequest(http.MethodPost, "/alert-incident-mappings", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))
		w := httptest.NewRecorder()
		h.CreateAlertIncidentMapping(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewAlertIncidentMappingHandler(&mockEntityAlertIncidentMappingClient{})
		r := httptest.NewRequest(http.MethodPost, "/alert-incident-mappings", strings.NewReader(`not-json`))
		w := httptest.NewRecorder()
		h.CreateAlertIncidentMapping(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty body", func(t *testing.T) {
		h := NewAlertIncidentMappingHandler(&mockEntityAlertIncidentMappingClient{})
		r := httptest.NewRequest(http.MethodPost, "/alert-incident-mappings", nil)
		w := httptest.NewRecorder()
		h.CreateAlertIncidentMapping(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body verbatim and returns upstream response", func(t *testing.T) {
		var capturedBody []byte
		reqBody := `{"alertNumber":"ALERT-0001","source":"prometheus","alertStatus":"firing","incidentId":"33333333-3333-3333-3333-333333333333"}`
		client := &mockEntityAlertIncidentMappingClient{
			createAlertIncidentMappingFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"id":"44444444-4444-4444-4444-444444444444","alertNumber":"ALERT-0001","source":"prometheus","alertStatus":"firing","incidentId":"33333333-3333-3333-3333-333333333333","createdAt":"2026-08-29T00:00:00Z"}`), nil
			},
		}
		h := NewAlertIncidentMappingHandler(client)
		r := httptest.NewRequest(http.MethodPost, "/alert-incident-mappings", strings.NewReader(reqBody))
		w := httptest.NewRecorder()
		h.CreateAlertIncidentMapping(w, r)

		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")

		if string(capturedBody) != reqBody {
			t.Errorf("upstream body = %q, want verbatim %q", string(capturedBody), reqBody)
		}

		resp := decodeJSON[map[string]any](t, w)
		if resp["alertNumber"] != "ALERT-0001" {
			t.Errorf("alertNumber = %v, want %v", resp["alertNumber"], "ALERT-0001")
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to create alert-incident mapping.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityAlertIncidentMappingClient{
					createAlertIncidentMappingFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewAlertIncidentMappingHandler(client)
				r := httptest.NewRequest(http.MethodPost, "/alert-incident-mappings", strings.NewReader(`{}`))
				w := httptest.NewRecorder()
				h.CreateAlertIncidentMapping(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestLookupAlertIncidentMappings(t *testing.T) {
	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewAlertIncidentMappingHandler(&mockEntityAlertIncidentMappingClient{})
		r := httptest.NewRequest(http.MethodPost, "/alert-incident-mappings/lookup", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))
		w := httptest.NewRecorder()
		h.LookupAlertIncidentMappings(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewAlertIncidentMappingHandler(&mockEntityAlertIncidentMappingClient{})
		r := httptest.NewRequest(http.MethodPost, "/alert-incident-mappings/lookup", strings.NewReader(`not-json`))
		w := httptest.NewRecorder()
		h.LookupAlertIncidentMappings(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty body", func(t *testing.T) {
		h := NewAlertIncidentMappingHandler(&mockEntityAlertIncidentMappingClient{})
		r := httptest.NewRequest(http.MethodPost, "/alert-incident-mappings/lookup", nil)
		w := httptest.NewRecorder()
		h.LookupAlertIncidentMappings(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body verbatim and returns upstream response", func(t *testing.T) {
		var capturedBody []byte
		reqBody := `{"source":"prometheus","uniqueIdentifier":"alert-uid-123"}`
		client := &mockEntityAlertIncidentMappingClient{
			lookupAlertIncidentMappingsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"mappings":[{"id":"44444444-4444-4444-4444-444444444444","alertNumber":"ALERT-0001","source":"prometheus","alertStatus":"firing","incidentId":"33333333-3333-3333-3333-333333333333","createdAt":"2026-08-29T00:00:00Z"}]}`), nil
			},
		}
		h := NewAlertIncidentMappingHandler(client)
		r := httptest.NewRequest(http.MethodPost, "/alert-incident-mappings/lookup", strings.NewReader(reqBody))
		w := httptest.NewRecorder()
		h.LookupAlertIncidentMappings(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if string(capturedBody) != reqBody {
			t.Errorf("upstream body = %q, want verbatim %q", string(capturedBody), reqBody)
		}

		resp := decodeJSON[map[string]any](t, w)
		mappings, ok := resp["mappings"].([]any)
		if !ok || len(mappings) != 1 {
			t.Errorf("mappings = %v, want a single-element array", resp["mappings"])
		}
	})

	t.Run("empty match returns 200 with empty mappings array", func(t *testing.T) {
		client := &mockEntityAlertIncidentMappingClient{
			lookupAlertIncidentMappingsFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return []byte(`{"mappings":[]}`), nil
			},
		}
		h := NewAlertIncidentMappingHandler(client)
		r := httptest.NewRequest(http.MethodPost, "/alert-incident-mappings/lookup", strings.NewReader(`{"source":"prometheus","uniqueIdentifier":"no-match"}`))
		w := httptest.NewRecorder()
		h.LookupAlertIncidentMappings(w, r)

		assertStatus(t, w, http.StatusOK)
		resp := decodeJSON[map[string]any](t, w)
		mappings, ok := resp["mappings"].([]any)
		if !ok || len(mappings) != 0 {
			t.Errorf("mappings = %v, want empty array", resp["mappings"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to look up alert-incident mappings.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityAlertIncidentMappingClient{
					lookupAlertIncidentMappingsFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewAlertIncidentMappingHandler(client)
				r := httptest.NewRequest(http.MethodPost, "/alert-incident-mappings/lookup", strings.NewReader(`{}`))
				w := httptest.NewRecorder()
				h.LookupAlertIncidentMappings(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
