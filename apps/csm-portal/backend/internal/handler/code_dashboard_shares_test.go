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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
)

// fakeCodeShareClient records the last call and its ids, and answers with
// what it is given.
type fakeCodeShareClient struct {
	called, gotID, gotOther string

	shares     []entity.CustomDashboardShare
	teamShares []entity.CustomDashboardTeamShare
	shared     []entity.SharedCustomDashboardSummary
	sharedOne  entity.SharedCodeDashboard
	err        error
}

func (f *fakeCodeShareClient) rec(call, id, other string) {
	f.called, f.gotID, f.gotOther = call, id, other
}
func (f *fakeCodeShareClient) ListCodeDashboardShares(_ context.Context, id string) ([]entity.CustomDashboardShare, error) {
	f.rec("ListCodeDashboardShares", id, "")
	return f.shares, f.err
}
func (f *fakeCodeShareClient) AddCodeDashboardShare(_ context.Context, id, userID string) ([]entity.CustomDashboardShare, error) {
	f.rec("AddCodeDashboardShare", id, userID)
	return f.shares, f.err
}
func (f *fakeCodeShareClient) RemoveCodeDashboardShare(_ context.Context, id, userID string) error {
	f.rec("RemoveCodeDashboardShare", id, userID)
	return f.err
}
func (f *fakeCodeShareClient) ListCodeDashboardTeamShares(_ context.Context, id string) ([]entity.CustomDashboardTeamShare, error) {
	f.rec("ListCodeDashboardTeamShares", id, "")
	return f.teamShares, f.err
}
func (f *fakeCodeShareClient) AddCodeDashboardTeamShare(_ context.Context, id, teamID string) ([]entity.CustomDashboardTeamShare, error) {
	f.rec("AddCodeDashboardTeamShare", id, teamID)
	return f.teamShares, f.err
}
func (f *fakeCodeShareClient) RemoveCodeDashboardTeamShare(_ context.Context, id, teamID string) error {
	f.rec("RemoveCodeDashboardTeamShare", id, teamID)
	return f.err
}
func (f *fakeCodeShareClient) GetCodeDashboardLink(_ context.Context, id string) (entity.CustomDashboardLink, error) {
	f.rec("GetCodeDashboardLink", id, "")
	return entity.CustomDashboardLink{}, f.err
}
func (f *fakeCodeShareClient) EnableCodeDashboardLink(_ context.Context, id string) (entity.CustomDashboardLink, error) {
	f.rec("EnableCodeDashboardLink", id, "")
	return entity.CustomDashboardLink{Enabled: true, LinkID: "l1"}, f.err
}
func (f *fakeCodeShareClient) DisableCodeDashboardLink(_ context.Context, id string) error {
	f.rec("DisableCodeDashboardLink", id, "")
	return f.err
}
func (f *fakeCodeShareClient) OpenCodeDashboardLink(_ context.Context, linkID string) (entity.OpenedCustomDashboardLink, error) {
	f.rec("OpenCodeDashboardLink", "", linkID)
	return entity.OpenedCustomDashboardLink{DashboardID: testCustomDashboardID}, f.err
}
func (f *fakeCodeShareClient) ListSharedCodeDashboards(context.Context) ([]entity.SharedCustomDashboardSummary, error) {
	f.rec("ListSharedCodeDashboards", "", "")
	return f.shared, f.err
}
func (f *fakeCodeShareClient) GetSharedCodeDashboard(_ context.Context, id string) (entity.SharedCodeDashboard, error) {
	f.rec("GetSharedCodeDashboard", id, "")
	return f.sharedOne, f.err
}

// serveCodeShares registers the handler with main.go's patterns and the
// custom dashboard tests' team registry.
func serveCodeShares(client *fakeCodeShareClient, method, path, body string, signedIn bool) *httptest.ResponseRecorder {
	h := NewCodeDashboardShareHandler(client, testTeamShareRegistry())
	mux := http.NewServeMux()
	mux.HandleFunc("GET /dashboards/code/{id}/shares", h.ListCodeDashboardShares)
	mux.HandleFunc("PUT /dashboards/code/{id}/shares/{userId}", h.AddCodeDashboardShare)
	mux.HandleFunc("DELETE /dashboards/code/{id}/shares/{userId}", h.RemoveCodeDashboardShare)
	mux.HandleFunc("GET /dashboards/code/{id}/team-shares", h.ListCodeDashboardTeamShares)
	mux.HandleFunc("PUT /dashboards/code/{id}/team-shares/{teamKey}", h.AddCodeDashboardTeamShare)
	mux.HandleFunc("DELETE /dashboards/code/{id}/team-shares/{teamKey}", h.RemoveCodeDashboardTeamShare)
	mux.HandleFunc("GET /dashboards/code/{id}/link", h.GetCodeDashboardLink)
	mux.HandleFunc("PUT /dashboards/code/{id}/link", h.EnableCodeDashboardLink)
	mux.HandleFunc("DELETE /dashboards/code/{id}/link", h.DisableCodeDashboardLink)
	mux.HandleFunc("GET /dashboards/shared/code", h.ListSharedCodeDashboards)
	mux.HandleFunc("GET /dashboards/shared/code/{id}", h.GetSharedCodeDashboard)
	mux.HandleFunc("POST /dashboards/shared/code/link", h.OpenCodeDashboardLink)
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if signedIn {
		r = withUser(r)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

const testCodeShareUser = "5e636a12-0000-4000-8000-0000000000aa"

// codeShareRoutes is every route, with the call it makes and the second id it
// passes (the user, or the team's entity-service id for a registry key).
var codeShareRoutes = []struct {
	method, path, body, wantCall, wantOther string
	wantStatus                              int
}{
	{http.MethodGet, "/dashboards/code/" + testCustomDashboardID + "/shares", "", "ListCodeDashboardShares", "", http.StatusOK},
	{http.MethodPut, "/dashboards/code/" + strings.ToUpper(testCustomDashboardID) + "/shares/" + strings.ToUpper(testCodeShareUser), "", "AddCodeDashboardShare", testCodeShareUser, http.StatusOK},
	{http.MethodDelete, "/dashboards/code/" + testCustomDashboardID + "/shares/" + testCodeShareUser, "", "RemoveCodeDashboardShare", testCodeShareUser, http.StatusNoContent},
	{http.MethodGet, "/dashboards/code/" + testCustomDashboardID + "/team-shares", "", "ListCodeDashboardTeamShares", "", http.StatusOK},
	{http.MethodPut, "/dashboards/code/" + testCustomDashboardID + "/team-shares/rigel", "", "AddCodeDashboardTeamShare", testTeamShareRigelID, http.StatusOK},
	{http.MethodDelete, "/dashboards/code/" + testCustomDashboardID + "/team-shares/apollo", "", "RemoveCodeDashboardTeamShare", testTeamShareApolloID, http.StatusNoContent},
	{http.MethodGet, "/dashboards/code/" + testCustomDashboardID + "/link", "", "GetCodeDashboardLink", "", http.StatusOK},
	{http.MethodPut, "/dashboards/code/" + testCustomDashboardID + "/link", "", "EnableCodeDashboardLink", "", http.StatusOK},
	{http.MethodDelete, "/dashboards/code/" + testCustomDashboardID + "/link", "", "DisableCodeDashboardLink", "", http.StatusNoContent},
	{http.MethodGet, "/dashboards/shared/code/" + strings.ToUpper(testCustomDashboardID), "", "GetSharedCodeDashboard", "", http.StatusOK},
}

func TestCodeDashboardShares_Routes(t *testing.T) {
	for _, tc := range codeShareRoutes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			client := &fakeCodeShareClient{}
			w := serveCodeShares(client, tc.method, tc.path, tc.body, true)
			if w.Code != tc.wantStatus || client.called != tc.wantCall {
				t.Fatalf("status %d, call %q; want %d, %q: %s", w.Code, client.called, tc.wantStatus, tc.wantCall, w.Body.String())
			}
			if client.gotID != testCustomDashboardID || client.gotOther != tc.wantOther {
				t.Errorf("ids = %q, %q; want %q, %q (lower case)", client.gotID, client.gotOther, testCustomDashboardID, tc.wantOther)
			}
		})
	}
}

func TestCodeDashboardShares_RequireUser(t *testing.T) {
	routes := append([]struct {
		method, path, body, wantCall, wantOther string
		wantStatus                              int
	}{
		{http.MethodGet, "/dashboards/shared/code", "", "", "", 0},
		{http.MethodPost, "/dashboards/shared/code/link", `{"linkId":"` + testCodeShareUser + `"}`, "", "", 0},
	}, codeShareRoutes...)
	for _, tc := range routes {
		client := &fakeCodeShareClient{}
		w := serveCodeShares(client, tc.method, tc.path, tc.body, false)
		if w.Code != http.StatusUnauthorized || client.called != "" {
			t.Errorf("%s %s: status %d, call %q; want 401 and no upstream call", tc.method, tc.path, w.Code, client.called)
		}
	}
}

// Bad ids and unknown teams are refused here, without an upstream call.
func TestCodeDashboardShares_RefusedBeforeTheUpstream(t *testing.T) {
	own := "/dashboards/code/" + testCustomDashboardID
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/dashboards/code/not-a-uuid/shares", http.StatusBadRequest},
		{http.MethodPut, own + "/shares/not-a-uuid", http.StatusBadRequest},
		{http.MethodGet, "/dashboards/code/not-a-uuid/team-shares", http.StatusBadRequest},
		{http.MethodPut, own + "/team-shares/unknown", http.StatusNotFound},
		{http.MethodPut, own + "/team-shares/orion", http.StatusNotFound}, // in the registry, no group id
		{http.MethodGet, "/dashboards/code/not-a-uuid/link", http.StatusBadRequest},
		{http.MethodGet, "/dashboards/shared/code/not-a-uuid", http.StatusBadRequest},
	} {
		client := &fakeCodeShareClient{}
		w := serveCodeShares(client, tc.method, tc.path, "", true)
		if w.Code != tc.want || client.called != "" {
			t.Errorf("%s %s: status %d, call %q; want %d and no upstream call", tc.method, tc.path, w.Code, client.called, tc.want)
		}
	}
}

func TestCodeDashboardShares_TeamSharesByRegistryKey(t *testing.T) {
	client := &fakeCodeShareClient{teamShares: []entity.CustomDashboardTeamShare{
		{Team: entity.CustomDashboardTeam{ID: strings.ToUpper(testTeamShareRigelID), Name: "Rigel", Type: "cre-abt"}, SharedOn: "2026-11-13T10:00:00Z"},
		{Team: entity.CustomDashboardTeam{ID: "00000000-0000-4000-8000-000000000999", Name: "Gone", Type: "sre-abt"}},
	}}
	w := serveCodeShares(client, http.MethodGet, "/dashboards/code/"+testCustomDashboardID+"/team-shares", "", true)
	var got customDashboardTeamShareListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if len(got.Shares) != 2 || got.Shares[0].Team.Key != "rigel" || got.Shares[0].Team.Family != "cre-abt" || got.Shares[1].Team.Key != "" {
		t.Errorf("team shares = %+v; want rigel by key, and no key for a team not in the registry", got.Shares)
	}
}

func TestCodeDashboardShares_RecipientSide(t *testing.T) {
	client := &fakeCodeShareClient{}
	w := serveCodeShares(client, http.MethodGet, "/dashboards/shared/code", "", true)
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"dashboards":[]}` {
		t.Errorf("empty shared list = %d %s; want {\"dashboards\":[]}", w.Code, w.Body.String())
	}
	client = &fakeCodeShareClient{sharedOne: entity.SharedCodeDashboard{ID: testCustomDashboardID, Name: "Open Cases", Source: testCodeSource, SharedViaLink: true}}
	w = serveCodeShares(client, http.MethodGet, "/dashboards/shared/code/"+testCustomDashboardID, "", true)
	var got entity.SharedCodeDashboard
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got.Source != testCodeSource || !got.SharedViaLink {
		t.Errorf("shared dashboard = %+v, %v (%s)", got, err, w.Body.String())
	}
}

func TestCodeDashboardShares_OpenLink(t *testing.T) {
	client := &fakeCodeShareClient{}
	w := serveCodeShares(client, http.MethodPost, "/dashboards/shared/code/link", `{"linkId":"`+strings.ToUpper(testCodeShareUser)+`"}`, true)
	if w.Code != http.StatusOK || client.called != "OpenCodeDashboardLink" || client.gotOther != testCodeShareUser {
		t.Fatalf("open = %d, call %q with %q (%s)", w.Code, client.called, client.gotOther, w.Body.String())
	}
	for _, body := range []string{`{"linkId":"not-a-uuid"}`, `{"linkId":`} {
		client := &fakeCodeShareClient{}
		w := serveCodeShares(client, http.MethodPost, "/dashboards/shared/code/link", body, true)
		if w.Code != http.StatusBadRequest || client.called != "" {
			t.Errorf("body %s: status %d, call %q; want 400 and no upstream call", body, w.Code, client.called)
		}
	}
	client = &fakeCodeShareClient{err: &apierror.Error{StatusCode: http.StatusNotFound, Body: `{"message":"x"}`}}
	w = serveCodeShares(client, http.MethodPost, "/dashboards/shared/code/link", `{"linkId":"`+testCodeShareUser+`"}`, true)
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), errMsgDashboardLinkNotFound) {
		t.Errorf("unknown link = %d %s; want 404 with its own message", w.Code, w.Body.String())
	}
}

// A refusal the user can act on (not internal staff) is passed on when
// sharing; other failures get the generic messages.
func TestCodeDashboardShares_UpstreamErrors(t *testing.T) {
	refused := &apierror.Error{StatusCode: http.StatusBadRequest, Body: `{"code":400,"message":"dashboards can only be shared with internal staff"}`}
	w := serveCodeShares(&fakeCodeShareClient{err: refused}, http.MethodPut, "/dashboards/code/"+testCustomDashboardID+"/shares/"+testCodeShareUser, "", true)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "internal staff") {
		t.Errorf("refused share = %d %s; want the 400 passed on", w.Code, w.Body.String())
	}
	failed := &apierror.Error{StatusCode: http.StatusInternalServerError, Body: `{"message":"pq: relation does not exist"}`}
	for _, tc := range []struct{ method, path, want string }{
		{http.MethodGet, "/dashboards/code/" + testCustomDashboardID + "/shares", errMsgCustomDashboardSharesFailed},
		{http.MethodDelete, "/dashboards/code/" + testCustomDashboardID + "/team-shares/rigel", errMsgTeamUnshareFailed},
		{http.MethodPut, "/dashboards/code/" + testCustomDashboardID + "/link", errMsgDashboardLinkEnableFailed},
		{http.MethodGet, "/dashboards/shared/code", errMsgSharedDashboardListFailed},
		{http.MethodGet, "/dashboards/shared/code/" + testCustomDashboardID, errMsgCodeDashboardLoadFailed},
	} {
		w := serveCodeShares(&fakeCodeShareClient{err: failed}, tc.method, tc.path, "", true)
		if w.Code < 500 || !strings.Contains(w.Body.String(), tc.want) || strings.Contains(w.Body.String(), "relation") {
			t.Errorf("%s %s = %d %s; want a 5xx with %q and no upstream detail", tc.method, tc.path, w.Code, w.Body.String(), tc.want)
		}
	}
	// Routes that keep the upstream's 4xx text to themselves answer their own
	// message instead (only sharing passes a refusal on).
	detail := &apierror.Error{StatusCode: http.StatusBadRequest, Body: `{"code":400,"message":"upstream detail"}`}
	for _, tc := range []struct{ method, path, want string }{
		{http.MethodPut, "/dashboards/code/" + testCustomDashboardID + "/link", errMsgDashboardLinkEnableFailed},
		{http.MethodGet, "/dashboards/code/" + testCustomDashboardID + "/link", errMsgDashboardLinkFailed},
		{http.MethodGet, "/dashboards/shared/code/" + testCustomDashboardID, errMsgCodeDashboardLoadFailed},
		{http.MethodGet, "/dashboards/code/" + testCustomDashboardID + "/team-shares", errMsgTeamSharesFailed},
	} {
		w := serveCodeShares(&fakeCodeShareClient{err: detail}, tc.method, tc.path, "", true)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), tc.want) || strings.Contains(w.Body.String(), "upstream detail") {
			t.Errorf("%s %s with an upstream 400 = %d %s; want 400 with %q", tc.method, tc.path, w.Code, w.Body.String(), tc.want)
		}
	}
}
