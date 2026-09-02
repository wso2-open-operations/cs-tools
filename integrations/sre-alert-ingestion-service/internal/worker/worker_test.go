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

	delivered     []struct{ id, incidentID string }
	attemptFailed []struct{ id, lastError string }
	escalated     []struct{ id, lastError string }
	failed        []struct{ id, lastError string }
}

func (m *mockStore) PendingBatch(ctx context.Context, limit int) ([]store.AlertRecord, error) {
	return m.pendingBatchFn(ctx, limit)
}

func (m *mockStore) MarkDelivered(ctx context.Context, id, incidentID string) error {
	m.delivered = append(m.delivered, struct{ id, incidentID string }{id, incidentID})
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

	// searchOpenFn is optional; when nil, SearchOpenIncidentByNumber reports
	// "no match, no error" — the common case for tests that never reach the
	// incident-grouping open-state confirmation step at all.
	searchOpenFn    func(ctx context.Context, number string) (*csmclient.CreateIncidentResult, bool, error)
	searchOpenCalls int
	searchOpenNums  []string

	// lookupMappingsFn is optional; when nil, LookupAlertIncidentMappings
	// reports "no mappings, no error" — the common case for tests where a
	// row has no UniqueIdentifier at all, so this is never called.
	lookupMappingsFn    func(ctx context.Context, source, uniqueIdentifier string) ([]csmclient.AlertIncidentMappingView, error)
	lookupMappingsCalls int

	// createMappingFn is optional; when nil, CreateAlertIncidentMapping
	// reports success with no error.
	createMappingFn    func(ctx context.Context, req csmclient.CreateAlertIncidentMappingRequest) (*csmclient.AlertIncidentMappingView, error)
	createMappingCalls int
	createMappingReqs  []csmclient.CreateAlertIncidentMappingRequest
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

func (m *mockIncidentCreator) SearchOpenIncidentByNumber(ctx context.Context, number string) (*csmclient.CreateIncidentResult, bool, error) {
	m.searchOpenCalls++
	m.searchOpenNums = append(m.searchOpenNums, number)
	if m.searchOpenFn != nil {
		return m.searchOpenFn(ctx, number)
	}
	return nil, false, nil
}

func (m *mockIncidentCreator) LookupAlertIncidentMappings(ctx context.Context, source, uniqueIdentifier string) ([]csmclient.AlertIncidentMappingView, error) {
	m.lookupMappingsCalls++
	if m.lookupMappingsFn != nil {
		return m.lookupMappingsFn(ctx, source, uniqueIdentifier)
	}
	return nil, nil
}

func (m *mockIncidentCreator) CreateAlertIncidentMapping(ctx context.Context, req csmclient.CreateAlertIncidentMappingRequest) (*csmclient.AlertIncidentMappingView, error) {
	m.createMappingCalls++
	m.createMappingReqs = append(m.createMappingReqs, req)
	if m.createMappingFn != nil {
		return m.createMappingFn(ctx, req)
	}
	return &csmclient.AlertIncidentMappingView{}, nil
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
// CreateIncident always 401s today (missing end-user identity forwarding),
// and that must be treated exactly like any other transient
// CSM-unavailability signal — retried, not treated as a permanent failure.
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
	if len(s.delivered) != 0 && len(s.attemptFailed) != 0 {
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
// and "the search call itself errored" (e.g. the same 401 CreateIncident
// gets today) — both must fail open toward attempting delivery, not toward
// silently giving up.
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
	csm := &mockIncidentCreator{
		createFn: func(ctx context.Context, req csmclient.CreateIncidentRequest) (*csmclient.CreateIncidentResult, error) {
			t.Fatal("CreateIncident should not be called once grouping finds an earlier, still-open incident")
			return nil, nil
		},
		lookupMappingsFn: func(ctx context.Context, source, uniqueIdentifier string) ([]csmclient.AlertIncidentMappingView, error) {
			if source != "azure" || uniqueIdentifier != "uid-123" {
				t.Errorf("lookup called with source=%q uniqueIdentifier=%q, want azure/uid-123", source, uniqueIdentifier)
			}
			return []csmclient.AlertIncidentMappingView{
				{ID: "map-1", IncidentID: "inc-old", IncidentNumber: strPtr("INC0009999")},
			}, nil
		},
		searchOpenFn: func(ctx context.Context, number string) (*csmclient.CreateIncidentResult, bool, error) {
			if number != "INC0009999" {
				t.Errorf("SearchOpenIncidentByNumber called with %q, want INC0009999", number)
			}
			return &csmclient.CreateIncidentResult{IncidentID: "inc-old", IncidentNumber: "INC0009999"}, true, nil
		},
	}
	tw := &mockEscalator{}

	w := New(s, csm, tw, Config{MaxRetries: 3})
	w.RunOnce(context.Background())

	if csm.lookupMappingsCalls != 1 {
		t.Fatalf("LookupAlertIncidentMappings called %d times, want 1", csm.lookupMappingsCalls)
	}
	if csm.searchOpenCalls != 1 {
		t.Fatalf("SearchOpenIncidentByNumber called %d times, want 1", csm.searchOpenCalls)
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
}

// TestRunOnce_GroupingFallsThroughOnNoMatchOrClosedOrLookupOrStateCheckFailure
// covers all of the "not groupable" branches at once: no earlier mapping,
// a mapping whose incident is confirmed no longer open, the lookup call
// itself erroring, and the open-state confirmation call itself erroring
// (the fail-open case this feature will actually hit in production today —
// see tryGroup's doc comment). Every one of these must fall through
// unchanged to the existing create-or-dedup-search flow: CreateIncident is
// still called exactly once, and the row is still delivered against the
// newly-created incident.
func TestRunOnce_GroupingFallsThroughOnNoMatchOrClosedOrLookupOrStateCheckFailure(t *testing.T) {
	cases := []struct {
		name             string
		lookupMappingsFn func(ctx context.Context, source, uniqueIdentifier string) ([]csmclient.AlertIncidentMappingView, error)
		searchOpenFn     func(ctx context.Context, number string) (*csmclient.CreateIncidentResult, bool, error)
	}{
		{
			name: "no earlier mapping found",
			lookupMappingsFn: func(ctx context.Context, source, uniqueIdentifier string) ([]csmclient.AlertIncidentMappingView, error) {
				return nil, nil
			},
		},
		{
			name: "mapping found but its incident is no longer open",
			lookupMappingsFn: func(ctx context.Context, source, uniqueIdentifier string) ([]csmclient.AlertIncidentMappingView, error) {
				return []csmclient.AlertIncidentMappingView{{ID: "map-1", IncidentID: "inc-old", IncidentNumber: strPtr("INC0009999")}}, nil
			},
			searchOpenFn: func(ctx context.Context, number string) (*csmclient.CreateIncidentResult, bool, error) {
				return nil, false, nil // resolved/closed/cancelled -> no longer open
			},
		},
		{
			name: "lookup call itself errors",
			lookupMappingsFn: func(ctx context.Context, source, uniqueIdentifier string) ([]csmclient.AlertIncidentMappingView, error) {
				return nil, errors.New("connection refused")
			},
		},
		{
			name: "open-state confirmation call itself errors (e.g. the same 401 CreateIncident gets today)",
			lookupMappingsFn: func(ctx context.Context, source, uniqueIdentifier string) ([]csmclient.AlertIncidentMappingView, error) {
				return []csmclient.AlertIncidentMappingView{{ID: "map-1", IncidentID: "inc-old", IncidentNumber: strPtr("INC0009999")}}, nil
			},
			searchOpenFn: func(ctx context.Context, number string) (*csmclient.CreateIncidentResult, bool, error) {
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
				lookupMappingsFn: tc.lookupMappingsFn,
				searchOpenFn:     tc.searchOpenFn,
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

	if csm.lookupMappingsCalls != 0 {
		t.Errorf("LookupAlertIncidentMappings called %d times, want 0 for a row with no UniqueIdentifier", csm.lookupMappingsCalls)
	}
	if csm.searchOpenCalls != 0 {
		t.Errorf("SearchOpenIncidentByNumber called %d times, want 0 for a row with no UniqueIdentifier", csm.searchOpenCalls)
	}
	if csm.calls != 1 {
		t.Errorf("CreateIncident called %d times, want 1", csm.calls)
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
