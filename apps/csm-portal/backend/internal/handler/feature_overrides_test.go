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

type fakeFeatureOverrideClient struct {
	listBody []byte
	listErr  error
	putBody  []byte
	putErr   error
	putCalls int
	gotBody  []byte
}

func (f *fakeFeatureOverrideClient) ListFeatureOverrides(context.Context) ([]byte, error) {
	return f.listBody, f.listErr
}

func (f *fakeFeatureOverrideClient) PutFeatureOverride(_ context.Context, body []byte) ([]byte, error) {
	f.putCalls++
	f.gotBody = body
	return f.putBody, f.putErr
}

func putFeatureOverride(h *FeatureOverrideHandler, body string, signedIn bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut, "/features", strings.NewReader(body))
	if signedIn {
		req = withUser(req)
	}
	w := httptest.NewRecorder()
	h.PutFeatureOverride(w, req)
	return w
}

func TestListFeatureOverridesPassesTheBodyThrough(t *testing.T) {
	const body = `{"overrides":[{"navNodeId":"operations","state":"preview","updatedOn":"2026-11-05T10:00:00Z","updatedBy":"admin@example.com"}],"savedOnce":true}`
	h := NewFeatureOverrideHandler(&fakeFeatureOverrideClient{listBody: []byte(body)})
	w := httptest.NewRecorder()
	h.ListFeatureOverrides(w, withUser(httptest.NewRequest(http.MethodGet, "/features", nil)))
	if w.Code != http.StatusOK || w.Body.String() != body {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", w.Header().Get("Cache-Control"))
	}
}

func TestListFeatureOverridesRequiresASignedInUser(t *testing.T) {
	h := NewFeatureOverrideHandler(&fakeFeatureOverrideClient{})
	w := httptest.NewRecorder()
	h.ListFeatureOverrides(w, httptest.NewRequest(http.MethodGet, "/features", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", w.Code)
	}
}

func TestPutFeatureOverrideForwardsAValidSave(t *testing.T) {
	for _, state := range []string{"enabled", "preview"} {
		client := &fakeFeatureOverrideClient{putBody: []byte(`{"navNodeId":"operations.incidents","state":"` + state + `"}`)}
		w := putFeatureOverride(NewFeatureOverrideHandler(client),
			`{"navNodeId":" operations.incidents ","state":"`+state+`","markSaved":true}`, true)
		if w.Code != http.StatusOK || client.putCalls != 1 {
			t.Fatalf("%s: status %d calls %d", state, w.Code, client.putCalls)
		}
		// The id is trimmed and markSaved is carried through.
		if got := string(client.gotBody); !strings.Contains(got, `"navNodeId":"operations.incidents"`) || !strings.Contains(got, `"markSaved":true`) {
			t.Fatalf("forwarded %s", got)
		}
	}
}

func TestPutFeatureOverrideRejectsABadSaveBeforeCallingEntity(t *testing.T) {
	cases := map[string]string{
		"wip state":         `{"navNodeId":"operations","state":"wip"}`,
		"hidden state":      `{"navNodeId":"operations","state":"hidden"}`,
		"unknown state":     `{"navNodeId":"operations","state":"on"}`,
		"missing state":     `{"navNodeId":"operations"}`,
		"blank id":          `{"navNodeId":"  ","state":"enabled"}`,
		"missing id":        `{"state":"enabled"}`,
		"upper-case id":     `{"navNodeId":"Operations","state":"enabled"}`,
		"id with slash":     `{"navNodeId":"operations/incidents","state":"enabled"}`,
		"id too long":       `{"navNodeId":"` + strings.Repeat("a", maxFeatureNavNodeIDLen+1) + `","state":"enabled"}`,
		"unknown field":     `{"navNodeId":"operations","state":"enabled","extra":1}`,
		"trailing bracket":  `{"navNodeId":"operations","state":"enabled"}]`,
		"second object":     `{"navNodeId":"operations","state":"enabled"}{"navNodeId":"a","state":"enabled"}`,
		"not json":          `not json`,
		"markSaved as text": `{"navNodeId":"operations","state":"enabled","markSaved":"yes"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			client := &fakeFeatureOverrideClient{}
			w := putFeatureOverride(NewFeatureOverrideHandler(client), body, true)
			if w.Code != http.StatusBadRequest || client.putCalls != 0 {
				t.Fatalf("status %d calls %d body %s", w.Code, client.putCalls, w.Body.String())
			}
		})
	}
}

func TestPutFeatureOverrideRejectsAnOversizeBody(t *testing.T) {
	client := &fakeFeatureOverrideClient{}
	body := `{"navNodeId":"operations","state":"enabled","markSaved":false}` + strings.Repeat(" ", maxRequestBodyBytes)
	w := putFeatureOverride(NewFeatureOverrideHandler(client), body, true)
	if w.Code != http.StatusRequestEntityTooLarge || client.putCalls != 0 {
		t.Fatalf("status %d calls %d", w.Code, client.putCalls)
	}
}

func TestPutFeatureOverrideRequiresASignedInUser(t *testing.T) {
	client := &fakeFeatureOverrideClient{}
	w := putFeatureOverride(NewFeatureOverrideHandler(client), `{"navNodeId":"operations","state":"enabled"}`, false)
	if w.Code != http.StatusUnauthorized || client.putCalls != 0 {
		t.Fatalf("status %d calls %d", w.Code, client.putCalls)
	}
}

func TestFeatureOverrideUpstreamErrorsAreMapped(t *testing.T) {
	for _, tc := range upstreamErrorsGeneric("Failed to save feature visibility.") {
		t.Run("put "+tc.name, func(t *testing.T) {
			w := putFeatureOverride(NewFeatureOverrideHandler(&fakeFeatureOverrideClient{putErr: tc.err}),
				`{"navNodeId":"operations","state":"enabled"}`, true)
			if w.Code != tc.wantCode {
				t.Fatalf("status %d, want %d", w.Code, tc.wantCode)
			}
		})
	}
	for _, tc := range upstreamErrorsGeneric("Failed to load feature visibility.") {
		t.Run("list "+tc.name, func(t *testing.T) {
			h := NewFeatureOverrideHandler(&fakeFeatureOverrideClient{listErr: tc.err})
			w := httptest.NewRecorder()
			h.ListFeatureOverrides(w, withUser(httptest.NewRequest(http.MethodGet, "/features", nil)))
			if w.Code != tc.wantCode {
				t.Fatalf("status %d, want %d", w.Code, tc.wantCode)
			}
		})
	}
}

// Every signed-in shell must be able to read the shared state, including a
// role that does not hold PermView, and only an admin may change it.
func TestFeatureOverridePermissions(t *testing.T) {
	g := NewAccessGuard(testAccessConfig())
	for _, roles := range [][]string{{"test-viewer"}, {"test-cs-engineer"}, {"test-worknote-creator"}} {
		if status, reached := serveWithRoles(g, PermAuthenticated, roles); status != http.StatusNoContent || !reached {
			t.Errorf("%v: read denied (%d)", roles, status)
		}
		if status, reached := serveWithRoles(g, PermAdmin, roles); status != http.StatusForbidden || reached {
			t.Errorf("%v: write allowed (%d)", roles, status)
		}
	}
	if status, reached := serveWithRoles(g, PermAdmin, []string{"test-admin"}); status != http.StatusNoContent || !reached {
		t.Errorf("admin write denied (%d)", status)
	}
}
