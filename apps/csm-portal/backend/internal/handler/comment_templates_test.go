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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

const testCommentTemplateID = "5e636a12-0000-4000-8000-000000000001"

// fakeCommentTemplateClient records which entity call ran and with what.
type fakeCommentTemplateClient struct {
	calls  []string
	lastID string
	body   string
	resp   []byte
	err    error
}

func (f *fakeCommentTemplateClient) record(op, id string, body []byte) ([]byte, error) {
	f.calls = append(f.calls, op)
	f.lastID, f.body = id, string(body)
	return f.resp, f.err
}

func (f *fakeCommentTemplateClient) ListCaseCommentTemplates(context.Context) ([]byte, error) {
	return f.record("list", "", nil)
}
func (f *fakeCommentTemplateClient) CreatePersonalCaseCommentTemplate(_ context.Context, b []byte) ([]byte, error) {
	return f.record("create-personal", "", b)
}
func (f *fakeCommentTemplateClient) UpdatePersonalCaseCommentTemplate(_ context.Context, id string, b []byte) ([]byte, error) {
	return f.record("update-personal", id, b)
}
func (f *fakeCommentTemplateClient) DeletePersonalCaseCommentTemplate(_ context.Context, id string) error {
	_, err := f.record("delete-personal", id, nil)
	return err
}
func (f *fakeCommentTemplateClient) CreateSharedCaseCommentTemplate(_ context.Context, b []byte) ([]byte, error) {
	return f.record("create-shared", "", b)
}
func (f *fakeCommentTemplateClient) UpdateSharedCaseCommentTemplate(_ context.Context, id string, b []byte) ([]byte, error) {
	return f.record("update-shared", id, b)
}
func (f *fakeCommentTemplateClient) DeleteSharedCaseCommentTemplate(_ context.Context, id string) error {
	_, err := f.record("delete-shared", id, nil)
	return err
}

func TestCommentTemplateHandler_RoutesEachCallToItsOwnEntityCall(t *testing.T) {
	body := `{"name":"Ack","body":"<p>Hi</p>","appliesTo":"COMMENT"}`
	tests := []struct {
		name       string
		method     string
		serve      func(*CommentTemplateHandler) http.HandlerFunc
		id, body   string
		wantStatus int
		wantCall   string
	}{
		{"list", http.MethodGet, func(h *CommentTemplateHandler) http.HandlerFunc { return h.ListCommentTemplates }, "", "", http.StatusOK, "list"},
		{"create personal", http.MethodPost, func(h *CommentTemplateHandler) http.HandlerFunc { return h.CreatePersonalCommentTemplate }, "", body, http.StatusCreated, "create-personal"},
		{"update personal", http.MethodPatch, func(h *CommentTemplateHandler) http.HandlerFunc { return h.UpdatePersonalCommentTemplate }, testCommentTemplateID, body, http.StatusOK, "update-personal"},
		{"delete personal", http.MethodDelete, func(h *CommentTemplateHandler) http.HandlerFunc { return h.DeletePersonalCommentTemplate }, testCommentTemplateID, "", http.StatusNoContent, "delete-personal"},
		{"create shared", http.MethodPost, func(h *CommentTemplateHandler) http.HandlerFunc { return h.CreateSharedCommentTemplate }, "", body, http.StatusCreated, "create-shared"},
		{"update shared", http.MethodPatch, func(h *CommentTemplateHandler) http.HandlerFunc { return h.UpdateSharedCommentTemplate }, testCommentTemplateID, body, http.StatusOK, "update-shared"},
		{"delete shared", http.MethodDelete, func(h *CommentTemplateHandler) http.HandlerFunc { return h.DeleteSharedCommentTemplate }, testCommentTemplateID, "", http.StatusNoContent, "delete-shared"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeCommentTemplateClient{resp: []byte(`{"id":"x"}`)}
			h := NewCommentTemplateHandler(fake)
			r := httptest.NewRequest(tt.method, "/", strings.NewReader(tt.body))
			if tt.id != "" {
				r.SetPathValue("id", strings.ToUpper(tt.id))
			}
			w := httptest.NewRecorder()
			tt.serve(h)(w, withUser(r))

			assertStatus(t, w, tt.wantStatus)
			if len(fake.calls) != 1 || fake.calls[0] != tt.wantCall {
				t.Fatalf("entity calls = %v, want [%s]", fake.calls, tt.wantCall)
			}
			if tt.id != "" && fake.lastID != tt.id {
				t.Errorf("forwarded id = %q, want the lower-cased %q", fake.lastID, tt.id)
			}
			if tt.body != "" && fake.body != tt.body {
				t.Errorf("forwarded body = %q, want it unchanged", fake.body)
			}
		})
	}
}

func TestCommentTemplateHandler_RejectsBeforeCallingEntity(t *testing.T) {
	t.Run("no user", func(t *testing.T) {
		fake := &fakeCommentTemplateClient{}
		w := httptest.NewRecorder()
		NewCommentTemplateHandler(fake).ListCommentTemplates(w, httptest.NewRequest(http.MethodGet, "/", nil))
		assertStatus(t, w, http.StatusUnauthorized)
		if len(fake.calls) != 0 {
			t.Errorf("entity called %v without a user", fake.calls)
		}
	})
	t.Run("bad id", func(t *testing.T) {
		fake := &fakeCommentTemplateClient{}
		r := httptest.NewRequest(http.MethodDelete, "/", nil)
		r.SetPathValue("id", "../../users/me")
		w := httptest.NewRecorder()
		NewCommentTemplateHandler(fake).DeletePersonalCommentTemplate(w, withUser(r))
		assertStatus(t, w, http.StatusBadRequest)
		if len(fake.calls) != 0 {
			t.Errorf("entity called %v with a bad id", fake.calls)
		}
	})
	t.Run("invalid JSON", func(t *testing.T) {
		fake := &fakeCommentTemplateClient{}
		w := httptest.NewRecorder()
		NewCommentTemplateHandler(fake).CreatePersonalCommentTemplate(w, withUser(httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{"))))
		assertStatus(t, w, http.StatusBadRequest)
		if len(fake.calls) != 0 {
			t.Errorf("entity called %v with invalid JSON", fake.calls)
		}
	})
}

func TestCommentTemplateHandler_MapsUpstreamErrors(t *testing.T) {
	t.Run("a name clash shows entity-service's message", func(t *testing.T) {
		fake := &fakeCommentTemplateClient{err: &apierror.Error{StatusCode: http.StatusConflict, Body: `{"message":"a template named Ack already exists"}`}}
		w := httptest.NewRecorder()
		NewCommentTemplateHandler(fake).CreatePersonalCommentTemplate(w, withUser(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Ack"}`))))
		assertStatus(t, w, http.StatusConflict)
		assertErrorMessage(t, w, "a template named Ack already exists")
	})
	t.Run("another user's template is a plain 404", func(t *testing.T) {
		fake := &fakeCommentTemplateClient{err: &apierror.Error{StatusCode: http.StatusNotFound, Body: `{"message":"comment template not found"}`}}
		r := httptest.NewRequest(http.MethodDelete, "/", nil)
		r.SetPathValue("id", testCommentTemplateID)
		w := httptest.NewRecorder()
		NewCommentTemplateHandler(fake).DeletePersonalCommentTemplate(w, withUser(r))
		assertStatus(t, w, http.StatusNotFound)
	})
}
