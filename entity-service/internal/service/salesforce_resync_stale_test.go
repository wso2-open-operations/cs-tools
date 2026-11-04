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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The resync's apply writes a snapshot read earlier in the request, so its
// guard bypass is for an equal version only: a newer recorded version means
// the record changed since, and the snapshot must not overwrite it.

func ledgerAt(eventType string, status domain.SalesforceIngestStatus, at time.Time) *fakeIngestStateRepo {
	return &fakeIngestStateRepo{rows: map[string]domain.SalesforceIngestState{
		"opportunity/006x": {Entity: "opportunity", SfID: "006x", Status: status, EventType: eventType, EventModifiedOn: at},
	}}
}

func TestResyncGuard(t *testing.T) {
	snap := "2026-10-01T10:00:00.000+0000"
	at, _ := parseSalesforceLastModified(&snap)
	cases := []struct {
		name      string
		eventType string
		status    domain.SalesforceIngestStatus
		recorded  time.Time
		wantSkip  bool
	}{
		{"equal version is bypassed", domain.SalesforceEventUpdated, domain.SalesforceIngestSucceeded, at, false},
		{"older ledger is written", domain.SalesforceEventUpdated, domain.SalesforceIngestSucceeded, at.Add(-time.Hour), false},
		{"strictly newer ledger refuses", domain.SalesforceEventUpdated, domain.SalesforceIngestSucceeded, at.Add(time.Second), true},
		{"newer DELETED refuses", domain.SalesforceEventDeleted, domain.SalesforceIngestSucceeded, at.Add(time.Second), true},
		{"equal DELETED allows (undelete)", domain.SalesforceEventDeleted, domain.SalesforceIngestSucceeded, at, false},
		{"older DELETED allows", domain.SalesforceEventDeleted, domain.SalesforceIngestSucceeded, at.Add(-time.Second), false},
		{"newer FAILED row does not block", domain.SalesforceEventUpdated, domain.SalesforceIngestFailed, at.Add(time.Hour), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, stale := withResyncApply(context.Background())
			skip, on, err := shouldSkipIngest(ctx, ledgerAt(tc.eventType, tc.status, tc.recorded), "opportunity", "006x", domain.SalesforceEventUpdated, &snap)
			if err != nil || skip != tc.wantSkip {
				t.Fatalf("skip = %v err = %v, want skip %v", skip, err, tc.wantSkip)
			}
			if !on.Equal(at) {
				t.Errorf("version = %v, want the snapshot's %v", on, at)
			}
			if got := len(stale.ids) == 1; got != tc.wantSkip {
				t.Errorf("stale ids = %v", stale.ids)
			}
		})
	}
}

func TestResyncGuard_NoLastModifiedKeepsStoredVersion(t *testing.T) {
	stored := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	ctx, stale := withResyncApply(context.Background())
	skip, on, err := shouldSkipIngest(ctx, ledgerAt(domain.SalesforceEventUpdated, domain.SalesforceIngestSucceeded, stored), "opportunity", "006x", "UPDATED", nil)
	if err != nil || skip || !on.Equal(stored) || len(stale.ids) != 0 {
		t.Errorf("skip=%v on=%v err=%v stale=%v, want the stored version kept and no skip", skip, on, err, stale.ids)
	}
	// No row: now, as the event path does.
	before := time.Now().UTC().Add(-time.Second)
	_, on, _ = shouldSkipIngest(ctx, &fakeIngestStateRepo{}, "opportunity", "006x", "UPDATED", nil)
	if on.Before(before) {
		t.Errorf("version = %v, want about now", on)
	}
}

func TestResyncGuard_EventPathUnchanged(t *testing.T) {
	snap := "2026-10-01T10:00:00.000+0000"
	at, _ := parseSalesforceLastModified(&snap)
	// Equal version still skips on the event path.
	skip, _, _ := shouldSkipIngest(context.Background(), ledgerAt(domain.SalesforceEventUpdated, domain.SalesforceIngestSucceeded, at), "opportunity", "006x", "UPDATED", &snap)
	if !skip {
		t.Error("the event path must still skip an equal version")
	}
}

func TestResyncMembershipGuard(t *testing.T) {
	at, _ := parseSalesforceLastModified(sampleStr(testLastModified))
	cases := []struct {
		name      string
		eventType string
		recorded  time.Time
		wantWrite bool
	}{
		{"equal version is bypassed", "UPDATED", at, true},
		{"older step is written", "UPDATED", at.Add(-time.Hour), true},
		{"strictly newer step refuses", "UPDATED", at.Add(time.Second), false},
		{"newer deactivation (DELETED step) refuses", "DELETED", at.Add(time.Second), false},
		{"older or equal DELETED step allows", "DELETED", at, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pc, contact := sampleProjectContact("REGISTERED", "Portal user"), sampleContact()
			h := newIngestHarness(pc, contact, true)
			h.steps.existing = []domain.OnboardingStep{{MembershipSfID: testMembershipID, Step: domain.OnboardingStepDatabase,
				Status: domain.OnboardingStepSucceeded, EventType: tc.eventType, EventModifiedOn: tc.recorded}}
			ctx, stale := withResyncApply(context.Background())
			if err := h.svc.(*salesforceEventService).ingestMembershipRecord(ctx, testMembershipID, "UPDATED", pc, &contact); err != nil {
				t.Fatal(err)
			}
			if wrote := len(h.repo.upserts) == 1; wrote != tc.wantWrite {
				t.Errorf("wrote = %v, want %v", wrote, tc.wantWrite)
			}
			if got := len(stale.ids) == 1; got == tc.wantWrite {
				t.Errorf("stale ids = %v", stale.ids)
			}
			if !tc.wantWrite {
				return
			}
			if !h.repo.steps[0].EventModifiedOn.Equal(at) {
				t.Errorf("recorded version = %v, want the snapshot's", h.repo.steps[0].EventModifiedOn)
			}
		})
	}
}

func TestResyncApply_ReportsSkippedStale(t *testing.T) {
	changed, fresh := resyncOpp("006changedxxxxxxAAA", "Changed"), resyncOpp("006freshxxxxxxxxAAA", "Fresh")
	h := newResyncHarness(t, changed, fresh)
	mod, _ := parseSalesforceLastModified(sampleStr(resyncLastMod))
	h.states.rows = map[string]domain.SalesforceIngestState{
		"opportunity/" + changed.ID: {Entity: "opportunity", SfID: changed.ID, Status: domain.SalesforceIngestSucceeded, EventType: domain.SalesforceEventDeleted, EventModifiedOn: mod.Add(time.Minute)},
	}
	res, err := h.svc.Apply(resyncAdminCtx(), domain.SalesforceResyncApplyRequest{Scope: "account", ID: resyncAccountRow, Components: []string{"opportunities"}})
	if err != nil {
		t.Fatal(err)
	}
	c := res.Components[0]
	if c.Status != domain.SalesforceResyncApplyApplied || c.Succeeded != 1 || c.Failed != 0 || c.SkippedStale != 1 ||
		len(c.SkippedStaleSfIDs) != 1 || c.SkippedStaleSfIDs[0] != changed.ID {
		t.Fatalf("apply = %+v", c)
	}
	if len(h.oppRepo.upserts) != 1 || h.oppRepo.upserts[0].SfID != fresh.ID {
		t.Errorf("upserts = %+v, want only the unchanged opportunity", h.oppRepo.upserts)
	}
}
