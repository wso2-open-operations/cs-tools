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
		{"apierror 409", &apierror.Error{StatusCode: http.StatusConflict, Body: "conflict upstream message"}, http.StatusConflict, "conflict upstream message"},
		{"apierror 422", &apierror.Error{StatusCode: http.StatusUnprocessableEntity, Body: "invalid state transition"}, http.StatusUnprocessableEntity, "invalid state transition"},
		{"apierror 502", &apierror.Error{StatusCode: http.StatusBadGateway}, http.StatusServiceUnavailable, fallback},
		{"apierror 503", &apierror.Error{StatusCode: http.StatusServiceUnavailable}, http.StatusServiceUnavailable, fallback},
		{"apierror 504", &apierror.Error{StatusCode: http.StatusGatewayTimeout}, http.StatusServiceUnavailable, fallback},
		{"apierror unmapped (418)", &apierror.Error{StatusCode: http.StatusTeapot}, http.StatusInternalServerError, fallback},
		{"non-apierror error", errors.New("upstream connection refused"), http.StatusInternalServerError, fallback},
	}
}

// ----- CreateCase -----

func TestCreateCase(t *testing.T) {
	const validPayload = `{"projectId":"proj-1","deploymentId":"dep-1","deployedProductId":"dp-1","subject":"Login failure","description":"Users cannot log in","priority":"high","issueType":"error"}`

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPost, "/cases", strings.NewReader(validPayload))
		w := httptest.NewRecorder()
		h.CreateCase(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 10 MiB case limit", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases", strings.NewReader(strings.Repeat("x", maxCaseBodyBytes+1))))
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

	t.Run("forwards body to upstream and returns created case", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityCaseClient{
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
		var gotProjectID string
		if err := json.Unmarshal(sent["projectId"], &gotProjectID); err != nil || gotProjectID != "proj-1" {
			t.Errorf("upstream projectId = %q, want \"proj-1\"", gotProjectID)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["id"] != "case-1" {
			t.Errorf("response id = %v, want case-1", resp["id"])
		}
	})

	t.Run("strips client-supplied createdBy before forwarding", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityCaseClient{
			createCaseFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"id":"case-1","state":"open"}`), nil
			},
		}
		h := NewCaseHandler(client)
		payload := `{"projectId":"proj-1","createdBy":"attacker-uuid","subject":"Login failure"}`
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases", strings.NewReader(payload)))
		w := httptest.NewRecorder()
		h.CreateCase(w, r)

		assertStatus(t, w, http.StatusCreated)

		var sent map[string]json.RawMessage
		if err := json.Unmarshal(capturedBody, &sent); err != nil {
			t.Fatalf("upstream received invalid JSON: %v", err)
		}
		if _, present := sent["createdBy"]; present {
			t.Error("upstream received createdBy but it should have been stripped")
		}
	})

	t.Run("upstream errors on create are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to create case.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
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
	const validPayload = `{"type":"comment","content":"Looking into this now."}`

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

	t.Run("rejects body exceeding 10 MiB comment limit", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(strings.Repeat("x", maxCommentBodyBytes+1))))
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

	const ongoingCase = `{"state":"work_in_progress","workState":"ongoing"}`

	t.Run("rejects comment when state is not work_in_progress", func(t *testing.T) {
		for _, state := range []string{"open", "waiting_on_wso2", "closed"} {
			state := state
			t.Run(state, func(t *testing.T) {
				client := &mockEntityCaseClient{
					getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
						return []byte(`{"state":"` + state + `","workState":null}`), nil
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(validPayload)))
				r.SetPathValue("id", "case-1")
				w := httptest.NewRecorder()
				h.CreateCaseComment(w, r)
				assertStatus(t, w, http.StatusConflict)
				assertErrorMessage(t, w, ErrMsgCommentNotAllowed)
			})
		}
	})

	t.Run("rejects comment when work_state is not ongoing", func(t *testing.T) {
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"state":"work_in_progress","workState":"paused"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(validPayload)))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusConflict)
		assertErrorMessage(t, w, ErrMsgCommentNotAllowed)
	})

	t.Run("rejects comment when workState is absent", func(t *testing.T) {
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"state":"work_in_progress"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(validPayload)))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusConflict)
		assertErrorMessage(t, w, ErrMsgCommentNotAllowed)
	})

	t.Run("forwards body to entity and returns response", func(t *testing.T) {
		var capturedCaseID string
		var capturedBody []byte
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(ongoingCase), nil
			},
			createCaseCommentFn: func(_ context.Context, caseID string, body []byte) ([]byte, error) {
				capturedCaseID = caseID
				capturedBody = body
				return []byte(`{"message":"Comment created successfully","comment":{"id":"comment-1","createdOn":"2026-06-03T00:00:00Z","createdBy":"agent@example.com"}}`), nil
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
		if string(capturedBody) != validPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, validPayload)
		}

		resp := decodeJSON[map[string]any](t, w)
		comment, _ := resp["comment"].(map[string]any)
		if comment["id"] != "comment-1" {
			t.Errorf("response comment.id = %v, want comment-1", comment["id"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to create case comment.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
						return []byte(ongoingCase), nil
					},
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
			strings.NewReader(`{"filters":{"projectIds":["proj-1"],"stateKeys":["open"]},"pagination":{"limit":10,"offset":0}}`)))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		var sent map[string]json.RawMessage
		if err := json.Unmarshal(capturedBody, &sent); err != nil {
			t.Fatalf("upstream received invalid JSON: %v", err)
		}
		var filters map[string]json.RawMessage
		if err := json.Unmarshal(sent["filters"], &filters); err != nil {
			t.Fatalf("upstream filters field invalid JSON: %v", err)
		}
		var ids []string
		if err := json.Unmarshal(filters["projectIds"], &ids); err != nil || len(ids) != 1 || ids[0] != "proj-1" {
			t.Errorf("upstream filters.projectIds = %v, want [\"proj-1\"]", ids)
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
			strings.NewReader(`{"filters":{"stateKeys":["open"]},"pagination":{"limit":10,"offset":0}}`)))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)

		assertStatus(t, w, http.StatusOK)
		var sent map[string]json.RawMessage
		if err := json.Unmarshal(capturedBody, &sent); err != nil {
			t.Fatalf("upstream received invalid JSON: %v", err)
		}
		var filters map[string]json.RawMessage
		if err := json.Unmarshal(sent["filters"], &filters); err != nil {
			t.Fatalf("upstream filters field invalid JSON: %v", err)
		}
		if _, exists := filters["projectIds"]; exists {
			t.Errorf("upstream filters unexpectedly contains projectIds: %s", filters["projectIds"])
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
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("allows non-UUID case ID when x-user-id-token is present", func(t *testing.T) {
		var capturedID string
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, caseID string) ([]byte, error) {
				capturedID = caseID
				return []byte(`{"id":"sn-123","state":"open"}`), nil
			},
			patchCaseFn: func(_ context.Context, caseID string, _ []byte) ([]byte, error) {
				capturedID = caseID
				return []byte(`{"id":"sn-123","state":"work_in_progress"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/sn-123", strings.NewReader(validPayload)))
		r.SetPathValue("id", "sn-123")
		r.Header.Set("x-user-id-token", "token-value")
		w := httptest.NewRecorder()
		h.PatchCase(w, r)
		assertStatus(t, w, http.StatusOK)
		if capturedID != "sn-123" {
			t.Errorf("upstream received caseID %q, want %q", capturedID, "sn-123")
		}
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

	t.Run("forwards case ID and body, returns 200 with upstream response", func(t *testing.T) {
		var capturedID string
		var capturedBody []byte
		const upstreamResp = `{"message":"Case updated successfully","case":{"id":"` + testCaseID + `","updatedOn":"2026-06-18T10:00:00Z","state":"work_in_progress"}}`
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"id":"` + testCaseID + `","state":"open"}`), nil
			},
			patchCaseFn: func(_ context.Context, caseID string, body []byte) ([]byte, error) {
				capturedID = caseID
				capturedBody = body
				return []byte(upstreamResp), nil
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

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
			t.Fatalf("decode raw response body: %v; raw: %s", err, w.Body.String())
		}
		if _, ok := raw["nextStates"]; ok {
			t.Errorf("response must not include legacy top-level nextStates")
		}

		type patchCaseResp struct {
			Message string `json:"message"`
			Case    struct {
				ID    string `json:"id"`
				State string `json:"state"`
			} `json:"case"`
		}
		resp := decodeJSON[patchCaseResp](t, w)
		if resp.Message != "Case updated successfully" {
			t.Errorf("response message = %q, want %q", resp.Message, "Case updated successfully")
		}
		if resp.Case.ID != testCaseID {
			t.Errorf("response case.id = %q, want %q", resp.Case.ID, testCaseID)
		}
		if resp.Case.State != caseStateWorkInProgress {
			t.Errorf("response case.state = %q, want %q", resp.Case.State, caseStateWorkInProgress)
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
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("allows non-UUID case ID when x-user-id-token is present", func(t *testing.T) {
		var capturedID string
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, caseID string) ([]byte, error) {
				capturedID = caseID
				return []byte(`{"id":"sn-123","state":"open"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/cases/sn-123", nil))
		r.SetPathValue("id", "sn-123")
		r.Header.Set("x-user-id-token", "token-value")
		w := httptest.NewRecorder()
		h.GetCase(w, r)
		assertStatus(t, w, http.StatusOK)
		if capturedID != "sn-123" {
			t.Errorf("upstream received caseID %q, want %q", capturedID, "sn-123")
		}
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

// ----- CreateCaseAttachment -----

func TestCreateCaseAttachment(t *testing.T) {
	const (
		testCaseID   = "11111111-1111-1111-1111-111111111111"
		validPayload = `{"name":"screenshot.png","type":"image/png","file":"data:image/png;base64,aGVsbG8="}`
	)

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPost, "/cases/"+testCaseID+"/attachments", strings.NewReader(validPayload))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.CreateCaseAttachment(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects invalid case UUID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/not-a-uuid/attachments", strings.NewReader(validPayload)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.CreateCaseAttachment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
	})

	t.Run("rejects body exceeding 15 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testCaseID+"/attachments", strings.NewReader(strings.Repeat("x", maxAttachmentBodyBytes+1))))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.CreateCaseAttachment(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testCaseID+"/attachments", strings.NewReader(`not-json`)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.CreateCaseAttachment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("returns 201 on success", func(t *testing.T) {
		t.Parallel()
		want := `{"message":"uploaded","attachment":{"id":"` + testCaseID + `"}}`
		client := &mockEntityCaseClient{
			createCaseAttachmentFn: func(_ context.Context, caseID string, _ []byte) ([]byte, error) {
				if caseID != testCaseID {
					t.Errorf("caseID = %q, want %q", caseID, testCaseID)
				}
				return []byte(want), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testCaseID+"/attachments", strings.NewReader(validPayload)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.CreateCaseAttachment(w, r)
		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")
	})

	t.Run("maps upstream errors", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to create case attachment.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					createCaseAttachmentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testCaseID+"/attachments", strings.NewReader(validPayload)))
				r.SetPathValue("id", testCaseID)
				w := httptest.NewRecorder()
				h.CreateCaseAttachment(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

// ----- SearchCaseAttachments -----

func TestSearchCaseAttachments(t *testing.T) {
	const (
		testCaseID   = "11111111-1111-1111-1111-111111111111"
		validPayload = `{"pagination":{"limit":20,"offset":0}}`
	)

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPost, "/cases/"+testCaseID+"/attachments/search", strings.NewReader(validPayload))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.SearchCaseAttachments(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects invalid case UUID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/not-a-uuid/attachments/search", strings.NewReader(validPayload)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.SearchCaseAttachments(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
	})

	t.Run("returns 200 on success", func(t *testing.T) {
		t.Parallel()
		client := &mockEntityCaseClient{}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testCaseID+"/attachments/search", strings.NewReader(validPayload)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.SearchCaseAttachments(w, r)
		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
	})

	t.Run("maps upstream errors", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to search case attachments.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					searchCaseAttachmentsFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testCaseID+"/attachments/search", strings.NewReader(validPayload)))
				r.SetPathValue("id", testCaseID)
				w := httptest.NewRecorder()
				h.SearchCaseAttachments(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

// ----- GetCaseAttachmentContent -----

func TestGetCaseAttachmentContent(t *testing.T) {
	const (
		testCaseID       = "11111111-1111-1111-1111-111111111111"
		testAttachmentID = "22222222-2222-2222-2222-222222222222"
	)

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodGet, "/cases/"+testCaseID+"/attachments/"+testAttachmentID+"/content", nil)
		r.SetPathValue("case_id", testCaseID)
		r.SetPathValue("attachment_id", testAttachmentID)
		w := httptest.NewRecorder()
		h.GetCaseAttachmentContent(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects invalid case UUID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/cases/not-a-uuid/attachments/"+testAttachmentID+"/content", nil))
		r.SetPathValue("case_id", "not-a-uuid")
		r.SetPathValue("attachment_id", testAttachmentID)
		w := httptest.NewRecorder()
		h.GetCaseAttachmentContent(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
	})

	t.Run("rejects invalid attachment UUID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/cases/"+testCaseID+"/attachments/not-a-uuid/content", nil))
		r.SetPathValue("case_id", testCaseID)
		r.SetPathValue("attachment_id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetCaseAttachmentContent(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
	})

	t.Run("streams binary content with upstream Content-Type", func(t *testing.T) {
		t.Parallel()
		client := &mockEntityCaseClient{
			getCaseAttachmentContentFn: func(_ context.Context, caseID, attachmentID string) ([]byte, string, error) {
				if caseID != testCaseID || attachmentID != testAttachmentID {
					t.Errorf("ids = (%q,%q), want (%q,%q)", caseID, attachmentID, testCaseID, testAttachmentID)
				}
				return []byte("PNG_BYTES"), "image/png", nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/cases/"+testCaseID+"/attachments/"+testAttachmentID+"/content", nil))
		r.SetPathValue("case_id", testCaseID)
		r.SetPathValue("attachment_id", testAttachmentID)
		w := httptest.NewRecorder()
		h.GetCaseAttachmentContent(w, r)
		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "image/png")
		if w.Body.String() != "PNG_BYTES" {
			t.Errorf("body = %q, want %q", w.Body.String(), "PNG_BYTES")
		}
		if w.Header().Get("Content-Disposition") != "attachment" {
			t.Errorf("Content-Disposition = %q, want %q", w.Header().Get("Content-Disposition"), "attachment")
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("X-Content-Type-Options = %q, want %q", w.Header().Get("X-Content-Type-Options"), "nosniff")
		}
	})

	t.Run("coerces unsafe Content-Type to octet-stream", func(t *testing.T) {
		t.Parallel()
		client := &mockEntityCaseClient{
			getCaseAttachmentContentFn: func(_ context.Context, _, _ string) ([]byte, string, error) {
				return []byte("<script>alert(1)</script>"), "text/html; charset=utf-8", nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/cases/"+testCaseID+"/attachments/"+testAttachmentID+"/content", nil))
		r.SetPathValue("case_id", testCaseID)
		r.SetPathValue("attachment_id", testAttachmentID)
		w := httptest.NewRecorder()
		h.GetCaseAttachmentContent(w, r)
		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/octet-stream")
	})

	t.Run("maps upstream errors", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to retrieve attachment content.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					getCaseAttachmentContentFn: func(_ context.Context, _, _ string) ([]byte, string, error) {
						return nil, "", tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/cases/"+testCaseID+"/attachments/"+testAttachmentID+"/content", nil))
				r.SetPathValue("case_id", testCaseID)
				r.SetPathValue("attachment_id", testAttachmentID)
				w := httptest.NewRecorder()
				h.GetCaseAttachmentContent(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
