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

const testAlertMuteRuleID = "11111111-2222-3333-4444-555555555555"

const testAlertMuteRuleSave = `{"name":"Ballerina Central deploy","description":"","timeZone":"Asia/Colombo","startOn":"2026-11-07T00:00:00Z","endOn":"2026-11-07T02:00:00Z","conditions":[{"field":"service","value":"svc"},{"field":"environment","value":"prod"}]}`

// alertMuteRuleRoute describes one handler so the shared guards (auth, UUID, body)
// and the upstream-error mapping are tested for every route the same way.
type alertMuteRuleRoute struct {
	name     string
	method   string
	path     string // with testAlertMuteRuleID substituted for {id}
	hasID    bool
	body     string // "" means the route takes no body
	okStatus int
	generic  bool // reads use mapUpstreamErrorGeneric, writes mapUpstreamError
	fallback string
	call     func(h *AlertMuteRuleHandler, w http.ResponseWriter, r *http.Request)
	failWith func(m *mockEntityAlertMuteRuleClient, err error)
}

func alertMuteRuleRoutes() []alertMuteRuleRoute {
	return []alertMuteRuleRoute{
		{
			name: "search", method: http.MethodPost, path: "/alert-mute-rules/search",
			body: `{"filters":{"states":["active"]},"pagination":{"offset":0,"limit":20}}`, okStatus: http.StatusOK,
			generic: true, fallback: "Failed to search alert mute rules.",
			call: (*AlertMuteRuleHandler).SearchAlertMuteRules,
			failWith: func(m *mockEntityAlertMuteRuleClient, err error) {
				m.searchFn = func(context.Context, []byte) ([]byte, error) { return nil, err }
			},
		},
		{
			name: "get", method: http.MethodGet, path: "/alert-mute-rules/" + testAlertMuteRuleID, hasID: true,
			okStatus: http.StatusOK, generic: true, fallback: "Failed to retrieve alert mute rule.",
			call: (*AlertMuteRuleHandler).GetAlertMuteRule,
			failWith: func(m *mockEntityAlertMuteRuleClient, err error) {
				m.getFn = func(context.Context, string) ([]byte, error) { return nil, err }
			},
		},
		{
			name: "create", method: http.MethodPost, path: "/alert-mute-rules",
			body: testAlertMuteRuleSave, okStatus: http.StatusCreated, fallback: "Failed to create alert mute rule.",
			call: (*AlertMuteRuleHandler).CreateAlertMuteRule,
			failWith: func(m *mockEntityAlertMuteRuleClient, err error) {
				m.createFn = func(context.Context, []byte) ([]byte, error) { return nil, err }
			},
		},
		{
			name: "patch", method: http.MethodPatch, path: "/alert-mute-rules/" + testAlertMuteRuleID, hasID: true,
			body: testAlertMuteRuleSave, okStatus: http.StatusOK, fallback: "Failed to update alert mute rule.",
			call: (*AlertMuteRuleHandler).PatchAlertMuteRule,
			failWith: func(m *mockEntityAlertMuteRuleClient, err error) {
				m.patchFn = func(context.Context, string, []byte) ([]byte, error) { return nil, err }
			},
		},
		{
			name: "extend", method: http.MethodPost, path: "/alert-mute-rules/" + testAlertMuteRuleID + "/extend", hasID: true,
			body: `{"endOn":"2026-11-07T04:00:00Z"}`, okStatus: http.StatusOK, fallback: "Failed to extend alert mute rule.",
			call: (*AlertMuteRuleHandler).ExtendAlertMuteRule,
			failWith: func(m *mockEntityAlertMuteRuleClient, err error) {
				m.extendFn = func(context.Context, string, []byte) ([]byte, error) { return nil, err }
			},
		},
		{
			name: "end", method: http.MethodPost, path: "/alert-mute-rules/" + testAlertMuteRuleID + "/end", hasID: true,
			okStatus: http.StatusOK, fallback: "Failed to end alert mute rule.",
			call: (*AlertMuteRuleHandler).EndAlertMuteRule,
			failWith: func(m *mockEntityAlertMuteRuleClient, err error) {
				m.endFn = func(context.Context, string) ([]byte, error) { return nil, err }
			},
		},
		{
			name: "delete", method: http.MethodDelete, path: "/alert-mute-rules/" + testAlertMuteRuleID, hasID: true,
			okStatus: http.StatusNoContent, fallback: "Failed to delete alert mute rule.",
			call: (*AlertMuteRuleHandler).DeleteAlertMuteRule,
			failWith: func(m *mockEntityAlertMuteRuleClient, err error) {
				m.deleteFn = func(context.Context, string) error { return err }
			},
		},
	}
}

func (rt alertMuteRuleRoute) request(body, id string) *http.Request {
	r := httptest.NewRequest(rt.method, rt.path, strings.NewReader(body))
	if rt.hasID {
		r.SetPathValue("id", id)
	}
	return r
}

func TestAlertMuteRuleRoutesGuards(t *testing.T) {
	for _, rt := range alertMuteRuleRoutes() {
		t.Run(rt.name+" requires authenticated user", func(t *testing.T) {
			w := httptest.NewRecorder()
			rt.call(NewAlertMuteRuleHandler(&mockEntityAlertMuteRuleClient{}), w, rt.request(rt.body, testAlertMuteRuleID))
			assertStatus(t, w, http.StatusUnauthorized)
			assertErrorMessage(t, w, ErrMsgUnauthorized)
		})

		if rt.hasID {
			t.Run(rt.name+" rejects malformed UUID", func(t *testing.T) {
				w := httptest.NewRecorder()
				rt.call(NewAlertMuteRuleHandler(&mockEntityAlertMuteRuleClient{}), w, withUser(rt.request(rt.body, "active")))
				assertStatus(t, w, http.StatusBadRequest)
				assertErrorMessage(t, w, ErrMsgInvalidUUID)
			})
		}

		if rt.body != "" {
			t.Run(rt.name+" rejects invalid JSON", func(t *testing.T) {
				w := httptest.NewRecorder()
				rt.call(NewAlertMuteRuleHandler(&mockEntityAlertMuteRuleClient{}), w, withUser(rt.request("not-json", testAlertMuteRuleID)))
				assertStatus(t, w, http.StatusBadRequest)
			})
			t.Run(rt.name+" rejects body exceeding 1 MiB", func(t *testing.T) {
				w := httptest.NewRecorder()
				rt.call(NewAlertMuteRuleHandler(&mockEntityAlertMuteRuleClient{}), w, withUser(rt.request(strings.Repeat("x", maxRequestBodyBytes+1), testAlertMuteRuleID)))
				assertStatus(t, w, http.StatusRequestEntityTooLarge)
				assertErrorMessage(t, w, ErrMsgTooLarge)
			})
		}
	}
}

func TestAlertMuteRuleRoutesSuccess(t *testing.T) {
	for _, rt := range alertMuteRuleRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			rt.call(NewAlertMuteRuleHandler(&mockEntityAlertMuteRuleClient{}), w, withUser(rt.request(rt.body, testAlertMuteRuleID)))
			assertStatus(t, w, rt.okStatus)
			if rt.okStatus == http.StatusNoContent && w.Body.Len() != 0 {
				t.Errorf("204 response has a body: %q", w.Body.String())
			}
		})
	}
}

func TestAlertMuteRuleRoutesUpstreamErrors(t *testing.T) {
	for _, rt := range alertMuteRuleRoutes() {
		cases := upstreamErrors(rt.fallback)
		if rt.generic {
			cases = upstreamErrorsGeneric(rt.fallback)
		}
		for _, tc := range cases {
			t.Run(rt.name+"/"+tc.name, func(t *testing.T) {
				m := &mockEntityAlertMuteRuleClient{}
				rt.failWith(m, tc.err)
				w := httptest.NewRecorder()
				rt.call(NewAlertMuteRuleHandler(m), w, withUser(rt.request(rt.body, testAlertMuteRuleID)))
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	}
}

func TestCreateAlertMuteRuleForwardsBodyUnchanged(t *testing.T) {
	var got []byte
	m := &mockEntityAlertMuteRuleClient{createFn: func(_ context.Context, body []byte) ([]byte, error) {
		got = body
		return []byte(`{"id":"x"}`), nil
	}}
	w := httptest.NewRecorder()
	NewAlertMuteRuleHandler(m).CreateAlertMuteRule(w, withUser(httptest.NewRequest(http.MethodPost, "/alert-mute-rules", strings.NewReader(testAlertMuteRuleSave))))
	assertStatus(t, w, http.StatusCreated)
	if string(got) != testAlertMuteRuleSave {
		t.Errorf("upstream got %q, want %q", got, testAlertMuteRuleSave)
	}
	if w.Body.String() != `{"id":"x"}` {
		t.Errorf("response %q is not the upstream body", w.Body.String())
	}
}

func TestPatchAlertMuteRuleForwardsIDAndBody(t *testing.T) {
	var gotID, gotBody string
	m := &mockEntityAlertMuteRuleClient{patchFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
		gotID, gotBody = id, string(body)
		return []byte(`{}`), nil
	}}
	r := withUser(httptest.NewRequest(http.MethodPatch, "/alert-mute-rules/"+testAlertMuteRuleID, strings.NewReader(testAlertMuteRuleSave)))
	r.SetPathValue("id", testAlertMuteRuleID)
	w := httptest.NewRecorder()
	NewAlertMuteRuleHandler(m).PatchAlertMuteRule(w, r)
	assertStatus(t, w, http.StatusOK)
	if gotID != testAlertMuteRuleID || gotBody != testAlertMuteRuleSave {
		t.Errorf("upstream got id=%q body=%q", gotID, gotBody)
	}
}

func TestAlertMuteRuleSaveRejectsNonObjectBody(t *testing.T) {
	for _, body := range []string{`[1,2]`, `null`} {
		t.Run(body, func(t *testing.T) {
			w := httptest.NewRecorder()
			NewAlertMuteRuleHandler(&mockEntityAlertMuteRuleClient{}).CreateAlertMuteRule(w, withUser(httptest.NewRequest(http.MethodPost, "/alert-mute-rules", strings.NewReader(body))))
			assertStatus(t, w, http.StatusBadRequest)
			assertErrorMessage(t, w, ErrMsgBadRequest)
		})
	}
}

func TestExtendAlertMuteRuleRequiresEndOn(t *testing.T) {
	for _, body := range []string{`{}`, `{"endOn":""}`, `{"endOn":"  "}`, `{"endOn":5}`} {
		t.Run(body, func(t *testing.T) {
			called := false
			m := &mockEntityAlertMuteRuleClient{extendFn: func(context.Context, string, []byte) ([]byte, error) {
				called = true
				return nil, nil
			}}
			r := withUser(httptest.NewRequest(http.MethodPost, "/alert-mute-rules/"+testAlertMuteRuleID+"/extend", strings.NewReader(body)))
			r.SetPathValue("id", testAlertMuteRuleID)
			w := httptest.NewRecorder()
			NewAlertMuteRuleHandler(m).ExtendAlertMuteRule(w, r)
			assertStatus(t, w, http.StatusBadRequest)
			assertErrorMessage(t, w, "endOn is required.")
			if called {
				t.Error("entity service was called for an invalid body")
			}
		})
	}
}

func TestEndAlertMuteRuleForwardsID(t *testing.T) {
	var got string
	m := &mockEntityAlertMuteRuleClient{endFn: func(_ context.Context, id string) ([]byte, error) {
		got = id
		return []byte(`{"endedOn":"2026-11-07T01:00:00Z"}`), nil
	}}
	r := withUser(httptest.NewRequest(http.MethodPost, "/alert-mute-rules/"+testAlertMuteRuleID+"/end", nil))
	r.SetPathValue("id", testAlertMuteRuleID)
	w := httptest.NewRecorder()
	NewAlertMuteRuleHandler(m).EndAlertMuteRule(w, r)
	assertStatus(t, w, http.StatusOK)
	if got != testAlertMuteRuleID {
		t.Errorf("upstream got id %q", got)
	}
}
