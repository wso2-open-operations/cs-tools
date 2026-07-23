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

func TestSearchIncidents(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := httptest.NewRequest(http.MethodPost, "/incidents/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchIncidents(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/incidents/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchIncidents(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/incidents/search", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SearchIncidents(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects unknown priority enum value", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/incidents/search", strings.NewReader(`{"filters":{"priorities":["URGENT"]}}`)))
		w := httptest.NewRecorder()
		h.SearchIncidents(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID parentIds entry", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/incidents/search", strings.NewReader(`{"filters":{"parentIds":["not-a-uuid"]}}`)))
		w := httptest.NewRecorder()
		h.SearchIncidents(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid sortBy field", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/incidents/search", strings.NewReader(`{"sortBy":{"field":"subject","order":"asc"}}`)))
		w := httptest.NewRecorder()
		h.SearchIncidents(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid sortBy order", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/incidents/search", strings.NewReader(`{"sortBy":{"field":"createdOn","order":"sideways"}}`)))
		w := httptest.NewRecorder()
		h.SearchIncidents(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 200 with response", func(t *testing.T) {
		const reqPayload = `{"filters":{"searchQuery":"outage","priorities":["CRITICAL"]},"pagination":{"limit":10,"offset":0}}`
		var capturedBody []byte
		client := &mockEntityIncidentClient{
			searchIncidentsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"incidents":[{"id":"11111111-1111-1111-1111-111111111111","number":"INC0001"}],"total":1}`), nil
			},
		}
		h := NewIncidentHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/incidents/search", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.SearchIncidents(w, r)

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
		for _, tc := range upstreamErrors("Failed to search incidents.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityIncidentClient{
					searchIncidentsFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewIncidentHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/incidents/search", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.SearchIncidents(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestCreateIncident(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.CreateIncident(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.CreateIncident(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.CreateIncident(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects missing required fields", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(`{"category":"SECURITY","impact":"HIGH","urgency":"HIGH","subject":"Something broke"}`)))
		w := httptest.NewRecorder()
		h.CreateIncident(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects unknown category enum value", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(`{"callerId":"11111111-1111-1111-1111-111111111111","category":"NOT_A_CATEGORY","serviceId":"22222222-2222-2222-2222-222222222222","impact":"HIGH","urgency":"HIGH","subject":"Something broke"}`)))
		w := httptest.NewRecorder()
		h.CreateIncident(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID callerId", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(`{"callerId":"not-a-uuid","category":"SECURITY","serviceId":"22222222-2222-2222-2222-222222222222","impact":"HIGH","urgency":"HIGH","subject":"Something broke"}`)))
		w := httptest.NewRecorder()
		h.CreateIncident(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID parentIncidentId", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(`{"callerId":"11111111-1111-1111-1111-111111111111","category":"SECURITY","serviceId":"22222222-2222-2222-2222-222222222222","impact":"HIGH","urgency":"HIGH","subject":"Something broke","parentIncidentId":"not-a-uuid"}`)))
		w := httptest.NewRecorder()
		h.CreateIncident(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 201 with response", func(t *testing.T) {
		const reqPayload = `{"callerId":"11111111-1111-1111-1111-111111111111","category":"SECURITY","serviceId":"22222222-2222-2222-2222-222222222222","impact":"HIGH","urgency":"HIGH","subject":"Something broke"}`
		var capturedBody []byte
		client := &mockEntityIncidentClient{
			createIncidentFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"incident created","incident":{"id":"33333333-3333-3333-3333-333333333333","number":"INC0002"}}`), nil
			},
		}
		h := NewIncidentHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.CreateIncident(w, r)

		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["message"] != "incident created" {
			t.Errorf("message = %v, want %q", resp["message"], "incident created")
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		const validPayload = `{"callerId":"11111111-1111-1111-1111-111111111111","category":"SECURITY","serviceId":"22222222-2222-2222-2222-222222222222","impact":"HIGH","urgency":"HIGH","subject":"Something broke"}`
		for _, tc := range upstreamErrors("Failed to create incident.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityIncidentClient{
					createIncidentFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewIncidentHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(validPayload)))
				w := httptest.NewRecorder()
				h.CreateIncident(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestGetIncident(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := httptest.NewRequest(http.MethodGet, "/incidents/11111111-1111-1111-1111-111111111111", nil)
		r.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
		w := httptest.NewRecorder()
		h.GetIncident(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty incident ID", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/incidents/", nil))
		w := httptest.NewRecorder()
		h.GetIncident(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID incident ID", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/incidents/inc-42", nil))
		r.SetPathValue("id", "inc-42")
		w := httptest.NewRecorder()
		h.GetIncident(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("passes ID to upstream and returns 200 with response", func(t *testing.T) {
		const incidentID = "11111111-1111-1111-1111-111111111111"
		var capturedID string
		client := &mockEntityIncidentClient{
			getIncidentFn: func(_ context.Context, id string) ([]byte, error) {
				capturedID = id
				return []byte(`{"id":"` + incidentID + `","number":"INC0001"}`), nil
			},
		}
		h := NewIncidentHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/incidents/"+incidentID, nil))
		r.SetPathValue("id", incidentID)
		w := httptest.NewRecorder()
		h.GetIncident(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedID != incidentID {
			t.Errorf("upstream received id %q, want %q", capturedID, incidentID)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["number"] != "INC0001" {
			t.Errorf("number = %v, want %q", resp["number"], "INC0001")
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		const incidentID = "11111111-1111-1111-1111-111111111111"
		for _, tc := range upstreamErrors("Failed to retrieve incident.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityIncidentClient{
					getIncidentFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewIncidentHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/incidents/"+incidentID, nil))
				r.SetPathValue("id", incidentID)
				w := httptest.NewRecorder()
				h.GetIncident(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestPatchIncident(t *testing.T) {
	const incidentID = "11111111-1111-1111-1111-111111111111"

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := httptest.NewRequest(http.MethodPatch, "/incidents/"+incidentID, strings.NewReader(`{"subject":"Updated"}`))
		r.SetPathValue("id", incidentID)
		w := httptest.NewRecorder()
		h.PatchIncident(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty incident ID", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/incidents/", strings.NewReader(`{"subject":"Updated"}`)))
		w := httptest.NewRecorder()
		h.PatchIncident(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID incident ID", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/incidents/inc-42", strings.NewReader(`{"subject":"Updated"}`)))
		r.SetPathValue("id", "inc-42")
		w := httptest.NewRecorder()
		h.PatchIncident(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/incidents/"+incidentID, strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		r.SetPathValue("id", incidentID)
		w := httptest.NewRecorder()
		h.PatchIncident(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/incidents/"+incidentID, strings.NewReader(`not-json`)))
		r.SetPathValue("id", incidentID)
		w := httptest.NewRecorder()
		h.PatchIncident(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty JSON object", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/incidents/"+incidentID, strings.NewReader(`{}`)))
		r.SetPathValue("id", incidentID)
		w := httptest.NewRecorder()
		h.PatchIncident(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects unknown state enum value", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/incidents/"+incidentID, strings.NewReader(`{"state":"DONE"}`)))
		r.SetPathValue("id", incidentID)
		w := httptest.NewRecorder()
		h.PatchIncident(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID assignedEngineerId", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/incidents/"+incidentID, strings.NewReader(`{"assignedEngineerId":"not-a-uuid"}`)))
		r.SetPathValue("id", incidentID)
		w := httptest.NewRecorder()
		h.PatchIncident(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("accepts explicit null to clear an optional reference field", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{
			patchIncidentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				return []byte(`{"message":"incident updated","incident":{"id":"` + incidentID + `"}}`), nil
			},
		})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/incidents/"+incidentID, strings.NewReader(`{"parentIncidentId":null}`)))
		r.SetPathValue("id", incidentID)
		w := httptest.NewRecorder()
		h.PatchIncident(w, r)
		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards ID and body to upstream and returns 200 with response", func(t *testing.T) {
		const reqPayload = `{"state":"RESOLVED","resolutionCode":"Solved (Work Around)"}`
		var capturedID string
		var capturedBody []byte
		client := &mockEntityIncidentClient{
			patchIncidentFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedID = id
				capturedBody = body
				return []byte(`{"message":"incident updated","incident":{"id":"` + incidentID + `","state":"RESOLVED"}}`), nil
			},
		}
		h := NewIncidentHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/incidents/"+incidentID, strings.NewReader(reqPayload)))
		r.SetPathValue("id", incidentID)
		w := httptest.NewRecorder()
		h.PatchIncident(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedID != incidentID {
			t.Errorf("upstream received id %q, want %q", capturedID, incidentID)
		}
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["message"] != "incident updated" {
			t.Errorf("message = %v, want %q", resp["message"], "incident updated")
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to update incident.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityIncidentClient{
					patchIncidentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewIncidentHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPatch, "/incidents/"+incidentID, strings.NewReader(`{"subject":"Updated"}`)))
				r.SetPathValue("id", incidentID)
				w := httptest.NewRecorder()
				h.PatchIncident(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestCreateIncidentComment(t *testing.T) {
	const incidentID = "11111111-1111-1111-1111-111111111111"

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := httptest.NewRequest(http.MethodPost, "/incidents/"+incidentID+"/comments", strings.NewReader(`{"type":"comment","content":"hi"}`))
		r.SetPathValue("id", incidentID)
		w := httptest.NewRecorder()
		h.CreateIncidentComment(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/incidents/not-a-uuid/comments", strings.NewReader(`{"type":"comment","content":"hi"}`)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.CreateIncidentComment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/incidents/"+incidentID+"/comments", strings.NewReader(`not-json`)))
		r.SetPathValue("id", incidentID)
		w := httptest.NewRecorder()
		h.CreateIncidentComment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("injects referenceId and referenceType and forwards to the generic comment endpoint", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityIncidentClient{
			getIncidentFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"id":"` + incidentID + `"}`), nil
			},
			createCommentFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"Comment created.","comment":{"id":"22222222-2222-2222-2222-222222222222","createdOn":"2026-01-01T00:00:00Z","createdBy":"user@example.com"}}`), nil
			},
		}
		h := NewIncidentHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/incidents/"+incidentID+"/comments", strings.NewReader(`{"type":"comment","content":"hi"}`)))
		r.SetPathValue("id", incidentID)
		w := httptest.NewRecorder()
		h.CreateIncidentComment(w, r)

		assertStatus(t, w, http.StatusCreated)
		if !strings.Contains(string(capturedBody), `"referenceId":"`+incidentID+`"`) {
			t.Errorf("expected referenceId to be injected, got %q", capturedBody)
		}
		if !strings.Contains(string(capturedBody), `"referenceType":"incident"`) {
			t.Errorf("expected referenceType incident to be injected, got %q", capturedBody)
		}
	})

	t.Run("upstream GetIncident error is mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to create incident comment.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityIncidentClient{
					getIncidentFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewIncidentHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/incidents/"+incidentID+"/comments", strings.NewReader(`{"type":"comment","content":"hi"}`)))
				r.SetPathValue("id", incidentID)
				w := httptest.NewRecorder()
				h.CreateIncidentComment(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}

func TestSearchIncidentComments(t *testing.T) {
	const incidentID = "11111111-1111-1111-1111-111111111111"

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewIncidentHandler(&mockEntityIncidentClient{})
		r := httptest.NewRequest(http.MethodPost, "/incidents/"+incidentID+"/comments/search", strings.NewReader(`{}`))
		r.SetPathValue("id", incidentID)
		w := httptest.NewRecorder()
		h.SearchIncidentComments(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("injects referenceId and referenceType and forwards to the generic search endpoint", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityIncidentClient{
			searchCommentsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"comments":[],"total":0,"limit":20,"offset":0}`), nil
			},
		}
		h := NewIncidentHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/incidents/"+incidentID+"/comments/search", strings.NewReader(`{"pagination":{"offset":0,"limit":20}}`)))
		r.SetPathValue("id", incidentID)
		w := httptest.NewRecorder()
		h.SearchIncidentComments(w, r)

		assertStatus(t, w, http.StatusOK)
		if !strings.Contains(string(capturedBody), `"referenceId":"`+incidentID+`"`) {
			t.Errorf("expected referenceId to be injected, got %q", capturedBody)
		}
		if !strings.Contains(string(capturedBody), `"referenceType":"incident"`) {
			t.Errorf("expected referenceType incident to be injected, got %q", capturedBody)
		}
	})
}
