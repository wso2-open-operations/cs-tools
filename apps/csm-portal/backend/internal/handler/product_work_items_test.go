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
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

const (
	pwiCaseID = "11111111-1111-1111-1111-111111111111"
	pwiItemID = "22222222-2222-2222-2222-222222222222"
)

func TestCreateCaseGithubIssue_GitInternalsForward(t *testing.T) {
	var gotCase string
	var gotBody []byte
	calls := 0
	mock := &mockEntityCaseClient{
		createCaseProductWorkItemFn: func(_ context.Context, caseID string, body []byte) ([]byte, error) {
			calls++
			gotCase, gotBody = caseID, body
			return []byte(`{"message":"GitHub issue created.","issue":{"url":"https://github.com/o/r/issues/7","number":7,"repo":"o/r"},"kind":"PRODUCT_ISSUE","workItem":{"id":"x","number":"PI0000001"}}`), nil
		},
		createCaseGithubIssueFn: func(context.Context, string, []byte) ([]byte, error) {
			t.Error("the entity service's github-issues route must not be called")
			return nil, nil
		},
	}
	post := func(h *CaseHandler, body string, hdr map[string]string) *httptest.ResponseRecorder {
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+pwiCaseID+"/github-issues", strings.NewReader(body)))
		r.SetPathValue("id", pwiCaseID)
		for k, v := range hdr {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		h.CreateCaseGithubIssue(w, r)
		return w
	}
	on := NewCaseHandler(mock).WithGitInternals(true).WithEngineeringClient(&mockEngineeringClient{})

	t.Run("rebuilds the body and uses the header key", func(t *testing.T) {
		calls = 0
		w := post(on, `{"title":"  Crash  ","description":"It fails.","reason":"migration","updateLevel":"U1","publicIssueUrl":"https://x/y","regression":true,"hotFixRequired":true,"issueTypeLabel":"Type/Patch","priorityLevel":"Priority/High","onboardingInProgress":true,"repoOverride":{"owner":"a","repo":"b"},"unknownField":1}`,
			map[string]string{"Idempotency-Key": "my-key-1"})
		assertStatus(t, w, http.StatusCreated)
		if calls != 1 || gotCase != pwiCaseID {
			t.Fatalf("calls=%d case=%q", calls, gotCase)
		}
		var got map[string]any
		if err := json.Unmarshal(gotBody, &got); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"idempotencyKey": "my-key-1", "title": "Crash", "description": "It fails.", "reason": "migration", "updateLevel": "U1", "publicIssueUrl": "https://x/y", "regression": true, "hotFixRequired": true, "issueTypeLabel": "Type/Patch", "priorityLevel": "Priority/High", "onboardingInProgress": true}
		if len(got) != len(want) {
			t.Errorf("forwarded body = %s", gotBody)
		}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("%s = %v, want %v", k, got[k], v)
			}
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["kind"] != "PRODUCT_ISSUE" || resp["workItem"] == nil || resp["issue"] == nil {
			t.Errorf("response = %v", resp)
		}
	})

	t.Run("derives a stable key from user, case and body when no header", func(t *testing.T) {
		keyOf := func(h *CaseHandler, caseID, body string) string {
			t.Helper()
			r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/github-issues", strings.NewReader(body)))
			r.SetPathValue("id", caseID)
			w := httptest.NewRecorder()
			h.CreateCaseGithubIssue(w, r)
			assertStatus(t, w, http.StatusCreated)
			var got struct {
				Key string `json:"idempotencyKey"`
			}
			_ = json.Unmarshal(gotBody, &got)
			return got.Key
		}
		const base = `{"title":"t","description":"d","reason":"r","updateLevel":"u","publicIssueUrl":"p","regression":false,"hotFixRequired":false,"issueTypeLabel":"Type/Discussion","priorityLevel":"Priority/High","onboardingInProgress":false}`
		k1 := keyOf(on, pwiCaseID, base)
		if !strings.HasPrefix(k1, "bff-") || len(k1) > 200 || !validIdempotencyKey(k1) {
			t.Fatalf("key = %q", k1)
		}
		if k2 := keyOf(on, pwiCaseID, base); k2 != k1 {
			t.Errorf("same request gave %q then %q", k1, k2)
		}
		// Field order and an ignored repoOverride do not change the key.
		if k := keyOf(on, pwiCaseID, `{"repoOverride":{"owner":"a","repo":"b"},"title":" t ","description":"d","reason":"r","updateLevel":"u","publicIssueUrl":"p","issueTypeLabel":"Type/Discussion","priorityLevel":"Priority/High"}`); k != k1 {
			t.Errorf("equivalent body gave a different key")
		}
		for field, changed := range map[string]string{
			"title":                `{"title":"t2","description":"d","reason":"r","updateLevel":"u","publicIssueUrl":"p","issueTypeLabel":"Type/Discussion","priorityLevel":"Priority/High"}`,
			"description":          `{"title":"t","description":"d2","reason":"r","updateLevel":"u","publicIssueUrl":"p","issueTypeLabel":"Type/Discussion","priorityLevel":"Priority/High"}`,
			"reason":               `{"title":"t","description":"d","reason":"r2","updateLevel":"u","publicIssueUrl":"p","issueTypeLabel":"Type/Discussion","priorityLevel":"Priority/High"}`,
			"updateLevel":          `{"title":"t","description":"d","reason":"r","updateLevel":"u2","publicIssueUrl":"p","issueTypeLabel":"Type/Discussion","priorityLevel":"Priority/High"}`,
			"publicIssueUrl":       `{"title":"t","description":"d","reason":"r","updateLevel":"u","publicIssueUrl":"p2","issueTypeLabel":"Type/Discussion","priorityLevel":"Priority/High"}`,
			"issueTypeLabel":       `{"title":"t","description":"d","reason":"r","updateLevel":"u","publicIssueUrl":"p","issueTypeLabel":"Type/Patch","priorityLevel":"Priority/High"}`,
			"priorityLevel":        `{"title":"t","description":"d","reason":"r","updateLevel":"u","publicIssueUrl":"p","issueTypeLabel":"Type/Discussion","priorityLevel":"Priority/Low"}`,
			"regression":           `{"title":"t","description":"d","reason":"r","updateLevel":"u","publicIssueUrl":"p","issueTypeLabel":"Type/Discussion","priorityLevel":"Priority/High","regression":true}`,
			"hotFixRequired":       `{"title":"t","description":"d","reason":"r","updateLevel":"u","publicIssueUrl":"p","issueTypeLabel":"Type/Discussion","priorityLevel":"Priority/High","hotFixRequired":true}`,
			"onboardingInProgress": `{"title":"t","description":"d","reason":"r","updateLevel":"u","publicIssueUrl":"p","issueTypeLabel":"Type/Discussion","priorityLevel":"Priority/High","onboardingInProgress":true}`,
		} {
			if keyOf(on, pwiCaseID, changed) == k1 {
				t.Errorf("changing %s did not change the key", field)
			}
		}
		if keyOf(on, "33333333-3333-3333-3333-333333333333", base) == k1 {
			t.Error("a different case did not change the key")
		}
		// A different user gets a different key.
		other := *testUser
		other.UserID = "aaaaaaaa-7067-43dc-8578-802c8623af5d"
		r := httptest.NewRequest(http.MethodPost, "/cases/"+pwiCaseID+"/github-issues", strings.NewReader(base))
		r.SetPathValue("id", pwiCaseID)
		r = r.WithContext(middleware.WithUserInfo(r.Context(), &other))
		on.CreateCaseGithubIssue(httptest.NewRecorder(), r)
		var got struct {
			Key string `json:"idempotencyKey"`
		}
		_ = json.Unmarshal(gotBody, &got)
		if got.Key == k1 || got.Key == "" {
			t.Errorf("a different user got key %q (same as %q)", got.Key, k1)
		}
	})

	t.Run("rejects an invalid header key and bad bodies before any upstream call", func(t *testing.T) {
		calls = 0
		cases := []struct {
			name string
			body string
			hdr  map[string]string
		}{
			{"key too long", `{"title":"t"}`, map[string]string{"Idempotency-Key": strings.Repeat("k", 201)}},
			{"key with tab", `{"title":"t"}`, map[string]string{"Idempotency-Key": "a\tb"}},
			{"empty title", `{"title":"  "}`, nil},
			{"long title", `{"title":"` + strings.Repeat("a", 257) + `"}`, nil},
			{"long body", `{"title":"t","description":"` + strings.Repeat("a", 65537) + `"}`, nil},
			{"not an object", `[1]`, nil},
			{"title not text", `{"title":5}`, nil},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				assertStatus(t, post(on, tc.body, tc.hdr), http.StatusBadRequest)
			})
		}
		if calls != 0 {
			t.Errorf("entity called %d times", calls)
		}
	})

	t.Run("never touches the engineering client", func(t *testing.T) {
		eng := &mockEngineeringClient{}
		h := NewCaseHandler(mock).WithGitInternals(true).WithEngineeringClient(eng)
		assertStatus(t, post(h, `{"title":"t"}`, nil), http.StatusCreated)
		if len(eng.calls) != 0 {
			t.Errorf("engineering client called %d times", len(eng.calls))
		}
	})

	t.Run("maps entity errors", func(t *testing.T) {
		for _, tc := range []struct {
			err  error
			want int
			msg  string
			code string
		}{
			{&apierror.Error{StatusCode: 400, Body: `{"message":"No GitHub repository is mapped for this product."}`}, 400, "No GitHub repository is mapped for this product.", ""},
			{&apierror.Error{StatusCode: 409, Body: `{"message":"Another attempt is still running.","errorCode":"request_in_progress"}`}, 409, "Another attempt is still running.", "request_in_progress"},
			{&apierror.Error{StatusCode: 502, Body: `{"message":"secret upstream text"}`}, 503, "Failed to create GitHub issue.", ""},
			{&apierror.Error{StatusCode: 500, Body: `secret`}, 500, "Failed to create GitHub issue.", ""},
			{&apierror.Error{StatusCode: 403, Body: `{"message":"internal only"}`}, 403, ErrMsgForbidden, ""},
			{&apierror.Error{StatusCode: 409, Body: `<html>Bad gateway page</html>`}, 409, "The request conflicts with the current state.", ""},
			{&apierror.Error{StatusCode: 409, Body: `{"errorCode":"x"}`}, 409, "The request conflicts with the current state.", "x"},
			{errors.New("boom"), 500, "Failed to create GitHub issue.", ""},
		} {
			m := &mockEntityCaseClient{createCaseProductWorkItemFn: func(context.Context, string, []byte) ([]byte, error) { return nil, tc.err }}
			w := post(NewCaseHandler(m).WithGitInternals(true), `{"title":"t"}`, nil)
			assertStatus(t, w, tc.want)
			body := decodeJSON[map[string]string](t, w)
			if body["message"] != tc.msg || body["errorCode"] != tc.code {
				t.Errorf("%v: body = %v", tc.err, body)
			}
		}
	})

	t.Run("flag off keeps the entity-service forward and never calls the new endpoint", func(t *testing.T) {
		entityCalled := false
		m := &mockEntityCaseClient{
			createCaseGithubIssueFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				entityCalled = true
				if string(body) != `{"title":"t","extra":1}` {
					t.Errorf("body = %s, want the raw body forwarded", body)
				}
				return []byte(`{"message":"ok"}`), nil
			},
			createCaseProductWorkItemFn: func(context.Context, string, []byte) ([]byte, error) {
				t.Error("product-work-items called with the flag off")
				return nil, nil
			},
		}
		w := post(NewCaseHandler(m).WithGitInternals(false), `{"title":"t","extra":1}`, nil)
		assertStatus(t, w, http.StatusCreated)
		if !entityCalled {
			t.Error("entity github-issues not called")
		}
	})
}

type mockProductWorkItemClient struct {
	calls []string
	resp  []byte
	err   error
}

func (m *mockProductWorkItemClient) rec(s string) ([]byte, error) {
	m.calls = append(m.calls, s)
	return m.resp, m.err
}
func (m *mockProductWorkItemClient) ListCaseProductWorkItems(_ context.Context, c string) ([]byte, error) {
	return m.rec("list " + c)
}
func (m *mockProductWorkItemClient) GetProductWorkItem(_ context.Context, id string) ([]byte, error) {
	return m.rec("get " + id)
}
func (m *mockProductWorkItemClient) SearchProductWorkItems(_ context.Context, b []byte) ([]byte, error) {
	return m.rec("search " + string(b))
}
func (m *mockProductWorkItemClient) LinkCaseProductWorkItem(_ context.Context, c string, b []byte) ([]byte, error) {
	return m.rec("link " + c + " " + string(b))
}
func (m *mockProductWorkItemClient) UnlinkCaseProductWorkItem(_ context.Context, c, w string) ([]byte, error) {
	return m.rec("unlink " + c + " " + w)
}

func TestProductWorkItemProxies(t *testing.T) {
	type route struct {
		name    string
		method  string
		path    string
		paths   map[string]string
		body    string
		call    func(h *ProductWorkItemHandler) http.HandlerFunc
		want    string
		okCode  int
		generic bool
	}
	routes := []route{
		{"list", "GET", "/cases/" + pwiCaseID + "/product-work-items", map[string]string{"id": pwiCaseID}, "", func(h *ProductWorkItemHandler) http.HandlerFunc { return h.ListCaseProductWorkItems }, "list " + pwiCaseID, 200, true},
		{"get", "GET", "/product-work-items/" + pwiItemID, map[string]string{"id": pwiItemID}, "", func(h *ProductWorkItemHandler) http.HandlerFunc { return h.GetProductWorkItem }, "get " + pwiItemID, 200, true},
		{"search", "POST", "/product-work-items/search", nil, `{"query":"PI1"}`, func(h *ProductWorkItemHandler) http.HandlerFunc { return h.SearchProductWorkItems }, `search {"query":"PI1"}`, 200, true},
		{"link", "POST", "/cases/" + pwiCaseID + "/product-work-items/links", map[string]string{"id": pwiCaseID}, `{"workItemId":"` + pwiItemID + `"}`, func(h *ProductWorkItemHandler) http.HandlerFunc { return h.LinkCaseProductWorkItem }, "link " + pwiCaseID + ` {"workItemId":"` + pwiItemID + `"}`, 200, false},
		{"unlink", "DELETE", "/cases/" + pwiCaseID + "/product-work-items/" + pwiItemID + "/link", map[string]string{"id": pwiCaseID, "workItemId": pwiItemID}, "", func(h *ProductWorkItemHandler) http.HandlerFunc { return h.UnlinkCaseProductWorkItem }, "unlink " + pwiCaseID + " " + pwiItemID, 204, false},
	}
	do := func(rt route, h *ProductWorkItemHandler, user bool, paths map[string]string, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(rt.method, rt.path, strings.NewReader(body))
		if user {
			r = withUser(r)
		}
		for k, v := range paths {
			r.SetPathValue(k, v)
		}
		w := httptest.NewRecorder()
		rt.call(h)(w, r)
		return w
	}
	for _, rt := range routes {
		t.Run(rt.name+" passes the response through", func(t *testing.T) {
			m := &mockProductWorkItemClient{resp: []byte(`{"items":[]}`)}
			w := do(rt, NewProductWorkItemHandler(m), true, rt.paths, rt.body)
			assertStatus(t, w, rt.okCode)
			if rt.okCode == 200 && w.Body.String() != `{"items":[]}` {
				t.Errorf("body = %q", w.Body.String())
			}
			if len(m.calls) != 1 || m.calls[0] != rt.want {
				t.Errorf("calls = %v, want %q", m.calls, rt.want)
			}
		})
		t.Run(rt.name+" requires a user", func(t *testing.T) {
			m := &mockProductWorkItemClient{}
			assertStatus(t, do(rt, NewProductWorkItemHandler(m), false, rt.paths, rt.body), http.StatusUnauthorized)
			if len(m.calls) != 0 {
				t.Error("upstream called")
			}
		})
		if rt.paths != nil {
			t.Run(rt.name+" rejects a non-UUID id without an upstream call", func(t *testing.T) {
				for k := range rt.paths {
					bad := map[string]string{}
					for k2, v := range rt.paths {
						bad[k2] = v
					}
					bad[k] = "not-a-uuid"
					m := &mockProductWorkItemClient{}
					w := do(rt, NewProductWorkItemHandler(m), true, bad, rt.body)
					assertStatus(t, w, http.StatusBadRequest)
					assertErrorMessage(t, w, ErrMsgInvalidUUID)
					if len(m.calls) != 0 {
						t.Error("upstream called")
					}
				}
			})
		}
		if rt.body != "" {
			t.Run(rt.name+" rejects non-object and oversized bodies", func(t *testing.T) {
				for body, want := range map[string]int{`[1]`: 400, `nope`: 400, `null`: 400, strings.Repeat("x", maxRequestBodyBytes+1): 413} {
					m := &mockProductWorkItemClient{}
					assertStatus(t, do(rt, NewProductWorkItemHandler(m), true, rt.paths, body), want)
					if len(m.calls) != 0 {
						t.Error("upstream called")
					}
				}
			})
		}
		t.Run(rt.name+" maps errors", func(t *testing.T) {
			conflict := &apierror.Error{StatusCode: 409, Body: `{"message":"Already linked to case CS1.","errorCode":"already_linked"}`}
			m := &mockProductWorkItemClient{err: conflict}
			w := do(rt, NewProductWorkItemHandler(m), true, rt.paths, rt.body)
			if rt.generic {
				assertStatus(t, w, 409)
				if body := decodeJSON[map[string]string](t, w); strings.Contains(body["message"], "Already linked") {
					t.Errorf("read route echoed the upstream reason: %v", body)
				}
			} else {
				assertStatus(t, w, 409)
				if body := decodeJSON[map[string]string](t, w); body["message"] != "Already linked to case CS1." || body["errorCode"] != "already_linked" {
					t.Errorf("body = %v", body)
				}
			}
			m = &mockProductWorkItemClient{err: &apierror.Error{StatusCode: 404}}
			assertStatus(t, do(rt, NewProductWorkItemHandler(m), true, rt.paths, rt.body), http.StatusNotFound)
			m = &mockProductWorkItemClient{err: &apierror.Error{StatusCode: 500, Body: "secret"}}
			w = do(rt, NewProductWorkItemHandler(m), true, rt.paths, rt.body)
			assertStatus(t, w, 500)
			if strings.Contains(w.Body.String(), "secret") {
				t.Error("upstream body leaked")
			}
		})
	}

	t.Run("link and unlink never echo a raw 409 body and map 403", func(t *testing.T) {
		for _, rt := range routes[3:] {
			m := &mockProductWorkItemClient{err: &apierror.Error{StatusCode: 409, Body: "<html>secret page</html>"}}
			w := do(rt, NewProductWorkItemHandler(m), true, rt.paths, rt.body)
			assertStatus(t, w, 409)
			if strings.Contains(w.Body.String(), "secret") || decodeJSON[map[string]string](t, w)["message"] != "The request conflicts with the current state." {
				t.Errorf("%s: raw body echoed or wrong fallback: %s", rt.name, w.Body.String())
			}
			m = &mockProductWorkItemClient{err: &apierror.Error{StatusCode: 403, Body: "x"}}
			w = do(rt, NewProductWorkItemHandler(m), true, rt.paths, rt.body)
			assertStatus(t, w, 403)
			assertErrorMessage(t, w, ErrMsgForbidden)
		}
	})

	t.Run("detail redacts inline images unless the caller may download attachments", func(t *testing.T) {
		const payload = `{"conversation":[{"body":"![x](data:image/png;base64,AAAA)"}]}`
		get := routes[1]
		w := do(get, NewProductWorkItemHandler(&mockProductWorkItemClient{resp: []byte(payload)}), true, get.paths, "")
		if strings.Contains(w.Body.String(), "AAAA") {
			t.Error("nil guard must redact")
		}
		viewer := *testUser
		viewer.Roles = []string{"test-viewer"}
		h := NewProductWorkItemHandler(&mockProductWorkItemClient{resp: []byte(payload)}).WithAccessGuard(viewerAccessGuard)
		r := httptest.NewRequest("GET", get.path, nil)
		r.SetPathValue("id", pwiItemID)
		r = r.WithContext(middleware.WithUserInfo(r.Context(), &viewer))
		rec := httptest.NewRecorder()
		h.GetProductWorkItem(rec, r)
		if strings.Contains(rec.Body.String(), "AAAA") {
			t.Error("a viewer must be redacted")
		}
		// testUser holds the attachment downloader role.
		rec = httptest.NewRecorder()
		h.GetProductWorkItem(rec, withUserPath(get.path, pwiItemID))
		if !strings.Contains(rec.Body.String(), "AAAA") {
			t.Errorf("a downloader should see the image: %s", rec.Body.String())
		}
	})
}

func withUserPath(path, id string) *http.Request {
	r := withUser(httptest.NewRequest("GET", path, nil))
	r.SetPathValue("id", id)
	return r
}
