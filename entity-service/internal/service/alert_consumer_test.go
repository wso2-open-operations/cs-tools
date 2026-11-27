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
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakeQueueMessage struct {
	id, source, contentType string
	body                    []byte
}

func (m fakeQueueMessage) ID() string            { return m.id }
func (m fakeQueueMessage) Body() []byte          { return m.body }
func (m fakeQueueMessage) ContentType() string   { return m.contentType }
func (m fakeQueueMessage) Source() string        { return m.source }
func (m fakeQueueMessage) EnqueuedOn() time.Time { return time.Time{} }

// fakeAlertQueue hands out its pending messages, then waits like a long poll until ctx ends.
type fakeAlertQueue struct {
	name          string
	renewInterval time.Duration

	mu          sync.Mutex
	pending     []AlertQueueMessage
	completed   []string
	abandoned   []string
	deadLetters []string
	renewals    map[string]int
}

func newFakeAlertQueue(name string, msgs ...AlertQueueMessage) *fakeAlertQueue {
	return &fakeAlertQueue{name: name, pending: msgs, renewals: map[string]int{}}
}

func (q *fakeAlertQueue) Name() string { return q.name }

func (q *fakeAlertQueue) Receive(ctx context.Context, limit int) ([]AlertQueueMessage, error) {
	q.mu.Lock()
	if len(q.pending) > 0 {
		n := min(limit, len(q.pending))
		out := q.pending[:n]
		q.pending = q.pending[n:]
		q.mu.Unlock()
		return out, nil
	}
	q.mu.Unlock()
	<-ctx.Done()
	return nil, ctx.Err()
}

func (q *fakeAlertQueue) Complete(_ context.Context, m AlertQueueMessage) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.completed = append(q.completed, m.ID())
	return nil
}

func (q *fakeAlertQueue) Abandon(_ context.Context, m AlertQueueMessage) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.abandoned = append(q.abandoned, m.ID())
	return nil
}

func (q *fakeAlertQueue) DeadLetter(_ context.Context, m AlertQueueMessage, reason, _ string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.deadLetters = append(q.deadLetters, m.ID()+":"+reason)
	return nil
}

func (q *fakeAlertQueue) RenewLock(_ context.Context, m AlertQueueMessage) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.renewals[m.ID()]++
	return nil
}

func (q *fakeAlertQueue) LockRenewInterval() time.Duration { return q.renewInterval }
func (q *fakeAlertQueue) Close(context.Context) error      { return nil }

func (q *fakeAlertQueue) snapshot() (completed, abandoned, deadLetters []string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return slices.Clone(q.completed), slices.Clone(q.abandoned), slices.Clone(q.deadLetters)
}

// fakeAlertStore records the rows it is asked to store; insertErrs are returned by successive InsertAlerts calls.
type fakeAlertStore struct {
	mu         sync.Mutex
	next       int
	inserts    [][]domain.NewAlert
	insertErrs []error
	// block, when set, holds every InsertAlerts until it is closed.
	block chan struct{}
	// onInsert runs after each insert is recorded.
	onInsert func(n int)
}

func (s *fakeAlertStore) ReserveAlertIDs(_ context.Context, n int) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, n)
	for i := range ids {
		s.next++
		ids[i] = fmt.Sprintf("ALT%09d", s.next)
	}
	return ids, nil
}

func (s *fakeAlertStore) InsertAlerts(_ context.Context, rows []domain.NewAlert) (int64, error) {
	if s.block != nil {
		<-s.block
	}
	s.mu.Lock()
	s.inserts = append(s.inserts, slices.Clone(rows))
	n := len(s.inserts)
	var err error
	if len(s.insertErrs) > 0 {
		err, s.insertErrs = s.insertErrs[0], s.insertErrs[1:]
	}
	onInsert := s.onInsert
	s.mu.Unlock()
	if onInsert != nil {
		onInsert(n)
	}
	if err != nil {
		return 0, err
	}
	return int64(len(rows)), nil
}

type fakeTransforms map[string]alertsource.Transform

func (f fakeTransforms) Lookup(source string) (alertsource.Transform, bool) {
	t, ok := f[source]
	return t, ok
}

var twoAlertTransform = fakeTransforms{
	"prometheus": func(raw []byte) ([]model.Alert, error) {
		return []model.Alert{
			{Service: "svc", MetricName: "cpu", Severity: "critical", Environment: "prod", Source: "Prometheus", UniqueIdentifier: "a"},
			{Service: "svc", MetricName: "mem", Severity: "warning", Environment: "prod", Source: "Prometheus", UniqueIdentifier: "b"},
		}, nil
	},
	"broken": func([]byte) ([]model.Alert, error) { return nil, errors.New("bad payload") },
	"empty":  func([]byte) ([]model.Alert, error) { return nil, nil },
}

func newTestConsumer(store *fakeAlertStore, priority, standard []AlertQueue, concurrency int) *AlertConsumer {
	c := NewAlertConsumer(store, twoAlertTransform, priority, standard, AlertConsumerConfig{Concurrency: concurrency})
	c.now = func() time.Time { return testAlertReceivedOn }
	return c
}

func handleOne(t *testing.T, c *AlertConsumer, q *fakeAlertQueue, tier domain.AlertTier, m AlertQueueMessage) {
	t.Helper()
	c.handle(context.Background(), c.hold(context.Background(), q, tier, m))
}

func TestAlertConsumer_StoresEveryAlertThenCompletes(t *testing.T) {
	store, q := &fakeAlertStore{}, newFakeAlertQueue("priority-queue")
	c := newTestConsumer(store, nil, nil, 1)
	body := []byte(`{"alerts":[]}`)

	handleOne(t, c, q, domain.AlertTierHigh, fakeQueueMessage{id: "m1", source: " Prometheus ", contentType: "application/json", body: body})

	if len(store.inserts) != 1 || len(store.inserts[0]) != 2 {
		t.Fatalf("inserts = %v, want one insert of two rows", store.inserts)
	}
	rows := store.inserts[0]
	for i, r := range rows {
		var a model.Alert
		if err := json.Unmarshal(r.Alert, &a); err != nil {
			t.Fatal(err)
		}
		if r.Source != "prometheus" || r.Tier != domain.AlertTierHigh || r.MessageID != "priority-queue/m1" || r.MessageIndex != i ||
			r.Fingerprint != model.Fingerprint(a) || r.ErrorReason != "" || !r.ReceivedOn.Equal(testAlertReceivedOn) || r.ID == "" {
			t.Errorf("row %d = %+v, want the transform's alert with its fingerprint, tier and message key", i, r)
		}
	}
	if string(rows[0].RawBody) != string(body) || rows[0].ContentType != "application/json" || rows[1].RawBody != nil {
		t.Errorf("raw body on rows = %q / %q, want it on the first row only", rows[0].RawBody, rows[1].RawBody)
	}
	if completed, abandoned, _ := q.snapshot(); !slices.Equal(completed, []string{"m1"}) || len(abandoned) != 0 {
		t.Errorf("completed %v, abandoned %v; want m1 completed after the insert", completed, abandoned)
	}
}

func TestAlertConsumer_KeepsUnreadableMessagesAsErrorRows(t *testing.T) {
	cases := map[string]struct {
		source, wantReason, wantSource string
	}{
		"no source":        {source: "", wantReason: domain.AlertErrorNoSource},
		"unknown source":   {source: "nagios", wantReason: domain.AlertErrorUnknownSource, wantSource: "nagios"},
		"transform failed": {source: "broken", wantReason: domain.AlertErrorTransformFailed, wantSource: "broken"},
		"no alerts":        {source: "empty", wantReason: domain.AlertErrorNoAlerts, wantSource: "empty"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			store, q := &fakeAlertStore{}, newFakeAlertQueue("standard-queue")
			c := newTestConsumer(store, nil, nil, 1)
			handleOne(t, c, q, domain.AlertTierDefault, fakeQueueMessage{id: "m2", source: tc.source, body: []byte("not json")})

			if len(store.inserts) != 1 || len(store.inserts[0]) != 1 {
				t.Fatalf("inserts = %v, want one error row", store.inserts)
			}
			r := store.inserts[0][0]
			if r.ErrorReason != tc.wantReason || r.Error == "" || r.Source != tc.wantSource || r.Alert != nil || r.Fingerprint != "" ||
				string(r.RawBody) != "not json" || r.MessageID != "standard-queue/m2" || r.Tier != domain.AlertTierDefault {
				t.Fatalf("row = %+v, want an error row %s with the raw body", r, tc.wantReason)
			}
			if completed, _, _ := q.snapshot(); !slices.Equal(completed, []string{"m2"}) {
				t.Errorf("completed %v, want the message completed: it is kept, never retried", completed)
			}
		})
	}
}

func TestAlertConsumer_GivesTheMessageBackWhenTheDatabaseIsDown(t *testing.T) {
	store, q := &fakeAlertStore{insertErrs: []error{errors.New("connection refused")}}, newFakeAlertQueue("priority-queue")
	c := newTestConsumer(store, nil, nil, 1)
	handleOne(t, c, q, domain.AlertTierHigh, fakeQueueMessage{id: "m3", source: "prometheus"})

	completed, abandoned, deadLetters := q.snapshot()
	if len(completed) != 0 || !slices.Equal(abandoned, []string{"m3"}) || len(deadLetters) != 0 {
		t.Fatalf("completed %v, abandoned %v, dead-lettered %v; want m3 given back for redelivery", completed, abandoned, deadLetters)
	}
}

func TestAlertConsumer_KeepsARefusedMessageAsAnErrorRow(t *testing.T) {
	refused := &pgconn.PgError{Code: "23514", Message: "check constraint"}
	store, q := &fakeAlertStore{insertErrs: []error{refused}}, newFakeAlertQueue("priority-queue")
	c := newTestConsumer(store, nil, nil, 1)
	handleOne(t, c, q, domain.AlertTierHigh, fakeQueueMessage{id: "m4", source: "prometheus", body: []byte("{}")})

	if len(store.inserts) != 2 || len(store.inserts[1]) != 1 || store.inserts[1][0].ErrorReason != domain.AlertErrorStoreRefused || string(store.inserts[1][0].RawBody) != "{}" {
		t.Fatalf("inserts = %+v, want the alerts, then one StoreRefused error row with the raw body", store.inserts)
	}
	if completed, _, _ := q.snapshot(); !slices.Equal(completed, []string{"m4"}) {
		t.Errorf("completed %v, want m4 completed once its error row is stored", completed)
	}
}

func TestAlertConsumer_DeadLettersOnlyWhenEvenTheErrorRowIsRefused(t *testing.T) {
	refused := &pgconn.PgError{Code: "22001", Message: "value too long"}
	store, q := &fakeAlertStore{insertErrs: []error{refused, refused}}, newFakeAlertQueue("priority-queue")
	c := newTestConsumer(store, nil, nil, 1)
	handleOne(t, c, q, domain.AlertTierHigh, fakeQueueMessage{id: "m5", source: "prometheus"})

	completed, abandoned, deadLetters := q.snapshot()
	if len(completed) != 0 || len(abandoned) != 0 || !slices.Equal(deadLetters, []string{"m5:" + domain.AlertErrorStoreRefused}) {
		t.Fatalf("completed %v, abandoned %v, dead-lettered %v; want m5 dead-lettered", completed, abandoned, deadLetters)
	}
}

func TestAlertConsumer_AFreeWorkerTakesPriorityFirst(t *testing.T) {
	q := newFakeAlertQueue("q")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := &fakeAlertStore{onInsert: func(n int) {
		if n == 4 {
			cancel()
		}
	}}
	c := newTestConsumer(store, nil, nil, 1)
	priority, standard := make(chan *heldAlertMessage, 4), make(chan *heldAlertMessage, 4)
	for _, id := range []string{"s1", "s2"} {
		standard <- c.hold(ctx, q, domain.AlertTierDefault, fakeQueueMessage{id: id, source: "prometheus"})
	}
	for _, id := range []string{"p1", "p2"} {
		priority <- c.hold(ctx, q, domain.AlertTierHigh, fakeQueueMessage{id: id, source: "prometheus"})
	}

	c.work(ctx, priority, standard, make(chan struct{}, 4))

	var order []domain.AlertTier
	for _, rows := range store.inserts {
		order = append(order, rows[0].Tier)
	}
	want := []domain.AlertTier{domain.AlertTierHigh, domain.AlertTierHigh, domain.AlertTierDefault, domain.AlertTierDefault}
	if !slices.Equal(order, want) {
		t.Fatalf("stored in order %v, want both priority messages before the standard ones", order)
	}
}

func TestAlertConsumer_RenewsTheLockWhileAMessageIsHeld(t *testing.T) {
	release := make(chan struct{})
	store, q := &fakeAlertStore{block: release}, newFakeAlertQueue("priority-queue")
	q.renewInterval = 5 * time.Millisecond
	c := newTestConsumer(store, nil, nil, 1)
	h := c.hold(context.Background(), q, domain.AlertTierHigh, fakeQueueMessage{id: "m6", source: "prometheus"})
	done := make(chan struct{})
	go func() { c.handle(context.Background(), h); close(done) }()

	deadline := time.After(5 * time.Second)
	for {
		q.mu.Lock()
		n := q.renewals["m6"]
		q.mu.Unlock()
		if n >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("renewals = %d while the message was held, want at least 2", n)
		case <-time.After(5 * time.Millisecond):
		}
	}
	close(release)
	<-done
	q.mu.Lock()
	after := q.renewals["m6"]
	q.mu.Unlock()
	time.Sleep(30 * time.Millisecond)
	q.mu.Lock()
	later := q.renewals["m6"]
	q.mu.Unlock()
	if later != after {
		t.Errorf("renewals went on after the message was settled: %d then %d", after, later)
	}
}

func TestAlertConsumer_ShutdownFinishesInFlightAndGivesTheRestBack(t *testing.T) {
	release := make(chan struct{})
	store := &fakeAlertStore{block: release}
	q := newFakeAlertQueue("priority-queue",
		fakeQueueMessage{id: "m1", source: "prometheus"}, fakeQueueMessage{id: "m2", source: "prometheus"})
	c := newTestConsumer(store, []AlertQueue{q}, nil, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()

	// Wait until one message is being stored and the other waits in the channel.
	deadline := time.After(5 * time.Second)
	for {
		q.mu.Lock()
		left := len(q.pending)
		q.mu.Unlock()
		if left == 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("the consumer never received the messages")
		case <-time.After(5 * time.Millisecond):
		}
	}
	time.Sleep(20 * time.Millisecond)
	cancel()
	close(release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after shutdown")
	}
	completed, abandoned, _ := q.snapshot()
	if !slices.Equal(completed, []string{"m1"}) || !slices.Equal(abandoned, []string{"m2"}) {
		t.Fatalf("completed %v, abandoned %v; want m1 finished and m2 given back", completed, abandoned)
	}
}
