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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// upstreamErrorCases is the table used by every handler that calls mapUpstreamError.
// It covers all four explicit apierror mappings plus an unmapped code and a plain error.
type upstreamErrorCase struct {
	name         string
	err          error
	wantCode     int
	wantMsg      string
}

func upstreamErrors(fallback string) []upstreamErrorCase {
	return []upstreamErrorCase{
		{"apierror 401", &apierror.Error{StatusCode: http.StatusUnauthorized}, http.StatusUnauthorized, ErrMsgUnauthorized},
		{"apierror 403", &apierror.Error{StatusCode: http.StatusForbidden}, http.StatusForbidden, ErrMsgForbidden},
		{"apierror 404", &apierror.Error{StatusCode: http.StatusNotFound}, http.StatusNotFound, ErrMsgNotFound},
		{"apierror 400", &apierror.Error{StatusCode: http.StatusBadRequest}, http.StatusBadRequest, ErrMsgBadRequest},
		{"apierror unmapped (418)", &apierror.Error{StatusCode: http.StatusTeapot}, http.StatusInternalServerError, fallback},
		{"non-apierror error", errors.New("upstream connection refused"), http.StatusInternalServerError, fallback},
	}
}

// ----- CreateCase -----

func TestCreateCase(t *testing.T) {
	// createdBy is NOT in the payload — it is resolved server-side via users/search.
	const validPayload = `{"projectId":"proj-1","deploymentId":"dep-1","deployedProductId":"dp-1","subject":"Login failure","description":"Users cannot log in","priority":"high","issueType":"error"}`
	const resolvedUserID = "entity-user-uuid-42"

	userSearchOK := func(_ context.Context, _ []byte) ([]byte, error) {
		return []byte(`{"users":[{"id":"` + resolvedUserID + `","email":"agent@example.com"}],"total":1}`), nil
	}

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPost, "/cases", strings.NewReader(validPayload))
		w := httptest.NewRecorder()
		h.CreateCase(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.CreateCase(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.CreateCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("resolves createdBy from users/search and injects it", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityCaseClient{
			searchUsersFn: userSearchOK,
			createCaseFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"id":"case-1","subject":"Login failure","state":"open"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases", strings.NewReader(validPayload)))
		w := httptest.NewRecorder()
		h.CreateCase(w, r)

		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")

		var sent map[string]json.RawMessage
		if err := json.Unmarshal(capturedBody, &sent); err != nil {
			t.Fatalf("upstream received invalid JSON: %v", err)
		}
		var gotID string
		if err := json.Unmarshal(sent["createdBy"], &gotID); err != nil || gotID != resolvedUserID {
			t.Errorf("upstream createdBy = %q, want %q", gotID, resolvedUserID)
		}
		var gotProjectID string
		if err := json.Unmarshal(sent["projectId"], &gotProjectID); err != nil || gotProjectID != "proj-1" {
			t.Errorf("upstream projectId = %q, want \"proj-1\" (original fields must be preserved)", gotProjectID)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["id"] != "case-1" {
			t.Errorf("response id = %v, want case-1", resp["id"])
		}
	})

	t.Run("returns 403 when user not found in entity service", func(t *testing.T) {
		client := &mockEntityCaseClient{
			searchUsersFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return []byte(`{"users":[],"total":0}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases", strings.NewReader(validPayload)))
		w := httptest.NewRecorder()
		h.CreateCase(w, r)
		assertStatus(t, w, http.StatusForbidden)
		assertErrorMessage(t, w, ErrMsgForbidden)
		assertContentType(t, w, "application/json")
	})

	t.Run("returns 403 when search result email does not match token email", func(t *testing.T) {
		client := &mockEntityCaseClient{
			searchUsersFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return []byte(`{"users":[{"id":"other-uuid","email":"alice-admin@example.com"}],"total":1}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases", strings.NewReader(validPayload)))
		w := httptest.NewRecorder()
		h.CreateCase(w, r)
		assertStatus(t, w, http.StatusForbidden)
		assertErrorMessage(t, w, ErrMsgForbidden)
		assertContentType(t, w, "application/json")
	})

	t.Run("returns 500 when user lookup fails", func(t *testing.T) {
		client := &mockEntityCaseClient{
			searchUsersFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return nil, errors.New("entity service unavailable")
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases", strings.NewReader(validPayload)))
		w := httptest.NewRecorder()
		h.CreateCase(w, r)
		assertStatus(t, w, http.StatusInternalServerError)
		assertErrorMessage(t, w, ErrMsgInternal)
		assertContentType(t, w, "application/json")
	})

	t.Run("upstream errors on create are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to create case.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					searchUsersFn: userSearchOK,
					createCaseFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases", strings.NewReader(validPayload)))
				w := httptest.NewRecorder()
				h.CreateCase(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

// ----- CreateCaseComment -----

func TestCreateCaseComment(t *testing.T) {
	const validPayload = `{"commentType":"comment","body":"Looking into this now."}`
	const resolvedUserID = "entity-user-uuid-42"

	userSearchOK := func(_ context.Context, _ []byte) ([]byte, error) {
		return []byte(`{"users":[{"id":"` + resolvedUserID + `","email":"agent@example.com"}],"total":1}`), nil
	}

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(validPayload))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases//comments", strings.NewReader(validPayload)))
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(`not-json`)))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("resolves createdBy server-side and forwards to upstream", func(t *testing.T) {
		var capturedCaseID string
		var capturedBody []byte
		client := &mockEntityCaseClient{
			searchUsersFn: userSearchOK,
			createCaseCommentFn: func(_ context.Context, caseID string, body []byte) ([]byte, error) {
				capturedCaseID = caseID
				capturedBody = body
				return []byte(`{"id":"comment-1","caseId":"case-1","commentType":"comment","body":"Looking into this now.","createdBy":"` + resolvedUserID + `","createdAt":"2026-06-03T00:00:00Z"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(validPayload)))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)

		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")

		if capturedCaseID != "case-1" {
			t.Errorf("upstream received caseID %q, want %q", capturedCaseID, "case-1")
		}

		var sent map[string]json.RawMessage
		if err := json.Unmarshal(capturedBody, &sent); err != nil {
			t.Fatalf("upstream received invalid JSON: %v", err)
		}
		var gotID string
		if err := json.Unmarshal(sent["createdBy"], &gotID); err != nil || gotID != resolvedUserID {
			t.Errorf("upstream createdBy = %q, want %q", gotID, resolvedUserID)
		}

		resp := decodeJSON[map[string]any](t, w)
		if resp["id"] != "comment-1" {
			t.Errorf("response id = %v, want comment-1", resp["id"])
		}
	})

	t.Run("returns 403 when user not found in entity service", func(t *testing.T) {
		client := &mockEntityCaseClient{
			searchUsersFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return []byte(`{"users":[],"total":0}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(validPayload)))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusForbidden)
		assertErrorMessage(t, w, ErrMsgForbidden)
		assertContentType(t, w, "application/json")
	})

	t.Run("returns 500 when user lookup fails", func(t *testing.T) {
		client := &mockEntityCaseClient{
			searchUsersFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return nil, errors.New("entity service unavailable")
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(validPayload)))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusInternalServerError)
		assertErrorMessage(t, w, ErrMsgInternal)
		assertContentType(t, w, "application/json")
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to create case comment.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					searchUsersFn: userSearchOK,
					createCaseCommentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(validPayload)))
				r.SetPathValue("id", "case-1")
				w := httptest.NewRecorder()
				h.CreateCaseComment(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

// ----- SearchCaseComments -----

func TestSearchCaseComments(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPost, "/cases/case-1/comments/search", strings.NewReader(`{}`))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.SearchCaseComments(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases//comments/search", strings.NewReader(`{}`)))
		w := httptest.NewRecorder()
		h.SearchCaseComments(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.SearchCaseComments(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments/search", strings.NewReader(`not-json`)))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.SearchCaseComments(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards case ID and body to upstream and returns 200", func(t *testing.T) {
		var capturedCaseID string
		var capturedBody []byte
		client := &mockEntityCaseClient{
			searchCaseCommentsFn: func(_ context.Context, caseID string, body []byte) ([]byte, error) {
				capturedCaseID = caseID
				capturedBody = body
				return []byte(`{"comments":[{"id":"c-1","caseId":"case-42","commentType":"comment","body":"First comment","createdBy":"user-1","createdAt":"2026-06-03T00:00:00Z"}],"total":1,"limit":20,"offset":0,"hasMore":false}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-42/comments/search",
			strings.NewReader(`{"pagination":{"limit":20,"offset":0}}`)))
		r.SetPathValue("id", "case-42")
		w := httptest.NewRecorder()
		h.SearchCaseComments(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedCaseID != "case-42" {
			t.Errorf("upstream received caseID %q, want %q", capturedCaseID, "case-42")
		}
		if !json.Valid(capturedBody) {
			t.Errorf("upstream received invalid JSON body: %s", capturedBody)
		}

		resp := decodeJSON[map[string]any](t, w)
		if resp["total"] != float64(1) {
			t.Errorf("total = %v, want 1", resp["total"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to search case comments.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					searchCaseCommentsFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments/search", strings.NewReader(`{}`)))
				r.SetPathValue("id", "case-1")
				w := httptest.NewRecorder()
				h.SearchCaseComments(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

// ----- SearchCases -----

func TestSearchCases(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 200", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityCaseClient{
			searchCasesFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"cases":[{"id":"case-1"}],"total":1}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/search",
			strings.NewReader(`{"projectIds":["proj-1"],"stateKeys":["open"],"pagination":{"limit":10,"offset":0}}`)))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		var sent map[string]json.RawMessage
		if err := json.Unmarshal(capturedBody, &sent); err != nil {
			t.Fatalf("upstream received invalid JSON: %v", err)
		}
		var ids []string
		if err := json.Unmarshal(sent["projectIds"], &ids); err != nil || len(ids) != 1 || ids[0] != "proj-1" {
			t.Errorf("upstream projectIds = %v, want [\"proj-1\"]", ids)
		}

		resp := decodeJSON[map[string]any](t, w)
		if resp["total"] != float64(1) {
			t.Errorf("total = %v, want 1", resp["total"])
		}
	})

	t.Run("forwards body without projectIds unchanged", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityCaseClient{
			searchCasesFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"cases":[],"total":0}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/search",
			strings.NewReader(`{"stateKeys":["open"],"pagination":{"limit":10,"offset":0}}`)))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)

		assertStatus(t, w, http.StatusOK)
		var sent map[string]json.RawMessage
		if err := json.Unmarshal(capturedBody, &sent); err != nil {
			t.Fatalf("upstream received invalid JSON: %v", err)
		}
		if _, exists := sent["projectIds"]; exists {
			t.Errorf("upstream body unexpectedly contains projectIds: %s", sent["projectIds"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to search cases.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					searchCasesFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.SearchCases(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

// ----- PatchCase -----

func TestPatchCase(t *testing.T) {
	const testCaseID = "11111111-1111-1111-1111-111111111111"
	const validPayload = `{"state":"work_in_progress"}`

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(validPayload))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/", strings.NewReader(validPayload)))
		w := httptest.NewRecorder()
		h.PatchCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, "Case ID cannot be empty!")
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/not-a-uuid", strings.NewReader(validPayload)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.PatchCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(`not-json`)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards case ID and body, returns 200 with nextStates injected", func(t *testing.T) {
		var capturedID string
		var capturedBody []byte
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"id":"` + testCaseID + `","state":"open"}`), nil
			},
			patchCaseFn: func(_ context.Context, caseID string, body []byte) ([]byte, error) {
				capturedID = caseID
				capturedBody = body
				return []byte(`{"id":"` + testCaseID + `","state":"work_in_progress"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(validPayload)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedID != testCaseID {
			t.Errorf("upstream received caseID %q, want %q", capturedID, testCaseID)
		}
		if !json.Valid(capturedBody) {
			t.Errorf("upstream received invalid JSON body: %s", capturedBody)
		}

		type patchCaseResp struct {
			ID         string   `json:"id"`
			State      string   `json:"state"`
			NextStates []string `json:"nextStates"`
		}
		resp := decodeJSON[patchCaseResp](t, w)
		if resp.ID != testCaseID {
			t.Errorf("response id = %q, want %q", resp.ID, testCaseID)
		}
		if resp.State != caseStateWorkInProgress {
			t.Errorf("response state = %q, want %q", resp.State, caseStateWorkInProgress)
		}
		wantNext := []string{caseStateWaitingOnWSO2, caseStateAwaitingInfo, caseStateSolutionProposed, caseStateClosed}
		if len(resp.NextStates) != len(wantNext) {
			t.Fatalf("nextStates = %v, want %v", resp.NextStates, wantNext)
		}
		for i, got := range resp.NextStates {
			if got != wantNext[i] {
				t.Errorf("nextStates[%d] = %q, want %q", i, got, wantNext[i])
			}
		}
	})

	t.Run("rejects invalid state transition", func(t *testing.T) {
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				// current state is closed (terminal — no valid transitions)
				return []byte(`{"id":"` + testCaseID + `","state":"closed"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(`{"state":"work_in_progress"}`)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidTransition)
		assertContentType(t, w, "application/json")
	})

	t.Run("allows priority-only update without state validation", func(t *testing.T) {
		client := &mockEntityCaseClient{
			patchCaseFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				return []byte(`{"id":"` + testCaseID + `","state":"open","priority":"high"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(`{"priority":"high"}`)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)
		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
	})

	t.Run("GetCase failure during state validation is mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to retrieve current case state.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(validPayload)))
				r.SetPathValue("id", testCaseID)
				w := httptest.NewRecorder()
				h.PatchCase(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to update case.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
						return []byte(`{"id":"` + testCaseID + `","state":"open"}`), nil
					},
					patchCaseFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(validPayload)))
				r.SetPathValue("id", testCaseID)
				w := httptest.NewRecorder()
				h.PatchCase(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

// ----- GetCase -----

func TestGetCase(t *testing.T) {
	const (
		testCaseID   = "11111111-1111-1111-1111-111111111111"
		testCaseID42 = "42424242-4242-4242-4242-424242424242"
	)

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodGet, "/cases/"+testCaseID, nil)
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.GetCase(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		// PathValue("id") returns "" when not set — exercises the explicit guard.
		r := withUser(httptest.NewRequest(http.MethodGet, "/cases/", nil))
		w := httptest.NewRecorder()
		h.GetCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, "Case ID cannot be empty!")
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/cases/not-a-uuid", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	type getCaseResp struct {
		ID         string   `json:"id"`
		NextStates []string `json:"nextStates"`
	}

	t.Run("passes case ID to upstream and returns 200 with nextStates injected", func(t *testing.T) {
		var capturedID string
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, caseID string) ([]byte, error) {
				capturedID = caseID
				return []byte(`{"id":"` + testCaseID42 + `","state":"open"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/cases/"+testCaseID42, nil))
		r.SetPathValue("id", testCaseID42)
		w := httptest.NewRecorder()
		h.GetCase(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedID != testCaseID42 {
			t.Errorf("upstream received caseID %q, want %q", capturedID, testCaseID42)
		}
		resp := decodeJSON[getCaseResp](t, w)
		if resp.ID != testCaseID42 {
			t.Errorf("response id = %v, want %s", resp.ID, testCaseID42)
		}
		if len(resp.NextStates) != 1 || resp.NextStates[0] != caseStateWorkInProgress {
			t.Errorf("nextStates = %v, want [%s]", resp.NextStates, caseStateWorkInProgress)
		}
	})

	t.Run("nextStates reflects current state", func(t *testing.T) {
		cases := []struct {
			state    string
			wantNext []string
		}{
			{caseStateOpen, []string{caseStateWorkInProgress}},
			{caseStateWorkInProgress, []string{caseStateWaitingOnWSO2, caseStateAwaitingInfo, caseStateSolutionProposed, caseStateClosed}},
			{caseStateWaitingOnWSO2, []string{caseStateWorkInProgress}},
			{caseStateAwaitingInfo, []string{caseStateWaitingOnWSO2}},
			{caseStateReopened, []string{caseStateWaitingOnWSO2}},
			{caseStateSolutionProposed, []string{caseStateClosed, caseStateWaitingOnWSO2}},
			{caseStateClosed, []string{}},
		}
		for _, tc := range cases {
			t.Run(tc.state, func(t *testing.T) {
				t.Parallel()
				body, _ := json.Marshal(map[string]string{"id": testCaseID, "state": tc.state})
				client := &mockEntityCaseClient{
					getCaseFn: func(_ context.Context, _ string) ([]byte, error) { return body, nil },
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/cases/"+testCaseID, nil))
				r.SetPathValue("id", testCaseID)
				w := httptest.NewRecorder()
				h.GetCase(w, r)

				assertStatus(t, w, http.StatusOK)
				resp := decodeJSON[getCaseResp](t, w)
				if len(resp.NextStates) != len(tc.wantNext) {
					t.Fatalf("nextStates = %v, want %v", resp.NextStates, tc.wantNext)
				}
				for i, got := range resp.NextStates {
					if got != tc.wantNext[i] {
						t.Errorf("nextStates[%d] = %v, want %v", i, got, tc.wantNext[i])
					}
				}
			})
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to retrieve case details.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/cases/"+testCaseID, nil))
				r.SetPathValue("id", testCaseID)
				w := httptest.NewRecorder()
				h.GetCase(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
