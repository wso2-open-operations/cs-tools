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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
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
		for _, tc := range upstreamErrorsGeneric("Failed to create time card.") {
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

	t.Run("a body whose state cannot be decoded is 400, not a skipped permission check", func(t *testing.T) {
		for _, body := range []string{`{"state":5}`, `{"state":5,"state":"approved"}`, `[]`, `"x"`} {
			called := false
			h := NewTimeCardHandler(&mockEntityTimeCardClient{
				updateTimeCardFn: func(context.Context, string, []byte) ([]byte, error) {
					called = true
					return []byte(`{}`), nil
				},
			}).WithAccessGuard(NewAccessGuard(testAccessConfig()))
			r := withCsEngineerUser(httptest.NewRequest(http.MethodPatch, "/time-cards/"+testTCID, strings.NewReader(body)))
			r.SetPathValue("id", testTCID)
			w := httptest.NewRecorder()
			h.UpdateTimeCard(w, r)
			if w.Code != http.StatusBadRequest {
				t.Errorf("body %s: status = %d, want 400", body, w.Code)
			}
			if called {
				t.Errorf("body %s: upstream must not be called", body)
			}
		}
	})

	t.Run("forwards id and body for a state transition and returns 200 for an approver", func(t *testing.T) {
		var capturedID string
		var capturedBody []byte
		client := &mockEntityTimeCardClient{
			updateTimeCardFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedID, capturedBody = id, body
				return []byte(`{"timeCard":{"id":"` + id + `","state":"approved"}}`), nil
			},
		}
		h := NewTimeCardHandler(client).WithAccessGuard(NewAccessGuard(testAccessConfig()))
		r := httptest.NewRequest(http.MethodPatch, "/time-cards/"+testTCID, strings.NewReader(`{"state":"approved"}`))
		r = r.WithContext(middleware.WithUserInfo(r.Context(), &middleware.UserInfo{Email: "approver@example.com", UserID: "u1", Roles: []string{"test-timecard-approver"}}))
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

	t.Run("state transition denied without PermApproveTimeCard (no guard wired)", func(t *testing.T) {
		called := false
		client := &mockEntityTimeCardClient{
			updateTimeCardFn: func(context.Context, string, []byte) ([]byte, error) {
				called = true
				return []byte(`{}`), nil
			},
		}
		h := NewTimeCardHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/time-cards/"+testTCID, strings.NewReader(`{"state":"approved"}`)))
		r.SetPathValue("id", testTCID)
		w := httptest.NewRecorder()
		h.UpdateTimeCard(w, r)
		assertStatus(t, w, http.StatusForbidden)
		if called {
			t.Fatal("entity service must not be called for a denied state transition")
		}
	})

	t.Run("state transition denied for a CS engineer (not a timecard approver)", func(t *testing.T) {
		h := NewTimeCardHandler(&mockEntityTimeCardClient{
			updateTimeCardFn: func(context.Context, string, []byte) ([]byte, error) { return []byte(`{}`), nil },
		}).WithAccessGuard(NewAccessGuard(testAccessConfig()))
		r := httptest.NewRequest(http.MethodPatch, "/time-cards/"+testTCID, strings.NewReader(`{"state":"rejected","leadComment":"needs more detail"}`))
		r = r.WithContext(middleware.WithUserInfo(r.Context(), &middleware.UserInfo{Email: "cs@example.com", UserID: "u2", Roles: []string{"test-cs-engineer"}}))
		r.SetPathValue("id", testTCID)
		w := httptest.NewRecorder()
		h.UpdateTimeCard(w, r)
		assertStatus(t, w, http.StatusForbidden)
	})

	t.Run("state transition allowed for admin", func(t *testing.T) {
		h := NewTimeCardHandler(&mockEntityTimeCardClient{
			updateTimeCardFn: func(_ context.Context, id string, _ []byte) ([]byte, error) {
				return []byte(`{"timeCard":{"id":"` + id + `","state":"approved"}}`), nil
			},
		}).WithAccessGuard(NewAccessGuard(testAccessConfig()))
		r := httptest.NewRequest(http.MethodPatch, "/time-cards/"+testTCID, strings.NewReader(`{"state":"approved"}`))
		r = r.WithContext(middleware.WithUserInfo(r.Context(), &middleware.UserInfo{Email: "admin@example.com", UserID: "u3", Roles: []string{"test-admin"}}))
		r.SetPathValue("id", testTCID)
		w := httptest.NewRecorder()
		h.UpdateTimeCard(w, r)
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("plain field edit (no state) does not require PermApproveTimeCard", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityTimeCardClient{
			updateTimeCardFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"timeCard":{"id":"` + testTCID + `","state":"submitted"}}`), nil
			},
		}
		// No WithAccessGuard at all -- must still succeed, since a nil-body
		// state field never triggers the check in the first place.
		h := NewTimeCardHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/time-cards/"+testTCID, strings.NewReader(`{"workLogComment":"updated"}`)))
		r.SetPathValue("id", testTCID)
		w := httptest.NewRecorder()
		h.UpdateTimeCard(w, r)
		assertStatus(t, w, http.StatusOK)
		if string(capturedBody) != `{"workLogComment":"updated"}` {
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

func TestDeleteTimeCard(t *testing.T) {
	t.Run("rejects unauthenticated requests", func(t *testing.T) {
		h := NewTimeCardHandler(&mockEntityTimeCardClient{})
		r := httptest.NewRequest(http.MethodDelete, "/time-cards/"+testTCID, nil)
		r.SetPathValue("id", testTCID)
		w := httptest.NewRecorder()
		h.DeleteTimeCard(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewTimeCardHandler(&mockEntityTimeCardClient{})
		r := withUser(httptest.NewRequest(http.MethodDelete, "/time-cards/not-a-uuid", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.DeleteTimeCard(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("forwards id and returns 200", func(t *testing.T) {
		var capturedID string
		client := &mockEntityTimeCardClient{
			deleteTimeCardFn: func(_ context.Context, id string) ([]byte, error) {
				capturedID = id
				return []byte(`{"message":"Time card deleted"}`), nil
			},
		}
		h := NewTimeCardHandler(client)
		r := withUser(httptest.NewRequest(http.MethodDelete, "/time-cards/"+testTCID, nil))
		r.SetPathValue("id", testTCID)
		w := httptest.NewRecorder()
		h.DeleteTimeCard(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedID != testTCID {
			t.Errorf("upstream received id %q, want %q", capturedID, testTCID)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["message"] != "Time card deleted" {
			t.Errorf("message = %v, want %q", resp["message"], "Time card deleted")
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to delete time card.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityTimeCardClient{
					deleteTimeCardFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewTimeCardHandler(client)
				r := withUser(httptest.NewRequest(http.MethodDelete, "/time-cards/"+testTCID, nil))
				r.SetPathValue("id", testTCID)
				w := httptest.NewRecorder()
				h.DeleteTimeCard(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}
