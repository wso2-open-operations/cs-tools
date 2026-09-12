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

const testIncidentTaskID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

func TestSearchIncidentTasks(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewIncidentTaskHandler(&mockEntityIncidentTaskClient{})
		r := httptest.NewRequest(http.MethodPost, "/incident-tasks/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchIncidentTasks(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewIncidentTaskHandler(&mockEntityIncidentTaskClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/incident-tasks/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchIncidentTasks(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewIncidentTaskHandler(&mockEntityIncidentTaskClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/incident-tasks/search", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SearchIncidentTasks(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 200 with response", func(t *testing.T) {
		const reqPayload = `{"filters":{"assignmentGroupId":["6c3db375-1b1c-b2d0-a002-c9d3604bcb0c"]},"pagination":{"limit":10,"offset":0}}`
		var capturedBody []byte
		client := &mockEntityIncidentTaskClient{
			searchIncidentTasksFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"incidentTasks":[{"id":"` + testIncidentTaskID + `","number":"SCTASK0001"}],"total":1}`), nil
			},
		}
		h := NewIncidentTaskHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/incident-tasks/search", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.SearchIncidentTasks(w, r)

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
		for _, tc := range upstreamErrorsGeneric("Failed to search incident tasks.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityIncidentTaskClient{
					searchIncidentTasksFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewIncidentTaskHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/incident-tasks/search", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.SearchIncidentTasks(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestGetIncidentTask(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewIncidentTaskHandler(&mockEntityIncidentTaskClient{})
		r := httptest.NewRequest(http.MethodGet, "/incident-tasks/"+testIncidentTaskID, nil)
		r.SetPathValue("id", testIncidentTaskID)
		w := httptest.NewRecorder()
		h.GetIncidentTask(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewIncidentTaskHandler(&mockEntityIncidentTaskClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/incident-tasks/not-a-uuid", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetIncidentTask(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty id", func(t *testing.T) {
		h := NewIncidentTaskHandler(&mockEntityIncidentTaskClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/incident-tasks/", nil))
		w := httptest.NewRecorder()
		h.GetIncidentTask(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards id to upstream and returns 200", func(t *testing.T) {
		var capturedID string
		client := &mockEntityIncidentTaskClient{
			getIncidentTaskFn: func(_ context.Context, id string) ([]byte, error) {
				capturedID = id
				return []byte(`{"id":"` + testIncidentTaskID + `","number":"SCTASK0001"}`), nil
			},
		}
		h := NewIncidentTaskHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/incident-tasks/"+testIncidentTaskID, nil))
		r.SetPathValue("id", testIncidentTaskID)
		w := httptest.NewRecorder()
		h.GetIncidentTask(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedID != testIncidentTaskID {
			t.Errorf("upstream received id %q, want %q", capturedID, testIncidentTaskID)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["number"] != "SCTASK0001" {
			t.Errorf("response number = %v, want SCTASK0001", resp["number"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to retrieve incident task.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityIncidentTaskClient{
					getIncidentTaskFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewIncidentTaskHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/incident-tasks/"+testIncidentTaskID, nil))
				r.SetPathValue("id", testIncidentTaskID)
				w := httptest.NewRecorder()
				h.GetIncidentTask(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestAggregateIncidentTasks(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewIncidentTaskHandler(&mockEntityIncidentTaskClient{})
		r := httptest.NewRequest(http.MethodPost, "/incident-tasks/aggregate", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.AggregateIncidentTasks(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewIncidentTaskHandler(&mockEntityIncidentTaskClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/incident-tasks/aggregate", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.AggregateIncidentTasks(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewIncidentTaskHandler(&mockEntityIncidentTaskClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/incident-tasks/aggregate", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.AggregateIncidentTasks(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 200 with response", func(t *testing.T) {
		const reqPayload = `{"filters":{},"groupBy":"state","maxGroups":12}`
		var capturedBody []byte
		client := &mockEntityIncidentTaskClient{
			aggregateIncidentTasksFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"groups":[{"key":"open","label":"Open","count":2}],"othersCount":0,"totalRecords":2}`), nil
			},
		}
		h := NewIncidentTaskHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/incident-tasks/aggregate", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.AggregateIncidentTasks(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["totalRecords"] != float64(2) {
			t.Errorf("totalRecords = %v, want 2", resp["totalRecords"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to aggregate incident tasks.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityIncidentTaskClient{
					aggregateIncidentTasksFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewIncidentTaskHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/incident-tasks/aggregate", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.AggregateIncidentTasks(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
