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

const testTCID = "dddddddd-eeee-ffff-0000-111111111111"

func TestCreateTimeCard(t *testing.T) {
	t.Run("rejects unauthenticated requests", func(t *testing.T) {
		h := NewTimeCardHandler(&mockEntityTimeCardClient{})
		r := httptest.NewRequest(http.MethodPost, "/time-cards", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.CreateTimeCard(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("rejects malformed JSON", func(t *testing.T) {
		h := NewTimeCardHandler(&mockEntityTimeCardClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/time-cards", strings.NewReader(`{bad`)))
		w := httptest.NewRecorder()
		h.CreateTimeCard(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("forwards body and returns 201", func(t *testing.T) {
		var captured []byte
		client := &mockEntityTimeCardClient{
			createTimeCardFn: func(_ context.Context, body []byte) ([]byte, error) {
				captured = body
				return []byte(`{"timeCard":{"id":"` + testTCID + `","state":"submitted"}}`), nil
			},
		}
		h := NewTimeCardHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/time-cards", strings.NewReader(`{"caseId":"x"}`)))
		w := httptest.NewRecorder()
		h.CreateTimeCard(w, r)

		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")
		if string(captured) != `{"caseId":"x"}` {
			t.Errorf("upstream received body %q", captured)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to create time card.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityTimeCardClient{
					createTimeCardFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewTimeCardHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/time-cards", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.CreateTimeCard(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}

func TestUpdateTimeCard(t *testing.T) {
	t.Run("rejects unauthenticated requests", func(t *testing.T) {
		h := NewTimeCardHandler(&mockEntityTimeCardClient{})
		r := httptest.NewRequest(http.MethodPatch, "/time-cards/"+testTCID, strings.NewReader(`{}`))
		r.SetPathValue("id", testTCID)
		w := httptest.NewRecorder()
		h.UpdateTimeCard(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewTimeCardHandler(&mockEntityTimeCardClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/time-cards/not-a-uuid", strings.NewReader(`{}`)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.UpdateTimeCard(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("forwards id and body for a state transition and returns 200", func(t *testing.T) {
		var capturedID string
		var capturedBody []byte
		client := &mockEntityTimeCardClient{
			updateTimeCardFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedID, capturedBody = id, body
				return []byte(`{"timeCard":{"id":"` + id + `","state":"approved"}}`), nil
			},
		}
		h := NewTimeCardHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/time-cards/"+testTCID, strings.NewReader(`{"state":"approved"}`)))
		r.SetPathValue("id", testTCID)
		w := httptest.NewRecorder()
		h.UpdateTimeCard(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedID != testTCID {
			t.Errorf("upstream received id %q, want %q", capturedID, testTCID)
		}
		if string(capturedBody) != `{"state":"approved"}` {
			t.Errorf("upstream received body %q", capturedBody)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to update time card.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityTimeCardClient{
					updateTimeCardFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewTimeCardHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPatch, "/time-cards/"+testTCID, strings.NewReader(`{}`)))
				r.SetPathValue("id", testTCID)
				w := httptest.NewRecorder()
				h.UpdateTimeCard(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}
