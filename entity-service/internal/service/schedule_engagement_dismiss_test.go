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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const dismissAllocID = "a1c00000-0000-4000-8000-000000000001"

func dismissRepo(team string) *leadOf {
	repo := &leadOf{team: team}
	repo.allocationOnRoster = &domain.EngagementAllocationOnRoster{
		UserID: "11111111-1111-1111-1111-111111111111", StartDate: "2026-11-02", EndDate: "2026-11-06", Code: "E08110",
	}
	repo.dismissDays = 2
	return repo
}

// The team's lead takes days off the roster; the span is cut to the
// allocation's own dates and the lead is recorded as the actor.
func TestDismissEngagementAllocation_LeadRemovesDaysCutToTheAllocation(t *testing.T) {
	repo := dismissRepo("castor")
	svc := NewScheduleService(repo, alwaysUnrestrictedAccess{})
	resp, err := svc.DismissEngagementAllocation(leadCtx("lead@example.com"), domain.DismissScheduleEngagementAllocationRequest{
		ID: dismissAllocID, TeamKey: "castor", From: "2026-10-30", To: "2026-11-03",
	})
	if err != nil {
		t.Fatalf("DismissEngagementAllocation: %v", err)
	}
	if resp.Days != 2 {
		t.Fatalf("days = %d, want what the repository changed (2)", resp.Days)
	}
	if got := repo.gotDismissReq; got.From != "2026-11-02" || got.To != "2026-11-03" || got.Restore {
		t.Fatalf("asked the repository for %+v, want 2026-11-02..2026-11-03, a removal", got)
	}
	if repo.gotDismissActor != "lead@example.com" {
		t.Fatalf("actor = %q", repo.gotDismissActor)
	}
}

// Restore goes through the same check and passes the flag on.
func TestDismissEngagementAllocation_RestorePassesThrough(t *testing.T) {
	repo := dismissRepo("castor")
	if _, err := NewScheduleService(repo, alwaysUnrestrictedAccess{}).DismissEngagementAllocation(leadCtx("lead@example.com"), domain.DismissScheduleEngagementAllocationRequest{
		ID: dismissAllocID, TeamKey: "castor", From: "2026-11-04", To: "2026-11-04", Restore: true,
	}); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if !repo.gotDismissReq.Restore {
		t.Fatal("the restore flag was dropped")
	}
}

// Somebody who may not change that team's roster is refused before anything is written.
func TestDismissEngagementAllocation_RefusesAnotherTeamsLead(t *testing.T) {
	repo := dismissRepo("vega")
	_, err := NewScheduleService(repo, alwaysUnrestrictedAccess{}).DismissEngagementAllocation(leadCtx("lead@example.com"), domain.DismissScheduleEngagementAllocationRequest{
		ID: dismissAllocID, TeamKey: "castor", From: "2026-11-02", To: "2026-11-02",
	})
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("err = %v, want a ForbiddenError", err)
	}
	if repo.gotDismissReq.ID != "" {
		t.Fatal("wrote although refused")
	}
}

func TestDismissEngagementAllocation_Validation(t *testing.T) {
	for name, c := range map[string]struct {
		req      domain.DismissScheduleEngagementAllocationRequest
		noRoster bool
	}{
		"missing id":       {req: domain.DismissScheduleEngagementAllocationRequest{TeamKey: "castor", From: "2026-11-02", To: "2026-11-02"}},
		"id not a uuid":    {req: domain.DismissScheduleEngagementAllocationRequest{ID: "x", TeamKey: "castor", From: "2026-11-02", To: "2026-11-02"}},
		"outside its days": {req: domain.DismissScheduleEngagementAllocationRequest{ID: dismissAllocID, TeamKey: "castor", From: "2026-11-09", To: "2026-11-10"}},
		"not on a roster":  {req: domain.DismissScheduleEngagementAllocationRequest{ID: dismissAllocID, TeamKey: "castor", From: "2026-11-02", To: "2026-11-02"}, noRoster: true},
	} {
		t.Run(name, func(t *testing.T) {
			repo := dismissRepo("castor")
			if c.noRoster {
				repo.allocationOnRoster = nil
			}
			_, err := NewScheduleService(repo, alwaysUnrestrictedAccess{}).DismissEngagementAllocation(leadCtx("lead@example.com"), c.req)
			var v *apierror.ValidationError
			var nf *apierror.NotFoundError
			if !errors.As(err, &v) && !errors.As(err, &nf) {
				t.Fatalf("err = %v, want a 400 or 404", err)
			}
			if repo.gotDismissReq.ID != "" {
				t.Fatal("wrote although refused")
			}
		})
	}
}

// An external caller never reaches the repository.
func TestDismissEngagementAllocation_InternalCallersOnly(t *testing.T) {
	repo := dismissRepo("castor")
	if _, err := NewScheduleService(repo, restrictedAccess{}).DismissEngagementAllocation(context.Background(), domain.DismissScheduleEngagementAllocationRequest{
		ID: dismissAllocID, TeamKey: "castor", From: "2026-11-02", To: "2026-11-02",
	}); err == nil {
		t.Fatal("an anonymous caller was let through")
	}
}
