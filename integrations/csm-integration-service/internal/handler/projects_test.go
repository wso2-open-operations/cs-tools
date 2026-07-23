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

func TestGetProject(t *testing.T) {
	t.Run("rejects empty project ID", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := httptest.NewRequest(http.MethodGet, "/projects/", nil)
		w := httptest.NewRecorder()
		h.GetProject(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID project ID", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := httptest.NewRequest(http.MethodGet, "/projects/proj-42", nil)
		r.SetPathValue("id", "proj-42")
		w := httptest.NewRecorder()
		h.GetProject(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("passes ID to upstream and returns 200 with response", func(t *testing.T) {
		const projectID = "11111111-1111-1111-1111-111111111111"
		var capturedID string
		client := &mockEntityProjectClient{
			getProjectFn: func(_ context.Context, id string) ([]byte, error) {
				capturedID = id
				return []byte(`{"id":"` + projectID + `","name":"platform"}`), nil
			},
		}
		h := NewProjectHandler(client)
		r := httptest.NewRequest(http.MethodGet, "/projects/"+projectID, nil)
		r.SetPathValue("id", projectID)
		w := httptest.NewRecorder()
		h.GetProject(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedID != projectID {
			t.Errorf("upstream received id %q, want %q", capturedID, projectID)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["id"] != projectID {
			t.Errorf("response id = %v, want %v", resp["id"], projectID)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to retrieve project.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityProjectClient{
					getProjectFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewProjectHandler(client)
				r := httptest.NewRequest(http.MethodGet, "/projects/11111111-1111-1111-1111-111111111111", nil)
				r.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
				w := httptest.NewRecorder()
				h.GetProject(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestSearchProjects(t *testing.T) {
	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := httptest.NewRequest(http.MethodPost, "/projects/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))
		w := httptest.NewRecorder()
		h.SearchProjects(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := httptest.NewRequest(http.MethodPost, "/projects/search", strings.NewReader(`not-json`))
		w := httptest.NewRecorder()
		h.SearchProjects(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 200 with response", func(t *testing.T) {
		const reqPayload = `{"name":"platform","key":"PLAT"}`
		var capturedBody []byte
		client := &mockEntityProjectClient{
			searchProjectsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"projects":[{"id":"proj-1","name":"platform"}],"total":1}`), nil
			},
		}
		h := NewProjectHandler(client)
		r := httptest.NewRequest(http.MethodPost, "/projects/search", strings.NewReader(reqPayload))
		w := httptest.NewRecorder()
		h.SearchProjects(w, r)

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
		for _, tc := range upstreamErrors("Failed to search projects.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityProjectClient{
					searchProjectsFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewProjectHandler(client)
				r := httptest.NewRequest(http.MethodPost, "/projects/search", strings.NewReader(`{}`))
				w := httptest.NewRecorder()
				h.SearchProjects(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestSearchProjectContacts(t *testing.T) {
	const projectID = "11111111-1111-1111-1111-111111111111"

	t.Run("rejects empty project ID", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := httptest.NewRequest(http.MethodPost, "/projects//contacts/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchProjectContacts(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID project ID", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := httptest.NewRequest(http.MethodPost, "/projects/proj-42/contacts/search", strings.NewReader(`{}`))
		r.SetPathValue("id", "proj-42")
		w := httptest.NewRecorder()
		h.SearchProjectContacts(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := httptest.NewRequest(http.MethodPost, "/projects/"+projectID+"/contacts/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))
		r.SetPathValue("id", projectID)
		w := httptest.NewRecorder()
		h.SearchProjectContacts(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := httptest.NewRequest(http.MethodPost, "/projects/"+projectID+"/contacts/search", strings.NewReader(`not-json`))
		r.SetPathValue("id", projectID)
		w := httptest.NewRecorder()
		h.SearchProjectContacts(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("allows an empty body", func(t *testing.T) {
		client := &mockEntityProjectClient{
			searchProjectContactsFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				if len(body) != 0 {
					t.Errorf("upstream body = %q, want empty", body)
				}
				return []byte(`{"contacts":[],"total":0}`), nil
			},
		}
		h := NewProjectHandler(client)
		r := httptest.NewRequest(http.MethodPost, "/projects/"+projectID+"/contacts/search", nil)
		r.SetPathValue("id", projectID)
		w := httptest.NewRecorder()
		h.SearchProjectContacts(w, r)
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("forwards body verbatim and returns upstream response", func(t *testing.T) {
		var capturedProjectID string
		var capturedBody []byte
		reqBody := `{"filters":{"searchQuery":"jane"},"pagination":{"limit":20,"offset":0}}`
		client := &mockEntityProjectClient{
			searchProjectContactsFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedProjectID = id
				capturedBody = body
				return []byte(`{"contacts":[{"name":"Jane Doe"}],"total":1,"limit":20,"offset":0}`), nil
			},
		}
		h := NewProjectHandler(client)
		r := httptest.NewRequest(http.MethodPost, "/projects/"+projectID+"/contacts/search", strings.NewReader(reqBody))
		r.SetPathValue("id", projectID)
		w := httptest.NewRecorder()
		h.SearchProjectContacts(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedProjectID != projectID {
			t.Errorf("projectID = %q, want %q", capturedProjectID, projectID)
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
		for _, tc := range upstreamErrors("Failed to search project contacts.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityProjectClient{
					searchProjectContactsFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewProjectHandler(client)
				r := httptest.NewRequest(http.MethodPost, "/projects/"+projectID+"/contacts/search", strings.NewReader(`{}`))
				r.SetPathValue("id", projectID)
				w := httptest.NewRecorder()
				h.SearchProjectContacts(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestUpdateProject(t *testing.T) {
	const projectID = "11111111-1111-1111-1111-111111111111"

	t.Run("rejects empty project ID", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := httptest.NewRequest(http.MethodPatch, "/projects/", strings.NewReader(`{"hasAgent":true}`))
		w := httptest.NewRecorder()
		h.UpdateProject(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID project ID", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := httptest.NewRequest(http.MethodPatch, "/projects/proj-42", strings.NewReader(`{"hasAgent":true}`))
		r.SetPathValue("id", "proj-42")
		w := httptest.NewRecorder()
		h.UpdateProject(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := httptest.NewRequest(http.MethodPatch, "/projects/"+projectID, strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))
		r.SetPathValue("id", projectID)
		w := httptest.NewRecorder()
		h.UpdateProject(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := httptest.NewRequest(http.MethodPatch, "/projects/"+projectID, strings.NewReader(`not-json`))
		r.SetPathValue("id", projectID)
		w := httptest.NewRecorder()
		h.UpdateProject(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty body", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := httptest.NewRequest(http.MethodPatch, "/projects/"+projectID, nil)
		r.SetPathValue("id", projectID)
		w := httptest.NewRecorder()
		h.UpdateProject(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body verbatim and returns upstream response", func(t *testing.T) {
		var capturedProjectID string
		var capturedBody []byte
		reqBody := `{"hasAgent":true,"endDateClosureState":"completed"}`
		client := &mockEntityProjectClient{
			updateProjectFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedProjectID = id
				capturedBody = body
				return []byte(`{"message":"Project updated.","project":{"id":"` + projectID + `","updatedBy":"acp-automation"}}`), nil
			},
		}
		h := NewProjectHandler(client)
		r := httptest.NewRequest(http.MethodPatch, "/projects/"+projectID, strings.NewReader(reqBody))
		r.SetPathValue("id", projectID)
		w := httptest.NewRecorder()
		h.UpdateProject(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedProjectID != projectID {
			t.Errorf("projectID = %q, want %q", capturedProjectID, projectID)
		}
		if string(capturedBody) != reqBody {
			t.Errorf("upstream body = %q, want verbatim %q", string(capturedBody), reqBody)
		}

		resp := decodeJSON[map[string]any](t, w)
		if resp["message"] != "Project updated." {
			t.Errorf("message = %v, want %v", resp["message"], "Project updated.")
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to update project.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityProjectClient{
					updateProjectFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewProjectHandler(client)
				r := httptest.NewRequest(http.MethodPatch, "/projects/"+projectID, strings.NewReader(`{"hasAgent":true}`))
				r.SetPathValue("id", projectID)
				w := httptest.NewRecorder()
				h.UpdateProject(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
