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

const testCRID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

func TestCreateChangeRequest(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewChangeRequestHandler(&mockEntityChangeRequestClient{})
		r := httptest.NewRequest(http.MethodPost, "/change-requests", strings.NewReader(`{"subject":"Deploy v2"}`))
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
		const reqPayload = `{"subject":"Deploy v2"}`
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
		for _, tc := range upstreamErrors("Failed to create change request.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityChangeRequestClient{
					createChangeRequestFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewChangeRequestHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/change-requests", strings.NewReader(`{"subject":"Deploy v2"}`)))
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
		for _, tc := range upstreamErrors("Failed to retrieve change request.") {
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
		for _, tc := range upstreamErrors("Failed to retrieve change request approvals.") {
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
		for _, tc := range upstreamErrors("Failed to submit change request approval decision.") {
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
		for _, tc := range upstreamErrors("Failed to search change requests.") {
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
