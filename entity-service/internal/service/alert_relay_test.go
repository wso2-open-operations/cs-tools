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
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
)

// fakeRelayRepo hands out waiting alerts as the outbox would, honouring the in-flight limit.
type fakeRelayRepo struct {
	mu        sync.Mutex
	waiting   []domain.AlertToPublish
	inFlight  int
	unmarked  []string
	sweeps    int
	marks     int
	maxSeen   []int
	markCalls chan struct{}
}

func (r *fakeRelayRepo) MarkAlertsToPublish(_ context.Context, maxInFlight, limit int) ([]domain.AlertToPublish, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.marks++
	r.maxSeen = append(r.maxSeen, maxInFlight)
	if r.markCalls != nil {
		select {
		case r.markCalls <- struct{}{}:
		default:
		}
	}
	n := min(limit, max(maxInFlight-r.inFlight, 0), len(r.waiting))
	out := slices.Clone(r.waiting[:n])
	r.waiting = r.waiting[n:]
	r.inFlight += n
	return out, nil
}

func (r *fakeRelayRepo) UnmarkPublished(_ context.Context, ids []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.unmarked = append(r.unmarked, ids...)
	r.inFlight -= len(ids)
	return nil
}

func (r *fakeRelayRepo) RepublishStale(context.Context, time.Duration) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweeps++
	return 0, nil
}

func (r *fakeRelayRepo) add(alerts ...domain.AlertToPublish) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.waiting = append(r.waiting, alerts...)
}

type publishedRecord struct {
	key   string
	value []byte
}

// fakeAlertProducer records what was published; failOn makes the publish of that alert id fail.
type fakeAlertProducer struct {
	mu        sync.Mutex
	published []publishedRecord
	failOn    string
}

func (p *fakeAlertProducer) Publish(_ context.Context, key, value []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var env events.Envelope
	if err := json.Unmarshal(value, &env); err == nil && env.EntityID == p.failOn {
		return errors.New("broker unavailable")
	}
	p.published = append(p.published, publishedRecord{key: string(key), value: value})
	return nil
}

func (p *fakeAlertProducer) ids() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var ids []string
	for _, r := range p.published {
		var env events.Envelope
		_ = json.Unmarshal(r.value, &env)
		ids = append(ids, env.EntityID)
	}
	return ids
}

func toPublish(id, fingerprint string, tier domain.AlertTier) domain.AlertToPublish {
	return domain.AlertToPublish{ID: id, Fingerprint: fingerprint, Tier: tier, ReceivedOn: testAlertReceivedOn}
}

func TestAlertRelay_PublishesInOrderKeyedByFingerprintWithoutTheBody(t *testing.T) {
	repo := &fakeRelayRepo{}
	repo.add(toPublish("ALT000000001", "fp-a", domain.AlertTierHigh), toPublish("ALT000000002", "", domain.AlertTierDefault))
	producer := &fakeAlertProducer{}
	relay := NewAlertRelay(repo, producer, AlertRelayConfig{})

	n, err := relay.publishBatch(context.Background())
	if err != nil || n != 2 {
		t.Fatalf("publishBatch = %d, %v; want 2, nil", n, err)
	}
	if got := producer.ids(); !slices.Equal(got, []string{"ALT000000001", "ALT000000002"}) {
		t.Fatalf("published %v, want both in the order taken", got)
	}
	if producer.published[0].key != "fp-a" || producer.published[1].key != "ALT000000002" {
		t.Errorf("keys = %q, %q; want the fingerprint, and the id for an alert without one", producer.published[0].key, producer.published[1].key)
	}
	var env events.Envelope
	if err := json.Unmarshal(producer.published[0].value, &env); err != nil || env.Type != events.TypeAlertCreated {
		t.Fatalf("envelope = %+v, %v; want alert.created", env, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["alertId"] != "ALT000000001" || payload["tier"] != "high" || payload["fingerprint"] != "fp-a" {
		t.Errorf("payload = %v, want the alert id, tier and fingerprint", payload)
	}
	if _, ok := payload["alert"]; ok {
		t.Errorf("payload carries the alert body %v; it must name the alert only", payload["alert"])
	}
}

func TestAlertRelay_StopsAtTheFirstFailedPublishAndGivesTheRestBack(t *testing.T) {
	repo := &fakeRelayRepo{}
	repo.add(toPublish("ALT000000001", "fp-a", domain.AlertTierHigh), toPublish("ALT000000002", "fp-a", domain.AlertTierHigh),
		toPublish("ALT000000003", "fp-b", domain.AlertTierDefault))
	producer := &fakeAlertProducer{failOn: "ALT000000002"}
	relay := NewAlertRelay(repo, producer, AlertRelayConfig{})

	n, err := relay.publishBatch(context.Background())
	if err == nil || n != 1 {
		t.Fatalf("publishBatch = %d, %v; want 1 and the publish error", n, err)
	}
	if got := producer.ids(); !slices.Equal(got, []string{"ALT000000001"}) {
		t.Errorf("published %v, want only the alert before the failure", got)
	}
	if !slices.Equal(repo.unmarked, []string{"ALT000000002", "ALT000000003"}) {
		t.Errorf("given back %v, want the failed alert and everything after it, so order is kept", repo.unmarked)
	}
}

func TestAlertRelay_PublishesNoMoreThanTheInFlightLimit(t *testing.T) {
	repo := &fakeRelayRepo{inFlight: 3}
	for _, id := range []string{"ALT000000001", "ALT000000002", "ALT000000003"} {
		repo.add(toPublish(id, "fp-"+id, domain.AlertTierDefault))
	}
	producer := &fakeAlertProducer{}
	relay := NewAlertRelay(repo, producer, AlertRelayConfig{MaxInFlight: 4, BatchSize: 10})

	if n, err := relay.publishBatch(context.Background()); err != nil || n != 1 {
		t.Fatalf("publishBatch = %d, %v; want 1, the room left under 4 in flight", n, err)
	}
	if n, _ := relay.publishBatch(context.Background()); n != 0 {
		t.Fatalf("publishBatch = %d at the limit, want 0", n)
	}
	if !slices.Equal(repo.maxSeen, []int{4, 4}) {
		t.Errorf("max in flight passed %v, want the configured 4", repo.maxSeen)
	}
}

func TestAlertRelay_WakePublishesBeforeThePollAndSweepsAtStart(t *testing.T) {
	repo := &fakeRelayRepo{markCalls: make(chan struct{}, 16)}
	producer := &fakeAlertProducer{}
	relay := NewAlertRelay(repo, producer, AlertRelayConfig{PollInterval: time.Hour, SweepInterval: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { relay.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	select {
	case <-repo.markCalls: // the first look, then idle for an hour
	case <-time.After(5 * time.Second):
		t.Fatal("the relay never looked")
	}
	repo.add(toPublish("ALT000000009", "fp-z", domain.AlertTierHigh))
	relay.Wake()

	deadline := time.After(5 * time.Second)
	for len(producer.ids()) == 0 {
		select {
		case <-deadline:
			t.Fatal("a wake-up did not publish the waiting alert before the hour-long poll")
		case <-time.After(10 * time.Millisecond):
		}
	}
	repo.mu.Lock()
	sweeps := repo.sweeps
	repo.mu.Unlock()
	if sweeps != 1 {
		t.Errorf("sweeps = %d, want 1 at start", sweeps)
	}
}
