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
	"testing"
)

func TestGetConversationMessages(t *testing.T) {
	const validID = "11111111-1111-1111-1111-111111111111"

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewConversationHandler(&mockEntityConversationClient{})
		r := httptest.NewRequest(http.MethodGet, "/conversations/"+validID+"/messages", nil)
		r.SetPathValue("id", validID)
		w := httptest.NewRecorder()
		h.GetConversationMessages(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects empty conversation ID", func(t *testing.T) {
		h := NewConversationHandler(&mockEntityConversationClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/conversations//messages", nil))
		r.SetPathValue("id", "")
		w := httptest.NewRecorder()
		h.GetConversationMessages(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
	})

	t.Run("rejects non-UUID conversation ID", func(t *testing.T) {
		h := NewConversationHandler(&mockEntityConversationClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/conversations/not-a-uuid/messages", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetConversationMessages(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
	})

	t.Run("rejects invalid limit", func(t *testing.T) {
		h := NewConversationHandler(&mockEntityConversationClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/conversations/"+validID+"/messages?limit=abc", nil))
		r.SetPathValue("id", validID)
		w := httptest.NewRecorder()
		h.GetConversationMessages(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("rejects negative offset", func(t *testing.T) {
		h := NewConversationHandler(&mockEntityConversationClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/conversations/"+validID+"/messages?offset=-1", nil))
		r.SetPathValue("id", validID)
		w := httptest.NewRecorder()
		h.GetConversationMessages(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("sends referenceType=conversation payload with correct ID and pagination", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityConversationClient{
			searchCommentsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"comments":[],"total":0,"limit":10,"offset":5,"hasMore":false}`), nil
			},
		}
		h := NewConversationHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/conversations/"+validID+"/messages?limit=10&offset=5", nil))
		r.SetPathValue("id", validID)
		w := httptest.NewRecorder()
		h.GetConversationMessages(w, r)
		assertStatus(t, w, http.StatusOK)

		var payload map[string]any
		if err := json.Unmarshal(capturedBody, &payload); err != nil {
			t.Fatalf("captured body is not valid JSON: %v", err)
		}
		if payload["referenceId"] != validID {
			t.Errorf("referenceId = %v, want %q", payload["referenceId"], validID)
		}
		if payload["referenceType"] != "conversation" {
			t.Errorf("referenceType = %v, want %q", payload["referenceType"], "conversation")
		}
		pag, _ := payload["pagination"].(map[string]any)
		if pag["limit"] != float64(10) {
			t.Errorf("pagination.limit = %v, want 10", pag["limit"])
		}
		if pag["offset"] != float64(5) {
			t.Errorf("pagination.offset = %v, want 5", pag["offset"])
		}
	})

	t.Run("returns 200 with messages payload", func(t *testing.T) {
		const payload = `{"comments":[{"id":"22222222-2222-2222-2222-222222222222","content":"Hello","type":"comment"}],"total":1,"limit":20,"offset":0,"hasMore":false}`
		client := &mockEntityConversationClient{
			searchCommentsFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return []byte(payload), nil
			},
		}
		h := NewConversationHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/conversations/"+validID+"/messages", nil))
		r.SetPathValue("id", validID)
		w := httptest.NewRecorder()
		h.GetConversationMessages(w, r)
		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
	})

	t.Run("maps upstream errors", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to retrieve conversation messages.") {
			t.Run(tc.name, func(t *testing.T) {
				client := &mockEntityConversationClient{
					searchCommentsFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewConversationHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/conversations/"+validID+"/messages", nil))
				r.SetPathValue("id", validID)
				w := httptest.NewRecorder()
				h.GetConversationMessages(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}
