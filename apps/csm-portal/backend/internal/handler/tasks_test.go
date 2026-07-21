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

const testTaskCaseID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
const testTaskID = "11111111-1111-1111-1111-111111111111"

func TestSearchCaseTasks(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewTaskHandler(&mockEntityTaskClient{})
		r := httptest.NewRequest(http.MethodPost, "/cases/"+testTaskCaseID+"/tasks/search", strings.NewReader(`{}`))
		r.SetPathValue("caseId", testTaskCaseID)
		w := httptest.NewRecorder()
		h.SearchCaseTasks(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed case UUID", func(t *testing.T) {
		h := NewTaskHandler(&mockEntityTaskClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/not-a-uuid/tasks/search", strings.NewReader(`{}`)))
		r.SetPathValue("caseId", "not-a-uuid")
		w := httptest.NewRecorder()
		h.SearchCaseTasks(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty case id", func(t *testing.T) {
		h := NewTaskHandler(&mockEntityTaskClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases//tasks/search", strings.NewReader(`{}`)))
		w := httptest.NewRecorder()
		h.SearchCaseTasks(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewTaskHandler(&mockEntityTaskClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testTaskCaseID+"/tasks/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		r.SetPathValue("caseId", testTaskCaseID)
		w := httptest.NewRecorder()
		h.SearchCaseTasks(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewTaskHandler(&mockEntityTaskClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testTaskCaseID+"/tasks/search", strings.NewReader(`not-json`)))
		r.SetPathValue("caseId", testTaskCaseID)
		w := httptest.NewRecorder()
		h.SearchCaseTasks(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards case id and body verbatim to upstream", func(t *testing.T) {
		var capturedCaseID string
		var capturedBody []byte
		reqBody := `{"pagination":{"limit":20,"offset":0}}`
		client := &mockEntityTaskClient{
			searchCaseTasksFn: func(_ context.Context, caseID string, body []byte) ([]byte, error) {
				capturedCaseID = caseID
				capturedBody = body
				return []byte(`{"tasks":[{"id":"` + testTaskID + `"}],"total":1,"limit":20,"offset":0}`), nil
			},
		}
		h := NewTaskHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testTaskCaseID+"/tasks/search", strings.NewReader(reqBody)))
		r.SetPathValue("caseId", testTaskCaseID)
		w := httptest.NewRecorder()
		h.SearchCaseTasks(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedCaseID != testTaskCaseID {
			t.Errorf("upstream received caseID %q, want %q", capturedCaseID, testTaskCaseID)
		}
		if string(capturedBody) != reqBody {
			t.Errorf("upstream body = %q, want verbatim %q", string(capturedBody), reqBody)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["total"] != float64(1) {
			t.Errorf("total = %v, want 1", resp["total"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to retrieve case tasks.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityTaskClient{
					searchCaseTasksFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewTaskHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testTaskCaseID+"/tasks/search", strings.NewReader(`{}`)))
				r.SetPathValue("caseId", testTaskCaseID)
				w := httptest.NewRecorder()
				h.SearchCaseTasks(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestGetTask(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewTaskHandler(&mockEntityTaskClient{})
		r := httptest.NewRequest(http.MethodGet, "/tasks/"+testTaskID, nil)
		r.SetPathValue("id", testTaskID)
		w := httptest.NewRecorder()
		h.GetTask(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewTaskHandler(&mockEntityTaskClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/tasks/not-a-uuid", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetTask(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty id", func(t *testing.T) {
		h := NewTaskHandler(&mockEntityTaskClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/tasks/", nil))
		w := httptest.NewRecorder()
		h.GetTask(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards id to upstream and returns 200", func(t *testing.T) {
		var capturedID string
		client := &mockEntityTaskClient{
			getTaskFn: func(_ context.Context, id string) ([]byte, error) {
				capturedID = id
				return []byte(`{"id":"` + testTaskID + `","subject":"Investigate outage","state":"in_progress"}`), nil
			},
		}
		h := NewTaskHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/tasks/"+testTaskID, nil))
		r.SetPathValue("id", testTaskID)
		w := httptest.NewRecorder()
		h.GetTask(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedID != testTaskID {
			t.Errorf("upstream received id %q, want %q", capturedID, testTaskID)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["subject"] != "Investigate outage" {
			t.Errorf("response subject = %v, want %q", resp["subject"], "Investigate outage")
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to retrieve task.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityTaskClient{
					getTaskFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewTaskHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/tasks/"+testTaskID, nil))
				r.SetPathValue("id", testTaskID)
				w := httptest.NewRecorder()
				h.GetTask(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
