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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// fakeScheduleRepo records what the service asked it for, so a test can assert
// the request reached the repository unchanged — and returns fixed rows, since
// what the repository does with them is SQL's business, not the service's.
type fakeScheduleRepo struct {
	gotAssignmentReq domain.SearchScheduleAssignmentsRequest
	gotAbsenceReq    domain.SearchScheduleAbsencesRequest
	gotOnDutyAt      time.Time
	called           bool

	assignments []domain.ScheduleAssignment
	absences    []domain.ScheduleAbsence
	catalogue   domain.ScheduleCatalogue
	err         error

	// lead edit
	leadsTeam   bool
	byID        domain.ScheduleAssignment
	created     domain.CreateScheduleAssignmentRequest
	updated     domain.UpdateScheduleAssignmentRequest
	deletedID   string
	gotActorEml string
	absenceByID domain.ScheduleAbsence
	gotKindCode string
	gotRange    domain.ApplyScheduleRangeRequest
	adminTeams  []string

	// Per-team answers, when set: which teams the caller leads, which the
	// engineer belongs to, and a span that moved the engineer to movedTo
	// from movedHome. Unset, the blanket answers above apply.
	ledTeams   []string
	memberOf   []string
	movedTo    string
	movedHome  string
	gotAbsence domain.ApplyScheduleAbsenceRequest
}

func (f *fakeScheduleRepo) AssignmentByID(context.Context, string) (domain.ScheduleAssignment, error) {
	if f.err != nil {
		return domain.ScheduleAssignment{}, f.err
	}
	return f.byID, nil
}

func (f *fakeScheduleRepo) LeadsTeam(_ context.Context, _ string, teamKey string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	if f.ledTeams != nil {
		return containsFold(f.ledTeams, teamKey), nil
	}
	return f.leadsTeam, nil
}

func (f *fakeScheduleRepo) CreateAssignment(_ context.Context, req domain.CreateScheduleAssignmentRequest, actor string) (domain.ScheduleAssignment, error) {
	f.called, f.created, f.gotActorEml = true, req, actor
	return f.byID, f.err
}

func (f *fakeScheduleRepo) UpdateAssignment(_ context.Context, _ string, req domain.UpdateScheduleAssignmentRequest, actor string) (domain.ScheduleAssignment, error) {
	f.called, f.updated, f.gotActorEml = true, req, actor
	return f.byID, f.err
}

func (f *fakeScheduleRepo) DeleteAssignment(_ context.Context, id, actor string, _ *string) error {
	f.called, f.deletedID, f.gotActorEml = true, id, actor
	return f.err
}

func (f *fakeScheduleRepo) ActivityForTeam(context.Context, string, string, string) ([]domain.ScheduleAssignmentActivity, error) {
	return nil, f.err
}

func (f *fakeScheduleRepo) ApplyRange(_ context.Context, req domain.ApplyScheduleRangeRequest, actor string) (domain.ApplyScheduleRangeResponse, error) {
	f.called, f.gotActorEml, f.gotRange = true, actor, req
	return domain.ApplyScheduleRangeResponse{Applied: 1, SkippedDates: []string{}}, f.err
}

func (f *fakeScheduleRepo) ApplyAbsence(_ context.Context, req domain.ApplyScheduleAbsenceRequest, actor string) (domain.ApplyScheduleAbsenceResponse, error) {
	f.called, f.gotActorEml, f.gotAbsence = true, actor, req
	return domain.ApplyScheduleAbsenceResponse{Created: 1}, f.err
}

func (f *fakeScheduleRepo) AbsenceByID(_ context.Context, id string) (domain.ScheduleAbsence, error) {
	if f.err != nil {
		return domain.ScheduleAbsence{}, f.err
	}
	a := f.absenceByID
	if a.ID == "" {
		a.ID = id
	}
	return a, nil
}

func (f *fakeScheduleRepo) DeleteAbsence(_ context.Context, id, actor string, _ *string) error {
	f.called, f.deletedID, f.gotActorEml = true, id, actor
	return f.err
}

func (f *fakeScheduleRepo) CreateAbsenceKind(_ context.Context, code string, req domain.CreateScheduleAbsenceKindRequest, actor string) (domain.ScheduleAbsenceKind, error) {
	f.called, f.gotKindCode, f.gotActorEml = true, code, actor
	return domain.ScheduleAbsenceKind{Code: code, ShortCode: req.ShortCode, Label: req.Label, Bucket: req.Bucket, ColourToken: req.ColourToken}, f.err
}

func (f *fakeScheduleRepo) DeleteAbsenceKind(_ context.Context, code, actor string) error {
	f.called, f.gotKindCode, f.gotActorEml = true, code, actor
	return f.err
}

func (f *fakeScheduleRepo) EditMarkers(context.Context, string, string) ([]domain.ScheduleEditMarker, error) {
	return nil, f.err
}

func (f *fakeScheduleRepo) UserInTeam(_ context.Context, _ string, teamKey string) (bool, error) {
	if f.memberOf != nil {
		return containsFold(f.memberOf, teamKey), f.err
	}
	return true, f.err
}

func (f *fakeScheduleRepo) MovedToTeamOver(_ context.Context, _ string, teamKey, _, _ string) (string, bool, error) {
	if f.movedTo != "" && strings.EqualFold(f.movedTo, teamKey) {
		return f.movedHome, true, f.err
	}
	return "", false, f.err
}

func (f *fakeScheduleRepo) LeadTeamsFor(context.Context, string) ([]string, error) {
	if f.leadsTeam {
		return []string{"castor"}, f.err
	}
	return []string{}, f.err
}

// adminTeams is what a rota admin holds. Nil by default, so every existing
// test describes somebody who is a lead or nothing at all -- the behaviour
// before the role existed.
func (f *fakeScheduleRepo) RotaAdminTeamsFor(context.Context, string) ([]string, error) {
	if f.adminTeams == nil {
		return []string{}, f.err
	}
	return f.adminTeams, f.err
}

func (f *fakeScheduleRepo) Catalogue(context.Context) (domain.ScheduleCatalogue, error) {
	f.called = true
	return f.catalogue, f.err
}

func (f *fakeScheduleRepo) SearchAssignments(_ context.Context, req domain.SearchScheduleAssignmentsRequest) ([]domain.ScheduleAssignment, error) {
	f.called = true
	f.gotAssignmentReq = req
	return f.assignments, f.err
}

func (f *fakeScheduleRepo) SearchAbsences(_ context.Context, req domain.SearchScheduleAbsencesRequest) ([]domain.ScheduleAbsence, error) {
	f.called = true
	f.gotAbsenceReq = req
	return f.absences, f.err
}

func (f *fakeScheduleRepo) OnDutyAt(_ context.Context, at time.Time) ([]domain.ScheduleAssignment, error) {
	f.called = true
	f.gotOnDutyAt = at
	return f.assignments, f.err
}

func TestSearchAssignmentsRejectsBadWindows(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		req  domain.SearchScheduleAssignmentsRequest
	}{
		{"no dates at all", domain.SearchScheduleAssignmentsRequest{}},
		{"from missing", domain.SearchScheduleAssignmentsRequest{To: "2026-09-21"}},
		{"to missing", domain.SearchScheduleAssignmentsRequest{From: "2026-09-21"}},
		{"from not a date", domain.SearchScheduleAssignmentsRequest{From: "yesterday", To: "2026-09-21"}},
		{"to not a date", domain.SearchScheduleAssignmentsRequest{From: "2026-09-21", To: "soon"}},
		{"to before from", domain.SearchScheduleAssignmentsRequest{From: "2026-09-21", To: "2026-09-20"}},
		{"window too wide", domain.SearchScheduleAssignmentsRequest{From: "2026-01-01", To: "2026-12-31"}},
		{"family is not a rota family", domain.SearchScheduleAssignmentsRequest{From: "2026-09-21", To: "2026-09-21", Family: "OPS"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := &fakeScheduleRepo{}
			_, err := NewScheduleService(repo, alwaysUnrestrictedAccess{}).SearchAssignments(context.Background(), tc.req)

			var validation *apierror.ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("want a ValidationError, got %v", err)
			}
			// A rejected request must not reach the database: an unbounded rota
			// read would happily return every row in the table.
			if repo.called {
				t.Fatal("repository was called for a request that failed validation")
			}
		})
	}
}

// Every rota family reaches the repository: CRE and SRE as before, and SME,
// the product special rotations added in migration 0199. An unknown family is
// still refused (TestSearchAssignmentsRejectsBadWindows).
func TestSearchAssignmentsAcceptsEveryRotaFamily(t *testing.T) {
	t.Parallel()

	for _, family := range []string{"", "CRE", "SRE", "SME"} {
		t.Run("family "+family, func(t *testing.T) {
			t.Parallel()
			repo := &fakeScheduleRepo{}
			req := domain.SearchScheduleAssignmentsRequest{From: "2026-10-05", To: "2026-10-05", Family: family}
			if _, err := NewScheduleService(repo, alwaysUnrestrictedAccess{}).SearchAssignments(context.Background(), req); err != nil {
				t.Fatalf("family %q: unexpected error: %v", family, err)
			}
			if !repo.called {
				t.Fatalf("family %q: the repository was not asked", family)
			}
		})
	}
}

func TestSearchAssignmentsPassesTheRequestThrough(t *testing.T) {
	t.Parallel()

	repo := &fakeScheduleRepo{assignments: []domain.ScheduleAssignment{{ID: "a"}, {ID: "b"}}}
	req := domain.SearchScheduleAssignmentsRequest{
		From:             "2026-09-21",
		To:               "2026-09-27",
		Family:           "SRE",
		TeamKeys:         []string{"apollo"},
		UserEmail:        "sre.01@example.com",
		IncludeOvernight: true,
	}

	resp, err := NewScheduleService(repo, alwaysUnrestrictedAccess{}).SearchAssignments(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Count != 2 || len(resp.Assignments) != 2 {
		t.Fatalf("want 2 assignments counted, got %d/%d", resp.Count, len(resp.Assignments))
	}
	if !reflect.DeepEqual(repo.gotAssignmentReq, req) {
		t.Fatalf("the repository saw a different request:\n got %+v\nwant %+v", repo.gotAssignmentReq, req)
	}
}

func TestSearchAssignmentsAcceptsAWindowAtTheLimit(t *testing.T) {
	t.Parallel()

	// Exactly maxScheduleWindowDays apart is allowed; one day more is not.
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	ok := from.AddDate(0, 0, maxScheduleWindowDays).Format("2006-01-02")
	tooFar := from.AddDate(0, 0, maxScheduleWindowDays+1).Format("2006-01-02")

	svc := NewScheduleService(&fakeScheduleRepo{}, alwaysUnrestrictedAccess{})
	if _, err := svc.SearchAssignments(context.Background(), domain.SearchScheduleAssignmentsRequest{
		From: from.Format("2006-01-02"), To: ok,
	}); err != nil {
		t.Fatalf("a window of exactly the limit should be allowed, got %v", err)
	}
	if _, err := svc.SearchAssignments(context.Background(), domain.SearchScheduleAssignmentsRequest{
		From: from.Format("2006-01-02"), To: tooFar,
	}); err == nil {
		t.Fatal("a window one day past the limit should be rejected")
	}
}

func TestSearchAbsencesValidatesTheSameWindow(t *testing.T) {
	t.Parallel()

	repo := &fakeScheduleRepo{}
	_, err := NewScheduleService(repo, alwaysUnrestrictedAccess{}).SearchAbsences(context.Background(),
		domain.SearchScheduleAbsencesRequest{From: "2026-09-27", To: "2026-09-21"})

	var validation *apierror.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("want a ValidationError, got %v", err)
	}
	if repo.called {
		t.Fatal("repository was called for a request that failed validation")
	}
}

func TestOnDutyDefaultsToNow(t *testing.T) {
	t.Parallel()

	repo := &fakeScheduleRepo{}
	before := time.Now()
	if _, err := NewScheduleService(repo, alwaysUnrestrictedAccess{}).OnDuty(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.gotOnDutyAt.Before(before) || repo.gotOnDutyAt.After(time.Now()) {
		t.Fatalf("want the lookup pinned to now, got %v", repo.gotOnDutyAt)
	}
}

func TestOnDutyUsesTheInstantAskedFor(t *testing.T) {
	t.Parallel()

	repo := &fakeScheduleRepo{}
	at := time.Date(2026, 9, 21, 22, 15, 0, 0, time.UTC)
	if _, err := NewScheduleService(repo, alwaysUnrestrictedAccess{}).OnDuty(context.Background(), &at); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.gotOnDutyAt.Equal(at) {
		t.Fatalf("want %v, got %v", at, repo.gotOnDutyAt)
	}
}

func TestCatalogueIsServedStraightThrough(t *testing.T) {
	t.Parallel()

	repo := &fakeScheduleRepo{catalogue: domain.ScheduleCatalogue{
		Zones:  []domain.ScheduleZone{{Code: "TZ1"}},
		Shifts: []domain.ScheduleShift{{Code: "CRE_REGULAR"}},
	}}
	cat, err := NewScheduleService(repo, alwaysUnrestrictedAccess{}).Catalogue(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cat.Zones) != 1 || len(cat.Shifts) != 1 {
		t.Fatalf("catalogue did not survive the round trip: %+v", cat)
	}
}

// The rota is staff data with no project to scope it by, so a caller who is
// not internal -- a customer's own token -- is refused on every read, before
// the repository is reached.
func TestScheduleReadsRequireInternalCaller(t *testing.T) {
	t.Parallel()

	window := domain.SearchScheduleAssignmentsRequest{From: "2026-09-21", To: "2026-09-27"}
	reads := map[string]func(ScheduleService) error{
		"catalogue": func(s ScheduleService) error {
			_, err := s.Catalogue(context.Background())
			return err
		},
		"assignments": func(s ScheduleService) error {
			_, err := s.SearchAssignments(context.Background(), window)
			return err
		},
		"absences": func(s ScheduleService) error {
			_, err := s.SearchAbsences(context.Background(),
				domain.SearchScheduleAbsencesRequest{From: window.From, To: window.To})
			return err
		},
		"on-duty": func(s ScheduleService) error {
			_, err := s.OnDuty(context.Background(), nil)
			return err
		},
	}

	for name, read := range reads {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := &fakeScheduleRepo{}
			err := read(NewScheduleService(repo, restrictedAccess{}))

			var forbidden *apierror.ForbiddenError
			if !errors.As(err, &forbidden) {
				t.Fatalf("want a ForbiddenError, got %v", err)
			}
			if repo.called {
				t.Fatal("repository was called for a non-internal caller")
			}
		})
	}
}

// A malformed userId is the caller's mistake, so it is a 400 here rather than
// the 500 Postgres would produce casting it into a uuid column.
func TestScheduleSearchesRejectMalformedUserID(t *testing.T) {
	t.Parallel()

	repo := &fakeScheduleRepo{}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})

	_, errA := svc.SearchAssignments(context.Background(),
		domain.SearchScheduleAssignmentsRequest{From: "2026-09-21", To: "2026-09-27", UserID: "not-a-uuid"})
	_, errB := svc.SearchAbsences(context.Background(),
		domain.SearchScheduleAbsencesRequest{From: "2026-09-21", To: "2026-09-27", UserID: "42"})

	for _, err := range []error{errA, errB} {
		var validation *apierror.ValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("want a ValidationError, got %v", err)
		}
	}
	if repo.called {
		t.Fatal("repository was called with a malformed userId")
	}

	if _, err := svc.SearchAssignments(context.Background(), domain.SearchScheduleAssignmentsRequest{
		From: "2026-09-21", To: "2026-09-27", UserID: "3f2b8c1e-9d4a-4e7b-a1c2-5d6e7f8a9b0c",
	}); err != nil {
		t.Fatalf("a well-formed userId was refused: %v", err)
	}
}

// leadOf answers "is this the team you lead", so a test can hand the service a
// repository that knows one lead relationship and nothing else.
type leadOf struct {
	fakeScheduleRepo
	team string
	// notOnTeam makes the engineer being changed a member of some other team,
	// for the case where a lead names their own team and hands over somebody
	// else's id.
	notOnTeam bool
}

func (l *leadOf) LeadsTeam(_ context.Context, _, teamKey string) (bool, error) {
	return teamKey == l.team, nil
}

func (l *leadOf) UserInTeam(context.Context, string, string) (bool, error) {
	return !l.notOnTeam, nil
}

func leadCtx(email string) context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: email})
}

// A lead may change their own team's rota and nobody else's. This is the whole
// of the edit permission, so it is worth testing from both sides rather than
// only the happy one.
func TestEditsAreLimitedToTheCallersOwnTeam(t *testing.T) {
	repo := &leadOf{team: "castor"}
	repo.byID = domain.ScheduleAssignment{
		ID:       "11111111-1111-1111-1111-111111111111",
		TeamKey:  "draco", // a team this caller does not lead
		Engineer: domain.ScheduleEngineer{UserID: "22222222-2222-2222-2222-222222222222"},
	}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
	ctx := leadCtx("castor.01@example.com")

	newUser := "33333333-3333-3333-3333-333333333333"
	if _, err := svc.UpdateAssignment(ctx, repo.byID.ID, domain.UpdateScheduleAssignmentRequest{UserID: &newUser}); err == nil {
		t.Fatal("a Castor lead was allowed to edit a Draco slot")
	} else {
		var forbidden *apierror.ForbiddenError
		if !errors.As(err, &forbidden) {
			t.Fatalf("want ForbiddenError, got %v", err)
		}
	}
	if repo.called {
		t.Fatal("the repository was written to despite the caller not leading the team")
	}

	// The same lead, on their own team, is allowed through.
	repo.byID.TeamKey = "castor"
	if _, err := svc.UpdateAssignment(ctx, repo.byID.ID, domain.UpdateScheduleAssignmentRequest{UserID: &newUser}); err != nil {
		t.Fatalf("a Castor lead was refused their own team: %v", err)
	}
	if !repo.called {
		t.Fatal("the write never reached the repository")
	}
}

// A rota admin may edit a team they do not lead -- but only inside their own
// family. Both halves matter: the first is the whole point of the role, and
// the second is the only thing stopping it from being a key to everything.
func TestRotaAdminMayEditTheirOwnFamilyAndNoOther(t *testing.T) {
	// Leads nothing at all. Every permission this caller has comes from the
	// role, which is exactly the case the role exists for.
	repo := &leadOf{team: ""}
	repo.adminTeams = []string{"castor", "draco", "vega"} // the CRE teams
	repo.byID = domain.ScheduleAssignment{
		ID:       "11111111-1111-1111-1111-111111111111",
		TeamKey:  "draco",
		Engineer: domain.ScheduleEngineer{UserID: "22222222-2222-2222-2222-222222222222"},
	}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
	ctx := leadCtx("cre.rota.admin@example.com")

	newUser := "33333333-3333-3333-3333-333333333333"
	if _, err := svc.UpdateAssignment(ctx, repo.byID.ID, domain.UpdateScheduleAssignmentRequest{UserID: &newUser}); err != nil {
		t.Fatalf("a CRE rota admin was refused a CRE team they do not lead: %v", err)
	}
	if !repo.called {
		t.Fatal("the write never reached the repository")
	}

	// An SRE team is not theirs, and holding the CRE role says nothing about
	// it. This is the refusal the family split is for.
	repo.called = false
	repo.byID.TeamKey = "apollo"
	_, err := svc.UpdateAssignment(ctx, repo.byID.ID, domain.UpdateScheduleAssignmentRequest{UserID: &newUser})
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("want ForbiddenError editing the other family, got %v", err)
	}
	if repo.called {
		t.Fatal("a CRE rota admin wrote to an SRE team")
	}
}

// A team key arrives from a request body, where nothing lowercases it, while
// the repository returns it as stored. An admin refused their own team over
// capitalisation would be a maddening bug to report, and LeadsTeam already
// folds case in SQL -- this check has to agree with it.
func TestRotaAdminTeamMatchIgnoresCase(t *testing.T) {
	repo := &leadOf{team: ""}
	repo.adminTeams = []string{"castor"}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})

	_, err := svc.CreateAssignment(leadCtx("cre.rota.admin@example.com"), domain.CreateScheduleAssignmentRequest{
		UserID:    "22222222-2222-2222-2222-222222222222",
		TeamKey:   "CASTOR",
		ShiftCode: "CRE_EVENING",
		RotaDate:  "2026-10-01",
	})
	if err != nil {
		t.Fatalf("a rota admin was refused their own team over its spelling: %v", err)
	}
}

// The engineer check is not waived for an admin. Their reach is over teams,
// not over people: a row still belongs to the team it is filed under, and
// ApplyRange's delete is scoped by team_key on the strength of that.
func TestRotaAdminStillCannotWriteAgainstSomebodyElsesTeam(t *testing.T) {
	repo := &leadOf{team: "", notOnTeam: true}
	repo.adminTeams = []string{"castor"}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})

	_, err := svc.ApplyRange(leadCtx("cre.rota.admin@example.com"), domain.ApplyScheduleRangeRequest{
		UserID:    "22222222-2222-2222-2222-222222222222",
		TeamKey:   "castor",
		ShiftCode: "CRE_EVENING",
		From:      "2026-10-01",
		To:        "2026-10-03",
	})
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("want ForbiddenError for an engineer not on the team, got %v", err)
	}
	if repo.called {
		t.Fatal("a rota admin wrote a row against somebody not on the team")
	}
}

// The endpoint the UI reads to decide which rows get an edit control has to
// answer for both routes at once, or the page and the permission disagree --
// an edit control that 403s, or a team quietly withheld from somebody who may
// in fact edit it.
func TestMyLeadTeamsCoversLeadingAndAdministering(t *testing.T) {
	repo := &fakeScheduleRepo{leadsTeam: true} // LeadTeamsFor returns castor
	repo.adminTeams = []string{"vega", "castor", "draco"}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})

	teams, err := svc.MyLeadTeams(leadCtx("cre.rota.admin@example.com"))
	if err != nil {
		t.Fatalf("MyLeadTeams: %v", err)
	}
	// castor is held both ways and must appear once, not twice.
	want := []string{"castor", "draco", "vega"}
	if !reflect.DeepEqual(teams, want) {
		t.Fatalf("MyLeadTeams = %v, want %v", teams, want)
	}
}

// The team is read from the row, not from the request. Otherwise a lead could
// name their own team and edit anybody's slot: the check would pass and the
// write would land somewhere else entirely.
func TestCreateChecksTheTeamItWasGiven(t *testing.T) {
	repo := &leadOf{team: "castor"}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})

	_, err := svc.CreateAssignment(leadCtx("castor.01@example.com"), domain.CreateScheduleAssignmentRequest{
		UserID:    "22222222-2222-2222-2222-222222222222",
		TeamKey:   "draco",
		ShiftCode: "CRE_EVENING",
		RotaDate:  "2026-10-01",
	})
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("want ForbiddenError creating into another team, got %v", err)
	}
	if repo.called {
		t.Fatal("the repository was written to for a team the caller does not lead")
	}
}

// A service credential has no user to be a lead of. Editing is a person's
// action, and a machine presenting a client credential is not one.
func TestEditsNeedAUserNotAServiceCredential(t *testing.T) {
	repo := &leadOf{team: "castor"}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})

	// Validated, unrestricted, but carrying no user email.
	ctx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, ClientID: "some-internal-service"})
	_, err := svc.CreateAssignment(ctx, domain.CreateScheduleAssignmentRequest{
		UserID:    "22222222-2222-2222-2222-222222222222",
		TeamKey:   "castor",
		ShiftCode: "CRE_EVENING",
		RotaDate:  "2026-10-01",
	})
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("want ForbiddenError for a service credential, got %v", err)
	}
}

// Marking somebody away is an edit to the rota like any other, so it is behind
// the same gate. Worth its own test because it reaches a different table and
// could easily have been wired up without one.
func TestMarkingLeaveIsLimitedToTheCallersOwnTeam(t *testing.T) {
	repo := &leadOf{team: "castor"}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
	ctx := leadCtx("castor.01@example.com")

	req := domain.ApplyScheduleAbsenceRequest{
		UserID:   "22222222-2222-2222-2222-222222222222",
		TeamKey:  "draco", // a team this caller does not lead
		KindCode: "ANNUAL_LEAVE",
		From:     "2026-09-23",
		To:       "2026-09-25",
	}
	if _, err := svc.ApplyAbsence(ctx, req); err == nil {
		t.Fatal("a Castor lead was allowed to book leave on a Draco engineer")
	} else {
		var forbidden *apierror.ForbiddenError
		if !errors.As(err, &forbidden) {
			t.Fatalf("want ForbiddenError, got %v", err)
		}
	}
	if repo.called {
		t.Fatal("the repository was written to despite the caller not leading the team")
	}

	req.TeamKey = "castor"
	if _, err := svc.ApplyAbsence(ctx, req); err != nil {
		t.Fatalf("a Castor lead was refused their own team: %v", err)
	}
	if !repo.called {
		t.Fatal("the write never reached the repository")
	}
}

// An empty kindCode is how the picker clears a span, so it must stay valid --
// while the fields that say who and when must not be droppable with it.
func TestClearingLeaveStillNeedsWhoAndWhen(t *testing.T) {
	repo := &leadOf{team: "castor"}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
	ctx := leadCtx("castor.01@example.com")

	clear := domain.ApplyScheduleAbsenceRequest{
		UserID:  "22222222-2222-2222-2222-222222222222",
		TeamKey: "castor",
		From:    "2026-09-23",
		To:      "2026-09-25",
	}
	if _, err := svc.ApplyAbsence(ctx, clear); err != nil {
		t.Fatalf("clearing a span was refused: %v", err)
	}

	repo.called = false
	noDates := clear
	noDates.From = ""
	if _, err := svc.ApplyAbsence(ctx, noDates); err == nil {
		t.Fatal("a span with no start was accepted")
	}
	if repo.called {
		t.Fatal("the repository was written to on an invalid request")
	}
}

// Leading a team decides what a lead may change, not whose rota they may
// change it on. The team key in the request is the lead's own here, so the
// lead check passes -- and the engineer named belongs to somebody else.
func TestALeadCannotEditAnEngineerFromAnotherTeam(t *testing.T) {
	repo := &leadOf{team: "castor", notOnTeam: true}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
	ctx := leadCtx("castor.01@example.com")

	stranger := "22222222-2222-2222-2222-222222222222"

	if _, err := svc.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: stranger, TeamKey: "castor", ShiftCode: "CRE_EVENING",
		From: "2026-09-21", To: "2026-09-21",
	}); err == nil {
		t.Fatal("a lead set a rotation on an engineer who is not on their team")
	} else {
		var forbidden *apierror.ForbiddenError
		if !errors.As(err, &forbidden) {
			t.Fatalf("want ForbiddenError, got %v", err)
		}
	}
	if repo.called {
		t.Fatal("the write reached the repository despite the engineer not being on the team")
	}

	if _, err := svc.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: stranger, TeamKey: "castor", KindCode: "ANNUAL_LEAVE",
		From: "2026-09-21", To: "2026-09-21",
	}); err == nil {
		t.Fatal("a lead booked leave for an engineer who is not on their team")
	}
	if repo.called {
		t.Fatal("the absence write reached the repository")
	}

	// The same lead, on somebody who is on their team, is allowed through.
	repo.notOnTeam = false
	if _, err := svc.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: stranger, TeamKey: "castor", ShiftCode: "CRE_EVENING",
		From: "2026-09-21", To: "2026-09-21",
	}); err != nil {
		t.Fatalf("a lead was refused an engineer on their own team: %v", err)
	}
	if !repo.called {
		t.Fatal("the write never reached the repository")
	}
}

// Removing an absence is gated on the team the absence belongs to, read from
// the row, not on anything the caller says.
func TestRemovingAnAbsenceIsLimitedToTheCallersOwnTeam(t *testing.T) {
	repo := &leadOf{team: "castor"}
	repo.absenceByID = domain.ScheduleAbsence{
		TeamKey:  "draco",
		Engineer: domain.ScheduleEngineer{UserID: "22222222-2222-2222-2222-222222222222"},
	}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
	ctx := leadCtx("castor.01@example.com")
	id := "33333333-3333-3333-3333-333333333333"

	err := svc.DeleteAbsence(ctx, id, nil)
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("a Castor lead removing Draco leave: want ForbiddenError, got %v", err)
	}
	if repo.called {
		t.Fatal("the absence was deleted despite the caller not leading its team")
	}

	repo.absenceByID.TeamKey = "castor"
	if err := svc.DeleteAbsence(ctx, id, nil); err != nil {
		t.Fatalf("a Castor lead was refused their own team's leave: %v", err)
	}
	if repo.deletedID != id {
		t.Fatalf("deleted %q, want %q", repo.deletedID, id)
	}
}

func TestRemovingAnAbsenceNeedsAUUID(t *testing.T) {
	repo := &leadOf{team: "castor"}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
	var invalid *apierror.ValidationError
	if err := svc.DeleteAbsence(leadCtx("castor.01@example.com"), "not-a-uuid", nil); !errors.As(err, &invalid) {
		t.Fatalf("want ValidationError, got %v", err)
	}
}

func TestANewTagIsDerivedAndChecked(t *testing.T) {
	ok := domain.CreateScheduleAbsenceKindRequest{
		ShortCode: " Trn ", Label: "Training — external", Bucket: "allocation", ColourToken: "int",
	}

	t.Run("a lead's tag gets a code from its label", func(t *testing.T) {
		repo := &fakeScheduleRepo{leadsTeam: true}
		svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
		k, err := svc.CreateAbsenceKind(leadCtx("castor.01@example.com"), ok)
		if err != nil {
			t.Fatalf("CreateAbsenceKind: %v", err)
		}
		if repo.gotKindCode != "TRAINING_EXTERNAL" {
			t.Fatalf("code %q, want TRAINING_EXTERNAL", repo.gotKindCode)
		}
		if k.ShortCode != "Trn" || k.Bucket != "ALLOCATION" || k.ColourToken != "INT" {
			t.Fatalf("stored %+v, want the fields trimmed and upper-cased", k)
		}
	})

	t.Run("somebody who leads nothing is refused", func(t *testing.T) {
		repo := &fakeScheduleRepo{leadsTeam: false}
		svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
		var forbidden *apierror.ForbiddenError
		if _, err := svc.CreateAbsenceKind(leadCtx("engineer@example.com"), ok); !errors.As(err, &forbidden) {
			t.Fatalf("want ForbiddenError, got %v", err)
		}
		if repo.called {
			t.Fatal("the catalogue was written to by a non-lead")
		}
	})

	bad := map[string]func(r *domain.CreateScheduleAbsenceKindRequest){
		"no short code":             func(r *domain.CreateScheduleAbsenceKindRequest) { r.ShortCode = " " },
		"short code too long":       func(r *domain.CreateScheduleAbsenceKindRequest) { r.ShortCode = "ABCDEFGHIJKLM" },
		"no label":                  func(r *domain.CreateScheduleAbsenceKindRequest) { r.Label = "" },
		"label with no letters":     func(r *domain.CreateScheduleAbsenceKindRequest) { r.Label = "— —" },
		"excluded is not for leads": func(r *domain.CreateScheduleAbsenceKindRequest) { r.Bucket = "EXCLUDED" },
		"a colour the rota lacks":   func(r *domain.CreateScheduleAbsenceKindRequest) { r.ColourToken = "TZ1" },
	}
	for name, mutate := range bad {
		t.Run(name, func(t *testing.T) {
			repo := &fakeScheduleRepo{leadsTeam: true}
			svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
			req := ok
			mutate(&req)
			var invalid *apierror.ValidationError
			if _, err := svc.CreateAbsenceKind(leadCtx("castor.01@example.com"), req); !errors.As(err, &invalid) {
				t.Fatalf("want ValidationError, got %v", err)
			}
			if repo.called {
				t.Fatal("an invalid tag reached the repository")
			}
		})
	}
}

func TestApplyRangeChecksTheTier(t *testing.T) {
	req := func(tier string) domain.ApplyScheduleRangeRequest {
		return domain.ApplyScheduleRangeRequest{
			UserID: "22222222-2222-2222-2222-222222222222", TeamKey: "castor",
			ShiftCode: "SRE_TZ1", From: "2026-09-21", To: "2026-09-21", Tier: &tier,
		}
	}
	ctx := leadCtx("castor.01@example.com")

	repo := &leadOf{team: "castor"}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
	if _, err := svc.ApplyRange(ctx, req(" l3 ")); err != nil {
		t.Fatalf("L3: %v", err)
	}
	if repo.gotRange.Tier == nil || *repo.gotRange.Tier != "L3" {
		t.Fatalf("tier reached the repository as %v, want L3", repo.gotRange.Tier)
	}

	for _, bad := range []string{"L4", "tz1"} {
		repo := &leadOf{team: "castor"}
		svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
		var invalid *apierror.ValidationError
		if _, err := svc.ApplyRange(ctx, req(bad)); !errors.As(err, &invalid) {
			t.Fatalf("tier %q: want ValidationError, got %v", bad, err)
		}
		if repo.called {
			t.Fatalf("tier %q reached the repository", bad)
		}
	}

	clearing := req("L1")
	clearing.ShiftCode = ""
	var invalid *apierror.ValidationError
	if _, err := svc.ApplyRange(ctx, clearing); !errors.As(err, &invalid) {
		t.Fatalf("a tier with no window: want ValidationError, got %v", err)
	}
}

func TestDeletingATagNeedsALead(t *testing.T) {
	repo := &fakeScheduleRepo{leadsTeam: false}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
	var forbidden *apierror.ForbiddenError
	if err := svc.DeleteAbsenceKind(leadCtx("engineer@example.com"), "TRAINING"); !errors.As(err, &forbidden) {
		t.Fatalf("want ForbiddenError, got %v", err)
	}
	if repo.called {
		t.Fatal("a non-lead reached the repository")
	}

	repo.leadsTeam = true
	if err := svc.DeleteAbsenceKind(leadCtx("castor.01@example.com"), " TRAINING "); err != nil {
		t.Fatalf("a lead was refused: %v", err)
	}
	if repo.gotKindCode != "TRAINING" {
		t.Fatalf("deleted %q, want TRAINING", repo.gotKindCode)
	}
}

// ── A span that moves somebody to another team ─────────────────────────────
// A Draco engineer on the Brazil rotation is filed under the Americas team for
// its dates. These cover who may then change their rota.

const movedEngineer = "33333333-3333-3333-3333-333333333333"

func movedRepo(callerLeads ...string) *fakeScheduleRepo {
	return &fakeScheduleRepo{
		ledTeams:  callerLeads,
		memberOf:  []string{"draco"},
		movedTo:   "americas",
		movedHome: "draco",
	}
}

func TestTheTeamASpanMovedSomebodyToMayRosterThem(t *testing.T) {
	repo := movedRepo("americas")
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
	_, err := svc.ApplyRange(leadCtx("americas.lead@example.com"), domain.ApplyScheduleRangeRequest{
		UserID: movedEngineer, TeamKey: "americas", ShiftCode: "CRE_WEEKEND",
		From: "2026-10-10", To: "2026-10-10",
	})
	if err != nil {
		t.Fatalf("the Americas lead was refused someone on the Brazil rotation: %v", err)
	}
	if !repo.called {
		t.Fatal("the write never reached the repository")
	}
}

func TestTheHomeLeadKeepsChargeOfTheSpanButCannotRosterThere(t *testing.T) {
	repo := movedRepo("draco")
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
	ctx := leadCtx("draco.lead@example.com")

	// Ending the rotation early, from the Americas row.
	if _, err := svc.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: movedEngineer, TeamKey: "americas", KindCode: "",
		From: "2026-10-20", To: "2026-10-31",
	}); err != nil {
		t.Fatalf("the Draco lead could not end their engineer's rotation: %v", err)
	}
	if repo.gotAbsence.HomeTeamKey != "draco" {
		t.Fatalf("home team handed to the repository = %q, want draco", repo.gotAbsence.HomeTeamKey)
	}

	// But rostering on the Americas rota is the Americas lead's call.
	repo.called = false
	_, err := svc.ApplyRange(ctx, domain.ApplyScheduleRangeRequest{
		UserID: movedEngineer, TeamKey: "americas", ShiftCode: "CRE_WEEKEND",
		From: "2026-10-10", To: "2026-10-10",
	})
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("the Draco lead rostered on the Americas rota: got %v, want ForbiddenError", err)
	}
	if repo.called {
		t.Fatal("the repository was written to")
	}
}

func TestAThirdTeamsLeadHasNoSayOverAMovedEngineer(t *testing.T) {
	repo := movedRepo("castor")
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
	_, err := svc.ApplyAbsence(leadCtx("castor.lead@example.com"), domain.ApplyScheduleAbsenceRequest{
		UserID: movedEngineer, TeamKey: "americas", KindCode: "",
		From: "2026-10-20", To: "2026-10-31",
	})
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("got %v, want ForbiddenError", err)
	}
	if repo.called {
		t.Fatal("the repository was written to")
	}
}

// Marking the rotation from the engineer's own row hands the repository their
// team as home, so the span can be filed under the Americas team beside it.
func TestMarkingAMoveFromTheHomeRowNamesTheHomeTeam(t *testing.T) {
	repo := &fakeScheduleRepo{ledTeams: []string{"draco"}, memberOf: []string{"draco"}}
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
	if _, err := svc.ApplyAbsence(leadCtx("draco.lead@example.com"), domain.ApplyScheduleAbsenceRequest{
		UserID: movedEngineer, TeamKey: "draco", KindCode: "ALLO_BR",
		From: "2026-10-01", To: "2027-03-31",
	}); err != nil {
		t.Fatalf("ApplyAbsence: %v", err)
	}
	if repo.gotAbsence.HomeTeamKey != "draco" {
		t.Fatalf("home team = %q, want draco", repo.gotAbsence.HomeTeamKey)
	}
}
