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

func TestCreateIncident(t *testing.T) {
	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))
		w := httptest.NewRecorder()
		h.CreateIncident(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(`not-json`))
		w := httptest.NewRecorder()
		h.CreateIncident(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty body", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := httptest.NewRequest(http.MethodPost, "/incidents", nil)
		w := httptest.NewRecorder()
		h.CreateIncident(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body verbatim and returns upstream response", func(t *testing.T) {
		var capturedBody []byte
		reqBody := `{"callerId":"11111111-1111-1111-1111-111111111111","category":"INQUIRY","serviceId":"22222222-2222-2222-2222-222222222222","impact":"LOW","urgency":"LOW","subject":"Test incident"}`
		client := &mockEntityIncidentClient{
			createIncidentFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"Incident created.","incident":{"id":"33333333-3333-3333-3333-333333333333","number":"INC0001234"}}`), nil
			},
		}
		h := NewIncidentHandler(client)
		r := httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(reqBody))
		w := httptest.NewRecorder()
		h.CreateIncident(w, r)

		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")

		if string(capturedBody) != reqBody {
			t.Errorf("upstream body = %q, want verbatim %q", string(capturedBody), reqBody)
		}

		resp := decodeJSON[map[string]any](t, w)
		if resp["message"] != "Incident created." {
			t.Errorf("message = %v, want %v", resp["message"], "Incident created.")
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to create incident.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityIncidentClient{
					createIncidentFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewIncidentHandler(client)
				r := httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(`{}`))
				w := httptest.NewRecorder()
				h.CreateIncident(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestSearchIncidents(t *testing.T) {
	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := httptest.NewRequest(http.MethodPost, "/incidents/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))
		w := httptest.NewRecorder()
		h.SearchIncidents(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := httptest.NewRequest(http.MethodPost, "/incidents/search", strings.NewReader(`not-json`))
		w := httptest.NewRecorder()
		h.SearchIncidents(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty body", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := httptest.NewRequest(http.MethodPost, "/incidents/search", nil)
		w := httptest.NewRecorder()
		h.SearchIncidents(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body verbatim and returns upstream response", func(t *testing.T) {
		var capturedBody []byte
		reqBody := `{"filters":{"searchQuery":"disk space"},"sortBy":{"field":"createdOn","order":"desc"},"pagination":{"limit":20,"offset":0}}`
		client := &mockEntityIncidentClient{
			searchIncidentsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"incidents":[{"id":"33333333-3333-3333-3333-333333333333","number":"INC0001234"}],"total":1,"limit":20,"offset":0}`), nil
			},
		}
		h := NewIncidentHandler(client)
		r := httptest.NewRequest(http.MethodPost, "/incidents/search", strings.NewReader(reqBody))
		w := httptest.NewRecorder()
		h.SearchIncidents(w, r)

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
		for _, tc := range upstreamErrors("Failed to search incidents.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityIncidentClient{
					searchIncidentsFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewIncidentHandler(client)
				r := httptest.NewRequest(http.MethodPost, "/incidents/search", strings.NewReader(`{}`))
				w := httptest.NewRecorder()
				h.SearchIncidents(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
