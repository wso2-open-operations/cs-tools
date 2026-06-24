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

const testCRID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

func TestGetChangeRequest(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := httptest.NewRequest(http.MethodGet, "/change-requests/"+testCRID, nil)
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.GetChangeRequest(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/change-requests/not-a-uuid", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetChangeRequest(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty id", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/change-requests/", nil))
		w := httptest.NewRecorder()
		h.GetChangeRequest(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards id to upstream and returns 200", func(t *testing.T) {
		var capturedID string
		client := &mockEntityChangeRequestClient{
			getChangeRequestFn: func(_ context.Context, id string) ([]byte, error) {
				capturedID = id
				return []byte(`{"id":"` + testCRID + `","number":"CHG001","state":"scheduled"}`), nil
			},
		}
		h := NewChangeRequestHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/change-requests/"+testCRID, nil))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.GetChangeRequest(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedID != testCRID {
			t.Errorf("upstream received id %q, want %q", capturedID, testCRID)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["number"] != "CHG001" {
			t.Errorf("response number = %v, want CHG001", resp["number"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to retrieve change request.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityChangeRequestClient{
					getChangeRequestFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewChangeRequestHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/change-requests/"+testCRID, nil))
				r.SetPathValue("id", testCRID)
				w := httptest.NewRecorder()
				h.GetChangeRequest(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestSearchChangeRequests(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := httptest.NewRequest(http.MethodPost, "/change-requests/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchChangeRequests(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchChangeRequests(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/search", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SearchChangeRequests(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 200", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityChangeRequestClient{
			searchChangeRequestsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"changeRequests":[{"id":"cr-1"}],"total":1,"limit":20,"offset":0}`), nil
			},
		}
		h := NewChangeRequestHandler(client)
		const payload = `{"filters":{"states":["scheduled"]},"pagination":{"limit":20,"offset":0}}`
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/search", strings.NewReader(payload)))
		w := httptest.NewRecorder()
		h.SearchChangeRequests(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if string(capturedBody) != payload {
			t.Errorf("upstream received body %q, want %q", capturedBody, payload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["total"] != float64(1) {
			t.Errorf("total = %v, want 1", resp["total"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to search change requests.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityChangeRequestClient{
					searchChangeRequestsFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewChangeRequestHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/search", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.SearchChangeRequests(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

