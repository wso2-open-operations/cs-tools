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

const testTaskSlaID = "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6"

func TestSearchTaskSlas(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewTaskSlaHandler(&mockEntityTaskSlaClient{})
		r := httptest.NewRequest(http.MethodPost, "/slas/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchTaskSlas(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewTaskSlaHandler(&mockEntityTaskSlaClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/slas/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchTaskSlas(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewTaskSlaHandler(&mockEntityTaskSlaClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/slas/search", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SearchTaskSlas(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream unchanged and returns raw 200 response", func(t *testing.T) {
		const payload = `{"filters":{"caseId":"case-1"},"pagination":{"limit":20,"offset":0}}`
		const upstream = `{"taskSlas":[{"id":"sla-1"}],"total":1,"limit":20,"offset":0}`
		var capturedBody []byte
		client := &mockEntityTaskSlaClient{
			searchTaskSlasFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(upstream), nil
			},
		}
		h := NewTaskSlaHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/slas/search", strings.NewReader(payload)))
		w := httptest.NewRecorder()
		h.SearchTaskSlas(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != payload {
			t.Errorf("upstream received body %q, want %q", capturedBody, payload)
		}
		if w.Body.String() != upstream {
			t.Errorf("response body = %q, want raw passthrough %q", w.Body.String(), upstream)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to search task SLAs.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityTaskSlaClient{
					searchTaskSlasFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewTaskSlaHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/slas/search", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.SearchTaskSlas(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestGetTaskSla(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewTaskSlaHandler(&mockEntityTaskSlaClient{})
		r := httptest.NewRequest(http.MethodGet, "/slas/"+testTaskSlaID, nil)
		r.SetPathValue("id", testTaskSlaID)
		w := httptest.NewRecorder()
		h.GetTaskSla(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty id", func(t *testing.T) {
		h := NewTaskSlaHandler(&mockEntityTaskSlaClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/slas/", nil))
		w := httptest.NewRecorder()
		h.GetTaskSla(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards non-UUID id to upstream and returns raw 200 response", func(t *testing.T) {
		const upstream = `{"id":"` + testTaskSlaID + `","definition":"Response time","stage":"in_progress"}`
		var capturedID string
		client := &mockEntityTaskSlaClient{
			getTaskSlaFn: func(_ context.Context, id string) ([]byte, error) {
				capturedID = id
				return []byte(upstream), nil
			},
		}
		h := NewTaskSlaHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/slas/"+testTaskSlaID, nil))
		r.SetPathValue("id", testTaskSlaID)
		w := httptest.NewRecorder()
		h.GetTaskSla(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedID != testTaskSlaID {
			t.Errorf("upstream received id %q, want %q", capturedID, testTaskSlaID)
		}
		if w.Body.String() != upstream {
			t.Errorf("response body = %q, want raw passthrough %q", w.Body.String(), upstream)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to retrieve task SLA.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityTaskSlaClient{
					getTaskSlaFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewTaskSlaHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/slas/"+testTaskSlaID, nil))
				r.SetPathValue("id", testTaskSlaID)
				w := httptest.NewRecorder()
				h.GetTaskSla(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
