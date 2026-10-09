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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// fakeCloudStatusRepo records what the service asked it to do.
type fakeCloudStatusRepo struct {
	candidates []repository.CloudStatusCandidate
	candErr    error

	recorded  []repository.CloudStatusCandidate
	conflicts map[string]bool // key -> already present, so Record reports false

	monitors       map[string][]string
	affectedClouds map[string][]string
	outbox         []repository.OutboxChange
	monitorsErr    error
	statusWrites   []statusWrite

	pending    []domain.PendingCloudStatusWebhook
	pendingErr error

	deliveries []struct {
		id        string
		delivered bool
		errMsg    string
	}

	// claimed are the ids RecordAndClaim handed out; rows backs PendingByID.
	claimed  []string
	released []string
	rows     map[string]domain.PendingCloudStatusWebhook

	unknown     []domain.PendingCloudStatusWebhook
	attempts    []string // "id|token" StartAttempt was asked for
	startOK     bool     // what StartAttempt answers
	outcomes    []repository.DeliveryOutcome
	recordStale bool // RecordDelivery finds no open attempt
}

func (f *fakeCloudStatusRepo) Candidates(_ context.Context, _ []string) ([]repository.CloudStatusCandidate, error) {
	return f.candidates, f.candErr
}

func (f *fakeCloudStatusRepo) CandidatesByOutage(_ context.Context, _, outageIDs []string) ([]repository.CloudStatusCandidate, error) {
	want := map[string]bool{}
	for _, id := range outageIDs {
		want[id] = true
	}
	var out []repository.CloudStatusCandidate
	for _, c := range f.candidates {
		if want[c.OutageID] {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeCloudStatusRepo) ClaimChanges(_ context.Context, _ []string, _ int) ([]repository.OutboxChange, error) {
	claimed := f.outbox
	f.outbox = nil
	return claimed, nil
}

func (f *fakeCloudStatusRepo) Record(_ context.Context, c repository.CloudStatusCandidate) (bool, error) {
	f.recorded = append(f.recorded, c)
	if f.conflicts[c.OutageID+string(c.Event)] {
		return false, nil
	}
	return true, nil
}

// RecordAndReserve records like Record, and remembers the row so PendingByID
// can hand it back -- the publish path reads it straight after.
func (f *fakeCloudStatusRepo) RecordAndReserve(ctx context.Context, c repository.CloudStatusCandidate, _ time.Duration) (string, string, bool, error) {
	recorded, err := f.Record(ctx, c)
	if !recorded || err != nil {
		return "", "", recorded, err
	}
	id := "evt-" + c.OutageID + "-" + string(c.Event) + "-" + c.Cloud
	token := "tok-" + id
	f.claimed = append(f.claimed, id)
	if f.rows == nil {
		f.rows = map[string]domain.PendingCloudStatusWebhook{}
	}
	f.rows[id] = domain.PendingCloudStatusWebhook{ID: id, OutageID: c.OutageID, Number: c.Number,
		Event: c.Event, Cloud: c.Cloud, Timestamp: c.Timestamp, ClaimToken: token}
	return id, token, true, nil
}

func (f *fakeCloudStatusRepo) ReleaseReservation(_ context.Context, id, _ string) error {
	f.released = append(f.released, id)
	return nil
}

func (f *fakeCloudStatusRepo) PendingByID(_ context.Context, id string) (*domain.PendingCloudStatusWebhook, error) {
	w, ok := f.rows[id]
	if !ok {
		return nil, nil
	}
	return &w, nil
}

func (f *fakeCloudStatusRepo) StartAttempt(_ context.Context, id, token string, _ time.Duration) (bool, error) {
	f.attempts = append(f.attempts, id+"|"+token)
	return f.startOK, nil
}

func (f *fakeCloudStatusRepo) AffectedMonitors(_ context.Context, outageID string) ([]string, error) {
	return f.monitors[outageID], f.monitorsErr
}

func (f *fakeCloudStatusRepo) AffectedClouds(_ context.Context, outageID string, _ []string) ([]string, error) {
	return f.affectedClouds[outageID], nil
}

func (f *fakeCloudStatusRepo) SetMonitorStatus(_ context.Context, ids []string, status domain.CloudMonitorStatus) (int64, error) {
	f.statusWrites = append(f.statusWrites, statusWrite{ids: ids, status: status})
	return int64(len(ids)), nil
}

func (f *fakeCloudStatusRepo) ClaimPending(_ context.Context, _, _ int, _ time.Duration) ([]domain.PendingCloudStatusWebhook, error) {
	return f.pending, f.pendingErr
}

func (f *fakeCloudStatusRepo) UnknownOutcomes(_ context.Context, _ int) ([]domain.PendingCloudStatusWebhook, error) {
	return f.unknown, nil
}

func (f *fakeCloudStatusRepo) RecordDelivery(_ context.Context, id string, o repository.DeliveryOutcome) (bool, error) {
	f.deliveries = append(f.deliveries, struct {
		id        string
		delivered bool
		errMsg    string
	}{id, o.Delivered, o.Error})
	f.outcomes = append(f.outcomes, o)
	return !f.recordStale, nil
}

type statusWrite struct {
	ids    []string
	status domain.CloudMonitorStatus
}

const testServiceID = "11111111-1111-1111-1111-111111111111"

// TestCloudStatusSweep_RecordsRoutableTransitions is the ordinary path: two
// in-scope outages, both routable, both new.
func TestCloudStatusSweep_RecordsRoutableTransitions(t *testing.T) {
	repo := &fakeCloudStatusRepo{candidates: []repository.CloudStatusCandidate{
		{OutageID: "a", Number: "OUT001", Cloud: "CHOREO", Event: domain.CloudStatusEventOutageBegin, Timestamp: "2026-09-28T10:00:00Z"},
		{OutageID: "b", Number: "OUT002", Cloud: "ASGARDEO", Event: domain.CloudStatusEventOutageEnd, Timestamp: "2026-09-28T11:00:00Z"},
	}}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	got, err := svc.Sweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Scanned != 2 || got.Recorded != 2 || got.SkippedNoCloud != 0 {
		t.Errorf("got %+v, want scanned 2 recorded 2 skipped 0", got)
	}
}

// TestCloudStatusSweep_AlreadyRecordedIsNotCountedAgain is the steady state:
// the sweep runs on a schedule and mostly finds work it has already done.
//
// This is the behaviour that replaces ServiceNow's re-posting. There, every
// update to an ongoing outage re-fired the flow and posted another identical
// begin webhook; here the second sweep records nothing.
func TestCloudStatusSweep_AlreadyRecordedIsNotCountedAgain(t *testing.T) {
	repo := &fakeCloudStatusRepo{
		candidates: []repository.CloudStatusCandidate{
			{OutageID: "a", Cloud: "CHOREO", Event: domain.CloudStatusEventOutageBegin},
		},
		conflicts: map[string]bool{"a" + string(domain.CloudStatusEventOutageBegin): true},
	}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	got, err := svc.Sweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Scanned != 1 {
		t.Errorf("scanned: got %d, want 1", got.Scanned)
	}
	if got.Recorded != 0 {
		t.Errorf("recorded: got %d, want 0 -- a repeat sweep must record nothing", got.Recorded)
	}
}

// TestCloudStatusSweep_SkipsUnroutableAndKeepsGoing covers the divergence from
// ServiceNow's Look Up Record step, which failed the whole execution when the
// monitor was missing. The sweep handles many outages per run, so one
// unroutable record must not stop the rest -- but it must be counted.
func TestCloudStatusSweep_SkipsUnroutableAndKeepsGoing(t *testing.T) {
	repo := &fakeCloudStatusRepo{candidates: []repository.CloudStatusCandidate{
		{OutageID: "a", Cloud: "", Event: domain.CloudStatusEventOutageBegin},            // no monitor at all
		{OutageID: "b", Cloud: "NOT_A_CLOUD", Event: domain.CloudStatusEventOutageBegin}, // unknown enum value
		{OutageID: "c", Cloud: "DEVANT", Event: domain.CloudStatusEventOutageBegin},
	}}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	got, err := svc.Sweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.SkippedNoCloud != 2 {
		t.Errorf("skippedNoCloud: got %d, want 2", got.SkippedNoCloud)
	}
	if got.Recorded != 1 {
		t.Errorf("recorded: got %d, want 1 -- the routable outage must still be recorded", got.Recorded)
	}
	if len(repo.recorded) != 1 || repo.recorded[0].OutageID != "c" {
		t.Errorf("wrong outage recorded: %+v", repo.recorded)
	}
}

// TestCloudStatusSweep_NoConfiguredScopeIsANoOp guards the safe default. An
// unconfigured deployment must post nothing to a public status page.
func TestCloudStatusSweep_NoConfiguredScopeIsANoOp(t *testing.T) {
	repo := &fakeCloudStatusRepo{candidates: []repository.CloudStatusCandidate{
		{OutageID: "a", Cloud: "CHOREO", Event: domain.CloudStatusEventOutageBegin},
	}}
	svc := NewCloudStatusService(repo, nil)

	got, err := svc.Sweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Scanned != 0 || got.Recorded != 0 {
		t.Errorf("got %+v, want an empty sweep", got)
	}
	if len(repo.recorded) != 0 {
		t.Errorf("recorded %d transitions with no scope configured", len(repo.recorded))
	}
}

// TestCloudOfferingSlug_CoversEveryStoredEnumValue is the regression guard for
// Defect 1.
//
// The list here is cloud_monitor_cloud_offering_enum from csm-sync-service
// migration 0079, in full. ServiceNow's script covered five of these seven and
// returned undefined for the other two. If the sync ever adds an eighth value,
// this test is what should fail -- a missing slug means a cloud whose outages
// are silently never published.
func TestCloudOfferingSlug_CoversEveryStoredEnumValue(t *testing.T) {
	want := map[string]string{
		"ASGARDEO":      "asgardeo",
		"BIJIRA":        "bijira",
		"CHOREO":        "choreo",
		"DEVANT":        "devant",
		"MOESIF":        "moesif",
		"CHOREO_EU":     "choreo-eu",
		"AGENT_MANAGER": "agent-manager",
	}
	for stored, slug := range want {
		if got := domain.CloudOfferingSlug(stored); got != slug {
			t.Errorf("CloudOfferingSlug(%q) = %q, want %q", stored, got, slug)
		}
	}
	if got := domain.CloudOfferingSlug("SOMETHING_NEW"); got != "" {
		t.Errorf("an unknown offering must be unroutable, got %q", got)
	}
}

// TestCloudStatusEventWireValues pins the strings that go on the wire. The
// dashboard switches on these, so a rename in Go must not change them.
func TestCloudStatusEventWireValues(t *testing.T) {
	if got := domain.CloudStatusEventOutageBegin.WireValue(); got != "outage_begin" {
		t.Errorf("begin wire value: got %q, want %q (confirmed from the flow's step 15)", got, "outage_begin")
	}
	// NOTE: "outage_end" is INFERRED, not confirmed -- step 8's Event input
	// was never captured. This test pins what the port currently sends so the
	// value is changed deliberately, not so it is known to be right.
	if got := domain.CloudStatusEventOutageEnd.WireValue(); got != "outage_end" {
		t.Errorf("end wire value: got %q, want %q", got, "outage_end")
	}
}

// TestCloudStatusPendingWebhooks_TranslatesCloudToSlug checks the dashboard
// sees slugs, never the stored enum spelling.
func TestCloudStatusPendingWebhooks_TranslatesCloudToSlug(t *testing.T) {
	repo := &fakeCloudStatusRepo{pending: []domain.PendingCloudStatusWebhook{
		{ID: "1", Cloud: "CHOREO_EU", Event: domain.CloudStatusEventOutageBegin},
		{ID: "2", Cloud: "UNKNOWN", Event: domain.CloudStatusEventOutageBegin},
	}}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	got, err := svc.PendingWebhooks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Count != 1 {
		t.Fatalf("count: got %d, want 1 -- the unmappable row must not be dispatched", got.Count)
	}
	if got.Webhooks[0].Cloud != "choreo-eu" {
		t.Errorf("cloud: got %q, want %q", got.Webhooks[0].Cloud, "choreo-eu")
	}
}

// TestCloudStatusRecordDelivery_FailureNeedsAReason keeps the failure path
// from recording a silent nothing.
func TestCloudStatusRecordDelivery_FailureNeedsAReason(t *testing.T) {
	repo := &fakeCloudStatusRepo{}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	err := svc.RecordDelivery(context.Background(), domain.RecordCloudStatusDeliveryRequest{
		ID: testServiceID, Delivered: false,
	})
	var verr *apierror.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("got %v, want a ValidationError", err)
	}
	if len(repo.deliveries) != 0 {
		t.Errorf("a rejected report must not reach the repository")
	}
}

// TestCloudStatusRecordDelivery_SuccessNeedsNoReason is the companion.
func TestCloudStatusRecordDelivery_SuccessNeedsNoReason(t *testing.T) {
	repo := &fakeCloudStatusRepo{}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	if err := svc.RecordDelivery(context.Background(), domain.RecordCloudStatusDeliveryRequest{
		ID: testServiceID, Delivered: true,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.deliveries) != 1 || !repo.deliveries[0].delivered {
		t.Errorf("delivery not recorded: %+v", repo.deliveries)
	}
}

// TestCloudStatusSweep_OngoingOutageSetsSeverityByType is the ongoing arm's
// mapping, straight from the flow's script. Note that type "outage" produces
// PARTIAL_OUTAGE, not MAJOR_OUTAGE -- that is what the flow did.
func TestCloudStatusSweep_OngoingOutageSetsSeverityByType(t *testing.T) {
	cases := []struct {
		outageType string
		want       domain.CloudMonitorStatus
	}{
		{"PLANNED", domain.CloudMonitorStatusMaintenance},
		{"DEGRADATION", domain.CloudMonitorStatusDegraded},
		{"OUTAGE", domain.CloudMonitorStatusPartialOutage},
	}
	for _, tc := range cases {
		t.Run(tc.outageType, func(t *testing.T) {
			repo := &fakeCloudStatusRepo{
				candidates: []repository.CloudStatusCandidate{{
					OutageID: "o1", Cloud: "CHOREO",
					Event: domain.CloudStatusEventOutageBegin, Type: tc.outageType,
				}},
				monitors: map[string][]string{"o1": {"m1", "m2"}},
			}
			svc := NewCloudStatusService(repo, []string{testServiceID})

			got, err := svc.Sweep(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(repo.statusWrites) != 1 || repo.statusWrites[0].status != tc.want {
				t.Fatalf("status writes: %+v, want %s", repo.statusWrites, tc.want)
			}
			if got.MonitorsUpdated != 2 {
				t.Errorf("monitorsUpdated: got %d, want 2", got.MonitorsUpdated)
			}
			if got.UnknownOutageType != 0 {
				t.Errorf("a known type must not be counted as unknown")
			}
		})
	}
}

// TestCloudStatusSweep_EndedOutageReturnsMonitorsToOperational is the
// completed arm's literal 0.
func TestCloudStatusSweep_EndedOutageReturnsMonitorsToOperational(t *testing.T) {
	repo := &fakeCloudStatusRepo{
		candidates: []repository.CloudStatusCandidate{{
			OutageID: "o1", Cloud: "CHOREO",
			Event: domain.CloudStatusEventOutageEnd, Type: "OUTAGE",
		}},
		monitors: map[string][]string{"o1": {"m1"}},
	}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	if _, err := svc.Sweep(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.statusWrites) != 1 {
		t.Fatalf("status writes: %+v", repo.statusWrites)
	}
	// The outage's own type is OUTAGE and must be ignored on this arm: a
	// completed outage is Operational regardless of what it was.
	if repo.statusWrites[0].status != domain.CloudMonitorStatusOperational {
		t.Errorf("status: got %s, want OPERATIONAL", repo.statusWrites[0].status)
	}
}

// TestCloudStatusSweep_MissingTypeFallsBackAndIsCounted covers the defect
// ServiceNow had: its script fell off the end and wrote undefined. A public
// status page must not be left asserting all is well during an incident.
func TestCloudStatusSweep_MissingTypeFallsBackAndIsCounted(t *testing.T) {
	repo := &fakeCloudStatusRepo{
		candidates: []repository.CloudStatusCandidate{{
			OutageID: "o1", Cloud: "CHOREO",
			Event: domain.CloudStatusEventOutageBegin, Type: "",
		}},
		monitors: map[string][]string{"o1": {"m1"}},
	}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	got, err := svc.Sweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UnknownOutageType != 1 {
		t.Errorf("unknownOutageType: got %d, want 1", got.UnknownOutageType)
	}
	if len(repo.statusWrites) != 1 || repo.statusWrites[0].status != domain.CloudMonitorStatusUnknownType {
		t.Fatalf("a missing type must still write a non-operational status: %+v", repo.statusWrites)
	}
	if repo.statusWrites[0].status == domain.CloudMonitorStatusOperational {
		t.Error("a missing type must never leave the page claiming Operational")
	}
}

// TestCloudStatusSweep_StatusIsReassertedEvenWhenAlreadyRecorded is the
// self-healing property: recording is once-only, the status is a desired end
// state and is re-asserted every sweep.
func TestCloudStatusSweep_StatusIsReassertedEvenWhenAlreadyRecorded(t *testing.T) {
	repo := &fakeCloudStatusRepo{
		candidates: []repository.CloudStatusCandidate{{
			OutageID: "o1", Cloud: "CHOREO",
			Event: domain.CloudStatusEventOutageBegin, Type: "OUTAGE",
		}},
		conflicts: map[string]bool{"o1" + string(domain.CloudStatusEventOutageBegin): true},
		monitors:  map[string][]string{"o1": {"m1"}},
	}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	got, err := svc.Sweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Recorded != 0 {
		t.Errorf("the webhook must not be re-recorded, got %d", got.Recorded)
	}
	if len(repo.statusWrites) != 1 {
		t.Errorf("the status must still be re-asserted: %+v", repo.statusWrites)
	}
}

// TestCloudStatusSweep_NoAffectedMonitorsIsNotAnError: most outages name no
// affected CIs, and the webhook still goes out regardless.
func TestCloudStatusSweep_NoAffectedMonitorsIsNotAnError(t *testing.T) {
	repo := &fakeCloudStatusRepo{
		candidates: []repository.CloudStatusCandidate{{
			OutageID: "o1", Cloud: "CHOREO",
			Event: domain.CloudStatusEventOutageBegin, Type: "OUTAGE",
		}},
		monitors: map[string][]string{},
	}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	got, err := svc.Sweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Recorded != 1 {
		t.Errorf("the webhook must still be recorded, got %d", got.Recorded)
	}
	if len(repo.statusWrites) != 0 {
		t.Errorf("nothing to write, got %+v", repo.statusWrites)
	}
}

// TestCloudStatusSweep_OngoingFansOutToEveryAffectedCloud is the sibling flow,
// `Cloud Status Event Notification Flow - Affected CI`, folded into the sweep.
//
// An outage on one cloud whose affected CIs sit on another must refresh both
// dashboards: they are separate deployments showing separate components, and
// telling only one leaves the other stale.
func TestCloudStatusSweep_OngoingFansOutToEveryAffectedCloud(t *testing.T) {
	repo := &fakeCloudStatusRepo{
		candidates: []repository.CloudStatusCandidate{{
			OutageID: "o1", Number: "OUT1", Cloud: "DEVANT",
			Event: domain.CloudStatusEventOutageBegin, Type: "OUTAGE",
		}},
		// One repeats the outage's own cloud and must not produce a second
		// event for it; one is new; one is unroutable and must be dropped.
		affectedClouds: map[string][]string{"o1": {"ASGARDEO", "DEVANT", "NOT_A_CLOUD"}},
	}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	got, err := svc.Sweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Recorded != 2 {
		t.Errorf("recorded = %d, want 2 (devant once, asgardeo once)", got.Recorded)
	}
	seen := map[string]int{}
	for _, r := range repo.recorded {
		seen[r.Cloud]++
	}
	if seen["DEVANT"] != 1 || seen["ASGARDEO"] != 1 {
		t.Errorf("wrong cloud fan-out: %v", seen)
	}
	if seen["NOT_A_CLOUD"] != 0 {
		t.Error("an unroutable cloud must never reach the events table")
	}
}

// TestCloudStatusSweep_CompletedDoesNotFanOut is the asymmetry between the two
// flows. The affected-CI flow's condition is "Outage is not Completed", so it
// never runs on resolution — only the outage's own cloud is told.
//
// Fanning out here would look like a tidy symmetry and would post all-clears
// to dashboards ServiceNow never tells.
func TestCloudStatusSweep_CompletedDoesNotFanOut(t *testing.T) {
	repo := &fakeCloudStatusRepo{
		candidates: []repository.CloudStatusCandidate{{
			OutageID: "o1", Number: "OUT1", Cloud: "DEVANT",
			Event: domain.CloudStatusEventOutageEnd, Type: "OUTAGE",
		}},
		affectedClouds: map[string][]string{"o1": {"ASGARDEO", "BIJIRA"}},
	}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	got, err := svc.Sweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Recorded != 1 {
		t.Errorf("recorded = %d, want 1 — completion tells the outage's own cloud only", got.Recorded)
	}
	if len(repo.recorded) != 1 || repo.recorded[0].Cloud != "DEVANT" {
		t.Errorf("wrong cloud on completion: %+v", repo.recorded)
	}
}

// TestCloudStatusSweep_ReachableOnlyViaAffectedCI covers the scope union: the
// affected-CI flow's trigger qualifies on the AFFECTED CI's parent, so an
// outage whose own configuration item is out of scope still counts.
func TestCloudStatusSweep_ReachableOnlyViaAffectedCI(t *testing.T) {
	repo := &fakeCloudStatusRepo{
		candidates: []repository.CloudStatusCandidate{{
			OutageID: "o1", Number: "OUT1", Cloud: "", // no usable cloud of its own
			Event: domain.CloudStatusEventOutageBegin, Type: "OUTAGE",
		}},
		affectedClouds: map[string][]string{"o1": {"CHOREO"}},
	}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	got, err := svc.Sweep(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.SkippedNoCloud != 0 {
		t.Errorf("an outage reachable through its affected CIs must not be skipped")
	}
	if got.Recorded != 1 || repo.recorded[0].Cloud != "CHOREO" {
		t.Errorf("recorded %+v, want one CHOREO event", repo.recorded)
	}
}

// TestCloudStatusDrainer_ResolvesOutageFromEitherTable checks the one piece of
// routing the drainer does: an `outage` row names the outage directly, an
// `outage_affected_ci` row names the join row and carries the outage in its
// snapshot.
func TestCloudStatusDrainer_ResolvesOutageFromEitherTable(t *testing.T) {
	repo := &fakeCloudStatusRepo{
		candidates: []repository.CloudStatusCandidate{
			{OutageID: "o1", Cloud: "CHOREO", Event: domain.CloudStatusEventOutageBegin, Type: "OUTAGE"},
			{OutageID: "o2", Cloud: "DEVANT", Event: domain.CloudStatusEventOutageBegin, Type: "OUTAGE"},
		},
		outbox: []repository.OutboxChange{
			{ID: 1, EntityType: "outage", EntityID: "o1"},
			{ID: 2, EntityType: "outage_affected_ci", EntityID: "join-row-id",
				Snapshot: map[string]any{"outage_id": "o2"}},
			// The orphan the affected-CI mirror documents: a join row with no
			// outage. Must be skipped, not crash the batch.
			{ID: 3, EntityType: "outage_affected_ci", EntityID: "orphan",
				Snapshot: map[string]any{}},
		},
	}
	svc := NewCloudStatusService(repo, []string{testServiceID})
	d := NewCloudStatusDrainer(repo, svc, time.Second)

	n, err := d.drainOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 3 {
		t.Errorf("claimed %d rows, want 3", n)
	}
	got := map[string]bool{}
	for _, r := range repo.recorded {
		got[r.OutageID] = true
	}
	if !got["o1"] || !got["o2"] {
		t.Errorf("both outages should have been handled, got %v", got)
	}
}

// TestCloudStatusDrainer_CollapsesRowsPerOutage: a batch routinely holds
// several rows for one outage, and they all ask the same question.
func TestCloudStatusDrainer_CollapsesRowsPerOutage(t *testing.T) {
	repo := &fakeCloudStatusRepo{
		candidates: []repository.CloudStatusCandidate{
			{OutageID: "o1", Cloud: "CHOREO", Event: domain.CloudStatusEventOutageBegin, Type: "OUTAGE"},
		},
		outbox: []repository.OutboxChange{
			{ID: 1, EntityType: "outage", EntityID: "o1"},
			{ID: 2, EntityType: "outage", EntityID: "o1"},
			{ID: 3, EntityType: "outage_affected_ci", EntityID: "j1", Snapshot: map[string]any{"outage_id": "o1"}},
		},
	}
	svc := NewCloudStatusService(repo, []string{testServiceID})

	if _, err := NewCloudStatusDrainer(repo, svc, time.Second).drainOnce(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// One transition, recorded once, despite three rows naming it.
	if len(repo.recorded) != 1 {
		t.Errorf("recorded %d times, want 1: %+v", len(repo.recorded), repo.recorded)
	}
}

// TestCloudStatusDrainer_ShortOutageGetsBothEvents is why the trigger path
// exists at all.
//
// An outage that begins and ends inside one sweep interval is only ever seen
// finished by a sweep, so it produced an end event, no begin event, and never
// appeared on the public status page. The trigger sees both writes, so the
// begin is recorded when it happens and the end when it happens.
func TestCloudStatusDrainer_ShortOutageGetsBothEvents(t *testing.T) {
	repo := &fakeCloudStatusRepo{
		candidates: []repository.CloudStatusCandidate{
			{OutageID: "o1", Cloud: "CHOREO", Event: domain.CloudStatusEventOutageBegin, Type: "OUTAGE"},
		},
		outbox: []repository.OutboxChange{{ID: 1, EntityType: "outage", EntityID: "o1"}},
	}
	svc := NewCloudStatusService(repo, []string{testServiceID})
	d := NewCloudStatusDrainer(repo, svc, time.Second)

	// The declaration.
	if _, err := d.drainOnce(context.Background()); err != nil {
		t.Fatalf("begin pass: %v", err)
	}

	// The resolution, moments later — a second write, so a second outbox row.
	repo.candidates = []repository.CloudStatusCandidate{
		{OutageID: "o1", Cloud: "CHOREO", Event: domain.CloudStatusEventOutageEnd, Type: "OUTAGE"},
	}
	repo.outbox = []repository.OutboxChange{{ID: 2, EntityType: "outage", EntityID: "o1"}}
	if _, err := d.drainOnce(context.Background()); err != nil {
		t.Fatalf("end pass: %v", err)
	}

	var begins, ends int
	for _, r := range repo.recorded {
		switch r.Event {
		case domain.CloudStatusEventOutageBegin:
			begins++
		case domain.CloudStatusEventOutageEnd:
			ends++
		}
	}
	if begins != 1 || ends != 1 {
		t.Errorf("got %d begin and %d end events, want 1 of each — a sweep would have produced 0 and 1",
			begins, ends)
	}
}

// TestCloudStatusHandleOutages_NoScopeIsANoOp keeps the unconfigured case safe
// on the trigger path too, not just the sweep.
func TestCloudStatusHandleOutages_NoScopeIsANoOp(t *testing.T) {
	repo := &fakeCloudStatusRepo{
		candidates: []repository.CloudStatusCandidate{
			{OutageID: "o1", Cloud: "CHOREO", Event: domain.CloudStatusEventOutageBegin},
		},
	}
	svc := NewCloudStatusService(repo, nil)

	if err := svc.HandleOutages(context.Background(), []string{"o1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.recorded) != 0 {
		t.Error("nothing may be recorded with no scope configured")
	}
}
