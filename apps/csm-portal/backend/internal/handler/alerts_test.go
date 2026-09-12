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
	"testing"
)

const testAlertID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

func TestGetAlert(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewAlertHandler(&mockEntityAlertClient{})
		r := httptest.NewRequest(http.MethodGet, "/alerts/"+testAlertID, nil)
		r.SetPathValue("id", testAlertID)
		w := httptest.NewRecorder()
		h.GetAlert(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewAlertHandler(&mockEntityAlertClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/alerts/not-a-uuid", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetAlert(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty id", func(t *testing.T) {
		h := NewAlertHandler(&mockEntityAlertClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/alerts/", nil))
		w := httptest.NewRecorder()
		h.GetAlert(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards id to upstream and returns 200", func(t *testing.T) {
		var capturedID string
		client := &mockEntityAlertClient{
			getAlertFn: func(_ context.Context, id string) ([]byte, error) {
				capturedID = id
				return []byte(`{"id":"` + testAlertID + `","number":"ALT0001","severity":"CRITICAL"}`), nil
			},
		}
		h := NewAlertHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/alerts/"+testAlertID, nil))
		r.SetPathValue("id", testAlertID)
		w := httptest.NewRecorder()
		h.GetAlert(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedID != testAlertID {
			t.Errorf("upstream received id %q, want %q", capturedID, testAlertID)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["number"] != "ALT0001" {
			t.Errorf("response number = %v, want ALT0001", resp["number"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to retrieve alert.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityAlertClient{
					getAlertFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewAlertHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/alerts/"+testAlertID, nil))
				r.SetPathValue("id", testAlertID)
				w := httptest.NewRecorder()
				h.GetAlert(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestGetSmartAlert(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewAlertHandler(&mockEntityAlertClient{})
		r := httptest.NewRequest(http.MethodGet, "/smart-alerts/"+testAlertID, nil)
		r.SetPathValue("id", testAlertID)
		w := httptest.NewRecorder()
		h.GetSmartAlert(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewAlertHandler(&mockEntityAlertClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/smart-alerts/not-a-uuid", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetSmartAlert(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty id", func(t *testing.T) {
		h := NewAlertHandler(&mockEntityAlertClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/smart-alerts/", nil))
		w := httptest.NewRecorder()
		h.GetSmartAlert(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards id to upstream and returns 200", func(t *testing.T) {
		var capturedID string
		client := &mockEntityAlertClient{
			getSmartAlertFn: func(_ context.Context, id string) ([]byte, error) {
				capturedID = id
				return []byte(`{"id":"` + testAlertID + `","alertStatus":"OPEN","severity":"CRITICAL"}`), nil
			},
		}
		h := NewAlertHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/smart-alerts/"+testAlertID, nil))
		r.SetPathValue("id", testAlertID)
		w := httptest.NewRecorder()
		h.GetSmartAlert(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedID != testAlertID {
			t.Errorf("upstream received id %q, want %q", capturedID, testAlertID)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["alertStatus"] != "OPEN" {
			t.Errorf("response alertStatus = %v, want OPEN", resp["alertStatus"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to retrieve smart alert.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityAlertClient{
					getSmartAlertFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewAlertHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/smart-alerts/"+testAlertID, nil))
				r.SetPathValue("id", testAlertID)
				w := httptest.NewRecorder()
				h.GetSmartAlert(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
