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
	"fmt"
	"reflect"
	"strings"
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

// fakeAlertRepo stores rows in memory in the order they arrive, and can fail its first calls.
type fakeAlertRepo struct {
	mu          sync.Mutex
	seq         int64
	rows        []domain.NewAlert
	failInserts int
	// refuseInserts fails that many inserts with a data exception, which fails the same way on every attempt.
	refuseInserts int
	failReserves  int
	insertCalls   int
	maxConcurrent int
	inFlight      int
}

func newFakeAlertRepo() *fakeAlertRepo {
	return &fakeAlertRepo{}
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

func (r *fakeAlertRepo) InsertAlerts(_ context.Context, alerts []domain.NewAlert) ([]domain.NewAlert, error) {
	r.mu.Lock()
	r.insertCalls++
	if r.failInserts > 0 {
		r.failInserts--
		r.mu.Unlock()
		return nil, errors.New("connection reset")
	}
	if r.refuseInserts > 0 {
		r.refuseInserts--
		r.mu.Unlock()
		return nil, fmt.Errorf("insert alerts: %w", sqlStateError("22P05"))
	}
	r.inFlight++
	r.maxConcurrent = max(r.maxConcurrent, r.inFlight)
	r.mu.Unlock()

	time.Sleep(2 * time.Millisecond)

	r.mu.Lock()
	defer r.mu.Unlock()
	r.inFlight--
	stored := make([]domain.NewAlert, len(alerts))
	for i, a := range alerts {
		// The database gives each row its UUID.
		a.ID = fmt.Sprintf("00000000-0000-7000-8000-%012d", len(r.rows)+1)
		r.rows = append(r.rows, a)
		stored[i] = a
	}
	return stored, nil
}

// row is the i-th stored row, or the zero row when there are fewer.
func (r *fakeAlertRepo) row(i int) domain.NewAlert {
	r.mu.Lock()
	defer r.mu.Unlock()
	if i >= len(r.rows) {
		return domain.NewAlert{}
	}
	return r.rows[i]
}

// sqlStateError is a database error carrying a SQLSTATE, as pgconn.PgError does.
type sqlStateError string

func (e sqlStateError) Error() string    { return "SQLSTATE " + string(e) }
func (e sqlStateError) SQLState() string { return string(e) }

func (r *fakeAlertRepo) rowCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.rows)
}

func mustRegistry(t *testing.T) *alertsource.Registry {
	t.Helper()
	return alertsource.New()
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

	decision := c.process(context.Background(), domain.AlertTierPriority, "priority", alertMessage("m-1", "ServiceNow", canonicalAlert))
	if decision.settlement != alertComplete || decision.stored != 1 {
		t.Fatalf("decision = %+v, want complete with one stored alert", decision)
	}
	row := repo.row(0)
	if row.Number != "ALT000000001" || row.ID == "" || row.Tier != domain.AlertTierPriority {
		t.Fatalf("row = %+v, want ALT000000001 with a UUID on the priority tier", row)
	}
	stored := row.Alert
	if stored.Service == "" || stored.Environment == "" || stored.MetricName == "" {
		t.Fatalf("alert = %+v, want its fields kept", stored)
	}
	if stored.UniqueIdentifier != row.Number {
		t.Fatalf("unique_identifier = %q, want the alert's own number when the source sent none", stored.UniqueIdentifier)
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

	c.process(context.Background(), domain.AlertTierPriority, "priority", message)
	if got := repo.row(0).ReceivedOn; !got.Equal(enqueuedOn) {
		t.Fatalf("received_on = %s, want the queue's %s", got, enqueuedOn)
	}
}

func TestAlertConsumerProcess_KeepsSourceIdentifier(t *testing.T) {
	repo := newFakeAlertRepo()
	c, _ := singleQueueConsumer(t, repo)
	body := `{"service":"svc-a","metric_name":"cpu","severity":"critical","environment":"prod","source":"Test","unique_identifier":"alarm-7"}`

	c.process(context.Background(), domain.AlertTierStandard, "priority", alertMessage("m-1", "servicenow", body))
	if stored := repo.row(0).Alert; stored.UniqueIdentifier != "alarm-7" {
		t.Fatalf("unique_identifier = %q, want the source's own alarm-7", stored.UniqueIdentifier)
	}
}

func TestAlertConsumerProcess_StoresEveryAlertOfABatch(t *testing.T) {
	repo := newFakeAlertRepo()
	c, _ := singleQueueConsumer(t, repo)

	decision := c.process(context.Background(), domain.AlertTierStandard, "priority", alertMessage("m-1", "servicenow", "["+canonicalAlert+","+canonicalAlert+"]"))
	if decision.stored != 2 || repo.rowCount() != 2 {
		t.Fatalf("stored %d rows, want 2", repo.rowCount())
	}
	if repo.row(0).Number == repo.row(1).Number {
		t.Fatal("two alerts of one message share a number")
	}
}

func TestAlertConsumerProcess_KeepsTheRawBodyOnTheFirstRow(t *testing.T) {
	repo := newFakeAlertRepo()
	c, _ := singleQueueConsumer(t, repo)
	body := "[" + canonicalAlert + "," + canonicalAlert + "]"
	message := alertMessage("m-1", "servicenow", body)

	c.process(context.Background(), domain.AlertTierStandard, "priority", message)
	first, second := repo.row(0), repo.row(1)
	if string(first.RawBody) != body {
		t.Fatalf("first row raw body %q, want the message's", first.RawBody)
	}
	if second.RawBody != nil {
		t.Fatalf("second row raw body %q, want none, so a batch keeps one copy", second.RawBody)
	}
}

// A message that cannot be normalised is dead-lettered with why, and nothing is stored; the dead-letter queue keeps its body.
func TestAlertConsumerProcess_DeadLettersAMessageItCannotNormalise(t *testing.T) {
	alerts := make([]string, maxAlertsPerMessage+1)
	for i := range alerts {
		alerts[i] = canonicalAlert
	}
	cases := map[string]struct {
		message *domain.AlertQueueMessage
		reason  string
	}{
		"no source":       {&domain.AlertQueueMessage{ID: "m", Body: []byte(canonicalAlert)}, alertErrorMissingSource},
		"unknown source":  {alertMessage("m", "nagios", canonicalAlert), alertErrorUnknownSource},
		"bad body":        {alertMessage("m", "servicenow", "not json"), alertErrorTransformFailed},
		"too many alerts": {alertMessage("m", "servicenow", "["+strings.Join(alerts, ",")+"]"), alertErrorTooManyAlerts},
		"no alerts":       {alertMessage("m", "prometheus", `{"alerts":[1,"two",null]}`), alertErrorNoAlerts},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := newFakeAlertRepo()
			c, _ := singleQueueConsumer(t, repo)
			decision := c.process(context.Background(), domain.AlertTierPriority, "priority", tc.message)
			if decision.settlement != alertDeadLetter || decision.reason != tc.reason || decision.description == "" {
				t.Fatalf("decision = %+v, want dead-letter with %s and a description", decision, tc.reason)
			}
			if repo.rowCount() != 0 || repo.insertCalls != 0 {
				t.Fatalf("stored %d rows in %d inserts, want nothing stored", repo.rowCount(), repo.insertCalls)
			}
		})
	}
}

// An alert the database refuses outright is dead-lettered instead of being retried until the queue gives up.
func TestAlertConsumerProcess_DeadLettersAnAlertTheDatabaseRefuses(t *testing.T) {
	repo := newFakeAlertRepo()
	repo.refuseInserts = 1
	c, _ := singleQueueConsumer(t, repo)

	decision := c.process(context.Background(), domain.AlertTierPriority, "priority", alertMessage("m-1", "servicenow", canonicalAlert))
	if decision.settlement != alertDeadLetter || decision.reason != alertErrorUnstorable {
		t.Fatalf("decision = %+v, want dead-letter with %s", decision, alertErrorUnstorable)
	}
	if repo.insertCalls != 1 {
		t.Fatalf("insert ran %d times, want the refused one once", repo.insertCalls)
	}
}

func TestStorableText(t *testing.T) {
	if got := storableText("a\x00b\xffc"); got != "ab\uFFFDc" {
		t.Fatalf("storableText = %q, want NUL dropped and invalid UTF-8 replaced", got)
	}
}

func TestAlertConsumerProcess_RetriesAShortStoreOutage(t *testing.T) {
	repo := newFakeAlertRepo()
	repo.failInserts = 2
	c, _ := singleQueueConsumer(t, repo)

	decision := c.process(context.Background(), domain.AlertTierPriority, "priority", alertMessage("m-1", "servicenow", canonicalAlert))
	if decision.settlement != alertComplete || repo.rowCount() != 1 {
		t.Fatalf("decision = %+v with %d rows, want the third attempt to store the alert", decision, repo.rowCount())
	}
}

func TestAlertConsumerProcess_AbandonsWhenTheStoreStaysDown(t *testing.T) {
	for name, repo := range map[string]*fakeAlertRepo{
		"reserve": {failReserves: 99},
		"insert":  {failInserts: 99},
	} {
		t.Run(name, func(t *testing.T) {
			c, _ := singleQueueConsumer(t, repo)
			decision := c.process(context.Background(), domain.AlertTierPriority, "priority", alertMessage("m-1", "servicenow", canonicalAlert))
			if decision.settlement != alertAbandon {
				t.Fatalf("decision = %+v, want abandon so the queue redelivers it", decision)
			}
		})
	}
}

func TestAlertConsumerProcess_DeadLettersATransformThatPanics(t *testing.T) {
	repo := newFakeAlertRepo()
	c, _ := singleQueueConsumer(t, repo)
	c.transforms = panickingTransforms{}

	decision := c.process(context.Background(), domain.AlertTierPriority, "priority", alertMessage("m-1", "anything", canonicalAlert))
	if decision.settlement != alertDeadLetter || decision.reason != alertErrorTransformFailed {
		t.Fatalf("decision = %+v, want dead-letter with %s instead of a crash", decision, alertErrorTransformFailed)
	}
}

// A NUL character, which a PostgreSQL text column refuses, is dropped so the alert is still stored.
func TestAlertConsumerProcess_StoresAnAlertWithANULCharacter(t *testing.T) {
	repo := newFakeAlertRepo()
	c, _ := singleQueueConsumer(t, repo)
	body := `{"service":"svc-a","metric_name":"cpu","severity":"critical","environment":"prod","source":"Test","description":"bad \u0000 byte"}`

	decision := c.process(context.Background(), domain.AlertTierPriority, "priority", alertMessage("m-1", "servicenow", body))
	row := repo.row(0)
	if decision.settlement != alertComplete || row.Alert.Description != "bad  byte" {
		t.Fatalf("decision = %+v, description %q; want the alert stored without the NUL", decision, row.Alert.Description)
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
	defer cancel()
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
	priority := 0
	for _, row := range repo.rows {
		if row.Tier == domain.AlertTierPriority {
			priority++
		}
	}
	if priority != 2*perQueue {
		t.Fatalf("%d alerts on the priority tier, want the %d from the two priority queues", priority, 2*perQueue)
	}
}

func TestAlertConsumer_ShutdownHandsBufferedMessagesBack(t *testing.T) {
	priority := newFakeAlertQueue("priority")
	// Two workers, so the priority buffer has room for both messages.
	c := newTestConsumer(t, newFakeAlertRepo(), []AlertQueue{priority}, []AlertQueue{newFakeAlertQueue("standard")}, 2, 4)
	lane := c.lanes[domain.AlertTierPriority]
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

func (r slowAlertRepo) InsertAlerts(ctx context.Context, alerts []domain.NewAlert) ([]domain.NewAlert, error) {
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

	c.handle(context.Background(), domain.AlertTierPriority, priority, lockedFor(alertMessage("m-1", "servicenow", canonicalAlert), 5*time.Second))
	if priority.renewCount() == 0 || priority.completedCount() != 1 {
		t.Fatalf("renewals %d, completed %v; want the near-expiry lock renewed before the message is completed", priority.renewCount(), priority.completed)
	}
}

// renewalGatedRepo holds every insert until the queue records a lock renewal, and fails the insert if none comes in time.
type renewalGatedRepo struct {
	*fakeAlertRepo
	queue *fakeAlertQueue
}

func (r renewalGatedRepo) InsertAlerts(ctx context.Context, alerts []domain.NewAlert) ([]domain.NewAlert, error) {
	deadline := time.Now().Add(5 * time.Second)
	for r.queue.renewCount() == 0 {
		if time.Now().After(deadline) {
			return nil, errors.New("no lock renewal while the insert ran")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return r.fakeAlertRepo.InsertAlerts(ctx, alerts)
}

func TestAlertConsumerHandle_KeepsTheLockDuringSlowProcessing(t *testing.T) {
	priority := newFakeAlertQueue("priority")
	repo := renewalGatedRepo{fakeAlertRepo: newFakeAlertRepo(), queue: priority}
	c := newTestConsumer(t, repo, []AlertQueue{priority}, []AlertQueue{newFakeAlertQueue("standard")}, 1, 4)
	c.lockRenewMargin = 950 * time.Millisecond

	c.handle(context.Background(), domain.AlertTierPriority, priority, lockedFor(alertMessage("m-1", "servicenow", canonicalAlert), time.Second))
	if priority.completedCount() != 1 {
		t.Fatalf("completed %v; want the insert to see a renewal while it ran, then the message completed", priority.completed)
	}
}

func TestAlertConsumerHandle_RenewsBeforeSettlingWhenTheLockCannotCoverASlowSettlement(t *testing.T) {
	c, priority := singleQueueConsumer(t, newFakeAlertRepo())

	// 20 seconds is past the processing margin but short of a settlement that takes its full timeout.
	c.handle(context.Background(), domain.AlertTierPriority, priority, lockedFor(alertMessage("m-1", "servicenow", canonicalAlert), 20*time.Second))
	if priority.renewCount() == 0 || priority.completedCount() != 1 {
		t.Fatalf("renewals %d, completed %v; want the lock renewed before settling", priority.renewCount(), priority.completed)
	}
}

func TestAlertConsumerReceive_RenewsTheLockWhileAMessageWaitsInTheBuffer(t *testing.T) {
	priority := newFakeAlertQueue("priority", lockedFor(alertMessage("m-1", "servicenow", canonicalAlert), time.Second))
	c := newTestConsumer(t, newFakeAlertRepo(), []AlertQueue{priority}, nil, 1, 4)
	c.lockRenewMargin = 950 * time.Millisecond
	lane := c.lanes[domain.AlertTierPriority]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.receive(ctx, lane, priority)
	deadline := time.Now().Add(5 * time.Second)
	for priority.renewCount() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the lock of a buffered message was never renewed")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	queued := <-lane.messages
	queued.takeOver()
}

// hangingRenewQueue never answers a renewal until the call's context ends, like an endpoint that accepts the connection and goes silent.
type hangingRenewQueue struct {
	*fakeAlertQueue
}

func (q hangingRenewQueue) RenewLock(ctx context.Context, _ *domain.AlertQueueMessage) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestAlertConsumerKeepLock_AHungRenewalCannotBlockTheTakeover(t *testing.T) {
	queue := hangingRenewQueue{newFakeAlertQueue("priority")}
	c := newTestConsumer(t, newFakeAlertRepo(), []AlertQueue{queue}, nil, 1, 4)
	c.lockRenewMargin = 950 * time.Millisecond
	c.lockRenewCallTimeout = 50 * time.Millisecond

	stop := c.keepLock(context.Background(), queue, lockedFor(alertMessage("m-1", "servicenow", canonicalAlert), time.Second))
	// Past the first renewal, which hangs until its own deadline.
	time.Sleep(1200 * time.Millisecond)
	stopped := make(chan struct{})
	go func() {
		stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("stopping the renewal waited on a renewal call that never answers")
	}
}

func TestAlertConsumerReceive_OneQueueCannotFillTheTier(t *testing.T) {
	var messages []*domain.AlertQueueMessage
	for i := range 10 {
		messages = append(messages, alertMessage(fmt.Sprintf("m-%d", i), "servicenow", canonicalAlert))
	}
	busy := newFakeAlertQueue("sqs/priority-queue", messages...)
	idle := newFakeAlertQueue("servicebus/priority-queue")
	c := newTestConsumer(t, newFakeAlertRepo(), []AlertQueue{busy, idle}, nil, 8, 4)
	lane := c.lanes[domain.AlertTierPriority]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.receive(ctx, lane, busy)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for len(lane.messages) < 4 {
		if time.Now().After(deadline) {
			t.Fatalf("buffered %d messages, want the busy queue's share of 4", len(lane.messages))
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	if got := len(lane.messages); got != 4 {
		t.Fatalf("buffered %d messages, want the busy queue held to its share of 4 so the idle queue keeps room", got)
	}
	cancel()
	<-done
	c.abandonBuffered(lane)
}

func TestAlertConsumerHandle_SkipsAMessageWhoseLockIsAlreadyLost(t *testing.T) {
	repo := newFakeAlertRepo()
	c, priority := singleQueueConsumer(t, repo)
	priority.renewErr = errors.New("lock lost")

	c.handle(context.Background(), domain.AlertTierPriority, priority, lockedFor(alertMessage("m-1", "servicenow", canonicalAlert), -time.Second))
	if repo.rowCount() != 0 || priority.settledCount() != 0 {
		t.Fatal("a message whose lock was lost was still processed or settled")
	}
}

// ackCheckingQueue records whether the alert row already existed when the message was completed.
type ackCheckingQueue struct {
	*fakeAlertQueue
	repo            *fakeAlertRepo
	rowsAtComplete  int
	completeChecked bool
}

func (q *ackCheckingQueue) Complete(ctx context.Context, m *domain.AlertQueueMessage) error {
	q.rowsAtComplete, q.completeChecked = q.repo.rowCount(), true
	return q.fakeAlertQueue.Complete(ctx, m)
}

func TestAlertConsumerHandle_AcksOnlyAfterTheAlertIsSaved(t *testing.T) {
	repo := newFakeAlertRepo()
	queue := &ackCheckingQueue{fakeAlertQueue: newFakeAlertQueue("priority"), repo: repo}
	c := newTestConsumer(t, repo, []AlertQueue{queue}, nil, 1, 4)

	c.handle(context.Background(), domain.AlertTierPriority, queue, alertMessage("m-1", "servicenow", canonicalAlert))
	if !queue.completeChecked || queue.rowsAtComplete != 1 {
		t.Fatalf("completed with %d rows saved (completed %v), want the alert saved before the ack", queue.rowsAtComplete, queue.completeChecked)
	}
}

func TestAlertConsumerHandle_NeverAcksAnAlertThatWasNotSaved(t *testing.T) {
	repo := newFakeAlertRepo()
	repo.failInserts = 99
	c, priority := singleQueueConsumer(t, repo)

	c.handle(context.Background(), domain.AlertTierPriority, priority, alertMessage("m-1", "servicenow", canonicalAlert))
	if priority.completedCount() != 0 || len(priority.abandoned) != 1 {
		t.Fatalf("completed %v, abandoned %v; want the unsaved alert handed back, not acked", priority.completed, priority.abandoned)
	}
}

func TestAlertConsumerSettle_DeadLettersWithTheReason(t *testing.T) {
	c, priority := singleQueueConsumer(t, newFakeAlertRepo())
	c.settle(context.Background(), priority, alertMessage("m-1", "nagios", canonicalAlert), alertDeadLetterDecision(alertErrorUnknownSource, "no transform"))
	if priority.deadLettered["m-1"] != alertErrorUnknownSource {
		t.Fatalf("dead-lettered = %v, want m-1 with %s", priority.deadLettered, alertErrorUnknownSource)
	}
}

// A field longer than its VARCHAR column is cut to the column's length, so the alert is stored rather than refused; description is TEXT and kept whole.
func TestAlertConsumerProcess_CutsALongFieldToItsColumn(t *testing.T) {
	repo := newFakeAlertRepo()
	c, _ := singleQueueConsumer(t, repo)
	body := `{"service":"` + strings.Repeat("s", 300) + `","metric_name":"cpu","severity":"critical","environment":"prod","source":"Test","description":"` + strings.Repeat("é", 5000) + `"}`

	decision := c.process(context.Background(), domain.AlertTierPriority, "priority", alertMessage("m-1", "servicenow", body))
	row := repo.row(0)
	if decision.settlement != alertComplete || len([]rune(row.Alert.Service)) != 255 || len([]rune(row.Alert.Description)) != 5000 {
		t.Fatalf("decision = %+v, service of %d and description of %d characters; want service cut to 255 and description kept at 5000",
			decision, len([]rune(row.Alert.Service)), len([]rune(row.Alert.Description)))
	}
}

// The fields a payload did not carry reach the stored row with the default each was given.
func TestAlertConsumerProcess_KeepsTheDefaultedFields(t *testing.T) {
	repo := newFakeAlertRepo()
	c, _ := singleQueueConsumer(t, repo)
	body := `{"data":{"essentials":{"alertRule":"cpu-high","alertId":"/subscriptions/s/alerts/1","severity":"Sev1"}}}`

	decision := c.process(context.Background(), domain.AlertTierPriority, "priority", alertMessage("m-1", "azure", body))
	want := []model.Defaulted{
		{Field: "service", Value: "Managed Services"},
		{Field: "category", Value: "Service Interruption"},
		{Field: "environment", Value: "Unknown"},
	}
	if got := repo.row(0).Alert.Defaulted; decision.settlement != alertComplete || !reflect.DeepEqual(got, want) {
		t.Fatalf("decision = %+v, defaulted %+v; want the alert stored with %+v", decision, got, want)
	}
}
