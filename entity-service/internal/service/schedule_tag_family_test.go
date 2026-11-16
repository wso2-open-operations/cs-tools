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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func scheduleCaller(email string) context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: email})
}

// A rota admin reads every family, as everyone does; only their writes are
// limited to their own family.
func TestRotaAdminReads_EveryFamily(t *testing.T) {
	repo := &fakeScheduleRepo{
		adminFamilies: []string{"SME"},
		assignments:   []domain.ScheduleAssignment{{TeamKey: "vega"}, {TeamKey: "apollo"}, {TeamKey: "asgardeo"}},
		catalogue: domain.ScheduleCatalogue{
			Teams: []domain.ScheduleTeam{{Key: "vega", Family: "CRE"}, {Key: "apollo", Family: "SRE"}, {Key: "asgardeo", Family: "SME"}},
		},
	}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
	ctx := scheduleCaller("sme.rota.admin@example.com")

	cat, err := svc.Catalogue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Teams) != 3 {
		t.Errorf("teams = %+v, want all three families'", cat.Teams)
	}
	got, err := svc.SearchAssignments(ctx, domain.SearchScheduleAssignmentsRequest{From: "2026-10-01", To: "2026-10-31", Family: "CRE"})
	if err != nil {
		t.Fatalf("an SME rota admin reading CRE: %v", err)
	}
	if len(got.Assignments) != 3 {
		t.Errorf("assignments = %d, want all three", len(got.Assignments))
	}
}

func TestAbsenceKinds_ARotaAdminsAreTheirFamilys(t *testing.T) {
	req := domain.CreateScheduleAbsenceKindRequest{ShortCode: "TR", Label: "Training", Bucket: "ALLOCATION", ColourToken: "INT"}

	t.Run("a rota admin's new kind is their family's", func(t *testing.T) {
		repo := &fakeScheduleRepo{adminTeams: []string{"asgardeo"}, adminFamilies: []string{"SME"}}
		if _, err := NewScheduleService(repo, alwaysUnrestrictedAccess{}).CreateAbsenceKind(scheduleCaller("sme.rota.admin@example.com"), req); err != nil {
			t.Fatal(err)
		}
		if repo.gotKindFamily == nil || *repo.gotKindFamily != "SME" {
			t.Errorf("family = %v, want SME", repo.gotKindFamily)
		}
	})
	t.Run("a lead's new kind is the family of the team they lead, not shared", func(t *testing.T) {
		repo := &fakeScheduleRepo{leadsTeam: true, leadFamilies: []string{"CRE"}}
		other := req
		other.Family = "SRE" // what the form sends is not theirs to pick
		_, err := NewScheduleService(repo, alwaysUnrestrictedAccess{}).CreateAbsenceKind(scheduleCaller("cre.abt.lead@example.com"), other)
		var forbidden *apierror.ForbiddenError
		if !errors.As(err, &forbidden) || repo.called {
			t.Fatalf("a CRE lead adding an SRE tag: want ForbiddenError and no write, got %v", err)
		}
		if _, err := NewScheduleService(repo, alwaysUnrestrictedAccess{}).CreateAbsenceKind(scheduleCaller("cre.abt.lead@example.com"), req); err != nil {
			t.Fatal(err)
		}
		if repo.gotKindFamily == nil || *repo.gotKindFamily != "CRE" {
			t.Errorf("family = %v, want CRE", repo.gotKindFamily)
		}
	})
	t.Run("a rota admin cannot add another family's kind", func(t *testing.T) {
		repo := &fakeScheduleRepo{adminTeams: []string{"asgardeo"}, adminFamilies: []string{"SME"}}
		other := req
		other.Family = "CRE"
		_, err := NewScheduleService(repo, alwaysUnrestrictedAccess{}).CreateAbsenceKind(scheduleCaller("sme.rota.admin@example.com"), other)
		var forbidden *apierror.ForbiddenError
		if !errors.As(err, &forbidden) || repo.called {
			t.Fatalf("want ForbiddenError and no write, got %v", err)
		}
	})
	t.Run("an admin of two families must say which", func(t *testing.T) {
		repo := &fakeScheduleRepo{adminTeams: []string{"asgardeo", "apollo"}, adminFamilies: []string{"SME", "SRE"}}
		_, err := NewScheduleService(repo, alwaysUnrestrictedAccess{}).CreateAbsenceKind(scheduleCaller("two@example.com"), req)
		var invalid *apierror.ValidationError
		if !errors.As(err, &invalid) || repo.called {
			t.Fatalf("want ValidationError and no write, got %v", err)
		}
	})
	t.Run("delete passes the admin's families, and nothing for a lead", func(t *testing.T) {
		repo := &fakeScheduleRepo{adminTeams: []string{"asgardeo"}, adminFamilies: []string{"SME"}}
		if err := NewScheduleService(repo, alwaysUnrestrictedAccess{}).DeleteAbsenceKind(scheduleCaller("sme.rota.admin@example.com"), "SME_TRAINING"); err != nil {
			t.Fatal(err)
		}
		if len(repo.gotKindAllowed) != 1 || repo.gotKindAllowed[0] != "SME" {
			t.Errorf("allowed families = %v, want [SME]", repo.gotKindAllowed)
		}
		lead := &fakeScheduleRepo{leadsTeam: true, leadFamilies: []string{"CRE"}}
		if err := NewScheduleService(lead, alwaysUnrestrictedAccess{}).DeleteAbsenceKind(scheduleCaller("lead@example.com"), "SME_TRAINING"); err != nil {
			t.Fatal(err)
		}
		if len(lead.gotKindAllowed) != 1 || lead.gotKindAllowed[0] != "CRE" {
			t.Errorf("allowed families for a CRE lead = %v, want [CRE]", lead.gotKindAllowed)
		}
	})
}

func TestPortalAdminTagFamily(t *testing.T) {
	for in, want := range map[string]string{"sre": "SRE", " CRE ": "CRE", "SME": "SME", "": ""} {
		got, err := portalAdminTagFamily(in)
		if err != nil {
			t.Fatalf("portalAdminTagFamily(%q): %v", in, err)
		}
		if want == "" && got != nil {
			t.Errorf("portalAdminTagFamily(%q) = %v, want none (shared)", in, got)
		}
		if want != "" && (len(got) != 1 || got[0] != want) {
			t.Errorf("portalAdminTagFamily(%q) = %v, want [%s]", in, got, want)
		}
	}
	// A mistyped family is refused, never turned into a shared tag.
	for _, in := range []string{"OPS", "SRF"} {
		var invalid *apierror.ValidationError
		if got, err := portalAdminTagFamily(in); !errors.As(err, &invalid) || got != nil {
			t.Errorf("portalAdminTagFamily(%q) = %v, %v; want a ValidationError", in, got, err)
		}
	}
}

// A portal admin by the database admin role alone (no AUTH_ADMIN_ROLES role on
// the token) is admitted by requireAnyTeamLead, has no family, and may still
// add a tag for a family and delete one.
func TestAbsenceKinds_ADatabasePortalAdminNeedsNoFamily(t *testing.T) {
	req := domain.CreateScheduleAbsenceKindRequest{ShortCode: "TR", Label: "TR", Bucket: "ALLOCATION", ColourToken: "INT", Family: "SRE"}
	repo := &fakeScheduleRepo{leadsTeam: true, portalAdmin: true}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
	if _, err := svc.CreateAbsenceKind(scheduleCaller("db.admin@example.com"), req); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.DeleteAbsenceKind(scheduleCaller("db.admin@example.com"), "SOME_TAG"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	req.Family = "SRF"
	var invalid *apierror.ValidationError
	if _, err := svc.CreateAbsenceKind(scheduleCaller("db.admin@example.com"), req); !errors.As(err, &invalid) {
		t.Fatalf("create for SRF: want ValidationError, got %v", err)
	}
}

// A portal admin who also leads a CRE team is not limited to CRE: they may add
// a tag for another family, add a shared one, and delete any family's tag, as
// before tags were family-scoped.
func TestAbsenceKinds_APortalAdminWhoLeadsIsNotLimitedToTheirFamily(t *testing.T) {
	req := domain.CreateScheduleAbsenceKindRequest{ShortCode: "TR", Label: "TR", Bucket: "ALLOCATION", ColourToken: "INT", Family: "SRE"}
	repo := &fakeScheduleRepo{leadsTeam: true, leadFamilies: []string{"CRE"}, portalAdmin: true}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
	if _, err := svc.CreateAbsenceKind(scheduleCaller("admin.lead@example.com"), req); err != nil {
		t.Fatalf("create for SRE: %v", err)
	}
	if repo.gotKindFamily == nil || *repo.gotKindFamily != "SRE" {
		t.Errorf("family = %v, want SRE", repo.gotKindFamily)
	}
	req.Family = ""
	if _, err := svc.CreateAbsenceKind(scheduleCaller("admin.lead@example.com"), req); err != nil {
		t.Fatalf("create shared: %v", err)
	}
	if repo.gotKindFamily != nil {
		t.Errorf("family = %v, want none (shared)", *repo.gotKindFamily)
	}
	if err := svc.DeleteAbsenceKind(scheduleCaller("admin.lead@example.com"), "SRE_TAG"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if repo.gotKindAllowed != nil {
		t.Errorf("allowed families = %v, want none (any family)", repo.gotKindAllowed)
	}
}

// A lead of a team that works no rota (a change approval board, say) passes
// the "any lead" gate but has no family, so may neither add a tag -- for any
// family or shared -- nor delete one. Only a portal admin may without a family.
func TestAbsenceKinds_ALeadWithNoRotaFamilyIsRefused(t *testing.T) {
	req := domain.CreateScheduleAbsenceKindRequest{ShortCode: "TR", Label: "TR", Bucket: "ALLOCATION", ColourToken: "INT", Family: "SRE"}
	repo := &fakeScheduleRepo{leadsTeam: true}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
	var forbidden *apierror.ForbiddenError
	if _, err := svc.CreateAbsenceKind(scheduleCaller("board.lead@example.com"), req); !errors.As(err, &forbidden) || repo.called {
		t.Fatalf("create: want ForbiddenError and no write, got %v", err)
	}
	if err := svc.DeleteAbsenceKind(scheduleCaller("board.lead@example.com"), "SOME_TAG"); !errors.As(err, &forbidden) || repo.called {
		t.Fatalf("delete: want ForbiddenError and no write, got %v", err)
	}
}
