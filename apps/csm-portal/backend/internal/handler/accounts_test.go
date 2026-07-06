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

func TestGetAccount(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := httptest.NewRequest(http.MethodGet, "/accounts/acc-1", nil)
		r.SetPathValue("id", "acc-1")
		w := httptest.NewRecorder()
		h.GetAccount(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty account ID", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/accounts/", nil))
		w := httptest.NewRecorder()
		h.GetAccount(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID account ID", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/accounts/acc-42", nil))
		r.SetPathValue("id", "acc-42")
		w := httptest.NewRecorder()
		h.GetAccount(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("passes ID to upstream and returns 200 with response", func(t *testing.T) {
		const accountID = "11111111-1111-1111-1111-111111111111"
		var capturedID string
		client := &mockEntityAccountClient{
			getAccountFn: func(_ context.Context, id string) ([]byte, error) {
				capturedID = id
				return []byte(`{"id":"` + accountID + `","name":"WSO2"}`), nil
			},
		}
		h := NewAccountHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/accounts/"+accountID, nil))
		r.SetPathValue("id", accountID)
		w := httptest.NewRecorder()
		h.GetAccount(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedID != accountID {
			t.Errorf("upstream received id %q, want %q", capturedID, accountID)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["id"] != accountID {
			t.Errorf("response id = %v, want %s", resp["id"], accountID)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		const accountID = "11111111-1111-1111-1111-111111111111"
		for _, tc := range upstreamErrors("Failed to retrieve account.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityAccountClient{
					getAccountFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewAccountHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/accounts/"+accountID, nil))
				r.SetPathValue("id", accountID)
				w := httptest.NewRecorder()
				h.GetAccount(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestSearchAccounts(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := httptest.NewRequest(http.MethodPost, "/accounts/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchAccounts(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/accounts/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchAccounts(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/accounts/search", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SearchAccounts(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 200 with response", func(t *testing.T) {
		const reqPayload = `{"name":"WSO2","limit":10}`
		var capturedBody []byte
		client := &mockEntityAccountClient{
			searchAccountsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"accounts":[{"id":"acc-1","name":"WSO2"}],"total":1}`), nil
			},
		}
		h := NewAccountHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/accounts/search", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.SearchAccounts(w, r)

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
		for _, tc := range upstreamErrors("Failed to search accounts.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityAccountClient{
					searchAccountsFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewAccountHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/accounts/search", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.SearchAccounts(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
