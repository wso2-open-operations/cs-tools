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

const testProblemID = "aaaaaaaa-bbbb-cccc-dddd-ffffffffffff"

func TestGetProblem(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewProblemHandler(&mockEntityProblemClient{})
		r := httptest.NewRequest(http.MethodGet, "/problems/"+testProblemID, nil)
		r.SetPathValue("id", testProblemID)
		w := httptest.NewRecorder()
		h.GetProblem(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewProblemHandler(&mockEntityProblemClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/problems/not-a-uuid", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetProblem(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty id", func(t *testing.T) {
		h := NewProblemHandler(&mockEntityProblemClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/problems/", nil))
		w := httptest.NewRecorder()
		h.GetProblem(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards id to upstream and returns 200", func(t *testing.T) {
		var capturedID string
		client := &mockEntityProblemClient{
			getProblemFn: func(_ context.Context, id string) ([]byte, error) {
				capturedID = id
				return []byte(`{"id":"` + testProblemID + `","number":"PRB0001","state":"assess"}`), nil
			},
		}
		h := NewProblemHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/problems/"+testProblemID, nil))
		r.SetPathValue("id", testProblemID)
		w := httptest.NewRecorder()
		h.GetProblem(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedID != testProblemID {
			t.Errorf("upstream received id %q, want %q", capturedID, testProblemID)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["number"] != "PRB0001" {
			t.Errorf("response number = %v, want PRB0001", resp["number"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to retrieve problem.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityProblemClient{
					getProblemFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewProblemHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/problems/"+testProblemID, nil))
				r.SetPathValue("id", testProblemID)
				w := httptest.NewRecorder()
				h.GetProblem(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestSearchProblems(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewProblemHandler(&mockEntityProblemClient{})
		r := httptest.NewRequest(http.MethodPost, "/problems/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchProblems(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewProblemHandler(&mockEntityProblemClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/problems/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchProblems(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewProblemHandler(&mockEntityProblemClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/problems/search", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SearchProblems(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 200 with response", func(t *testing.T) {
		const reqPayload = `{"filters":{"searchQuery":"outage"},"pagination":{"limit":10,"offset":0}}`
		var capturedBody []byte
		client := &mockEntityProblemClient{
			searchProblemsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"problems":[{"id":"11111111-1111-1111-1111-111111111111","number":"PRB0001"}],"total":1}`), nil
			},
		}
		h := NewProblemHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/problems/search", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.SearchProblems(w, r)

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
		for _, tc := range upstreamErrors("Failed to search problems.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityProblemClient{
					searchProblemsFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewProblemHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/problems/search", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.SearchProblems(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
