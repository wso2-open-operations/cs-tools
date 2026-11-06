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

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

type fakeStoredEntity struct {
	gotBody string
	gotID   string
	out     []byte
	err     error
}

func (f *fakeStoredEntity) InitiateStoredAttachmentUpload(_ context.Context, b json.RawMessage) ([]byte, error) {
	f.gotBody = string(b)
	return f.out, f.err
}
func (f *fakeStoredEntity) ConfirmStoredAttachmentUpload(_ context.Context, id string, b json.RawMessage) ([]byte, error) {
	f.gotID, f.gotBody = id, string(b)
	return f.out, f.err
}
func (f *fakeStoredEntity) DeleteStoredAttachment(_ context.Context, b json.RawMessage) error {
	f.gotBody = string(b)
	return f.err
}

type permitCall struct {
	module middleware.Module
	action middleware.Action
}

func serveStored(t *testing.T, f *fakeStoredEntity, allow bool, method, target, body string) (*httptest.ResponseRecorder, []permitCall) {
	t.Helper()
	var calls []permitCall
	h := NewStoredAttachmentHandler(f, func(w http.ResponseWriter, _ *http.Request, m middleware.Module, a middleware.Action) bool {
		calls = append(calls, permitCall{m, a})
		if !allow {
			writeError(w, http.StatusForbidden, ErrMsgForbidden)
		}
		return allow
	})
	mux := http.NewServeMux()
	mux.HandleFunc("POST /attachments/upload", h.InitiateUpload)
	mux.HandleFunc("POST /attachments/upload/{id}/confirm", h.ConfirmUpload)
	mux.HandleFunc("POST /attachments/delete", h.DeleteAttachment)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req = req.WithContext(middleware.WithUserInfo(req.Context(), &middleware.UserInfo{UserID: "u1"}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec, calls
}

const storedID = "11111111-2222-3333-4444-555555555555"

func TestStoredAttachments_RelayBodiesAndStatuses(t *testing.T) {
	f := &fakeStoredEntity{out: []byte(`{"ok":true}`)}
	rec, calls := serveStored(t, f, true, http.MethodPost, "/attachments/upload", `{"type":"deployment","parentId":"p","name":"a","sizeBytes":1}`)
	if rec.Code != http.StatusCreated || calls[0] != (permitCall{middleware.ModuleDeployments, middleware.ActionCreate}) {
		t.Errorf("upload: %d %v", rec.Code, calls)
	}

	rec, calls = serveStored(t, f, true, http.MethodPost, "/attachments/upload/"+storedID+"/confirm", `{"type":"comment"}`)
	if rec.Code != http.StatusOK || f.gotID != storedID || calls[0] != (permitCall{middleware.ModuleCases, middleware.ActionCreate}) {
		t.Errorf("confirm: %d id=%q %v", rec.Code, f.gotID, calls)
	}

	rec, calls = serveStored(t, f, true, http.MethodPost, "/attachments/delete", `{"type":"work_item","id":"x"}`)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 || calls[0].action != middleware.ActionCreate {
		t.Errorf("delete: %d %q %v", rec.Code, rec.Body.String(), calls)
	}
}

func TestStoredAttachments_RejectsBeforeCallingEntity(t *testing.T) {
	f := &fakeStoredEntity{}
	for name, tc := range map[string]struct {
		target, body string
		allow        bool
		want         int
	}{
		"unknown type":     {"/attachments/upload", `{"type":"case"}`, true, http.StatusBadRequest},
		"no type":          {"/attachments/upload", `{"sizeBytes":1}`, true, http.StatusBadRequest},
		"not json":         {"/attachments/delete", `nope`, true, http.StatusBadRequest},
		"no permission":    {"/attachments/upload", `{"type":"work_item"}`, false, http.StatusForbidden},
		"confirm bad uuid": {"/attachments/upload/not-a-uuid/confirm", `{"type":"work_item"}`, true, http.StatusBadRequest},
	} {
		rec, _ := serveStored(t, f, tc.allow, http.MethodPost, tc.target, tc.body)
		if rec.Code != tc.want || f.gotBody != "" {
			t.Errorf("%s: status %d (want %d), entity called with %q", name, rec.Code, tc.want, f.gotBody)
		}
	}
}

func TestStoredAttachments_UpstreamErrorsKeepTheirStatus(t *testing.T) {
	for status, want := range map[int]int{
		http.StatusBadRequest: 400, http.StatusForbidden: 403, http.StatusNotFound: 404,
		http.StatusConflict: 409, http.StatusTooManyRequests: 429, http.StatusServiceUnavailable: 503,
		http.StatusUnauthorized: 401,
	} {
		f := &fakeStoredEntity{err: apierror.NewUpstreamError(status, []byte(`{"code":"x","message":"upstream said so"}`))}
		rec, _ := serveStored(t, f, true, http.MethodPost, "/attachments/upload", `{"type":"work_item","parentId":"p","name":"a","sizeBytes":1}`)
		if rec.Code != want {
			t.Errorf("upstream %d -> %d, want %d", status, rec.Code, want)
		}
	}
}

func TestModuleForStoredAttachmentType(t *testing.T) {
	tests := map[string]struct {
		want middleware.Module
		ok   bool
	}{
		"work_item":  {middleware.ModuleCases, true},
		"comment":    {middleware.ModuleCases, true},
		"deployment": {middleware.ModuleDeployments, true},
		"case":       {"", false},
		"":           {"", false},
	}
	for in, tc := range tests {
		got, ok := moduleForStoredAttachmentType(in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("moduleForStoredAttachmentType(%q) = %q, %v; want %q, %v", in, got, ok, tc.want, tc.ok)
		}
	}
}
