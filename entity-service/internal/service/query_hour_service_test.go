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
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
)

// Project ids here are real UUIDs, not short labels: the service validates
// the id before anything else touches it, so "p1" is now a 400 rather than a
// project. Each is a recognisable repeated-digit pattern so a failure
// message still identifies which project it is about.

// --- fakes ---------------------------------------------------------------

type fakeQueryHourRepo struct {
	consumption  domain.QueryHourConsumption
	consumptErr  error
	stored       *domain.ProjectQueryHours
	getErr       error
	upsertErr    error
	markPushErr  error
	staleIDs     []string
	staleErr     error
	timeCardProj string
	timeCardErr  error

	notifCtx domain.QueryHourNotificationContext
	notifErr error

	upsertedState int
	markedState   int
	markCalls     int
}

func (f *fakeQueryHourRepo) Consumption(_ context.Context, _ string) (domain.QueryHourConsumption, error) {
	return f.consumption, f.consumptErr
}

func (f *fakeQueryHourRepo) Upsert(_ context.Context, c domain.QueryHourConsumption, state int) (domain.ProjectQueryHours, *int, error) {
	if f.upsertErr != nil {
		return domain.ProjectQueryHours{}, nil, f.upsertErr
	}
	f.upsertedState = state
	// The real Upsert reports the state it replaced from inside the same
	// statement. Model that here: the previous state is whatever `stored`
	// held BEFORE this call, and nil when there was no row at all.
	var previous *int
	if f.stored != nil {
		prev := f.stored.QueryHourState
		previous = &prev
	}
	out := domain.ProjectQueryHours{
		ProjectID:          c.ProjectID,
		ProjectKey:         c.ProjectKey,
		ProjectSFID:        c.ProjectSFID,
		EntitlementMinutes: c.EntitlementMinutes,
		ConsumedMinutes:    c.ConsumedMinutes(),
		BillableMinutes:    c.BillableMinutes,
		NonBillableMinutes: c.NonBillableMinutes,
		QueryHourState:     state,
		ComputedAt:         time.Now().UTC(),
	}
	if f.stored != nil {
		out.LastPushedState = f.stored.LastPushedState
		out.LastPushedAt = f.stored.LastPushedAt
	}
	return out, previous, nil
}

func (f *fakeQueryHourRepo) MarkPushed(_ context.Context, _ string, state int, _ time.Time) error {
	f.markCalls++
	f.markedState = state
	return f.markPushErr
}

func (f *fakeQueryHourRepo) Get(_ context.Context, _ string) (domain.ProjectQueryHours, error) {
	if f.getErr != nil {
		return domain.ProjectQueryHours{}, f.getErr
	}
	if f.stored == nil {
		return domain.ProjectQueryHours{}, &apierror.NotFoundError{Msg: "none"}
	}
	return *f.stored, nil
}

func (f *fakeQueryHourRepo) StaleProjectIDs(_ context.Context, _ time.Time, _ int) ([]string, error) {
	return f.staleIDs, f.staleErr
}

func (f *fakeQueryHourRepo) ProjectIDForTimeCard(_ context.Context, _ string) (string, error) {
	return f.timeCardProj, f.timeCardErr
}

func (f *fakeQueryHourRepo) NotificationContext(_ context.Context, _ string) (domain.QueryHourNotificationContext, error) {
	return f.notifCtx, f.notifErr
}

type fakeNotifier struct {
	calls   int
	lastSub string
	lastPay domain.SubscriptionClosureUpdate
	err     error
}

func (f *fakeNotifier) NotifyClosureState(_ context.Context, sub string, p domain.SubscriptionClosureUpdate) error {
	f.calls++
	f.lastSub = sub
	f.lastPay = p
	return f.err
}

// queryHourStatePtr is local to these tests; the package already has an
// intPtr from sn_catalog_service_test.go.
func queryHourStatePtr(i int) *int { return &i }

// --- threshold mapping ----------------------------------------------------

// The thresholds are lifted from ServiceNow's `Set Project Query Hour State`.
// Boundaries are inclusive there (`>= 75`), so they are tested exactly.
func TestQueryHourStateFor_Thresholds(t *testing.T) {
	cases := []struct {
		name string
		pct  float64
		want int
	}{
		{"zero", 0, domain.QueryHourStateNormal},
		{"just under warning", 74.999, domain.QueryHourStateNormal},
		{"exactly warning", 75, domain.QueryHourStateWarning},
		{"between warning and critical", 80, domain.QueryHourStateWarning},
		{"exactly critical", 90, domain.QueryHourStateCritical},
		{"just under exceeded", 99.999, domain.QueryHourStateCritical},
		{"exactly exceeded", 100, domain.QueryHourStateExceeded},
		{"way over", 250, domain.QueryHourStateExceeded},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := QueryHourStateFor(tc.pct); got != tc.want {
				t.Fatalf("QueryHourStateFor(%v) = %d, want %d", tc.pct, got, tc.want)
			}
		})
	}
}

// ServiceNow skipped projects with no entitlement to dodge a divide-by-zero.
// Here the project is still recorded, at 0%.
func TestRecompute_NoEntitlementIsZeroPercentNotADivideByZero(t *testing.T) {
	repo := &fakeQueryHourRepo{consumption: domain.QueryHourConsumption{
		ProjectID: "11111111-1111-1111-1111-111111111111", EntitlementMinutes: 0, BillableMinutes: 600,
	}}
	svc := NewQueryHourService(repo, nil, nil, unrestrictedAccess{}, true)

	got, err := svc.Recompute(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	if got.PercentConsumed != 0 {
		t.Fatalf("PercentConsumed = %v, want 0", got.PercentConsumed)
	}
	if got.QueryHourState != domain.QueryHourStateNormal {
		t.Fatalf("QueryHourState = %d, want %d", got.QueryHourState, domain.QueryHourStateNormal)
	}
}

// THE DIVERGENCE FROM SERVICENOW, PINNED.
// SN's rule is `if (newState > 0 && current != newState)`, so it never writes
// a lower state: once a project crossed 75% it stayed there even if the
// consumption was corrected downwards. This port recomputes honestly.
func TestRecompute_StateWalksBackDownWhenConsumptionDrops(t *testing.T) {
	repo := &fakeQueryHourRepo{
		// Previously at 3 (exceeded), and Choreo was told so.
		stored: &domain.ProjectQueryHours{
			QueryHourState: domain.QueryHourStateExceeded, LastPushedState: queryHourStatePtr(domain.QueryHourStateExceeded),
		},
		// A recalled time card has since dropped consumption to 50%.
		consumption: domain.QueryHourConsumption{
			ProjectID: "11111111-1111-1111-1111-111111111111", ProjectSFID: "sf1",
			EntitlementMinutes: 6000, BillableMinutes: 3000,
		},
	}
	notifier := &fakeNotifier{}
	svc := NewQueryHourService(repo, notifier, nil, unrestrictedAccess{}, true)

	got, err := svc.Recompute(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	if got.QueryHourState != domain.QueryHourStateNormal {
		t.Fatalf("QueryHourState = %d, want %d (state must walk back down)",
			got.QueryHourState, domain.QueryHourStateNormal)
	}
	if !got.StateChanged {
		t.Fatal("StateChanged = false, want true")
	}
	if notifier.calls != 1 {
		t.Fatalf("notifier calls = %d, want 1 (a downward move must be pushed too)", notifier.calls)
	}
}

// Remaining minutes go negative on an overrun rather than clamping at zero —
// an overrun is real information and SN recorded it too.
func TestRecompute_OverrunReportsNegativeRemaining(t *testing.T) {
	repo := &fakeQueryHourRepo{consumption: domain.QueryHourConsumption{
		ProjectID: "11111111-1111-1111-1111-111111111111", EntitlementMinutes: 6000,
		BillableMinutes: 7000, NonBillableMinutes: 500,
	}}
	svc := NewQueryHourService(repo, nil, nil, unrestrictedAccess{}, true)

	got, err := svc.Recompute(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	if got.ConsumedMinutes != 7500 {
		t.Fatalf("ConsumedMinutes = %d, want 7500 (billable + non-billable)", got.ConsumedMinutes)
	}
	if got.RemainingMinutes != -1500 {
		t.Fatalf("RemainingMinutes = %d, want -1500", got.RemainingMinutes)
	}
	if got.QueryHourState != domain.QueryHourStateExceeded {
		t.Fatalf("QueryHourState = %d, want %d", got.QueryHourState, domain.QueryHourStateExceeded)
	}
}

// Re-running with unchanged numbers must not re-push: last_pushed_state
// already equals the computed state.
func TestRecompute_DoesNotRePushWhenStateUnchanged(t *testing.T) {
	repo := &fakeQueryHourRepo{
		stored: &domain.ProjectQueryHours{
			QueryHourState: domain.QueryHourStateWarning, LastPushedState: queryHourStatePtr(domain.QueryHourStateWarning),
		},
		consumption: domain.QueryHourConsumption{
			ProjectID: "11111111-1111-1111-1111-111111111111", ProjectSFID: "sf1",
			EntitlementMinutes: 6000, BillableMinutes: 4800, // exactly 80%
		},
	}
	notifier := &fakeNotifier{}
	svc := NewQueryHourService(repo, notifier, nil, unrestrictedAccess{}, true)

	got, err := svc.Recompute(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	if notifier.calls != 0 {
		t.Fatalf("notifier calls = %d, want 0", notifier.calls)
	}
	if got.Pushed {
		t.Fatal("Pushed = true, want false")
	}
}

// A previously failed push is retried on the next recompute even when the
// state itself has not moved — that is what last_pushed_state is for.
func TestRecompute_RetriesPushAfterEarlierFailure(t *testing.T) {
	repo := &fakeQueryHourRepo{
		// State is 1, but Choreo was last told 0: the earlier push failed.
		stored: &domain.ProjectQueryHours{
			QueryHourState: domain.QueryHourStateWarning, LastPushedState: queryHourStatePtr(domain.QueryHourStateNormal),
		},
		consumption: domain.QueryHourConsumption{
			ProjectID: "11111111-1111-1111-1111-111111111111", ProjectSFID: "sf1",
			EntitlementMinutes: 6000, BillableMinutes: 4800,
		},
	}
	notifier := &fakeNotifier{}
	svc := NewQueryHourService(repo, notifier, nil, unrestrictedAccess{}, true)

	if _, err := svc.Recompute(context.Background(), "11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	if notifier.calls != 1 {
		t.Fatalf("notifier calls = %d, want 1 (failed push must retry)", notifier.calls)
	}
	if repo.markedState != domain.QueryHourStateWarning {
		t.Fatalf("markedState = %d, want %d", repo.markedState, domain.QueryHourStateWarning)
	}
}

// A push failure must not fail the recompute: the position is already stored
// and the next sweep retries. Same reasoning as the nil Event Hub publisher.
func TestRecompute_PushFailureDoesNotFailTheCall(t *testing.T) {
	repo := &fakeQueryHourRepo{consumption: domain.QueryHourConsumption{
		ProjectID: "11111111-1111-1111-1111-111111111111", ProjectSFID: "sf1",
		EntitlementMinutes: 6000, BillableMinutes: 6000,
	}}
	notifier := &fakeNotifier{err: errors.New("choreo 503")}
	svc := NewQueryHourService(repo, notifier, nil, unrestrictedAccess{}, true)

	got, err := svc.Recompute(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("Recompute returned error %v, want nil", err)
	}
	if got.Pushed {
		t.Fatal("Pushed = true, want false")
	}
	if got.PushError == "" {
		t.Fatal("PushError is empty, want the failure reported")
	}
	if repo.markCalls != 0 {
		t.Fatalf("MarkPushed called %d times after a failed push, want 0", repo.markCalls)
	}
}

// A project with no Salesforce id cannot be pushed. SN would have sent
// `undefined` as the subscription id; skipping and saying so is the honest
// equivalent.
func TestRecompute_SkipsPushWhenProjectHasNoSalesforceID(t *testing.T) {
	repo := &fakeQueryHourRepo{consumption: domain.QueryHourConsumption{
		ProjectID: "11111111-1111-1111-1111-111111111111", ProjectSFID: "",
		EntitlementMinutes: 6000, BillableMinutes: 6000,
	}}
	notifier := &fakeNotifier{}
	svc := NewQueryHourService(repo, notifier, nil, unrestrictedAccess{}, true)

	got, err := svc.Recompute(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	if notifier.calls != 0 {
		t.Fatalf("notifier calls = %d, want 0", notifier.calls)
	}
	if got.PushError == "" {
		t.Fatal("PushError is empty, want the skip reported")
	}
}

// A nil notifier (QUERY_HOUR_CHOREO_BASE_URL unset) disables pushing without
// disabling the recompute, and must not panic.
func TestRecompute_NilNotifierStillRecords(t *testing.T) {
	repo := &fakeQueryHourRepo{consumption: domain.QueryHourConsumption{
		ProjectID: "11111111-1111-1111-1111-111111111111", ProjectSFID: "sf1",
		EntitlementMinutes: 6000, BillableMinutes: 6000,
	}}
	svc := NewQueryHourService(repo, nil, nil, unrestrictedAccess{}, true)

	got, err := svc.Recompute(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	if got.QueryHourState != domain.QueryHourStateExceeded {
		t.Fatalf("QueryHourState = %d, want %d", got.QueryHourState, domain.QueryHourStateExceeded)
	}
	if got.Pushed {
		t.Fatal("Pushed = true, want false")
	}
}

// The payload field names and units are what the Choreo service already
// receives from ServiceNow, so cutover needs no change on their side.
func TestRecompute_PushPayloadMatchesServiceNowShape(t *testing.T) {
	repo := &fakeQueryHourRepo{consumption: domain.QueryHourConsumption{
		ProjectID: "11111111-1111-1111-1111-111111111111", ProjectSFID: "a0d7h00000Dt0R4AAJ",
		EntitlementMinutes: 12600, BillableMinutes: 19281,
	}}
	notifier := &fakeNotifier{}
	svc := NewQueryHourService(repo, notifier, nil, unrestrictedAccess{}, true)

	if _, err := svc.Recompute(context.Background(), "11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	if notifier.lastSub != "a0d7h00000Dt0R4AAJ" {
		t.Fatalf("subscriptionId = %q, want the project's Salesforce id", notifier.lastSub)
	}
	if notifier.lastPay.ConsumedQueryTime != 19281 || notifier.lastPay.TotalQueryTime != 12600 {
		t.Fatalf("payload = %+v, want consumed 19281 / total 12600 (minutes)", notifier.lastPay)
	}
}

// THE OTHER DIVERGENCE, PINNED: the flow's account-wide fan-out is gone.
// One time card resolves to exactly one project.
func TestRecomputeForTimeCard_ScopesToTheCardsOwnProject(t *testing.T) {
	repo := &fakeQueryHourRepo{
		timeCardProj: "0e0e0e0e-0000-0000-0000-00000000000e",
		consumption: domain.QueryHourConsumption{
			ProjectID: "0e0e0e0e-0000-0000-0000-00000000000e", EntitlementMinutes: 6000, BillableMinutes: 600,
		},
	}
	svc := NewQueryHourService(repo, nil, nil, unrestrictedAccess{}, true)

	got, err := svc.RecomputeForTimeCard(context.Background(), "tc1")
	if err != nil {
		t.Fatalf("RecomputeForTimeCard: %v", err)
	}
	if got.ProjectID != "0e0e0e0e-0000-0000-0000-00000000000e" {
		t.Fatalf("ProjectID = %q, want p-owning", got.ProjectID)
	}
}

func TestRecomputeForTimeCard_PropagatesNotFound(t *testing.T) {
	repo := &fakeQueryHourRepo{timeCardErr: &apierror.NotFoundError{Msg: "time card missing"}}
	svc := NewQueryHourService(repo, nil, nil, unrestrictedAccess{}, true)

	_, err := svc.RecomputeForTimeCard(context.Background(), "nope")
	var nfe *apierror.NotFoundError
	if !errors.As(err, &nfe) {
		t.Fatalf("error = %v, want NotFoundError", err)
	}
}

func TestRecompute_RejectsEmptyProjectID(t *testing.T) {
	svc := NewQueryHourService(&fakeQueryHourRepo{}, nil, nil, unrestrictedAccess{}, true)
	_, err := svc.Recompute(context.Background(), "   ")
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("error = %v, want ValidationError", err)
	}
}

// One bad project must not abandon the sweep: it stays the stalest, so the
// next run would hit it first and stall there forever.
func TestSweep_ContinuesPastAFailingProject(t *testing.T) {
	repo := &failOnFirstRepo{
		fakeQueryHourRepo: fakeQueryHourRepo{
			staleIDs: []string{"badbadba-0000-0000-0000-00000000000b", "a0000001-0000-0000-0000-000000000001", "a0000002-0000-0000-0000-000000000002"},
			consumption: domain.QueryHourConsumption{
				EntitlementMinutes: 6000, BillableMinutes: 600,
			},
		},
		failFor: "badbadba-0000-0000-0000-00000000000b",
	}
	svc := NewQueryHourService(repo, nil, nil, unrestrictedAccess{}, true)

	got, err := svc.Sweep(context.Background(), time.Hour, 10)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if got.Requested != 3 || got.Succeeded != 2 || got.Failed != 1 {
		t.Fatalf("got requested=%d succeeded=%d failed=%d, want 3/2/1",
			got.Requested, got.Succeeded, got.Failed)
	}
	if _, ok := got.Errors["badbadba-0000-0000-0000-00000000000b"]; !ok {
		t.Fatalf("Errors = %v, want an entry for the failing project", got.Errors)
	}
}

func TestSweep_RejectsLimitAboveMaximum(t *testing.T) {
	svc := NewQueryHourService(&fakeQueryHourRepo{}, nil, nil, unrestrictedAccess{}, true)
	_, err := svc.Sweep(context.Background(), time.Hour, maxSweepLimit+1)
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("error = %v, want ValidationError", err)
	}
}

func TestSweep_RejectsNegativeStaleFor(t *testing.T) {
	svc := NewQueryHourService(&fakeQueryHourRepo{}, nil, nil, unrestrictedAccess{}, true)
	_, err := svc.Sweep(context.Background(), -time.Minute, 10)
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("error = %v, want ValidationError", err)
	}
}

// failOnFirstRepo makes Consumption fail for one nominated project id.
type failOnFirstRepo struct {
	fakeQueryHourRepo
	failFor string
}

func (f *failOnFirstRepo) Consumption(ctx context.Context, projectID string) (domain.QueryHourConsumption, error) {
	if projectID == f.failFor {
		return domain.QueryHourConsumption{}, errors.New("boom")
	}
	c := f.fakeQueryHourRepo.consumption
	c.ProjectID = projectID
	return c, nil
}

// --- review findings, pinned -------------------------------------------

// The previous state must come from the write, not from a separate read.
//
// This replaces an older test that pinned "any error from Get other than
// NotFound must propagate". Recompute no longer calls Get at all: the state it
// replaced is reported by the upsert itself, so a transport error can no
// longer be mistaken for "no previous state" — the upsert simply fails and the
// whole recompute fails with it. What is worth pinning now is that Get is not
// consulted, because reintroducing it would reopen the race where two
// concurrent recomputes both read the old state and both publish.
func TestRecompute_ReadsThePreviousStateFromTheWriteNotFromGet(t *testing.T) {
	repo := &fakeQueryHourRepo{
		// Any call to Get would fail outright. The recompute must not make one.
		getErr: errors.New("Get must not be called by Recompute"),
		consumption: domain.QueryHourConsumption{
			ProjectID: "11111111-1111-1111-1111-111111111111", EntitlementMinutes: 6000, BillableMinutes: 6000,
		},
	}
	_, err := NewQueryHourService(repo, nil, nil, unrestrictedAccess{}, true).
		Recompute(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("Recompute consulted Get, or failed for another reason: %v", err)
	}
}

// A failing upsert fails the whole recompute. Nothing may be published on the
// strength of a state the database never accepted.
func TestRecompute_UpsertFailurePropagates(t *testing.T) {
	repo := &fakeQueryHourRepo{
		upsertErr: errors.New("connection reset"),
		consumption: domain.QueryHourConsumption{
			ProjectID: "11111111-1111-1111-1111-111111111111", EntitlementMinutes: 6000, BillableMinutes: 6000,
		},
	}
	notifier := &fakeNotifier{}
	_, err := NewQueryHourService(repo, notifier, nil, unrestrictedAccess{}, true).
		Recompute(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err == nil {
		t.Fatal("Recompute returned nil, want the upsert error propagated")
	}
	if notifier.calls != 0 {
		t.Errorf("published %d notice(s) despite the write failing", notifier.calls)
	}
}

// A project with no stored row is a FIRST computation, not a crossing. Without
// this, the first sweep after deploy emails every project already past 75% —
// notices ServiceNow has already sent.
func TestRecompute_FirstComputationIsABaselineAndDoesNotNotify(t *testing.T) {
	repo := &fakeQueryHourRepo{
		stored: nil, // no row yet
		consumption: domain.QueryHourConsumption{
			ProjectID: "11111111-1111-1111-1111-111111111111", ProjectSFID: "sf1",
			EntitlementMinutes: 6000, BillableMinutes: 6000, // 100% -> state 3
		},
	}
	pub := &fakePublisher{}
	got, err := NewQueryHourService(repo, nil, queryHourPublisher{pub}, unrestrictedAccess{}, true).Recompute(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	if got.QueryHourState != domain.QueryHourStateExceeded {
		t.Fatalf("state = %d, want %d — the position must still be recorded",
			got.QueryHourState, domain.QueryHourStateExceeded)
	}
	if len(pub.sent) != 0 {
		t.Fatalf("published %d events on a first computation, want 0", len(pub.sent))
	}
}

// The second recompute onwards can notify, once a baseline exists.
func TestRecompute_NotifiesOnceABaselineExists(t *testing.T) {
	repo := &fakeQueryHourRepo{
		stored: &domain.ProjectQueryHours{QueryHourState: domain.QueryHourStateNormal},
		consumption: domain.QueryHourConsumption{
			ProjectID: "11111111-1111-1111-1111-111111111111", ProjectSFID: "sf1",
			EntitlementMinutes: 6000, BillableMinutes: 4500, // 75% -> state 1
		},
	}
	pub := &fakePublisher{}
	if _, err := NewQueryHourService(repo, nil, queryHourPublisher{pub}, unrestrictedAccess{}, true).Recompute(context.Background(), "11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	if len(pub.sent) != 1 {
		t.Fatalf("published %d events, want 1", len(pub.sent))
	}
}

// The notification gate is independent of Event Hub: with it off, a real
// crossing records and pushes but never emails. This is what makes the
// parallel run alongside ServiceNow actually silent.
func TestRecompute_NotificationsDisabledSuppressesTheEmailOnly(t *testing.T) {
	repo := &fakeQueryHourRepo{
		stored: &domain.ProjectQueryHours{QueryHourState: domain.QueryHourStateNormal},
		consumption: domain.QueryHourConsumption{
			ProjectID: "11111111-1111-1111-1111-111111111111", ProjectSFID: "sf1",
			EntitlementMinutes: 6000, BillableMinutes: 4500,
		},
	}
	pub := &fakePublisher{}
	notifier := &fakeNotifier{}
	got, err := NewQueryHourService(repo, notifier, queryHourPublisher{pub}, unrestrictedAccess{}, false).Recompute(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	if len(pub.sent) != 0 {
		t.Fatalf("published %d events with notifications disabled, want 0", len(pub.sent))
	}
	if notifier.calls != 1 {
		t.Fatalf("Choreo push calls = %d, want 1 — the gate must not affect the push", notifier.calls)
	}
	if got.QueryHourState != domain.QueryHourStateWarning {
		t.Fatalf("state = %d, want %d — the position must still be recorded",
			got.QueryHourState, domain.QueryHourStateWarning)
	}
}

// A cancelled request context stops the sweep cleanly rather than converting
// every remaining project into a spurious failure.
func TestSweep_StopsCleanlyWhenTheDeadlineIsReached(t *testing.T) {
	repo := &fakeQueryHourRepo{
		staleIDs:    []string{"aaaaaaaa-0000-0000-0000-000000000001", "bbbbbbbb-0000-0000-0000-000000000002", "cccccccc-0000-0000-0000-000000000003"},
		consumption: domain.QueryHourConsumption{EntitlementMinutes: 6000, BillableMinutes: 600},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already past the deadline

	got, err := NewQueryHourService(repo, nil, nil, unrestrictedAccess{}, true).Sweep(ctx, time.Hour, 10)
	if err != nil {
		t.Fatalf("Sweep returned %v, want a clean partial result", err)
	}
	if got.Failed != 0 {
		t.Fatalf("Failed = %d, want 0 — a deadline is not a per-project failure", got.Failed)
	}
	if got.Requested != 0 {
		t.Fatalf("Requested = %d, want 0 attempted", got.Requested)
	}
}

// queryHourPublisher adapts the package's fakePublisher to
// EventPublisherService, which also requires Close. Defined here rather than
// adding Close to fakePublisher so cr_notice_service_test.go is untouched.
type queryHourPublisher struct{ *fakePublisher }

func (queryHourPublisher) Close() {}

// The sweep must finish inside the server's 15s WriteTimeout, not the 30s
// request context — otherwise it does the work and cannot deliver it. This
// pins the budget as the binding constraint.
func TestSweep_BudgetIsInsideTheServerWriteDeadline(t *testing.T) {
	const serverWriteTimeout = 15 * time.Second // entity-service/internal/server/server.go
	if sweepBudget >= serverWriteTimeout {
		t.Fatalf("sweepBudget %v must leave headroom inside the %v write deadline",
			sweepBudget, serverWriteTimeout)
	}
	// Enough room left to serialise and write a full batch response.
	if margin := serverWriteTimeout - sweepBudget; margin < 3*time.Second {
		t.Fatalf("only %v left to write the response; want at least 3s", margin)
	}
}

// A slow project must not let the sweep run past its budget.
func TestSweep_StopsOnTheTimeBudget(t *testing.T) {
	// Shrink the budget so the test is fast; restore it afterwards.
	original := sweepBudget
	sweepBudget = 40 * time.Millisecond
	defer func() { sweepBudget = original }()

	repo := &slowRepo{
		fakeQueryHourRepo: fakeQueryHourRepo{
			staleIDs:    []string{"aaaaaaaa-0000-0000-0000-000000000001", "bbbbbbbb-0000-0000-0000-000000000002", "cccccccc-0000-0000-0000-000000000003", "dddddddd-0000-0000-0000-000000000004"},
			consumption: domain.QueryHourConsumption{EntitlementMinutes: 6000, BillableMinutes: 600},
		},
		delay: 25 * time.Millisecond,
	}
	got, err := NewQueryHourService(repo, nil, nil, unrestrictedAccess{}, true).Sweep(context.Background(), time.Hour, 10)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	// Two projects consume the budget; the rest must be left for the next run.
	if got.Requested >= 4 {
		t.Fatalf("Requested = %d, want fewer than 4 — the budget should have cut it short", got.Requested)
	}
	if got.Failed != 0 {
		t.Fatalf("Failed = %d, want 0 — a budget stop is not a per-project failure", got.Failed)
	}
}

// slowRepo makes each Consumption call take a fixed time.
type slowRepo struct {
	fakeQueryHourRepo
	delay time.Duration
}

func (r *slowRepo) Consumption(_ context.Context, projectID string) (domain.QueryHourConsumption, error) {
	time.Sleep(r.delay)
	c := r.fakeQueryHourRepo.consumption
	c.ProjectID = projectID
	return c, nil
}

// unrestrictedAccess is the internal-service caller: everything in scope.
// Used by every test that is not itself about authorization.
type unrestrictedAccess struct{}

func (unrestrictedAccess) ResolveScope(context.Context) (AccessScope, error) {
	return AccessScope{Unrestricted: true}, nil
}

// queryHourScopedAccess is an EXTERNAL caller limited to the listed projects.
// Named for this suite rather than the plainer scopedAccess, which
// project_consumption_service_test.go already defines in this package for
// the same purpose with a different field name.
type queryHourScopedAccess struct{ projects []string }

func (a queryHourScopedAccess) ResolveScope(context.Context) (AccessScope, error) {
	return AccessScope{Unrestricted: false, ProjectIDs: a.projects}, nil
}

// A caller outside the project's scope must not be able to read its position,
// and must not be able to tell a forbidden project from a missing one.
func TestGet_OutOfScopeProjectIsNotFound(t *testing.T) {
	repo := &fakeQueryHourRepo{stored: &domain.ProjectQueryHours{QueryHourState: 2}}
	svc := NewQueryHourService(repo, nil, nil, queryHourScopedAccess{projects: []string{"99999999-9999-9999-9999-999999999999"}}, true)

	_, err := svc.Get(context.Background(), "11111111-1111-1111-1111-111111111111")
	var nfe *apierror.NotFoundError
	if !errors.As(err, &nfe) {
		t.Fatalf("error = %v, want NotFoundError (never Forbidden — that leaks existence)", err)
	}
}

// Recompute has side effects (a Choreo push and possibly an email), so it must
// refuse before doing any of them.
func TestRecompute_OutOfScopeProjectDoesNothing(t *testing.T) {
	repo := &fakeQueryHourRepo{consumption: domain.QueryHourConsumption{
		ProjectID: "11111111-1111-1111-1111-111111111111", ProjectSFID: "sf1", EntitlementMinutes: 6000, BillableMinutes: 6000,
	}}
	notifier := &fakeNotifier{}
	svc := NewQueryHourService(repo, notifier, nil, queryHourScopedAccess{projects: []string{"99999999-9999-9999-9999-999999999999"}}, true)

	if _, err := svc.Recompute(context.Background(), "11111111-1111-1111-1111-111111111111"); err == nil {
		t.Fatal("Recompute succeeded for an out-of-scope project")
	}
	if notifier.calls != 0 {
		t.Fatalf("pushed to Choreo %d times for an out-of-scope project, want 0", notifier.calls)
	}
	if repo.markCalls != 0 {
		t.Fatalf("wrote %d times for an out-of-scope project, want 0", repo.markCalls)
	}
}

// A caller inside scope is unaffected.
func TestRecompute_InScopeProjectSucceeds(t *testing.T) {
	repo := &fakeQueryHourRepo{consumption: domain.QueryHourConsumption{
		ProjectID: "11111111-1111-1111-1111-111111111111", EntitlementMinutes: 6000, BillableMinutes: 600,
	}}
	svc := NewQueryHourService(repo, nil, nil, queryHourScopedAccess{projects: []string{"11111111-1111-1111-1111-111111111111"}}, true)
	if _, err := svc.Recompute(context.Background(), "11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("Recompute: %v", err)
	}
}

// The sweep touches the whole estate and pushes outward, so only an internal
// service may run it.
func TestSweep_RefusesANonInternalCaller(t *testing.T) {
	repo := &fakeQueryHourRepo{staleIDs: []string{"aaaaaaaa-0000-0000-0000-000000000001"}}
	svc := NewQueryHourService(repo, nil, nil, queryHourScopedAccess{projects: []string{"aaaaaaaa-0000-0000-0000-000000000001"}}, true)

	_, err := svc.Sweep(context.Background(), time.Hour, 10)
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("error = %v, want ForbiddenError", err)
	}
}

// The greeting must name the account manager. The first cut of this port had
// no OwnerName on the payload at all, so every email opened "Hi Account
// Manager," where ServiceNow's opened "Hi Ivan Saverus,".
func TestRecompute_PayloadCarriesTheAccountManagersName(t *testing.T) {
	repo := &fakeQueryHourRepo{
		stored: &domain.ProjectQueryHours{QueryHourState: domain.QueryHourStateCritical},
		consumption: domain.QueryHourConsumption{
			ProjectID: "11111111-1111-1111-1111-111111111111", ProjectKey: "INTREPIDSUBSUB", ProjectSFID: "sf1",
			EntitlementMinutes: 6000, BillableMinutes: 6095,
		},
		notifCtx: domain.QueryHourNotificationContext{
			AccountName:         "Intrepid Travel",
			ProjectName:         "Intrepidsub - Subscription",
			AccountManagerEmail: "ivan.saverus@wso2.com",
			AccountManagerName:  "Ivan Saverus",
		},
	}
	pub := &fakePublisher{}
	svc := NewQueryHourService(repo, nil, queryHourPublisher{pub}, unrestrictedAccess{}, true)
	if _, err := svc.Recompute(context.Background(), "11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("Recompute: %v", err)
	}
	if len(pub.sent) != 1 {
		t.Fatalf("published %d events, want 1", len(pub.sent))
	}
	var got events.QueryHourThresholdReachedPayload
	if err := json.Unmarshal(pub.sent[0].Payload, &got); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got.OwnerName != "Ivan Saverus" {
		t.Fatalf("OwnerName = %q, want %q", got.OwnerName, "Ivan Saverus")
	}
	if got.ProjectKey != "INTREPIDSUBSUB" {
		t.Fatalf("ProjectKey = %q, want the key for the project cell", got.ProjectKey)
	}
}
