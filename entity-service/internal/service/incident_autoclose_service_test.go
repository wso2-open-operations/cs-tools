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

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type fakeIncidentAutocloseRepo struct {
	due      []repository.IncidentAutocloseCandidate
	listErr  error
	gotDays  int
	gotLimit int
}

func (f *fakeIncidentAutocloseRepo) ListIncidentsDueForAutoclose(_ context.Context, days, limit int) ([]repository.IncidentAutocloseCandidate, error) {
	f.gotDays, f.gotLimit = days, limit
	return f.due, f.listErr
}

// fakeIncidentCloser stands in for the incident service: it records each
// update and the auto-close mark on its context.
type fakeIncidentCloser struct {
	closeErr map[string]error
	notDue   map[string]bool
	closed   []string
	states   []domain.IncidentState
	actor    string
	days     int
}

func (f *fakeIncidentCloser) UpdateIncident(ctx context.Context, req domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error) {
	email, days, ok := incidentAutocloseFromContext(ctx)
	if !ok {
		return domain.UpdateIncidentResponse{}, errors.New("closed without the auto-close mark")
	}
	f.actor, f.days = email, days
	if req.State != nil {
		f.states = append(f.states, *req.State)
	}
	if err := f.closeErr[req.ID]; err != nil {
		return domain.UpdateIncidentResponse{}, err
	}
	if f.notDue[req.ID] {
		return domain.UpdateIncidentResponse{}, repository.ErrIncidentNotDueForAutoclose
	}
	f.closed = append(f.closed, req.ID)
	return domain.UpdateIncidentResponse{}, nil
}

func autoclosePtr(v int) *int { return &v }

// TestIncidentAutoclose_Validation: resolvedForDays is required and 1..3650;
// limit is 1..1000, 0 meaning the default of 200.
func TestIncidentAutoclose_Validation(t *testing.T) {
	for name, tc := range map[string]struct {
		req       domain.IncidentAutocloseRequest
		wantLimit int
	}{
		"missing days":    {req: domain.IncidentAutocloseRequest{}},
		"zero days":       {req: domain.IncidentAutocloseRequest{ResolvedForDays: autoclosePtr(0)}},
		"negative days":   {req: domain.IncidentAutocloseRequest{ResolvedForDays: autoclosePtr(-3)}},
		"days too large":  {req: domain.IncidentAutocloseRequest{ResolvedForDays: autoclosePtr(3651)}},
		"max days":        {req: domain.IncidentAutocloseRequest{ResolvedForDays: autoclosePtr(3650)}, wantLimit: 200},
		"limit too large": {req: domain.IncidentAutocloseRequest{ResolvedForDays: autoclosePtr(14), Limit: 1001}},
		"limit negative":  {req: domain.IncidentAutocloseRequest{ResolvedForDays: autoclosePtr(14), Limit: -1}},
		"default limit":   {req: domain.IncidentAutocloseRequest{ResolvedForDays: autoclosePtr(14)}, wantLimit: 200},
		"max limit":       {req: domain.IncidentAutocloseRequest{ResolvedForDays: autoclosePtr(1), Limit: 1000}, wantLimit: 1000},
	} {
		t.Run(name, func(t *testing.T) {
			repo, closer := &fakeIncidentAutocloseRepo{}, &fakeIncidentCloser{}
			_, err := NewIncidentAutocloseService(repo, closer, internalAccess{}, "").Autoclose(context.Background(), tc.req)
			if tc.wantLimit == 0 {
				var ve *apierror.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("err = %v, want ValidationError", err)
				}
				return
			}
			if err != nil || repo.gotLimit != tc.wantLimit || repo.gotDays != *tc.req.ResolvedForDays {
				t.Fatalf("err = %v, limit = %d, days = %d", err, repo.gotLimit, repo.gotDays)
			}
		})
	}
}

// TestIncidentAutoclose_ClosesEachAndReportsFailures: each incident is closed
// through the incident service (state Closed, marked as the auto-close with the
// system user and the threshold); one failing incident does not stop the
// others; a no-longer-due one is neither closed nor failed; a database
// error's text is not handed to the caller.
func TestIncidentAutoclose_ClosesEachAndReportsFailures(t *testing.T) {
	resolved := time.Date(2026, 10, 1, 8, 0, 0, 0, time.FixedZone("x", 19800))
	repo := &fakeIncidentAutocloseRepo{
		due: []repository.IncidentAutocloseCandidate{
			{ID: "a", Number: "INC-A", ResolvedOn: &resolved},
			{ID: "b", Number: "INC-B"},
			{ID: "c", Number: "INC-C"},
			{ID: "d", Number: "INC-D"},
		},
	}
	closer := &fakeIncidentCloser{
		closeErr: map[string]error{"b": &pgconn.PgError{Code: "23503", Message: "insert or update on table \"incident\" violates foreign key"}},
		notDue:   map[string]bool{"c": true},
	}
	resp, err := NewIncidentAutocloseService(repo, closer, internalAccess{}, "").Autoclose(context.Background(), domain.IncidentAutocloseRequest{ResolvedForDays: autoclosePtr(14), Limit: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Closed) != 2 || resp.Closed[0].Number != "INC-A" || resp.Closed[1].Number != "INC-D" {
		t.Fatalf("closed = %+v", resp.Closed)
	}
	if resp.Closed[0].ResolvedOn == nil || *resp.Closed[0].ResolvedOn != "2026-10-01T02:30:00Z" || resp.Closed[1].ResolvedOn != nil {
		t.Errorf("resolvedOn = %v / %v", resp.Closed[0].ResolvedOn, resp.Closed[1].ResolvedOn)
	}
	if len(resp.Failed) != 1 || resp.Failed[0].Number != "INC-B" {
		t.Fatalf("failed = %+v", resp.Failed)
	}
	if msg := resp.Failed[0].Error; msg == "" || strings.Contains(msg, "foreign key") || strings.Contains(msg, "23503") {
		t.Errorf("failure message leaks the database error: %q", msg)
	}
	if !resp.More || resp.DryRun {
		t.Errorf("more = %v, dryRun = %v", resp.More, resp.DryRun)
	}
	if closer.actor != domain.AutoclosureDefaultActor || closer.days != 14 {
		t.Errorf("auto-close mark = %q / %d, want the default system actor and 14", closer.actor, closer.days)
	}
	for _, st := range closer.states {
		if st != domain.IncidentStateClosed {
			t.Errorf("state sent = %q, want CLOSED", st)
		}
	}
}

// TestIncidentAutoclose_SystemUserIsTheConfiguredOne: the close is marked with
// AUTOCLOSURE_SYSTEM_USER_EMAIL when it is set, as case auto-closure's is.
func TestIncidentAutoclose_SystemUserIsTheConfiguredOne(t *testing.T) {
	repo := &fakeIncidentAutocloseRepo{due: []repository.IncidentAutocloseCandidate{{ID: "a", Number: "INC-A"}}}
	closer := &fakeIncidentCloser{}
	if _, err := NewIncidentAutocloseService(repo, closer, internalAccess{}, "  autoclose@wso2.com ").Autoclose(context.Background(),
		domain.IncidentAutocloseRequest{ResolvedForDays: autoclosePtr(14)}); err != nil {
		t.Fatal(err)
	}
	if closer.actor != "autoclose@wso2.com" {
		t.Fatalf("actor = %q, want the configured system user", closer.actor)
	}
}

// TestIncidentAutoclose_DryRunChangesNothing: a dry run lists what is due as
// closed and never closes or records anything.
func TestIncidentAutoclose_DryRunChangesNothing(t *testing.T) {
	repo := &fakeIncidentAutocloseRepo{due: []repository.IncidentAutocloseCandidate{{ID: "a", Number: "INC-A"}}}
	closer := &fakeIncidentCloser{}
	resp, err := NewIncidentAutocloseService(repo, closer, internalAccess{}, "").Autoclose(context.Background(), domain.IncidentAutocloseRequest{ResolvedForDays: autoclosePtr(14), DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.DryRun || len(resp.Closed) != 1 || len(closer.states) != 0 || resp.More {
		t.Fatalf("resp = %+v, updates sent = %v", resp, closer.states)
	}
}

// TestIncidentAutoclose_ListFailureIsAnError: nothing to report per incident when the listing fails.
func TestIncidentAutoclose_ListFailureIsAnError(t *testing.T) {
	repo, closer := &fakeIncidentAutocloseRepo{listErr: errors.New("boom")}, &fakeIncidentCloser{}
	if _, err := NewIncidentAutocloseService(repo, closer, internalAccess{}, "").Autoclose(context.Background(), domain.IncidentAutocloseRequest{ResolvedForDays: autoclosePtr(14)}); err == nil {
		t.Fatal("want an error")
	}
}

// TestIncidentAutoclose_CancelledRequestStops: a request that ended is
// returned as its context error, not a 200 listing everything as failed.
func TestIncidentAutoclose_CancelledRequestStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := &fakeIncidentAutocloseRepo{due: []repository.IncidentAutocloseCandidate{{ID: "a", Number: "INC-A"}}}
	closer := &fakeIncidentCloser{}
	_, err := NewIncidentAutocloseService(repo, closer, internalAccess{}, "").Autoclose(ctx, domain.IncidentAutocloseRequest{ResolvedForDays: autoclosePtr(14)})
	if !errors.Is(err, context.Canceled) || len(closer.closed) != 0 {
		t.Fatalf("err = %v, closed = %v", err, closer.closed)
	}
}

// TestIncidentAutoclose_RefusesANonInternalCaller: as case auto-closure's
// service does, the service itself refuses a caller that is not internal with a
// 403, before anything is listed or closed (the route's internalOnly gate is the
// first line, this the second).
func TestIncidentAutoclose_RefusesANonInternalCaller(t *testing.T) {
	repo := &fakeIncidentAutocloseRepo{due: []repository.IncidentAutocloseCandidate{{ID: "a", Number: "INC-A"}}}
	closer := &fakeIncidentCloser{}
	_, err := NewIncidentAutocloseService(repo, closer, customerAccess{}, "").Autoclose(context.Background(),
		domain.IncidentAutocloseRequest{ResolvedForDays: autoclosePtr(14)})
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("err = %v, want ForbiddenError", err)
	}
	if repo.gotDays != 0 || len(closer.states) != 0 {
		t.Fatalf("listed (days %d) or closed (%v) for a refused caller", repo.gotDays, closer.states)
	}
}
