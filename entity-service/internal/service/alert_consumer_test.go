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
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// fakeAlertReceiver hands out queued messages and records how each one was settled.
type fakeAlertReceiver struct {
	queue chan *azservicebus.ReceivedMessage

	mu           sync.Mutex
	completed    []string
	abandoned    []string
	deadLettered map[string]string
	renewals     int
	renewErr     error
}

func newFakeAlertReceiver(messages ...*azservicebus.ReceivedMessage) *fakeAlertReceiver {
	r := &fakeAlertReceiver{queue: make(chan *azservicebus.ReceivedMessage, 1024), deadLettered: map[string]string{}}
	for _, m := range messages {
		r.queue <- m
	}
	return r
}

func (r *fakeAlertReceiver) ReceiveMessages(ctx context.Context, maxMessages int, _ *azservicebus.ReceiveMessagesOptions) ([]*azservicebus.ReceivedMessage, error) {
	var out []*azservicebus.ReceivedMessage
	select {
	case m := <-r.queue:
		out = append(out, m)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	for len(out) < maxMessages {
		select {
		case m := <-r.queue:
			out = append(out, m)
		default:
			return out, nil
		}
	}
	return out, nil
}

func (r *fakeAlertReceiver) CompleteMessage(_ context.Context, m *azservicebus.ReceivedMessage, _ *azservicebus.CompleteMessageOptions) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.completed = append(r.completed, m.MessageID)
	return nil
}

func (r *fakeAlertReceiver) AbandonMessage(_ context.Context, m *azservicebus.ReceivedMessage, _ *azservicebus.AbandonMessageOptions) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.abandoned = append(r.abandoned, m.MessageID)
	return nil
}

func (r *fakeAlertReceiver) DeadLetterMessage(_ context.Context, m *azservicebus.ReceivedMessage, o *azservicebus.DeadLetterOptions) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deadLettered[m.MessageID] = *o.Reason
	return nil
}

func (r *fakeAlertReceiver) RenewMessageLock(_ context.Context, m *azservicebus.ReceivedMessage, _ *azservicebus.RenewMessageLockOptions) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.renewErr != nil {
		return r.renewErr
	}
	r.renewals++
	lockedUntil := time.Now().Add(time.Minute)
	m.LockedUntil = &lockedUntil
	return nil
}

func (r *fakeAlertReceiver) renewCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.renewals
}

func (r *fakeAlertReceiver) settledCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.completed) + len(r.abandoned) + len(r.deadLettered)
}

// fakeAlertRepo stores rows in memory with the table's unique message key, and can fail its first calls.
type fakeAlertRepo struct {
	mu            sync.Mutex
	seq           int64
	rows          map[string]domain.NewAlert
	failInserts   int
	failReserves  int
	insertCalls   int
	maxConcurrent int
	inFlight      int
}

func newFakeAlertRepo() *fakeAlertRepo {
	return &fakeAlertRepo{rows: map[string]domain.NewAlert{}}
}

func (r *fakeAlertRepo) ReserveAlertIDs(_ context.Context, n int) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failReserves > 0 {
		r.failReserves--
		return nil, errors.New("connection refused")
	}
	ids := make([]string, n)
	for i := range ids {
		r.seq++
		ids[i] = repository.FormatAlertID(r.seq)
	}
	return ids, nil
}

func (r *fakeAlertRepo) InsertAlerts(_ context.Context, alerts []domain.NewAlert) (int64, error) {
	r.mu.Lock()
	r.insertCalls++
	if r.failInserts > 0 {
		r.failInserts--
		r.mu.Unlock()
		return 0, errors.New("connection reset")
	}
	r.inFlight++
	r.maxConcurrent = max(r.maxConcurrent, r.inFlight)
	r.mu.Unlock()

	time.Sleep(2 * time.Millisecond)

	r.mu.Lock()
	defer r.mu.Unlock()
	r.inFlight--
	var stored int64
	for _, a := range alerts {
		key := fmt.Sprintf("%s/%d", a.MessageID, a.MessageIndex)
		if _, exists := r.rows[key]; exists {
			continue
		}
		r.rows[key] = a
		stored++
	}
	return stored, nil
}

func (r *fakeAlertRepo) rowCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.rows)
}

func mustRegistry(t *testing.T) *alertsource.Registry {
	t.Helper()
	registry, err := alertsource.New(nil)
	if err != nil {
		t.Fatalf("alertsource.New: %v", err)
	}
	return registry
}

func alertMessage(id, source, body string) *azservicebus.ReceivedMessage {
	return &azservicebus.ReceivedMessage{
		MessageID: id,
		Body:      []byte(body),
		ApplicationProperties: map[string]any{
			"source":      source,
			"received_at": "2026-11-13T10:00:00.1234567Z",
		},
	}
}

const canonicalAlert = `{"service":"svc-a","metric_name":"cpu","severity":"critical","category":"infra","environment":"prod","source":"Test","description":"high cpu"}`

func newTestConsumer(t *testing.T, repo AlertStore, high, standard AlertReceiver, workers, weight int) *AlertConsumer {
	t.Helper()
	c := NewAlertConsumer(repo, mustRegistry(t), high, "priority-queue", standard, "standard-queue", AlertConsumerConfig{Concurrency: workers, PriorityWeight: weight})
	c.sleep = func(ctx context.Context, _ time.Duration) bool { return ctx.Err() == nil }
	return c
}

func TestAlertConsumerProcess_StoresAlertLikeIngestion(t *testing.T) {
	repo := newFakeAlertRepo()
	c := newTestConsumer(t, repo, newFakeAlertReceiver(), newFakeAlertReceiver(), 1, 4)

	decision := c.process(context.Background(), domain.AlertTierHigh, alertMessage("m-1", "ServiceNow", canonicalAlert))
	if decision.settlement != alertComplete || decision.stored != 1 {
		t.Fatalf("decision = %+v, want complete with one stored alert", decision)
	}
	row := repo.rows["m-1/0"]
	if row.ID != "ALT000000001" || row.Source != "servicenow" || row.Tier != domain.AlertTierHigh {
		t.Fatalf("row = %+v, want ALT000000001 from servicenow on the high tier", row)
	}
	var stored model.Alert
	if err := json.Unmarshal(row.Alert, &stored); err != nil {
		t.Fatalf("stored alert is not JSON: %v", err)
	}
	if stored.UniqueIdentifier != row.ID {
		t.Fatalf("unique_identifier = %q, want the alert's own id when the source sent none", stored.UniqueIdentifier)
	}
	if row.Fingerprint != model.Fingerprint(stored) {
		t.Fatal("fingerprint was not computed from the stored alert")
	}
	if want := time.Date(2026, 11, 13, 10, 0, 0, 123456700, time.UTC); !row.ReceivedOn.Equal(want) {
		t.Fatalf("received_on = %s, want the gateway's %s", row.ReceivedOn, want)
	}
}

func TestAlertConsumerProcess_KeepsSourceIdentifier(t *testing.T) {
	repo := newFakeAlertRepo()
	c := newTestConsumer(t, repo, newFakeAlertReceiver(), newFakeAlertReceiver(), 1, 4)
	body := `{"service":"svc-a","metric_name":"cpu","severity":"critical","environment":"prod","source":"Test","unique_identifier":"alarm-7"}`

	c.process(context.Background(), domain.AlertTierDefault, alertMessage("m-1", "servicenow", body))
	var stored model.Alert
	_ = json.Unmarshal(repo.rows["m-1/0"].Alert, &stored)
	if stored.UniqueIdentifier != "alarm-7" {
		t.Fatalf("unique_identifier = %q, want the source's own alarm-7", stored.UniqueIdentifier)
	}
}

func TestAlertConsumerProcess_StoresEveryAlertOfABatch(t *testing.T) {
	repo := newFakeAlertRepo()
	c := newTestConsumer(t, repo, newFakeAlertReceiver(), newFakeAlertReceiver(), 1, 4)

	decision := c.process(context.Background(), domain.AlertTierDefault, alertMessage("m-1", "servicenow", "["+canonicalAlert+","+canonicalAlert+"]"))
	if decision.stored != 2 || repo.rowCount() != 2 {
		t.Fatalf("stored %d rows, want 2", repo.rowCount())
	}
	if repo.rows["m-1/0"].ID == repo.rows["m-1/1"].ID {
		t.Fatal("two alerts of one message share an id")
	}
}

func TestAlertConsumerProcess_RedeliveryStoresNothingNew(t *testing.T) {
	repo := newFakeAlertRepo()
	c := newTestConsumer(t, repo, newFakeAlertReceiver(), newFakeAlertReceiver(), 1, 4)
	message := alertMessage("m-1", "servicenow", "["+canonicalAlert+","+canonicalAlert+"]")

	c.process(context.Background(), domain.AlertTierDefault, message)
	again := c.process(context.Background(), domain.AlertTierDefault, message)
	if again.settlement != alertComplete || again.stored != 0 {
		t.Fatalf("redelivery decision = %+v, want complete with nothing new stored", again)
	}
	if repo.rowCount() != 2 {
		t.Fatalf("rows = %d after a redelivery, want 2", repo.rowCount())
	}
}

func TestAlertConsumerProcess_DeadLettersPayloadsThatCanNeverBeStored(t *testing.T) {
	cases := map[string]struct {
		message *azservicebus.ReceivedMessage
		reason  string
	}{
		"no source":      {&azservicebus.ReceivedMessage{MessageID: "m", Body: []byte(canonicalAlert)}, alertDeadLetterMissingSource},
		"unknown source": {alertMessage("m", "nagios", canonicalAlert), alertDeadLetterUnknownSource},
		"bad body":       {alertMessage("m", "servicenow", "not json"), alertDeadLetterTransformFailed},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := newFakeAlertRepo()
			c := newTestConsumer(t, repo, newFakeAlertReceiver(), newFakeAlertReceiver(), 1, 4)
			decision := c.process(context.Background(), domain.AlertTierHigh, tc.message)
			if decision.settlement != alertDeadLetter || decision.reason != tc.reason {
				t.Fatalf("decision = %+v, want dead-letter with %s", decision, tc.reason)
			}
			if repo.rowCount() != 0 {
				t.Fatal("a rejected message stored a row")
			}
		})
	}
}

func TestAlertConsumerProcess_RetriesAShortStoreOutage(t *testing.T) {
	repo := newFakeAlertRepo()
	repo.failInserts = 2
	c := newTestConsumer(t, repo, newFakeAlertReceiver(), newFakeAlertReceiver(), 1, 4)

	decision := c.process(context.Background(), domain.AlertTierHigh, alertMessage("m-1", "servicenow", canonicalAlert))
	if decision.settlement != alertComplete || repo.rowCount() != 1 {
		t.Fatalf("decision = %+v with %d rows, want the third attempt to store the alert", decision, repo.rowCount())
	}
}

func TestAlertConsumerProcess_AbandonsWhenTheStoreStaysDown(t *testing.T) {
	for name, repo := range map[string]*fakeAlertRepo{
		"reserve": {rows: map[string]domain.NewAlert{}, failReserves: 99},
		"insert":  {rows: map[string]domain.NewAlert{}, failInserts: 99},
	} {
		t.Run(name, func(t *testing.T) {
			c := newTestConsumer(t, repo, newFakeAlertReceiver(), newFakeAlertReceiver(), 1, 4)
			decision := c.process(context.Background(), domain.AlertTierHigh, alertMessage("m-1", "servicenow", canonicalAlert))
			if decision.settlement != alertAbandon {
				t.Fatalf("decision = %+v, want abandon so Service Bus redelivers it", decision)
			}
		})
	}
}

func TestAlertConsumerRun_DrainsBothQueuesConcurrently(t *testing.T) {
	const perQueue = 40
	var highMessages, standardMessages []*azservicebus.ReceivedMessage
	for i := range perQueue {
		highMessages = append(highMessages, alertMessage(fmt.Sprintf("h-%d", i), "servicenow", canonicalAlert))
		standardMessages = append(standardMessages, alertMessage(fmt.Sprintf("s-%d", i), "servicenow", canonicalAlert))
	}
	high, standard := newFakeAlertReceiver(highMessages...), newFakeAlertReceiver(standardMessages...)
	repo := newFakeAlertRepo()
	c := newTestConsumer(t, repo, high, standard, 8, 4)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.Run(ctx)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for high.settledCount()+standard.settledCount() < 2*perQueue {
		if time.Now().After(deadline) {
			t.Fatalf("settled %d of %d messages", high.settledCount()+standard.settledCount(), 2*perQueue)
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the context was cancelled")
	}

	if len(high.completed) != perQueue || len(standard.completed) != perQueue {
		t.Fatalf("completed %d high and %d standard, want %d each", len(high.completed), len(standard.completed), perQueue)
	}
	if repo.rowCount() != 2*perQueue {
		t.Fatalf("stored %d rows, want %d", repo.rowCount(), 2*perQueue)
	}
	if repo.maxConcurrent < 2 {
		t.Fatalf("at most %d insert ran at once, want concurrent processing", repo.maxConcurrent)
	}
	for _, row := range repo.rows {
		if (row.MessageID[0] == 'h') != (row.Tier == domain.AlertTierHigh) {
			t.Fatalf("message %s was stored on the %s tier", row.MessageID, row.Tier)
		}
	}
}

func TestAlertConsumer_ShutdownHandsBufferedMessagesBack(t *testing.T) {
	high := newFakeAlertReceiver()
	c := newTestConsumer(t, newFakeAlertRepo(), high, newFakeAlertReceiver(), 2, 4)
	lane := c.lanes[domain.AlertTierHigh]
	lane.messages <- alertMessage("waiting-1", "servicenow", canonicalAlert)
	lane.messages <- alertMessage("waiting-2", "servicenow", canonicalAlert)

	c.abandonBuffered(lane)
	if len(high.abandoned) != 2 || len(lane.messages) != 0 {
		t.Fatalf("abandoned %v with %d still buffered, want both handed back", high.abandoned, len(lane.messages))
	}
}

func TestMessageStringProperty(t *testing.T) {
	properties := map[string]any{"source": "  azure ", "count": 3, "empty": nil}
	if got := messageStringProperty(properties, "source"); got != "azure" {
		t.Fatalf("source = %q", got)
	}
	if got := messageStringProperty(properties, "count"); got != "3" {
		t.Fatalf("count = %q", got)
	}
	if got := messageStringProperty(properties, "empty") + messageStringProperty(properties, "missing"); got != "" {
		t.Fatalf("absent properties = %q, want empty", got)
	}
}

// slowAlertRepo holds every insert for a while, standing in for a slow database.
type slowAlertRepo struct {
	*fakeAlertRepo
	delay time.Duration
}

func (r slowAlertRepo) InsertAlerts(ctx context.Context, alerts []domain.NewAlert) (int64, error) {
	time.Sleep(r.delay)
	return r.fakeAlertRepo.InsertAlerts(ctx, alerts)
}

func lockedFor(m *azservicebus.ReceivedMessage, d time.Duration) *azservicebus.ReceivedMessage {
	lockedUntil := time.Now().Add(d)
	m.LockedUntil = &lockedUntil
	return m
}

func TestAlertConsumerHandle_RenewsTheLockOfAMessageThatWaitedInTheBuffer(t *testing.T) {
	high := newFakeAlertReceiver()
	c := newTestConsumer(t, newFakeAlertRepo(), high, newFakeAlertReceiver(), 1, 4)

	c.handle(context.Background(), c.lanes[domain.AlertTierHigh], lockedFor(alertMessage("m-1", "servicenow", canonicalAlert), 5*time.Second))
	if high.renewCount() == 0 || len(high.completed) != 1 {
		t.Fatalf("renewals %d, completed %v; want the near-expiry lock renewed before the message is completed", high.renewCount(), high.completed)
	}
}

func TestAlertConsumerHandle_KeepsTheLockDuringSlowProcessing(t *testing.T) {
	high := newFakeAlertReceiver()
	c := newTestConsumer(t, slowAlertRepo{fakeAlertRepo: newFakeAlertRepo(), delay: 300 * time.Millisecond}, high, newFakeAlertReceiver(), 1, 4)
	c.lockRenewMargin = 950 * time.Millisecond

	c.handle(context.Background(), c.lanes[domain.AlertTierHigh], lockedFor(alertMessage("m-1", "servicenow", canonicalAlert), time.Second))
	if high.renewCount() == 0 || len(high.completed) != 1 {
		t.Fatalf("renewals %d, completed %v; want the lock renewed while the insert ran", high.renewCount(), high.completed)
	}
}

func TestAlertConsumerHandle_SkipsAMessageWhoseLockIsAlreadyLost(t *testing.T) {
	high := newFakeAlertReceiver()
	high.renewErr = errors.New("lock lost")
	repo := newFakeAlertRepo()
	c := newTestConsumer(t, repo, high, newFakeAlertReceiver(), 1, 4)

	c.handle(context.Background(), c.lanes[domain.AlertTierHigh], lockedFor(alertMessage("m-1", "servicenow", canonicalAlert), -time.Second))
	if repo.rowCount() != 0 || high.settledCount() != 0 {
		t.Fatal("a message whose lock was lost was still processed or settled")
	}
}

func TestAlertConsumerProcess_DeadLettersATransformThatPanics(t *testing.T) {
	c := newTestConsumer(t, newFakeAlertRepo(), newFakeAlertReceiver(), newFakeAlertReceiver(), 1, 4)
	c.transforms = panickingTransforms{}

	decision := c.process(context.Background(), domain.AlertTierHigh, alertMessage("m-1", "anything", canonicalAlert))
	if decision.settlement != alertDeadLetter || decision.reason != alertDeadLetterTransformFailed {
		t.Fatalf("decision = %+v, want dead-letter instead of a crash", decision)
	}
}

type panickingTransforms struct{}

func (panickingTransforms) Lookup(string) (alertsource.Transform, bool) {
	return func([]byte) ([]model.Alert, error) { panic("assignment to entry in nil map") }, true
}
