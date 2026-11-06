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

const storedAttID = "11111111-1111-1111-1111-111111111111"

type fakeStoredEntity struct {
	upload []byte
	err    error
}

func (f *fakeStoredEntity) InitiateStoredAttachmentUpload(_ context.Context, b []byte) ([]byte, error) {
	f.upload = b
	return []byte(`{"attachmentId":"a","maxBytes":104857600}`), f.err
}

func serveStoredAtt(h http.HandlerFunc, method, target, body string, user bool) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /attachments/upload", h)
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	if user {
		r = withUser(r)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, r)
	return rec
}

func TestStoredAttachments_ForwardBodiesAndStatuses(t *testing.T) {
	f := &fakeStoredEntity{}
	h := NewStoredAttachmentHandler(f)

	rec := serveStoredAtt(h.InitiateUpload, http.MethodPost, "/attachments/upload", `{"type":"work_item","parentId":"p","name":"a.pdf","sizeBytes":5}`, true)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"attachmentId"`) || len(f.upload) == 0 || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("upload: %d %s cache=%q", rec.Code, rec.Body.String(), rec.Header().Get("Cache-Control"))
	}
}

func TestStoredAttachments_RequireAUserAndAValidRequest(t *testing.T) {
	f := &fakeStoredEntity{}
	h := NewStoredAttachmentHandler(f)
	for name, tc := range map[string]struct {
		body string
		user bool
		want int
	}{
		"no user":      {`{}`, false, http.StatusUnauthorized},
		"empty body":   {``, true, http.StatusBadRequest},
		"body too big": {strings.Repeat("a", maxRequestBodyBytes+10), true, http.StatusRequestEntityTooLarge},
	} {
		if rec := serveStoredAtt(h.InitiateUpload, "POST", "/attachments/upload", tc.body, tc.user); rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", name, rec.Code, tc.want)
		}
	}
	if f.upload != nil {
		t.Error("the entity service was called for a request that should have been refused here")
	}
}

func TestStoredAttachments_MapEntityErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		status  int
		body    string
		want    int
		message string
	}{
		"closed item":  {409, `{"message":"This case is closed, so attachments can't be added."}`, 409, "This case is closed"},
		"not uploader": {403, `{"message":"Only the person who uploaded an attachment can delete it."}`, 403, "Only the person who uploaded"},
		"rate limit":   {429, `{"message":"too many attachment requests; try again in a minute"}`, 429, "too many attachment"},
		"bad input":    {400, `{"message":"the file is larger than the 100 MiB limit"}`, 400, "larger than"},
		"not found":    {404, `{"message":"attachment not found"}`, 404, ErrMsgNotFound},
		"removed":      {410, `{"message":"secret upstream wording","errorCode":"attachment_removed"}`, 410, ErrMsgAttachmentRemoved},
		"storage down": {503, `{"message":"file storage is unavailable"}`, 503, "Failed to start"},
		"internal":     {500, `{"message":"boom: secret detail"}`, 500, "Failed to start"},
		"raw body":     {409, `not json secret`, 409, "Failed to start"},
	} {
		f := &fakeStoredEntity{err: &apierror.Error{StatusCode: tc.status, Body: tc.body}}
		h := NewStoredAttachmentHandler(f)
		rec := serveStoredAtt(h.InitiateUpload, "POST", "/attachments/upload", `{"type":"work_item","parentId":"x"}`, true)
		if rec.Code != tc.want || !strings.Contains(rec.Body.String(), tc.message) || strings.Contains(rec.Body.String(), "secret") {
			t.Errorf("%s: status = %d body = %s, want %d containing %q and no upstream detail", name, rec.Code, rec.Body.String(), tc.want, tc.message)
		}
	}
}

func TestStoredAttachments_CommentImageTarget(t *testing.T) {
	const pid = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	for name, tc := range map[string]struct {
		body string
		want int
	}{
		"parentId only":       {`{"type":"comment","parentId":"` + pid + `","name":"a.png","sizeBytes":1}`, http.StatusCreated},
		"projectId only":      {`{"type":"comment","projectId":"` + pid + `","name":"a.png","sizeBytes":1}`, http.StatusCreated},
		"both":                {`{"type":"comment","parentId":"` + pid + `","projectId":"` + pid + `"}`, http.StatusBadRequest},
		"neither":             {`{"type":"comment","name":"a.png","sizeBytes":1}`, http.StatusBadRequest},
		"empty projectId":     {`{"type":"comment","projectId":""}`, http.StatusBadRequest},
		"bad projectId":       {`{"type":"comment","projectId":"nope"}`, http.StatusBadRequest},
		"null ids":            {`{"type":"comment","parentId":null,"projectId":null}`, http.StatusBadRequest},
		"work_item untouched": {`{"type":"work_item","parentId":"p","name":"a","sizeBytes":1}`, http.StatusCreated},
	} {
		f := &fakeStoredEntity{}
		h := NewStoredAttachmentHandler(f)
		rec := serveStoredAtt(h.InitiateUpload, http.MethodPost, "/attachments/upload", tc.body, true)
		if rec.Code != tc.want {
			t.Errorf("%s: status %d, want %d (%s)", name, rec.Code, tc.want, rec.Body.String())
		}
		if tc.want == http.StatusCreated && string(f.upload) != tc.body {
			t.Errorf("%s: body not forwarded untouched: %q", name, f.upload)
		}
		if tc.want != http.StatusCreated && f.upload != nil {
			t.Errorf("%s: entity was called", name)
		}
	}
}
