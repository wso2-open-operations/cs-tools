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

const testCommentID = "11111111-1111-1111-1111-111111111111"

func TestUpdateComment(t *testing.T) {
	t.Run("rejects unauthenticated requests", func(t *testing.T) {
		h := NewCommentHandler(&mockEntityCommentClient{})
		r := httptest.NewRequest(http.MethodPatch, "/comments/"+testCommentID, strings.NewReader(`{"content":"edited"}`))
		r.SetPathValue("id", testCommentID)
		w := httptest.NewRecorder()
		h.UpdateComment(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewCommentHandler(&mockEntityCommentClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/comments/not-a-uuid", strings.NewReader(`{"content":"edited"}`)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.UpdateComment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed JSON body", func(t *testing.T) {
		h := NewCommentHandler(&mockEntityCommentClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/comments/"+testCommentID, strings.NewReader(`{bad`)))
		r.SetPathValue("id", testCommentID)
		w := httptest.NewRecorder()
		h.UpdateComment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards id and body, returns 200 with the updated comment", func(t *testing.T) {
		var capturedID string
		var capturedBody []byte
		client := &mockEntityCommentClient{
			updateCommentFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedID, capturedBody = id, body
				return []byte(`{"id":"` + id + `","content":"edited"}`), nil
			},
		}
		h := NewCommentHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/comments/"+testCommentID, strings.NewReader(`{"content":"edited"}`)))
		r.SetPathValue("id", testCommentID)
		w := httptest.NewRecorder()
		h.UpdateComment(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedID != testCommentID {
			t.Errorf("upstream received id %q, want %q", capturedID, testCommentID)
		}
		if string(capturedBody) != `{"content":"edited"}` {
			t.Errorf("upstream received body %q", capturedBody)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["content"] != "edited" {
			t.Errorf("content = %v, want %q", resp["content"], "edited")
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to update comment.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCommentClient{
					updateCommentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCommentHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPatch, "/comments/"+testCommentID, strings.NewReader(`{"content":"edited"}`)))
				r.SetPathValue("id", testCommentID)
				w := httptest.NewRecorder()
				h.UpdateComment(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}

func TestDeleteComment(t *testing.T) {
	t.Run("rejects unauthenticated requests", func(t *testing.T) {
		h := NewCommentHandler(&mockEntityCommentClient{})
		r := httptest.NewRequest(http.MethodDelete, "/comments/"+testCommentID, nil)
		r.SetPathValue("id", testCommentID)
		w := httptest.NewRecorder()
		h.DeleteComment(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewCommentHandler(&mockEntityCommentClient{})
		r := withUser(httptest.NewRequest(http.MethodDelete, "/comments/not-a-uuid", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.DeleteComment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards id to upstream, returns 204 with no body", func(t *testing.T) {
		var capturedID string
		client := &mockEntityCommentClient{
			deleteCommentFn: func(_ context.Context, id string) ([]byte, error) {
				capturedID = id
				return nil, nil
			},
		}
		h := NewCommentHandler(client)
		r := withUser(httptest.NewRequest(http.MethodDelete, "/comments/"+testCommentID, nil))
		r.SetPathValue("id", testCommentID)
		w := httptest.NewRecorder()
		h.DeleteComment(w, r)

		assertStatus(t, w, http.StatusNoContent)
		if capturedID != testCommentID {
			t.Errorf("upstream received id %q, want %q", capturedID, testCommentID)
		}
		if w.Body.Len() != 0 {
			t.Errorf("body = %q, want empty", w.Body.String())
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to delete comment.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCommentClient{
					deleteCommentFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCommentHandler(client)
				r := withUser(httptest.NewRequest(http.MethodDelete, "/comments/"+testCommentID, nil))
				r.SetPathValue("id", testCommentID)
				w := httptest.NewRecorder()
				h.DeleteComment(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}
