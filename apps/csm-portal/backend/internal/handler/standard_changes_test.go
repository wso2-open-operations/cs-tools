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

type fakeStandardChangeClient struct {
	method, path string
	body         []byte
	calls        int
	result       []byte
	err          error
}

func (f *fakeStandardChangeClient) StandardChangeCall(_ context.Context, method, path string, body []byte) ([]byte, error) {
	f.calls++
	f.method, f.path, f.body = method, path, body
	if f.result == nil {
		f.result = []byte(`{"ok":true}`)
	}
	return f.result, f.err
}

const stdID = "11111111-2222-3333-4444-555555555555"
const stdVersionID = "66666666-7777-8888-9999-000000000000"

func TestStandardChangeHandler_ForwardsEachRoute(t *testing.T) {
	cases := []struct {
		name       string
		method     string
		path       string
		body       string
		pathValues map[string]string
		call       func(h *StandardChangeHandler) http.HandlerFunc
		wantMethod string
		wantPath   string
		wantStatus int
	}{
		{"search templates", http.MethodPost, "/x", `{"query":"rotate"}`, nil,
			func(h *StandardChangeHandler) http.HandlerFunc { return h.SearchTemplates },
			http.MethodPost, "/standard-change-templates/search", http.StatusOK},
		{"get template", http.MethodGet, "/x", "", map[string]string{"id": stdID},
			func(h *StandardChangeHandler) http.HandlerFunc { return h.GetTemplate },
			http.MethodGet, "/standard-change-templates/" + stdID, http.StatusOK},
		{"get version", http.MethodGet, "/x", "", map[string]string{"id": stdID, "versionId": stdVersionID},
			func(h *StandardChangeHandler) http.HandlerFunc { return h.GetTemplateVersion },
			http.MethodGet, "/standard-change-templates/" + stdID + "/versions/" + stdVersionID, http.StatusOK},
		{"suspend", http.MethodPost, "/x", "", map[string]string{"id": stdID},
			func(h *StandardChangeHandler) http.HandlerFunc { return h.SuspendTemplate },
			http.MethodPost, "/standard-change-templates/" + stdID + "/suspend", http.StatusOK},
		{"resume", http.MethodPost, "/x", "", map[string]string{"id": stdID},
			func(h *StandardChangeHandler) http.HandlerFunc { return h.ResumeTemplate },
			http.MethodPost, "/standard-change-templates/" + stdID + "/resume", http.StatusOK},
		{"clear review", http.MethodPost, "/x", "", map[string]string{"id": stdID},
			func(h *StandardChangeHandler) http.HandlerFunc { return h.ClearTemplateReview },
			http.MethodPost, "/standard-change-templates/" + stdID + "/clear-review", http.StatusOK},
		{"create proposal", http.MethodPost, "/x", `{"kind":"new"}`, nil,
			func(h *StandardChangeHandler) http.HandlerFunc { return h.CreateProposal },
			http.MethodPost, "/standard-change-proposals", http.StatusCreated},
		{"search proposals", http.MethodPost, "/x", `{}`, nil,
			func(h *StandardChangeHandler) http.HandlerFunc { return h.SearchProposals },
			http.MethodPost, "/standard-change-proposals/search", http.StatusOK},
		{"get proposal", http.MethodGet, "/x", "", map[string]string{"id": stdID},
			func(h *StandardChangeHandler) http.HandlerFunc { return h.GetProposal },
			http.MethodGet, "/standard-change-proposals/" + stdID, http.StatusOK},
		{"update proposal", http.MethodPatch, "/x", `{"name":"n"}`, map[string]string{"id": stdID},
			func(h *StandardChangeHandler) http.HandlerFunc { return h.UpdateProposal },
			http.MethodPatch, "/standard-change-proposals/" + stdID, http.StatusOK},
		{"submit", http.MethodPost, "/x", "", map[string]string{"id": stdID},
			func(h *StandardChangeHandler) http.HandlerFunc { return h.SubmitProposal },
			http.MethodPost, "/standard-change-proposals/" + stdID + "/submit", http.StatusOK},
		{"withdraw", http.MethodPost, "/x", "", map[string]string{"id": stdID},
			func(h *StandardChangeHandler) http.HandlerFunc { return h.WithdrawProposal },
			http.MethodPost, "/standard-change-proposals/" + stdID + "/withdraw", http.StatusOK},
		{"decide", http.MethodPost, "/x", `{"decision":"approved"}`, map[string]string{"id": stdID},
			func(h *StandardChangeHandler) http.HandlerFunc { return h.DecideProposal },
			http.MethodPost, "/standard-change-proposals/" + stdID + "/decision", http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &fakeStandardChangeClient{}
			var body *strings.Reader
			if c.body != "" {
				body = strings.NewReader(c.body)
			} else {
				body = strings.NewReader("")
			}
			r := withUser(httptest.NewRequest(c.method, c.path, body))
			for k, v := range c.pathValues {
				r.SetPathValue(k, v)
			}
			w := httptest.NewRecorder()
			c.call(NewStandardChangeHandler(fake))(w, r)
			if w.Code != c.wantStatus || fake.method != c.wantMethod || fake.path != c.wantPath {
				t.Fatalf("status %d, call %s %s; want %d, %s %s", w.Code, fake.method, fake.path, c.wantStatus, c.wantMethod, c.wantPath)
			}
			if c.body != "" && string(fake.body) != c.body {
				t.Fatalf("body forwarded = %s, want %s", fake.body, c.body)
			}
			if c.body == "" && fake.body != nil {
				t.Fatalf("a call with no body forwarded %s", fake.body)
			}
		})
	}
}

func TestStandardChangeHandler_RefusesBadIdsAndBodiesBeforeCalling(t *testing.T) {
	fake := &fakeStandardChangeClient{}
	h := NewStandardChangeHandler(fake)

	r := withUser(httptest.NewRequest(http.MethodPost, "/x", nil))
	r.SetPathValue("id", "../../users")
	w := httptest.NewRecorder()
	h.SubmitProposal(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("a path id that is not a UUID: %d", w.Code)
	}

	r = withUser(httptest.NewRequest(http.MethodGet, "/x", nil))
	r.SetPathValue("id", stdID)
	r.SetPathValue("versionId", "nope")
	w = httptest.NewRecorder()
	h.GetTemplateVersion(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("a version id that is not a UUID: %d", w.Code)
	}

	r = withUser(httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{not json`)))
	w = httptest.NewRecorder()
	h.CreateProposal(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("a body that is not JSON: %d", w.Code)
	}

	w = httptest.NewRecorder()
	h.SearchTemplates(w, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{}`)))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no user: %d", w.Code)
	}
	if fake.calls != 0 {
		t.Fatalf("entity-service was called %d times", fake.calls)
	}
}

func TestStandardChangeHandler_SuspendForwardsTheOptionalReason(t *testing.T) {
	suspend := func(body string) (*httptest.ResponseRecorder, *fakeStandardChangeClient) {
		fake := &fakeStandardChangeClient{}
		r := withUser(httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body)))
		r.SetPathValue("id", stdID)
		w := httptest.NewRecorder()
		NewStandardChangeHandler(fake).SuspendTemplate(w, r)
		return w, fake
	}

	w, fake := suspend(`{"reason":"Rolled back twice"}`)
	if w.Code != http.StatusOK || fake.path != "/standard-change-templates/"+stdID+"/suspend" || string(fake.body) != `{"reason":"Rolled back twice"}` {
		t.Fatalf("with a reason: %d, %s %s", w.Code, fake.path, fake.body)
	}
	for _, empty := range []string{"", "  \n"} {
		w, fake = suspend(empty)
		if w.Code != http.StatusOK || fake.calls != 1 || fake.body != nil {
			t.Fatalf("empty body %q: %d, %d calls, forwarded %q", empty, w.Code, fake.calls, fake.body)
		}
	}
	for _, bad := range []string{`{"reason":`, `"just a string"`, `[1]`, `{} trailing`} {
		w, fake = suspend(bad)
		if w.Code != http.StatusBadRequest || fake.calls != 0 {
			t.Fatalf("body %q: %d, %d calls", bad, w.Code, fake.calls)
		}
	}
	w, fake = suspend(`{"reason":"` + strings.Repeat("x", maxRequestBodyBytes) + `"}`)
	if w.Code != http.StatusRequestEntityTooLarge || fake.calls != 0 {
		t.Fatalf("an oversized body: %d, %d calls", w.Code, fake.calls)
	}

	// entity-service's refusal of a too-long reason keeps its words.
	fake = &fakeStandardChangeClient{err: &apierror.Error{StatusCode: http.StatusBadRequest,
		Body: `{"code":0,"message":"reason must be at most 1000 characters"}`}}
	r := withUser(httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"reason":"x"}`)))
	r.SetPathValue("id", stdID)
	w = httptest.NewRecorder()
	NewStandardChangeHandler(fake).SuspendTemplate(w, r)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "at most 1000 characters") {
		t.Fatalf("refusal: %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{}`))
	r.SetPathValue("id", stdID)
	NewStandardChangeHandler(fake).SuspendTemplate(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no user: %d", w.Code)
	}
}

func TestStandardChangeHandler_KeepsTheEntityReasonAndCode(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound, http.StatusConflict} {
		fake := &fakeStandardChangeClient{err: &apierror.Error{StatusCode: status,
			Body: `{"code":0,"message":"only the proposer can edit this proposal","errorCode":"standard_change_template_unavailable"}`}}
		r := withUser(httptest.NewRequest(http.MethodPatch, "/x", strings.NewReader(`{"name":"n"}`)))
		r.SetPathValue("id", stdID)
		w := httptest.NewRecorder()
		NewStandardChangeHandler(fake).UpdateProposal(w, r)
		if w.Code != status || !strings.Contains(w.Body.String(), "only the proposer") || !strings.Contains(w.Body.String(), "standard_change_template_unavailable") {
			t.Fatalf("%d: got %d %s", status, w.Code, w.Body.String())
		}
	}
	fake := &fakeStandardChangeClient{err: &apierror.Error{StatusCode: http.StatusInternalServerError, Body: `{"message":"pq: secret detail"}`}}
	w := httptest.NewRecorder()
	NewStandardChangeHandler(fake).SearchProposals(w, withUser(httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{}`))))
	if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("a 500 leaked: %d %s", w.Code, w.Body.String())
	}
}
