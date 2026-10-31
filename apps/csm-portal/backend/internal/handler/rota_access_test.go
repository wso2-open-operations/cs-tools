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

type fakeRotaAccessClient struct {
	got string // "op scope user"
	err error
}

func (f *fakeRotaAccessClient) record(op, scope, user string) ([]byte, error) {
	f.got = op + " " + scope + " " + user
	return nil, f.err
}

func (f *fakeRotaAccessClient) GetRotaAccess(context.Context) ([]byte, error) {
	f.got = "get"
	return []byte(`{"canManage":{"rotaAdmins":false,"markerFamilies":[]},"rotaAdmins":[],"markers":[]}`), f.err
}

func (f *fakeRotaAccessClient) PutRotaAdmin(_ context.Context, family, user string) ([]byte, error) {
	return f.record("put-admin", family, user)
}

func (f *fakeRotaAccessClient) DeleteRotaAdmin(_ context.Context, family, user string) ([]byte, error) {
	return f.record("delete-admin", family, user)
}

func (f *fakeRotaAccessClient) PutRotaMarker(_ context.Context, team, user string) ([]byte, error) {
	return f.record("put-marker", team, user)
}

func (f *fakeRotaAccessClient) DeleteRotaMarker(_ context.Context, team, user string) ([]byte, error) {
	return f.record("delete-marker", team, user)
}

func (f *fakeRotaAccessClient) PutTeamMember(_ context.Context, team, user string) ([]byte, error) {
	return f.record("put-member", team, user)
}

func (f *fakeRotaAccessClient) DeleteTeamMember(_ context.Context, team, user string) ([]byte, error) {
	return f.record("delete-member", team, user)
}

func (f *fakeRotaAccessClient) PatchTeamMember(_ context.Context, team, user string, body []byte) ([]byte, error) {
	f.got = "patch-member " + team + " " + user + " " + string(body)
	return []byte(`{"teamKey":"` + team + `","userId":"` + user + `","role":"sub_lead","alertTier":"T1"}`), f.err
}

func (f *fakeRotaAccessClient) MoveTeamMember(_ context.Context, team, user string, body []byte) ([]byte, error) {
	f.got = "move-member " + team + " " + user + " " + string(body)
	return []byte(`{"teamKey":"apollo","userId":"` + user + `","role":"engineer","alertTier":null}`), f.err
}

const rotaAccessUser = "33333333-3333-3333-3333-333333333333"

func rotaAccessRequest(method, scopeName, scope, user string) *http.Request {
	r := httptest.NewRequest(method, "/team-schedule/rota-access/x/"+scope+"/"+user, nil)
	r.SetPathValue(scopeName, scope)
	r.SetPathValue("userId", user)
	return r
}

func TestRotaAccess_ForwardsAGrantOrRemoval(t *testing.T) {
	fake := &fakeRotaAccessClient{}
	h := NewRotaAccessHandler(fake)
	for _, c := range []struct {
		call      func(http.ResponseWriter, *http.Request)
		method    string
		scopeName string
		scope     string
		want      string
	}{
		{h.PutRotaAdmin, http.MethodPut, "family", "sre", "put-admin sre " + rotaAccessUser},
		{h.DeleteRotaAdmin, http.MethodDelete, "family", "CRE", "delete-admin CRE " + rotaAccessUser},
		{h.PutRotaMarker, http.MethodPut, "teamKey", "apollo", "put-marker apollo " + rotaAccessUser},
		{h.DeleteRotaMarker, http.MethodDelete, "teamKey", "phoenix_abt_cre_team", "delete-marker phoenix_abt_cre_team " + rotaAccessUser},
		{h.PutTeamMember, http.MethodPut, "teamKey", "iaas", "put-member iaas " + rotaAccessUser},
		{h.DeleteTeamMember, http.MethodDelete, "teamKey", "cloud-agent", "delete-member cloud-agent " + rotaAccessUser},
	} {
		w := httptest.NewRecorder()
		c.call(w, withUser(rotaAccessRequest(c.method, c.scopeName, c.scope, rotaAccessUser)))
		if w.Code != http.StatusNoContent || fake.got != c.want {
			t.Fatalf("%s: status %d, forwarded %q", c.want, w.Code, fake.got)
		}
	}
}

func TestRotaAccess_RefusesABadPathWithoutCallingOut(t *testing.T) {
	fake := &fakeRotaAccessClient{}
	h := NewRotaAccessHandler(fake)
	for name, r := range map[string]*http.Request{
		"unknown family": rotaAccessRequest(http.MethodPut, "family", "QA", rotaAccessUser),
		"not a uuid":     rotaAccessRequest(http.MethodPut, "family", "SRE", "someone"),
		"bad team key":   rotaAccessRequest(http.MethodPut, "teamKey", "../admins", rotaAccessUser),
	} {
		w := httptest.NewRecorder()
		if strings.Contains(name, "team") {
			h.PutRotaMarker(w, withUser(r))
		} else {
			h.PutRotaAdmin(w, withUser(r))
		}
		if w.Code != http.StatusBadRequest || fake.got != "" {
			t.Fatalf("%s: status %d, forwarded %q", name, w.Code, fake.got)
		}
	}
}

func TestRotaAccess_NeedsAUser(t *testing.T) {
	w := httptest.NewRecorder()
	NewRotaAccessHandler(&fakeRotaAccessClient{}).GetRotaAccess(w, httptest.NewRequest(http.MethodGet, "/team-schedule/rota-access", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", w.Code)
	}
}

// A refusal keeps entity-service's words, so the tab can say why nothing
// changed; anything else stays generic.
func TestRotaAccess_KeepsTheRefusalReason(t *testing.T) {
	for _, c := range []struct {
		status int
		msg    string
		want   string
	}{
		{http.StatusForbidden, "only a CSM admin can grant or remove rota admin", "only a CSM admin"},
		{http.StatusBadRequest, "a rota manager marks their own team, and that person is not on lyra", "not on lyra"},
		{http.StatusNotFound, "cab is not a team on the schedule", "not a team on the schedule"},
		{http.StatusInternalServerError, "pq: relation does not exist", "Failed to add the rota manager."},
	} {
		fake := &fakeRotaAccessClient{err: &apierror.Error{StatusCode: c.status, Body: `{"code":1,"message":"` + c.msg + `"}`}}
		w := httptest.NewRecorder()
		NewRotaAccessHandler(fake).PutRotaMarker(w, withUser(rotaAccessRequest(http.MethodPut, "teamKey", "lyra", rotaAccessUser)))
		if w.Code != c.status || !strings.Contains(w.Body.String(), c.want) {
			t.Fatalf("%d: got %d %s", c.status, w.Code, w.Body.String())
		}
	}
}

// A refused removal reaches the tab with entity-service's own reason (409
// for upcoming turns or a tier, 403 for a lead), and a server fault does not.
func TestRotaAccess_TeamMemberRemovalKeepsTheRefusalReason(t *testing.T) {
	for _, c := range []struct {
		status int
		msg    string
		want   string
	}{
		{http.StatusConflict, "this person still has 3 upcoming turns on iaas: remove or reassign them on the roster first", "3 upcoming turns"},
		{http.StatusForbidden, "this person is a lead of iaas, which decides who Case Alerts calls: only a CSM admin can remove them", "only a CSM admin"},
		{http.StatusInternalServerError, "pq: relation does not exist", "Failed to remove the team member."},
	} {
		fake := &fakeRotaAccessClient{err: &apierror.Error{StatusCode: c.status, Body: `{"code":1,"message":"` + c.msg + `"}`}}
		w := httptest.NewRecorder()
		NewRotaAccessHandler(fake).DeleteTeamMember(w, withUser(rotaAccessRequest(http.MethodDelete, "teamKey", "iaas", rotaAccessUser)))
		if w.Code != c.status || !strings.Contains(w.Body.String(), c.want) {
			t.Fatalf("%d: got %d %s", c.status, w.Code, w.Body.String())
		}
	}
}

func memberBodyRequest(method, path, teamKey, user, body string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.SetPathValue("teamKey", teamKey)
	r.SetPathValue("userId", user)
	return r
}

func TestRotaAccess_TeamMemberChangeForwardsPathAndBody(t *testing.T) {
	fake := &fakeRotaAccessClient{}
	h := NewRotaAccessHandler(fake)
	for _, c := range []struct {
		name string
		call func(http.ResponseWriter, *http.Request)
		body string
		want string
	}{
		{"patch", h.PatchTeamMember, `{"role":"sub_lead","alertTier":null}`, `patch-member iaas ` + rotaAccessUser + ` {"role":"sub_lead","alertTier":null}`},
		{"move", h.MoveTeamMember, `{"toTeamKey":"apollo"}`, `move-member iaas ` + rotaAccessUser + ` {"toTeamKey":"apollo"}`},
	} {
		w := httptest.NewRecorder()
		c.call(w, withUser(memberBodyRequest(http.MethodPost, "/team-schedule/teams/iaas/members/x", "iaas", rotaAccessUser, c.body)))
		if w.Code != http.StatusOK || fake.got != c.want || !strings.Contains(w.Body.String(), rotaAccessUser) {
			t.Fatalf("%s: status %d, forwarded %q, body %s", c.name, w.Code, fake.got, w.Body.String())
		}
	}
}

func TestRotaAccess_TeamMemberChangeRefusesABadRequestWithoutCallingOut(t *testing.T) {
	for _, c := range []struct {
		name, teamKey, user, body string
		authed                    bool
		want                      int
	}{
		{"bad team key", "../x", rotaAccessUser, `{"role":"engineer"}`, true, http.StatusBadRequest},
		{"not a uuid", "iaas", "someone", `{"role":"engineer"}`, true, http.StatusBadRequest},
		{"malformed JSON", "iaas", rotaAccessUser, `{`, true, http.StatusBadRequest},
		{"no user", "iaas", rotaAccessUser, `{"role":"engineer"}`, false, http.StatusUnauthorized},
	} {
		for op, call := range map[string]func(*RotaAccessHandler) func(http.ResponseWriter, *http.Request){
			"patch": func(h *RotaAccessHandler) func(http.ResponseWriter, *http.Request) { return h.PatchTeamMember },
			"move":  func(h *RotaAccessHandler) func(http.ResponseWriter, *http.Request) { return h.MoveTeamMember },
		} {
			fake := &fakeRotaAccessClient{}
			r := memberBodyRequest(http.MethodPatch, "/team-schedule/teams/x/members/y", c.teamKey, c.user, c.body)
			if c.authed {
				r = withUser(r)
			}
			w := httptest.NewRecorder()
			call(NewRotaAccessHandler(fake))(w, r)
			if w.Code != c.want || fake.got != "" {
				t.Fatalf("%s %s: status %d, forwarded %q", op, c.name, w.Code, fake.got)
			}
		}
	}
}

// Every refusal entity-service gives reaches the caller with its reason; a
// server fault does not.
func TestRotaAccess_TeamMemberChangeKeepsTheRefusalReason(t *testing.T) {
	for _, c := range []struct {
		status int
		msg    string
		want   string
	}{
		{http.StatusBadRequest, "alertTier must be T1, T2 or T3", "alertTier must be"},
		{http.StatusForbidden, "only a CSM admin can change a team member", "only a CSM admin"},
		{http.StatusNotFound, "that person is not on iaas", "not on iaas"},
		{http.StatusConflict, "this person has 2 upcoming turns on iaas", "2 upcoming turns"},
		{http.StatusUnauthorized, "token expired", ErrMsgUnauthorized},
		{http.StatusInternalServerError, "pq: relation does not exist", "Failed to "},
	} {
		for op, call := range map[string]func(*RotaAccessHandler) func(http.ResponseWriter, *http.Request){
			"patch": func(h *RotaAccessHandler) func(http.ResponseWriter, *http.Request) { return h.PatchTeamMember },
			"move":  func(h *RotaAccessHandler) func(http.ResponseWriter, *http.Request) { return h.MoveTeamMember },
		} {
			fake := &fakeRotaAccessClient{err: &apierror.Error{StatusCode: c.status, Body: `{"code":1,"message":"` + c.msg + `"}`}}
			w := httptest.NewRecorder()
			call(NewRotaAccessHandler(fake))(w, withUser(memberBodyRequest(http.MethodPost, "/x", "iaas", rotaAccessUser, `{"toTeamKey":"apollo"}`)))
			if w.Code != c.status || !strings.Contains(w.Body.String(), c.want) || strings.Contains(w.Body.String(), "pq:") {
				t.Fatalf("%s %d: got %d %s", op, c.status, w.Code, w.Body.String())
			}
		}
	}
}
