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
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/directory"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
)

const testCustomDashboardID = "5e636a12-0000-4000-8000-000000000001"

type fakeCustomDashboardClient struct {
	called         bool
	gotID, gotName string
	gotDefinition  string
	list           []entity.CustomDashboardSummary
	stored         entity.CustomDashboard
	gotUserID      string
	shares         []entity.CustomDashboardShare
	sharedList     []entity.SharedCustomDashboardSummary
	shared         entity.SharedCustomDashboard
	gotTeamID      string
	teamShares     []entity.CustomDashboardTeamShare
	err            error
}

func (f *fakeCustomDashboardClient) ListCustomDashboardTeamShares(_ context.Context, id string) ([]entity.CustomDashboardTeamShare, error) {
	f.called = true
	f.gotID = id
	if f.err != nil {
		return nil, f.err
	}
	return f.teamShares, nil
}

func (f *fakeCustomDashboardClient) AddCustomDashboardTeamShare(_ context.Context, id, teamID string) ([]entity.CustomDashboardTeamShare, error) {
	f.called = true
	f.gotID, f.gotTeamID = id, teamID
	if f.err != nil {
		return nil, f.err
	}
	return f.teamShares, nil
}

func (f *fakeCustomDashboardClient) RemoveCustomDashboardTeamShare(_ context.Context, id, teamID string) error {
	f.called = true
	f.gotID, f.gotTeamID = id, teamID
	return f.err
}

func (f *fakeCustomDashboardClient) GetCustomDashboard(_ context.Context, id string) (entity.CustomDashboard, error) {
	f.called = true
	f.gotID = id
	if f.err != nil {
		return entity.CustomDashboard{}, f.err
	}
	return f.stored, nil
}

func (f *fakeCustomDashboardClient) ListCustomDashboardShares(_ context.Context, id string) ([]entity.CustomDashboardShare, error) {
	f.called = true
	f.gotID = id
	if f.err != nil {
		return nil, f.err
	}
	return f.shares, nil
}

func (f *fakeCustomDashboardClient) AddCustomDashboardShare(_ context.Context, id, userID string) ([]entity.CustomDashboardShare, error) {
	f.called = true
	f.gotID, f.gotUserID = id, userID
	if f.err != nil {
		return nil, f.err
	}
	return f.shares, nil
}

func (f *fakeCustomDashboardClient) RemoveCustomDashboardShare(_ context.Context, id, userID string) error {
	f.called = true
	f.gotID, f.gotUserID = id, userID
	return f.err
}

func (f *fakeCustomDashboardClient) ListSharedCustomDashboards(context.Context) ([]entity.SharedCustomDashboardSummary, error) {
	f.called = true
	if f.err != nil {
		return nil, f.err
	}
	return f.sharedList, nil
}

func (f *fakeCustomDashboardClient) GetSharedCustomDashboard(_ context.Context, id string) (entity.SharedCustomDashboard, error) {
	f.called = true
	f.gotID = id
	if f.err != nil {
		return entity.SharedCustomDashboard{}, f.err
	}
	return f.shared, nil
}

func (f *fakeCustomDashboardClient) DeleteCustomDashboard(_ context.Context, id string) error {
	f.called = true
	f.gotID = id
	return f.err
}

func (f *fakeCustomDashboardClient) ListCustomDashboards(context.Context) ([]entity.CustomDashboardSummary, error) {
	f.called = true
	if f.err != nil {
		return nil, f.err
	}
	return f.list, nil
}

func (f *fakeCustomDashboardClient) SaveCustomDashboard(_ context.Context, id, name, definition string) (entity.CustomDashboard, error) {
	f.called = true
	f.gotID, f.gotName, f.gotDefinition = id, name, definition
	if f.err != nil {
		return entity.CustomDashboard{}, f.err
	}
	return entity.CustomDashboard{ID: id, Name: name, Definition: definition, CreatedOn: "2026-10-26T10:00:00Z", UpdatedOn: "2026-10-26T10:05:00Z"}, nil
}

// customDraft is authoredDraftDashboard with id, as the generate page holds
// it: in the served form.
func customDraft(t *testing.T, id string) (dashboard.Dashboard, json.RawMessage) {
	t.Helper()
	return servedDraft(t, strings.Replace(authoredDraftDashboard, `"ai-draft-round-trip"`, `"`+id+`"`, 1))
}

func putCustomDashboard(t *testing.T, client *fakeCustomDashboardClient, pathID string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /dashboards/custom/{id}", NewCustomDashboardHandler(client).SaveCustomDashboard)
	req := withUser(httptest.NewRequest(http.MethodPut, "/dashboards/custom/"+pathID, bytes.NewReader(body)))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func saveBody(t *testing.T, draft json.RawMessage) []byte {
	t.Helper()
	b, err := json.Marshal(saveCustomDashboardRequest{Dashboard: draft})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSaveCustomDashboard_StoresCheckedDefinition(t *testing.T) {
	_, draft := customDraft(t, testCustomDashboardID)
	client := &fakeCustomDashboardClient{}

	w := putCustomDashboard(t, client, testCustomDashboardID, saveBody(t, draft))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if client.gotID != testCustomDashboardID || client.gotName != "Draft Round Trip" {
		t.Errorf("saved (id %q, name %q)", client.gotID, client.gotName)
	}
	// What is stored is the file format: it loads back through the strict
	// check, and carries "id" (not "widgetId") on its widgets.
	stored, err := dashboard.ParseGenerated([]byte(client.gotDefinition))
	if err != nil {
		t.Fatalf("stored definition does not parse: %v\n%s", err, client.gotDefinition)
	}
	if len(stored.Widgets) != 7 || strings.Contains(client.gotDefinition, "widgetId") {
		t.Errorf("stored definition = %s", client.gotDefinition)
	}

	got := decodeJSON[customDashboardResponse](t, w)
	if got.Dashboard.ID != testCustomDashboardID || got.CreatedOn == "" || got.UpdatedOn == "" {
		t.Errorf("response = %+v", got)
	}
	// The saved dashboard comes back exactly as the draft was served.
	var sent dashboardDetailView
	if err := json.Unmarshal(draft, &sent); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Dashboard, sent) {
		t.Errorf("response dashboard differs from the draft\n got: %+v\nwant: %+v", got.Dashboard, sent)
	}
}

func TestSaveCustomDashboard_LowercasesID(t *testing.T) {
	upper := strings.ToUpper(testCustomDashboardID)
	_, draft := customDraft(t, upper)
	client := &fakeCustomDashboardClient{}

	w := putCustomDashboard(t, client, upper, saveBody(t, draft))

	if w.Code != http.StatusOK || client.gotID != testCustomDashboardID {
		t.Fatalf("status %d, saved id %q; want 200 and the lower-case id", w.Code, client.gotID)
	}
}

func TestSaveCustomDashboard_RequiresUser(t *testing.T) {
	_, draft := customDraft(t, testCustomDashboardID)
	client := &fakeCustomDashboardClient{}
	req := httptest.NewRequest(http.MethodPut, "/dashboards/custom/"+testCustomDashboardID, bytes.NewReader(saveBody(t, draft)))
	req.SetPathValue("id", testCustomDashboardID)
	w := httptest.NewRecorder()

	NewCustomDashboardHandler(client).SaveCustomDashboard(w, req)

	if w.Code != http.StatusUnauthorized || client.called {
		t.Fatalf("status %d, called %v; want 401 and no upstream call", w.Code, client.called)
	}
}

func TestSaveCustomDashboard_RejectsBeforeCallingEntity(t *testing.T) {
	_, draft := customDraft(t, testCustomDashboardID)
	_, otherDraft := customDraft(t, "5e636a12-0000-4000-8000-0000000000ff")

	withField := func(field, value string) json.RawMessage {
		var m map[string]any
		if err := json.Unmarshal(draft, &m); err != nil {
			t.Fatal(err)
		}
		var v any
		if err := json.Unmarshal([]byte(value), &v); err != nil {
			t.Fatal(err)
		}
		m[field] = v
		b, _ := json.Marshal(m)
		return b
	}
	bigWidgets := `[{"widgetId":"big","displayName":"Big","description":"` + strings.Repeat("x", customDashboardDefinitionMaxBytes) +
		`","resourceType":"case","shape":"count","gridWidth":3,"query":{}}]`

	cases := []struct {
		name   string
		pathID string
		body   []byte
		want   string
	}{
		{"non-UUID id", "abc", saveBody(t, draft), ErrMsgInvalidUUID},
		{"not JSON", testCustomDashboardID, []byte(`dashboard`), ErrMsgBadRequest},
		{"no dashboard", testCustomDashboardID, []byte(`{}`), errMsgCustomDashboardMissing},
		{"null dashboard", testCustomDashboardID, []byte(`{"dashboard":null}`), errMsgCustomDashboardMissing},
		{"another dashboard's id", testCustomDashboardID, saveBody(t, otherDraft), errMsgCustomDashboardIDMismatch},
		{"field the served form lacks", testCustomDashboardID, saveBody(t, withField("restricted", "true")), errMsgCustomDashboardInvalid},
		{"isDefault", testCustomDashboardID, saveBody(t, withField("isDefault", "true")), errMsgCustomDashboardInvalid},
		{"a resource type generation does not support", testCustomDashboardID, saveBody(t, withField("widgets",
			`[{"widgetId":"inc","displayName":"Incident Tasks","resourceType":"incident_task","shape":"count","gridWidth":3,"query":{}}]`)), errMsgCustomDashboardInvalid},
		{"blank name", testCustomDashboardID, saveBody(t, withField("displayName", `"   "`)), ""},
		{"name too long", testCustomDashboardID, saveBody(t, withField("displayName", `"`+strings.Repeat("é", customDashboardNameMaxLength+1)+`"`)), errMsgCustomDashboardNameTooLong},
		{"definition too large", testCustomDashboardID, saveBody(t, withField("widgets", bigWidgets)), errMsgCustomDashboardTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeCustomDashboardClient{}
			w := putCustomDashboard(t, client, tc.pathID, tc.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", w.Code, w.Body.String())
			}
			if client.called {
				t.Error("entity-service was called")
			}
			if tc.want != "" {
				if got := decodeJSON[errorBody](t, w); got.Message != tc.want {
					t.Errorf("message = %q, want %q", got.Message, tc.want)
				}
			}
		})
	}
}

func TestSaveCustomDashboard_UpstreamErrors(t *testing.T) {
	_, draft := customDraft(t, testCustomDashboardID)
	cases := []struct {
		name     string
		status   int
		body     string
		want     int
		wantText string
	}{
		{"name taken", http.StatusConflict, `{"code":409,"message":"you already have a dashboard named: Draft Round Trip"}`,
			http.StatusConflict, "you already have a dashboard named: Draft Round Trip"},
		{"limit reached", http.StatusBadRequest, `{"code":400,"message":"at most 50 saved dashboards are allowed"}`,
			http.StatusBadRequest, "at most 50 saved dashboards are allowed"},
		{"another user's id", http.StatusNotFound, `{"code":404,"message":"dashboard not found"}`,
			http.StatusNotFound, ErrMsgNotFound},
		{"entity-service failed", http.StatusInternalServerError, `{"code":500,"message":"pq: connection refused"}`,
			http.StatusInternalServerError, errMsgCustomDashboardSaveFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeCustomDashboardClient{err: &apierror.Error{StatusCode: tc.status, Body: tc.body}}
			w := putCustomDashboard(t, client, testCustomDashboardID, saveBody(t, draft))
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if got := decodeJSON[errorBody](t, w); got.Message != tc.wantText {
				t.Errorf("message = %q, want %q", got.Message, tc.wantText)
			}
		})
	}
}

func getCustomDashboards(client *fakeCustomDashboardClient) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	NewCustomDashboardHandler(client).ListCustomDashboards(w, withUser(httptest.NewRequest(http.MethodGet, "/dashboards/custom", nil)))
	return w
}

func TestListCustomDashboards(t *testing.T) {
	client := &fakeCustomDashboardClient{list: []entity.CustomDashboardSummary{
		{ID: testCustomDashboardID, Name: "My Cases", CreatedOn: "2026-10-26T10:00:00Z", UpdatedOn: "2026-10-26T10:05:00Z"},
	}}
	w := getCustomDashboards(client)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[customDashboardListResponse](t, w)
	want := []customDashboardSummary{{ID: testCustomDashboardID, Name: "My Cases", CreatedOn: "2026-10-26T10:00:00Z", UpdatedOn: "2026-10-26T10:05:00Z"}}
	if !reflect.DeepEqual(got.Dashboards, want) {
		t.Errorf("dashboards = %+v, want %+v", got.Dashboards, want)
	}
}

func TestListCustomDashboards_EmptyIsAnEmptyArray(t *testing.T) {
	w := getCustomDashboards(&fakeCustomDashboardClient{})
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"dashboards":[]}` {
		t.Fatalf("status %d, body %s; want 200 and an empty array", w.Code, w.Body.String())
	}
}

func TestListCustomDashboards_RequiresUser(t *testing.T) {
	client := &fakeCustomDashboardClient{}
	w := httptest.NewRecorder()
	NewCustomDashboardHandler(client).ListCustomDashboards(w, httptest.NewRequest(http.MethodGet, "/dashboards/custom", nil))
	if w.Code != http.StatusUnauthorized || client.called {
		t.Fatalf("status %d, called %v; want 401 and no upstream call", w.Code, client.called)
	}
}

func TestListCustomDashboards_UpstreamErrorsAreGeneric(t *testing.T) {
	cases := []struct {
		status int
		want   int
		msg    string
	}{
		{http.StatusUnauthorized, http.StatusUnauthorized, ErrMsgUnauthorized},
		{http.StatusNotFound, http.StatusNotFound, ErrMsgNotFound},
		{http.StatusBadRequest, http.StatusBadRequest, errMsgCustomDashboardListFailed},
		{http.StatusInternalServerError, http.StatusInternalServerError, errMsgCustomDashboardListFailed},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			client := &fakeCustomDashboardClient{err: &apierror.Error{StatusCode: tc.status, Body: `{"code":0,"message":"upstream detail"}`}}
			w := getCustomDashboards(client)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if got := decodeJSON[errorBody](t, w); got.Message != tc.msg {
				t.Errorf("message = %q, want %q", got.Message, tc.msg)
			}
		})
	}
}

func getCustomDashboard(client *fakeCustomDashboardClient, pathID string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /dashboards/custom/{id}", NewCustomDashboardHandler(client).GetCustomDashboard)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, withUser(httptest.NewRequest(http.MethodGet, "/dashboards/custom/"+pathID, nil)))
	return w
}

// storedDefinition is what a save of the served draft stores.
func storedDefinition(t *testing.T, draft json.RawMessage) string {
	t.Helper()
	def, err := draftToDefinition(draft)
	if err != nil {
		t.Fatal(err)
	}
	return string(def)
}

func TestGetCustomDashboard_ServesWhatWasSaved(t *testing.T) {
	_, draft := customDraft(t, testCustomDashboardID)
	client := &fakeCustomDashboardClient{stored: entity.CustomDashboard{
		ID: testCustomDashboardID, Name: "Draft Round Trip", Definition: storedDefinition(t, draft),
		CreatedOn: "2026-10-26T10:00:00Z", UpdatedOn: "2026-10-26T10:05:00Z",
	}}

	w := getCustomDashboard(client, testCustomDashboardID)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[customDashboardResponse](t, w)
	var sent dashboardDetailView
	if err := json.Unmarshal(draft, &sent); err != nil {
		t.Fatal(err)
	}
	// Saved, then read back: the same dashboard the draft was, implied type
	// filters included.
	if !reflect.DeepEqual(got.Dashboard, sent) {
		t.Errorf("dashboard differs from the saved draft\n got: %+v\nwant: %+v", got.Dashboard, sent)
	}
	if got.CreatedOn != "2026-10-26T10:00:00Z" || got.UpdatedOn != "2026-10-26T10:05:00Z" {
		t.Errorf("times = %q, %q", got.CreatedOn, got.UpdatedOn)
	}
}

func TestGetCustomDashboard_UsesTheRowsID(t *testing.T) {
	// The stored definition carries some other id; the row's id wins.
	_, draft := customDraft(t, "5e636a12-0000-4000-8000-0000000000ff")
	client := &fakeCustomDashboardClient{stored: entity.CustomDashboard{ID: testCustomDashboardID, Definition: storedDefinition(t, draft)}}

	w := getCustomDashboard(client, strings.ToUpper(testCustomDashboardID))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if client.gotID != testCustomDashboardID {
		t.Errorf("asked entity-service for %q, want the lower-case id", client.gotID)
	}
	if got := decodeJSON[customDashboardResponse](t, w); got.Dashboard.ID != testCustomDashboardID {
		t.Errorf("dashboard id = %q, want the row's", got.Dashboard.ID)
	}
}

func TestGetCustomDashboard_RejectsBadIDAndNoUser(t *testing.T) {
	client := &fakeCustomDashboardClient{}
	if w := getCustomDashboard(client, "abc"); w.Code != http.StatusBadRequest || client.called {
		t.Errorf("non-UUID: status %d, called %v; want 400 and no upstream call", w.Code, client.called)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboards/custom/"+testCustomDashboardID, nil)
	req.SetPathValue("id", testCustomDashboardID)
	NewCustomDashboardHandler(client).GetCustomDashboard(w, req)
	if w.Code != http.StatusUnauthorized || client.called {
		t.Errorf("no user: status %d, called %v; want 401 and no upstream call", w.Code, client.called)
	}
}

func TestGetCustomDashboard_OutdatedDefinitionIs422(t *testing.T) {
	stored := `{"id":"x","displayName":"Old","type":"cs","widgets":[{"id":"inc","displayName":"Incident Tasks","resourceType":"incident_task","shape":"count","gridWidth":3,"query":{}}]}`
	client := &fakeCustomDashboardClient{stored: entity.CustomDashboard{ID: testCustomDashboardID, Definition: stored}}

	w := getCustomDashboard(client, testCustomDashboardID)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", w.Code, w.Body.String())
	}
	if got := decodeJSON[errorBody](t, w); got.Message != errMsgCustomDashboardOutdated {
		t.Errorf("message = %q, want the fixed message (no validation detail)", got.Message)
	}
}

func TestGetCustomDashboard_UpstreamErrorsAreGeneric(t *testing.T) {
	cases := []struct {
		status int
		want   int
		msg    string
	}{
		{http.StatusNotFound, http.StatusNotFound, ErrMsgNotFound},
		{http.StatusBadRequest, http.StatusBadRequest, errMsgCustomDashboardLoadFailed},
		{http.StatusInternalServerError, http.StatusInternalServerError, errMsgCustomDashboardLoadFailed},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			client := &fakeCustomDashboardClient{err: &apierror.Error{StatusCode: tc.status, Body: `{"code":0,"message":"upstream detail"}`}}
			w := getCustomDashboard(client, testCustomDashboardID)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if got := decodeJSON[errorBody](t, w); got.Message != tc.msg {
				t.Errorf("message = %q, want %q", got.Message, tc.msg)
			}
		})
	}
}

func deleteCustomDashboard(client *fakeCustomDashboardClient, pathID string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /dashboards/custom/{id}", NewCustomDashboardHandler(client).DeleteCustomDashboard)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, withUser(httptest.NewRequest(http.MethodDelete, "/dashboards/custom/"+pathID, nil)))
	return w
}

func TestDeleteCustomDashboard(t *testing.T) {
	client := &fakeCustomDashboardClient{}
	w := deleteCustomDashboard(client, strings.ToUpper(testCustomDashboardID))
	if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
		t.Fatalf("status %d, body %q; want 204 and no body", w.Code, w.Body.String())
	}
	if client.gotID != testCustomDashboardID {
		t.Errorf("deleted %q, want the lower-case id", client.gotID)
	}
}

func TestDeleteCustomDashboard_RejectsBadIDAndNoUser(t *testing.T) {
	client := &fakeCustomDashboardClient{}
	if w := deleteCustomDashboard(client, "abc"); w.Code != http.StatusBadRequest || client.called {
		t.Errorf("non-UUID: status %d, called %v; want 400 and no upstream call", w.Code, client.called)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/dashboards/custom/"+testCustomDashboardID, nil)
	req.SetPathValue("id", testCustomDashboardID)
	NewCustomDashboardHandler(client).DeleteCustomDashboard(w, req)
	if w.Code != http.StatusUnauthorized || client.called {
		t.Errorf("no user: status %d, called %v; want 401 and no upstream call", w.Code, client.called)
	}
}

func TestDeleteCustomDashboard_UpstreamErrorsAreGeneric(t *testing.T) {
	cases := []struct {
		status int
		want   int
		msg    string
	}{
		{http.StatusNotFound, http.StatusNotFound, ErrMsgNotFound},
		{http.StatusBadRequest, http.StatusBadRequest, errMsgCustomDashboardDeleteFailed},
		{http.StatusInternalServerError, http.StatusInternalServerError, errMsgCustomDashboardDeleteFailed},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			client := &fakeCustomDashboardClient{err: &apierror.Error{StatusCode: tc.status, Body: `{"code":0,"message":"upstream detail"}`}}
			w := deleteCustomDashboard(client, testCustomDashboardID)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if got := decodeJSON[errorBody](t, w); got.Message != tc.msg {
				t.Errorf("message = %q, want %q", got.Message, tc.msg)
			}
		})
	}
}

const testShareUserID = "00000000-0000-0000-0000-000000000011"

func serveShareRoute(client *fakeCustomDashboardClient, method, path string) *httptest.ResponseRecorder {
	h := NewCustomDashboardHandler(client)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /dashboards/custom/{id}/shares", h.ListCustomDashboardShares)
	mux.HandleFunc("PUT /dashboards/custom/{id}/shares/{userId}", h.AddCustomDashboardShare)
	mux.HandleFunc("DELETE /dashboards/custom/{id}/shares/{userId}", h.RemoveCustomDashboardShare)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, withUser(httptest.NewRequest(method, path, nil)))
	return w
}

func aliceShare() []entity.CustomDashboardShare {
	id := testShareUserID
	return []entity.CustomDashboardShare{{User: entity.CustomDashboardPerson{ID: &id, Email: "alice@example.com", Name: "Alice Perera"}, SharedOn: "2026-10-27T04:21:07Z"}}
}

func TestListCustomDashboardShares(t *testing.T) {
	client := &fakeCustomDashboardClient{shares: aliceShare()}
	w := serveShareRoute(client, http.MethodGet, "/dashboards/custom/"+strings.ToUpper(testCustomDashboardID)+"/shares")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if client.gotID != testCustomDashboardID {
		t.Errorf("id = %q, want the lower-case id", client.gotID)
	}
	got := decodeJSON[customDashboardShareListResponse](t, w)
	if len(got.Shares) != 1 || got.Shares[0].User.Name != "Alice Perera" || got.Shares[0].SharedOn == "" {
		t.Fatalf("body = %+v", got)
	}
}

func TestListCustomDashboardShares_EmptyIsAnEmptyArray(t *testing.T) {
	w := serveShareRoute(&fakeCustomDashboardClient{shares: []entity.CustomDashboardShare{}}, http.MethodGet, "/dashboards/custom/"+testCustomDashboardID+"/shares")
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"shares":[]}` {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
}

func TestAddCustomDashboardShare(t *testing.T) {
	client := &fakeCustomDashboardClient{shares: aliceShare()}
	w := serveShareRoute(client, http.MethodPut, "/dashboards/custom/"+testCustomDashboardID+"/shares/"+strings.ToUpper(testShareUserID))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if client.gotID != testCustomDashboardID || client.gotUserID != testShareUserID {
		t.Errorf("shared (%q, %q), want lower-case ids", client.gotID, client.gotUserID)
	}
	if got := decodeJSON[customDashboardShareListResponse](t, w); len(got.Shares) != 1 {
		t.Fatalf("body = %+v", got)
	}
}

func TestRemoveCustomDashboardShare(t *testing.T) {
	client := &fakeCustomDashboardClient{}
	w := serveShareRoute(client, http.MethodDelete, "/dashboards/custom/"+testCustomDashboardID+"/shares/"+testShareUserID)
	if w.Code != http.StatusNoContent || w.Body.Len() != 0 || client.gotUserID != testShareUserID {
		t.Fatalf("status %d, body %q, user %q", w.Code, w.Body.String(), client.gotUserID)
	}
}

func TestCustomDashboardShares_RejectBadIDsAndNoUser(t *testing.T) {
	bad := []struct{ method, path string }{
		{http.MethodGet, "/dashboards/custom/abc/shares"},
		{http.MethodPut, "/dashboards/custom/abc/shares/" + testShareUserID},
		{http.MethodPut, "/dashboards/custom/" + testCustomDashboardID + "/shares/abc"},
		{http.MethodDelete, "/dashboards/custom/" + testCustomDashboardID + "/shares/abc"},
	}
	for _, tc := range bad {
		client := &fakeCustomDashboardClient{}
		if w := serveShareRoute(client, tc.method, tc.path); w.Code != http.StatusBadRequest || client.called {
			t.Errorf("%s %s: status %d, called %v; want 400 and no upstream call", tc.method, tc.path, w.Code, client.called)
		}
	}

	handlers := map[string]func(*CustomDashboardHandler) http.HandlerFunc{
		"list":   func(h *CustomDashboardHandler) http.HandlerFunc { return h.ListCustomDashboardShares },
		"add":    func(h *CustomDashboardHandler) http.HandlerFunc { return h.AddCustomDashboardShare },
		"remove": func(h *CustomDashboardHandler) http.HandlerFunc { return h.RemoveCustomDashboardShare },
	}
	for name, pick := range handlers {
		client := &fakeCustomDashboardClient{}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetPathValue("id", testCustomDashboardID)
		req.SetPathValue("userId", testShareUserID)
		w := httptest.NewRecorder()
		pick(NewCustomDashboardHandler(client))(w, req)
		if w.Code != http.StatusUnauthorized || client.called {
			t.Errorf("%s without a user: status %d, called %v; want 401 and no upstream call", name, w.Code, client.called)
		}
	}
}

func TestAddCustomDashboardShare_PassesRefusalsOn(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		want     int
		wantText string
	}{
		{"yourself", http.StatusBadRequest, `{"code":400,"message":"you cannot share a dashboard with yourself"}`, http.StatusBadRequest, "you cannot share a dashboard with yourself"},
		{"not internal staff", http.StatusBadRequest, `{"code":400,"message":"dashboards can only be shared with internal staff"}`, http.StatusBadRequest, "dashboards can only be shared with internal staff"},
		{"unknown user or dashboard", http.StatusNotFound, `{"code":404,"message":"user not found"}`, http.StatusNotFound, ErrMsgNotFound},
		{"entity-service failed", http.StatusInternalServerError, `{"code":500,"message":"pq: boom"}`, http.StatusInternalServerError, errMsgCustomDashboardShareFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeCustomDashboardClient{err: &apierror.Error{StatusCode: tc.status, Body: tc.body}}
			w := serveShareRoute(client, http.MethodPut, "/dashboards/custom/"+testCustomDashboardID+"/shares/"+testShareUserID)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if got := decodeJSON[errorBody](t, w); got.Message != tc.wantText {
				t.Errorf("message = %q, want %q", got.Message, tc.wantText)
			}
		})
	}
}

func TestListAndRemoveShares_UpstreamErrorsAreGeneric(t *testing.T) {
	cases := []struct {
		name, method, path string
		status, want       int
		msg                string
	}{
		{"list not the owner's", http.MethodGet, "/dashboards/custom/" + testCustomDashboardID + "/shares", http.StatusNotFound, http.StatusNotFound, ErrMsgNotFound},
		{"list failed", http.MethodGet, "/dashboards/custom/" + testCustomDashboardID + "/shares", http.StatusInternalServerError, http.StatusInternalServerError, errMsgCustomDashboardSharesFailed},
		{"remove not shared", http.MethodDelete, "/dashboards/custom/" + testCustomDashboardID + "/shares/" + testShareUserID, http.StatusNotFound, http.StatusNotFound, ErrMsgNotFound},
		{"remove failed", http.MethodDelete, "/dashboards/custom/" + testCustomDashboardID + "/shares/" + testShareUserID, http.StatusBadRequest, http.StatusBadRequest, errMsgCustomDashboardUnshareFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeCustomDashboardClient{err: &apierror.Error{StatusCode: tc.status, Body: `{"code":0,"message":"upstream detail"}`}}
			w := serveShareRoute(client, tc.method, tc.path)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if got := decodeJSON[errorBody](t, w); got.Message != tc.msg {
				t.Errorf("message = %q, want %q", got.Message, tc.msg)
			}
		})
	}
}

func serveSharedRoute(client *fakeCustomDashboardClient, path string) *httptest.ResponseRecorder {
	h := NewCustomDashboardHandler(client)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /dashboards/shared", h.ListSharedDashboards)
	mux.HandleFunc("GET /dashboards/shared/{id}", h.GetSharedDashboard)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, withUser(httptest.NewRequest(http.MethodGet, path, nil)))
	return w
}

func janeOwner() *entity.CustomDashboardPerson {
	id := "00000000-0000-0000-0000-000000000001"
	return &entity.CustomDashboardPerson{ID: &id, Email: "jane.doe@example.com", Name: "Jane Doe"}
}

func TestListSharedDashboards(t *testing.T) {
	client := &fakeCustomDashboardClient{sharedList: []entity.SharedCustomDashboardSummary{
		{ID: testCustomDashboardID, Name: "Live Save Test", Owner: janeOwner(), SharedOn: "a", UpdatedOn: "b"},
	}}
	w := serveSharedRoute(client, "/dashboards/shared")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[sharedDashboardListResponse](t, w)
	if len(got.Dashboards) != 1 || got.Dashboards[0].Owner.Name != "Jane Doe" || got.Dashboards[0].UpdatedOn != "b" {
		t.Fatalf("body = %+v", got)
	}
	if strings.Contains(w.Body.String(), "definition") {
		t.Errorf("list carries a definition: %s", w.Body.String())
	}
}

// A dashboard whose owner's user was deleted reaches the webapp with
// "owner": null, in the list and the detail.
func TestSharedDashboards_OwnerDeletedIsNull(t *testing.T) {
	list := serveSharedRoute(&fakeCustomDashboardClient{sharedList: []entity.SharedCustomDashboardSummary{
		{ID: testCustomDashboardID, Name: "Orphan", SharedOn: "a", UpdatedOn: "b"},
	}}, "/dashboards/shared")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"owner":null`) {
		t.Errorf("list: status %d, body %s; want \"owner\":null", list.Code, list.Body.String())
	}

	_, draft := customDraft(t, testCustomDashboardID)
	get := serveSharedRoute(&fakeCustomDashboardClient{shared: entity.SharedCustomDashboard{
		ID: testCustomDashboardID, Name: "Orphan", Definition: storedDefinition(t, draft), SharedOn: "a", UpdatedOn: "b",
	}}, "/dashboards/shared/"+testCustomDashboardID)
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `"owner":null`) {
		t.Errorf("get: status %d, body %s; want \"owner\":null", get.Code, get.Body.String())
	}
}

func TestListSharedDashboards_EmptyIsAnEmptyArray(t *testing.T) {
	w := serveSharedRoute(&fakeCustomDashboardClient{}, "/dashboards/shared")
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"dashboards":[]}` {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
}

func TestGetSharedDashboard_ServesTheOwnersVersion(t *testing.T) {
	_, draft := customDraft(t, testCustomDashboardID)
	client := &fakeCustomDashboardClient{shared: entity.SharedCustomDashboard{
		ID: testCustomDashboardID, Name: "Draft Round Trip", Definition: storedDefinition(t, draft),
		Owner: janeOwner(), SharedOn: "2026-10-27T04:00:00Z", UpdatedOn: "2026-10-27T05:00:00Z",
	}}

	w := serveSharedRoute(client, "/dashboards/shared/"+strings.ToUpper(testCustomDashboardID))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if client.gotID != testCustomDashboardID {
		t.Errorf("asked for %q, want the lower-case id", client.gotID)
	}
	got := decodeJSON[sharedDashboardResponse](t, w)
	var sent dashboardDetailView
	if err := json.Unmarshal(draft, &sent); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Dashboard, sent) {
		t.Errorf("dashboard differs from what the owner saved\n got: %+v\nwant: %+v", got.Dashboard, sent)
	}
	if got.Owner.Name != "Jane Doe" || got.SharedOn != "2026-10-27T04:00:00Z" || got.UpdatedOn != "2026-10-27T05:00:00Z" {
		t.Errorf("owner and times = %+v, %q, %q", got.Owner, got.SharedOn, got.UpdatedOn)
	}
}

func TestGetSharedDashboard_OutdatedAsksForTheOwner(t *testing.T) {
	stored := `{"id":"x","displayName":"Old","type":"cs","widgets":[{"id":"inc","displayName":"Incident Tasks","resourceType":"incident_task","shape":"count","gridWidth":3,"query":{}}]}`
	client := &fakeCustomDashboardClient{shared: entity.SharedCustomDashboard{ID: testCustomDashboardID, Definition: stored, Owner: janeOwner()}}
	w := serveSharedRoute(client, "/dashboards/shared/"+testCustomDashboardID)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", w.Code, w.Body.String())
	}
	if got := decodeJSON[errorBody](t, w); got.Message != errMsgSharedDashboardOutdated {
		t.Errorf("message = %q", got.Message)
	}
}

func TestGetSharedDashboard_RejectsBadIDAndNoUser(t *testing.T) {
	client := &fakeCustomDashboardClient{}
	if w := serveSharedRoute(client, "/dashboards/shared/abc"); w.Code != http.StatusBadRequest || client.called {
		t.Errorf("non-UUID: status %d, called %v", w.Code, client.called)
	}
	for name, h := range map[string]http.HandlerFunc{
		"list": NewCustomDashboardHandler(client).ListSharedDashboards,
		"get":  NewCustomDashboardHandler(client).GetSharedDashboard,
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetPathValue("id", testCustomDashboardID)
		w := httptest.NewRecorder()
		h(w, req)
		if w.Code != http.StatusUnauthorized || client.called {
			t.Errorf("%s without a user: status %d, called %v", name, w.Code, client.called)
		}
	}
}

func TestSharedDashboards_UpstreamErrorsAreGeneric(t *testing.T) {
	cases := []struct {
		name, path   string
		status, want int
		msg          string
	}{
		{"get not shared with me", "/dashboards/shared/" + testCustomDashboardID, http.StatusNotFound, http.StatusNotFound, ErrMsgNotFound},
		{"get failed", "/dashboards/shared/" + testCustomDashboardID, http.StatusInternalServerError, http.StatusInternalServerError, errMsgCustomDashboardLoadFailed},
		{"list failed", "/dashboards/shared", http.StatusInternalServerError, http.StatusInternalServerError, errMsgSharedDashboardListFailed},
		{"list unauthorized", "/dashboards/shared", http.StatusUnauthorized, http.StatusUnauthorized, ErrMsgUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeCustomDashboardClient{err: &apierror.Error{StatusCode: tc.status, Body: `{"code":0,"message":"upstream detail"}`}}
			w := serveSharedRoute(client, tc.path)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if got := decodeJSON[errorBody](t, w); got.Message != tc.msg {
				t.Errorf("message = %q, want %q", got.Message, tc.msg)
			}
		})
	}
}

// Team shares: the registry (testTeamShareRegistry) gives rigel a CRE group,
// apollo an SRE group, and orion none (so it cannot be shared with).
const (
	testTeamShareRigelID  = "391737f4-0588-2f9f-9bdd-b18d39f83d9a"
	testTeamShareApolloID = "f9e9df56-1215-e316-c6f2-ce9180c0af64"
)

func testTeamShareRegistry() []directory.TeamResult {
	return []directory.TeamResult{
		{ID: "apollo", Name: "Apollo", Family: "sre-abt", SreGroupID: testTeamShareApolloID},
		{ID: "orion", Name: "Orion", Family: "cre-abt"},
		{ID: "rigel", Name: "Rigel", Family: "cre-abt", CreGroupID: strings.ToUpper(testTeamShareRigelID)},
	}
}

func serveTeamShareRoute(client *fakeCustomDashboardClient, method, path string) *httptest.ResponseRecorder {
	h := NewCustomDashboardHandler(client).WithTeams(testTeamShareRegistry())
	mux := http.NewServeMux()
	mux.HandleFunc("GET /dashboards/custom/{id}/team-shares", h.ListCustomDashboardTeamShares)
	mux.HandleFunc("PUT /dashboards/custom/{id}/team-shares/{teamKey}", h.AddCustomDashboardTeamShare)
	mux.HandleFunc("DELETE /dashboards/custom/{id}/team-shares/{teamKey}", h.RemoveCustomDashboardTeamShare)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, withUser(httptest.NewRequest(method, path, nil)))
	return w
}

func rigelTeamShare() []entity.CustomDashboardTeamShare {
	return []entity.CustomDashboardTeamShare{{Team: entity.CustomDashboardTeam{ID: testTeamShareRigelID, Name: "Rigel", Type: "cre-abt"}, SharedOn: "2026-10-27T11:23:23Z"}}
}

func TestListCustomDashboardTeamShares_NamesTeamsByKey(t *testing.T) {
	client := &fakeCustomDashboardClient{teamShares: append(rigelTeamShare(),
		// No longer in the registry: listed by name, with no key.
		entity.CustomDashboardTeamShare{Team: entity.CustomDashboardTeam{ID: "00000000-0000-4000-8000-0000000000ee", Name: "Old Team", Type: "cre"}, SharedOn: "x"})}
	w := serveTeamShareRoute(client, http.MethodGet, "/dashboards/custom/"+strings.ToUpper(testCustomDashboardID)+"/team-shares")
	if w.Code != http.StatusOK || client.gotID != testCustomDashboardID {
		t.Fatalf("status %d, id %q: %s", w.Code, client.gotID, w.Body.String())
	}
	got := decodeJSON[customDashboardTeamShareListResponse](t, w)
	want := []customDashboardTeamShareTeam{{Key: "rigel", Name: "Rigel", Family: "cre-abt"}, {Key: "", Name: "Old Team", Family: "cre"}}
	if len(got.Shares) != 2 || got.Shares[0].Team != want[0] || got.Shares[1].Team != want[1] || got.Shares[0].SharedOn == "" {
		t.Fatalf("shares = %+v, want %+v", got.Shares, want)
	}
}

func TestListCustomDashboardTeamShares_EmptyIsAnEmptyArray(t *testing.T) {
	w := serveTeamShareRoute(&fakeCustomDashboardClient{}, http.MethodGet, "/dashboards/custom/"+testCustomDashboardID+"/team-shares")
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"shares":[]}` {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
}

func TestAddCustomDashboardTeamShare_SendsTheTeamsGroupID(t *testing.T) {
	cases := []struct{ key, wantID string }{
		{"rigel", testTeamShareRigelID},   // CRE group, lower-cased
		{"RIGEL", testTeamShareRigelID},   // key in any case
		{"apollo", testTeamShareApolloID}, // SRE group
	}
	for _, tc := range cases {
		client := &fakeCustomDashboardClient{teamShares: rigelTeamShare()}
		w := serveTeamShareRoute(client, http.MethodPut, "/dashboards/custom/"+testCustomDashboardID+"/team-shares/"+tc.key)
		if w.Code != http.StatusOK || client.gotTeamID != tc.wantID || client.gotID != testCustomDashboardID {
			t.Errorf("%s: status %d, team id %q (want %q): %s", tc.key, w.Code, client.gotTeamID, tc.wantID, w.Body.String())
		}
		if got := decodeJSON[customDashboardTeamShareListResponse](t, w); len(got.Shares) != 1 || got.Shares[0].Team.Key != "rigel" {
			t.Errorf("%s: body = %+v", tc.key, got)
		}
	}
}

func TestRemoveCustomDashboardTeamShare(t *testing.T) {
	client := &fakeCustomDashboardClient{}
	w := serveTeamShareRoute(client, http.MethodDelete, "/dashboards/custom/"+testCustomDashboardID+"/team-shares/apollo")
	if w.Code != http.StatusNoContent || w.Body.Len() != 0 || client.gotTeamID != testTeamShareApolloID {
		t.Fatalf("status %d, body %q, team %q", w.Code, w.Body.String(), client.gotTeamID)
	}
}

func TestCustomDashboardTeamShares_RefusedWithoutAnUpstreamCall(t *testing.T) {
	cases := []struct {
		name, method, path string
		want               int
		msg                string
	}{
		{"list, bad id", http.MethodGet, "/dashboards/custom/abc/team-shares", http.StatusBadRequest, ErrMsgInvalidUUID},
		{"add, bad id", http.MethodPut, "/dashboards/custom/abc/team-shares/rigel", http.StatusBadRequest, ErrMsgInvalidUUID},
		{"add, unknown team", http.MethodPut, "/dashboards/custom/" + testCustomDashboardID + "/team-shares/vega", http.StatusNotFound, errMsgTeamShareUnknownTeam},
		{"add, team with no group", http.MethodPut, "/dashboards/custom/" + testCustomDashboardID + "/team-shares/orion", http.StatusNotFound, errMsgTeamShareUnknownTeam},
		{"add, a team id instead of a key", http.MethodPut, "/dashboards/custom/" + testCustomDashboardID + "/team-shares/" + testTeamShareRigelID, http.StatusNotFound, errMsgTeamShareUnknownTeam},
		{"remove, unknown team", http.MethodDelete, "/dashboards/custom/" + testCustomDashboardID + "/team-shares/vega", http.StatusNotFound, errMsgTeamShareUnknownTeam},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeCustomDashboardClient{}
			w := serveTeamShareRoute(client, tc.method, tc.path)
			if w.Code != tc.want || client.called {
				t.Fatalf("status %d, called %v; want %d and no upstream call", w.Code, client.called, tc.want)
			}
			if got := decodeJSON[errorBody](t, w); got.Message != tc.msg {
				t.Errorf("message = %q, want %q", got.Message, tc.msg)
			}
		})
	}

	handlers := map[string]func(*CustomDashboardHandler) http.HandlerFunc{
		"list":   func(h *CustomDashboardHandler) http.HandlerFunc { return h.ListCustomDashboardTeamShares },
		"add":    func(h *CustomDashboardHandler) http.HandlerFunc { return h.AddCustomDashboardTeamShare },
		"remove": func(h *CustomDashboardHandler) http.HandlerFunc { return h.RemoveCustomDashboardTeamShare },
	}
	for name, pick := range handlers {
		client := &fakeCustomDashboardClient{}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetPathValue("id", testCustomDashboardID)
		req.SetPathValue("teamKey", "rigel")
		w := httptest.NewRecorder()
		pick(NewCustomDashboardHandler(client).WithTeams(testTeamShareRegistry()))(w, req)
		if w.Code != http.StatusUnauthorized || client.called {
			t.Errorf("%s without a user: status %d, called %v; want 401 and no upstream call", name, w.Code, client.called)
		}
	}
}

func TestCustomDashboardTeamShares_UpstreamErrors(t *testing.T) {
	cases := []struct {
		name, method, path string
		status, want       int
		msg                string
	}{
		{"list not the owner's", http.MethodGet, "/dashboards/custom/" + testCustomDashboardID + "/team-shares", http.StatusNotFound, http.StatusNotFound, ErrMsgNotFound},
		{"list failed", http.MethodGet, "/dashboards/custom/" + testCustomDashboardID + "/team-shares", http.StatusInternalServerError, http.StatusInternalServerError, errMsgTeamSharesFailed},
		{"add, dashboard gone", http.MethodPut, "/dashboards/custom/" + testCustomDashboardID + "/team-shares/rigel", http.StatusNotFound, http.StatusNotFound, ErrMsgNotFound},
		{"add failed", http.MethodPut, "/dashboards/custom/" + testCustomDashboardID + "/team-shares/rigel", http.StatusInternalServerError, http.StatusInternalServerError, errMsgTeamShareFailed},
		{"remove not shared", http.MethodDelete, "/dashboards/custom/" + testCustomDashboardID + "/team-shares/rigel", http.StatusNotFound, http.StatusNotFound, ErrMsgNotFound},
		{"remove failed", http.MethodDelete, "/dashboards/custom/" + testCustomDashboardID + "/team-shares/rigel", http.StatusInternalServerError, http.StatusInternalServerError, errMsgTeamUnshareFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeCustomDashboardClient{err: &apierror.Error{StatusCode: tc.status, Body: `{"code":0,"message":"upstream detail"}`}}
			w := serveTeamShareRoute(client, tc.method, tc.path)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if got := decodeJSON[errorBody](t, w); got.Message != tc.msg {
				t.Errorf("message = %q, want %q", got.Message, tc.msg)
			}
		})
	}
}

func TestSharedDashboards_PassTheTeamOn(t *testing.T) {
	rigel := &entity.CustomDashboardTeam{ID: testTeamShareRigelID, Name: "Rigel", Type: "cre-abt"}
	client := &fakeCustomDashboardClient{sharedList: []entity.SharedCustomDashboardSummary{
		{ID: testCustomDashboardID, Name: "Live Save Test", Owner: janeOwner(), SharedViaTeam: rigel, SharedOn: "a", UpdatedOn: "b"},
	}}
	w := serveSharedRoute(client, "/dashboards/shared")
	if got := decodeJSON[sharedDashboardListResponse](t, w); w.Code != http.StatusOK || len(got.Dashboards) != 1 || got.Dashboards[0].SharedViaTeam == nil || got.Dashboards[0].SharedViaTeam.Name != "Rigel" {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	// A direct share says so with null, not by leaving the field out.
	client.sharedList[0].SharedViaTeam = nil
	if w := serveSharedRoute(client, "/dashboards/shared"); !strings.Contains(w.Body.String(), `"sharedViaTeam":null`) {
		t.Fatalf("body = %s, want sharedViaTeam null", w.Body.String())
	}
}
