// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package allocator

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sre-alert-ingestion-service/internal/model"
	"sre-alert-ingestion-service/internal/postgres"
)

type fakeRow struct{ source, alert, fingerprint string }

// fakeStore is an in-memory alert_seq + alerts table with knobs for the failure cases.
type fakeStore struct {
	mu   sync.Mutex
	seq  int64
	rows map[string]fakeRow

	claims       atomic.Int64
	insertCalls  atomic.Int64
	claimErr     error
	claimDelay   time.Duration
	claimGate    chan struct{}
	claimEntered chan struct{}
	// interleave advances the sequence between each value handed out, the way a second replica drawing from alert_seq does.
	interleave bool
	// failInserts fails this many InsertBatch calls before letting them through; -1 fails them all.
	failInserts int
	insertGate  chan struct{}
}

func newFakeStore() *fakeStore {
	return &fakeStore{rows: map[string]fakeRow{}}
}

func (f *fakeStore) ClaimIDs(_ context.Context, n int) ([]int64, error) {
	f.claims.Add(1)
	if f.claimEntered != nil {
		select {
		case f.claimEntered <- struct{}{}:
		default:
		}
	}
	if f.claimGate != nil {
		<-f.claimGate
	}
	if f.claimDelay > 0 {
		time.Sleep(f.claimDelay)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	ids := make([]int64, n)
	for i := range ids {
		f.seq++
		ids[i] = f.seq
		if f.interleave {
			f.seq++
		}
	}
	return ids, nil
}

func (f *fakeStore) InsertBatch(_ context.Context, rows []postgres.InsertRow) error {
	f.insertCalls.Add(1)
	if f.insertGate != nil {
		<-f.insertGate
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failInserts != 0 {
		if f.failInserts > 0 {
			f.failInserts--
		}
		return errors.New("write timeout")
	}
	for _, r := range rows {
		if _, ok := f.rows[r.ID]; !ok {
			f.rows[r.ID] = fakeRow{source: r.Source, alert: string(r.Alert), fingerprint: r.Fingerprint}
		}
	}
	return nil
}

func (f *fakeStore) rowCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.rows)
}

type countingWaker struct{ n atomic.Int64 }

func (w *countingWaker) Wake() { w.n.Add(1) }

func testConfig() Config {
	return Config{
		QueueSize: 5000, MaxBatch: 200, WriteConcurrency: 64,
		InsertAttempts: 3, InsertBaseDelay: time.Millisecond,
		QueueMaxBytes: 1 << 30, QueryTimeout: time.Millisecond, WriteDeadline: time.Minute,
	}
}

func newTestAllocator(t *testing.T, store *fakeStore, w Waker, cfg Config) *Allocator {
	t.Helper()
	a := New(slog.New(slog.NewTextHandler(io.Discard, nil)), store, w, nil, cfg)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.Close(ctx)
	})
	return a
}

func alert(service, uid string) model.Alert {
	return model.Alert{Service: service, MetricName: "HighCPU", Severity: "Critical", Source: "Test", UniqueIdentifier: uid}
}

func seqOf(t *testing.T, id string) int64 {
	t.Helper()
	if !strings.HasPrefix(id, "ALT") || len(id) != 12 {
		t.Fatalf("id %q is not ALT + 9 digits", id)
	}
	n, err := strconv.ParseInt(id[3:], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestConcurrentSubmits_UniqueIDsWithFewClaims(t *testing.T) {
	store := newFakeStore()
	store.claimDelay = 2 * time.Millisecond
	cfg := testConfig()
	cfg.WriteConcurrency = 4
	a := newTestAllocator(t, store, nil, cfg)

	const total = 1000
	ids := make([]string, total)
	var wg sync.WaitGroup
	for i := range total {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := a.Submit(context.Background(), "aws", "req", []model.Alert{alert("svc", "u"+strconv.Itoa(i))})
			if err != nil {
				t.Errorf("Submit: %v", err)
				return
			}
			ids[i] = got[0]
		}()
	}
	wg.Wait()

	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("id %s issued twice", id)
		}
		seen[id] = true
	}
	if store.rowCount() != total {
		t.Errorf("rows = %d, want %d", store.rowCount(), total)
	}
	if c := store.claims.Load(); c >= total/10 {
		t.Errorf("claim calls = %d, want far fewer than %d", c, total)
	}
}

func TestInterleavedSequence_UsesTheIDsItWasGiven(t *testing.T) {
	store := newFakeStore()
	store.interleave = true
	a := newTestAllocator(t, store, nil, testConfig())

	batch := []model.Alert{alert("a", "1"), alert("b", "2"), alert("c", "3")}
	ids, err := a.Submit(context.Background(), "prometheus", "req", batch)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int64{1, 3, 5} {
		if seqOf(t, ids[i]) != want {
			t.Fatalf("ids = %v, want the claimed values 1, 3, 5 rather than an assumed consecutive range", ids)
		}
		if !strings.Contains(store.rows[ids[i]].alert, `"service":"`+batch[i].Service+`"`) {
			t.Errorf("row %s holds the wrong alert: %s", ids[i], store.rows[ids[i]].alert)
		}
	}
}

func TestBatch_OneClaimAndOneInsertPerBatch(t *testing.T) {
	store := newFakeStore()
	a := newTestAllocator(t, store, nil, testConfig())

	if _, err := a.Submit(context.Background(), "prometheus", "req", []model.Alert{alert("a", "1"), alert("b", "2")}); err != nil {
		t.Fatal(err)
	}
	if store.claims.Load() != 1 || store.insertCalls.Load() != 1 {
		t.Errorf("claims = %d inserts = %d, want 1 and 1", store.claims.Load(), store.insertCalls.Load())
	}
}

func TestLargeSubmissionIsNotSplit(t *testing.T) {
	store := newFakeStore()
	cfg := testConfig()
	cfg.MaxBatch = 2
	a := newTestAllocator(t, store, nil, cfg)

	batch := make([]model.Alert, 5)
	for i := range batch {
		batch[i] = alert("svc", strconv.Itoa(i))
	}
	ids, err := a.Submit(context.Background(), "prometheus", "req", batch)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 5 || store.claims.Load() != 1 || store.rowCount() != 5 {
		t.Errorf("ids = %v, claims = %d; want one claim of 5 ids", ids, store.claims.Load())
	}
}

func TestMissingUniqueIdentifier_UsesOwnAltIDInFingerprint(t *testing.T) {
	store := newFakeStore()
	a := newTestAllocator(t, store, nil, testConfig())

	ids, err := a.Submit(context.Background(), "datadog", "req", []model.Alert{alert("svc", "")})
	if err != nil {
		t.Fatal(err)
	}
	var stored model.Alert
	if err := json.Unmarshal([]byte(store.rows[ids[0]].alert), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.UniqueIdentifier != ids[0] {
		t.Errorf("unique_identifier = %q, want %q", stored.UniqueIdentifier, ids[0])
	}
	if got := store.rows[ids[0]].fingerprint; got != model.Fingerprint(stored) {
		t.Errorf("fingerprint = %q, want it computed from the stored alert", got)
	}
}

func TestWakesOncePerBatch(t *testing.T) {
	store := newFakeStore()
	waker := &countingWaker{}
	a := newTestAllocator(t, store, waker, testConfig())

	if _, err := a.Submit(context.Background(), "aws", "req", []model.Alert{alert("a", "1"), alert("b", "2")}); err != nil {
		t.Fatal(err)
	}
	if n := waker.n.Load(); n != 1 {
		t.Errorf("wakes = %d, want 1 for one batch", n)
	}
}

func TestClaimFailure_FailsWithoutWritingRows(t *testing.T) {
	store := newFakeStore()
	store.claimErr = errors.New("connection refused")
	a := newTestAllocator(t, store, nil, testConfig())

	if _, err := a.Submit(context.Background(), "aws", "req", []model.Alert{alert("svc", "u")}); !errors.Is(err, ErrClaimFailed) {
		t.Fatalf("err = %v, want ErrClaimFailed", err)
	}
	if store.rowCount() != 0 || store.insertCalls.Load() != 0 {
		t.Errorf("rows = %d inserts = %d, want none", store.rowCount(), store.insertCalls.Load())
	}
}

func TestInsertFailure_RetriesThenSucceeds(t *testing.T) {
	store := newFakeStore()
	store.failInserts = 2
	waker := &countingWaker{}
	a := newTestAllocator(t, store, waker, testConfig())

	ids, err := a.Submit(context.Background(), "aws", "req", []model.Alert{alert("svc", "u")})
	if err != nil {
		t.Fatalf("err = %v, want success on the third attempt", err)
	}
	if store.insertCalls.Load() != 3 || store.rowCount() != 1 || store.claims.Load() != 1 {
		t.Errorf("inserts = %d rows = %d claims = %d, want 3, 1, 1 (retries reuse the claimed ids)", store.insertCalls.Load(), store.rowCount(), store.claims.Load())
	}
	if _, ok := store.rows[ids[0]]; !ok || waker.n.Load() != 1 {
		t.Errorf("row for %s missing or wake not sent", ids[0])
	}
}

func TestInsertFailure_ExhaustsAttemptsAndFails(t *testing.T) {
	store := newFakeStore()
	store.failInserts = -1
	waker := &countingWaker{}
	a := newTestAllocator(t, store, waker, testConfig())

	if _, err := a.Submit(context.Background(), "aws", "req", []model.Alert{alert("svc", "u")}); !errors.Is(err, ErrStoreFailed) {
		t.Fatalf("err = %v, want ErrStoreFailed (503)", err)
	}
	if store.insertCalls.Load() != 3 || store.rowCount() != 0 || waker.n.Load() != 0 {
		t.Errorf("inserts = %d rows = %d wakes = %d, want 3, 0, 0", store.insertCalls.Load(), store.rowCount(), waker.n.Load())
	}
}

func TestInsertFailure_StopsAtWriteDeadline(t *testing.T) {
	store := newFakeStore()
	store.failInserts = -1
	cfg := testConfig()
	cfg.InsertAttempts = 1000
	cfg.InsertBaseDelay = 20 * time.Millisecond
	cfg.WriteDeadline = 100 * time.Millisecond
	cfg.QueryTimeout = 10 * time.Millisecond
	a := newTestAllocator(t, store, nil, cfg)

	start := time.Now()
	if _, err := a.Submit(context.Background(), "aws", "req", []model.Alert{alert("svc", "u")}); !errors.Is(err, ErrStoreFailed) {
		t.Fatalf("err = %v, want ErrStoreFailed", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("retries ran %v, want them cut off near the 100ms write deadline", elapsed)
	}
}

func TestQueueFull_FailsImmediately(t *testing.T) {
	store := newFakeStore()
	store.claimGate = make(chan struct{})
	store.claimEntered = make(chan struct{}, 1)
	cfg := testConfig()
	cfg.QueueSize = 1
	cfg.WriteConcurrency = 1
	a := newTestAllocator(t, store, nil, cfg)

	results := make(chan error, 3)
	submit := func(uid string) {
		_, err := a.Submit(context.Background(), "aws", uid, []model.Alert{alert("a", uid)})
		results <- err
	}
	go submit("1")
	<-store.claimEntered // the only writer is blocked claiming for submission 1
	go submit("2")
	waitFor(t, func() bool { return len(a.queue) == 0 }) // the batcher holds 2, waiting for a writer
	go submit("3")
	waitFor(t, func() bool { return len(a.queue) == 1 })

	start := time.Now()
	if _, err := a.Submit(context.Background(), "aws", "4", []model.Alert{alert("d", "4")}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("err = %v, want ErrQueueFull", err)
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Error("a full queue must fail immediately, not wait")
	}

	close(store.claimGate)
	for range 3 {
		if err := <-results; err != nil {
			t.Errorf("queued submissions should still succeed, got %v", err)
		}
	}
}

func TestShutdown_DrainsQueueThenRejects(t *testing.T) {
	store := newFakeStore()
	store.claimGate = make(chan struct{})
	store.claimEntered = make(chan struct{}, 1)
	cfg := testConfig()
	cfg.MaxBatch = 1
	cfg.WriteConcurrency = 1
	a := New(slog.New(slog.NewTextHandler(io.Discard, nil)), store, nil, nil, cfg)

	results := make(chan error, 5)
	for i := range 5 {
		go func() {
			_, err := a.Submit(context.Background(), "aws", "r", []model.Alert{alert("s", strconv.Itoa(i))})
			results <- err
		}()
	}
	<-store.claimEntered
	waitFor(t, func() bool { return len(a.queue) == 3 })

	closed := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		closed <- a.Close(ctx)
	}()
	waitFor(t, func() bool { a.mu.RLock(); defer a.mu.RUnlock(); return a.closed })
	if _, err := a.Submit(context.Background(), "aws", "late", []model.Alert{alert("s", "x")}); !errors.Is(err, ErrShuttingDown) {
		t.Errorf("Submit after Close = %v, want ErrShuttingDown", err)
	}

	close(store.claimGate)
	for range 5 {
		if err := <-results; err != nil {
			t.Errorf("queued submission lost on shutdown: %v", err)
		}
	}
	if err := <-closed; err != nil {
		t.Errorf("Close = %v", err)
	}
	if store.rowCount() != 5 {
		t.Errorf("rows = %d, want 5", store.rowCount())
	}
}

func TestCancelledRequest_StillWritesItsRow(t *testing.T) {
	store := newFakeStore()
	store.claimGate = make(chan struct{})
	a := newTestAllocator(t, store, nil, testConfig())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := a.Submit(ctx, "aws", "r", []model.Alert{alert("s", "u")}); done <- err }()
	waitFor(t, func() bool { return len(a.queue) == 0 })
	cancel()
	if err := <-done; !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}

	close(store.claimGate)
	closeCtx, cancelClose := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelClose()
	if err := a.Close(closeCtx); err != nil {
		t.Fatal(err)
	}
	if store.rowCount() != 1 {
		t.Errorf("rows = %d, want 1: an accepted alert must be written even after the client left", store.rowCount())
	}
}

func TestQueueBytes_LimitAndRelease(t *testing.T) {
	big := alert("svc", "u")
	big.Description = strings.Repeat("x", 200)

	t.Run("over the limit", func(t *testing.T) {
		store := newFakeStore()
		cfg := testConfig()
		cfg.QueueMaxBytes = 100
		a := newTestAllocator(t, store, nil, cfg)
		if _, err := a.Submit(context.Background(), "aws", "req", []model.Alert{big}); !errors.Is(err, ErrQueueBytesFull) {
			t.Fatalf("err = %v, want ErrQueueBytesFull", err)
		}
		if store.claims.Load() != 0 || a.QueueBytes() != 0 {
			t.Errorf("claimed %d times, %d bytes held; want nothing", store.claims.Load(), a.QueueBytes())
		}
	})

	cases := map[string]func(*fakeStore){
		"stored":         func(*fakeStore) {},
		"insert failure": func(s *fakeStore) { s.failInserts = -1 },
		"claim failure":  func(s *fakeStore) { s.claimErr = errors.New("connection refused") },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			store := newFakeStore()
			setup(store)
			a := newTestAllocator(t, store, nil, testConfig())
			_, _ = a.Submit(context.Background(), "aws", "req", []model.Alert{big})
			if b := a.QueueBytes(); b != 0 {
				t.Errorf("queue bytes = %d, want 0", b)
			}
		})
	}

	t.Run("shutdown", func(t *testing.T) {
		store := newFakeStore()
		store.insertGate = make(chan struct{})
		a := New(slog.New(slog.NewTextHandler(io.Discard, nil)), store, nil, nil, testConfig())
		done := make(chan struct{})
		go func() {
			_, _ = a.Submit(context.Background(), "aws", "req", []model.Alert{big})
			close(done)
		}()
		waitFor(t, func() bool { return a.QueueBytes() > 0 })
		closed := make(chan error, 1)
		go func() { closed <- a.Close(context.Background()) }()
		close(store.insertGate)
		<-done
		if err := <-closed; err != nil {
			t.Fatal(err)
		}
		if b := a.QueueBytes(); b != 0 {
			t.Errorf("queue bytes = %d after shutdown, want 0", b)
		}
		if _, err := a.Submit(context.Background(), "aws", "req", []model.Alert{big}); !errors.Is(err, ErrShuttingDown) || a.QueueBytes() != 0 {
			t.Errorf("submit after close: err = %v, bytes = %d", err, a.QueueBytes())
		}
	})
}

func TestSizeOf_SharedDescriptionCountedOnce(t *testing.T) {
	desc := strings.Repeat("d", 1000)
	one := alert("svc", "u")
	one.Description = desc
	batch := []model.Alert{one, one, one}
	fields := sizeOf([]model.Alert{one}) - int64(len(desc))
	if got, want := sizeOf(batch), 3*fields+int64(len(desc)); got != want {
		t.Errorf("sizeOf = %d, want %d (description counted once)", got, want)
	}
}

type recordingFallback struct {
	mu         sync.Mutex
	alerts     []model.Alert
	recoveries int
}

func (f *recordingFallback) Notify(_, _ string, alerts []model.Alert) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.alerts = append(f.alerts, alerts...)
}

func (f *recordingFallback) Recovered() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recoveries++
}

func (f *recordingFallback) counts() (alerts, recoveries int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.alerts), f.recoveries
}

func TestFallback_GetsOnlyAlertsThatWereNotStored(t *testing.T) {
	cases := map[string]struct {
		setup          func(*fakeStore)
		wantAlerts     int
		wantRecoveries int
	}{
		"stored":         {func(*fakeStore) {}, 0, 1},
		"insert failure": {func(s *fakeStore) { s.failInserts = -1 }, 2, 0},
		"claim failure":  {func(s *fakeStore) { s.claimErr = errors.New("connection refused") }, 2, 0},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			store := newFakeStore()
			tc.setup(store)
			fb := &recordingFallback{}
			a := New(slog.New(slog.NewTextHandler(io.Discard, nil)), store, nil, fb, testConfig())
			_, _ = a.Submit(context.Background(), "aws", "req", []model.Alert{alert("svc", "a"), alert("svc", "b")})
			// Close waits for the writer, so the recovery signal sent after answering has landed.
			if err := a.Close(context.Background()); err != nil {
				t.Fatal(err)
			}
			if alerts, recoveries := fb.counts(); alerts != tc.wantAlerts || recoveries != tc.wantRecoveries {
				t.Errorf("fallback got %d alerts and %d recoveries, want %d and %d", alerts, recoveries, tc.wantAlerts, tc.wantRecoveries)
			}
		})
	}
}
