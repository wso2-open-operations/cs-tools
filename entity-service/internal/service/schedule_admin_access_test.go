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
	"slices"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func adminCtx(email string, roles ...string) context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: email, Roles: roles})
}

// A CSM admin (an AUTH_ADMIN_ROLES role on the token) edits any team's rota
// without leading it or holding a rota admin role.
func TestCSMAdminEditsAnyTeamsRota(t *testing.T) {
	repo := &leadOf{team: ""}
	repo.byID = domain.ScheduleAssignment{
		ID:       "11111111-1111-1111-1111-111111111111",
		TeamKey:  "apollo",
		Engineer: domain.ScheduleEngineer{UserID: "22222222-2222-2222-2222-222222222222"},
	}
	svc := WithScheduleAdminRoles(NewScheduleService(repo, alwaysUnrestrictedAccess{}), []string{"cs_admin"})
	newUser := "33333333-3333-3333-3333-333333333333"

	if _, err := svc.UpdateAssignment(adminCtx("admin@example.com", "cs_admin"), repo.byID.ID, domain.UpdateScheduleAssignmentRequest{UserID: &newUser}); err != nil {
		t.Fatalf("a CSM admin was refused a team's rota: %v", err)
	}
	if !repo.called {
		t.Fatal("the write never reached the repository")
	}

	// The role must be the configured one, matched exactly.
	repo.called = false
	_, err := svc.UpdateAssignment(adminCtx("eng@example.com", "cs_engineer", "CS_ADMIN"), repo.byID.ID, domain.UpdateScheduleAssignmentRequest{UserID: &newUser})
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) || repo.called {
		t.Fatalf("a caller without the admin role edited the rota: err=%v called=%v", err, repo.called)
	}
}

// With no admin roles configured, a token role grants nothing.
func TestCSMAdminRoleIgnoredWhenNotConfigured(t *testing.T) {
	repo := &leadOf{team: ""}
	repo.byID = domain.ScheduleAssignment{ID: "11111111-1111-1111-1111-111111111111", TeamKey: "apollo"}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
	newUser := "33333333-3333-3333-3333-333333333333"
	_, err := svc.UpdateAssignment(adminCtx("admin@example.com", "cs_admin"), repo.byID.ID, domain.UpdateScheduleAssignmentRequest{UserID: &newUser})
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("want ForbiddenError with no admin roles configured, got %v", err)
	}
}

// The engineer check still applies to an admin: a row belongs to its team.
func TestCSMAdminStillCannotWriteAgainstSomebodyElsesTeam(t *testing.T) {
	repo := &leadOf{team: "", notOnTeam: true}
	svc := WithScheduleAdminRoles(NewScheduleService(repo, alwaysUnrestrictedAccess{}), []string{"cs_admin"})
	_, err := svc.CreateAssignment(adminCtx("admin@example.com", "cs_admin"), domain.CreateScheduleAssignmentRequest{
		UserID: "22222222-2222-2222-2222-222222222222", TeamKey: "apollo", ShiftCode: "SRE_DAY", RotaDate: "2026-10-01",
	})
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("want ForbiddenError for an engineer not on the team, got %v", err)
	}
}

// A CSM admin's editable teams are every team on the schedule, as full teams.
func TestCSMAdminLeadTeamsAreEveryTeam(t *testing.T) {
	repo := &fakeScheduleRepo{catalogue: domain.ScheduleCatalogue{Teams: []domain.ScheduleTeam{{Key: "vega"}, {Key: "apollo"}, {Key: ""}}}}
	svc := WithScheduleAdminRoles(NewScheduleService(repo, alwaysUnrestrictedAccess{}), []string{"cs_admin"})
	got, err := svc.MyLeadTeams(adminCtx("admin@example.com", "cs_admin"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.TeamKeys, []string{"apollo", "vega"}) || len(got.MarkerOnlyTeamKeys) != 0 {
		t.Fatalf("a CSM admin's teams: %+v, want [apollo vega] and no marker-only teams", got)
	}

	// Nobody else gets the catalogue's teams.
	got, err = svc.MyLeadTeams(adminCtx("eng@example.com", "cs_engineer"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.TeamKeys) != 0 {
		t.Fatalf("an engineer got teams %v", got.TeamKeys)
	}
}

// A CSM admin may add a tag (a shared one: they hold no rota admin family).
func TestCSMAdminMayAddATag(t *testing.T) {
	repo := &fakeScheduleRepo{}
	svc := WithScheduleAdminRoles(NewScheduleService(repo, alwaysUnrestrictedAccess{}), []string{"cs_admin"})
	_, err := svc.CreateAbsenceKind(adminCtx("admin@example.com", "cs_admin"), domain.CreateScheduleAbsenceKindRequest{
		Label: "Conference", ShortCode: "CONF", Bucket: "LEAVE", ColourToken: firstAbsenceColour(),
	})
	if err != nil {
		t.Fatalf("a CSM admin was refused adding a tag: %v", err)
	}
}

func firstAbsenceColour() string {
	for c, ok := range absenceKindColours {
		if ok {
			return c
		}
	}
	return ""
}
