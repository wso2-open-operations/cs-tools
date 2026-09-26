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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockEntityScheduleClient stands in for entity-service, recording what the
// handler forwarded so a test can assert the body reached it untouched.
type mockEntityScheduleClient struct {
	catalogueFn   func(ctx context.Context) ([]byte, error)
	assignmentsFn func(ctx context.Context, body []byte) ([]byte, error)
	absencesFn    func(ctx context.Context, body []byte) ([]byte, error)
	onDutyFn      func(ctx context.Context, at string) ([]byte, error)
}

func (m *mockEntityScheduleClient) GetScheduleCatalogue(ctx context.Context) ([]byte, error) {
	if m.catalogueFn != nil {
		return m.catalogueFn(ctx)
	}
	return []byte(`{"zones":[],"shifts":[],"absenceKinds":[]}`), nil
}

func (m *mockEntityScheduleClient) SearchScheduleAssignments(ctx context.Context, body []byte) ([]byte, error) {
	if m.assignmentsFn != nil {
		return m.assignmentsFn(ctx, body)
	}
	return []byte(`{"assignments":[],"count":0}`), nil
}

func (m *mockEntityScheduleClient) SearchScheduleAbsences(ctx context.Context, body []byte) ([]byte, error) {
	if m.absencesFn != nil {
		return m.absencesFn(ctx, body)
	}
	return []byte(`{"absences":[],"count":0}`), nil
}

func (m *mockEntityScheduleClient) GetScheduleOnDuty(ctx context.Context, at string) ([]byte, error) {
	if m.onDutyFn != nil {
		return m.onDutyFn(ctx, at)
	}
	return []byte(`{"assignments":[],"count":0}`), nil
}

func TestSearchScheduleAssignments(t *testing.T) {
	t.Run("requires an authenticated user", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		r := httptest.NewRequest(http.MethodPost, "/team-schedule/assignments/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchScheduleAssignments(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects a body over the size limit", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/team-schedule/assignments/search",
			strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchScheduleAssignments(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
	})

	t.Run("rejects a body that is not JSON", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/team-schedule/assignments/search",
			strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SearchScheduleAssignments(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("forwards the search to entity-service unchanged", func(t *testing.T) {
		const payload = `{"from":"2026-09-21","to":"2026-09-27","family":"SRE","includeOvernight":true}`
		var captured []byte
		client := &mockEntityScheduleClient{
			assignmentsFn: func(_ context.Context, body []byte) ([]byte, error) {
				captured = body
				return []byte(`{"assignments":[{"id":"a"}],"count":1}`), nil
			},
		}
		h := NewScheduleHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/team-schedule/assignments/search", strings.NewReader(payload)))
		w := httptest.NewRecorder()
		h.SearchScheduleAssignments(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if string(captured) != payload {
			t.Fatalf("body was altered in transit:\n got %s\nwant %s", captured, payload)
		}
		if !strings.Contains(w.Body.String(), `"count":1`) {
			t.Fatalf("upstream response did not reach the caller: %s", w.Body.String())
		}
	})

	t.Run("surfaces an upstream failure rather than pretending it worked", func(t *testing.T) {
		client := &mockEntityScheduleClient{
			assignmentsFn: func(context.Context, []byte) ([]byte, error) {
				return nil, errors.New("upstream is down")
			},
		}
		h := NewScheduleHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/team-schedule/assignments/search", strings.NewReader(`{}`)))
		w := httptest.NewRecorder()
		h.SearchScheduleAssignments(w, r)
		if w.Code == http.StatusOK {
			t.Fatal("an upstream error must not come back as 200")
		}
	})
}

func TestGetScheduleCatalogue(t *testing.T) {
	t.Run("requires an authenticated user", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		w := httptest.NewRecorder()
		h.GetScheduleCatalogue(w, httptest.NewRequest(http.MethodGet, "/team-schedule/catalogue", nil))
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("returns the catalogue", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{
			catalogueFn: func(context.Context) ([]byte, error) {
				return []byte(`{"zones":[{"code":"TZ1"}],"shifts":[],"absenceKinds":[]}`), nil
			},
		})
		w := httptest.NewRecorder()
		h.GetScheduleCatalogue(w, withUser(httptest.NewRequest(http.MethodGet, "/team-schedule/catalogue", nil)))
		assertStatus(t, w, http.StatusOK)
		if !strings.Contains(w.Body.String(), "TZ1") {
			t.Fatalf("catalogue did not reach the caller: %s", w.Body.String())
		}
	})
}

func TestGetScheduleOnDuty(t *testing.T) {
	t.Run("passes the instant through when one is asked for", func(t *testing.T) {
		var gotAt string
		h := NewScheduleHandler(&mockEntityScheduleClient{
			onDutyFn: func(_ context.Context, at string) ([]byte, error) {
				gotAt = at
				return []byte(`{"assignments":[],"count":0}`), nil
			},
		})
		w := httptest.NewRecorder()
		h.GetScheduleOnDuty(w, withUser(httptest.NewRequest(http.MethodGet,
			"/team-schedule/on-duty?at=2026-09-21T22%3A15%3A00Z", nil)))
		assertStatus(t, w, http.StatusOK)
		if gotAt != "2026-09-21T22:15:00Z" {
			t.Fatalf("want the instant forwarded verbatim, got %q", gotAt)
		}
	})

	t.Run("asks about now when no instant is given", func(t *testing.T) {
		var gotAt = "unset"
		h := NewScheduleHandler(&mockEntityScheduleClient{
			onDutyFn: func(_ context.Context, at string) ([]byte, error) {
				gotAt = at
				return []byte(`{"assignments":[],"count":0}`), nil
			},
		})
		w := httptest.NewRecorder()
		h.GetScheduleOnDuty(w, withUser(httptest.NewRequest(http.MethodGet, "/team-schedule/on-duty", nil)))
		assertStatus(t, w, http.StatusOK)
		if gotAt != "" {
			t.Fatalf("want an empty instant so the service defaults to now, got %q", gotAt)
		}
	})
}

func TestSearchScheduleAbsences(t *testing.T) {
	t.Run("forwards the search and returns the response", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{
			absencesFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return []byte(`{"absences":[{"id":"x"}],"count":1}`), nil
			},
		})
		w := httptest.NewRecorder()
		h.SearchScheduleAbsences(w, withUser(httptest.NewRequest(http.MethodPost,
			"/team-schedule/absences/search", strings.NewReader(`{"from":"2026-09-21","to":"2026-09-21"}`))))
		assertStatus(t, w, http.StatusOK)
		if !strings.Contains(w.Body.String(), `"count":1`) {
			t.Fatalf("upstream response did not reach the caller: %s", w.Body.String())
		}
	})
}
