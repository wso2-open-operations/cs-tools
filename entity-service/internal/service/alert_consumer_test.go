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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// fakeAlertQueue hands out queued messages and records how each one was settled, standing in for either cloud.
type fakeAlertQueue struct {
	name  string
	queue chan *domain.AlertQueueMessage

	mu           sync.Mutex
	completed    []string
	abandoned    []string
	deadLettered map[string]string
	renewals     int
	renewErr     error
}

func newFakeAlertQueue(name string, messages ...*domain.AlertQueueMessage) *fakeAlertQueue {
	q := &fakeAlertQueue{name: name, queue: make(chan *domain.AlertQueueMessage, 1024), deadLettered: map[string]string{}}
	for _, m := range messages {
		q.queue <- m
	}
	return q
}

func (q *fakeAlertQueue) Name() string { return q.name }

func (q *fakeAlertQueue) Receive(ctx context.Context, maxMessages int) ([]*domain.AlertQueueMessage, error) {
	var out []*domain.AlertQueueMessage
	select {
	case m := <-q.queue:
		out = append(out, m)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	for len(out) < maxMessages {
		select {
		case m := <-q.queue:
			out = append(out, m)
		default:
			return out, nil
		}
	}
	return out, nil
}

func (q *fakeAlertQueue) Complete(_ context.Context, m *domain.AlertQueueMessage) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.completed = append(q.completed, m.ID)
	return nil
}

func (q *fakeAlertQueue) Abandon(_ context.Context, m *domain.AlertQueueMessage) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.abandoned = append(q.abandoned, m.ID)
	return nil
}

func (q *fakeAlertQueue) DeadLetter(_ context.Context, m *domain.AlertQueueMessage, reason, _ string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.deadLettered[m.ID] = reason
	return nil
}

func (q *fakeAlertQueue) RenewLock(_ context.Context, m *domain.AlertQueueMessage) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.renewErr != nil {
		return q.renewErr
	}
	q.renewals++
	lockedUntil := time.Now().Add(time.Minute)
	m.LockedUntil = &lockedUntil
	return nil
}

func (q *fakeAlertQueue) renewCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.renewals
}

func (q *fakeAlertQueue) completedCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.completed)
}

func (q *fakeAlertQueue) settledCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.completed) + len(q.abandoned) + len(q.deadLettered)
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
	return alertsource.New("00000000-0000-4000-8000-000000000001", nil)
}

func alertMessage(id, source, body string) *domain.AlertQueueMessage {
	return &domain.AlertQueueMessage{
		ID:   id,
		Body: []byte(body),
		Properties: map[string]string{
			"source":      source,
			"received_at": "2026-11-13T10:00:00.1234567Z",
		},
	}
}

const canonicalAlert = `{"service":"svc-a","metric_name":"cpu","severity":"critical","category":"infra","environment":"prod","source":"Test","description":"high cpu"}`

func newTestConsumer(t *testing.T, repo AlertStore, priority, standard []AlertQueue, workers, weight int) *AlertConsumer {
	t.Helper()
	c := NewAlertConsumer(repo, mustRegistry(t), priority, standard, AlertConsumerConfig{Concurrency: workers, PriorityWeight: weight})
	c.sleep = func(ctx context.Context, _ time.Duration) bool { return ctx.Err() == nil }
	return c
}

func singleQueueConsumer(t *testing.T, repo AlertStore) (*AlertConsumer, *fakeAlertQueue) {
	t.Helper()
	priority := newFakeAlertQueue("priority")
	return newTestConsumer(t, repo, []AlertQueue{priority}, []AlertQueue{newFakeAlertQueue("standard")}, 1, 4), priority
}

func TestAlertConsumerProcess_StoresAlertLikeIngestion(t *testing.T) {
	repo := newFakeAlertRepo()
	c, _ := singleQueueConsumer(t, repo)

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

func TestAlertConsumerProcess_FallsBackToTheQueueTime(t *testing.T) {
	repo := newFakeAlertRepo()
	c, _ := singleQueueConsumer(t, repo)
	enqueuedOn := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	message := alertMessage("m-1", "servicenow", canonicalAlert)
	delete(message.Properties, "received_at")
	message.EnqueuedOn = &enqueuedOn

	c.process(context.Background(), domain.AlertTierHigh, message)
	if got := repo.rows["m-1/0"].ReceivedOn; !got.Equal(enqueuedOn) {
		t.Fatalf("received_on = %s, want the queue's %s", got, enqueuedOn)
	}
}

func TestAlertConsumerProcess_KeepsSourceIdentifier(t *testing.T) {
	repo := newFakeAlertRepo()
	c, _ := singleQueueConsumer(t, repo)
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
	c, _ := singleQueueConsumer(t, repo)

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
	c, _ := singleQueueConsumer(t, repo)
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
		message *domain.AlertQueueMessage
		reason  string
	}{
		"no source":      {&domain.AlertQueueMessage{ID: "m", Body: []byte(canonicalAlert)}, alertDeadLetterMissingSource},
		"unknown source": {alertMessage("m", "nagios", canonicalAlert), alertDeadLetterUnknownSource},
		"bad body":       {alertMessage("m", "servicenow", "not json"), alertDeadLetterTransformFailed},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := newFakeAlertRepo()
			c, _ := singleQueueConsumer(t, repo)
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
	c, _ := singleQueueConsumer(t, repo)

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
			c, _ := singleQueueConsumer(t, repo)
			decision := c.process(context.Background(), domain.AlertTierHigh, alertMessage("m-1", "servicenow", canonicalAlert))
			if decision.settlement != alertAbandon {
				t.Fatalf("decision = %+v, want abandon so the queue redelivers it", decision)
			}
		})
	}
}

func TestAlertConsumerProcess_DeadLettersATransformThatPanics(t *testing.T) {
	c, _ := singleQueueConsumer(t, newFakeAlertRepo())
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

func TestAlertConsumerRun_DrainsEveryQueueOnBothCloudsConcurrently(t *testing.T) {
	const perQueue = 20
	queues := map[string]*fakeAlertQueue{}
	for _, name := range []string{"servicebus/priority-queue", "sqs/priority-queue", "servicebus/standard-queue", "sqs/standard-queue"} {
		var messages []*domain.AlertQueueMessage
		for i := range perQueue {
			messages = append(messages, alertMessage(fmt.Sprintf("%s-%d", name, i), "servicenow", canonicalAlert))
		}
		queues[name] = newFakeAlertQueue(name, messages...)
	}
	repo := newFakeAlertRepo()
	c := newTestConsumer(t, repo,
		[]AlertQueue{queues["servicebus/priority-queue"], queues["sqs/priority-queue"]},
		[]AlertQueue{queues["servicebus/standard-queue"], queues["sqs/standard-queue"]},
		8, 4)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.Run(ctx)
	}()
	settled := func() int {
		n := 0
		for _, q := range queues {
			n += q.settledCount()
		}
		return n
	}
	deadline := time.Now().Add(5 * time.Second)
	for settled() < 4*perQueue {
		if time.Now().After(deadline) {
			t.Fatalf("settled %d of %d messages", settled(), 4*perQueue)
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the context was cancelled")
	}

	for name, q := range queues {
		if q.completedCount() != perQueue {
			t.Fatalf("%s completed %d messages, want %d", name, q.completedCount(), perQueue)
		}
	}
	if repo.rowCount() != 4*perQueue {
		t.Fatalf("stored %d rows, want %d", repo.rowCount(), 4*perQueue)
	}
	if repo.maxConcurrent < 2 {
		t.Fatalf("at most %d insert ran at once, want concurrent processing", repo.maxConcurrent)
	}
	for _, row := range repo.rows {
		wantHigh := len(row.MessageID) > 0 && (row.MessageID[:len("servicebus/priority")] == "servicebus/priority" || row.MessageID[:len("sqs/priority")] == "sqs/priority")
		if wantHigh != (row.Tier == domain.AlertTierHigh) {
			t.Fatalf("message %s was stored on the %s tier", row.MessageID, row.Tier)
		}
	}
}

func TestAlertConsumer_ShutdownHandsBufferedMessagesBack(t *testing.T) {
	priority := newFakeAlertQueue("priority")
	// Two workers, so the priority buffer has room for both messages.
	c := newTestConsumer(t, newFakeAlertRepo(), []AlertQueue{priority}, []AlertQueue{newFakeAlertQueue("standard")}, 2, 4)
	lane := c.lanes[domain.AlertTierHigh]
	lane.messages <- queuedMessage{message: alertMessage("waiting-1", "servicenow", canonicalAlert), queue: priority}
	lane.messages <- queuedMessage{message: alertMessage("waiting-2", "servicenow", canonicalAlert), queue: priority}

	c.abandonBuffered(lane)
	if len(priority.abandoned) != 2 || len(lane.messages) != 0 {
		t.Fatalf("abandoned %v with %d still buffered, want both handed back", priority.abandoned, len(lane.messages))
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

func lockedFor(m *domain.AlertQueueMessage, d time.Duration) *domain.AlertQueueMessage {
	lockedUntil := time.Now().Add(d)
	m.LockedUntil = &lockedUntil
	return m
}

func TestAlertConsumerHandle_RenewsTheLockOfAMessageThatWaitedInTheBuffer(t *testing.T) {
	c, priority := singleQueueConsumer(t, newFakeAlertRepo())

	c.handle(context.Background(), domain.AlertTierHigh, priority, lockedFor(alertMessage("m-1", "servicenow", canonicalAlert), 5*time.Second))
	if priority.renewCount() == 0 || priority.completedCount() != 1 {
		t.Fatalf("renewals %d, completed %v; want the near-expiry lock renewed before the message is completed", priority.renewCount(), priority.completed)
	}
}

func TestAlertConsumerHandle_KeepsTheLockDuringSlowProcessing(t *testing.T) {
	c, priority := singleQueueConsumer(t, slowAlertRepo{fakeAlertRepo: newFakeAlertRepo(), delay: 300 * time.Millisecond})
	c.lockRenewMargin = 950 * time.Millisecond

	c.handle(context.Background(), domain.AlertTierHigh, priority, lockedFor(alertMessage("m-1", "servicenow", canonicalAlert), time.Second))
	if priority.renewCount() == 0 || priority.completedCount() != 1 {
		t.Fatalf("renewals %d, completed %v; want the lock renewed while the insert ran", priority.renewCount(), priority.completed)
	}
}

func TestAlertConsumerHandle_SkipsAMessageWhoseLockIsAlreadyLost(t *testing.T) {
	repo := newFakeAlertRepo()
	c, priority := singleQueueConsumer(t, repo)
	priority.renewErr = errors.New("lock lost")

	c.handle(context.Background(), domain.AlertTierHigh, priority, lockedFor(alertMessage("m-1", "servicenow", canonicalAlert), -time.Second))
	if repo.rowCount() != 0 || priority.settledCount() != 0 {
		t.Fatal("a message whose lock was lost was still processed or settled")
	}
}

func TestAlertConsumerSettle_DeadLettersWithTheReason(t *testing.T) {
	c, priority := singleQueueConsumer(t, newFakeAlertRepo())
	c.settle(context.Background(), priority, alertMessage("m-1", "nagios", canonicalAlert), alertDeadLetterDecision(alertDeadLetterUnknownSource, "no transform"))
	if priority.deadLettered["m-1"] != alertDeadLetterUnknownSource {
		t.Fatalf("dead-lettered = %v, want m-1 with %s", priority.deadLettered, alertDeadLetterUnknownSource)
	}
}

func riaConsumer(t *testing.T, repo AlertStore, enabled bool) *AlertConsumer {
	t.Helper()
	c := NewAlertConsumer(repo, mustRegistry(t), []AlertQueue{newFakeAlertQueue("priority")}, []AlertQueue{newFakeAlertQueue("standard")}, AlertConsumerConfig{Concurrency: 1, PriorityWeight: 4, RIAEnabled: enabled})
	c.sleep = func(ctx context.Context, _ time.Duration) bool { return ctx.Err() == nil }
	return c
}

func TestAlertConsumerProcess_RoutesEachAlertByItsRIALabel(t *testing.T) {
	tests := []struct {
		name      string
		enabled   bool
		property  string
		source    string
		body      string
		pipelines []domain.AlertPipeline
	}{
		{"off keeps every alert on core", false, "true", "servicenow", `{"service":"s","metric_name":"m","ria":"true"}`, []domain.AlertPipeline{domain.AlertPipelineCore}},
		{"label asks for ria", true, "", "servicenow", `{"service":"s","metric_name":"m","ria":"true"}`, []domain.AlertPipeline{domain.AlertPipelineRIA}},
		{"no label and no property stays core", true, "", "servicenow", `{"service":"s","metric_name":"m"}`, []domain.AlertPipeline{domain.AlertPipelineCore}},
		{"property asks for ria", true, "true", "servicenow", `{"service":"s","metric_name":"m"}`, []domain.AlertPipeline{domain.AlertPipelineRIA}},
		{"label false beats the property", true, "true", "servicenow", `{"service":"s","metric_name":"m","ria":"false"}`, []domain.AlertPipeline{domain.AlertPipelineCore}},
		{"label true beats a false property", true, "false", "servicenow", `{"service":"s","metric_name":"m","ria":"true"}`, []domain.AlertPipeline{domain.AlertPipelineRIA}},
		{"each alert of a batch on its own", true, "", "prometheus", `{"receiver":"r","status":"firing","alerts":[{"labels":{"alertname":"a","ria":"true"}},{"labels":{"alertname":"b"}}]}`, []domain.AlertPipeline{domain.AlertPipelineRIA, domain.AlertPipelineCore}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeAlertRepo()
			c := riaConsumer(t, repo, tt.enabled)
			message := alertMessage("m-1", tt.source, tt.body)
			if tt.property != "" {
				message.Properties["ria"] = tt.property
			}
			if decision := c.process(context.Background(), domain.AlertTierDefault, message); decision.settlement != alertComplete {
				t.Fatalf("decision = %+v, want complete", decision)
			}
			for i, want := range tt.pipelines {
				if got := repo.rows[fmt.Sprintf("m-1/%d", i)].Pipeline; got != want {
					t.Errorf("alert %d pipeline = %q, want %q", i, got, want)
				}
			}
		})
	}
}

func TestAlertPipeline(t *testing.T) {
	for _, tt := range []struct {
		label, requested alertsource.RIAFlag
		want             domain.AlertPipeline
	}{
		{alertsource.RIAUnset, alertsource.RIAUnset, domain.AlertPipelineCore},
		{alertsource.RIAUnset, alertsource.RIAOn, domain.AlertPipelineRIA},
		{alertsource.RIAUnset, alertsource.RIAOff, domain.AlertPipelineCore},
		{alertsource.RIAOn, alertsource.RIAOff, domain.AlertPipelineRIA},
		{alertsource.RIAOff, alertsource.RIAOn, domain.AlertPipelineCore},
	} {
		if got := alertPipeline(tt.label, tt.requested); got != tt.want {
			t.Errorf("alertPipeline(%v, %v) = %q, want %q", tt.label, tt.requested, got, tt.want)
		}
	}
}
