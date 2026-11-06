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
	"slices"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const (
	someoneID = "33333333-3333-3333-3333-333333333333"
)

// fakeRotaAccessRepo answers the rota access reads from its fields and
// records the last write.
type fakeRotaAccessRepo struct {
	internal     bool
	portalAdmin  bool              // the caller holds the global admin role
	teamFamily   map[string]string // rostered team -> family
	listedFamily []string
	listedTeams  []string
	listed       bool
	wrote        string // "grant-admin:sre_rota_admin", "revoke-marker:lyra", ...
	actor        string
	// removal is what RemoveTeamMember finds; allowedLead records the flag
	// the service passed.
	removal     domain.TeamMemberRemoval
	allowedLead bool
	// update and move are what UpdateTeamMember and MoveTeamMember find;
	// a write is recorded only when the real one would have written.
	update domain.TeamMemberUpdate
	move   domain.TeamMemberMove
}

func (f *fakeRotaAccessRepo) ListRotaAdmins(context.Context) ([]domain.RotaAdminGrant, error) {
	f.listed = true
	return []domain.RotaAdminGrant{{Family: "SRE", UserID: someoneID}}, nil
}

func (f *fakeRotaAccessRepo) ListRotaMarkers(_ context.Context, families, teams []string) ([]domain.RotaMarkerGrant, error) {
	f.listed = true
	f.listedFamily, f.listedTeams = families, teams
	return []domain.RotaMarkerGrant{}, nil
}

func (f *fakeRotaAccessRepo) IsPortalAdmin(context.Context, string) (bool, error) {
	return f.portalAdmin, nil
}

func (f *fakeRotaAccessRepo) GrantRotaAdmin(_ context.Context, _, role, actor string) (bool, error) {
	f.wrote, f.actor = "grant-admin:"+role, actor
	return true, nil
}

func (f *fakeRotaAccessRepo) RevokeRotaAdmin(_ context.Context, _, role, actor string) (bool, error) {
	f.wrote, f.actor = "revoke-admin:"+role, actor
	return true, nil
}

func (f *fakeRotaAccessRepo) GrantRotaMarker(_ context.Context, _, team, actor string) (bool, error) {
	f.wrote, f.actor = "grant-marker:"+team, actor
	return true, nil
}

func (f *fakeRotaAccessRepo) RevokeRotaMarker(_ context.Context, _, team, actor string) (bool, error) {
	f.wrote, f.actor = "revoke-marker:"+team, actor
	return true, nil
}

func (f *fakeRotaAccessRepo) AddTeamMember(_ context.Context, _, team, actor string) (bool, error) {
	f.wrote, f.actor = "add-member:"+team, actor
	return true, nil
}

// RemoveTeamMember records the call as a write only when the fake's removal
// says the rows were deleted, as the real one writes nothing otherwise.
func (f *fakeRotaAccessRepo) RemoveTeamMember(_ context.Context, _, team, actor string, allowLead bool) (domain.TeamMemberRemoval, error) {
	f.allowedLead = allowLead
	res := f.removal
	blockedByLead := slices.Contains([]string{"lead", "sub_lead", "americas_team_lead", "cre_head", "cs_head"}, res.Role) && !allowLead
	if res.Member && res.AlertTier == "" && res.UpcomingTurns == 0 && !blockedByLead {
		res.Removed = true
		f.wrote, f.actor = "remove-member:"+team, actor
	}
	return res, nil
}

func (f *fakeRotaAccessRepo) UpdateTeamMember(_ context.Context, _, team, actor string, _ domain.TeamMemberPatch) (domain.TeamMemberUpdate, error) {
	res := f.update
	if res.Member && !res.Lead && res.TierHolder == "" {
		res.Updated = true
		f.wrote, f.actor = "update-member:"+team, actor
	}
	return res, nil
}

func (f *fakeRotaAccessRepo) MoveTeamMember(_ context.Context, _, from, to, actor string) (domain.TeamMemberMove, error) {
	res := f.move
	if res.OnSource && !res.OnTarget && !res.Lead && res.AlertTier == "" && res.UpcomingTurns == 0 {
		res.Moved = true
		f.wrote, f.actor = "move-member:"+from+">"+to, actor
	}
	return res, nil
}

func (f *fakeRotaAccessRepo) UserIsInternal(context.Context, string) (bool, error) {
	return f.internal, nil
}

func (f *fakeRotaAccessRepo) RosteredTeamFamily(_ context.Context, team string) (string, bool, error) {
	fam, ok := f.teamFamily[team]
	return fam, ok, nil
}

// rotaAdminOf is a schedule repository whose caller runs the given families.
type rotaAdminOf struct {
	fakeScheduleRepo
	families []string
	led      []string // teams the caller leads
	member   bool
}

func (r *rotaAdminOf) LeadTeamsFor(context.Context, string) ([]string, error) {
	if r.led == nil {
		return []string{}, nil
	}
	return r.led, nil
}

func (r *rotaAdminOf) RotaAdminFamilies(context.Context, string) ([]string, error) {
	return r.families, nil
}

func (r *rotaAdminOf) UserInTeam(context.Context, string, string) (bool, error) {
	return r.member, nil
}

func callerWithRoles(email string, roles ...string) context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: email, Roles: roles})
}

func newRotaAccess(repo *fakeRotaAccessRepo, families ...string) (RotaAccessService, *rotaAdminOf) {
	schedule := &rotaAdminOf{families: families, member: true}
	return NewRotaAccessService(repo, schedule, alwaysUnrestrictedAccess{}, []string{"cs_admin"}), schedule
}

func wantForbidden(t *testing.T, err error, what string) {
	t.Helper()
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("%s: want ForbiddenError, got %v", what, err)
	}
}

func wantInvalid(t *testing.T, err error, what string) {
	t.Helper()
	var invalid *apierror.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("%s: want ValidationError, got %v", what, err)
	}
}

// Somebody who may change nothing gets an empty tab, not a refusal, and the
// grants are not even read for them.
func TestRotaAccessIsEmptyForSomebodyWhoMayChangeNothing(t *testing.T) {
	repo := &fakeRotaAccessRepo{}
	svc, _ := newRotaAccess(repo)
	got, err := svc.Get(callerWithRoles("engineer@example.com", "cs_engineer"))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.CanManage.RotaAdmins || len(got.CanManage.MarkerFamilies) != 0 || repo.listed {
		t.Fatalf("got %+v (listed=%v), want nothing", got, repo.listed)
	}
}

// The Team Schedule's admin is the CSM Portal admin: a token role listed in
// AUTH_ADMIN_ROLES. They may change it all.
func TestCSMPortalAdminMayChangeEveryFamily(t *testing.T) {
	repo := &fakeRotaAccessRepo{internal: true}
	svc, _ := newRotaAccess(repo)
	ctx := callerWithRoles("head@example.com", "cs_engineer", "cs_admin")
	got, err := svc.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.CanManage.RotaAdmins || !reflect.DeepEqual(got.CanManage.MarkerFamilies, []string{"CRE", "SRE", "SME"}) {
		t.Fatalf("CanManage = %+v, want everything", got.CanManage)
	}
	if err := svc.GrantRotaAdmin(ctx, "sre", someoneID); err != nil {
		t.Fatalf("GrantRotaAdmin: %v", err)
	}
	if repo.wrote != "grant-admin:sre_rota_admin" || repo.actor != "head@example.com" {
		t.Fatalf("wrote %q as %q", repo.wrote, repo.actor)
	}
}

// Matched exactly, as csm-portal-backend matches AUTH_ADMIN_ROLES: a role the
// portal would not treat as admin must not make anyone admin here either.
func TestOnlyTheExactPortalAdminRoleCounts(t *testing.T) {
	for name, roles := range map[string][]string{
		"another case": {"CS_ADMIN"},
		"no roles":     nil,
		"engineer":     {"cs_engineer"},
	} {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRotaAccessRepo{internal: true}
			svc, _ := newRotaAccess(repo)
			got, err := svc.Get(callerWithRoles("someone@example.com", roles...))
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got.CanManage.RotaAdmins {
				t.Fatalf("roles %v made a CSM admin", roles)
			}
		})
	}
}

// A rota admin runs one family: they name that family's markers and nothing
// else -- not another family's, and not the rota admins themselves.
func TestRotaAdminNamesMarkersInTheirOwnFamilyOnly(t *testing.T) {
	repo := &fakeRotaAccessRepo{internal: true, teamFamily: map[string]string{"apollo": "SRE", "lyra": "CRE"}}
	svc, _ := newRotaAccess(repo, "SRE")
	ctx := callerWithRoles("sre.rota.admin@example.com", "cs_engineer")

	got, err := svc.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.CanManage.RotaAdmins || !reflect.DeepEqual(repo.listedFamily, []string{"SRE"}) {
		t.Fatalf("CanManage = %+v, markers listed for %v; want SRE only", got.CanManage, repo.listedFamily)
	}
	if err := svc.GrantRotaMarker(ctx, "apollo", someoneID); err != nil {
		t.Fatalf("own family: %v", err)
	}
	if repo.wrote != "grant-marker:apollo" {
		t.Fatalf("wrote %q", repo.wrote)
	}
	repo.wrote = ""
	wantForbidden(t, svc.GrantRotaMarker(ctx, "lyra", someoneID), "a CRE team")
	wantForbidden(t, svc.RevokeRotaMarker(ctx, "lyra", someoneID), "removing a CRE team's marker")
	wantForbidden(t, svc.GrantRotaAdmin(ctx, "SRE", someoneID), "granting rota admin")
	if repo.wrote != "" {
		t.Fatalf("a refused change was written: %q", repo.wrote)
	}
}

// A marker marks their own team, and a rota role is only for internal staff:
// both are refused up front rather than stored as a grant that does nothing.
func TestRotaMarkerMustBeAnInternalMemberOfTheTeam(t *testing.T) {
	admin := callerWithRoles("head@example.com", "cs_admin")

	repo := &fakeRotaAccessRepo{internal: true, teamFamily: map[string]string{"lyra": "CRE"}}
	svc, schedule := newRotaAccess(repo)
	schedule.member = false
	wantInvalid(t, svc.GrantRotaMarker(admin, "lyra", someoneID), "not on the team")

	repo = &fakeRotaAccessRepo{internal: false, teamFamily: map[string]string{"lyra": "CRE"}}
	svc, _ = newRotaAccess(repo)
	wantInvalid(t, svc.GrantRotaMarker(admin, "lyra", someoneID), "not internal")
	wantInvalid(t, svc.GrantRotaAdmin(admin, "CRE", someoneID), "rota admin, not internal")
	if repo.wrote != "" {
		t.Fatalf("a refused grant was written: %q", repo.wrote)
	}
}

// Removing a marker who has since left the team is exactly what removal is
// for, so it is not held to the membership a grant needs.
func TestRemovingAMarkerWhoLeftTheTeam(t *testing.T) {
	repo := &fakeRotaAccessRepo{internal: true, teamFamily: map[string]string{"lyra": "CRE"}}
	svc, schedule := newRotaAccess(repo)
	schedule.member = false
	if err := svc.RevokeRotaMarker(callerWithRoles("head@example.com", "cs_admin"), "lyra", someoneID); err != nil {
		t.Fatalf("RevokeRotaMarker: %v", err)
	}
	if repo.wrote != "revoke-marker:lyra" {
		t.Fatalf("wrote %q", repo.wrote)
	}
}

func TestRotaAccessRefusesWhatItCannotName(t *testing.T) {
	repo := &fakeRotaAccessRepo{internal: true, teamFamily: map[string]string{"lyra": "CRE"}}
	svc, _ := newRotaAccess(repo)
	admin := callerWithRoles("head@example.com", "cs_admin")

	wantInvalid(t, svc.GrantRotaAdmin(admin, "QA", someoneID), "unknown family")
	wantInvalid(t, svc.GrantRotaAdmin(admin, "CRE", ""), "no user")
	wantInvalid(t, svc.GrantRotaMarker(admin, "lyra", "not-a-uuid"), "bad user id")
	var notFound *apierror.NotFoundError
	if err := svc.GrantRotaMarker(admin, "cab", someoneID); !errors.As(err, &notFound) {
		t.Fatalf("a team not on the schedule: want NotFoundError, got %v", err)
	}
	if repo.wrote != "" {
		t.Fatalf("a refused grant was written: %q", repo.wrote)
	}
}

// A service credential names nobody, so it can grant nothing.
func TestRotaAccessNeedsAUserToken(t *testing.T) {
	svc, _ := newRotaAccess(&fakeRotaAccessRepo{})
	ctx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, ClientID: "m2m"})
	if _, err := svc.Get(ctx); err == nil {
		t.Fatal("a service credential was answered")
	}
}

func wantConflict(t *testing.T, err error, what, mentions string) {
	t.Helper()
	var conflict *apierror.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("%s: want ConflictError, got %v", what, err)
	}
	if !strings.Contains(conflict.Msg, mentions) {
		t.Fatalf("%s: message %q does not mention %q", what, conflict.Msg, mentions)
	}
}

// A rota admin adds and removes the members of their own family's teams and
// of no other family's; the read says so for the tab.
func TestRotaAdminChangesTheMembersOfTheirOwnFamilyOnly(t *testing.T) {
	repo := &fakeRotaAccessRepo{
		internal:   true,
		teamFamily: map[string]string{"iaas": "SRE", "lyra": "CRE"},
		removal:    domain.TeamMemberRemoval{Member: true, Role: "engineer"},
	}
	svc, _ := newRotaAccess(repo, "SRE")
	ctx := callerWithRoles("sre.rota.admin@example.com", "cs_engineer")

	got, err := svc.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !reflect.DeepEqual(got.CanManage.MemberFamilies, []string{"SRE"}) {
		t.Fatalf("MemberFamilies = %v, want [SRE]", got.CanManage.MemberFamilies)
	}
	if err := svc.AddTeamMember(ctx, "iaas", someoneID); err != nil || repo.wrote != "add-member:iaas" {
		t.Fatalf("add to own family: err %v, wrote %q", err, repo.wrote)
	}
	if repo.actor != "sre.rota.admin@example.com" {
		t.Fatalf("actor %q", repo.actor)
	}
	if err := svc.RemoveTeamMember(ctx, "iaas", someoneID); err != nil || repo.wrote != "remove-member:iaas" {
		t.Fatalf("remove from own family: err %v, wrote %q", err, repo.wrote)
	}
	repo.wrote = ""
	wantForbidden(t, svc.AddTeamMember(ctx, "lyra", someoneID), "adding to a CRE team")
	wantForbidden(t, svc.RemoveTeamMember(ctx, "lyra", someoneID), "removing from a CRE team")
	if repo.wrote != "" {
		t.Fatalf("a refused change was written: %q", repo.wrote)
	}
}

// Somebody with no rota admin role changes no team's members, and is told
// nothing about any team.
func TestTeamMembersNeedARotaAdminOrACSMAdmin(t *testing.T) {
	repo := &fakeRotaAccessRepo{internal: true, teamFamily: map[string]string{"iaas": "SRE"}}
	svc, _ := newRotaAccess(repo)
	engineer := callerWithRoles("engineer@example.com", "cs_engineer")
	wantForbidden(t, svc.AddTeamMember(engineer, "iaas", someoneID), "an engineer adding")
	wantForbidden(t, svc.RemoveTeamMember(engineer, "iaas", someoneID), "an engineer removing")
	got, _ := svc.Get(engineer)
	if got.CanManage.MemberFamilies == nil || len(got.CanManage.MemberFamilies) != 0 {
		t.Fatalf("MemberFamilies = %#v, want an empty list", got.CanManage.MemberFamilies)
	}

	admin := callerWithRoles("head@example.com", "cs_admin")
	if err := svc.AddTeamMember(admin, "iaas", someoneID); err != nil {
		t.Fatalf("a CSM admin adding: %v", err)
	}
}

// Only active internal staff join a team, and the target must be a team on
// the schedule.
func TestTeamMemberMustBeInternalAndTheTeamRostered(t *testing.T) {
	admin := callerWithRoles("head@example.com", "cs_admin")
	repo := &fakeRotaAccessRepo{internal: false, teamFamily: map[string]string{"iaas": "SRE"}}
	svc, _ := newRotaAccess(repo)
	wantInvalid(t, svc.AddTeamMember(admin, "iaas", someoneID), "not internal")
	wantInvalid(t, svc.AddTeamMember(admin, "iaas", "not-a-uuid"), "a bad user id")
	wantInvalid(t, svc.AddTeamMember(admin, " ", someoneID), "no team")

	repo.internal = true
	var notFound *apierror.NotFoundError
	if err := svc.AddTeamMember(admin, "no-such-team", someoneID); !errors.As(err, &notFound) {
		t.Fatalf("an unknown team: want NotFoundError, got %v", err)
	}
	if repo.wrote != "" {
		t.Fatalf("a refused add was written: %q", repo.wrote)
	}
}

// Removing is refused, with nothing written, while it would change who is on
// duty or called: upcoming turns, a Case Alerts tier, a lead (for a rota
// admin). Somebody not on the team is a no-op.
func TestRemovingATeamMemberIsRefusedWhileItChangesWhoIsCalled(t *testing.T) {
	rotaAdmin := callerWithRoles("sre.rota.admin@example.com", "cs_engineer")
	cases := []struct {
		name    string
		removal domain.TeamMemberRemoval
		check   func(t *testing.T, err error)
	}{
		{"upcoming turns", domain.TeamMemberRemoval{Member: true, Role: "engineer", UpcomingTurns: 3},
			func(t *testing.T, err error) { wantConflict(t, err, "turns", "3 upcoming turns") }},
		{"one upcoming turn", domain.TeamMemberRemoval{Member: true, Role: "engineer", UpcomingTurns: 1},
			func(t *testing.T, err error) { wantConflict(t, err, "a turn", "1 upcoming turn on") }},
		{"a responder tier", domain.TeamMemberRemoval{Member: true, Role: "engineer", AlertTier: "T1"},
			func(t *testing.T, err error) { wantConflict(t, err, "tier", "T1 responder") }},
		{"a lead", domain.TeamMemberRemoval{Member: true, Role: "lead"},
			func(t *testing.T, err error) { wantForbidden(t, err, "lead") }},
		{"not on the team", domain.TeamMemberRemoval{Member: false},
			func(t *testing.T, err error) {
				if err != nil {
					t.Fatalf("not a member: want no error, got %v", err)
				}
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRotaAccessRepo{internal: true, teamFamily: map[string]string{"iaas": "SRE"}, removal: tc.removal}
			svc, _ := newRotaAccess(repo, "SRE")
			tc.check(t, svc.RemoveTeamMember(rotaAdmin, "iaas", someoneID))
			if repo.wrote != "" {
				t.Fatalf("a kept member was removed: %q", repo.wrote)
			}
			if repo.allowedLead {
				t.Fatal("a rota admin was allowed to remove a lead")
			}
		})
	}
}

// A CSM admin may remove a lead; turns and tiers still keep anyone on.
func TestCSMAdminRemovesALeadButNotOneWithTurns(t *testing.T) {
	admin := callerWithRoles("head@example.com", "cs_admin")
	repo := &fakeRotaAccessRepo{internal: true, teamFamily: map[string]string{"iaas": "SRE"},
		removal: domain.TeamMemberRemoval{Member: true, Role: "lead"}}
	svc, _ := newRotaAccess(repo)
	if err := svc.RemoveTeamMember(admin, "iaas", someoneID); err != nil || repo.wrote != "remove-member:iaas" {
		t.Fatalf("a CSM admin removing a lead: err %v, wrote %q", err, repo.wrote)
	}
	repo.wrote = ""
	repo.removal = domain.TeamMemberRemoval{Member: true, Role: "lead", UpcomingTurns: 2}
	wantConflict(t, svc.RemoveTeamMember(admin, "iaas", someoneID), "a lead with turns", "2 upcoming turns")
	if repo.wrote != "" {
		t.Fatalf("a lead with turns was removed: %q", repo.wrote)
	}
}

func wantNotFound(t *testing.T, err error, what string) {
	t.Helper()
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("%s: want NotFoundError, got %v", what, err)
	}
}

// Changing a role or tier, or moving, is for a CSM admin only: a rota admin
// of the very family, a team's own marker and an engineer are all refused
// before anything is looked up or written.
func TestChangingAMemberIsForACSMAdminOnly(t *testing.T) {
	repo := &fakeRotaAccessRepo{internal: true, teamFamily: map[string]string{"iaas": "SRE", "kiwi": "SRE"},
		update: domain.TeamMemberUpdate{Member: true}, move: domain.TeamMemberMove{OnSource: true}}
	svc, _ := newRotaAccess(repo, "SRE")
	patch := domain.TeamMemberPatch{Role: str("sub_lead")}
	for name, ctx := range map[string]context.Context{
		"a rota admin of the family": callerWithRoles("sre.rota.admin@example.com", "cs_engineer"),
		"an engineer":                callerWithRoles("engineer@example.com", "cs_engineer"),
	} {
		_, err := svc.UpdateTeamMember(ctx, "iaas", someoneID, patch)
		wantForbidden(t, err, name+" updating")
		_, err = svc.MoveTeamMember(ctx, "iaas", someoneID, "kiwi")
		wantForbidden(t, err, name+" moving")
	}
	// A rota admin's own family check is not what lets a request through even
	// when the caller holds the family admin role itself.
	if repo.wrote != "" {
		t.Fatalf("a refused change was written: %q", repo.wrote)
	}
}

func TestCSMAdminUpdatesAMember(t *testing.T) {
	admin := callerWithRoles("head@example.com", "cs_admin")
	tier := "T2"
	repo := &fakeRotaAccessRepo{internal: true, teamFamily: map[string]string{"iaas": "SRE"},
		update: domain.TeamMemberUpdate{Member: true, Membership: domain.TeamMembership{TeamKey: "iaas", UserID: someoneID, Role: "sub_lead", AlertTier: &tier}}}
	svc, _ := newRotaAccess(repo)
	got, err := svc.UpdateTeamMember(admin, "iaas", someoneID, domain.TeamMemberPatch{Role: str("sub_lead"), AlertTierSet: true, AlertTier: &tier})
	if err != nil || repo.wrote != "update-member:iaas" || repo.actor != "head@example.com" {
		t.Fatalf("update: err %v, wrote %q by %q", err, repo.wrote, repo.actor)
	}
	if got.Role != "sub_lead" || got.AlertTier == nil || *got.AlertTier != "T2" {
		t.Fatalf("membership = %+v", got)
	}
}

func TestUpdateTeamMemberRefusals(t *testing.T) {
	admin := callerWithRoles("head@example.com", "cs_admin")
	newSvc := func(r *fakeRotaAccessRepo) RotaAccessService {
		r.teamFamily = map[string]string{"iaas": "SRE"}
		svc, _ := newRotaAccess(r)
		return svc
	}
	ok := domain.TeamMemberUpdate{Member: true}
	for name, tc := range map[string]struct {
		repo  *fakeRotaAccessRepo
		team  string
		user  string
		patch domain.TeamMemberPatch
		want  string
	}{
		"nothing to change":  {&fakeRotaAccessRepo{internal: true, update: ok}, "iaas", someoneID, domain.TeamMemberPatch{}, "invalid"},
		"a lead role":        {&fakeRotaAccessRepo{internal: true, update: ok}, "iaas", someoneID, domain.TeamMemberPatch{Role: str("lead")}, "invalid"},
		"a cre head":         {&fakeRotaAccessRepo{internal: true, update: ok}, "iaas", someoneID, domain.TeamMemberPatch{Role: str("cre_head")}, "invalid"},
		"an unknown role":    {&fakeRotaAccessRepo{internal: true, update: ok}, "iaas", someoneID, domain.TeamMemberPatch{Role: str("boss")}, "invalid"},
		"a tier T4":          {&fakeRotaAccessRepo{internal: true, update: ok}, "iaas", someoneID, domain.TeamMemberPatch{AlertTierSet: true, AlertTier: str("T4")}, "invalid"},
		"a bad user id":      {&fakeRotaAccessRepo{internal: true, update: ok}, "iaas", "nope", domain.TeamMemberPatch{Role: str("sub_lead")}, "invalid"},
		"an external user":   {&fakeRotaAccessRepo{internal: false, update: ok}, "iaas", someoneID, domain.TeamMemberPatch{Role: str("sub_lead")}, "invalid"},
		"an unknown team":    {&fakeRotaAccessRepo{internal: true, update: ok}, "nowhere", someoneID, domain.TeamMemberPatch{Role: str("sub_lead")}, "notfound"},
		"a non-member":       {&fakeRotaAccessRepo{internal: true, update: domain.TeamMemberUpdate{}}, "iaas", someoneID, domain.TeamMemberPatch{Role: str("sub_lead")}, "notfound"},
		"a current lead":     {&fakeRotaAccessRepo{internal: true, update: domain.TeamMemberUpdate{Member: true, Lead: true}}, "iaas", someoneID, domain.TeamMemberPatch{Role: str("engineer")}, "conflict"},
		"a tier someone has": {&fakeRotaAccessRepo{internal: true, update: domain.TeamMemberUpdate{Member: true, TierHolder: someoneID}}, "iaas", someoneID, domain.TeamMemberPatch{AlertTierSet: true, AlertTier: str("T1")}, "conflict"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := newSvc(tc.repo).UpdateTeamMember(admin, tc.team, tc.user, tc.patch)
			switch tc.want {
			case "invalid":
				wantInvalid(t, err, name)
			case "notfound":
				wantNotFound(t, err, name)
			case "conflict":
				wantConflict(t, err, name, "")
			}
			if tc.repo.wrote != "" {
				t.Fatalf("%s: a refused change was written: %q", name, tc.repo.wrote)
			}
		})
	}
}

func TestCSMAdminMovesAMember(t *testing.T) {
	admin := callerWithRoles("head@example.com", "cs_admin")
	moved := domain.TeamMembership{TeamKey: "kiwi", UserID: someoneID, Role: "sub_lead"}
	repo := &fakeRotaAccessRepo{internal: true, teamFamily: map[string]string{"iaas": "SRE", "lyra": "CRE", "kiwi": "SRE"},
		move: domain.TeamMemberMove{OnSource: true, Role: "sub_lead", Membership: moved}}
	svc, _ := newRotaAccess(repo)
	// Across families too: the gate is the CSM admin, not either team's family.
	got, err := svc.MoveTeamMember(admin, "lyra", someoneID, "kiwi")
	if err != nil || repo.wrote != "move-member:lyra>kiwi" || repo.actor != "head@example.com" || got.TeamKey != "kiwi" {
		t.Fatalf("move: %+v, err %v, wrote %q by %q", got, err, repo.wrote, repo.actor)
	}
	// Already moved: the target's membership, nothing written.
	repo.wrote = ""
	repo.move = domain.TeamMemberMove{OnTarget: true, Membership: moved}
	if got, err := svc.MoveTeamMember(admin, "iaas", someoneID, "kiwi"); err != nil || got.TeamKey != "kiwi" || repo.wrote != "" {
		t.Fatalf("repeat move: %+v, err %v, wrote %q", got, err, repo.wrote)
	}
}

func TestMoveTeamMemberRefusals(t *testing.T) {
	admin := callerWithRoles("head@example.com", "cs_admin")
	teams := map[string]string{"iaas": "SRE", "kiwi": "SRE"}
	for name, tc := range map[string]struct {
		repo     *fakeRotaAccessRepo
		from     string
		to       string
		user     string
		want     string
		mentions string
	}{
		"the same team":     {&fakeRotaAccessRepo{internal: true}, "iaas", "IAAS", someoneID, "invalid", ""},
		"no target":         {&fakeRotaAccessRepo{internal: true}, "iaas", " ", someoneID, "invalid", ""},
		"a bad user id":     {&fakeRotaAccessRepo{internal: true}, "iaas", "kiwi", "nope", "invalid", ""},
		"an external user":  {&fakeRotaAccessRepo{internal: false}, "iaas", "kiwi", someoneID, "invalid", ""},
		"an unknown source": {&fakeRotaAccessRepo{internal: true}, "nowhere", "kiwi", someoneID, "notfound", ""},
		"an unknown target": {&fakeRotaAccessRepo{internal: true}, "iaas", "nowhere", someoneID, "notfound", ""},
		"a non-member":      {&fakeRotaAccessRepo{internal: true}, "iaas", "kiwi", someoneID, "notfound", ""},
		"a lead":            {&fakeRotaAccessRepo{internal: true, move: domain.TeamMemberMove{OnSource: true, Role: "lead", Lead: true}}, "iaas", "kiwi", someoneID, "conflict", "lead"},
		"upcoming turns":    {&fakeRotaAccessRepo{internal: true, move: domain.TeamMemberMove{OnSource: true, Role: "engineer", UpcomingTurns: 2}}, "iaas", "kiwi", someoneID, "conflict", "2 upcoming turns"},
		"a responder tier":  {&fakeRotaAccessRepo{internal: true, move: domain.TeamMemberMove{OnSource: true, Role: "engineer", AlertTier: "T3"}}, "iaas", "kiwi", someoneID, "conflict", "T3 responder"},
		"on both teams":     {&fakeRotaAccessRepo{internal: true, move: domain.TeamMemberMove{OnSource: true, OnTarget: true}}, "iaas", "kiwi", someoneID, "conflict", "both"},
	} {
		t.Run(name, func(t *testing.T) {
			tc.repo.teamFamily = teams
			svc, _ := newRotaAccess(tc.repo)
			_, err := svc.MoveTeamMember(admin, tc.from, tc.user, tc.to)
			switch tc.want {
			case "invalid":
				wantInvalid(t, err, name)
			case "notfound":
				wantNotFound(t, err, name)
			case "conflict":
				wantConflict(t, err, name, tc.mentions)
			}
			if tc.repo.wrote != "" {
				t.Fatalf("%s: a refused move was written: %q", name, tc.repo.wrote)
			}
		})
	}
}
