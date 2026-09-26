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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
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
		{"family is neither CRE nor SRE", domain.SearchScheduleAssignmentsRequest{From: "2026-09-21", To: "2026-09-21", Family: "OPS"}},
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
