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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

func TestGetProject(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := httptest.NewRequest(http.MethodGet, "/projects/proj-1", nil)
		r.SetPathValue("id", "proj-1")
		w := httptest.NewRecorder()
		h.GetProject(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty project ID", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/projects/", nil))
		w := httptest.NewRecorder()
		h.GetProject(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("passes ID to upstream and returns 200 with response", func(t *testing.T) {
		var capturedID string
		client := &mockEntityProjectClient{
			getProjectFn: func(_ context.Context, id string) ([]byte, error) {
				capturedID = id
				return []byte(`{"id":"proj-42","name":"platform"}`), nil
			},
		}
		h := NewProjectHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/projects/proj-42", nil))
		r.SetPathValue("id", "proj-42")
		w := httptest.NewRecorder()
		h.GetProject(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedID != "proj-42" {
			t.Errorf("upstream received id %q, want %q", capturedID, "proj-42")
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["id"] != "proj-42" {
			t.Errorf("response id = %v, want proj-42", resp["id"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to retrieve project.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityProjectClient{
					getProjectFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewProjectHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/projects/proj-1", nil))
				r.SetPathValue("id", "proj-1")
				w := httptest.NewRecorder()
				h.GetProject(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestGetProjectMetadata(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := httptest.NewRequest(http.MethodGet, "/projects/proj-1/metadata", nil)
		r.SetPathValue("id", "proj-1")
		w := httptest.NewRecorder()
		h.GetProjectMetadata(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty project ID", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/projects//metadata", nil))
		w := httptest.NewRecorder()
		h.GetProjectMetadata(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("passes ID to upstream and returns 200 with response", func(t *testing.T) {
		var capturedID string
		client := &mockEntityProjectClient{
			getProjectMetadataFn: func(_ context.Context, id string) ([]byte, error) {
				capturedID = id
				return []byte(`{"srProductCategories":["ms","pc"]}`), nil
			},
		}
		h := NewProjectHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/projects/proj-42/metadata", nil))
		r.SetPathValue("id", "proj-42")
		w := httptest.NewRecorder()
		h.GetProjectMetadata(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedID != "proj-42" {
			t.Errorf("upstream received id %q, want %q", capturedID, "proj-42")
		}
		resp := decodeJSON[map[string]any](t, w)
		cats, ok := resp["srProductCategories"].([]any)
		if !ok || len(cats) != 2 {
			t.Errorf("response srProductCategories = %v, want [ms pc]", resp["srProductCategories"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to retrieve project metadata.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityProjectClient{
					getProjectMetadataFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewProjectHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/projects/proj-1/metadata", nil))
				r.SetPathValue("id", "proj-1")
				w := httptest.NewRecorder()
				h.GetProjectMetadata(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestSearchProjects(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := httptest.NewRequest(http.MethodPost, "/projects/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchProjects(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/projects/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchProjects(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/projects/search", strings.NewReader(`not-json`)))
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
		r := withUser(httptest.NewRequest(http.MethodPost, "/projects/search", strings.NewReader(reqPayload)))
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
		for _, tc := range upstreamErrorsGeneric("Failed to search projects.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityProjectClient{
					searchProjectsFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewProjectHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/projects/search", strings.NewReader(`{}`)))
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

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := httptest.NewRequest(http.MethodPost, "/projects/"+projectID+"/contacts/search", strings.NewReader(`{}`))
		r.SetPathValue("id", projectID)
		w := httptest.NewRecorder()
		h.SearchProjectContacts(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty project ID", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/projects//contacts/search", strings.NewReader(`{}`)))
		w := httptest.NewRecorder()
		h.SearchProjectContacts(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID project ID", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/projects/proj-42/contacts/search", strings.NewReader(`{}`)))
		r.SetPathValue("id", "proj-42")
		w := httptest.NewRecorder()
		h.SearchProjectContacts(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/projects/"+projectID+"/contacts/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		r.SetPathValue("id", projectID)
		w := httptest.NewRecorder()
		h.SearchProjectContacts(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/projects/"+projectID+"/contacts/search", strings.NewReader(`not-json`)))
		r.SetPathValue("id", projectID)
		w := httptest.NewRecorder()
		h.SearchProjectContacts(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
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
		r := withUser(httptest.NewRequest(http.MethodPost, "/projects/"+projectID+"/contacts/search", strings.NewReader(reqBody)))
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
		for _, tc := range upstreamErrorsGeneric("Failed to search project contacts.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityProjectClient{
					searchProjectContactsFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewProjectHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/projects/"+projectID+"/contacts/search", strings.NewReader(`{}`)))
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

func TestGetProjectContact(t *testing.T) {
	const projectID = "11111111-1111-1111-1111-111111111111"
	const contactID = "22222222-2222-2222-2222-222222222222"

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := httptest.NewRequest(http.MethodGet, "/projects/"+projectID+"/contacts/"+contactID, nil)
		r.SetPathValue("id", projectID)
		r.SetPathValue("contactId", contactID)
		w := httptest.NewRecorder()
		h.GetProjectContact(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID project ID", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/projects/not-a-uuid/contacts/"+contactID, nil))
		r.SetPathValue("id", "not-a-uuid")
		r.SetPathValue("contactId", contactID)
		w := httptest.NewRecorder()
		h.GetProjectContact(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID contact ID", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/projects/"+projectID+"/contacts/not-a-uuid", nil))
		r.SetPathValue("id", projectID)
		r.SetPathValue("contactId", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetProjectContact(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("passes both IDs to upstream and returns 200 with response", func(t *testing.T) {
		var capturedProjectID, capturedContactID string
		client := &mockEntityProjectClient{
			getProjectContactFn: func(_ context.Context, pID, cID string) ([]byte, error) {
				capturedProjectID = pID
				capturedContactID = cID
				return []byte(`{"id":"` + contactID + `","name":"Jane Doe"}`), nil
			},
		}
		h := NewProjectHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/projects/"+projectID+"/contacts/"+contactID, nil))
		r.SetPathValue("id", projectID)
		r.SetPathValue("contactId", contactID)
		w := httptest.NewRecorder()
		h.GetProjectContact(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedProjectID != projectID {
			t.Errorf("upstream received projectID %q, want %q", capturedProjectID, projectID)
		}
		if capturedContactID != contactID {
			t.Errorf("upstream received contactID %q, want %q", capturedContactID, contactID)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["id"] != contactID {
			t.Errorf("response id = %v, want %v", resp["id"], contactID)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to fetch the project contact.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityProjectClient{
					getProjectContactFn: func(_ context.Context, _, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewProjectHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/projects/"+projectID+"/contacts/"+contactID, nil))
				r.SetPathValue("id", projectID)
				r.SetPathValue("contactId", contactID)
				w := httptest.NewRecorder()
				h.GetProjectContact(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestUpdateProject(t *testing.T) {
	const projectID = "11111111-1111-1111-1111-111111111111"

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := httptest.NewRequest(http.MethodPatch, "/projects/"+projectID, strings.NewReader(`{"hasAgent":true}`))
		r.SetPathValue("id", projectID)
		w := httptest.NewRecorder()
		h.UpdateProject(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty project ID", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/projects/", strings.NewReader(`{"hasAgent":true}`)))
		w := httptest.NewRecorder()
		h.UpdateProject(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID project ID", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/projects/proj-42", strings.NewReader(`{"hasAgent":true}`)))
		r.SetPathValue("id", "proj-42")
		w := httptest.NewRecorder()
		h.UpdateProject(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/projects/"+projectID, strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		r.SetPathValue("id", projectID)
		w := httptest.NewRecorder()
		h.UpdateProject(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/projects/"+projectID, strings.NewReader(`not-json`)))
		r.SetPathValue("id", projectID)
		w := httptest.NewRecorder()
		h.UpdateProject(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body verbatim and returns upstream response", func(t *testing.T) {
		var capturedID string
		var capturedBody []byte
		reqBody := `{"endDateClosureState":"in_progress","complianceViolationClosureState":"completed"}`
		client := &mockEntityProjectClient{
			updateProjectFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedID = id
				capturedBody = body
				return []byte(`{"message":"Project updated.","project":{"id":"` + projectID + `","updatedBy":"user@example.com","updatedOn":"2026-01-01T00:00:00Z","closureState":"in_progress","endDateClosureState":"in_progress","invoiceDueDateClosureState":null,"complianceViolationClosureState":"completed"}}`), nil
			},
		}
		h := NewProjectHandler(client).WithAccessGuard(NewAccessGuard(testAccessConfig()))
		r := withProjectCaller(httptest.NewRequest(http.MethodPatch, "/projects/"+projectID, strings.NewReader(reqBody)), "test-cs-engineer")
		r.SetPathValue("id", projectID)
		w := httptest.NewRecorder()
		h.UpdateProject(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedID != projectID {
			t.Errorf("projectID = %q, want %q", capturedID, projectID)
		}
		if string(capturedBody) != reqBody {
			t.Errorf("upstream body = %q, want verbatim %q", string(capturedBody), reqBody)
		}

		resp := decodeJSON[map[string]any](t, w)
		if resp["message"] != "Project updated." {
			t.Errorf("message = %v, want %q", resp["message"], "Project updated.")
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
				h := NewProjectHandler(client).WithAccessGuard(NewAccessGuard(testAccessConfig()))
				r := withProjectCaller(httptest.NewRequest(http.MethodPatch, "/projects/"+projectID, strings.NewReader(`{"hasAgent":true}`)), "test-cs-engineer")
				r.SetPathValue("id", projectID)
				w := httptest.NewRecorder()
				h.UpdateProject(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})

	t.Run("closureState: denied without PermAdmin (no guard wired)", func(t *testing.T) {
		called := false
		client := &mockEntityProjectClient{
			updateProjectFn: func(context.Context, string, []byte) ([]byte, error) {
				called = true
				return []byte(`{}`), nil
			},
		}
		h := NewProjectHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/projects/"+projectID, strings.NewReader(`{"closureState":"SUSPENDED"}`)))
		r.SetPathValue("id", projectID)
		w := httptest.NewRecorder()
		h.UpdateProject(w, r)
		assertStatus(t, w, http.StatusForbidden)
		if called {
			t.Fatal("entity service must not be called for a denied closureState update")
		}
	})

	t.Run("closureState: denied for cs_engineer, who holds PermWrite but not PermAdmin", func(t *testing.T) {
		called := false
		h := NewProjectHandler(&mockEntityProjectClient{
			updateProjectFn: func(context.Context, string, []byte) ([]byte, error) {
				called = true
				return []byte(`{}`), nil
			},
		}).WithAccessGuard(NewAccessGuard(testAccessConfig()))
		r := httptest.NewRequest(http.MethodPatch, "/projects/"+projectID, strings.NewReader(`{"closureState":"SUSPENDED"}`))
		r.SetPathValue("id", projectID)
		r = r.WithContext(middleware.WithUserInfo(r.Context(), &middleware.UserInfo{Email: "cs@example.com", UserID: "u1", Roles: []string{"test-cs-engineer"}}))
		w := httptest.NewRecorder()
		h.UpdateProject(w, r)
		assertStatus(t, w, http.StatusForbidden)
		if called {
			t.Fatal("entity service must not be called for a denied closureState update")
		}
	})

	t.Run("closureState: allowed for admin", func(t *testing.T) {
		var capturedBody []byte
		h := NewProjectHandler(&mockEntityProjectClient{
			updateProjectFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"Project updated.","project":{}}`), nil
			},
		}).WithAccessGuard(NewAccessGuard(testAccessConfig()))
		const reqBody = `{"closureState":"SUSPENDED"}`
		r := httptest.NewRequest(http.MethodPatch, "/projects/"+projectID, strings.NewReader(reqBody))
		r.SetPathValue("id", projectID)
		r = r.WithContext(middleware.WithUserInfo(r.Context(), &middleware.UserInfo{Email: "admin@example.com", UserID: "u2", Roles: []string{"test-admin"}}))
		w := httptest.NewRecorder()
		h.UpdateProject(w, r)
		assertStatus(t, w, http.StatusOK)
		if string(capturedBody) != reqBody {
			t.Errorf("upstream received %q, want verbatim %q", capturedBody, reqBody)
		}
	})

	t.Run("closureState: case-insensitive key spelling still requires PermAdmin", func(t *testing.T) {
		// entity-service's own JSON decoding (encoding/json) matches a key to
		// a struct field without regard to case, so this guard must catch a
		// differently-cased spelling too -- see projectUpdateTargetsClosureState's
		// own doc comment.
		called := false
		h := NewProjectHandler(&mockEntityProjectClient{
			updateProjectFn: func(context.Context, string, []byte) ([]byte, error) {
				called = true
				return []byte(`{}`), nil
			},
		}).WithAccessGuard(NewAccessGuard(testAccessConfig()))
		r := httptest.NewRequest(http.MethodPatch, "/projects/"+projectID, strings.NewReader(`{"ClosureState":"SUSPENDED"}`))
		r.SetPathValue("id", projectID)
		r = r.WithContext(middleware.WithUserInfo(r.Context(), &middleware.UserInfo{Email: "cs@example.com", UserID: "u1", Roles: []string{"test-cs-engineer"}}))
		w := httptest.NewRecorder()
		h.UpdateProject(w, r)
		assertStatus(t, w, http.StatusForbidden)
		if called {
			t.Fatal("entity service must not be called for a denied closureState update")
		}
	})

	t.Run("non-closureState fields stay reachable for cs_engineer with a guard wired", func(t *testing.T) {
		var capturedBody []byte
		h := NewProjectHandler(&mockEntityProjectClient{
			updateProjectFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"Project updated.","project":{}}`), nil
			},
		}).WithAccessGuard(NewAccessGuard(testAccessConfig()))
		const reqBody = `{"hasAgent":true}`
		r := httptest.NewRequest(http.MethodPatch, "/projects/"+projectID, strings.NewReader(reqBody))
		r.SetPathValue("id", projectID)
		r = r.WithContext(middleware.WithUserInfo(r.Context(), &middleware.UserInfo{Email: "cs@example.com", UserID: "u1", Roles: []string{"test-cs-engineer"}}))
		w := httptest.NewRecorder()
		h.UpdateProject(w, r)
		assertStatus(t, w, http.StatusOK)
		if string(capturedBody) != reqBody {
			t.Errorf("upstream received %q, want verbatim %q", capturedBody, reqBody)
		}
	})

	// Field-level permissions: the route floor (PermUpdateProject) lets
	// onboarding_lead through, so UpdateProject itself decides per field.
	t.Run("field permissions by caller role", func(t *testing.T) {
		tests := []struct {
			name  string
			roles []string
			body  string
			want  int
		}{
			{"onboarding_lead may set onboardingStatus", []string{"test-onboarding-lead"}, `{"onboardingStatus":"In-Progress"}`, http.StatusOK},
			{"onboarding_lead may set onboardingOwner", []string{"test-onboarding-lead"}, `{"onboardingOwner":"00000000-0000-0000-0000-000000000000"}`, http.StatusOK},
			{"onboarding_lead may set both onboarding fields", []string{"test-onboarding-lead"}, `{"onboardingStatus":"Completed","onboardingOwner":"00000000-0000-0000-0000-000000000000"}`, http.StatusOK},
			{"onboarding key match is case-insensitive", []string{"test-onboarding-lead"}, `{"OnboardingStatus":"Completed"}`, http.StatusOK},
			{"admin may set onboarding fields", []string{"test-admin"}, `{"onboardingStatus":"Completed"}`, http.StatusOK},
			{"cs_engineer alone is denied onboardingStatus", []string{"test-cs-engineer"}, `{"onboardingStatus":"Completed"}`, http.StatusForbidden},
			{"cs_engineer alone is denied onboardingOwner", []string{"test-cs-engineer"}, `{"onboardingOwner":"00000000-0000-0000-0000-000000000000"}`, http.StatusForbidden},
			{"cs_engineer is denied a differently-cased onboarding key", []string{"test-cs-engineer"}, `{"ONBOARDINGOWNER":null}`, http.StatusForbidden},
			{"onboarding_lead may set onboardingStartDate", []string{"test-onboarding-lead"}, `{"onboardingStartDate":"2026-01-05"}`, http.StatusOK},
			{"onboarding_lead may set onboardingGoLivePlanDate", []string{"test-onboarding-lead"}, `{"onboardingGoLivePlanDate":"2026-03-01"}`, http.StatusOK},
			{"onboarding_lead may set onboardingGoLiveDate", []string{"test-onboarding-lead"}, `{"onboardingGoLiveDate":"2026-03-15"}`, http.StatusOK},
			{"onboarding_lead may set onboardingExpiryDate", []string{"test-onboarding-lead"}, `{"onboardingExpiryDate":"2026-12-31"}`, http.StatusOK},
			{"admin may set onboardingStartDate", []string{"test-admin"}, `{"onboardingStartDate":"2026-01-05"}`, http.StatusOK},
			{"admin may set onboardingGoLivePlanDate", []string{"test-admin"}, `{"onboardingGoLivePlanDate":"2026-03-01"}`, http.StatusOK},
			{"admin may set onboardingGoLiveDate", []string{"test-admin"}, `{"onboardingGoLiveDate":"2026-03-15"}`, http.StatusOK},
			{"admin may set onboardingExpiryDate", []string{"test-admin"}, `{"onboardingExpiryDate":"2026-12-31"}`, http.StatusOK},
			{"cs_engineer alone is denied onboardingStartDate", []string{"test-cs-engineer"}, `{"onboardingStartDate":"2026-01-05"}`, http.StatusForbidden},
			{"cs_engineer alone is denied onboardingGoLivePlanDate", []string{"test-cs-engineer"}, `{"onboardingGoLivePlanDate":"2026-03-01"}`, http.StatusForbidden},
			{"cs_engineer alone is denied onboardingGoLiveDate", []string{"test-cs-engineer"}, `{"onboardingGoLiveDate":"2026-03-15"}`, http.StatusForbidden},
			{"cs_engineer alone is denied onboardingExpiryDate", []string{"test-cs-engineer"}, `{"onboardingExpiryDate":"2026-12-31"}`, http.StatusForbidden},
			{"cs_engineer is denied a differently-cased date key", []string{"test-cs-engineer"}, `{"ONBOARDINGSTARTDATE":"2026-01-05"}`, http.StatusForbidden},
			{"onboarding_lead may set a differently-cased date key", []string{"test-onboarding-lead"}, `{"ONBOARDINGSTARTDATE":"2026-01-05"}`, http.StatusOK},
			{"onboarding_lead alone is denied a date mixed with hasAgent", []string{"test-onboarding-lead"}, `{"onboardingGoLiveDate":"2026-03-15","hasAgent":true}`, http.StatusForbidden},
			{"onboarding_lead may mix a date with onboardingStatus", []string{"test-onboarding-lead"}, `{"onboardingStatus":"In-Progress","onboardingExpiryDate":"2026-12-31"}`, http.StatusOK},
			{"cs_engineer alone is denied a date mixed with hasAgent", []string{"test-cs-engineer"}, `{"onboardingGoLiveDate":"2026-03-15","hasAgent":true}`, http.StatusForbidden},
			{"cs_engineer plus onboarding_lead may mix a date with hasAgent", []string{"test-cs-engineer", "test-onboarding-lead"}, `{"onboardingGoLiveDate":"2026-03-15","hasAgent":true}`, http.StatusOK},
			{"cs_engineer keeps other fields", []string{"test-cs-engineer"}, `{"hasAgent":true}`, http.StatusOK},
			{"admin keeps other fields", []string{"test-admin"}, `{"hasAgent":true}`, http.StatusOK},
			{"onboarding_lead alone is denied other fields", []string{"test-onboarding-lead"}, `{"hasAgent":true}`, http.StatusForbidden},
			{"onboarding_lead alone is denied closure sub-states", []string{"test-onboarding-lead"}, `{"endDateClosureState":"in_progress"}`, http.StatusForbidden},
			{"onboarding_lead alone is denied a mixed body", []string{"test-onboarding-lead"}, `{"onboardingStatus":"Completed","hasAgent":true}`, http.StatusForbidden},
			{"cs_engineer alone is denied a mixed body", []string{"test-cs-engineer"}, `{"onboardingStatus":"Completed","hasAgent":true}`, http.StatusForbidden},
			{"cs_engineer plus onboarding_lead may send a mixed body", []string{"test-cs-engineer", "test-onboarding-lead"}, `{"onboardingStatus":"Completed","hasAgent":true}`, http.StatusOK},
			{"admin may send a mixed body", []string{"test-admin"}, `{"onboardingStatus":"Completed","hasAgent":true}`, http.StatusOK},
			{"closureState stays admin-only for onboarding_lead plus cs_engineer", []string{"test-cs-engineer", "test-onboarding-lead"}, `{"closureState":"SUSPENDED"}`, http.StatusForbidden},
			{"closureState allowed for admin", []string{"test-admin"}, `{"closureState":"SUSPENDED"}`, http.StatusOK},
			{"a caller with no write or onboarding role is denied", []string{"test-viewer"}, `{"hasAgent":true}`, http.StatusForbidden},
			{"an empty body needs write access", []string{"test-onboarding-lead"}, `{}`, http.StatusForbidden},
			{"an empty body reaches entity-service for cs_engineer", []string{"test-cs-engineer"}, `{}`, http.StatusOK},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				called := false
				h := NewProjectHandler(&mockEntityProjectClient{
					updateProjectFn: func(context.Context, string, []byte) ([]byte, error) {
						called = true
						return []byte(`{"message":"Project updated.","project":{}}`), nil
					},
				}).WithAccessGuard(NewAccessGuard(testAccessConfig()))
				r := withProjectCaller(httptest.NewRequest(http.MethodPatch, "/projects/"+projectID, strings.NewReader(tc.body)), tc.roles...)
				r.SetPathValue("id", projectID)
				w := httptest.NewRecorder()
				h.UpdateProject(w, r)
				assertStatus(t, w, tc.want)
				if called != (tc.want == http.StatusOK) {
					t.Errorf("entity service called = %v, want %v", called, tc.want == http.StatusOK)
				}
			})
		}
	})

	t.Run("no guard wired fails closed for non-closureState fields too", func(t *testing.T) {
		called := false
		h := NewProjectHandler(&mockEntityProjectClient{
			updateProjectFn: func(context.Context, string, []byte) ([]byte, error) {
				called = true
				return []byte(`{}`), nil
			},
		})
		for _, body := range []string{`{"hasAgent":true}`, `{"onboardingStatus":"Completed"}`} {
			r := withProjectCaller(httptest.NewRequest(http.MethodPatch, "/projects/"+projectID, strings.NewReader(body)), "test-admin")
			r.SetPathValue("id", projectID)
			w := httptest.NewRecorder()
			h.UpdateProject(w, r)
			assertStatus(t, w, http.StatusForbidden)
		}
		if called {
			t.Fatal("entity service must not be called when no guard is wired")
		}
	})
}

// withProjectCaller attaches a caller whose token carries roles.
func withProjectCaller(r *http.Request, roles ...string) *http.Request {
	return r.WithContext(middleware.WithUserInfo(r.Context(), &middleware.UserInfo{Email: "jane.doe@example.com", UserID: "u1", Roles: roles}))
}

// encoding/json matches keys with Unicode simple folding, so the key checks
// must too: a long-s spelling still names the protected field to the decoder.
func TestProjectUpdateKeyChecksFoldLikeDecoder(t *testing.T) {
	if !projectUpdateTargetsClosureState([]byte(`{"closureſtate":"CLOSED"}`)) {
		t.Error("long-s spelling of closureState must be detected")
	}
	onb, other := projectUpdateTargets([]byte(`{"onboardingſtatus":"In-Progress"}`))
	if !onb || other {
		t.Errorf("long-s onboardingStatus: onboarding=%v other=%v, want true/false", onb, other)
	}
	onb, other = projectUpdateTargets([]byte(`{"ONBOARDINGSTARTDATE":"2026-01-05"}`))
	if !onb || other {
		t.Errorf("upper-case onboardingStartDate: onboarding=%v other=%v, want true/false", onb, other)
	}
	var req struct {
		ClosureState string `json:"closureState"`
	}
	if err := json.Unmarshal([]byte(`{"closureſtate":"CLOSED"}`), &req); err != nil || req.ClosureState != "CLOSED" {
		t.Fatalf("premise: decoder should fold long-s (got %q, err %v)", req.ClosureState, err)
	}
}
