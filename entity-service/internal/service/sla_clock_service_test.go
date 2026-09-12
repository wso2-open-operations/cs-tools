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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// fakeSLAClockRepo captures the request it was called with so tests can
// assert on what the service layer forwards to the repository, without a
// live Postgres connection.
type fakeSLAClockRepo struct {
	gotRegisterReq domain.RegisterSLAClockRequest

	gotGetCaseID    string
	gotGetClockType string
	getResult       domain.SLAClock
	getErr          error

	gotTierCaseID      string
	gotTierClockType   string
	gotTier            string
	tierResult         time.Time
	tierAlreadyReached bool
	tierErr            error

	gotSetPausedCaseID    string
	gotSetPausedClockType string
	gotSetPausedValue     bool
	setPausedResult       domain.SLAClock
	setPausedErr          error
}

func (f *fakeSLAClockRepo) Register(_ context.Context, req domain.RegisterSLAClockRequest) (domain.SLAClock, error) {
	f.gotRegisterReq = req
	return domain.SLAClock{CaseID: req.CaseID, ClockType: req.ClockType, StartedOn: req.StartedAt, DueOn: req.DueAt}, nil
}

func (f *fakeSLAClockRepo) Get(_ context.Context, caseID, clockType string) (domain.SLAClock, error) {
	f.gotGetCaseID = caseID
	f.gotGetClockType = clockType
	return f.getResult, f.getErr
}

func (f *fakeSLAClockRepo) SetTierReachedIfUnset(_ context.Context, caseID, clockType, tier string) (time.Time, bool, error) {
	f.gotTierCaseID = caseID
	f.gotTierClockType = clockType
	f.gotTier = tier
	return f.tierResult, f.tierAlreadyReached, f.tierErr
}

func (f *fakeSLAClockRepo) SetPaused(_ context.Context, caseID, clockType string, paused bool) (domain.SLAClock, error) {
	f.gotSetPausedCaseID = caseID
	f.gotSetPausedClockType = clockType
	f.gotSetPausedValue = paused
	return f.setPausedResult, f.setPausedErr
}

func TestSLAClockService_RegisterSLAClock_ForwardsValidRequest(t *testing.T) {
	repo := &fakeSLAClockRepo{}
	svc := NewSLAClockService(repo)

	started := time.Now()
	due := started.Add(2 * time.Hour)
	_, err := svc.RegisterSLAClock(context.Background(), domain.RegisterSLAClockRequest{
		CaseID: "case-1", ClockType: "response", StartedAt: started, DueAt: due,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.gotRegisterReq.CaseID != "case-1" || repo.gotRegisterReq.ClockType != "response" {
		t.Fatalf("expected request forwarded unchanged, got %+v", repo.gotRegisterReq)
	}
}

func TestSLAClockService_RegisterSLAClock_RejectsMissingFields(t *testing.T) {
	started := time.Now()
	due := started.Add(time.Hour)

	cases := []struct {
		name string
		req  domain.RegisterSLAClockRequest
	}{
		{"missing caseId", domain.RegisterSLAClockRequest{ClockType: "response", StartedAt: started, DueAt: due}},
		{"missing clockType", domain.RegisterSLAClockRequest{CaseID: "case-1", StartedAt: started, DueAt: due}},
		{"missing startedAt", domain.RegisterSLAClockRequest{CaseID: "case-1", ClockType: "response", DueAt: due}},
		{"missing dueAt", domain.RegisterSLAClockRequest{CaseID: "case-1", ClockType: "response", StartedAt: started}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &fakeSLAClockRepo{}
			svc := NewSLAClockService(repo)
			if _, err := svc.RegisterSLAClock(context.Background(), c.req); err == nil {
				t.Fatal("expected validation error, got nil")
			}
		})
	}
}

func TestSLAClockService_RegisterSLAClock_RejectsDueAtNotAfterStartedAt(t *testing.T) {
	repo := &fakeSLAClockRepo{}
	svc := NewSLAClockService(repo)

	now := time.Now()
	if _, err := svc.RegisterSLAClock(context.Background(), domain.RegisterSLAClockRequest{
		CaseID: "case-1", ClockType: "response", StartedAt: now, DueAt: now,
	}); err == nil {
		t.Fatal("expected validation error for dueAt == startedAt, got nil")
	}
	if _, err := svc.RegisterSLAClock(context.Background(), domain.RegisterSLAClockRequest{
		CaseID: "case-1", ClockType: "response", StartedAt: now, DueAt: now.Add(-time.Minute),
	}); err == nil {
		t.Fatal("expected validation error for dueAt before startedAt, got nil")
	}
}

func TestSLAClockService_GetSLAClock_RejectsEmptyFields(t *testing.T) {
	repo := &fakeSLAClockRepo{}
	svc := NewSLAClockService(repo)

	if _, err := svc.GetSLAClock(context.Background(), "", "response"); err == nil {
		t.Fatal("expected validation error for empty caseId, got nil")
	}
	if _, err := svc.GetSLAClock(context.Background(), "case-1", ""); err == nil {
		t.Fatal("expected validation error for empty clockType, got nil")
	}
}

func TestSLAClockService_GetSLAClock_ForwardsToRepo(t *testing.T) {
	repo := &fakeSLAClockRepo{getResult: domain.SLAClock{CaseID: "case-1", ClockType: "response"}}
	svc := NewSLAClockService(repo)

	got, err := svc.GetSLAClock(context.Background(), "case-1", "response")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.gotGetCaseID != "case-1" || repo.gotGetClockType != "response" {
		t.Fatalf("expected caseId/clockType forwarded, got %q/%q", repo.gotGetCaseID, repo.gotGetClockType)
	}
	if got.CaseID != "case-1" {
		t.Fatalf("expected repo result returned unchanged, got %+v", got)
	}
}

func TestSLAClockService_SetSLAClockTierReached_RejectsUnknownTier(t *testing.T) {
	repo := &fakeSLAClockRepo{}
	svc := NewSLAClockService(repo)

	req := domain.SetSLAClockTierRequest{Status: domain.SLATierStatusReached}
	if _, err := svc.SetSLAClockTierReached(context.Background(), "case-1", "response", "60", req); err == nil {
		t.Fatal("expected validation error for unknown tier, got nil")
	}
}

func TestSLAClockService_SetSLAClockTierReached_RejectsUnknownStatus(t *testing.T) {
	repo := &fakeSLAClockRepo{}
	svc := NewSLAClockService(repo)

	req := domain.SetSLAClockTierRequest{Status: domain.SLATierStatus("cleared")}
	if _, err := svc.SetSLAClockTierReached(context.Background(), "case-1", "response", "50", req); err == nil {
		t.Fatal("expected validation error for unknown status, got nil")
	}
}

func TestSLAClockService_SetSLAClockTierReached_AcceptsKnownTiers(t *testing.T) {
	for _, tier := range []string{"50", "75", "100"} {
		t.Run(tier, func(t *testing.T) {
			reached := time.Now()
			repo := &fakeSLAClockRepo{tierResult: reached}
			svc := NewSLAClockService(repo)

			req := domain.SetSLAClockTierRequest{Status: domain.SLATierStatusReached}
			resp, err := svc.SetSLAClockTierReached(context.Background(), "case-1", "response", tier, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo.gotTier != tier {
				t.Fatalf("expected tier %q forwarded, got %q", tier, repo.gotTier)
			}
			if !resp.ReachedOn.Equal(reached) {
				t.Fatalf("expected reached timestamp returned unchanged, got %v want %v", resp.ReachedOn, reached)
			}
			if resp.AlreadyReached {
				t.Fatalf("expected AlreadyReached=false when the repo reports a fresh write, got true")
			}
		})
	}
}

// TestSLAClockService_SetSLAClockTierReached_PropagatesAlreadyReached verifies
// the AlreadyReached flag (which callers use to decide whether to react to a
// tier being reached, e.g. by publishing a notification) is forwarded from
// the repository unchanged rather than always defaulting to false.
func TestSLAClockService_SetSLAClockTierReached_PropagatesAlreadyReached(t *testing.T) {
	reached := time.Now()
	repo := &fakeSLAClockRepo{tierResult: reached, tierAlreadyReached: true}
	svc := NewSLAClockService(repo)

	req := domain.SetSLAClockTierRequest{Status: domain.SLATierStatusReached}
	resp, err := svc.SetSLAClockTierReached(context.Background(), "case-1", "response", "50", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.AlreadyReached {
		t.Fatal("expected AlreadyReached=true when the repo reports the tier was already set, got false")
	}
	if !resp.ReachedOn.Equal(reached) {
		t.Fatalf("expected the pre-existing reached timestamp returned, got %v want %v", resp.ReachedOn, reached)
	}
}

func TestSLAClockService_SetSLAClockTierReached_PropagatesNotFound(t *testing.T) {
	wantErr := errors.New("sla_clock not found")
	repo := &fakeSLAClockRepo{tierErr: wantErr}
	svc := NewSLAClockService(repo)

	req := domain.SetSLAClockTierRequest{Status: domain.SLATierStatusReached}
	if _, err := svc.SetSLAClockTierReached(context.Background(), "case-1", "response", "50", req); err != wantErr {
		t.Fatalf("expected repo error propagated unchanged, got %v", err)
	}
}
