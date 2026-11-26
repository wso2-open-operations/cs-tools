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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
)

type fakeCodeDashboardClient struct {
	called bool
	gotID  string
	gotName,
	gotSource string

	list []entity.CodeDashboardSummary
	get  entity.CodeDashboard
	err  error
}

func (f *fakeCodeDashboardClient) ListCodeDashboards(context.Context) ([]entity.CodeDashboardSummary, error) {
	f.called = true
	return f.list, f.err
}

func (f *fakeCodeDashboardClient) GetCodeDashboard(_ context.Context, id string) (entity.CodeDashboard, error) {
	f.called, f.gotID = true, id
	return f.get, f.err
}

func (f *fakeCodeDashboardClient) SaveCodeDashboard(_ context.Context, id, name, source string) (entity.CodeDashboard, error) {
	f.called, f.gotID, f.gotName, f.gotSource = true, id, name, source
	if f.err != nil {
		return entity.CodeDashboard{}, f.err
	}
	return entity.CodeDashboard{ID: id, Name: name, Source: source, CreatedOn: "2026-11-13T10:00:00Z", UpdatedOn: "2026-11-13T10:05:00Z"}, nil
}

func (f *fakeCodeDashboardClient) DeleteCodeDashboard(_ context.Context, id string) error {
	f.called, f.gotID = true, id
	return f.err
}

// serveCodeDashboards registers the handler with main.go's patterns, so
// r.PathValue("id") is filled in as in production. withUser adds the caller.
func serveCodeDashboards(client *fakeCodeDashboardClient, method, path string, body []byte, signedIn bool) *httptest.ResponseRecorder {
	h := NewCodeDashboardHandler(client)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /dashboards/code", h.ListCodeDashboards)
	mux.HandleFunc("GET /dashboards/code/{id}", h.GetCodeDashboard)
	mux.HandleFunc("PUT /dashboards/code/{id}", h.SaveCodeDashboard)
	mux.HandleFunc("DELETE /dashboards/code/{id}", h.DeleteCodeDashboard)
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	if signedIn {
		r = withUser(r)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

func codeSaveBody(t *testing.T, name, source string) []byte {
	t.Helper()
	b, err := json.Marshal(saveCodeDashboardRequest{Name: name, Source: source})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

const testCodeSource = "async function render(portal, root) {\n  root.textContent = 'Open';\n}\n"

func TestCodeDashboards_RequireUser(t *testing.T) {
	path := "/dashboards/code/" + testCustomDashboardID
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/dashboards/code"}, {http.MethodGet, path}, {http.MethodPut, path}, {http.MethodDelete, path},
	} {
		client := &fakeCodeDashboardClient{}
		w := serveCodeDashboards(client, tc.method, tc.path, codeSaveBody(t, "X", "x"), false)
		if w.Code != http.StatusUnauthorized || client.called {
			t.Errorf("%s %s: status %d, called %v; want 401 and no upstream call", tc.method, tc.path, w.Code, client.called)
		}
	}
}

func TestCodeDashboards_RejectBadID(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		client := &fakeCodeDashboardClient{}
		w := serveCodeDashboards(client, method, "/dashboards/code/not-a-uuid", codeSaveBody(t, "X", "x"), true)
		if w.Code != http.StatusBadRequest || client.called {
			t.Errorf("%s: status %d, called %v; want 400 and no upstream call", method, w.Code, client.called)
		}
	}
}

func TestListCodeDashboards(t *testing.T) {
	client := &fakeCodeDashboardClient{list: []entity.CodeDashboardSummary{
		{ID: testCustomDashboardID, Name: "Open Cases", CreatedOn: "2026-11-13T10:00:00Z", UpdatedOn: "2026-11-13T10:05:00Z"},
	}}
	w := serveCodeDashboards(client, http.MethodGet, "/dashboards/code", nil, true)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[codeDashboardListResponse](t, w)
	want := []codeDashboardSummary{{ID: testCustomDashboardID, Name: "Open Cases", CreatedOn: "2026-11-13T10:00:00Z", UpdatedOn: "2026-11-13T10:05:00Z"}}
	if !reflect.DeepEqual(got.Dashboards, want) {
		t.Errorf("dashboards = %+v, want %+v", got.Dashboards, want)
	}
	empty := serveCodeDashboards(&fakeCodeDashboardClient{}, http.MethodGet, "/dashboards/code", nil, true)
	if strings.TrimSpace(empty.Body.String()) != `{"dashboards":[]}` {
		t.Errorf("empty list body = %s, want an empty array", empty.Body.String())
	}
}

func TestGetCodeDashboard(t *testing.T) {
	client := &fakeCodeDashboardClient{get: entity.CodeDashboard{
		ID: testCustomDashboardID, Name: "Open Cases", Source: testCodeSource, CreatedOn: "a", UpdatedOn: "b",
	}}
	w := serveCodeDashboards(client, http.MethodGet, "/dashboards/code/"+strings.ToUpper(testCustomDashboardID), nil, true)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if client.gotID != testCustomDashboardID {
		t.Errorf("id sent = %q, want it lower-cased", client.gotID)
	}
	got := decodeJSON[codeDashboardResponse](t, w)
	want := codeDashboardResponse{ID: testCustomDashboardID, Name: "Open Cases", Source: testCodeSource, CreatedOn: "a", UpdatedOn: "b"}
	if got != want {
		t.Errorf("body = %+v, want %+v", got, want)
	}
}

func TestSaveCodeDashboard_TrimsNameKeepsSource(t *testing.T) {
	client := &fakeCodeDashboardClient{}
	w := serveCodeDashboards(client, http.MethodPut, "/dashboards/code/"+strings.ToUpper(testCustomDashboardID),
		codeSaveBody(t, "  Open Cases  ", testCodeSource), true)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if client.gotID != testCustomDashboardID || client.gotName != "Open Cases" || client.gotSource != testCodeSource {
		t.Errorf("saved (%q, %q, %q), want the lower-cased id, the trimmed name and the source as sent", client.gotID, client.gotName, client.gotSource)
	}
	got := decodeJSON[codeDashboardResponse](t, w)
	if got.ID != testCustomDashboardID || got.Name != "Open Cases" || got.Source != testCodeSource || got.UpdatedOn == "" {
		t.Errorf("body = %+v", got)
	}
}

func TestSaveCodeDashboard_AcceptsTheLimits(t *testing.T) {
	client := &fakeCodeDashboardClient{}
	// Characters, not bytes: 255 two-byte characters of name, 60000 four-byte characters of code.
	w := serveCodeDashboards(client, http.MethodPut, "/dashboards/code/"+testCustomDashboardID,
		codeSaveBody(t, strings.Repeat("é", customDashboardNameMaxLength), strings.Repeat("😀", generatedCodeMaxLength)), true)
	if w.Code != http.StatusOK || !client.called {
		t.Fatalf("status = %d, called %v; want 200 at the limits: %s", w.Code, client.called, w.Body.String())
	}
}

func TestSaveCodeDashboard_RejectsBeforeCallingEntity(t *testing.T) {
	cases := []struct {
		name string
		body []byte
		want int
		msg  string
	}{
		{"blank name", codeSaveBody(t, "  ", testCodeSource), http.StatusBadRequest, errMsgCodeDashboardNameEmpty},
		{"name too long", codeSaveBody(t, strings.Repeat("é", customDashboardNameMaxLength+1), testCodeSource), http.StatusBadRequest, errMsgCodeDashboardNameTooLong},
		{"blank source", codeSaveBody(t, "X", " \n\t "), http.StatusBadRequest, errMsgCodeDashboardSourceEmpty},
		{"source too long", codeSaveBody(t, "X", strings.Repeat("a", generatedCodeMaxLength+1)), http.StatusRequestEntityTooLarge, errMsgCodeDashboardTooLarge},
		{"not JSON", []byte("name=X"), http.StatusBadRequest, ErrMsgBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeCodeDashboardClient{}
			w := serveCodeDashboards(client, http.MethodPut, "/dashboards/code/"+testCustomDashboardID, tc.body, true)
			if w.Code != tc.want || client.called {
				t.Fatalf("status %d, called %v; want %d and no upstream call: %s", w.Code, client.called, tc.want, w.Body.String())
			}
			if got := decodeJSON[errorBody](t, w); got.Message != tc.msg {
				t.Errorf("message = %q, want %q", got.Message, tc.msg)
			}
		})
	}
}

func TestSaveCodeDashboard_UpstreamErrors(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		want     int
		wantText string
	}{
		{"name taken", http.StatusConflict, `{"code":409,"message":"you already have an advanced dashboard named: Open Cases"}`,
			http.StatusConflict, "you already have an advanced dashboard named: Open Cases"},
		{"limit reached", http.StatusBadRequest, `{"code":400,"message":"at most 50 saved advanced dashboards are allowed"}`,
			http.StatusBadRequest, "at most 50 saved advanced dashboards are allowed"},
		{"another user's id", http.StatusNotFound, `{"code":404,"message":"dashboard not found"}`,
			http.StatusNotFound, ErrMsgNotFound},
		{"entity-service failed", http.StatusInternalServerError, `{"code":500,"message":"pq: connection refused"}`,
			http.StatusInternalServerError, errMsgCodeDashboardSaveFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeCodeDashboardClient{err: &apierror.Error{StatusCode: tc.status, Body: tc.body}}
			w := serveCodeDashboards(client, http.MethodPut, "/dashboards/code/"+testCustomDashboardID, codeSaveBody(t, "Open Cases", testCodeSource), true)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if got := decodeJSON[errorBody](t, w); got.Message != tc.wantText {
				t.Errorf("message = %q, want %q", got.Message, tc.wantText)
			}
		})
	}
}

// Reads and deletes show a fixed message for anything but 401 and 404: the
// upstream detail is logged, not echoed.
func TestCodeDashboards_ReadAndDeleteErrorsAreGeneric(t *testing.T) {
	path := "/dashboards/code/" + testCustomDashboardID
	cases := []struct {
		method, path string
		status, want int
		msg          string
	}{
		{http.MethodGet, "/dashboards/code", http.StatusInternalServerError, http.StatusInternalServerError, errMsgCodeDashboardListFailed},
		{http.MethodGet, "/dashboards/code", http.StatusBadRequest, http.StatusBadRequest, errMsgCodeDashboardListFailed},
		{http.MethodGet, path, http.StatusNotFound, http.StatusNotFound, ErrMsgNotFound},
		{http.MethodGet, path, http.StatusInternalServerError, http.StatusInternalServerError, errMsgCodeDashboardLoadFailed},
		{http.MethodDelete, path, http.StatusNotFound, http.StatusNotFound, ErrMsgNotFound},
		{http.MethodDelete, path, http.StatusInternalServerError, http.StatusInternalServerError, errMsgCodeDashboardDeleteFailed},
	}
	for _, tc := range cases {
		client := &fakeCodeDashboardClient{err: &apierror.Error{StatusCode: tc.status, Body: `{"code":0,"message":"upstream detail"}`}}
		w := serveCodeDashboards(client, tc.method, tc.path, nil, true)
		if w.Code != tc.want {
			t.Errorf("%s %s upstream %d: status = %d, want %d", tc.method, tc.path, tc.status, w.Code, tc.want)
			continue
		}
		if got := decodeJSON[errorBody](t, w); got.Message != tc.msg {
			t.Errorf("%s %s upstream %d: message = %q, want %q", tc.method, tc.path, tc.status, got.Message, tc.msg)
		}
	}
}

func TestDeleteCodeDashboard(t *testing.T) {
	client := &fakeCodeDashboardClient{}
	w := serveCodeDashboards(client, http.MethodDelete, "/dashboards/code/"+strings.ToUpper(testCustomDashboardID), nil, true)
	if w.Code != http.StatusNoContent || client.gotID != testCustomDashboardID {
		t.Fatalf("status %d, id %q; want 204 and the lower-cased id", w.Code, client.gotID)
	}
}
