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

// This package's tests exercise the retry/escalation logic entirely against
// hand-rolled mocks of Store, IncidentCreator, and Escalator (matching this
// repo's established no-mocking-library convention, e.g.
// acp-closure-service's internal/sweep tests) — no real Postgres or Twilio
// involved. This is deliberate, not a shortcut taken because a database
// wasn't available: RunOnce's branching (deliver / retry / escalate /
// terminal-fail) is pure decision logic over the Store/IncidentCreator/
// Escalator interfaces, so it's fully covered this way regardless of
// whether internal/store's own Postgres-backed test (postgres_test.go,
// which itself needs SRE_ALERT_TEST_DATABASE_URL) runs in a given
// environment.
package worker

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/csmclient"
	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/store"
)

// mockStore is a hand-rolled Store double recording every call it receives.
type mockStore struct {
	pendingBatchFn func(ctx context.Context, limit int) ([]store.AlertRecord, error)
	// markDeliveredErr, when non-nil, is returned by every MarkDelivered
	// call instead of the usual nil — for exercising the "MarkDelivered
	// fails after a successful create/short-circuit" paths.
	markDeliveredErr error
	// recordIncidentIDErr, when non-nil, is returned by every
	// RecordIncidentID call — for exercising "this service's own database
	// failed to persist the incident id" paths.
	recordIncidentIDErr error

	delivered        []struct{ id, incidentID string }
	recordedIncident []struct{ id, incidentID string }
	attemptFailed    []struct{ id, lastError string }
	escalated        []struct{ id, lastError string }
	failed           []struct{ id, lastError string }
}

func (m *mockStore) PendingBatch(ctx context.Context, limit int) ([]store.AlertRecord, error) {
	return m.pendingBatchFn(ctx, limit)
}

func (m *mockStore) MarkDelivered(ctx context.Context, id, incidentID string) error {
	if m.markDeliveredErr != nil {
		return m.markDeliveredErr
	}
	m.delivered = append(m.delivered, struct{ id, incidentID string }{id, incidentID})
	return nil
}

func (m *mockStore) RecordIncidentID(ctx context.Context, id, incidentID string) error {
	if m.recordIncidentIDErr != nil {
		return m.recordIncidentIDErr
	}
	m.recordedIncident = append(m.recordedIncident, struct{ id, incidentID string }{id, incidentID})
	return nil
}

func (m *mockStore) MarkAttemptFailed(ctx context.Context, id, lastError string) error {
	m.attemptFailed = append(m.attemptFailed, struct{ id, lastError string }{id, lastError})
	return nil
}

func (m *mockStore) MarkEscalated(ctx context.Context, id, lastError string) error {
	m.escalated = append(m.escalated, struct{ id, lastError string }{id, lastError})
	return nil
}

func (m *mockStore) MarkFailed(ctx context.Context, id, lastError string) error {
	m.failed = append(m.failed, struct{ id, lastError string }{id, lastError})
	return nil
}

// mockIncidentCreator is a hand-rolled IncidentCreator double.
type mockIncidentCreator struct {
	createFn func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error)
	calls    int

	// searchFn is optional; when nil, SearchIncidentByTag reports "no match,
	// no error" (found=false, err=nil) — the common case for tests that
	// don't care about the dedup check at all (e.g. every first-attempt
	// test, where it's never even called since row.RetryCount == 0).
	searchFn    func(ctx context.Context, tag string) (*csmclient.CreateIncidentResult, bool, error)
	searchCalls int
	searchTags  []string

	// searchGroupFn is optional; when nil, SearchOpenIncidentByGroupTag
	// reports "no match, no error" — the common case for tests that never
	// reach the incident-grouping search at all (no UniqueIdentifier).
	searchGroupFn    func(ctx context.Context, tag string, since time.Time) (*csmclient.CreateIncidentResult, bool, error)
	searchGroupCalls int
	searchGroupTags  []string
	searchGroupSince []time.Time

	// createMappingFn is optional; when nil, CreateAlertIncidentMapping
	// reports success with no error.
	createMappingFn    func(ctx context.Context, req csmclient.CreateAlertIncidentMappingRequest) (*csmclient.AlertIncidentMappingView, error)
	createMappingCalls int
	createMappingReqs  []csmclient.CreateAlertIncidentMappingRequest

	// searchServicesFn is optional; when nil, SearchServices reports "no
	// match, no error" (an empty slice) — the common case for tests whose
	// buffered row already carries a resolved ServiceID and never reaches
	// resolveServiceID at all.
	searchServicesFn     func(ctx context.Context, label string) ([]csmclient.ITService, error)
	searchServicesCalls  int
	searchServicesLabels []string

	// updateIncidentFn is optional; when nil, UpdateIncident reports success
	// with no error — the common case for tests that don't care about the
	// group-attach work-note push at all.
	updateIncidentFn    func(ctx context.Context, incidentID, workNotes string) error
	updateIncidentCalls int
	updateIncidentIDs   []string
	updateIncidentNotes []string
}

func (m *mockIncidentCreator) CreateIncident(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
	m.calls++
	return m.createFn(ctx, req)
}

func (m *mockIncidentCreator) SearchIncidentByTag(ctx context.Context, tag string) (*csmclient.CreateIncidentResult, bool, error) {
	m.searchCalls++
	m.searchTags = append(m.searchTags, tag)
	if m.searchFn != nil {
		return m.searchFn(ctx, tag)
	}
	return nil, false, nil
}

func (m *mockIncidentCreator) SearchOpenIncidentByGroupTag(ctx context.Context, tag string, since time.Time) (*csmclient.CreateIncidentResult, bool, error) {
	m.searchGroupCalls++
	m.searchGroupTags = append(m.searchGroupTags, tag)
	m.searchGroupSince = append(m.searchGroupSince, since)
	if m.searchGroupFn != nil {
		return m.searchGroupFn(ctx, tag, since)
	}
	return nil, false, nil
}

func (m *mockIncidentCreator) CreateAlertIncidentMapping(ctx context.Context, req csmclient.CreateAlertIncidentMappingRequest) (*csmclient.AlertIncidentMappingView, error) {
	m.createMappingCalls++
	m.createMappingReqs = append(m.createMappingReqs, req)
	if m.createMappingFn != nil {
		return m.createMappingFn(ctx, req)
	}
	return &csmclient.AlertIncidentMappingView{}, nil
}

func (m *mockIncidentCreator) SearchServices(ctx context.Context, label string) ([]csmclient.ITService, error) {
	m.searchServicesCalls++
	m.searchServicesLabels = append(m.searchServicesLabels, label)
	if m.searchServicesFn != nil {
		return m.searchServicesFn(ctx, label)
	}
	return nil, nil
}

func (m *mockIncidentCreator) UpdateIncident(ctx context.Context, incidentID, workNotes string) error {
	m.updateIncidentCalls++
	m.updateIncidentIDs = append(m.updateIncidentIDs, incidentID)
	m.updateIncidentNotes = append(m.updateIncidentNotes, workNotes)
	if m.updateIncidentFn != nil {
		return m.updateIncidentFn(ctx, incidentID, workNotes)
	}
	return nil
}

// mockEscalator is a hand-rolled Escalator double.
type mockEscalator struct {
	err      error
	messages []string
}

func (m *mockEscalator) Escalate(ctx context.Context, message string) error {
	m.messages = append(m.messages, message)
	return m.err
}

// rowWithPayload builds a row whose AlertNumber matches id (tests that care
// about the dedup tag pass id as both, e.g. csmclient.DedupTag("alert-1"))
// and whose payload carries no UniqueIdentifier — the common case where
// incident-grouping never even attempts a lookup. See
// rowWithGroupablePayload for the grouping-specific variant.
func rowWithPayload(t *testing.T, id string, retryCount int, lastAttemptAt *time.Time) store.AlertRecord {
	t.Helper()
	return store.AlertRecord{
		ID:            id,
		AlertNumber:   id,
		Status:        store.StatusPending,
		RetryCount:    retryCount,
		LastAttemptAt: lastAttemptAt,
		Payload:       []byte(`{"callerId":"caller-1","category":"SERVICE_INTERRUPTION","serviceId":"svc-1","impact":"HIGH","urgency":"HIGH","subject":"test"}`),
	}
}

// rowWithUnresolvedService is rowWithPayload's variant for service-UUID
// resolution tests: serviceId is the empty string
// (csmclient.UnresolvedServiceIDSentinel — what internal/handler.MapToIncident
// persists when SRE_ALERT_SERVICE_MAP has no entry for the alert's raw
// Service label), and the raw label itself is carried in the payload's own
// "service" field, matching what a real buffered row looks like in that
// case.
func rowWithUnresolvedService(t *testing.T, id, service string, retryCount int) store.AlertRecord {
	t.Helper()
	payload := fmt.Sprintf(
		`{"callerId":"caller-1","category":"SERVICE_INTERRUPTION","serviceId":"","impact":"HIGH","urgency":"HIGH","subject":"test","service":%q}`,
		service,
	)
	return store.AlertRecord{
		ID:          id,
		AlertNumber: id,
		Status:      store.StatusPending,
		RetryCount:  retryCount,
		Payload:     []byte(payload),
	}
}

// rowWithGroupablePayload is rowWithPayload's variant for incident-grouping
// tests: the buffered payload carries source/uniqueIdentifier/service/
// metricName/alertStatus (internal/alertpayload.Payload's fields, alongside
// the embedded CreateIncidentRequest), matching what internal/handler
// actually persists for an alert with a vendor-supplied UniqueIdentifier.
func rowWithGroupablePayload(t *testing.T, id, source, uniqueIdentifier string) store.AlertRecord {
	t.Helper()
	payload := fmt.Sprintf(
		`{"callerId":"caller-1","category":"SERVICE_INTERRUPTION","serviceId":"svc-1","impact":"HIGH","urgency":"HIGH","subject":"test",`+
			`"source":%q,"uniqueIdentifier":%q,"service":"svc-1","metricName":"error_rate","alertStatus":"FIRING"}`,
		source, uniqueIdentifier,
	)
	return store.AlertRecord{
		ID:          id,
		AlertNumber: id,
		Status:      store.StatusPending,
		RetryCount:  0,
		Payload:     []byte(payload),
	}
}

func TestRunOnce_DeliversSuccessfully(t *testing.T) {
	row := rowWithPayload(t, "alert-1", 0, nil)
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		return &csmclient.CreateIncidentResult{IncidentID: "inc-1", IncidentNumber: "INC0001"}, nil
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 3})
	w.RunOnce(context.Background())

	if len(s.delivered) != 1 || s.delivered[0].id != "alert-1" || s.delivered[0].incidentID != "inc-1" {
		t.Errorf("delivered = %+v, want one row for alert-1/inc-1", s.delivered)
	}
	if len(s.attemptFailed) != 0 || len(s.escalated) != 0 || len(s.failed) != 0 {
		t.Errorf("unexpected non-delivered transitions: attemptFailed=%v escalated=%v failed=%v", s.attemptFailed, s.escalated, s.failed)
	}
	if len(tw.messages) != 0 {
		t.Error("Twilio should not be called on a successful delivery")
	}
}

func TestRunOnce_RetriesOnTransientErrorBelowThreshold(t *testing.T) {
	row := rowWithPayload(t, "alert-1", 1, nil) // retryCount=1, MaxRetries=5 -> nextRetryCount=2, still below threshold
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		return nil, errors.New("connection refused")
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 5})
	w.RunOnce(context.Background())

	if len(s.attemptFailed) != 1 || s.attemptFailed[0].id != "alert-1" {
		t.Errorf("attemptFailed = %+v, want one row for alert-1", s.attemptFailed)
	}
	if len(s.escalated) != 0 || len(s.failed) != 0 || len(s.delivered) != 0 {
		t.Errorf("unexpected transitions: escalated=%v failed=%v delivered=%v", s.escalated, s.failed, s.delivered)
	}
	if len(tw.messages) != 0 {
		t.Error("Twilio should not be called before the retry threshold is reached")
	}
}

// The upstream 401 case is the load-bearing test: csm-integration-service's
// CreateIncident can still 401 (e.g. if the target ServiceNow environment's
// M2M integration credential isn't configured — it is not an unconditional
// limitation; a live end-to-end call against wso2sndev on 2026-09-20
// succeeded with no 401), and if it does occur that must be treated exactly
// like any other transient CSM-unavailability signal — retried, not treated
// as a permanent failure.
func TestRunOnce_401IsRetryableNotTerminal(t *testing.T) {
	row := rowWithPayload(t, "alert-1", 0, nil)
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		return nil, &apierror.Error{StatusCode: 401, Body: "Missing or invalid user ID token header."}
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 5})
	w.RunOnce(context.Background())

	if len(s.attemptFailed) != 1 {
		t.Fatalf("attemptFailed = %+v, want exactly one retryable-failure transition for a 401", s.attemptFailed)
	}
	if len(s.failed) != 0 {
		t.Errorf("failed = %+v, want zero — a 401 must not be treated as a terminal, non-retryable error", s.failed)
	}
}

func TestRunOnce_400IsNonRetryableTerminal(t *testing.T) {
	row := rowWithPayload(t, "alert-1", 0, nil)
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		return nil, &apierror.Error{StatusCode: 400, Body: "invalid payload"}
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 5})
	w.RunOnce(context.Background())

	if len(s.failed) != 1 || s.failed[0].id != "alert-1" {
		t.Errorf("failed = %+v, want one terminal-failure row for alert-1", s.failed)
	}
	if len(s.attemptFailed) != 0 || len(s.escalated) != 0 {
		t.Errorf("a 400 must not be retried or escalated: attemptFailed=%v escalated=%v", s.attemptFailed, s.escalated)
	}
	if len(tw.messages) != 0 {
		t.Error("Twilio should not be called for a non-retryable 400")
	}
}

func TestRunOnce_EscalatesAfterMaxRetries(t *testing.T) {
	// retryCount=4, MaxRetries=5 -> nextRetryCount=5 >= 5 -> escalate on this attempt.
	row := rowWithPayload(t, "alert-1", 4, nil)
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		return nil, errors.New("dial tcp: connection refused")
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 5})
	w.RunOnce(context.Background())

	if len(s.escalated) != 1 || s.escalated[0].id != "alert-1" {
		t.Fatalf("escalated = %+v, want one row for alert-1", s.escalated)
	}
	if len(s.attemptFailed) != 0 {
		t.Errorf("attemptFailed = %+v, want zero once escalation fires", s.attemptFailed)
	}
	if len(tw.messages) != 1 {
		t.Fatalf("Twilio Escalate called %d times, want 1", len(tw.messages))
	}
	if tw.messages[0] == "" {
		t.Error("escalation message is empty")
	}
}

func TestRunOnce_EscalationCallFailureDoesNotBlockStoreTransition(t *testing.T) {
	row := rowWithPayload(t, "alert-1", 4, nil)
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		return nil, errors.New("connection refused")
	}}
	tw := &mockEscalator{err: errors.New("twilio: 500 internal error")}

	w := New(s, csm, tw, Config{MaxRetries: 5})
	w.RunOnce(context.Background())

	if len(s.escalated) != 1 {
		t.Fatalf("escalated = %+v, want the row still marked escalated even though the Twilio call itself failed", s.escalated)
	}
}

func TestRunOnce_SkipsRowsNotYetDue(t *testing.T) {
	fixedNow := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	recentAttempt := fixedNow.Add(-1 * time.Second) // far inside the 30s base delay for retryCount 0
	row := rowWithPayload(t, "alert-1", 0, &recentAttempt)

	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		return &csmclient.CreateIncidentResult{IncidentID: "inc-1"}, nil
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 5})
	w.now = func() time.Time { return fixedNow }
	w.RunOnce(context.Background())

	if csm.calls != 0 {
		t.Errorf("CreateIncident called %d times, want 0 — row is not yet due per backoff", csm.calls)
	}
	if len(s.delivered) != 0 || len(s.attemptFailed) != 0 {
		t.Error("no store transition should occur for a row that isn't due")
	}
}

func TestRunOnce_AttemptsRowsPastTheirBackoffWindow(t *testing.T) {
	fixedNow := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	oldAttempt := fixedNow.Add(-1 * time.Hour) // well past even the max backoff delay
	row := rowWithPayload(t, "alert-1", 2, &oldAttempt)

	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		return &csmclient.CreateIncidentResult{IncidentID: "inc-1"}, nil
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 5})
	w.now = func() time.Time { return fixedNow }
	w.RunOnce(context.Background())

	if csm.calls != 1 {
		t.Errorf("CreateIncident called %d times, want 1 — row is well past its backoff window", csm.calls)
	}
}

func TestRunOnce_CorruptPayloadIsMarkedFailedWithoutCallingCSM(t *testing.T) {
	row := store.AlertRecord{ID: "alert-1", Status: store.StatusPending, Payload: []byte(`not json`)}
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		return &csmclient.CreateIncidentResult{IncidentID: "inc-1"}, nil
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 5})
	w.RunOnce(context.Background())

	if csm.calls != 0 {
		t.Errorf("CreateIncident called %d times, want 0 for a corrupt buffered payload", csm.calls)
	}
	if len(s.failed) != 1 {
		t.Fatalf("failed = %+v, want one terminal-failure row for the corrupt payload", s.failed)
	}
	if len(tw.messages) != 0 {
		t.Error("Twilio should not be called for a corrupt payload")
	}
}

func TestRunOnce_StoreLoadErrorIsLoggedNotPanicked(t *testing.T) {
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return nil, errors.New("connection reset by peer")
	}}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		t.Fatal("CreateIncident should not be called when PendingBatch itself fails")
		return nil, nil
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{})
	w.RunOnce(context.Background()) // must not panic
}

// TestRunOnce_FirstAttemptNeverSearchesForDuplicate is the "nothing could
// exist yet on attempt 1" half of the pre-retry dedup contract: RetryCount
// == 0 must skip SearchIncidentByTag entirely, not just tolerate a
// not-found result from it.
func TestRunOnce_FirstAttemptNeverSearchesForDuplicate(t *testing.T) {
	row := rowWithPayload(t, "alert-1", 0, nil)
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		return &csmclient.CreateIncidentResult{IncidentID: "inc-1"}, nil
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 3})
	w.RunOnce(context.Background())

	if csm.searchCalls != 0 {
		t.Errorf("SearchIncidentByTag called %d times on a first attempt, want 0", csm.searchCalls)
	}
	if csm.calls != 1 {
		t.Errorf("CreateIncident called %d times, want 1", csm.calls)
	}
}

// TestRunOnce_RetryFindsExistingIncident_SkipsDuplicateCreate is the "search
// finds a match -> treat as delivered, no duplicate create" branch: a prior
// attempt's response was lost, but the incident it created is found by the
// dedup search, so this retry must not call CreateIncident again.
func TestRunOnce_RetryFindsExistingIncident_SkipsDuplicateCreate(t *testing.T) {
	row := rowWithPayload(t, "alert-1", 1, nil) // retryCount=1 -> this is a retry
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{
		createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
			t.Fatal("CreateIncident should not be called once the dedup search finds an existing incident")
			return nil, nil
		},
		searchFn: func(ctx context.Context, tag string) (*csmclient.CreateIncidentResult, bool, error) {
			return &csmclient.CreateIncidentResult{IncidentID: "inc-existing", IncidentNumber: "INC0009999"}, true, nil
		},
	}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 3})
	w.RunOnce(context.Background())

	if csm.searchCalls != 1 {
		t.Fatalf("SearchIncidentByTag called %d times, want 1", csm.searchCalls)
	}
	wantTag := csmclient.DedupTag("alert-1")
	if csm.searchTags[0] != wantTag {
		t.Errorf("search tag = %q, want %q", csm.searchTags[0], wantTag)
	}
	if csm.calls != 0 {
		t.Errorf("CreateIncident called %d times, want 0", csm.calls)
	}
	if len(s.delivered) != 1 || s.delivered[0].id != "alert-1" || s.delivered[0].incidentID != "inc-existing" {
		t.Errorf("delivered = %+v, want one row for alert-1/inc-existing", s.delivered)
	}
	if len(s.attemptFailed) != 0 || len(s.escalated) != 0 || len(s.failed) != 0 {
		t.Errorf("unexpected non-delivered transitions: attemptFailed=%v escalated=%v failed=%v", s.attemptFailed, s.escalated, s.failed)
	}
}

// TestRunOnce_RetrySearchFailsOpen_ProceedsToCreate covers both "no match"
// and "the search call itself errored" (e.g. the same 401 CreateIncident can
// return, see CreateIncident's doc comment) — both must fail open toward
// attempting delivery, not toward silently giving up.
func TestRunOnce_RetrySearchFailsOpen_ProceedsToCreate(t *testing.T) {
	cases := []struct {
		name     string
		searchFn func(ctx context.Context, tag string) (*csmclient.CreateIncidentResult, bool, error)
	}{
		{
			name: "no match found",
			searchFn: func(ctx context.Context, tag string) (*csmclient.CreateIncidentResult, bool, error) {
				return nil, false, nil
			},
		},
		{
			name: "search call itself errors (e.g. the same 401 CreateIncident gets today)",
			searchFn: func(ctx context.Context, tag string) (*csmclient.CreateIncidentResult, bool, error) {
				return nil, false, &apierror.Error{StatusCode: 401, Body: "Missing or invalid user ID token header."}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := rowWithPayload(t, "alert-1", 1, nil) // retryCount=1 -> this is a retry
			s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
				return []store.AlertRecord{row}, nil
			}}
			csm := &mockIncidentCreator{
				createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
					return &csmclient.CreateIncidentResult{IncidentID: "inc-new"}, nil
				},
				searchFn: tc.searchFn,
			}
			tw := &mockEscalator{}

			w := New(s, csm, tw, Config{MaxRetries: 3})
			w.RunOnce(context.Background())

			if csm.searchCalls != 1 {
				t.Fatalf("SearchIncidentByTag called %d times, want 1", csm.searchCalls)
			}
			if csm.calls != 1 {
				t.Errorf("CreateIncident called %d times, want 1 (fail-open must still attempt delivery)", csm.calls)
			}
			if len(s.delivered) != 1 || s.delivered[0].incidentID != "inc-new" {
				t.Errorf("delivered = %+v, want one row for alert-1/inc-new", s.delivered)
			}
		})
	}
}

// TestRunOnce_DefaultRetryBudgetEscalatesOnThirdFailure pins the reduced
// retry budget: with Config{} (defaults applied, MaxRetries=3), a row must
// escalate once its 3rd failure is reached, not its 5th.
func TestRunOnce_DefaultRetryBudgetEscalatesOnThirdFailure(t *testing.T) {
	// retryCount=2 -> nextRetryCount=3 >= default MaxRetries(3) -> escalate.
	row := rowWithPayload(t, "alert-1", 2, nil)
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		return nil, errors.New("connection refused")
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{}) // defaults: MaxRetries=3
	w.RunOnce(context.Background())

	if len(s.escalated) != 1 || s.escalated[0].id != "alert-1" {
		t.Fatalf("escalated = %+v, want one row for alert-1 on the 3rd failure with the default retry budget", s.escalated)
	}
	if len(tw.messages) != 1 {
		t.Fatalf("Twilio Escalate called %d times, want 1", len(tw.messages))
	}
}

// TestRunOnce_DefaultRetryBudgetDoesNotEscalateOnSecondFailure is the
// complement of the above: one failure short of the default budget must
// still just retry, not escalate.
func TestRunOnce_DefaultRetryBudgetDoesNotEscalateOnSecondFailure(t *testing.T) {
	// retryCount=1 -> nextRetryCount=2 < default MaxRetries(3) -> retry, not escalate.
	row := rowWithPayload(t, "alert-1", 1, nil)
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		return nil, errors.New("connection refused")
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{}) // defaults: MaxRetries=3
	w.RunOnce(context.Background())

	if len(s.attemptFailed) != 1 {
		t.Fatalf("attemptFailed = %+v, want one row for alert-1", s.attemptFailed)
	}
	if len(s.escalated) != 0 {
		t.Errorf("escalated = %+v, want zero — one failure short of the default retry budget", s.escalated)
	}
}

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{}.withDefaults()
	if cfg.MaxRetries != 3 {
		t.Errorf("default MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.BatchSize != 50 {
		t.Errorf("default BatchSize = %d, want 50", cfg.BatchSize)
	}
	if cfg.PollInterval != 15*time.Second {
		t.Errorf("default PollInterval = %v, want 15s", cfg.PollInterval)
	}
	if cfg.GroupWindow != 15*time.Minute {
		t.Errorf("default GroupWindow = %v, want 15m", cfg.GroupWindow)
	}
	if cfg.ServiceCacheTTL != 15*time.Minute {
		t.Errorf("default ServiceCacheTTL = %v, want 15m", cfg.ServiceCacheTTL)
	}
}

// strPtr is a small test-local pointer helper, matching the *string fields
// on csmclient.AlertIncidentMappingView / CreateAlertIncidentMappingRequest.
func strPtr(s string) *string { return &s }

// ---- Incident-grouping tests -------------------------------------------

// TestRunOnce_GroupsOntoEarlierOpenIncident_SkipsCreate is the "match found,
// and it's still open" branch: this alert must attach to the earlier
// alert's incident (record a mapping, mark delivered against that incident)
// and must never call CreateIncident at all.
func TestRunOnce_GroupsOntoEarlierOpenIncident_SkipsCreate(t *testing.T) {
	row := rowWithGroupablePayload(t, "alert-2", "azure", "uid-123")
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	wantTag := csmclient.GroupTag("azure", "uid-123")
	csm := &mockIncidentCreator{
		createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
			t.Fatal("CreateIncident should not be called once grouping finds an earlier, still-open incident")
			return nil, nil
		},
		searchGroupFn: func(ctx context.Context, tag string, since time.Time) (*csmclient.CreateIncidentResult, bool, error) {
			if tag != wantTag {
				t.Errorf("SearchOpenIncidentByGroupTag called with tag=%q, want %q", tag, wantTag)
			}
			return &csmclient.CreateIncidentResult{IncidentID: "inc-old", IncidentNumber: "INC0009999"}, true, nil
		},
	}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 3})
	w.RunOnce(context.Background())

	if csm.searchGroupCalls != 1 {
		t.Fatalf("SearchOpenIncidentByGroupTag called %d times, want 1", csm.searchGroupCalls)
	}
	if csm.calls != 0 {
		t.Errorf("CreateIncident called %d times, want 0", csm.calls)
	}
	if csm.createMappingCalls != 1 {
		t.Fatalf("CreateAlertIncidentMapping called %d times, want 1", csm.createMappingCalls)
	}
	if got := csm.createMappingReqs[0]; got.IncidentID != "inc-old" || got.AlertNumber != "alert-2" {
		t.Errorf("mapping request = %+v, want IncidentID=inc-old AlertNumber=alert-2", got)
	}
	if len(s.delivered) != 1 || s.delivered[0].id != "alert-2" || s.delivered[0].incidentID != "inc-old" {
		t.Errorf("delivered = %+v, want one row for alert-2/inc-old", s.delivered)
	}
	if csm.updateIncidentCalls != 1 {
		t.Fatalf("UpdateIncident called %d times, want 1 (group-attach must push a work note onto the existing incident)", csm.updateIncidentCalls)
	}
	if csm.updateIncidentIDs[0] != "inc-old" {
		t.Errorf("UpdateIncident incidentID = %q, want %q", csm.updateIncidentIDs[0], "inc-old")
	}
	if csm.updateIncidentNotes[0] == "" {
		t.Error("UpdateIncident workNotes = \"\", want a non-empty summary of the new alert")
	}
}

// TestRunOnce_GroupAttachWorkNoteFailureDoesNotBlockDelivery mirrors
// recordMapping's own already-tested failure-tolerance pattern: a failed
// UpdateIncident call must not prevent MarkDelivered or the
// CreateAlertIncidentMapping call — this is a best-effort, non-blocking side
// effect, not a precondition for the alert being considered delivered.
func TestRunOnce_GroupAttachWorkNoteFailureDoesNotBlockDelivery(t *testing.T) {
	row := rowWithGroupablePayload(t, "alert-2", "azure", "uid-123")
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{
		searchGroupFn: func(ctx context.Context, tag string, since time.Time) (*csmclient.CreateIncidentResult, bool, error) {
			return &csmclient.CreateIncidentResult{IncidentID: "inc-old", IncidentNumber: "INC0009999"}, true, nil
		},
		updateIncidentFn: func(ctx context.Context, incidentID, workNotes string) error {
			return errors.New("upstream unavailable")
		},
	}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 3})
	w.RunOnce(context.Background())

	if csm.updateIncidentCalls != 1 {
		t.Fatalf("UpdateIncident called %d times, want 1", csm.updateIncidentCalls)
	}
	if len(s.delivered) != 1 || s.delivered[0].id != "alert-2" || s.delivered[0].incidentID != "inc-old" {
		t.Errorf("delivered = %+v, want one row for alert-2/inc-old despite UpdateIncident failing", s.delivered)
	}
	if csm.createMappingCalls != 1 {
		t.Errorf("CreateAlertIncidentMapping called %d times, want 1 (must still run after UpdateIncident fails)", csm.createMappingCalls)
	}
}

// TestRunOnce_GroupSearchUsesConfiguredWindow confirms the "since" argument
// passed to SearchOpenIncidentByGroupTag is now-GroupWindow, using the
// worker's own injected clock (w.now) rather than a bare time.Now() at call
// time — this is what makes the search actually bounded, not just tagged.
func TestRunOnce_GroupSearchUsesConfiguredWindow(t *testing.T) {
	row := rowWithGroupablePayload(t, "alert-2", "azure", "uid-123")
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{
		createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
			return &csmclient.CreateIncidentResult{IncidentID: "inc-new"}, nil
		},
	}
	tw := &mockEscalator{}

	fixedNow := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	w := New(s, csm, tw, Config{MaxRetries: 3, GroupWindow: 20 * time.Minute})
	w.now = func() time.Time { return fixedNow }
	w.RunOnce(context.Background())

	if csm.searchGroupCalls != 1 {
		t.Fatalf("SearchOpenIncidentByGroupTag called %d times, want 1", csm.searchGroupCalls)
	}
	wantSince := fixedNow.Add(-20 * time.Minute)
	if !csm.searchGroupSince[0].Equal(wantSince) {
		t.Errorf("since = %v, want %v (fixedNow - 20m configured GroupWindow)", csm.searchGroupSince[0], wantSince)
	}
}

// TestRunOnce_GroupingFallsThroughOnNoMatchOrSearchFailure covers both
// "not groupable" branches: no matching incident found (already excludes
// closed/resolved/out-of-window incidents server-side, per the search's own
// state+createdOn filters), and the search call itself erroring (a
// fail-open case this feature could hit in production if it recurs — see
// tryGroup's doc comment and CreateIncident's doc comment). Both must fall
// through unchanged to the existing create-or-dedup-search flow:
// CreateIncident is still called exactly once, and the row is still
// delivered against the newly-created incident.
func TestRunOnce_GroupingFallsThroughOnNoMatchOrSearchFailure(t *testing.T) {
	cases := []struct {
		name          string
		searchGroupFn func(ctx context.Context, tag string, since time.Time) (*csmclient.CreateIncidentResult, bool, error)
	}{
		{
			name: "no matching incident found",
			searchGroupFn: func(ctx context.Context, tag string, since time.Time) (*csmclient.CreateIncidentResult, bool, error) {
				return nil, false, nil
			},
		},
		{
			name: "search call itself errors (e.g. the same 401 CreateIncident can return, see CreateIncident's doc comment)",
			searchGroupFn: func(ctx context.Context, tag string, since time.Time) (*csmclient.CreateIncidentResult, bool, error) {
				return nil, false, &apierror.Error{StatusCode: 401, Body: "Missing or invalid user ID token header."}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := rowWithGroupablePayload(t, "alert-2", "azure", "uid-123")
			s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
				return []store.AlertRecord{row}, nil
			}}
			csm := &mockIncidentCreator{
				createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
					return &csmclient.CreateIncidentResult{IncidentID: "inc-new", IncidentNumber: "INC0000001"}, nil
				},
				searchGroupFn: tc.searchGroupFn,
			}
			tw := &mockEscalator{}

			w := New(s, csm, tw, Config{MaxRetries: 3})
			w.RunOnce(context.Background())

			if csm.calls != 1 {
				t.Errorf("CreateIncident called %d times, want 1 (grouping must fall open to the normal create flow)", csm.calls)
			}
			if len(s.delivered) != 1 || s.delivered[0].incidentID != "inc-new" {
				t.Errorf("delivered = %+v, want one row for alert-2/inc-new", s.delivered)
			}
		})
	}
}

// TestRunOnce_NoUniqueIdentifierSkipsGroupingEntirely is the third branch:
// a row whose buffered payload carries no UniqueIdentifier must never call
// the grouping lookup at all — there is nothing to group against or by.
func TestRunOnce_NoUniqueIdentifierSkipsGroupingEntirely(t *testing.T) {
	row := rowWithPayload(t, "alert-1", 0, nil) // no UniqueIdentifier in this payload
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		return &csmclient.CreateIncidentResult{IncidentID: "inc-1", IncidentNumber: "INC0001"}, nil
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 3})
	w.RunOnce(context.Background())

	if csm.searchGroupCalls != 0 {
		t.Errorf("SearchOpenIncidentByGroupTag called %d times, want 0 for a row with no UniqueIdentifier", csm.searchGroupCalls)
	}
	if csm.calls != 1 {
		t.Errorf("CreateIncident called %d times, want 1", csm.calls)
	}
	if csm.updateIncidentCalls != 0 {
		t.Errorf("UpdateIncident called %d times, want 0 (this row never groups, so there is no group-attach work note to push)", csm.updateIncidentCalls)
	}
}

// TestRunOnce_GroupingSkippedWhenPersistedFieldsContainTagDelimiter covers a
// legacy row buffered before the ingress delimiter check existed: its
// persisted Source or UniqueIdentifier can still contain
// csmclient.TagDelimiterChars. Building a GroupTag from either unvalidated
// would let distinct (source, uniqueIdentifier) pairs collide on the same
// tag, so tryGroup must bypass the search entirely and fall through to the
// normal create/dedup path.
func TestRunOnce_GroupingSkippedWhenPersistedFieldsContainTagDelimiter(t *testing.T) {
	cases := []struct {
		name             string
		source           string
		uniqueIdentifier string
	}{
		{name: "delimiter in source", source: "a:b", uniqueIdentifier: "c"},
		{name: "delimiter in uniqueIdentifier", source: "a", uniqueIdentifier: "b:c"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := rowWithGroupablePayload(t, "alert-2", tc.source, tc.uniqueIdentifier)
			s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
				return []store.AlertRecord{row}, nil
			}}
			csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
				return &csmclient.CreateIncidentResult{IncidentID: "inc-new", IncidentNumber: "INC0000001"}, nil
			}}
			tw := &mockEscalator{}

			w := New(s, csm, tw, Config{MaxRetries: 3})
			w.RunOnce(context.Background())

			if csm.searchGroupCalls != 0 {
				t.Errorf("SearchOpenIncidentByGroupTag called %d times, want 0 when persisted fields contain a tag delimiter", csm.searchGroupCalls)
			}
			if csm.calls != 1 {
				t.Errorf("CreateIncident called %d times, want 1", csm.calls)
			}
		})
	}
}

// TestRunOnce_RecordsMappingAfterNewIncidentCreated_BestEffort covers
// attempt's post-create call: once a new incident is successfully created,
// a mapping row must be recorded against it (so a later related alert can
// find and group onto it), with the correct fields.
func TestRunOnce_RecordsMappingAfterNewIncidentCreated_BestEffort(t *testing.T) {
	row := rowWithGroupablePayload(t, "alert-2", "azure", "uid-123")
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		return &csmclient.CreateIncidentResult{IncidentID: "inc-new", IncidentNumber: "INC0000001"}, nil
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 3})
	w.RunOnce(context.Background())

	if csm.createMappingCalls != 1 {
		t.Fatalf("CreateAlertIncidentMapping called %d times, want 1", csm.createMappingCalls)
	}
	got := csm.createMappingReqs[0]
	if got.AlertNumber != "alert-2" || got.Source != "azure" {
		t.Errorf("mapping request AlertNumber/Source = %q/%q, want alert-2/azure", got.AlertNumber, got.Source)
	}
	if got.UniqueIdentifier == nil || *got.UniqueIdentifier != "uid-123" {
		t.Errorf("mapping request UniqueIdentifier = %v, want uid-123", got.UniqueIdentifier)
	}
	if got.AlertStatus != "FIRING" {
		t.Errorf("mapping request AlertStatus = %q, want FIRING", got.AlertStatus)
	}
	if got.IncidentID != "inc-new" {
		t.Errorf("mapping request IncidentID = %q, want inc-new", got.IncidentID)
	}
}

// TestRunOnce_MappingCreateFailureAfterNewIncidentDoesNotFailDelivery is the
// best-effort contract: the mapping-create call failing must not fail the
// overall delivery, must not prevent the row from being marked delivered,
// and must not trigger any retry/escalation/terminal-failure path — the
// incident already exists, which is the primary goal.
func TestRunOnce_MappingCreateFailureAfterNewIncidentDoesNotFailDelivery(t *testing.T) {
	row := rowWithGroupablePayload(t, "alert-2", "azure", "uid-123")
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{
		createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
			return &csmclient.CreateIncidentResult{IncidentID: "inc-new", IncidentNumber: "INC0000001"}, nil
		},
		createMappingFn: func(ctx context.Context, req csmclient.CreateAlertIncidentMappingRequest) (*csmclient.AlertIncidentMappingView, error) {
			return nil, errors.New("csm-integration-service: connection refused")
		},
	}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 3})
	w.RunOnce(context.Background())

	if len(s.delivered) != 1 || s.delivered[0].incidentID != "inc-new" {
		t.Fatalf("delivered = %+v, want one row for alert-2/inc-new even though the mapping-create call failed", s.delivered)
	}
	if len(s.attemptFailed) != 0 || len(s.escalated) != 0 || len(s.failed) != 0 {
		t.Errorf("unexpected non-delivered transitions: attemptFailed=%v escalated=%v failed=%v", s.attemptFailed, s.escalated, s.failed)
	}
	if len(tw.messages) != 0 {
		t.Error("Twilio should not be called just because the best-effort mapping-create call failed")
	}
}

// TestRunOnce_RecordsIncidentIDBeforeMarkDelivered confirms RecordIncidentID
// is called (durably persisting the incident id) as part of a normal
// successful create, ahead of/alongside MarkDelivered — see attempt's own
// comment for why this ordering is what closes the duplicate-incident gap
// on a later MarkDelivered failure.
func TestRunOnce_RecordsIncidentIDBeforeMarkDelivered(t *testing.T) {
	row := rowWithPayload(t, "alert-1", 0, nil)
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		return &csmclient.CreateIncidentResult{IncidentID: "inc-1", IncidentNumber: "INC0001"}, nil
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 3})
	w.RunOnce(context.Background())

	if len(s.recordedIncident) != 1 || s.recordedIncident[0].id != "alert-1" || s.recordedIncident[0].incidentID != "inc-1" {
		t.Errorf("recordedIncident = %+v, want one row for alert-1/inc-1", s.recordedIncident)
	}
	if len(s.delivered) != 1 || s.delivered[0].incidentID != "inc-1" {
		t.Errorf("delivered = %+v, want one row for alert-1/inc-1", s.delivered)
	}
}

// TestRunOnce_MarkDeliveredFailsAfterCreate_IncidentIDStillDurablyRecorded is
// the actual fix for the gap CodeRabbit flagged in review: even though
// MarkDelivered fails right after a successful CreateIncident, the incident
// id must already be durably persisted (via RecordIncidentID, called
// first) — so a later attempt for this row can retry MarkDelivered directly
// without ever risking a second CreateIncident call.
func TestRunOnce_MarkDeliveredFailsAfterCreate_IncidentIDStillDurablyRecorded(t *testing.T) {
	row := rowWithPayload(t, "alert-1", 0, nil)
	s := &mockStore{
		pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
			return []store.AlertRecord{row}, nil
		},
		markDeliveredErr: errors.New("db: connection reset"),
	}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		return &csmclient.CreateIncidentResult{IncidentID: "inc-1", IncidentNumber: "INC0001"}, nil
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 3})
	w.RunOnce(context.Background())

	if len(s.recordedIncident) != 1 || s.recordedIncident[0].incidentID != "inc-1" {
		t.Fatalf("recordedIncident = %+v, want the incident id durably recorded even though MarkDelivered failed", s.recordedIncident)
	}
	if len(s.delivered) != 0 {
		t.Errorf("delivered = %+v, want none — MarkDelivered failed", s.delivered)
	}
	if len(s.attemptFailed) != 1 || s.attemptFailed[0].id != "alert-1" {
		t.Errorf("attemptFailed = %+v, want one row for alert-1 (bounds the retry loop toward escalation)", s.attemptFailed)
	}
	if csm.calls != 1 {
		t.Errorf("CreateIncident called %d times, want exactly 1 for this attempt", csm.calls)
	}
}

// TestRunOnce_ShortCircuitsToMarkDeliveredWhenIncidentIDAlreadyRecorded is
// the other half of the fix: a row that already has an IncidentID persisted
// (from an earlier attempt whose MarkDelivered failed) must retry
// MarkDelivered directly on the next attempt, with CreateIncident never
// called again — no dependency on SearchIncidentByTag succeeding, unlike
// the pre-retry dedup check.
func TestRunOnce_ShortCircuitsToMarkDeliveredWhenIncidentIDAlreadyRecorded(t *testing.T) {
	row := rowWithPayload(t, "alert-1", 1, nil)
	row.IncidentID = "inc-already-created"
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		t.Fatal("CreateIncident must not be called when row.IncidentID is already set")
		return nil, nil
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 3})
	w.RunOnce(context.Background())

	if csm.calls != 0 {
		t.Errorf("CreateIncident called %d times, want 0", csm.calls)
	}
	if csm.searchCalls != 0 {
		t.Errorf("SearchIncidentByTag called %d times, want 0 — the durable short-circuit needs no network call to confirm this", csm.searchCalls)
	}
	if len(s.delivered) != 1 || s.delivered[0].id != "alert-1" || s.delivered[0].incidentID != "inc-already-created" {
		t.Errorf("delivered = %+v, want one row for alert-1/inc-already-created", s.delivered)
	}
	if csm.createMappingCalls != 1 || csm.createMappingReqs[0].IncidentID != "inc-already-created" {
		t.Errorf("createMappingReqs = %+v, want one call recording inc-already-created (the original attempt never reached this step)", csm.createMappingReqs)
	}
}

// TestRunOnce_ShortCircuitRetryFails_FallsBackToAttemptFailed confirms the
// short-circuit path is itself bounded: if retrying MarkDelivered for an
// already-recorded incident keeps failing (this service's own database,
// not CSM), the row still advances toward the normal retry-budget/
// escalation path rather than looping on it forever.
func TestRunOnce_ShortCircuitRetryFails_FallsBackToAttemptFailed(t *testing.T) {
	row := rowWithPayload(t, "alert-1", 1, nil)
	row.IncidentID = "inc-already-created"
	s := &mockStore{
		pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
			return []store.AlertRecord{row}, nil
		},
		markDeliveredErr: errors.New("db: connection reset"),
	}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		t.Fatal("CreateIncident must not be called when row.IncidentID is already set")
		return nil, nil
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 3})
	w.RunOnce(context.Background())

	if csm.calls != 0 {
		t.Errorf("CreateIncident called %d times, want 0", csm.calls)
	}
	if len(s.delivered) != 0 {
		t.Errorf("delivered = %+v, want none — MarkDelivered failed again", s.delivered)
	}
	if len(s.attemptFailed) != 1 || s.attemptFailed[0].id != "alert-1" {
		t.Errorf("attemptFailed = %+v, want one row for alert-1", s.attemptFailed)
	}
}

// TestRunOnce_ShortCircuitRetryBudgetExhausted_Escalates is the regression
// test for the bug this branch's own comment already warned about: it
// always returns, so nothing past it (including the normal path's
// nextRetryCount check) ever runs for this row. Without a budget check
// here too, a row stuck retrying MarkDelivered for an already-recorded
// incident would retry forever and never escalate, since CreateIncident is
// never called again once row.IncidentID is set.
func TestRunOnce_ShortCircuitRetryBudgetExhausted_Escalates(t *testing.T) {
	row := rowWithPayload(t, "alert-1", 2, nil) // RetryCount 2, MaxRetries 3 -> next attempt exhausts the budget
	row.IncidentID = "inc-already-created"
	s := &mockStore{
		pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
			return []store.AlertRecord{row}, nil
		},
		markDeliveredErr: errors.New("db: connection reset"),
	}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		t.Fatal("CreateIncident must not be called when row.IncidentID is already set")
		return nil, nil
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 3})
	w.RunOnce(context.Background())

	if len(tw.messages) != 1 {
		t.Fatalf("Escalate called %d times, want 1", len(tw.messages))
	}
	if len(s.escalated) != 1 || s.escalated[0].id != "alert-1" {
		t.Fatalf("escalated = %+v, want one row for alert-1", s.escalated)
	}
	if len(s.attemptFailed) != 0 {
		t.Errorf("attemptFailed = %+v, want none — the budget was exhausted, so this must escalate, not retry again", s.attemptFailed)
	}
	if len(s.delivered) != 0 {
		t.Errorf("delivered = %+v, want none — MarkDelivered failed again", s.delivered)
	}
}

// ----- hybrid service-UUID resolution (resolveServiceID) -----

// TestRunOnce_ServiceAlreadyResolved_NeverCallsSearchServices pins the
// static-map fast path's contract from the worker's side: a row whose
// buffered ServiceID is already a real value (internal/handler.MapToIncident's
// SRE_ALERT_SERVICE_MAP hit) must never trigger a live /services/search call
// at all — resolveServiceID is gated entirely on the sentinel check in
// attempt.
func TestRunOnce_ServiceAlreadyResolved_NeverCallsSearchServices(t *testing.T) {
	row := rowWithPayload(t, "alert-1", 0, nil) // serviceId is already "svc-1", not the sentinel
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
		if req.ServiceID != "svc-1" {
			t.Errorf("ServiceID = %q, want the row's already-resolved svc-1 left untouched", req.ServiceID)
		}
		return &csmclient.CreateIncidentResult{IncidentID: "inc-1"}, nil
	}}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 3, UnknownServiceID: "unknown-svc-uuid"})
	w.RunOnce(context.Background())

	if csm.searchServicesCalls != 0 {
		t.Errorf("SearchServices called %d times, want 0 — the row's ServiceID was already resolved", csm.searchServicesCalls)
	}
	if len(s.delivered) != 1 {
		t.Errorf("delivered = %+v, want one row", s.delivered)
	}
}

// TestRunOnce_UnresolvedService_LiveSearchHit_ResolvesAndCaches covers both
// "static-map miss + live-search hit + cache populated" and "cache hit on a
// second alert with the same label": the second RunOnce pass, for a
// different alert reporting the same Service label, must not call
// SearchServices again.
func TestRunOnce_UnresolvedService_LiveSearchHit_ResolvesAndCaches(t *testing.T) {
	row := rowWithUnresolvedService(t, "alert-1", "Azure Monitoring", 0)
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	var gotServiceID string
	csm := &mockIncidentCreator{
		createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
			gotServiceID = req.ServiceID
			return &csmclient.CreateIncidentResult{IncidentID: "inc-1"}, nil
		},
		searchServicesFn: func(ctx context.Context, label string) ([]csmclient.ITService, error) {
			if label != "Azure Monitoring" {
				t.Errorf("SearchServices label = %q, want %q", label, "Azure Monitoring")
			}
			return []csmclient.ITService{{ID: "33333333-3333-3333-3333-333333333333", Name: "Azure Monitoring"}}, nil
		},
	}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 3, UnknownServiceID: "unknown-svc-uuid"})
	w.RunOnce(context.Background())

	if csm.searchServicesCalls != 1 {
		t.Fatalf("SearchServices called %d times, want 1", csm.searchServicesCalls)
	}
	if gotServiceID != "33333333-3333-3333-3333-333333333333" {
		t.Errorf("CreateIncident ServiceID = %q, want the live-resolved UUID", gotServiceID)
	}
	if len(s.delivered) != 1 || s.delivered[0].id != "alert-1" {
		t.Errorf("delivered = %+v, want one row for alert-1", s.delivered)
	}

	// A second, different alert reporting the exact same label: the cache
	// populated above must be reused, not a second SearchServices call.
	row2 := rowWithUnresolvedService(t, "alert-2", "Azure Monitoring", 0)
	s.pendingBatchFn = func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row2}, nil
	}
	w.RunOnce(context.Background())

	if csm.searchServicesCalls != 1 {
		t.Errorf("SearchServices called %d times across two RunOnce passes for the same label, want 1 (cache hit on the second)", csm.searchServicesCalls)
	}
	if len(s.delivered) != 2 {
		t.Errorf("delivered = %+v, want two rows (alert-1 and alert-2)", s.delivered)
	}
}

// TestRunOnce_UnresolvedService_ZeroResult_FallsBackToUnknownServiceID pins
// the confirmed-zero-result fallback: SearchServices returning an empty,
// error-free slice must resolve to Config.UnknownServiceID, not be treated
// as a failure of any kind.
func TestRunOnce_UnresolvedService_ZeroResult_FallsBackToUnknownServiceID(t *testing.T) {
	row := rowWithUnresolvedService(t, "alert-1", "Totally Unknown Service", 0)
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	var gotServiceID string
	csm := &mockIncidentCreator{
		createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
			gotServiceID = req.ServiceID
			return &csmclient.CreateIncidentResult{IncidentID: "inc-1"}, nil
		},
		searchServicesFn: func(ctx context.Context, label string) ([]csmclient.ITService, error) {
			return nil, nil // confirmed zero-result: no match, no error
		},
	}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 3, UnknownServiceID: "unknown-svc-uuid"})
	w.RunOnce(context.Background())

	if gotServiceID != "unknown-svc-uuid" {
		t.Errorf("CreateIncident ServiceID = %q, want the configured unknown-service fallback %q", gotServiceID, "unknown-svc-uuid")
	}
	if len(s.delivered) != 1 {
		t.Errorf("delivered = %+v, want one row — a zero-result search is not a failure", s.delivered)
	}
}

// TestRunOnce_UnresolvedService_TransientSearchError_StaysRetryable pins the
// last required case: a transient error from SearchServices itself (as
// opposed to a confirmed zero-result) must be folded into the exact same
// retryable-delivery-failure path a CreateIncident error takes — retried,
// never marked permanently failed, and never silently bucketed into the
// unknown-service fallback.
func TestRunOnce_UnresolvedService_TransientSearchError_StaysRetryable(t *testing.T) {
	row := rowWithUnresolvedService(t, "alert-1", "Azure Monitoring", 0)
	s := &mockStore{pendingBatchFn: func(ctx context.Context, limit int) ([]store.AlertRecord, error) {
		return []store.AlertRecord{row}, nil
	}}
	csm := &mockIncidentCreator{
		createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
			t.Fatal("CreateIncident must not be called when service resolution itself failed")
			return nil, nil
		},
		searchServicesFn: func(ctx context.Context, label string) ([]csmclient.ITService, error) {
			return nil, errors.New("connection refused")
		},
	}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 3, UnknownServiceID: "unknown-svc-uuid"})
	w.RunOnce(context.Background())

	if len(s.attemptFailed) != 1 || s.attemptFailed[0].id != "alert-1" {
		t.Errorf("attemptFailed = %+v, want one retryable failure for alert-1", s.attemptFailed)
	}
	if len(s.failed) != 0 {
		t.Errorf("failed = %+v, want none — a transient search error is retryable, not terminal", s.failed)
	}
	if len(s.delivered) != 0 {
		t.Errorf("delivered = %+v, want none", s.delivered)
	}
}
