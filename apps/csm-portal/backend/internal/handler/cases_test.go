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
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// upstreamErrorCases is the table used by every PATCH/update handler — the
// ones that call mapUpstreamError (see upstreamErrorsGeneric for every other
// handler, which calls mapUpstreamErrorGeneric instead). It covers all four
// explicit apierror mappings plus an unmapped code and a plain error.
type upstreamErrorCase struct {
	name     string
	err      error
	wantCode int
	wantMsg  string
}

func upstreamErrors(fallback string) []upstreamErrorCase {
	return []upstreamErrorCase{
		{"apierror 401", &apierror.Error{StatusCode: http.StatusUnauthorized}, http.StatusUnauthorized, ErrMsgUnauthorized},
		{"apierror 403", &apierror.Error{StatusCode: http.StatusForbidden}, http.StatusForbidden, ErrMsgForbidden},
		{"apierror 404", &apierror.Error{StatusCode: http.StatusNotFound}, http.StatusNotFound, ErrMsgNotFound},
		{"apierror 400 empty body falls back", &apierror.Error{StatusCode: http.StatusBadRequest, Body: ""}, http.StatusBadRequest, ErrMsgBadRequest},
		{"apierror 400 JSON envelope body surfaces upstream message", &apierror.Error{StatusCode: http.StatusBadRequest, Body: `{"code":400,"message":"invalid type \"bogus\""}`}, http.StatusBadRequest, `invalid type "bogus"`},
		{"apierror 400 malformed JSON body falls back", &apierror.Error{StatusCode: http.StatusBadRequest, Body: `{not valid json`}, http.StatusBadRequest, ErrMsgBadRequest},
		{"apierror 400 JSON body without message field falls back", &apierror.Error{StatusCode: http.StatusBadRequest, Body: `{"code":400}`}, http.StatusBadRequest, ErrMsgBadRequest},
		{"apierror 409 plain text body", &apierror.Error{StatusCode: http.StatusConflict, Body: "conflict upstream message"}, http.StatusConflict, "conflict upstream message"},
		{"apierror 422 plain text body", &apierror.Error{StatusCode: http.StatusUnprocessableEntity, Body: "invalid state transition"}, http.StatusUnprocessableEntity, "invalid state transition"},
		{"apierror 409 JSON envelope body", &apierror.Error{StatusCode: http.StatusConflict, Body: `{"code":409,"message":"State transition rejected"}`}, http.StatusConflict, "State transition rejected"},
		{"apierror 422 JSON envelope body", &apierror.Error{StatusCode: http.StatusUnprocessableEntity, Body: `{"code":422,"message":"invalid state transition"}`}, http.StatusUnprocessableEntity, "invalid state transition"},
		{"apierror 409 empty body falls back", &apierror.Error{StatusCode: http.StatusConflict, Body: ""}, http.StatusConflict, fallback},
		{"apierror 502", &apierror.Error{StatusCode: http.StatusBadGateway}, http.StatusServiceUnavailable, fallback},
		{"apierror 503", &apierror.Error{StatusCode: http.StatusServiceUnavailable}, http.StatusServiceUnavailable, fallback},
		{"apierror 504", &apierror.Error{StatusCode: http.StatusGatewayTimeout}, http.StatusServiceUnavailable, fallback},
		{"apierror unmapped (418)", &apierror.Error{StatusCode: http.StatusTeapot}, http.StatusInternalServerError, fallback},
		// The default branch never surfaces the upstream body, whatever its shape: a
		// 5xx or unmapped upstream failure is not caller-actionable, and this branch
		// is the error path for several clients (the identity provider's among them),
		// so a well-formed envelope proves shape, not that the content is safe to
		// show a portal client. Caller-actionable reasons come through the 4xx
		// branches above.
		{"apierror 500 JSON envelope body is not echoed", &apierror.Error{StatusCode: http.StatusInternalServerError, Body: `{"code":500,"message":"Invalid state transition: the change request is not in a state that allows this action."}`}, http.StatusInternalServerError, fallback},
		{"apierror 500 empty body falls back", &apierror.Error{StatusCode: http.StatusInternalServerError, Body: ""}, http.StatusInternalServerError, fallback},
		{"apierror 500 foreign body is not echoed", &apierror.Error{StatusCode: http.StatusInternalServerError, Body: `{"schemas":["urn:ietf:params:scim:api:messages:2.0:Error"],"detail":"internal identity provider detail","status":"500"}`}, http.StatusInternalServerError, fallback},
		{"apierror 500 malformed JSON body is not echoed", &apierror.Error{StatusCode: http.StatusInternalServerError, Body: `{not valid json`}, http.StatusInternalServerError, fallback},
		{"apierror 500 plain text body is not echoed", &apierror.Error{StatusCode: http.StatusInternalServerError, Body: `goroutine 1 [running]: internal stack detail`}, http.StatusInternalServerError, fallback},
		{"apierror unmapped (418) with envelope body is not echoed", &apierror.Error{StatusCode: http.StatusTeapot, Body: `{"code":418,"message":"upstream reason"}`}, http.StatusInternalServerError, fallback},
		{"non-apierror error", errors.New("upstream connection refused"), http.StatusInternalServerError, fallback},
	}
}

// upstreamErrorsGeneric is upstreamErrors' counterpart for every non-PATCH
// handler (the vast majority), which calls mapUpstreamErrorGeneric: every
// 4xx case falls back to fallback instead of surfacing the upstream body,
// since those endpoints forward a request that's only partially validated at
// this layer, so a 4xx from upstream isn't necessarily something the caller
// could have avoided.
func upstreamErrorsGeneric(fallback string) []upstreamErrorCase {
	return []upstreamErrorCase{
		{"apierror 401", &apierror.Error{StatusCode: http.StatusUnauthorized}, http.StatusUnauthorized, ErrMsgUnauthorized},
		{"apierror 403", &apierror.Error{StatusCode: http.StatusForbidden}, http.StatusForbidden, ErrMsgForbidden},
		{"apierror 404", &apierror.Error{StatusCode: http.StatusNotFound}, http.StatusNotFound, ErrMsgNotFound},
		{"apierror 400 JSON envelope body is not echoed", &apierror.Error{StatusCode: http.StatusBadRequest, Body: `{"code":400,"message":"invalid type \"bogus\""}`}, http.StatusBadRequest, fallback},
		{"apierror 400 empty body falls back", &apierror.Error{StatusCode: http.StatusBadRequest, Body: ""}, http.StatusBadRequest, fallback},
		{"apierror 409 plain text body is not echoed", &apierror.Error{StatusCode: http.StatusConflict, Body: "conflict upstream message"}, http.StatusConflict, fallback},
		{"apierror 422 plain text body is not echoed", &apierror.Error{StatusCode: http.StatusUnprocessableEntity, Body: "invalid state transition"}, http.StatusUnprocessableEntity, fallback},
		{"apierror 502", &apierror.Error{StatusCode: http.StatusBadGateway}, http.StatusServiceUnavailable, fallback},
		{"apierror 503", &apierror.Error{StatusCode: http.StatusServiceUnavailable}, http.StatusServiceUnavailable, fallback},
		{"apierror 504", &apierror.Error{StatusCode: http.StatusGatewayTimeout}, http.StatusServiceUnavailable, fallback},
		{"apierror unmapped (418)", &apierror.Error{StatusCode: http.StatusTeapot}, http.StatusInternalServerError, fallback},
		{"apierror 500 JSON envelope body is not echoed", &apierror.Error{StatusCode: http.StatusInternalServerError, Body: `{"code":500,"message":"internal detail"}`}, http.StatusInternalServerError, fallback},
		{"non-apierror error", errors.New("upstream connection refused"), http.StatusInternalServerError, fallback},
	}
}

// ----- CreateCase -----

func TestCreateCase(t *testing.T) {
	const validPayload = `{"type":"case","projectId":"proj-1","deploymentId":"dep-1","deployedProductId":"dp-1","subject":"Login failure","description":"Users cannot log in","severity":"high","issueType":"error"}`

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
		for _, tc := range upstreamErrorsGeneric("Failed to create case.") {
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

	// testPlatformUserID is the id GET /users/me resolves for the requesting
	// user (see helpers_test.go), so this fixture represents that user being
	// the case's assigned engineer. Note it is NOT testUser.UserID: assignee
	// ids live in the platform's id space, the token claim in the identity
	// provider's, and the guard must compare within the former.
	const ongoingCase = `{"state":"work_in_progress","workState":"ongoing","assignedEngineer":{"id":"` + testPlatformUserID + `"}}`

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

	t.Run("rejects public comment when requester is not the assigned engineer", func(t *testing.T) {
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"state":"work_in_progress","workState":"ongoing","assignedEngineer":{"id":"someone-else"}}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(validPayload)))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusForbidden)
		assertErrorMessage(t, w, ErrMsgCommentNotOwnCase)
	})

	t.Run("rejects public comment when case has no assigned engineer", func(t *testing.T) {
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"state":"work_in_progress","workState":"ongoing"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(validPayload)))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusForbidden)
		assertErrorMessage(t, w, ErrMsgCommentNotOwnCase)
	})

	// Regression guard for the ownership check comparing two unrelated id
	// spaces. The assignee id on a case is a platform user record id; the
	// caller's identity on the request is the identity provider's user id.
	// Comparing them denied everyone, the assignee included. The fixtures below
	// deliberately keep the two values distinct so that bug cannot come back.
	t.Run("allows public comment when requester is the assigned engineer", func(t *testing.T) {
		var getUserMeCalls int
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(ongoingCase), nil
			},
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				getUserMeCalls++
				return []byte(`{"id":"` + testPlatformUserID + `","email":"agent@example.com"}`), nil
			},
			createCaseCommentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				return []byte(`{"id":"comment-1"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(validPayload)))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusCreated)
		if getUserMeCalls != 1 {
			t.Errorf("GetUserMe calls = %d, want 1: the caller's own id must be resolved, once", getUserMeCalls)
		}
	})

	t.Run("rejects public comment when the assignee id equals the token user id", func(t *testing.T) {
		// The identity provider's user id must never satisfy the ownership
		// check: a case whose assignee id happens to hold that value is not
		// assigned to the caller.
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"state":"work_in_progress","workState":"ongoing","assignedEngineer":{"id":"` + testUser.UserID + `"}}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(validPayload)))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusForbidden)
		assertErrorMessage(t, w, ErrMsgCommentNotOwnCase)
	})

	t.Run("fails closed when the caller's own id cannot be resolved", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			fn   func(context.Context) ([]byte, error)
		}{
			{"lookup error", func(context.Context) ([]byte, error) { return nil, errors.New("entity unavailable") }},
			{"unparseable response", func(context.Context) ([]byte, error) { return []byte(`{not json`), nil }},
			{"empty id", func(context.Context) ([]byte, error) { return []byte(`{"id":""}`), nil }},
		} {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				var commentCreated bool
				client := &mockEntityCaseClient{
					getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
						return []byte(ongoingCase), nil
					},
					getUserMeFn: tc.fn,
					createCaseCommentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						commentCreated = true
						return []byte(`{"id":"comment-1"}`), nil
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(validPayload)))
				r.SetPathValue("id", "case-1")
				w := httptest.NewRecorder()
				h.CreateCaseComment(w, r)
				assertStatus(t, w, http.StatusInternalServerError)
				assertErrorMessage(t, w, ErrMsgInternal)
				if commentCreated {
					t.Error("comment was created despite the caller's identity being unresolvable")
				}
			})
		}
	})

	t.Run("does not resolve the caller's id for a work_note", func(t *testing.T) {
		// Work notes are internal-only and exempt from the ownership guard, so
		// they must not pay the extra lookup either.
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"state":"work_in_progress"}`), nil
			},
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				t.Error("GetUserMe must not be called for a work note")
				return nil, errors.New("unexpected call")
			},
			createCaseCommentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				return []byte(`{"id":"wn-1"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(`{"type":"work_note","content":"internal note"}`)))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusCreated)
	})

	t.Run("does not resolve the caller's id when the state gate already rejects", func(t *testing.T) {
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"state":"work_in_progress","workState":"paused","assignedEngineer":{"id":"` + testPlatformUserID + `"}}`), nil
			},
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				t.Error("GetUserMe must not be called once the state gate has rejected the request")
				return nil, errors.New("unexpected call")
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

	t.Run("allows work_note when case is not closed", func(t *testing.T) {
		for _, state := range []string{"open", "work_in_progress", "waiting_on_wso2", "awaiting_info", "solution_proposed"} {
			state := state
			t.Run(state, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
						return []byte(`{"state":"` + state + `"}`), nil
					},
					createCaseCommentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return []byte(`{"id":"wn-1"}`), nil
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(`{"type":"work_note","content":"internal note"}`)))
				r.SetPathValue("id", "case-1")
				w := httptest.NewRecorder()
				h.CreateCaseComment(w, r)
				assertStatus(t, w, http.StatusCreated)
			})
		}
	})

	t.Run("blocks work_note on closed case", func(t *testing.T) {
		t.Parallel()
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"state":"closed"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(`{"type":"work_note","content":"internal note"}`)))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusConflict)
		assertErrorMessage(t, w, ErrMsgWorkNoteOnClosedCase)
	})

	t.Run("allows public comment on an announcement case with no work-in-progress state and no assigned engineer", func(t *testing.T) {
		for _, state := range []string{"open", "published"} {
			state := state
			t.Run(state, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
						return []byte(`{"type":"announcement","state":"` + state + `"}`), nil
					},
					createCaseCommentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return []byte(`{"id":"comment-1"}`), nil
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(validPayload)))
				r.SetPathValue("id", "case-1")
				w := httptest.NewRecorder()
				h.CreateCaseComment(w, r)
				assertStatus(t, w, http.StatusCreated)
			})
		}
	})

	t.Run("blocks public comment on a closed announcement case", func(t *testing.T) {
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"type":"announcement","state":"closed"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(validPayload)))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusConflict)
		assertErrorMessage(t, w, ErrMsgCommentOnClosedCase)
	})

	t.Run("allows work_note on an announcement case with no assigned engineer", func(t *testing.T) {
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"type":"announcement","state":"open"}`), nil
			},
			createCaseCommentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				return []byte(`{"id":"wn-1"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/comments", strings.NewReader(`{"type":"work_note","content":"internal note"}`)))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusCreated)
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
		for _, tc := range upstreamErrorsGeneric("Failed to create case comment.") {
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

	t.Run("injects referenceId and referenceType into SearchComments payload", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityCaseClient{
			searchCommentsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"comments":[{"id":"c-1","referenceId":"case-42","type":"comment","content":"First comment","createdBy":"user-1","createdOn":"2026-06-03T00:00:00Z"}],"total":1,"limit":20,"offset":0,"hasMore":false}`), nil
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

		var payload map[string]any
		if err := json.Unmarshal(capturedBody, &payload); err != nil {
			t.Fatalf("upstream received invalid JSON: %v", err)
		}
		if payload["referenceId"] != "case-42" {
			t.Errorf("referenceId = %v, want %q", payload["referenceId"], "case-42")
		}
		if payload["referenceType"] != "case" {
			t.Errorf("referenceType = %v, want %q", payload["referenceType"], "case")
		}

		resp := decodeJSON[map[string]any](t, w)
		if resp["total"] != float64(1) {
			t.Errorf("total = %v, want 1", resp["total"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to search case comments.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					searchCommentsFn: func(_ context.Context, _ []byte) ([]byte, error) {
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

// ----- SearchCaseActivities -----

func TestSearchCaseActivities(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPost, "/cases/case-1/activities/search", strings.NewReader(`{}`))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.SearchCaseActivities(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases//activities/search", strings.NewReader(`{}`)))
		w := httptest.NewRecorder()
		h.SearchCaseActivities(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/activities/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.SearchCaseActivities(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/activities/search", strings.NewReader(`not-json`)))
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.SearchCaseActivities(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body verbatim and returns upstream response", func(t *testing.T) {
		var capturedCaseID string
		var capturedBody []byte
		reqBody := `{"pagination":{"limit":20,"offset":0},"includeFieldChanges":true}`
		client := &mockEntityCaseClient{
			searchCaseActivitiesFn: func(_ context.Context, caseID string, body []byte) ([]byte, error) {
				capturedCaseID = caseID
				capturedBody = body
				return []byte(`{"activities":[{"id":"a-1"}],"total":1,"limit":20,"offset":0,"hasMore":false}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-42/activities/search", strings.NewReader(reqBody)))
		r.SetPathValue("id", "case-42")
		w := httptest.NewRecorder()
		h.SearchCaseActivities(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedCaseID != "case-42" {
			t.Errorf("caseID = %q, want %q", capturedCaseID, "case-42")
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
		for _, tc := range upstreamErrorsGeneric("Failed to search case activities.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					searchCaseActivitiesFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/case-1/activities/search", strings.NewReader(`{}`)))
				r.SetPathValue("id", "case-1")
				w := httptest.NewRecorder()
				h.SearchCaseActivities(w, r)
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
			strings.NewReader(`{"filters":{"projectIds":["proj-1"],"states":["open"]},"pagination":{"limit":10,"offset":0}}`)))
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
			strings.NewReader(`{"filters":{"states":["open"]},"pagination":{"limit":10,"offset":0}}`)))
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

	t.Run("forwards parentId filter unchanged", func(t *testing.T) {
		// Item 9: the child-case list reuses this existing search endpoint with a
		// parentId filter — no new endpoint, no BFF-side modeling of the filter.
		var capturedBody []byte
		client := &mockEntityCaseClient{
			searchCasesFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"cases":[],"total":0}`), nil
			},
		}
		h := NewCaseHandler(client)
		const reqBody = `{"filters":{"parentId":"44444444-4444-4444-4444-444444444444"}}`
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(reqBody)))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)

		assertStatus(t, w, http.StatusOK)
		if string(capturedBody) != reqBody {
			t.Errorf("upstream received body %s, want %s (must forward verbatim)", capturedBody, reqBody)
		}
	})

	t.Run("passes through account.creTeam/sreTeam unchanged", func(t *testing.T) {
		// Item 10: additive nested read field on search results, zero BFF handling.
		const upstreamBody = `{"cases":[{"id":"11111111-1111-1111-1111-111111111111","account":{"id":"77777777-7777-7777-7777-777777777777","name":"Example Account","creTeam":{"id":"88888888-8888-8888-8888-888888888888","name":"CRE Team A"},"sreTeam":{"id":"99999999-9999-9999-9999-999999999999","name":"SRE Team B"}}}],"total":1}`
		client := &mockEntityCaseClient{
			searchCasesFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return []byte(upstreamBody), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(`{}`)))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)

		assertStatus(t, w, http.StatusOK)
		type teamRef struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		type respCase struct {
			Account struct {
				CreTeam teamRef `json:"creTeam"`
				SreTeam teamRef `json:"sreTeam"`
			} `json:"account"`
		}
		type resp struct {
			Cases []respCase `json:"cases"`
		}
		got := decodeJSON[resp](t, w)
		if len(got.Cases) != 1 {
			t.Fatalf("cases = %+v, want 1 entry", got.Cases)
		}
		if got.Cases[0].Account.CreTeam.Name != "CRE Team A" || got.Cases[0].Account.SreTeam.Name != "SRE Team B" {
			t.Errorf("account team refs = %+v, want CRE Team A / SRE Team B", got.Cases[0].Account)
		}
	})

	t.Run("passes through autoclosureStep/autoclosureStateTime unchanged", func(t *testing.T) {
		// Item 6 (revised): additive read fields on search results, zero BFF handling.
		const upstreamBody = `{"cases":[{"id":"11111111-1111-1111-1111-111111111111","autoclosureStep":"FIRST_COMMENT","autoclosureStateTime":"2026-07-30T00:00:00Z"}],"total":1}`
		client := &mockEntityCaseClient{
			searchCasesFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return []byte(upstreamBody), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(`{}`)))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)

		assertStatus(t, w, http.StatusOK)
		type respCase struct {
			AutoclosureStep      string `json:"autoclosureStep"`
			AutoclosureStateTime string `json:"autoclosureStateTime"`
		}
		type resp struct {
			Cases []respCase `json:"cases"`
		}
		got := decodeJSON[resp](t, w)
		if len(got.Cases) != 1 {
			t.Fatalf("cases = %+v, want 1 entry", got.Cases)
		}
		if got.Cases[0].AutoclosureStep != "FIRST_COMMENT" || got.Cases[0].AutoclosureStateTime != "2026-07-30T00:00:00Z" {
			t.Errorf("autoclosure fields = %+v, want FIRST_COMMENT / 2026-07-30T00:00:00Z", got.Cases[0])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to search cases.") {
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

	t.Run("rejects workState update when case is not work_in_progress", func(t *testing.T) {
		t.Parallel()
		for _, state := range []string{"open", "waiting_on_wso2", "awaiting_info", "solution_proposed", "closed"} {
			state := state
			t.Run("current_state="+state, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
						return []byte(`{"id":"` + testCaseID + `","state":"` + state + `"}`), nil
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(`{"workState":"ongoing"}`)))
				r.SetPathValue("id", testCaseID)
				w := httptest.NewRecorder()
				h.PatchCase(w, r)
				assertStatus(t, w, http.StatusBadRequest)
				assertErrorMessage(t, w, ErrMsgWorkStateNotAllowed)
				assertContentType(t, w, "application/json")
			})
		}
	})

	t.Run("allows workState update when case is work_in_progress", func(t *testing.T) {
		t.Parallel()
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"id":"` + testCaseID + `","state":"work_in_progress"}`), nil
			},
			patchCaseFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				return []byte(`{"id":"` + testCaseID + `","state":"work_in_progress","workState":"paused"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(`{"workState":"paused"}`)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)
		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
	})

	t.Run("allows severity-only update without state validation", func(t *testing.T) {
		client := &mockEntityCaseClient{
			patchCaseFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				return []byte(`{"id":"` + testCaseID + `","state":"open","severity":"high"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(`{"severity":"high"}`)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)
		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards new single-field PATCH variants verbatim", func(t *testing.T) {
		t.Parallel()
		// Items 7, 9: subject/description/deploymentId/deployedProductId/parentId/relatedCaseId
		// are pure pass-through single-field PATCH variants. None trip the state/workState peek,
		// so patchCaseFn is invoked directly with the raw body and the upstream response passes
		// through unchanged.
		cases := []struct {
			name       string
			reqBody    string
			upstream   string
			wantField  string
			wantString string
		}{
			{
				name:       "subject",
				reqBody:    `{"subject":"New subject text"}`,
				upstream:   `{"message":"Case updated successfully","case":{"id":"` + testCaseID + `","updatedOn":"2026-07-23T10:00:00Z","subject":"New subject text"}}`,
				wantField:  "subject",
				wantString: "New subject text",
			},
			{
				name:       "description",
				reqBody:    `{"description":"New description text"}`,
				upstream:   `{"message":"Case updated successfully","case":{"id":"` + testCaseID + `","updatedOn":"2026-07-23T10:00:00Z","description":"New description text"}}`,
				wantField:  "description",
				wantString: "New description text",
			},
			{
				name:       "deploymentId",
				reqBody:    `{"deploymentId":"22222222-2222-2222-2222-222222222222"}`,
				upstream:   `{"message":"Case updated successfully","case":{"id":"` + testCaseID + `","updatedOn":"2026-07-23T10:00:00Z","deploymentId":"22222222-2222-2222-2222-222222222222"}}`,
				wantField:  "deploymentId",
				wantString: "22222222-2222-2222-2222-222222222222",
			},
			{
				name:       "deployedProductId",
				reqBody:    `{"deployedProductId":"33333333-3333-3333-3333-333333333333"}`,
				upstream:   `{"message":"Case updated successfully","case":{"id":"` + testCaseID + `","updatedOn":"2026-07-23T10:00:00Z","deployedProductId":"33333333-3333-3333-3333-333333333333"}}`,
				wantField:  "deployedProductId",
				wantString: "33333333-3333-3333-3333-333333333333",
			},
			{
				name:       "parentId",
				reqBody:    `{"parentId":"44444444-4444-4444-4444-444444444444"}`,
				upstream:   `{"message":"Case updated successfully","case":{"id":"` + testCaseID + `","updatedOn":"2026-07-23T10:00:00Z","parentCase":{"id":"44444444-4444-4444-4444-444444444444","number":"CS0001"}}}`,
				wantField:  "",
				wantString: "",
			},
			{
				name:       "relatedCaseId",
				reqBody:    `{"relatedCaseId":"55555555-5555-5555-5555-555555555555"}`,
				upstream:   `{"message":"Case updated successfully","case":{"id":"` + testCaseID + `","updatedOn":"2026-07-23T10:00:00Z","relatedCase":{"id":"55555555-5555-5555-5555-555555555555","number":"CS0002"}}}`,
				wantField:  "",
				wantString: "",
			},
			{
				// Item 6 (revised): autocloseHoldUntil is the only supported write against
				// ServiceNow's staged auto-closure sequence; it internally sets
				// autoclosureStep=ON_HOLD + autoclosureStateTime, but the BFF forwards the
				// request/response verbatim with no knowledge of that mechanism.
				name:       "autocloseHoldUntil",
				reqBody:    `{"autocloseHoldUntil":"2026-08-01T00:00:00Z"}`,
				upstream:   `{"message":"Case updated successfully","case":{"id":"` + testCaseID + `","updatedOn":"2026-07-23T10:00:00Z","autoclosureStep":"ON_HOLD","autoclosureStateTime":"2026-08-01T00:00:00Z"}}`,
				wantField:  "autoclosureStep",
				wantString: "ON_HOLD",
			},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				var capturedBody []byte
				client := &mockEntityCaseClient{
					patchCaseFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
						capturedBody = body
						return []byte(tc.upstream), nil
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(tc.reqBody)))
				r.SetPathValue("id", testCaseID)
				w := httptest.NewRecorder()
				h.PatchCase(w, r)

				assertStatus(t, w, http.StatusOK)
				assertContentType(t, w, "application/json")

				if string(capturedBody) != tc.reqBody {
					t.Errorf("upstream received body %s, want %s (must forward verbatim)", capturedBody, tc.reqBody)
				}

				var raw map[string]json.RawMessage
				if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
					t.Fatalf("decode response: %v; raw: %s", err, w.Body.String())
				}
				var wrapper struct {
					Case map[string]json.RawMessage `json:"case"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &wrapper); err != nil {
					t.Fatalf("decode wrapper: %v", err)
				}
				if tc.wantField != "" {
					var got string
					if err := json.Unmarshal(wrapper.Case[tc.wantField], &got); err != nil {
						t.Fatalf("decode case.%s: %v", tc.wantField, err)
					}
					if got != tc.wantString {
						t.Errorf("case.%s = %q, want %q", tc.wantField, got, tc.wantString)
					}
				}
			})
		}
	})

	t.Run("autocloseHoldUntil PATCH also records a work note documenting the hold", func(t *testing.T) {
		var (
			commentCaseID string
			commentBody   []byte
		)
		commentCalled := make(chan struct{})
		client := &mockEntityCaseClient{
			// No prior autoclosureStateTime (getCaseFn unset -> default "{}"), so the
			// new hold date always counts as a change here.
			patchCaseFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				return []byte(`{"message":"Case updated successfully","case":{"id":"` + testCaseID + `","updatedOn":"2026-07-23T10:00:00Z","autoclosureStep":"ON_HOLD","autoclosureStateTime":"2026-08-01T00:00:00Z"}}`), nil
			},
			createCaseCommentFn: func(_ context.Context, caseID string, body []byte) ([]byte, error) {
				commentCaseID = caseID
				commentBody = body
				close(commentCalled)
				return []byte(`{"id":"wn-1"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(`{"autocloseHoldUntil":"2026-08-01T00:00:00Z"}`)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)

		assertStatus(t, w, http.StatusOK)

		// The work note is recorded fire-and-forget in a goroutine so it never delays
		// the PATCH response; wait for it (bounded) rather than asserting immediately.
		select {
		case <-commentCalled:
		case <-time.After(2 * time.Second):
			t.Fatal("expected CreateCaseComment to be called after a successful autocloseHoldUntil PATCH")
		}
		if commentCaseID != testCaseID {
			t.Errorf("comment posted against caseID %q, want %q", commentCaseID, testCaseID)
		}

		var note struct {
			Type    string `json:"type"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(commentBody, &note); err != nil {
			t.Fatalf("decode comment body: %v; raw: %s", err, commentBody)
		}
		if note.Type != "work_note" {
			t.Errorf("comment type = %q, want %q", note.Type, "work_note")
		}
		wantContent := "Please note that this case is on-hold until 2026-08-01, hence it will not go through the auto closure process. It will be eligible for auto-closure again after this date passes, or if the case state is changed to 'Waiting on WSO2'."
		if note.Content != wantContent {
			t.Errorf("comment content = %q, want %q", note.Content, wantContent)
		}
	})

	t.Run("autocloseHoldUntil PATCH work note survives the request context being canceled after the handler returns", func(t *testing.T) {
		// The work note is recorded from a goroutine detached (via
		// context.WithoutCancel) from the request context, which is canceled as soon
		// as the handler returns. A regression back to a plain child context (or to
		// context.Background(), which would silently drop the caller's identity
		// instead) should be caught here: this asserts the context the async call
		// actually receives is not Done at the moment it's used, even after the
		// request's own context has since been canceled. The error is captured
		// synchronously inside the mock, not read back afterwards from the test
		// goroutine — reading it later would race against the WithTimeout context's
		// own deferred cleanup cancel(), which is unrelated to request detachment.
		ctxDone := make(chan struct{})
		var gotErr error
		client := &mockEntityCaseClient{
			patchCaseFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				return []byte(`{"message":"Case updated successfully","case":{"id":"` + testCaseID + `","updatedOn":"2026-07-23T10:00:00Z","autoclosureStep":"ON_HOLD","autoclosureStateTime":"2026-08-01T00:00:00Z"}}`), nil
			},
			createCaseCommentFn: func(ctx context.Context, _ string, _ []byte) ([]byte, error) {
				gotErr = ctx.Err()
				close(ctxDone)
				return []byte(`{"id":"wn-1"}`), nil
			},
		}
		h := NewCaseHandler(client)
		reqCtx, cancel := context.WithCancel(context.Background())
		r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(`{"autocloseHoldUntil":"2026-08-01T00:00:00Z"}`)).WithContext(reqCtx))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)
		assertStatus(t, w, http.StatusOK)

		// Simulate the request finishing (connection torn down) right after the
		// handler returns, before the async work note has necessarily run.
		cancel()

		select {
		case <-ctxDone:
		case <-time.After(2 * time.Second):
			t.Fatal("expected CreateCaseComment to be called")
		}
		if gotErr != nil {
			t.Errorf("work-note context was already canceled (%v) once the request context was — it must be detached", gotErr)
		}
	})

	t.Run("autocloseHoldUntil PATCH does not record a duplicate work note when the hold date is unchanged", func(t *testing.T) {
		var commentCalled atomic.Bool
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"id":"` + testCaseID + `","autoclosureStep":"ON_HOLD","autoclosureStateTime":"2026-08-01T00:00:00Z"}`), nil
			},
			patchCaseFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				return []byte(`{"message":"Case updated successfully","case":{"id":"` + testCaseID + `","updatedOn":"2026-07-23T10:00:00Z","autoclosureStep":"ON_HOLD","autoclosureStateTime":"2026-08-01T00:00:00Z"}}`), nil
			},
			createCaseCommentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				commentCalled.Store(true)
				return []byte(`{"id":"wn-1"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(`{"autocloseHoldUntil":"2026-08-01T00:00:00Z"}`)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)

		assertStatus(t, w, http.StatusOK)
		// Give a would-be goroutine a moment to run; there should be none to wait for.
		time.Sleep(100 * time.Millisecond)
		if commentCalled.Load() {
			t.Error("expected no work note for a PATCH that resends the existing hold date")
		}
	})

	t.Run("autocloseHoldUntil PATCH records a work note on the first hold, even if autoclosureStateTime already held an unrelated staged-advance date", func(t *testing.T) {
		// A case not currently on hold can still carry a non-nil autoclosureStateTime
		// (e.g. when its autoclosureStep is FIRST_COMMENT) that happens to match the
		// FE's pre-filled hold-date picker default. The dedup check must gate on
		// autoclosureStep == ON_HOLD, not merely on the date matching, or this — the
		// default "open dialog, accept the pre-filled date, click Hold" path — would
		// wrongly be treated as a no-op and its note dropped.
		commentCalled := make(chan struct{})
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"id":"` + testCaseID + `","autoclosureStep":"FIRST_COMMENT","autoclosureStateTime":"2026-08-01T00:00:00Z"}`), nil
			},
			patchCaseFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				return []byte(`{"message":"Case updated successfully","case":{"id":"` + testCaseID + `","updatedOn":"2026-07-23T10:00:00Z","autoclosureStep":"ON_HOLD","autoclosureStateTime":"2026-08-01T00:00:00Z"}}`), nil
			},
			createCaseCommentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				close(commentCalled)
				return []byte(`{"id":"wn-1"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(`{"autocloseHoldUntil":"2026-08-01T00:00:00Z"}`)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)

		assertStatus(t, w, http.StatusOK)
		select {
		case <-commentCalled:
		case <-time.After(2 * time.Second):
			t.Fatal("expected CreateCaseComment to be called for the first hold, despite a matching prior autoclosureStateTime")
		}
	})

	t.Run("autocloseHoldUntil PATCH still succeeds when the work-note comment call fails", func(t *testing.T) {
		client := &mockEntityCaseClient{
			patchCaseFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				return []byte(`{"message":"Case updated successfully","case":{"id":"` + testCaseID + `","updatedOn":"2026-07-23T10:00:00Z","autoclosureStep":"ON_HOLD","autoclosureStateTime":"2026-08-01T00:00:00Z"}}`), nil
			},
			createCaseCommentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				return nil, errors.New("entity service unavailable")
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(`{"autocloseHoldUntil":"2026-08-01T00:00:00Z"}`)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
	})

	t.Run("PATCH without autocloseHoldUntil does not record a work note", func(t *testing.T) {
		var commentCalled atomic.Bool
		client := &mockEntityCaseClient{
			patchCaseFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				return []byte(`{"message":"Case updated successfully","case":{"id":"` + testCaseID + `","updatedOn":"2026-07-23T10:00:00Z","subject":"New subject text"}}`), nil
			},
			createCaseCommentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				commentCalled.Store(true)
				return []byte(`{"id":"wn-1"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(`{"subject":"New subject text"}`)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)

		assertStatus(t, w, http.StatusOK)
		if commentCalled.Load() {
			t.Error("expected CreateCaseComment not to be called when autocloseHoldUntil is absent from the PATCH")
		}
	})

	t.Run("GetCase failure during state validation is mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to retrieve current case state.") {
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
			{caseStateReopened, []string{caseStateWorkInProgress}},
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

	t.Run("nextStates surfaces reopened as the create-related-case signal", func(t *testing.T) {
		type getCaseNextStatesResp struct {
			NextStates []string `json:"nextStates"`
		}
		recentClosed := time.Now().Add(-10 * 24 * time.Hour).Format(time.RFC3339)
		oldClosed := time.Now().Add(-90 * 24 * time.Hour).Format(time.RFC3339)
		recentClosedNoZone := time.Now().Add(-10 * 24 * time.Hour).UTC().Format("2006-01-02 15:04:05")
		cases := []struct {
			name     string
			body     string
			wantNext []string
		}{
			{"closed case within the 60-day window", `{"id":"` + testCaseID + `","type":"case","state":"closed","closedOn":"` + recentClosed + `"}`, []string{caseStateReopened}},
			{"closed case outside the 60-day window", `{"id":"` + testCaseID + `","type":"case","state":"closed","closedOn":"` + oldClosed + `"}`, []string{}},
			{"closed case with a zoneless space-separated closedOn", `{"id":"` + testCaseID + `","type":"case","state":"closed","closedOn":"` + recentClosedNoZone + `"}`, []string{caseStateReopened}},
			{"closed case with no closedOn or updatedOn", `{"id":"` + testCaseID + `","type":"case","state":"closed"}`, []string{}},
			{"open case with a closedOn value", `{"id":"` + testCaseID + `","type":"case","state":"open","closedOn":"` + recentClosed + `"}`, []string{caseStateWorkInProgress}},
			{"closed service_request within the window", `{"id":"` + testCaseID + `","type":"service_request","state":"closed","closedOn":"` + recentClosed + `"}`, []string{}},
			{"closed case with no type set", `{"id":"` + testCaseID + `","state":"closed","closedOn":"` + recentClosed + `"}`, []string{}},
			// ServiceNow-backed cases never populate closedOn today, so
			// updatedOn stands in for it.
			{"closed case with no closedOn, falls back to a recent updatedOn", `{"id":"` + testCaseID + `","type":"case","state":"closed","updatedOn":"` + recentClosed + `"}`, []string{caseStateReopened}},
			{"closed case with no closedOn, falls back to an old updatedOn", `{"id":"` + testCaseID + `","type":"case","state":"closed","updatedOn":"` + oldClosed + `"}`, []string{}},
			{"closed case prefers closedOn over updatedOn when both present", `{"id":"` + testCaseID + `","type":"case","state":"closed","closedOn":"` + oldClosed + `","updatedOn":"` + recentClosed + `"}`, []string{}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
						return []byte(tc.body), nil
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/cases/"+testCaseID, nil))
				r.SetPathValue("id", testCaseID)
				w := httptest.NewRecorder()
				h.GetCase(w, r)
				assertStatus(t, w, http.StatusOK)
				resp := decodeJSON[getCaseNextStatesResp](t, w)
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

	t.Run("passes through watchList and account.creTeam/sreTeam unchanged", func(t *testing.T) {
		// Items 2, 10: watchList and account.creTeam/sreTeam are additive entity-response
		// fields with zero BFF handling — GetCase's injectNextStates merge must not drop or
		// alter them.
		const upstreamBody = `{
			"id":"` + testCaseID + `",
			"state":"open",
			"watchList":[{"id":"66666666-6666-6666-6666-666666666666","userName":"jdoe","name":"Jane Doe","email":"jane.doe@example.com"}],
			"account":{"id":"77777777-7777-7777-7777-777777777777","name":"Example Account","type":"customer","creTeam":{"id":"88888888-8888-8888-8888-888888888888","name":"CRE Team A"},"sreTeam":{"id":"99999999-9999-9999-9999-999999999999","name":"SRE Team B"}}
		}`
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(upstreamBody), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/cases/"+testCaseID, nil))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.GetCase(w, r)

		assertStatus(t, w, http.StatusOK)

		type watchListUser struct {
			ID       string `json:"id"`
			UserName string `json:"userName"`
			Name     string `json:"name"`
			Email    string `json:"email"`
		}
		type teamRef struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		type account struct {
			ID      string  `json:"id"`
			Name    string  `json:"name"`
			CreTeam teamRef `json:"creTeam"`
			SreTeam teamRef `json:"sreTeam"`
		}
		type resp struct {
			WatchList []watchListUser `json:"watchList"`
			Account   account         `json:"account"`
		}
		got := decodeJSON[resp](t, w)

		if len(got.WatchList) != 1 || got.WatchList[0].Email != "jane.doe@example.com" || got.WatchList[0].UserName != "jdoe" {
			t.Errorf("watchList = %+v, want a single entry for jdoe/jane.doe@example.com", got.WatchList)
		}
		if got.Account.CreTeam.ID != "88888888-8888-8888-8888-888888888888" || got.Account.CreTeam.Name != "CRE Team A" {
			t.Errorf("account.creTeam = %+v, want id 88888888-8888-8888-8888-888888888888 / CRE Team A", got.Account.CreTeam)
		}
		if got.Account.SreTeam.ID != "99999999-9999-9999-9999-999999999999" || got.Account.SreTeam.Name != "SRE Team B" {
			t.Errorf("account.sreTeam = %+v, want id 99999999-9999-9999-9999-999999999999 / SRE Team B", got.Account.SreTeam)
		}
	})

	t.Run("passes through autoclosureStep and autoclosureStateTime unchanged", func(t *testing.T) {
		// Item 6 (revised): autoclosureStep/autoclosureStateTime are read-only informational
		// fields on the entity response — GetCase's injectNextStates merge must not drop or
		// alter them.
		const upstreamBody = `{
			"id":"` + testCaseID + `",
			"state":"open",
			"autoclosureStep":"ON_HOLD",
			"autoclosureStateTime":"2026-08-01T00:00:00Z"
		}`
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(upstreamBody), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/cases/"+testCaseID, nil))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.GetCase(w, r)

		assertStatus(t, w, http.StatusOK)

		type resp struct {
			AutoclosureStep      string `json:"autoclosureStep"`
			AutoclosureStateTime string `json:"autoclosureStateTime"`
		}
		got := decodeJSON[resp](t, w)

		if got.AutoclosureStep != "ON_HOLD" {
			t.Errorf("autoclosureStep = %q, want ON_HOLD", got.AutoclosureStep)
		}
		if got.AutoclosureStateTime != "2026-08-01T00:00:00Z" {
			t.Errorf("autoclosureStateTime = %q, want 2026-08-01T00:00:00Z", got.AutoclosureStateTime)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to retrieve case details.") {
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

// ----- GetCaseEscalations -----

func TestGetCaseEscalations(t *testing.T) {
	const testCaseID = "11111111-1111-1111-1111-111111111111"

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodGet, "/cases/"+testCaseID+"/escalations", nil)
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.GetCaseEscalations(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/cases//escalations", nil))
		w := httptest.NewRecorder()
		h.GetCaseEscalations(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed case UUID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/cases/not-a-uuid/escalations", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetCaseEscalations(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("returns upstream escalation history", func(t *testing.T) {
		var capturedCaseID string
		want := `{"escalations":[{"id":"e-1","level":"2","action":"ESCALATE"}]}`
		client := &mockEntityCaseClient{
			searchCaseEscalationsFn: func(_ context.Context, caseID string) ([]byte, error) {
				capturedCaseID = caseID
				return []byte(want), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/cases/"+testCaseID+"/escalations", nil))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.GetCaseEscalations(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedCaseID != testCaseID {
			t.Errorf("caseID = %q, want %q", capturedCaseID, testCaseID)
		}

		resp := decodeJSON[map[string]any](t, w)
		escalations, ok := resp["escalations"].([]any)
		if !ok || len(escalations) != 1 {
			t.Errorf("escalations = %v, want a single entry", resp["escalations"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to retrieve case escalation history.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					searchCaseEscalationsFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/cases/"+testCaseID+"/escalations", nil))
				r.SetPathValue("id", testCaseID)
				w := httptest.NewRecorder()
				h.GetCaseEscalations(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

// ----- CreateCaseEscalation -----

func TestCreateCaseEscalation(t *testing.T) {
	const testCaseID = "11111111-1111-1111-1111-111111111111"

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPost, "/cases/"+testCaseID+"/escalations", strings.NewReader(`{"action":"ESCALATE"}`))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.CreateCaseEscalation(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases//escalations", strings.NewReader(`{"action":"ESCALATE"}`)))
		w := httptest.NewRecorder()
		h.CreateCaseEscalation(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed case UUID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/not-a-uuid/escalations", strings.NewReader(`{"action":"ESCALATE"}`)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.CreateCaseEscalation(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testCaseID+"/escalations", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.CreateCaseEscalation(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testCaseID+"/escalations", strings.NewReader(`not-json`)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.CreateCaseEscalation(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body verbatim and returns 201 with upstream response", func(t *testing.T) {
		var capturedCaseID string
		var capturedBody []byte
		reqBody := `{"reason":"Customer escalated via call","action":"ESCALATE"}`
		want := `{"id":"e-1","level":"1","action":"ESCALATE"}`
		client := &mockEntityCaseClient{
			createCaseEscalationFn: func(_ context.Context, caseID string, body []byte) ([]byte, error) {
				capturedCaseID = caseID
				capturedBody = body
				return []byte(want), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testCaseID+"/escalations", strings.NewReader(reqBody)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.CreateCaseEscalation(w, r)

		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")

		if capturedCaseID != testCaseID {
			t.Errorf("caseID = %q, want %q", capturedCaseID, testCaseID)
		}
		if string(capturedBody) != reqBody {
			t.Errorf("upstream body = %q, want verbatim %q", string(capturedBody), reqBody)
		}
	})

	t.Run("de-escalation is disabled for everyone when no roles are configured", func(t *testing.T) {
		client := &mockEntityCaseClient{
			createCaseEscalationFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				t.Fatal("upstream CreateCaseEscalation should not be called when the caller is not authorized to de-escalate")
				return nil, nil
			},
		}
		h := NewCaseHandler(client) // SetDeescalationAllowedRoles never called
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testCaseID+"/escalations", strings.NewReader(`{"action":"DEESCALATE"}`)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.CreateCaseEscalation(w, r)
		assertStatus(t, w, http.StatusForbidden)
		assertErrorMessage(t, w, ErrMsgForbidden)
		assertContentType(t, w, "application/json")
	})

	t.Run("de-escalation is rejected for a caller without an allowed role", func(t *testing.T) {
		client := &mockEntityCaseClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return []byte(`{"id":"u-1","roles":["agent"]}`), nil
			},
			createCaseEscalationFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				t.Fatal("upstream CreateCaseEscalation should not be called when the caller lacks an allowed role")
				return nil, nil
			},
		}
		h := NewCaseHandler(client)
		h.SetDeescalationAllowedRoles([]string{"admin"})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testCaseID+"/escalations", strings.NewReader(`{"action":"DEESCALATE"}`)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.CreateCaseEscalation(w, r)
		assertStatus(t, w, http.StatusForbidden)
		assertErrorMessage(t, w, ErrMsgForbidden)
		assertContentType(t, w, "application/json")
	})

	t.Run("de-escalation is allowed for a caller with an allowed role, matched case-insensitively", func(t *testing.T) {
		var upstreamCalled bool
		client := &mockEntityCaseClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return []byte(`{"id":"u-1","roles":["Admin"]}`), nil
			},
			createCaseEscalationFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				upstreamCalled = true
				return []byte(`{"id":"e-1","level":"1","action":"DEESCALATE"}`), nil
			},
		}
		h := NewCaseHandler(client)
		h.SetDeescalationAllowedRoles([]string{"admin"})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testCaseID+"/escalations", strings.NewReader(`{"action":"DEESCALATE"}`)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.CreateCaseEscalation(w, r)
		assertStatus(t, w, http.StatusCreated)
		if !upstreamCalled {
			t.Error("upstream CreateCaseEscalation was not called for an authorized de-escalation")
		}
	})

	t.Run("escalation is never role-gated, even with de-escalation roles configured", func(t *testing.T) {
		var upstreamCalled bool
		client := &mockEntityCaseClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				t.Fatal("GetUserMe should not be called for an ESCALATE action")
				return nil, nil
			},
			createCaseEscalationFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				upstreamCalled = true
				return []byte(`{"id":"e-1","level":"1","action":"ESCALATE"}`), nil
			},
		}
		h := NewCaseHandler(client)
		h.SetDeescalationAllowedRoles([]string{"admin"})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testCaseID+"/escalations", strings.NewReader(`{"reason":"needed","action":"ESCALATE"}`)))
		r.SetPathValue("id", testCaseID)
		w := httptest.NewRecorder()
		h.CreateCaseEscalation(w, r)
		assertStatus(t, w, http.StatusCreated)
		if !upstreamCalled {
			t.Error("upstream CreateCaseEscalation was not called for an ESCALATE action")
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to create case escalation.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					createCaseEscalationFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testCaseID+"/escalations", strings.NewReader(`{"action":"ESCALATE"}`)))
				r.SetPathValue("id", testCaseID)
				w := httptest.NewRecorder()
				h.CreateCaseEscalation(w, r)
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
		r := httptest.NewRequest(http.MethodPost, "/attachments", strings.NewReader(validPayload))
		w := httptest.NewRecorder()
		h.CreateCaseAttachment(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects body exceeding 15 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/attachments", strings.NewReader(strings.Repeat("x", maxAttachmentBodyBytes+1))))
		w := httptest.NewRecorder()
		h.CreateCaseAttachment(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/attachments", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.CreateCaseAttachment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("returns 201 on success", func(t *testing.T) {
		t.Parallel()
		want := `{"message":"uploaded","attachment":{"id":"` + testCaseID + `"}}`
		client := &mockEntityCaseClient{
			createCaseAttachmentFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return []byte(want), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/attachments", strings.NewReader(validPayload)))
		w := httptest.NewRecorder()
		h.CreateCaseAttachment(w, r)
		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")
	})

	t.Run("maps upstream errors", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to create case attachment.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					createCaseAttachmentFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/attachments", strings.NewReader(validPayload)))
				w := httptest.NewRecorder()
				h.CreateCaseAttachment(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})

	t.Run("blocks attachment upload on closed case", func(t *testing.T) {
		t.Parallel()
		casePayload := `{"referenceId":"` + testCaseID + `","referenceType":"case","name":"file.png","type":"image/png","file":"data:image/png;base64,aGVsbG8="}`
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"state":"closed"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/attachments", strings.NewReader(casePayload)))
		w := httptest.NewRecorder()
		h.CreateCaseAttachment(w, r)
		assertStatus(t, w, http.StatusConflict)
		assertErrorMessage(t, w, ErrMsgAttachmentOnClosedCase)
	})

	t.Run("allows attachment upload on open case", func(t *testing.T) {
		t.Parallel()
		casePayload := `{"referenceId":"` + testCaseID + `","referenceType":"case","name":"file.png","type":"image/png","file":"data:image/png;base64,aGVsbG8="}`
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"state":"work_in_progress"}`), nil
			},
			createCaseAttachmentFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return []byte(`{"id":"att-1"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/attachments", strings.NewReader(casePayload)))
		w := httptest.NewRecorder()
		h.CreateCaseAttachment(w, r)
		assertStatus(t, w, http.StatusCreated)
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
		r := httptest.NewRequest(http.MethodPost, "/attachments/search", strings.NewReader(validPayload))
		w := httptest.NewRecorder()
		h.SearchCaseAttachments(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("returns 200 on success", func(t *testing.T) {
		t.Parallel()
		client := &mockEntityCaseClient{}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/attachments/search", strings.NewReader(validPayload)))
		w := httptest.NewRecorder()
		h.SearchCaseAttachments(w, r)
		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
	})

	t.Run("maps upstream errors", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to search case attachments.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					searchCaseAttachmentsFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/attachments/search", strings.NewReader(validPayload)))
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
		r := httptest.NewRequest(http.MethodGet, "/attachments/"+testAttachmentID+"/content", nil)
		r.SetPathValue("id", testAttachmentID)
		w := httptest.NewRecorder()
		h.GetCaseAttachmentContent(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects invalid attachment UUID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/attachments/not-a-uuid/content", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetCaseAttachmentContent(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
	})

	t.Run("streams binary content with upstream Content-Type", func(t *testing.T) {
		t.Parallel()
		client := &mockEntityCaseClient{
			getCaseAttachmentContentFn: func(_ context.Context, attachmentID string) ([]byte, string, error) {
				if attachmentID != testAttachmentID {
					t.Errorf("attachmentID = %q, want %q", attachmentID, testAttachmentID)
				}
				return []byte("PNG_BYTES"), "image/png", nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/attachments/"+testAttachmentID+"/content", nil))
		r.SetPathValue("id", testAttachmentID)
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
	})

	t.Run("coerces unsafe Content-Type to octet-stream", func(t *testing.T) {
		t.Parallel()
		client := &mockEntityCaseClient{
			getCaseAttachmentContentFn: func(_ context.Context, _ string) ([]byte, string, error) {
				return []byte("<script>alert(1)</script>"), "text/html; charset=utf-8", nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/attachments/"+testAttachmentID+"/content", nil))
		r.SetPathValue("id", testAttachmentID)
		w := httptest.NewRecorder()
		h.GetCaseAttachmentContent(w, r)
		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/octet-stream")
	})

	t.Run("maps upstream errors", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to retrieve attachment content.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					getCaseAttachmentContentFn: func(_ context.Context, _ string) ([]byte, string, error) {
						return nil, "", tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/attachments/"+testAttachmentID+"/content", nil))
				r.SetPathValue("id", testAttachmentID)
				w := httptest.NewRecorder()
				h.GetCaseAttachmentContent(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

// ----- GetAttachment -----

func TestGetAttachment(t *testing.T) {
	const testAttachmentID = "22222222-2222-2222-2222-222222222222"

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodGet, "/attachments/"+testAttachmentID, nil)
		r.SetPathValue("id", testAttachmentID)
		w := httptest.NewRecorder()
		h.GetAttachment(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty attachment ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/attachments/", nil))
		w := httptest.NewRecorder()
		h.GetAttachment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID attachment ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/attachments/not-a-uuid", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetAttachment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards attachment ID and returns upstream metadata", func(t *testing.T) {
		const upstreamResp = `{"id":"` + testAttachmentID + `","name":"screenshot.png","description":"a screenshot"}`
		var capturedID string
		client := &mockEntityCaseClient{
			getAttachmentFn: func(_ context.Context, attachmentID string) ([]byte, error) {
				capturedID = attachmentID
				return []byte(upstreamResp), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/attachments/"+testAttachmentID, nil))
		r.SetPathValue("id", testAttachmentID)
		w := httptest.NewRecorder()
		h.GetAttachment(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedID != testAttachmentID {
			t.Errorf("upstream received attachmentID %q, want %q", capturedID, testAttachmentID)
		}
		if w.Body.String() != upstreamResp {
			t.Errorf("body = %q, want %q", w.Body.String(), upstreamResp)
		}
	})

	t.Run("maps upstream errors", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to retrieve attachment.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					getAttachmentFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/attachments/"+testAttachmentID, nil))
				r.SetPathValue("id", testAttachmentID)
				w := httptest.NewRecorder()
				h.GetAttachment(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

// ----- UpdateAttachment -----

func TestUpdateAttachment(t *testing.T) {
	const testAttachmentID = "22222222-2222-2222-2222-222222222222"
	const validPayload = `{"referenceId":"11111111-1111-1111-1111-111111111111","referenceType":"case","name":"renamed.png"}`

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPatch, "/attachments/"+testAttachmentID, strings.NewReader(validPayload))
		r.SetPathValue("id", testAttachmentID)
		w := httptest.NewRecorder()
		h.UpdateAttachment(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty attachment ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/attachments/", strings.NewReader(validPayload)))
		w := httptest.NewRecorder()
		h.UpdateAttachment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID attachment ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/attachments/not-a-uuid", strings.NewReader(validPayload)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.UpdateAttachment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/attachments/"+testAttachmentID, strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		r.SetPathValue("id", testAttachmentID)
		w := httptest.NewRecorder()
		h.UpdateAttachment(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/attachments/"+testAttachmentID, strings.NewReader(`not-json`)))
		r.SetPathValue("id", testAttachmentID)
		w := httptest.NewRecorder()
		h.UpdateAttachment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	invalidBodies := []struct {
		name string
		body string
	}{
		{"null body", `null`},
		{"array body", `[]`},
		{"empty object", `{}`},
		{"missing referenceId", `{"referenceType":"case","name":"renamed.png"}`},
		{"invalid referenceId", `{"referenceId":"not-a-uuid","referenceType":"case","name":"renamed.png"}`},
		{"invalid referenceType", `{"referenceId":"11111111-1111-1111-1111-111111111111","referenceType":"bogus","name":"renamed.png"}`},
		{"neither name nor description", `{"referenceId":"11111111-1111-1111-1111-111111111111","referenceType":"case"}`},
		{"unknown field", `{"referenceId":"11111111-1111-1111-1111-111111111111","referenceType":"case","name":"renamed.png","extra":"nope"}`},
	}
	for _, tc := range invalidBodies {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			h := NewCaseHandler(&mockEntityCaseClient{})
			r := withUser(httptest.NewRequest(http.MethodPatch, "/attachments/"+testAttachmentID, strings.NewReader(tc.body)))
			r.SetPathValue("id", testAttachmentID)
			w := httptest.NewRecorder()
			h.UpdateAttachment(w, r)
			assertStatus(t, w, http.StatusBadRequest)
			assertErrorMessage(t, w, ErrMsgBadRequest)
			assertContentType(t, w, "application/json")
		})
	}

	t.Run("accepts description-only update with explicit null (clear)", func(t *testing.T) {
		const payload = `{"referenceId":"11111111-1111-1111-1111-111111111111","referenceType":"deployment","description":null}`
		client := &mockEntityCaseClient{
			updateAttachmentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				return []byte(`{}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/attachments/"+testAttachmentID, strings.NewReader(payload)))
		r.SetPathValue("id", testAttachmentID)
		w := httptest.NewRecorder()
		h.UpdateAttachment(w, r)
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("forwards attachment ID and body, returns 200 with upstream response", func(t *testing.T) {
		var capturedID string
		var capturedBody []byte
		const upstreamResp = `{"id":"` + testAttachmentID + `","name":"renamed.png"}`
		client := &mockEntityCaseClient{
			updateAttachmentFn: func(_ context.Context, attachmentID string, body []byte) ([]byte, error) {
				capturedID = attachmentID
				capturedBody = body
				return []byte(upstreamResp), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/attachments/"+testAttachmentID, strings.NewReader(validPayload)))
		r.SetPathValue("id", testAttachmentID)
		w := httptest.NewRecorder()
		h.UpdateAttachment(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedID != testAttachmentID {
			t.Errorf("upstream received attachmentID %q, want %q", capturedID, testAttachmentID)
		}
		if string(capturedBody) != validPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, validPayload)
		}
		if w.Body.String() != upstreamResp {
			t.Errorf("body = %q, want %q", w.Body.String(), upstreamResp)
		}
	})

	t.Run("maps upstream errors", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to update attachment.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					updateAttachmentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPatch, "/attachments/"+testAttachmentID, strings.NewReader(validPayload)))
				r.SetPathValue("id", testAttachmentID)
				w := httptest.NewRecorder()
				h.UpdateAttachment(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestCreateCaseGithubIssue(t *testing.T) {
	const caseID = "11111111-1111-1111-1111-111111111111"

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/github-issues", strings.NewReader(`{"title":"crash"}`))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.CreateCaseGithubIssue(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases//github-issues", strings.NewReader(`{"title":"crash"}`)))
		w := httptest.NewRecorder()
		h.CreateCaseGithubIssue(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/not-a-uuid/github-issues", strings.NewReader(`{"title":"crash"}`)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.CreateCaseGithubIssue(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/github-issues", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.CreateCaseGithubIssue(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/github-issues", strings.NewReader(`not-json`)))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.CreateCaseGithubIssue(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards case ID and body to upstream and returns 201", func(t *testing.T) {
		const reqPayload = `{"reason":"default","title":"crash on startup","description":"details"}`
		var capturedCaseID string
		var capturedBody []byte
		client := &mockEntityCaseClient{
			createCaseGithubIssueFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedCaseID = id
				capturedBody = body
				return []byte(`{"issueUrl":"https://github.com/org/repo/issues/1","issueNumber":1,"repository":"org/repo"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/github-issues", strings.NewReader(reqPayload)))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.CreateCaseGithubIssue(w, r)

		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")
		if capturedCaseID != caseID {
			t.Errorf("upstream received caseID %q, want %q", capturedCaseID, caseID)
		}
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to create GitHub issue.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					createCaseGithubIssueFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/github-issues", strings.NewReader(`{"title":"crash"}`)))
				r.SetPathValue("id", caseID)
				w := httptest.NewRecorder()
				h.CreateCaseGithubIssue(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestAddCaseTag(t *testing.T) {
	const caseID = "11111111-1111-1111-1111-111111111111"

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", strings.NewReader(`{"label":"micro-gw"}`))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.AddCaseTag(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed case UUID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/not-a-uuid/tags", strings.NewReader(`{"label":"micro-gw"}`)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.AddCaseTag(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.AddCaseTag(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", strings.NewReader(`not-json`)))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.AddCaseTag(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards case id and body verbatim, returns 201", func(t *testing.T) {
		const reqBody = `{"label":"micro-gw"}`
		var capturedCaseID string
		var capturedBody []byte
		client := &mockEntityCaseClient{
			addCaseTagFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedCaseID = id
				capturedBody = body
				return []byte(`{"id":"22222222-2222-2222-2222-222222222222","label":"micro-gw","color":null}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", strings.NewReader(reqBody)))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.AddCaseTag(w, r)

		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")
		if capturedCaseID != caseID {
			t.Errorf("upstream received caseID %q, want %q", capturedCaseID, caseID)
		}
		if string(capturedBody) != reqBody {
			t.Errorf("upstream body = %q, want verbatim %q", string(capturedBody), reqBody)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to add case tag.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					addCaseTagFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", strings.NewReader(`{"label":"micro-gw"}`)))
				r.SetPathValue("id", caseID)
				w := httptest.NewRecorder()
				h.AddCaseTag(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestRemoveCaseTag(t *testing.T) {
	const caseID = "11111111-1111-1111-1111-111111111111"
	const tagID = "22222222-2222-2222-2222-222222222222"

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodDelete, "/cases/"+caseID+"/tags/"+tagID, nil)
		r.SetPathValue("id", caseID)
		r.SetPathValue("tagId", tagID)
		w := httptest.NewRecorder()
		h.RemoveCaseTag(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed case UUID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodDelete, "/cases/not-a-uuid/tags/"+tagID, nil))
		r.SetPathValue("id", "not-a-uuid")
		r.SetPathValue("tagId", tagID)
		w := httptest.NewRecorder()
		h.RemoveCaseTag(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed tag UUID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodDelete, "/cases/"+caseID+"/tags/not-a-uuid", nil))
		r.SetPathValue("id", caseID)
		r.SetPathValue("tagId", "not-a-uuid")
		w := httptest.NewRecorder()
		h.RemoveCaseTag(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards case id and tag id to upstream, returns 204 with no body", func(t *testing.T) {
		var capturedCaseID, capturedTagID string
		client := &mockEntityCaseClient{
			removeCaseTagFn: func(_ context.Context, cID, tID string) ([]byte, error) {
				capturedCaseID = cID
				capturedTagID = tID
				return nil, nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodDelete, "/cases/"+caseID+"/tags/"+tagID, nil))
		r.SetPathValue("id", caseID)
		r.SetPathValue("tagId", tagID)
		w := httptest.NewRecorder()
		h.RemoveCaseTag(w, r)

		assertStatus(t, w, http.StatusNoContent)
		if capturedCaseID != caseID {
			t.Errorf("upstream received caseID %q, want %q", capturedCaseID, caseID)
		}
		if capturedTagID != tagID {
			t.Errorf("upstream received tagID %q, want %q", capturedTagID, tagID)
		}
		if w.Body.Len() != 0 {
			t.Errorf("body = %q, want empty for 204", w.Body.String())
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to remove case tag.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					removeCaseTagFn: func(_ context.Context, _, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodDelete, "/cases/"+caseID+"/tags/"+tagID, nil))
				r.SetPathValue("id", caseID)
				r.SetPathValue("tagId", tagID)
				w := httptest.NewRecorder()
				h.RemoveCaseTag(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestGetCasePassesThroughTags(t *testing.T) {
	// Item 8 (read): tags are an additive entity-response field with zero BFF
	// handling — GetCase's injectNextStates merge must not drop or alter them.
	const testCaseID = "11111111-1111-1111-1111-111111111111"
	const upstreamBody = `{
		"id":"` + testCaseID + `",
		"state":"open",
		"tags":[{"id":"33333333-3333-3333-3333-333333333333","label":"micro-gw","color":"#FF6600"}]
	}`
	client := &mockEntityCaseClient{
		getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(upstreamBody), nil
		},
	}
	h := NewCaseHandler(client)
	r := withUser(httptest.NewRequest(http.MethodGet, "/cases/"+testCaseID, nil))
	r.SetPathValue("id", testCaseID)
	w := httptest.NewRecorder()
	h.GetCase(w, r)

	assertStatus(t, w, http.StatusOK)

	type tag struct {
		ID    string  `json:"id"`
		Label string  `json:"label"`
		Color *string `json:"color"`
	}
	type resp struct {
		Tags []tag `json:"tags"`
	}
	got := decodeJSON[resp](t, w)

	if len(got.Tags) != 1 || got.Tags[0].Label != "micro-gw" || got.Tags[0].Color == nil || *got.Tags[0].Color != "#FF6600" {
		t.Errorf("tags = %+v, want a single micro-gw/#FF6600 entry", got.Tags)
	}
}

func TestSearchCasesForwardsTagsFilter(t *testing.T) {
	// Item 8: the tags search filter is forwarded through SearchCases verbatim —
	// zero BFF handling.
	var capturedBody []byte
	client := &mockEntityCaseClient{
		searchCasesFn: func(_ context.Context, body []byte) ([]byte, error) {
			capturedBody = body
			return []byte(`{"cases":[],"total":0}`), nil
		},
	}
	h := NewCaseHandler(client)
	const reqBody = `{"filters":{"tags":["micro-gw","ws-policy"]}}`
	r := withUser(httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(reqBody)))
	w := httptest.NewRecorder()
	h.SearchCases(w, r)

	assertStatus(t, w, http.StatusOK)
	if string(capturedBody) != reqBody {
		t.Errorf("upstream received body %s, want %s (must forward verbatim)", capturedBody, reqBody)
	}
}

func TestPatchCaseBestCaseFixEta(t *testing.T) {
	// bestCaseFixEta is a pure pass-through single-field PATCH variant, same shape as
	// the existing fixEta test. It does not trip the state/workState peek, so
	// patchCaseFn is invoked directly with the raw body and the upstream response
	// passes through unchanged.
	const testCaseID = "11111111-1111-1111-1111-111111111111"
	const reqBody = `{"bestCaseFixEta":"2026-08-01"}`
	const upstream = `{"message":"Case updated successfully","case":{"id":"` + testCaseID + `","updatedOn":"2026-07-23T10:00:00Z","bestCaseFixEta":"2026-08-01"}}`

	var capturedBody []byte
	client := &mockEntityCaseClient{
		patchCaseFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
			capturedBody = body
			return []byte(upstream), nil
		},
	}
	h := NewCaseHandler(client)
	r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(reqBody)))
	r.SetPathValue("id", testCaseID)
	w := httptest.NewRecorder()
	h.PatchCase(w, r)

	assertStatus(t, w, http.StatusOK)
	assertContentType(t, w, "application/json")

	if string(capturedBody) != reqBody {
		t.Errorf("upstream received body %s, want %s (must forward verbatim)", capturedBody, reqBody)
	}

	var wrapper struct {
		Case struct {
			BestCaseFixEta string `json:"bestCaseFixEta"`
		} `json:"case"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &wrapper); err != nil {
		t.Fatalf("decode response: %v; raw: %s", err, w.Body.String())
	}
	if wrapper.Case.BestCaseFixEta != "2026-08-01" {
		t.Errorf("case.bestCaseFixEta = %q, want %q", wrapper.Case.BestCaseFixEta, "2026-08-01")
	}
}

func TestPatchCaseMostLikelyFixEta(t *testing.T) {
	// mostLikelyFixEta is a pure pass-through single-field PATCH variant, same
	// shape as the existing fixEta test.
	const testCaseID = "11111111-1111-1111-1111-111111111111"
	const reqBody = `{"mostLikelyFixEta":"2026-08-01"}`
	const upstream = `{"message":"Case updated successfully","case":{"id":"` + testCaseID + `","updatedOn":"2026-07-23T10:00:00Z","mostLikelyFixEta":"2026-08-01"}}`

	var capturedBody []byte
	client := &mockEntityCaseClient{
		patchCaseFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
			capturedBody = body
			return []byte(upstream), nil
		},
	}
	h := NewCaseHandler(client)
	r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(reqBody)))
	r.SetPathValue("id", testCaseID)
	w := httptest.NewRecorder()
	h.PatchCase(w, r)

	assertStatus(t, w, http.StatusOK)
	assertContentType(t, w, "application/json")

	if string(capturedBody) != reqBody {
		t.Errorf("upstream received body %s, want %s (must forward verbatim)", capturedBody, reqBody)
	}

	var wrapper struct {
		Case struct {
			MostLikelyFixEta string `json:"mostLikelyFixEta"`
		} `json:"case"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &wrapper); err != nil {
		t.Fatalf("decode response: %v; raw: %s", err, w.Body.String())
	}
	if wrapper.Case.MostLikelyFixEta != "2026-08-01" {
		t.Errorf("case.mostLikelyFixEta = %q, want %q", wrapper.Case.MostLikelyFixEta, "2026-08-01")
	}
}

func TestPatchCaseWorstCaseFixEta(t *testing.T) {
	// worstCaseFixEta is a pure pass-through single-field PATCH variant, same
	// shape as the existing fixEta test.
	const testCaseID = "11111111-1111-1111-1111-111111111111"
	const reqBody = `{"worstCaseFixEta":"2026-08-01"}`
	const upstream = `{"message":"Case updated successfully","case":{"id":"` + testCaseID + `","updatedOn":"2026-07-23T10:00:00Z","worstCaseFixEta":"2026-08-01"}}`

	var capturedBody []byte
	client := &mockEntityCaseClient{
		patchCaseFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
			capturedBody = body
			return []byte(upstream), nil
		},
	}
	h := NewCaseHandler(client)
	r := withUser(httptest.NewRequest(http.MethodPatch, "/cases/"+testCaseID, strings.NewReader(reqBody)))
	r.SetPathValue("id", testCaseID)
	w := httptest.NewRecorder()
	h.PatchCase(w, r)

	assertStatus(t, w, http.StatusOK)
	assertContentType(t, w, "application/json")

	if string(capturedBody) != reqBody {
		t.Errorf("upstream received body %s, want %s (must forward verbatim)", capturedBody, reqBody)
	}

	var wrapper struct {
		Case struct {
			WorstCaseFixEta string `json:"worstCaseFixEta"`
		} `json:"case"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &wrapper); err != nil {
		t.Fatalf("decode response: %v; raw: %s", err, w.Body.String())
	}
	if wrapper.Case.WorstCaseFixEta != "2026-08-01" {
		t.Errorf("case.worstCaseFixEta = %q, want %q", wrapper.Case.WorstCaseFixEta, "2026-08-01")
	}
}

func TestGetCasePassesThroughNewFixEtaFields(t *testing.T) {
	// bestCaseFixEta, mostLikelyFixEta, and worstCaseFixEta are additive
	// entity-response fields with zero BFF handling — GetCase's injectNextStates
	// merge must not drop or alter them, matching the existing fixEta coverage.
	const testCaseID = "11111111-1111-1111-1111-111111111111"
	const upstreamBody = `{
		"id":"` + testCaseID + `",
		"state":"open",
		"bestCaseFixEta":"2026-07-28",
		"mostLikelyFixEta":"2026-08-01",
		"worstCaseFixEta":"2026-08-15"
	}`
	client := &mockEntityCaseClient{
		getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(upstreamBody), nil
		},
	}
	h := NewCaseHandler(client)
	r := withUser(httptest.NewRequest(http.MethodGet, "/cases/"+testCaseID, nil))
	r.SetPathValue("id", testCaseID)
	w := httptest.NewRecorder()
	h.GetCase(w, r)

	assertStatus(t, w, http.StatusOK)

	type resp struct {
		BestCaseFixEta   string `json:"bestCaseFixEta"`
		MostLikelyFixEta string `json:"mostLikelyFixEta"`
		WorstCaseFixEta  string `json:"worstCaseFixEta"`
	}
	got := decodeJSON[resp](t, w)

	if got.BestCaseFixEta != "2026-07-28" {
		t.Errorf("bestCaseFixEta = %q, want 2026-07-28", got.BestCaseFixEta)
	}
	if got.MostLikelyFixEta != "2026-08-01" {
		t.Errorf("mostLikelyFixEta = %q, want 2026-08-01", got.MostLikelyFixEta)
	}
	if got.WorstCaseFixEta != "2026-08-15" {
		t.Errorf("worstCaseFixEta = %q, want 2026-08-15", got.WorstCaseFixEta)
	}
}

func TestSearchCasesPassesThroughNewFixEtaFields(t *testing.T) {
	// Item: the 3 new fix-ETA fields flow through SearchCases's response verbatim,
	// same zero-BFF-handling pattern as fixEta/tags on GetCase.
	const upstream = `{"cases":[{"id":"11111111-1111-1111-1111-111111111111","bestCaseFixEta":"2026-07-28","mostLikelyFixEta":"2026-08-01","worstCaseFixEta":"2026-08-15"}],"total":1}`
	client := &mockEntityCaseClient{
		searchCasesFn: func(_ context.Context, _ []byte) ([]byte, error) {
			return []byte(upstream), nil
		},
	}
	h := NewCaseHandler(client)
	r := withUser(httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(`{}`)))
	w := httptest.NewRecorder()
	h.SearchCases(w, r)

	assertStatus(t, w, http.StatusOK)

	type resp struct {
		Cases []struct {
			BestCaseFixEta   string `json:"bestCaseFixEta"`
			MostLikelyFixEta string `json:"mostLikelyFixEta"`
			WorstCaseFixEta  string `json:"worstCaseFixEta"`
		} `json:"cases"`
	}
	got := decodeJSON[resp](t, w)
	if len(got.Cases) != 1 {
		t.Fatalf("cases = %+v, want 1 entry", got.Cases)
	}
	if got.Cases[0].BestCaseFixEta != "2026-07-28" {
		t.Errorf("bestCaseFixEta = %q, want 2026-07-28", got.Cases[0].BestCaseFixEta)
	}
	if got.Cases[0].MostLikelyFixEta != "2026-08-01" {
		t.Errorf("mostLikelyFixEta = %q, want 2026-08-01", got.Cases[0].MostLikelyFixEta)
	}
	if got.Cases[0].WorstCaseFixEta != "2026-08-15" {
		t.Errorf("worstCaseFixEta = %q, want 2026-08-15", got.Cases[0].WorstCaseFixEta)
	}
}

func TestSearchTags(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPost, "/tags/search", strings.NewReader(`{"filters":{"searchQuery":"micro"}}`))
		w := httptest.NewRecorder()
		h.SearchTags(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects a malformed JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/tags/search", strings.NewReader(`{"filters":`)))
		w := httptest.NewRecorder()
		h.SearchTags(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	// The BFF is a pass-through here: it must hand the entity service the exact
	// bytes it received. Asserting on the raw body (not a struct decoded through
	// the same json tags) is what actually pins the wire format -- a decode-based
	// check agrees with whichever key the payload happens to carry.
	t.Run("forwards the request body byte-for-byte", func(t *testing.T) {
		const reqBody = `{"filters":{"searchQuery":"micro"},"limit":20}`
		var capturedBody []byte
		client := &mockEntityCaseClient{
			searchTagsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"tags":[{"id":"22222222-2222-2222-2222-222222222222","label":"micro-gw","color":null}]}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/tags/search", strings.NewReader(reqBody)))
		w := httptest.NewRecorder()
		h.SearchTags(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != reqBody {
			t.Errorf("upstream received body %s, want %s", capturedBody, reqBody)
		}

		var body struct {
			Tags []struct {
				ID    string  `json:"id"`
				Label string  `json:"label"`
				Color *string `json:"color"`
			} `json:"tags"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v; raw: %s", err, w.Body.String())
		}
		if len(body.Tags) != 1 || body.Tags[0].Label != "micro-gw" {
			t.Errorf("tags = %+v, want a single micro-gw entry", body.Tags)
		}
	})

	t.Run("works with an empty filter object", func(t *testing.T) {
		var capturedBody []byte
		var called bool
		client := &mockEntityCaseClient{
			searchTagsFn: func(_ context.Context, body []byte) ([]byte, error) {
				called = true
				capturedBody = body
				return []byte(`{"tags":[]}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/tags/search", strings.NewReader(`{}`)))
		w := httptest.NewRecorder()
		h.SearchTags(w, r)

		assertStatus(t, w, http.StatusOK)
		if !called {
			t.Fatal("expected entity SearchTags to be called")
		}
		if string(capturedBody) != `{}` {
			t.Errorf("upstream received body %s, want {}", capturedBody)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to search tags.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					searchTagsFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/tags/search", strings.NewReader(`{"filters":{"searchQuery":"micro"}}`)))
				w := httptest.NewRecorder()
				h.SearchTags(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

// TestSearchTagsQuery covers the deprecated GET /tags/search alias. It is kept
// for one release so this service and its callers can be deployed independently
// rather than in lockstep; delete these tests with the handler.
func TestSearchTagsQuery(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodGet, "/tags/search?q=micro&limit=5", nil)
		w := httptest.NewRecorder()
		h.SearchTagsQuery(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	// The whole point of the alias: whichever form a caller uses, exactly one
	// request shape leaves this service. Asserting on the raw outbound bytes
	// (not a struct decoded through the same json tags) is what pins that -- a
	// decode-based check would agree with whichever key the payload carries.
	t.Run("sends the same body as the equivalent POST", func(t *testing.T) {
		const postBody = `{"filters":{"searchQuery":"micro"},"limit":5}`

		var postCaptured []byte
		postClient := &mockEntityCaseClient{
			searchTagsFn: func(_ context.Context, body []byte) ([]byte, error) {
				postCaptured = body
				return []byte(`{"tags":[]}`), nil
			},
		}
		pr := withUser(httptest.NewRequest(http.MethodPost, "/tags/search", strings.NewReader(postBody)))
		pw := httptest.NewRecorder()
		NewCaseHandler(postClient).SearchTags(pw, pr)
		assertStatus(t, pw, http.StatusOK)

		var getCaptured []byte
		getClient := &mockEntityCaseClient{
			searchTagsFn: func(_ context.Context, body []byte) ([]byte, error) {
				getCaptured = body
				return []byte(`{"tags":[]}`), nil
			},
		}
		gr := withUser(httptest.NewRequest(http.MethodGet, "/tags/search?q=micro&limit=5", nil))
		gw := httptest.NewRecorder()
		NewCaseHandler(getClient).SearchTagsQuery(gw, gr)
		assertStatus(t, gw, http.StatusOK)
		assertContentType(t, gw, "application/json")

		if string(postCaptured) != postBody {
			t.Fatalf("POST forwarded %s, want %s", postCaptured, postBody)
		}
		if string(getCaptured) != string(postCaptured) {
			t.Errorf("GET alias forwarded %s, want the POST's %s", getCaptured, postCaptured)
		}
		if gw.Body.String() != pw.Body.String() {
			t.Errorf("GET alias returned %s, want the POST's %s", gw.Body.String(), pw.Body.String())
		}
	})

	t.Run("omitted parameters send the zero body", func(t *testing.T) {
		var captured []byte
		client := &mockEntityCaseClient{
			searchTagsFn: func(_ context.Context, body []byte) ([]byte, error) {
				captured = body
				return []byte(`{"tags":[]}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/tags/search", nil))
		w := httptest.NewRecorder()
		h.SearchTagsQuery(w, r)

		assertStatus(t, w, http.StatusOK)
		const want = `{"filters":{"searchQuery":""},"limit":0}`
		if string(captured) != want {
			t.Errorf("upstream received body %s, want %s", captured, want)
		}
	})

	t.Run("rejects a non-numeric limit", func(t *testing.T) {
		var called bool
		client := &mockEntityCaseClient{
			searchTagsFn: func(_ context.Context, _ []byte) ([]byte, error) {
				called = true
				return []byte(`{"tags":[]}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/tags/search?q=micro&limit=abc", nil))
		w := httptest.NewRecorder()
		h.SearchTagsQuery(w, r)

		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
		if called {
			t.Error("expected the request to be rejected before the upstream call")
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to search tags.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					searchTagsFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/tags/search?q=micro", nil))
				w := httptest.NewRecorder()
				h.SearchTagsQuery(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestAggregateCases(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPost, "/cases/aggregate", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.AggregateCases(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/aggregate", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.AggregateCases(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/aggregate", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.AggregateCases(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 200 with response", func(t *testing.T) {
		const reqPayload = `{"filters":{},"groupBy":"state","maxGroups":12}`
		var capturedBody []byte
		client := &mockEntityCaseClient{
			aggregateCasesFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"groups":[{"key":"open","label":"Open","count":3}],"othersCount":1,"totalRecords":4}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/aggregate", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.AggregateCases(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["totalRecords"] != float64(4) {
			t.Errorf("totalRecords = %v, want 4", resp["totalRecords"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to aggregate cases.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					aggregateCasesFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/aggregate", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.AggregateCases(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestSearchFeedback(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPost, "/cases/feedback/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchFeedback(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/feedback/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchFeedback(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/feedback/search", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SearchFeedback(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 200", func(t *testing.T) {
		reqPayload := `{"filters":{"accountIds":["acc-1"],"dateFrom":"2026-01-01","dateTo":"2026-02-01"},"page":1,"pageSize":20}`
		var capturedBody []byte
		client := &mockEntityCaseClient{
			searchFeedbackFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"results":[{"instanceId":"fb-1","caseId":"case-1","rating":5,"ratingLabel":"Satisfied","comment":null,"submittedAt":"2026-01-15T00:00:00Z"}],"totalRecords":1}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/feedback/search", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.SearchFeedback(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["totalRecords"] != float64(1) {
			t.Errorf("totalRecords = %v, want 1", resp["totalRecords"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to search case feedback.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					searchFeedbackFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/feedback/search", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.SearchFeedback(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestAggregateFeedback(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPost, "/cases/feedback/aggregate", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.AggregateFeedback(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/feedback/aggregate", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.AggregateFeedback(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/feedback/aggregate", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.AggregateFeedback(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 200", func(t *testing.T) {
		reqPayload := `{"filters":{"accountIds":["acc-1"]},"bucket":"week"}`
		var capturedBody []byte
		client := &mockEntityCaseClient{
			aggregateFeedbackFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"buckets":[{"bucketStart":"2026-01-05","avgRating":4.2,"count":3}],"totalRecords":3}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/feedback/aggregate", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.AggregateFeedback(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["totalRecords"] != float64(3) {
			t.Errorf("totalRecords = %v, want 3", resp["totalRecords"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to aggregate case feedback.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					aggregateFeedbackFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/feedback/aggregate", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.AggregateFeedback(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
