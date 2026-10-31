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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// patchSpy records the patch and the move the handler hands the service.
type patchSpy struct {
	service.RotaAccessService
	patch domain.TeamMemberPatch
	to    string
}

func (p *patchSpy) UpdateTeamMember(_ context.Context, team, user string, patch domain.TeamMemberPatch) (domain.TeamMembership, error) {
	p.patch = patch
	return domain.TeamMembership{TeamKey: team, UserID: user, Role: "engineer"}, nil
}

func (p *patchSpy) MoveTeamMember(_ context.Context, team, user, to string) (domain.TeamMembership, error) {
	p.to = to
	return domain.TeamMembership{TeamKey: to, UserID: user, Role: "engineer"}, nil
}

func callMemberRoute(t *testing.T, spy *patchSpy, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewRotaAccessHandler(spy)
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /team-schedule/teams/{teamKey}/members/{userId}", h.UpdateTeamMember)
	mux.HandleFunc("POST /team-schedule/teams/{teamKey}/members/{userId}/move", h.MoveTeamMember)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	return rec
}

// An absent alertTier leaves the tier, null clears it, a string sets it.
func TestUpdateTeamMemberTellsAbsentFromNull(t *testing.T) {
	const path = "/team-schedule/teams/iaas/members/00000000-0000-0000-0000-000000000000"
	for _, tc := range []struct {
		body   string
		set    bool
		tier   string // "" means nil
		role   string
		status int
	}{
		{`{"role":"sub_lead"}`, false, "", "sub_lead", 200},
		{`{"alertTier":null}`, true, "", "", 200},
		{`{"alertTier":"T2"}`, true, "T2", "", 200},
		{`{"alertTier":7}`, false, "", "", 400},
	} {
		spy := &patchSpy{}
		rec := callMemberRoute(t, spy, http.MethodPatch, path, tc.body)
		if rec.Code != tc.status {
			t.Fatalf("%s: status %d, want %d (%s)", tc.body, rec.Code, tc.status, rec.Body)
		}
		if tc.status != 200 {
			continue
		}
		if spy.patch.AlertTierSet != tc.set {
			t.Fatalf("%s: AlertTierSet = %v, want %v", tc.body, spy.patch.AlertTierSet, tc.set)
		}
		if (spy.patch.AlertTier == nil) != (tc.tier == "") || (tc.tier != "" && *spy.patch.AlertTier != tc.tier) {
			t.Fatalf("%s: AlertTier = %v, want %q", tc.body, spy.patch.AlertTier, tc.tier)
		}
		if (spy.patch.Role == nil) != (tc.role == "") || (tc.role != "" && *spy.patch.Role != tc.role) {
			t.Fatalf("%s: Role = %v, want %q", tc.body, spy.patch.Role, tc.role)
		}
	}
}

func TestMoveTeamMemberReadsTheTargetFromTheBody(t *testing.T) {
	spy := &patchSpy{}
	rec := callMemberRoute(t, spy, http.MethodPost, "/team-schedule/teams/iaas/members/00000000-0000-0000-0000-000000000000/move", `{"toTeamKey":"kiwi"}`)
	if rec.Code != 200 || spy.to != "kiwi" || !strings.Contains(rec.Body.String(), `"teamKey":"kiwi"`) || !strings.Contains(rec.Body.String(), `"alertTier":null`) {
		t.Fatalf("move: %d %s, to %q", rec.Code, rec.Body, spy.to)
	}
}
