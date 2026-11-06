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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Two different id spaces: the platform user record id, and the identity
// provider's user id. They are never equal, so a guard that compared the token's
// id with the allocation's engineer would fail every allocated caller.
const (
	allocTestAllocID    = "00000000-0000-0000-0000-0000000000a1"
	allocTestEngID      = "00000000-0000-0000-0000-0000000000e1"
	allocTestEngineer   = "11111111-1111-4111-8111-111111111111" // platform id of the allocated engineer
	allocTestOtherPlat  = "22222222-2222-4222-8222-222222222222" // platform id of a different engineer
	allocTestIdPUserID  = "99999999-9999-4999-8999-999999999999" // identity provider id, never a platform id
	allocTestEngineerEm = "jane.doe@example.com"
)

type fakeAllocationViewRepo struct {
	alloc     domain.EngagementAllocationView
	allocErr  error
	usersByEm map[string]string // email -> platform id

	gotSearch  domain.SearchEngagementAllocationsRequest
	rows       []domain.EngagementAllocationView
	total      int
	gotWrite   *repository.EngagementAllocationUpdateWrite
	gotLimit   int
	gotOffset  int
	updateRows []domain.EngagementAllocationUpdate
}

func (f *fakeAllocationViewRepo) SearchAllocations(_ context.Context, req domain.SearchEngagementAllocationsRequest) ([]domain.EngagementAllocationView, int, error) {
	f.gotSearch = req
	return f.rows, f.total, nil
}

func (f *fakeAllocationViewRepo) AllocationByID(context.Context, string) (domain.EngagementAllocationView, error) {
	return f.alloc, f.allocErr
}

func (f *fakeAllocationViewRepo) ListUpdates(_ context.Context, _ string, limit, offset int) ([]domain.EngagementAllocationUpdate, int, error) {
	f.gotLimit, f.gotOffset = limit, offset
	return f.updateRows, len(f.updateRows), nil
}

func (f *fakeAllocationViewRepo) CreateUpdate(_ context.Context, w repository.EngagementAllocationUpdateWrite) (domain.EngagementAllocationUpdate, error) {
	f.gotWrite = &w
	return domain.EngagementAllocationUpdate{ID: "u1", Subject: &w.Subject}, nil
}

func (f *fakeAllocationViewRepo) UserByEmail(_ context.Context, email string) (*domain.EngagementAllocationPerson, error) {
	id, ok := f.usersByEm[strings.ToLower(email)]
	if !ok {
		return nil, nil
	}
	return &domain.EngagementAllocationPerson{ID: id, Name: "Jane Doe"}, nil
}

func newAllocationViewFixture() *fakeAllocationViewRepo {
	return &fakeAllocationViewRepo{
		alloc: domain.EngagementAllocationView{
			ID:         allocTestAllocID,
			Engagement: domain.EngagementAllocationEngagement{ID: allocTestEngID},
			Engineer:   &domain.EngagementAllocationPerson{ID: allocTestEngineer},
		},
		usersByEm: map[string]string{
			allocTestEngineerEm:    allocTestEngineer,
			"john.roe@example.com": allocTestOtherPlat,
		},
	}
}

// allocViewCaller is a request whose user token names email and carries idpID as the
// identity provider's user id.
func allocViewCaller(email string) context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: email, UserID: allocTestIdPUserID})
}

func TestAllocationViewCreateUpdate_AllocatedEngineerPosts(t *testing.T) {
	repo := newAllocationViewFixture()
	svc := NewEngagementAllocationViewService(repo, alwaysUnrestrictedAccess{}).(*engagementAllocationViewService)
	// A Wednesday: the cycle must be the Monday of that week.
	svc.now = func() time.Time { return time.Date(2026, 10, 28, 15, 0, 0, 0, time.UTC) }

	// The caller's email differs in case from the stored one, and the token's
	// provider id differs from the platform id: only the platform id counts.
	up, err := svc.CreateUpdate(allocViewCaller("Jane.Doe@Example.com"), allocTestAllocID,
		domain.CreateEngagementAllocationUpdateRequest{Subject: "  Week 3  ", Content: " All on track "})
	if err != nil {
		t.Fatalf("CreateUpdate: %v", err)
	}
	w := repo.gotWrite
	if w == nil {
		t.Fatal("nothing was written")
	}
	if w.AuthorID != allocTestEngineer || w.AllocationID != allocTestAllocID || w.EngagementID != allocTestEngID {
		t.Errorf("ids = %+v", w)
	}
	if w.State != "PUBLISHED" || w.Visibility != "INTERNAL" || w.Scope != "INDIVIDUAL" || w.Frequency != "WEEKLY" {
		t.Errorf("state/visibility/scope/frequency = %s/%s/%s/%s", w.State, w.Visibility, w.Scope, w.Frequency)
	}
	if w.CycleStartDate != "2026-10-26" {
		t.Errorf("cycle = %s, want the Monday 2026-10-26", w.CycleStartDate)
	}
	if w.Subject != "Week 3" || w.Content != "All on track" {
		t.Errorf("subject/content not trimmed: %q %q", w.Subject, w.Content)
	}
	if up.Author == nil || up.Author.ID != allocTestEngineer {
		t.Errorf("author = %+v", up.Author)
	}
}

func TestAllocationViewCreateUpdate_OnlyTheAllocatedEngineer(t *testing.T) {
	tests := []struct {
		name  string
		email string
	}{
		{"another engineer", "john.roe@example.com"},
		{"unknown user", "nobody@example.com"},
		{"no user token (machine caller)", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newAllocationViewFixture()
			svc := NewEngagementAllocationViewService(repo, alwaysUnrestrictedAccess{})
			_, err := svc.CreateUpdate(allocViewCaller(tc.email), allocTestAllocID,
				domain.CreateEngagementAllocationUpdateRequest{Subject: "s", Content: "c"})
			var fe *apierror.ForbiddenError
			if !errors.As(err, &fe) {
				t.Fatalf("err = %v, want Forbidden", err)
			}
			if repo.gotWrite != nil {
				t.Error("a forbidden caller wrote a row")
			}
		})
	}
}

// The identity provider's id must never stand in for a platform id: a token
// whose provider id equals the allocation's engineer id, but whose email is
// somebody else's, is not the engineer.
func TestAllocationViewCreateUpdate_ProviderIDIsNotAPlatformID(t *testing.T) {
	repo := newAllocationViewFixture()
	svc := NewEngagementAllocationViewService(repo, alwaysUnrestrictedAccess{})
	ctx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "john.roe@example.com", UserID: allocTestEngineer})
	_, err := svc.CreateUpdate(ctx, allocTestAllocID, domain.CreateEngagementAllocationUpdateRequest{Subject: "s", Content: "c"})
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) || repo.gotWrite != nil {
		t.Fatalf("err = %v, wrote = %v; want Forbidden and no write", err, repo.gotWrite != nil)
	}
}

func TestAllocationViewCreateUpdate_MissingAllocationIs404(t *testing.T) {
	repo := newAllocationViewFixture()
	repo.allocErr = &apierror.NotFoundError{Msg: "engagement allocation not found"}
	_, err := NewEngagementAllocationViewService(repo, alwaysUnrestrictedAccess{}).CreateUpdate(allocViewCaller(allocTestEngineerEm), allocTestAllocID,
		domain.CreateEngagementAllocationUpdateRequest{Subject: "s", Content: "c"})
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("err = %v, want NotFound", err)
	}
}

func TestAllocationViewCreateUpdate_Validation(t *testing.T) {
	tests := []struct {
		name, id, subject, content string
	}{
		{"empty subject", allocTestAllocID, "  ", "c"},
		{"long subject", allocTestAllocID, strings.Repeat("s", 513), "c"},
		{"empty content", allocTestAllocID, "s", " "},
		{"long content", allocTestAllocID, "s", strings.Repeat("c", 8001)},
		{"bad id", "not-a-uuid", "s", "c"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newAllocationViewFixture()
			_, err := NewEngagementAllocationViewService(repo, alwaysUnrestrictedAccess{}).CreateUpdate(allocViewCaller(allocTestEngineerEm), tc.id,
				domain.CreateEngagementAllocationUpdateRequest{Subject: tc.subject, Content: tc.content})
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want Validation", err)
			}
			if repo.gotWrite != nil {
				t.Error("an invalid update was written")
			}
		})
	}
	// Exactly at the limits is fine.
	repo := newAllocationViewFixture()
	if _, err := NewEngagementAllocationViewService(repo, alwaysUnrestrictedAccess{}).CreateUpdate(allocViewCaller(allocTestEngineerEm), allocTestAllocID,
		domain.CreateEngagementAllocationUpdateRequest{Subject: strings.Repeat("s", 512), Content: strings.Repeat("c", 8000)}); err != nil {
		t.Fatalf("at the limits: %v", err)
	}
}

func TestAllocationView_ExternalCallersRefused(t *testing.T) {
	svc := NewEngagementAllocationViewService(newAllocationViewFixture(), restrictedAccess{})
	ctx := allocViewCaller(allocTestEngineerEm)
	calls := map[string]func() error{
		"search": func() error { _, e := svc.Search(ctx, domain.SearchEngagementAllocationsRequest{}); return e },
		"get":    func() error { _, e := svc.Get(ctx, allocTestAllocID); return e },
		"list":   func() error { _, e := svc.ListUpdates(ctx, allocTestAllocID, domain.Pagination{}); return e },
		"create": func() error {
			_, e := svc.CreateUpdate(ctx, allocTestAllocID, domain.CreateEngagementAllocationUpdateRequest{Subject: "s", Content: "c"})
			return e
		},
	}
	for name, call := range calls {
		var fe *apierror.ForbiddenError
		if err := call(); !errors.As(err, &fe) {
			t.Errorf("%s: err = %v, want Forbidden", name, err)
		}
	}
}

func TestAllocationViewSearch_DefaultsPagingAndPassesFilters(t *testing.T) {
	repo := newAllocationViewFixture()
	repo.rows = []domain.EngagementAllocationView{{ID: "a"}}
	repo.total = 45
	svc := NewEngagementAllocationViewService(repo, alwaysUnrestrictedAccess{})
	resp, err := svc.Search(context.Background(), domain.SearchEngagementAllocationsRequest{
		Filters: domain.EngagementAllocationFilters{SearchQuery: " rollout ", EngineerID: allocTestEngineer, States: []string{"CONFIRMED"}, From: "2026-10-01", To: "2026-10-31"},
		SortBy:  domain.EngagementAllocationSortBy{Field: "accountName", Order: "DESC"},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if repo.gotSearch.Pagination.Limit != defaultLimit || repo.gotSearch.Filters.SearchQuery != "rollout" {
		t.Errorf("request = %+v", repo.gotSearch)
	}
	if resp.Total != 45 || resp.Limit != defaultLimit || resp.Offset != 0 || !resp.HasMore {
		t.Errorf("response = %+v", resp)
	}
}

func TestAllocationViewSearch_Validation(t *testing.T) {
	bad := map[string]domain.SearchEngagementAllocationsRequest{
		"limit too big":   {Pagination: domain.Pagination{Limit: maxLimit + 1}},
		"bad account id":  {Filters: domain.EngagementAllocationFilters{AccountID: "x"}},
		"bad engineer id": {Filters: domain.EngagementAllocationFilters{EngineerID: "x"}},
		"bad state":       {Filters: domain.EngagementAllocationFilters{States: []string{"nope"}}},
		"bad date":        {Filters: domain.EngagementAllocationFilters{From: "10/01/2026"}},
		"inverted range":  {Filters: domain.EngagementAllocationFilters{From: "2026-10-02", To: "2026-10-01"}},
		"bad sort field":  {SortBy: domain.EngagementAllocationSortBy{Field: "id; drop"}},
		"bad sort order":  {SortBy: domain.EngagementAllocationSortBy{Field: "startDate", Order: "sideways"}},
	}
	for name, req := range bad {
		repo := newAllocationViewFixture()
		_, err := NewEngagementAllocationViewService(repo, alwaysUnrestrictedAccess{}).Search(context.Background(), req)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: err = %v, want Validation", name, err)
		}
	}
}

// Every value of engagement_allocation_state_enum (migrations 0079 and 0110)
// must be accepted as a filter, or the list page's state filter returns 400.
func TestAllocationViewSearch_AcceptsEveryEnumState(t *testing.T) {
	states := []string{"TENTATIVE", "READY_FOR_CLEARANCE", "CONFIRMED", "CLEARANCE_IN_PROGRESS", "ALLOCATION_CANCELLED",
		"ACCEPTED_BY_CONSULTANT", "REJECTED_OTHER", "REJECTED_BY_CONSULTANT", "CONFIRMED_VISA_PENDING", "REJECTED_VISA_ISSUES"}
	for _, st := range states {
		_, err := NewEngagementAllocationViewService(newAllocationViewFixture(), alwaysUnrestrictedAccess{}).Search(
			context.Background(), domain.SearchEngagementAllocationsRequest{Filters: domain.EngagementAllocationFilters{States: []string{st}}})
		if err != nil {
			t.Errorf("state %s: %v", st, err)
		}
	}
}

// With several bad ids the message always names the first in field order.
func TestAllocationViewSearch_InvalidIDMessageIsDeterministic(t *testing.T) {
	req := domain.SearchEngagementAllocationsRequest{Filters: domain.EngagementAllocationFilters{AccountID: "x", EngineerID: "y", EngagementID: "z"}}
	for i := 0; i < 20; i++ {
		_, err := NewEngagementAllocationViewService(newAllocationViewFixture(), alwaysUnrestrictedAccess{}).Search(context.Background(), req)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || ve.Msg != "accountId must be a UUID" {
			t.Fatalf("iteration %d: err = %v", i, err)
		}
	}
}

func TestAllocationViewListUpdates_PagesAndMissingAllocation(t *testing.T) {
	repo := newAllocationViewFixture()
	repo.updateRows = []domain.EngagementAllocationUpdate{{ID: "u1"}}
	resp, err := NewEngagementAllocationViewService(repo, alwaysUnrestrictedAccess{}).ListUpdates(context.Background(), allocTestAllocID, domain.Pagination{Limit: 5, Offset: 10})
	if err != nil || repo.gotLimit != 5 || repo.gotOffset != 10 || resp.HasMore {
		t.Fatalf("resp = %+v, err = %v, paged %d/%d", resp, err, repo.gotLimit, repo.gotOffset)
	}

	repo.allocErr = &apierror.NotFoundError{Msg: "engagement allocation not found"}
	_, err = NewEngagementAllocationViewService(repo, alwaysUnrestrictedAccess{}).ListUpdates(context.Background(), allocTestAllocID, domain.Pagination{})
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("err = %v, want NotFound", err)
	}
}

func TestWeekStart(t *testing.T) {
	for in, want := range map[string]string{
		"2026-10-26": "2026-10-26", // Monday
		"2026-10-28": "2026-10-26",
		"2026-11-01": "2026-10-26", // Sunday belongs to the week that began Monday
		"2026-11-02": "2026-11-02",
	} {
		d, _ := time.Parse("2006-01-02", in)
		if got := weekStart(d); got != want {
			t.Errorf("weekStart(%s) = %s, want %s", in, got, want)
		}
	}
}
