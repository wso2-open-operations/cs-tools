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

package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// stubPagingRepo holds memberships by id and callers by email, and records
// which write was asked for.
type stubPagingRepo struct {
	members map[string]repository.PagingChainTarget
	callers map[string]repository.PagingCaller
	wrote   string
}

func (s *stubPagingRepo) ListMembers(_ context.Context, family string) ([]domain.PagingChainMember, error) {
	var out []domain.PagingChainMember
	for _, m := range s.members {
		if m.Family == family {
			out = append(out, m.PagingChainMember)
		}
	}
	return out, nil
}

func (s *stubPagingRepo) GetMember(_ context.Context, id string) (repository.PagingChainTarget, error) {
	m, ok := s.members[id]
	if !ok {
		return repository.PagingChainTarget{}, &apierror.NotFoundError{Msg: "team membership not found"}
	}
	return m, nil
}

func (s *stubPagingRepo) CallerAuthority(_ context.Context, email string) (repository.PagingCaller, error) {
	return s.callers[email], nil
}

func (s *stubPagingRepo) SetResponderRank(_ context.Context, _, id string, rank int) error {
	s.wrote = "rank"
	return nil
}

func (s *stubPagingRepo) SetRole(_ context.Context, _, id, role string) error {
	s.wrote = "role:" + role
	return nil
}

func (s *stubPagingRepo) ReadinessFacts(context.Context, string, string) (repository.PagingReadinessFacts, error) {
	return repository.PagingReadinessFacts{}, nil
}

const (
	idVegaEng      = "00000000-0000-0000-0000-000000000001"
	idVegaLead     = "00000000-0000-0000-0000-000000000002"
	idAmEng        = "00000000-0000-0000-0000-000000000003"
	idAmTeamLead   = "00000000-0000-0000-0000-000000000004"
	idAmHead       = "00000000-0000-0000-0000-000000000005"
	idApolloEng    = "00000000-0000-0000-0000-000000000006"
	idLeaderPerson = "00000000-0000-0000-0000-000000000007"
)

func pagingMember(id, team, family, role string) repository.PagingChainTarget {
	return repository.PagingChainTarget{PagingChainMember: domain.PagingChainMember{
		MembershipID: id, TeamKey: team, Family: family, Role: role,
	}}
}

func pagingFixture() *stubPagingRepo {
	r := &stubPagingRepo{
		members: map[string]repository.PagingChainTarget{
			idVegaEng:      pagingMember(idVegaEng, "vega", "CRE", "engineer"),
			idVegaLead:     pagingMember(idVegaLead, "vega", "CRE", "lead"),
			idAmEng:        pagingMember(idAmEng, "americas", "CRE", "engineer"),
			idAmTeamLead:   pagingMember(idAmTeamLead, "americas", "CRE", "lead"),
			idAmHead:       pagingMember(idAmHead, "americas", "CRE", "americas_team_lead"),
			idApolloEng:    pagingMember(idApolloEng, "apollo", "SRE", "engineer"),
			idLeaderPerson: pagingMember(idLeaderPerson, "cre-leadership", "CRE", "engineer"),
		},
		callers: map[string]repository.PagingCaller{
			"rota@wso2.com":     {GlobalRoles: []string{"cre_rota_admin"}},
			"admin@wso2.com":    {GlobalRoles: []string{"admin"}},
			"vegalead@example.com": {Memberships: []repository.PagingCallerMembership{{TeamKey: "vega", Role: "lead"}}},
			"amlead@example.com":   {Memberships: []repository.PagingCallerMembership{{TeamKey: "americas", Role: "lead"}}},
			"amhead@wso2.com":   {Memberships: []repository.PagingCallerMembership{{TeamKey: "americas", Role: "americas_team_lead"}}},
			"crehead@wso2.com":  {Memberships: []repository.PagingCallerMembership{{TeamKey: "cre-leadership", Role: "cre_head"}}},
			"cshead@wso2.com":   {Memberships: []repository.PagingCallerMembership{{TeamKey: "cre-leadership", Role: "cs_head"}}},
			"eng@wso2.com":      {Memberships: []repository.PagingCallerMembership{{TeamKey: "vega", Role: "engineer"}}},
		},
	}
	// GetMember reports whether the team has a head other than the member.
	am := r.members[idAmEng]
	am.TeamHasHead = true
	r.members[idAmEng] = am
	tl := r.members[idAmTeamLead]
	tl.TeamHasHead = true
	r.members[idAmTeamLead] = tl
	return r
}

func pagingCaller(email string) context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: email})
}

// strp is problem_transition_test.go's.
func intp(v int) *int { return &v }

// The agreed table of who may change what, one row per (caller, edit).
func TestUpdatePagingMember_PermissionTable(t *testing.T) {
	for _, tc := range []struct {
		name   string
		caller string
		req    domain.UpdatePagingMemberRequest
		allow  bool
	}{
		// 1st-3rd responders: the team's lead, or its head where it has one, or
		// whoever runs the whole CRE chain -- a CRE rota admin, the CRE head, the CS head.
		{"vega lead picks vega responder", "vegalead@example.com", domain.UpdatePagingMemberRequest{MembershipID: idVegaEng, ResponderRank: intp(2)}, true},
		{"americas lead cannot pick vega responder", "amlead@example.com", domain.UpdatePagingMemberRequest{MembershipID: idVegaEng, ResponderRank: intp(2)}, false},
		{"rota admin picks any responder", "rota@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idVegaEng, ResponderRank: intp(1)}, true},
		{"america lead picks americas responder", "amhead@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idAmEng, ResponderRank: intp(1)}, true},
		{"americas team lead cannot pick americas responder", "amlead@example.com", domain.UpdatePagingMemberRequest{MembershipID: idAmEng, ResponderRank: intp(1)}, false},
		{"engineer cannot pick responders", "eng@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idVegaEng, ResponderRank: intp(1)}, false},
		{"portal admin alone cannot pick responders", "admin@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idVegaEng, ResponderRank: intp(1)}, false},
		{"CRE head picks an ABT responder", "crehead@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idVegaEng, ResponderRank: intp(1)}, true},
		{"CS head picks an Americas responder", "cshead@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idAmEng, ResponderRank: intp(2)}, true},
		{"CRE head cannot pick an SRE responder", "crehead@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idApolloEng, ResponderRank: intp(1)}, false},

		// Team leads: the head for a team that has one, and whoever runs the whole
		// CRE chain -- a CRE rota admin, the CRE head, the CS head -- for any team.
		{"CRE head sets an ABT Team lead", "crehead@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idVegaEng, Role: strp("lead")}, true},
		{"vega lead cannot set vega Team leads", "vegalead@example.com", domain.UpdatePagingMemberRequest{MembershipID: idVegaEng, Role: strp("lead")}, false},
		{"america lead sets an Americas Team lead", "amhead@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idAmEng, Role: strp("lead")}, true},
		{"CRE head sets an Americas Team lead", "crehead@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idAmEng, Role: strp("lead")}, true},
		{"CS head sets an ABT Team lead", "cshead@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idVegaEng, Role: strp("lead")}, true},
		{"rota admin sets any Team lead", "rota@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idAmTeamLead, Role: strp("engineer")}, true},

		// The America lead, the CRE head and the CS head: a CRE rota admin, the
		// CRE head, the CS head, or a portal admin.
		{"rota admin sets the America lead", "rota@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idAmTeamLead, Role: strp("americas_team_lead")}, true},
		{"portal admin sets the America lead", "admin@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idAmTeamLead, Role: strp("americas_team_lead")}, true},
		{"america lead cannot pass on the role", "amhead@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idAmTeamLead, Role: strp("americas_team_lead")}, false},
		{"CRE head sets the America lead", "crehead@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idAmTeamLead, Role: strp("americas_team_lead")}, true},
		{"CS head steps the America lead down", "cshead@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idAmHead, Role: strp("lead")}, true},
		{"portal admin sets the CS head", "admin@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idLeaderPerson, Role: strp("cs_head")}, true},
		{"CRE head sets the CS head", "crehead@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idLeaderPerson, Role: strp("cs_head")}, true},
		{"CS head sets the CRE head", "cshead@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idLeaderPerson, Role: strp("cre_head")}, true},
		{"an ABT lead still cannot set the CS head", "vegalead@example.com", domain.UpdatePagingMemberRequest{MembershipID: idLeaderPerson, Role: strp("cs_head")}, false},
		{"america lead cannot step themselves down", "amhead@wso2.com", domain.UpdatePagingMemberRequest{MembershipID: idAmHead, Role: strp("engineer")}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := pagingFixture()
			_, err := NewPagingChainService(repo, nil, alwaysUnrestrictedAccess{}, nil, nil).UpdatePagingMember(pagingCaller(tc.caller), tc.req)
			var forbidden *apierror.ForbiddenError
			switch {
			case tc.allow && err != nil:
				t.Fatalf("want allowed, got %v", err)
			case tc.allow && repo.wrote == "":
				t.Fatal("allowed, but nothing was written")
			case !tc.allow && !errors.As(err, &forbidden):
				t.Fatalf("want ForbiddenError, got %v", err)
			case !tc.allow && repo.wrote != "":
				t.Fatalf("refused, but %q was written", repo.wrote)
			}
		})
	}
}

// The SRE paging chain comes from the rota; nothing on the tab changes it,
// not even for a CRE rota admin.
func TestUpdatePagingMember_SREIsReadOnly(t *testing.T) {
	repo := pagingFixture()
	_, err := NewPagingChainService(repo, nil, alwaysUnrestrictedAccess{}, nil, nil).UpdatePagingMember(
		pagingCaller("rota@wso2.com"), domain.UpdatePagingMemberRequest{MembershipID: idApolloEng, ResponderRank: intp(1)})
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) || repo.wrote != "" {
		t.Fatalf("want ForbiddenError and no write, got %v (wrote %q)", err, repo.wrote)
	}
}

func TestUpdatePagingMember_RejectsBadRequests(t *testing.T) {
	for name, req := range map[string]domain.UpdatePagingMemberRequest{
		"nothing to change":       {MembershipID: idVegaEng},
		"two changes at once":     {MembershipID: idVegaEng, ResponderRank: intp(1), Role: strp("lead")},
		"rank out of range":       {MembershipID: idVegaEng, ResponderRank: intp(4)},
		"lead as a responder":     {MembershipID: idVegaLead, ResponderRank: intp(1)},
		"unknown role":            {MembershipID: idVegaEng, Role: strp("manager")},
		"sub_lead is not offered": {MembershipID: idVegaEng, Role: strp("sub_lead")},
		"not a uuid":              {MembershipID: "vega", ResponderRank: intp(1)},
	} {
		t.Run(name, func(t *testing.T) {
			repo := pagingFixture()
			_, err := NewPagingChainService(repo, nil, alwaysUnrestrictedAccess{}, nil, nil).UpdatePagingMember(pagingCaller("rota@wso2.com"), req)
			var invalid *apierror.ValidationError
			if !errors.As(err, &invalid) || repo.wrote != "" {
				t.Fatalf("want ValidationError and no write, got %v (wrote %q)", err, repo.wrote)
			}
		})
	}
}

// An edit is a person's: a service credential with no user may read but not write.
func TestUpdatePagingMember_NeedsAUser(t *testing.T) {
	repo := pagingFixture()
	_, err := NewPagingChainService(repo, nil, alwaysUnrestrictedAccess{}, nil, nil).UpdatePagingMember(
		context.Background(), domain.UpdatePagingMemberRequest{MembershipID: idVegaEng, ResponderRank: intp(1)})
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("want ForbiddenError, got %v", err)
	}
}

// What the tab locks follows the same table as the writes.
func TestGetPagingChain_CanEditFollowsTheTable(t *testing.T) {
	for _, tc := range []struct {
		caller       string
		responders   []string
		teamLeads    []string
		headsAndHead bool
	}{
		{"vegalead@example.com", []string{"vega"}, []string{}, false},
		{"amhead@wso2.com", []string{"americas"}, []string{"americas"}, false},
		{"crehead@wso2.com", []string{"americas", "cre-leadership", "vega"}, []string{"americas", "cre-leadership", "vega"}, true},
		{"cshead@wso2.com", []string{"americas", "cre-leadership", "vega"}, []string{"americas", "cre-leadership", "vega"}, true},
		{"rota@wso2.com", []string{"americas", "cre-leadership", "vega"}, []string{"americas", "cre-leadership", "vega"}, true},
		{"admin@wso2.com", []string{}, []string{}, true},
		{"eng@wso2.com", []string{}, []string{}, false},
	} {
		t.Run(tc.caller, func(t *testing.T) {
			resp, err := NewPagingChainService(pagingFixture(), nil, alwaysUnrestrictedAccess{}, nil, nil).GetPagingChain(pagingCaller(tc.caller), "cre")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(resp.CanEdit.ResponderTeams, tc.responders) {
				t.Errorf("responderTeams = %v, want %v", resp.CanEdit.ResponderTeams, tc.responders)
			}
			if !reflect.DeepEqual(resp.CanEdit.TeamLeadTeams, tc.teamLeads) {
				t.Errorf("teamLeadTeams = %v, want %v", resp.CanEdit.TeamLeadTeams, tc.teamLeads)
			}
			if resp.CanEdit.Heads != tc.headsAndHead || resp.CanEdit.AmericasTeamLead != tc.headsAndHead {
				t.Errorf("heads/americasTeamLead = %v/%v, want %v", resp.CanEdit.Heads, resp.CanEdit.AmericasTeamLead, tc.headsAndHead)
			}
			if resp.Family != "CRE" {
				t.Errorf("family = %q, want CRE", resp.Family)
			}
		})
	}
	resp, err := NewPagingChainService(pagingFixture(), nil, alwaysUnrestrictedAccess{}, nil, nil).GetPagingChain(pagingCaller("rota@wso2.com"), "SRE")
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.CanEdit.ResponderTeams) != 0 || resp.CanEdit.Heads {
		t.Errorf("SRE side must be read-only, got %+v", resp.CanEdit)
	}
}
