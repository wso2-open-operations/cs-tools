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
	"strconv"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

const testCRID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

func TestCreateChangeRequest(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := httptest.NewRequest(http.MethodPost, "/change-requests", strings.NewReader(`{"subject":"Deploy v2","type":"normal"}`))
		w := httptest.NewRecorder()
		h.CreateChangeRequest(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.CreateChangeRequest(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("requires a type of standard, normal or emergency", func(t *testing.T) {
		for name, payload := range map[string]string{
			"missing":      `{"subject":"Deploy v2"}`,
			"empty":        `{"subject":"Deploy v2","type":""}`,
			"not a string": `{"subject":"Deploy v2","type":3}`,
			"azure":        `{"subject":"Deploy v2","type":"azure"}`,
			"bogus":        `{"subject":"Deploy v2","type":"bogus"}`,
		} {
			t.Run(name, func(t *testing.T) {
				called := false
				client := &mockEntityChangeRequestClient{
					createChangeRequestFn: func(_ context.Context, _ []byte) ([]byte, error) {
						called = true
						return nil, nil
					},
				}
				h := NewChangeRequestHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests", strings.NewReader(payload)))
				w := httptest.NewRecorder()
				h.CreateChangeRequest(w, r)
				assertStatus(t, w, http.StatusBadRequest)
				if called {
					t.Error("upstream was called for a change request without a valid type")
				}
				if msg := w.Body.String(); !strings.Contains(msg, "standard, normal or emergency") {
					t.Errorf("message %q should name the allowed types", msg)
				}
			})
		}
	})

	t.Run("accepts each creatable type", func(t *testing.T) {
		for _, typ := range []string{"standard", "normal", "emergency"} {
			client := &mockEntityChangeRequestClient{
				createChangeRequestFn: func(_ context.Context, _ []byte) ([]byte, error) {
					return []byte(`{"message":"ok"}`), nil
				},
			}
			h := NewChangeRequestHandler(client)
			r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests", strings.NewReader(`{"subject":"x","type":"`+typ+`"}`)))
			w := httptest.NewRecorder()
			h.CreateChangeRequest(w, r)
			assertStatus(t, w, http.StatusCreated)
		}
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.CreateChangeRequest(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 201 with response", func(t *testing.T) {
		const reqPayload = `{"subject":"Deploy v2","type":"normal"}`
		var capturedBody []byte
		client := &mockEntityChangeRequestClient{
			createChangeRequestFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"Change request created.","changeRequest":{"id":"` + testCRID + `","number":"CHG0001","createdOn":"2026-01-01T00:00:00Z","createdBy":"user@example.com"}}`), nil
			},
		}
		h := NewChangeRequestHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.CreateChangeRequest(w, r)

		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		// Create uses mapUpstreamError (like PATCH), not the Generic variant: the
		// entity service's 400 explains why a project / deployments /
		// environments combination was refused, and the form must show it. 5xx
		// and unmapped statuses still map to the generic message.
		for _, tc := range upstreamErrors("Failed to create change request.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityChangeRequestClient{
					createChangeRequestFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewChangeRequestHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests", strings.NewReader(`{"subject":"Deploy v2","type":"normal"}`)))
				w := httptest.NewRecorder()
				h.CreateChangeRequest(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestGetChangeRequest(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := httptest.NewRequest(http.MethodGet, "/change-requests/"+testCRID, nil)
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.GetChangeRequest(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/change-requests/not-a-uuid", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetChangeRequest(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty id", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/change-requests/", nil))
		w := httptest.NewRecorder()
		h.GetChangeRequest(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards id to upstream and returns 200", func(t *testing.T) {
		var capturedID string
		client := &mockEntityChangeRequestClient{
			getChangeRequestFn: func(_ context.Context, id string) ([]byte, error) {
				capturedID = id
				return []byte(`{"id":"` + testCRID + `","number":"CHG001","state":"scheduled","legalNextStates":["implement","canceled"]}`), nil
			},
		}
		h := NewChangeRequestHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/change-requests/"+testCRID, nil))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.GetChangeRequest(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedID != testCRID {
			t.Errorf("upstream received id %q, want %q", capturedID, testCRID)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["number"] != "CHG001" {
			t.Errorf("response number = %v, want CHG001", resp["number"])
		}
		legalNextStates, ok := resp["legalNextStates"].([]any)
		if !ok || len(legalNextStates) != 2 {
			t.Fatalf("legalNextStates = %v, want 2 entries", resp["legalNextStates"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to retrieve change request.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityChangeRequestClient{
					getChangeRequestFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewChangeRequestHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/change-requests/"+testCRID, nil))
				r.SetPathValue("id", testCRID)
				w := httptest.NewRecorder()
				h.GetChangeRequest(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestPatchChangeRequest(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := httptest.NewRequest(http.MethodPatch, "/change-requests/"+testCRID, strings.NewReader(`{"requestApproval":true}`))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.PatchChangeRequest(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/change-requests/not-a-uuid", strings.NewReader(`{"requestApproval":true}`)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.PatchChangeRequest(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty id", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/change-requests/", strings.NewReader(`{"requestApproval":true}`)))
		w := httptest.NewRecorder()
		h.PatchChangeRequest(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/change-requests/"+testCRID, strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.PatchChangeRequest(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/change-requests/"+testCRID, strings.NewReader(`not-json`)))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.PatchChangeRequest(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards requestApproval to upstream and returns 200 with legalNextStates", func(t *testing.T) {
		const reqPayload = `{"requestApproval":true}`
		var capturedID string
		var capturedBody []byte
		client := &mockEntityChangeRequestClient{
			patchChangeRequestFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedID = id
				capturedBody = body
				return []byte(`{"id":"` + testCRID + `","number":"CHG0001","state":"assess","legalNextStates":["authorize","canceled"]}`), nil
			},
		}
		h := NewChangeRequestHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/change-requests/"+testCRID, strings.NewReader(reqPayload)))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.PatchChangeRequest(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedID != testCRID {
			t.Errorf("upstream received id %q, want %q", capturedID, testCRID)
		}
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}

		resp := decodeJSON[map[string]any](t, w)
		legalNextStates, ok := resp["legalNextStates"].([]any)
		if !ok || len(legalNextStates) != 2 {
			t.Fatalf("legalNextStates = %v, want 2 entries", resp["legalNextStates"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to update change request.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityChangeRequestClient{
					patchChangeRequestFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewChangeRequestHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPatch, "/change-requests/"+testCRID, strings.NewReader(`{"requestApproval":true}`)))
				r.SetPathValue("id", testCRID)
				w := httptest.NewRecorder()
				h.PatchChangeRequest(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestGetChangeRequestApprovals(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := httptest.NewRequest(http.MethodGet, "/change-requests/"+testCRID+"/approvals", nil)
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.GetChangeRequestApprovals(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/change-requests/not-a-uuid/approvals", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetChangeRequestApprovals(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty id", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/change-requests//approvals", nil))
		w := httptest.NewRecorder()
		h.GetChangeRequestApprovals(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards id to upstream and returns 200", func(t *testing.T) {
		var capturedID string
		client := &mockEntityChangeRequestClient{
			getChangeRequestApprovalsFn: func(_ context.Context, id string) ([]byte, error) {
				capturedID = id
				return []byte(`{"approvals":[{"stage":"Assess","approverType":"STATIC_GROUP","approverName":"Devops Approval","status":"APPROVED","approvers":[{"id":"11111111-1111-1111-1111-111111111111","name":"Thenuka Keerthibandara","status":"APPROVED","respondedOn":"2026-07-08 06:02:56"}]}]}`), nil
			},
		}
		h := NewChangeRequestHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/change-requests/"+testCRID+"/approvals", nil))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.GetChangeRequestApprovals(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedID != testCRID {
			t.Errorf("upstream received id %q, want %q", capturedID, testCRID)
		}
		resp := decodeJSON[map[string]any](t, w)
		approvals, ok := resp["approvals"].([]any)
		if !ok || len(approvals) != 1 {
			t.Fatalf("approvals = %v, want 1 entry", resp["approvals"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to retrieve change request approvals.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityChangeRequestClient{
					getChangeRequestApprovalsFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewChangeRequestHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/change-requests/"+testCRID+"/approvals", nil))
				r.SetPathValue("id", testCRID)
				w := httptest.NewRecorder()
				h.GetChangeRequestApprovals(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestDecideChangeRequestApproval(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/approvals/decision", strings.NewReader(`{"decision":"approved"}`))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.DecideChangeRequestApproval(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/not-a-uuid/approvals/decision", strings.NewReader(`{"decision":"approved"}`)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.DecideChangeRequestApproval(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty id", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests//approvals/decision", strings.NewReader(`{"decision":"approved"}`)))
		w := httptest.NewRecorder()
		h.DecideChangeRequestApproval(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/approvals/decision", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.DecideChangeRequestApproval(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/approvals/decision", strings.NewReader(`not-json`)))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.DecideChangeRequestApproval(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects unknown fields", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/approvals/decision", strings.NewReader(`{"decision":"approved","comment":"lgtm"}`)))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.DecideChangeRequestApproval(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects trailing data after the JSON value", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/approvals/decision", strings.NewReader(`{"decision":"approved"}{"decision":"rejected"}`)))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.DecideChangeRequestApproval(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects a decision value outside approved/rejected", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/approvals/decision", strings.NewReader(`{"decision":"maybe"}`)))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.DecideChangeRequestApproval(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects an empty decision value", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/approvals/decision", strings.NewReader(`{}`)))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.DecideChangeRequestApproval(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 200 with response", func(t *testing.T) {
		const reqPayload = `{"decision":"approved"}`
		var capturedID string
		var capturedBody []byte
		client := &mockEntityChangeRequestClient{
			decideChangeRequestApprovalFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedID = id
				capturedBody = body
				return []byte(`{"id":"11111111-1111-1111-1111-111111111111","state":"approved"}`), nil
			},
		}
		h := NewChangeRequestHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/approvals/decision", strings.NewReader(reqPayload)))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.DecideChangeRequestApproval(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedID != testCRID {
			t.Errorf("upstream received id %q, want %q", capturedID, testCRID)
		}
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["state"] != "approved" {
			t.Errorf("state = %v, want approved", resp["state"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to submit change request approval decision.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityChangeRequestClient{
					decideChangeRequestApprovalFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewChangeRequestHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/approvals/decision", strings.NewReader(`{"decision":"approved"}`)))
				r.SetPathValue("id", testCRID)
				w := httptest.NewRecorder()
				h.DecideChangeRequestApproval(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})

	// A refusal to decide must tell the approver why (creator may not approve,
	// SRE members may not give peer approval); the reason comes from the entity
	// service's own error envelope. Anything else stays generic.
	t.Run("a 403 carrying the entity service's reason shows it", func(t *testing.T) {
		client := &mockEntityChangeRequestClient{
			decideChangeRequestApprovalFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusForbidden, Body: `{"code":403,"message":"the creator of a change request cannot approve it"}`}
			},
		}
		h := NewChangeRequestHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/approvals/decision", strings.NewReader(`{"decision":"approved"}`)))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.DecideChangeRequestApproval(w, r)
		assertStatus(t, w, http.StatusForbidden)
		assertErrorMessage(t, w, "the creator of a change request cannot approve it")
	})

	// A decision on an approval whose stage the change has moved past (Review's
	// approver while the change is in Customer Review / Closed) is a 409 from the
	// entity service whose reason is shown: the approver must be able to read why
	// the button no longer works. A 409 with no readable envelope stays generic.
	t.Run("a 409 carrying the entity service's reason shows it", func(t *testing.T) {
		const msg = "this approval is no longer pending: the change request is in Closed, but the Review stage can only be decided while it is in Review"
		client := &mockEntityChangeRequestClient{
			decideChangeRequestApprovalFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusConflict, Body: `{"code":409,"message":` + jsonQuote(msg) + `}`}
			},
		}
		h := NewChangeRequestHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/approvals/decision", strings.NewReader(`{"decision":"approved"}`)))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.DecideChangeRequestApproval(w, r)
		assertStatus(t, w, http.StatusConflict)
		assertErrorMessage(t, w, msg)
		assertContentType(t, w, "application/json")
	})

	t.Run("a 409 without a readable reason stays generic", func(t *testing.T) {
		for _, body := range []string{"", "conflict upstream message", `{"code":409}`} {
			client := &mockEntityChangeRequestClient{
				decideChangeRequestApprovalFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
					return nil, &apierror.Error{StatusCode: http.StatusConflict, Body: body}
				},
			}
			h := NewChangeRequestHandler(client)
			r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/approvals/decision", strings.NewReader(`{"decision":"approved"}`)))
			r.SetPathValue("id", testCRID)
			w := httptest.NewRecorder()
			h.DecideChangeRequestApproval(w, r)
			assertStatus(t, w, http.StatusConflict)
			assertErrorMessage(t, w, "Failed to submit change request approval decision.")
		}
	})
}

// Customer Approval / Customer Review are answered by the change's customer
// group (the registered contacts of its project) in the customer portal, never
// through this BFF. What a CSM user can still run into -- a decision on a live
// customer stage (they are no member of the group) and the manual state change
// it refuses -- must reach the caller readable.
func TestCustomerGroupApprovalMessages(t *testing.T) {
	t.Run("a CSM user's decision on a live customer stage is refused with the reason", func(t *testing.T) {
		const msg = `only members of the customer group (the registered contacts of this change request's project) can approve or reject the customer's approval of this change request`
		client := &mockEntityChangeRequestClient{
			decideChangeRequestApprovalFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusForbidden, Body: `{"code":403,"message":` + jsonQuote(msg) + `}`}
			},
		}
		h := NewChangeRequestHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/approvals/decision", strings.NewReader(`{"decision":"approved"}`)))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.DecideChangeRequestApproval(w, r)
		assertStatus(t, w, http.StatusForbidden)
		assertErrorMessage(t, w, msg)
	})

	t.Run("a manual scheduled/closed out of a customer state is a readable 400 that says what to do instead", func(t *testing.T) {
		const msg = `state "scheduled" cannot be set manually from customer_approval: the customer's approval can only be given by the customer in the Customer Portal; cancel the change or re-schedule it instead`
		client := &mockEntityChangeRequestClient{
			patchChangeRequestFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusBadRequest, Body: `{"code":400,"message":` + jsonQuote(msg) + `}`}
			},
		}
		h := NewChangeRequestHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/change-requests/"+testCRID, strings.NewReader(`{"state":"scheduled"}`)))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.PatchChangeRequest(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, msg)
	})
}

func jsonQuote(s string) string { b, _ := json.Marshal(s); return string(b) }

// assertErrorBodyKeys decodes the error body as the raw JSON object it is, so a
// test can tell a key that is absent from one that is empty.
func assertErrorBodyKeys(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v; raw: %s", err, w.Body.String())
	}
	return body
}

// The machine-readable name entity-service gave a refusal (errorCode in its error
// body) goes on to the CSM portal beside the message, on the PATCH and on the
// approval decision route, with the status and the message exactly as they were;
// a refusal that has none, or an unusable one, adds no key at all.
func TestUpstreamErrorCodesPassThrough(t *testing.T) {
	const onHold = "this change request is on hold, so a new implementation time cannot be proposed now"
	envelope := func(status int, msg, code string) string {
		out := `{"code":` + strconv.Itoa(status) + `,"message":` + jsonQuote(msg)
		if code != "" {
			out += `,"errorCode":` + code
		}
		return out + `}`
	}
	patch := func(upstream *apierror.Error) *httptest.ResponseRecorder {
		client := &mockEntityChangeRequestClient{
			patchChangeRequestFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) { return nil, upstream },
		}
		r := withUser(httptest.NewRequest(http.MethodPatch, "/change-requests/"+testCRID, strings.NewReader(`{"onHold":false}`)))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		NewChangeRequestHandler(client).PatchChangeRequest(w, r)
		return w
	}
	decide := func(upstream *apierror.Error) *httptest.ResponseRecorder {
		client := &mockEntityChangeRequestClient{
			decideChangeRequestApprovalFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) { return nil, upstream },
		}
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/approvals/decision", strings.NewReader(`{"decision":"approved"}`)))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		NewChangeRequestHandler(client).DecideChangeRequestApproval(w, r)
		return w
	}

	t.Run("PATCH: a 409 keeps its message and its code", func(t *testing.T) {
		w := patch(&apierror.Error{StatusCode: http.StatusConflict, Body: envelope(409, onHold, `"change_request_on_hold"`)})
		assertStatus(t, w, http.StatusConflict)
		body := assertErrorBodyKeys(t, w)
		if body["message"] != onHold || body["errorCode"] != "change_request_on_hold" {
			t.Errorf("body = %v", body)
		}
	})
	t.Run("PATCH: a 403 keeps the fixed message and the code", func(t *testing.T) {
		w := patch(&apierror.Error{StatusCode: http.StatusForbidden, Body: envelope(403, "only members asked may answer", `"change_request_not_asked"`)})
		assertStatus(t, w, http.StatusForbidden)
		body := assertErrorBodyKeys(t, w)
		if body["message"] != ErrMsgForbidden || body["errorCode"] != "change_request_not_asked" {
			t.Errorf("body = %v", body)
		}
	})
	t.Run("PATCH: a 400 keeps its message and its code", func(t *testing.T) {
		w := patch(&apierror.Error{StatusCode: http.StatusBadRequest, Body: envelope(400, "bad", `"change_request_forbidden"`)})
		assertStatus(t, w, http.StatusBadRequest)
		body := assertErrorBodyKeys(t, w)
		if body["message"] != "bad" || body["errorCode"] != "change_request_forbidden" {
			t.Errorf("body = %v", body)
		}
	})
	t.Run("decision: a 409 and a 403 keep their message and their code", func(t *testing.T) {
		const stale = "this approval is no longer pending: the change request is in Closed, but the Review stage can only be decided while it is in Review"
		w := decide(&apierror.Error{StatusCode: http.StatusConflict, Body: envelope(409, stale, `"change_request_approval_not_pending"`)})
		assertStatus(t, w, http.StatusConflict)
		if body := assertErrorBodyKeys(t, w); body["message"] != stale || body["errorCode"] != "change_request_approval_not_pending" {
			t.Errorf("409 body = %v", body)
		}
		const creator = "the creator of a change request cannot approve it"
		w = decide(&apierror.Error{StatusCode: http.StatusForbidden, Body: envelope(403, creator, `"change_request_forbidden"`)})
		assertStatus(t, w, http.StatusForbidden)
		if body := assertErrorBodyKeys(t, w); body["message"] != creator || body["errorCode"] != "change_request_forbidden" {
			t.Errorf("403 body = %v", body)
		}
	})
	t.Run("an upstream that names no code adds no key", func(t *testing.T) {
		for name, w := range map[string]*httptest.ResponseRecorder{
			"PATCH 409":    patch(&apierror.Error{StatusCode: http.StatusConflict, Body: envelope(409, "stale", "")}),
			"PATCH 403":    patch(&apierror.Error{StatusCode: http.StatusForbidden, Body: envelope(403, "no", "")}),
			"decision 409": decide(&apierror.Error{StatusCode: http.StatusConflict, Body: envelope(409, "stale", "")}),
			"decision 403": decide(&apierror.Error{StatusCode: http.StatusForbidden, Body: envelope(403, "no", "")}),
		} {
			if _, has := assertErrorBodyKeys(t, w)["errorCode"]; has {
				t.Errorf("%s: body %s carries an errorCode", name, w.Body.String())
			}
		}
	})
	t.Run("a code that is not a plain lower-case name is not passed on", func(t *testing.T) {
		for _, code := range []string{`"Change_Request_On_Hold"`, `"on hold"`, `"<script>"`, `7`, `null`, `""`, `"` + strings.Repeat("a", 65) + `"`} {
			w := patch(&apierror.Error{StatusCode: http.StatusConflict, Body: envelope(409, onHold, code)})
			assertStatus(t, w, http.StatusConflict)
			body := assertErrorBodyKeys(t, w)
			if _, has := body["errorCode"]; has {
				t.Errorf("code %s: passed on as %v", code, body["errorCode"])
			}
			if body["message"] != onHold {
				t.Errorf("code %s: message = %v, want it kept", code, body["message"])
			}
		}
	})
	t.Run("the other statuses carry no code", func(t *testing.T) {
		for _, status := range []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError, http.StatusServiceUnavailable} {
			w := patch(&apierror.Error{StatusCode: status, Body: envelope(status, "x", `"change_request_on_hold"`)})
			if _, has := assertErrorBodyKeys(t, w)["errorCode"]; has {
				t.Errorf("status %d: body %s carries an errorCode", status, w.Body.String())
			}
		}
	})
}

// The customer's answer is the customer's: the BFF refuses isCustomerApproved /
// isCustomerReviewed from its (staff) callers before any upstream call, true or
// false, alone or with a state, and lets everything else through untouched.
func TestPatchChangeRequestRefusesTheCustomersAnswer(t *testing.T) {
	const approved = "isCustomerApproved cannot be set on the customer's behalf: the customer's approval can only be given by the customer in the Customer Portal"
	const reviewed = "isCustomerReviewed cannot be set on the customer's behalf: the customer's review can only be given by the customer in the Customer Portal"
	for name, tc := range map[string]struct{ body, want string }{
		"approved true":        {`{"isCustomerApproved":true}`, approved},
		"approved false":       {`{"isCustomerApproved":false}`, approved},
		"reviewed true":        {`{"isCustomerReviewed":true}`, reviewed},
		"reviewed false":       {`{"isCustomerReviewed":false}`, reviewed},
		"with a state":         {`{"state":"scheduled","isCustomerApproved":true}`, approved},
		"with another field":   {`{"title":"x","isCustomerReviewed":true}`, reviewed},
		"approval named first": {`{"isCustomerReviewed":true,"isCustomerApproved":true}`, approved},
		"not even as a string": {`{"isCustomerApproved":"true"}`, approved},
		// encoding/json, which the entity service decodes the body with, matches a
		// key to a field without regard to case: every spelling is the same flag.
		"approved in capitals":   {`{"ISCUSTOMERAPPROVED":true}`, approved},
		"reviewed in lower case": {`{"iscustomerreviewed":true}`, reviewed},
		"approved, mixed case":   {`{"isCUSTOMERApproved":false}`, approved},
		"reviewed, mixed case":   {`{"IsCustomerReviewed":true}`, reviewed},
		// A body that names the flag twice is read as the last one: null first and
		// true after, or true first and null after, is a flag either way.
		"a null then a true in another case": {`{"isCustomerApproved":null,"ISCUSTOMERAPPROVED":true}`, approved},
		"a true then a null in another case": {`{"ISCUSTOMERAPPROVED":true,"isCustomerApproved":null}`, approved},
		"reviewed null then true":            {`{"isCustomerReviewed":null,"iscustomerreviewed":false}`, reviewed},
		"a JSON-escaped key":                 {`{"isCustomerApprov\u0065d":true}`, approved},
		// encoding/json folds a key by Unicode simple folding, not by ASCII case alone:
		// the long s (U+017F) is an "s" to it, as it is to strings.EqualFold.
		"a key spelled with a long s": {`{"i\u017fCu\u017ftomerReviewed":true}`, reviewed},
		"with a state, in capitals":   {`{"state":"closed","ISCUSTOMERREVIEWED":true}`, reviewed},
	} {
		t.Run(name, func(t *testing.T) {
			called := false
			client := &mockEntityChangeRequestClient{
				patchChangeRequestFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
					called = true
					return []byte(`{}`), nil
				},
			}
			h := NewChangeRequestHandler(client)
			r := withUser(httptest.NewRequest(http.MethodPatch, "/change-requests/"+testCRID, strings.NewReader(tc.body)))
			r.SetPathValue("id", testCRID)
			w := httptest.NewRecorder()
			h.PatchChangeRequest(w, r)
			assertStatus(t, w, http.StatusBadRequest)
			assertErrorMessage(t, w, tc.want)
			if called {
				t.Fatal("the entity service was called for a request the BFF refuses")
			}
		})
	}
	for name, body := range map[string]string{
		"a state alone":             `{"state":"canceled"}`,
		"re-schedule":               `{"state":"authorize","plannedStartOn":"2030-03-01 09:00:00"}`,
		"a null flag is not a flag": `{"title":"x","isCustomerApproved":null}`,
		"the requirement boxes":     `{"customerApprovalRequired":true}`,
		// null in every spelling is still absent.
		"nulls in two spellings": `{"title":"x","isCustomerApproved":null,"ISCUSTOMERAPPROVED":null,"IsCustomerReviewed":null}`,
		// A flag-like key that is not the flag in any case.
		"a different key": `{"title":"x","isCustomerApprovedBy":true}`,
	} {
		t.Run("passes "+name, func(t *testing.T) {
			called := false
			client := &mockEntityChangeRequestClient{
				patchChangeRequestFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
					called = true
					return []byte(`{}`), nil
				},
			}
			h := NewChangeRequestHandler(client)
			r := withUser(httptest.NewRequest(http.MethodPatch, "/change-requests/"+testCRID, strings.NewReader(body)))
			r.SetPathValue("id", testCRID)
			w := httptest.NewRecorder()
			h.PatchChangeRequest(w, r)
			assertStatus(t, w, http.StatusOK)
			if !called {
				t.Fatal("the PATCH never reached the entity service")
			}
		})
	}
}

func TestSearchChangeRequests(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := httptest.NewRequest(http.MethodPost, "/change-requests/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchChangeRequests(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchChangeRequests(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/search", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SearchChangeRequests(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 200", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityChangeRequestClient{
			searchChangeRequestsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"changeRequests":[{"id":"cr-1"}],"total":1,"limit":20,"offset":0}`), nil
			},
		}
		h := NewChangeRequestHandler(client)
		const payload = `{"filters":{"states":["scheduled"]},"pagination":{"limit":20,"offset":0}}`
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/search", strings.NewReader(payload)))
		w := httptest.NewRecorder()
		h.SearchChangeRequests(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if string(capturedBody) != payload {
			t.Errorf("upstream received body %q, want %q", capturedBody, payload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["total"] != float64(1) {
			t.Errorf("total = %v, want 1", resp["total"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to search change requests.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityChangeRequestClient{
					searchChangeRequestsFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewChangeRequestHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/search", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.SearchChangeRequests(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestCreateChangeRequestComment(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/comments", strings.NewReader(`{"type":"comment","content":"hi"}`))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.CreateChangeRequestComment(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/not-a-uuid/comments", strings.NewReader(`{"type":"comment","content":"hi"}`)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.CreateChangeRequestComment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/comments", strings.NewReader(`not-json`)))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.CreateChangeRequestComment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("injects referenceId and referenceType and forwards to the generic comment endpoint", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityChangeRequestClient{
			getChangeRequestFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"id":"` + testCRID + `"}`), nil
			},
			createCommentFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"Comment created.","comment":{"id":"11111111-1111-1111-1111-111111111111","createdOn":"2026-01-01T00:00:00Z","createdBy":"user@example.com"}}`), nil
			},
		}
		h := NewChangeRequestHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/comments", strings.NewReader(`{"type":"comment","content":"hi"}`)))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.CreateChangeRequestComment(w, r)

		assertStatus(t, w, http.StatusCreated)
		if !strings.Contains(string(capturedBody), `"referenceId":"`+testCRID+`"`) {
			t.Errorf("expected referenceId to be injected, got %q", capturedBody)
		}
		if !strings.Contains(string(capturedBody), `"referenceType":"change_request"`) {
			t.Errorf("expected referenceType change_request to be injected, got %q", capturedBody)
		}
	})

	t.Run("upstream GetChangeRequest error is mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to create change request comment.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityChangeRequestClient{
					getChangeRequestFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewChangeRequestHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/comments", strings.NewReader(`{"type":"comment","content":"hi"}`)))
				r.SetPathValue("id", testCRID)
				w := httptest.NewRecorder()
				h.CreateChangeRequestComment(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})

	t.Run("upstream CreateComment error is mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to create change request comment.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityChangeRequestClient{
					getChangeRequestFn: func(_ context.Context, _ string) ([]byte, error) {
						return []byte(`{"id":"` + testCRID + `"}`), nil
					},
					createCommentFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewChangeRequestHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/comments", strings.NewReader(`{"type":"comment","content":"hi"}`)))
				r.SetPathValue("id", testCRID)
				w := httptest.NewRecorder()
				h.CreateChangeRequestComment(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}

func TestSearchChangeRequestComments(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/comments/search", strings.NewReader(`{}`))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.SearchChangeRequestComments(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("injects referenceId and referenceType and forwards to the generic search endpoint", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityChangeRequestClient{
			searchCommentsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"comments":[],"total":0,"limit":20,"offset":0}`), nil
			},
		}
		h := NewChangeRequestHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/"+testCRID+"/comments/search", strings.NewReader(`{"pagination":{"offset":0,"limit":20}}`)))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.SearchChangeRequestComments(w, r)

		assertStatus(t, w, http.StatusOK)
		if !strings.Contains(string(capturedBody), `"referenceId":"`+testCRID+`"`) {
			t.Errorf("expected referenceId to be injected, got %q", capturedBody)
		}
		if !strings.Contains(string(capturedBody), `"referenceType":"change_request"`) {
			t.Errorf("expected referenceType change_request to be injected, got %q", capturedBody)
		}
	})
}

func TestAggregateChangeRequests(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := httptest.NewRequest(http.MethodPost, "/change-requests/aggregate", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.AggregateChangeRequests(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/aggregate", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.AggregateChangeRequests(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/aggregate", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.AggregateChangeRequests(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 200 with response", func(t *testing.T) {
		const reqPayload = `{"filters":{},"groupBy":"state","maxGroups":12}`
		var capturedBody []byte
		client := &mockEntityChangeRequestClient{
			aggregateChangeRequestsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"groups":[{"key":"open","label":"Open","count":6}],"othersCount":1,"totalRecords":7}`), nil
			},
		}
		h := NewChangeRequestHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/aggregate", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.AggregateChangeRequests(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["totalRecords"] != float64(7) {
			t.Errorf("totalRecords = %v, want 7", resp["totalRecords"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to aggregate change requests.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityChangeRequestClient{
					aggregateChangeRequestsFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewChangeRequestHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests/aggregate", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.AggregateChangeRequests(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

// The creation form's "Customer Approval" / "Customer Review" checkboxes
// (customerApprovalRequired / customerReviewRequired) pass through the BFF
// untouched; only their type is checked here.
func TestCreateChangeRequest_CustomerGateFlags(t *testing.T) {
	t.Run("forwards both checkboxes to the entity service unchanged", func(t *testing.T) {
		const reqPayload = `{"subject":"Deploy v2","type":"normal","customerApprovalRequired":true,"customerReviewRequired":false}`
		var capturedBody []byte
		client := &mockEntityChangeRequestClient{
			createChangeRequestFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"ok"}`), nil
			},
		}
		h := NewChangeRequestHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.CreateChangeRequest(w, r)
		assertStatus(t, w, http.StatusCreated)
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
	})

	t.Run("rejects a checkbox that is not a boolean", func(t *testing.T) {
		for name, payload := range map[string]string{
			"approval as string":  `{"subject":"x","type":"normal","customerApprovalRequired":"yes"}`,
			"review as number":    `{"subject":"x","type":"normal","customerReviewRequired":1}`,
			"approval as null":    `{"subject":"x","type":"normal","customerApprovalRequired":null}`,
			"review as object":    `{"subject":"x","type":"normal","customerReviewRequired":{}}`,
			"one good, one wrong": `{"subject":"x","type":"normal","customerApprovalRequired":true,"customerReviewRequired":"true"}`,
		} {
			t.Run(name, func(t *testing.T) {
				called := false
				client := &mockEntityChangeRequestClient{
					createChangeRequestFn: func(_ context.Context, _ []byte) ([]byte, error) {
						called = true
						return nil, nil
					},
				}
				h := NewChangeRequestHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests", strings.NewReader(payload)))
				w := httptest.NewRecorder()
				h.CreateChangeRequest(w, r)
				assertStatus(t, w, http.StatusBadRequest)
				if called {
					t.Error("upstream was called with a non-boolean checkbox")
				}
				if msg := w.Body.String(); !strings.Contains(msg, "must be a boolean") {
					t.Errorf("message %q should say the checkbox must be a boolean", msg)
				}
			})
		}
	})
}

func TestPatchChangeRequest_CustomerGateFlags(t *testing.T) {
	patch := func(h *ChangeRequestHandler, payload string) *httptest.ResponseRecorder {
		r := withUser(httptest.NewRequest(http.MethodPatch, "/change-requests/"+testCRID, strings.NewReader(payload)))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		h.PatchChangeRequest(w, r)
		return w
	}

	t.Run("forwards the checkboxes and returns them with legalNextStates", func(t *testing.T) {
		const reqPayload = `{"customerApprovalRequired":true,"customerReviewRequired":true}`
		var capturedBody []byte
		client := &mockEntityChangeRequestClient{
			patchChangeRequestFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"ok","changeRequest":{"state":"review","customerApprovalRequired":true,"customerReviewRequired":true,"legalNextStates":["customer_review","rollback","canceled"]}}`), nil
			},
		}
		w := patch(NewChangeRequestHandler(client), reqPayload)
		assertStatus(t, w, http.StatusOK)
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		cr, _ := resp["changeRequest"].(map[string]any)
		if cr["customerApprovalRequired"] != true || cr["customerReviewRequired"] != true {
			t.Errorf("response flags = %v/%v, want true/true", cr["customerApprovalRequired"], cr["customerReviewRequired"])
		}
	})

	// Roll back is a plain state PATCH (the entity service owns where it is
	// legal); the BFF forwards the body untouched.
	t.Run("forwards a rollback state change verbatim", func(t *testing.T) {
		const reqPayload = `{"state":"rollback"}`
		var capturedBody []byte
		client := &mockEntityChangeRequestClient{
			patchChangeRequestFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"ok","changeRequest":{"state":"rollback","legalNextStates":null}}`), nil
			},
		}
		w := patch(NewChangeRequestHandler(client), reqPayload)
		assertStatus(t, w, http.StatusOK)
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if cr, _ := resp["changeRequest"].(map[string]any); cr["state"] != "rollback" {
			t.Errorf("response state = %v, want rollback", cr["state"])
		}
	})

	// Re-schedule is a plain state PATCH carrying the new window (and an
	// optional work note); the BFF forwards the body untouched.
	t.Run("forwards a re-schedule (authorize + new window) verbatim", func(t *testing.T) {
		const reqPayload = `{"state":"authorize","plannedStartOn":"2030-03-08 09:00:00","plannedEndOn":"2030-03-08 11:00:00","workNote":"Customer asked for next week."}`
		var capturedBody []byte
		client := &mockEntityChangeRequestClient{
			patchChangeRequestFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"ok","changeRequest":{"state":"authorize","legalNextStates":["canceled"]}}`), nil
			},
		}
		w := patch(NewChangeRequestHandler(client), reqPayload)
		assertStatus(t, w, http.StatusOK)
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if cr, _ := resp["changeRequest"].(map[string]any); cr["state"] != "authorize" {
			t.Errorf("response state = %v, want authorize", cr["state"])
		}
	})

	t.Run("rejects a checkbox that is not a boolean", func(t *testing.T) {
		for name, payload := range map[string]string{
			"approval as string": `{"customerApprovalRequired":"true"}`,
			"review as null":     `{"customerReviewRequired":null}`,
			"review as number":   `{"title":"x","customerReviewRequired":0}`,
		} {
			t.Run(name, func(t *testing.T) {
				called := false
				client := &mockEntityChangeRequestClient{
					patchChangeRequestFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						called = true
						return nil, nil
					},
				}
				w := patch(NewChangeRequestHandler(client), payload)
				assertStatus(t, w, http.StatusBadRequest)
				if called {
					t.Error("upstream was called with a non-boolean checkbox")
				}
				if msg := w.Body.String(); !strings.Contains(msg, "must be a boolean") {
					t.Errorf("message %q should say the checkbox must be a boolean", msg)
				}
			})
		}
	})

	// The entity service's refusals are caller-actionable 400s and reach the
	// form verbatim -- an edit after the gate, and a manual transition that the
	// checkboxes rule out.
	t.Run("surfaces the entity service's refusal messages", func(t *testing.T) {
		for name, tc := range map[string]struct{ payload, msg string }{
			"edit after the approval gate": {
				`{"customerApprovalRequired":false}`,
				"customerApprovalRequired can no longer be changed: the change request has already passed the approval stage (current state: scheduled)",
			},
			"edit after review": {
				`{"customerReviewRequired":true}`,
				"customerReviewRequired can no longer be changed: the change request has already left the review stage (current state: closed)",
			},
			"Request Approval on a project nobody can be asked on": {
				`{"state":"assess"}`,
				"customer approval and customer review are required but nobody on this project can be asked (no registered contact other than the requester): register a contact for the project first",
			},
			"a box turned on after Request Approval on a project nobody can be asked on": {
				`{"customerReviewRequired":true}`,
				"customer review is required but nobody on this project can be asked (no registered contact other than the requester): register a contact for the project first",
			},
			"Request Approval with a box set and no Customer Project": {
				`{"state":"assess"}`,
				"approval cannot be requested: the customer's approval and/or review is required but no Customer Project is set, so there is nobody to ask. Select a Customer Project first (or clear the requirement).",
			},
			"customer_review when not required": {
				`{"state":"customer_review"}`,
				`state "customer_review" cannot be set: customer review is not required for this change request (customerReviewRequired is false); close it from review instead`,
			},
			"closed from review when required": {
				`{"state":"closed"}`,
				`state "closed" cannot be set from review: customer review is required for this change request (customerReviewRequired is true); move it to customer_review first`,
			},
			"authorize outside customer_approval": {
				`{"state":"authorize","plannedStartOn":"2030-03-08 09:00:00"}`,
				`state "authorize" cannot be set manually: it is reached automatically through the approval flow (Request Approval, then peer approval); it can only be set by hand to re-schedule a change from customer_approval`,
			},
			"re-schedule without a changed window": {
				`{"state":"authorize","plannedStartOn":"2030-03-01 09:00:00"}`,
				`re-scheduling requires a changed planned start or end: send plannedStartOn and/or plannedEndOn with a value different from the stored one`,
			},
			"rollback outside the review states": {
				`{"state":"rollback"}`,
				`state "rollback" can only be set from review or customer_review`,
			},
			"scheduled outside customer_approval": {
				`{"state":"scheduled"}`,
				`state "scheduled" cannot be set manually: it is reached automatically through the approval flow (Request Approval, then peer/CAB approval); it can only be set by hand to record the customer's approval, from customer_approval`,
			},
		} {
			t.Run(name, func(t *testing.T) {
				body, _ := json.Marshal(map[string]any{"code": 400, "message": tc.msg})
				client := &mockEntityChangeRequestClient{
					patchChangeRequestFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, &apierror.Error{StatusCode: http.StatusBadRequest, Body: string(body)}
					},
				}
				w := patch(NewChangeRequestHandler(client), tc.payload)
				assertStatus(t, w, http.StatusBadRequest)
				assertErrorMessage(t, w, tc.msg)
			})
		}
	})
}

func TestGetChangeRequest_ReturnsCustomerGateFlags(t *testing.T) {
	client := &mockEntityChangeRequestClient{
		getChangeRequestFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(`{"id":"` + testCRID + `","state":"customer_approval","customerApprovalRequired":true,"customerReviewRequired":false,"legalNextStates":["scheduled","canceled"]}`), nil
		},
	}
	h := NewChangeRequestHandler(client)
	r := withUser(httptest.NewRequest(http.MethodGet, "/change-requests/"+testCRID, nil))
	r.SetPathValue("id", testCRID)
	w := httptest.NewRecorder()
	h.GetChangeRequest(w, r)
	assertStatus(t, w, http.StatusOK)
	resp := decodeJSON[map[string]any](t, w)
	if resp["customerApprovalRequired"] != true || resp["customerReviewRequired"] != false {
		t.Errorf("flags = %v/%v, want true/false", resp["customerApprovalRequired"], resp["customerReviewRequired"])
	}
	states, _ := resp["legalNextStates"].([]any)
	if len(states) != 2 || states[0] != "scheduled" {
		t.Errorf("legalNextStates = %v, want [scheduled canceled] passed through untouched", states)
	}
}

const (
	scopeProjectID    = "11111111-2222-3333-4444-555555555555"
	scopeDeploymentID = "66666666-7777-8888-9999-aaaaaaaaaaaa"
)

// The customer-scope fields (project, deployments, deployment products) and the
// journal entries are shape-checked at the
// BFF on both create and PATCH, so a stray string/null reaches the form as a
// readable 400 instead of an upstream decode failure; valid ones are forwarded
// byte-for-byte.
func TestChangeRequestScopeFieldValidation(t *testing.T) {
	bad := map[string]string{
		`"projectId":"not-a-uuid"`:                       "projectId must be a UUID string",
		`"projectId":null`:                               "projectId must be a UUID string",
		`"projectId":7`:                                  "projectId must be a UUID string",
		`"deploymentIds":"` + scopeDeploymentID + `"`:    "deploymentIds must be an array of UUID strings",
		`"deploymentIds":null`:                           "deploymentIds must be an array of UUID strings",
		`"deploymentIds":{"0":"x"}`:                      "deploymentIds must be an array of UUID strings",
		`"deploymentIds":["not-a-uuid"]`:                 "deploymentIds must be an array of UUID strings",
		`"deploymentIds":[1]`:                            "deploymentIds must be an array of UUID strings",
		`"environmentIds":["` + scopeDeploymentID + `"]`: errMsgEnvironmentIDsRemoved,
		`"environmentIds":[]`:                            errMsgEnvironmentIDsRemoved,
		`"environmentIds":null`:                          errMsgEnvironmentIDsRemoved,
		`"customerGroupId":"` + scopeProjectID + `"`:     errMsgCustomerGroupIDRemoved,
		`"customerGroupId":null`:                         errMsgCustomerGroupIDRemoved,
		`"deploymentProductIds":true`:                    "deploymentProductIds must be an array of UUID strings",
		`"customerGroupId":"x"`:                          errMsgCustomerGroupIDRemoved,
		`"category":3`:                                   "category must be a string",
		`"comment":5`:                                    "comment must be a string",
		`"workNote":["a"]`:                               "workNote must be a string",
		`"deploymentIds":[` + strings.TrimSuffix(strings.Repeat(`"`+scopeDeploymentID+`",`, 101), ",") + `]`: "deploymentIds must contain at most 100 entries",
	}
	for field, wantMsg := range bad {
		field, wantMsg := field, wantMsg
		t.Run("create rejects "+field[:min(len(field), 40)], func(t *testing.T) {
			called := false
			client := &mockEntityChangeRequestClient{createChangeRequestFn: func(_ context.Context, _ []byte) ([]byte, error) {
				called = true
				return []byte(`{}`), nil
			}}
			h := NewChangeRequestHandler(client)
			r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests", strings.NewReader(`{"subject":"s","type":"normal",`+field+`}`)))
			w := httptest.NewRecorder()
			h.CreateChangeRequest(w, r)
			assertStatus(t, w, http.StatusBadRequest)
			assertErrorMessage(t, w, wantMsg)
			if called {
				t.Fatal("the entity service was called for a rejected body")
			}
		})
		t.Run("patch rejects "+field[:min(len(field), 40)], func(t *testing.T) {
			called := false
			client := &mockEntityChangeRequestClient{patchChangeRequestFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				called = true
				return []byte(`{}`), nil
			}}
			h := NewChangeRequestHandler(client)
			r := withUser(httptest.NewRequest(http.MethodPatch, "/change-requests/"+testCRID, strings.NewReader(`{`+field+`}`)))
			r.SetPathValue("id", testCRID)
			w := httptest.NewRecorder()
			h.PatchChangeRequest(w, r)
			assertStatus(t, w, http.StatusBadRequest)
			assertErrorMessage(t, w, wantMsg)
			if called {
				t.Fatal("the entity service was called for a rejected body")
			}
		})
	}

	t.Run("patch rejects blank journal entries, create ignores them", func(t *testing.T) {
		for _, field := range []string{`"comment":""`, `"comment":"  "`, `"workNote":"\n"`} {
			h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
			r := withUser(httptest.NewRequest(http.MethodPatch, "/change-requests/"+testCRID, strings.NewReader(`{`+field+`}`)))
			r.SetPathValue("id", testCRID)
			w := httptest.NewRecorder()
			h.PatchChangeRequest(w, r)
			assertStatus(t, w, http.StatusBadRequest)
		}
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests", strings.NewReader(`{"subject":"s","type":"normal","comment":"","workNote":" "}`)))
		w := httptest.NewRecorder()
		h.CreateChangeRequest(w, r)
		assertStatus(t, w, http.StatusCreated)
	})

	t.Run("valid scope fields are forwarded unchanged; null clears category on patch", func(t *testing.T) {
		createBody := `{"subject":"s","type":"normal","projectId":"` + scopeProjectID + `","deploymentIds":["` + scopeDeploymentID + `"],"deploymentProductIds":[],"category":"devops","comment":"c","workNote":"w"}`
		var gotCreate string
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{createChangeRequestFn: func(_ context.Context, b []byte) ([]byte, error) {
			gotCreate = string(b)
			return []byte(`{"changeRequest":{"id":"x"}}`), nil
		}})
		w := httptest.NewRecorder()
		h.CreateChangeRequest(w, withUser(httptest.NewRequest(http.MethodPost, "/change-requests", strings.NewReader(createBody))))
		assertStatus(t, w, http.StatusCreated)
		if gotCreate != createBody {
			t.Fatalf("forwarded create body = %s, want unchanged", gotCreate)
		}

		patchBody := `{"projectId":"` + scopeProjectID + `","deploymentIds":[],"category":null,"comment":"c"}`
		var gotPatch string
		h = NewChangeRequestHandler(&mockEntityChangeRequestClient{patchChangeRequestFn: func(_ context.Context, _ string, b []byte) ([]byte, error) {
			gotPatch = string(b)
			return []byte(`{}`), nil
		}})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/change-requests/"+testCRID, strings.NewReader(patchBody)))
		r.SetPathValue("id", testCRID)
		w = httptest.NewRecorder()
		h.PatchChangeRequest(w, r)
		assertStatus(t, w, http.StatusOK)
		if gotPatch != patchBody {
			t.Fatalf("forwarded patch body = %s, want unchanged", gotPatch)
		}
	})

	t.Run("a refused combination surfaces the entity service's message on create and patch", func(t *testing.T) {
		upstream := &apierror.Error{StatusCode: http.StatusBadRequest, Body: `{"message":"deploymentIds contains a deployment that does not belong to the selected project: ` + scopeDeploymentID + `"}`}
		want := "deploymentIds contains a deployment that does not belong to the selected project: " + scopeDeploymentID
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{
			createChangeRequestFn: func(_ context.Context, _ []byte) ([]byte, error) { return nil, upstream },
			patchChangeRequestFn:  func(_ context.Context, _ string, _ []byte) ([]byte, error) { return nil, upstream },
		})
		w := httptest.NewRecorder()
		h.CreateChangeRequest(w, withUser(httptest.NewRequest(http.MethodPost, "/change-requests", strings.NewReader(`{"subject":"s","type":"normal"}`))))
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, want)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/change-requests/"+testCRID, strings.NewReader(`{"comment":"c"}`)))
		r.SetPathValue("id", testCRID)
		w = httptest.NewRecorder()
		h.PatchChangeRequest(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, want)
	})
}

// POST /change-requests/link-options: the form's Customer Project ->
// Deployments -> Deployment products lookup, plus the read-only Customer Group.
func TestChangeRequestLinkOptions(t *testing.T) {
	post := func(h *ChangeRequestHandler, body string, authed bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/change-requests/link-options", strings.NewReader(body))
		if authed {
			r = withUser(r)
		}
		w := httptest.NewRecorder()
		h.GetChangeRequestLinkOptions(w, r)
		return w
	}

	t.Run("requires authenticated user", func(t *testing.T) {
		w := post(NewChangeRequestHandler(&mockEntityChangeRequestClient{}), `{"projectId":"`+scopeProjectID+`"}`, false)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	for name, tc := range map[string]struct{ body, msg string }{
		"not json":                {`nope`, ErrMsgBadRequest},
		"not an object":           {`[]`, ErrMsgBadRequest},
		"missing projectId":       {`{"deploymentIds":[]}`, "projectId is required"},
		"projectId not a uuid":    {`{"projectId":"p-1"}`, "projectId must be a UUID string"},
		"projectId null":          {`{"projectId":null}`, "projectId must be a UUID string"},
		"deploymentIds not array": {`{"projectId":"` + scopeProjectID + `","deploymentIds":"` + scopeDeploymentID + `"}`, "deploymentIds must be an array of UUID strings"},
		"deploymentIds bad entry": {`{"projectId":"` + scopeProjectID + `","deploymentIds":["x"]}`, "deploymentIds must be an array of UUID strings"},
	} {
		tc := tc
		t.Run("rejects "+name, func(t *testing.T) {
			called := false
			h := NewChangeRequestHandler(&mockEntityChangeRequestClient{getChangeRequestLinkOptionsFn: func(_ context.Context, _ []byte) ([]byte, error) {
				called = true
				return []byte(`{}`), nil
			}})
			w := post(h, tc.body, true)
			assertStatus(t, w, http.StatusBadRequest)
			assertErrorMessage(t, w, tc.msg)
			if called {
				t.Fatal("the entity service was called for a rejected body")
			}
		})
	}

	t.Run("forwards the body and returns the entity response as is, customerContacts included", func(t *testing.T) {
		body := `{"projectId":"` + scopeProjectID + `","deploymentIds":["` + scopeDeploymentID + `"]}`
		const resp = `{"deployments":[{"id":"d1","name":"Prod","type":"primary_production"}],"deploymentProducts":[{"id":"p1","name":"APIM 4.3.0","deployment":{"id":"d1","name":"Prod"}}],"customerContacts":[{"id":"c1","name":"Jane Doe","email":"jane.doe@example.com"}]}`
		var got string
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{getChangeRequestLinkOptionsFn: func(_ context.Context, b []byte) ([]byte, error) {
			got = string(b)
			return []byte(resp), nil
		}})
		w := post(h, body, true)
		assertStatus(t, w, http.StatusOK)
		if got != body {
			t.Fatalf("forwarded body = %s, want %s", got, body)
		}
		if strings.TrimSpace(w.Body.String()) != resp {
			t.Fatalf("response = %s, want the entity response", w.Body.String())
		}
	})

	t.Run("upstream errors are mapped: 400 message surfaced, 5xx generic", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to load change request options.") {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				h := NewChangeRequestHandler(&mockEntityChangeRequestClient{getChangeRequestLinkOptionsFn: func(_ context.Context, _ []byte) ([]byte, error) {
					return nil, tc.err
				}})
				w := post(h, `{"projectId":"`+scopeProjectID+`"}`, true)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}

// The BFF's body guards judge the body the way the entity service's decoder will
// read it: a key is the field whatever its case, so a guard that looked up one
// spelling only would be bypassed by another.
func TestChangeRequestBodyGuardsIgnoreTheCaseOfKeys(t *testing.T) {
	for name, tc := range map[string]struct {
		method, path, body, want string
	}{
		"create without a type, Type is the type": {"POST", "/change-requests", `{"subject":"s","Type":"azure"}`, "type is not allowed: a change request must be one of standard, normal or emergency"},
		"create with a good and a bad type":       {"POST", "/change-requests", `{"subject":"s","type":"normal","TYPE":"azure"}`, "type is not allowed: a change request must be one of standard, normal or emergency"},
		"create, gate flag in capitals":           {"POST", "/change-requests", `{"subject":"s","type":"normal","CUSTOMERAPPROVALREQUIRED":"yes"}`, "customerApprovalRequired must be a boolean (true or false)"},
		"patch, gate flag in lower case":          {"PATCH", "/change-requests/" + testCRID, `{"customerreviewrequired":1}`, "customerReviewRequired must be a boolean (true or false)"},
		"patch, removed field in capitals":        {"PATCH", "/change-requests/" + testCRID, `{"CUSTOMERGROUPID":"` + scopeProjectID + `"}`, errMsgCustomerGroupIDRemoved},
		"patch, removed field in lower case":      {"PATCH", "/change-requests/" + testCRID, `{"environmentids":[]}`, errMsgEnvironmentIDsRemoved},
		"patch, project in capitals":              {"PATCH", "/change-requests/" + testCRID, `{"PROJECTID":"nope"}`, "projectId must be a UUID string"},
		"patch, blank comment in capitals":        {"PATCH", "/change-requests/" + testCRID, `{"COMMENT":"  "}`, "comment must not be empty"},
	} {
		t.Run(name, func(t *testing.T) {
			called := false
			client := &mockEntityChangeRequestClient{
				createChangeRequestFn: func(_ context.Context, _ []byte) ([]byte, error) { called = true; return []byte(`{}`), nil },
				patchChangeRequestFn:  func(_ context.Context, _ string, _ []byte) ([]byte, error) { called = true; return []byte(`{}`), nil },
			}
			h := NewChangeRequestHandler(client)
			r := withUser(httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			w := httptest.NewRecorder()
			if tc.method == "POST" {
				h.CreateChangeRequest(w, r)
			} else {
				r.SetPathValue("id", testCRID)
				h.PatchChangeRequest(w, r)
			}
			assertStatus(t, w, http.StatusBadRequest)
			assertErrorMessage(t, w, tc.want)
			if called {
				t.Fatal("the entity service was called for a body the BFF refuses")
			}
		})
	}
	t.Run("a create whose type is in another case of the key is accepted", func(t *testing.T) {
		called := false
		client := &mockEntityChangeRequestClient{createChangeRequestFn: func(_ context.Context, _ []byte) ([]byte, error) {
			called = true
			return []byte(`{"message":"ok"}`), nil
		}}
		h := NewChangeRequestHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests", strings.NewReader(`{"subject":"s","Type":"normal"}`)))
		w := httptest.NewRecorder()
		h.CreateChangeRequest(w, r)
		assertStatus(t, w, http.StatusCreated)
		if !called {
			t.Fatal("the create never reached the entity service")
		}
	})
}
