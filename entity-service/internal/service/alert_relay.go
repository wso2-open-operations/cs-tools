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
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// AlertEventProducer publishes one record and waits for the broker; *eventbus.Producer satisfies it.
type AlertEventProducer interface {
	Publish(ctx context.Context, key, value []byte) error
}

// AlertRelayConfig tunes the alert relay.
type AlertRelayConfig struct {
	// MaxInFlight is the backpressure: no more is published while this many alerts are published and undecided.
	MaxInFlight int
	// PollInterval is the backstop: how long an idle relay waits before looking again when no wake-up came.
	PollInterval time.Duration
	// StaleAfter is how long a published alert may stay undecided and unclaimed before it is published again.
	StaleAfter time.Duration
	// SweepInterval is how often the relay looks for stale alerts.
	SweepInterval time.Duration
	// BatchSize is how many alerts the relay takes from the table at a time.
	BatchSize int
}

// Defaults of AlertRelayConfig.
const (
	alertRelayDefaultMaxInFlight   = 20
	alertRelayDefaultPollInterval  = 5 * time.Second
	alertRelayDefaultStaleAfter    = 5 * time.Minute
	alertRelayDefaultSweepInterval = time.Minute
	alertRelayDefaultBatchSize     = 10
)

// alertPublishTimeout bounds one alert.created publish.
const alertPublishTimeout = 10 * time.Second

// After a failed publish or database error the relay waits from alertRelayRetryMin, doubling up to alertRelayRetryMax.
const (
	alertRelayRetryMin = time.Second
	alertRelayRetryMax = 30 * time.Second
)

// AlertRelay publishes alert.created for each stored alert, from the alert table as the outbox. It takes waiting
// alerts priority tier first and stamps published_on in a short transaction that commits at once, so it holds no lock
// while it publishes; a failed publish clears published_on again, and a crash in between is caught by the stale sweep.
// The backpressure (MaxInFlight) keeps a backlog in the table, where priority alerts go first, instead of on the
// topic, where they would queue behind it. One fingerprint's alerts are published in order, keyed by fingerprint, so
// they share a partition.
type AlertRelay struct {
	repo     repository.AlertRelayRepository
	producer AlertEventProducer
	cfg      AlertRelayConfig
	now      func() time.Time
	sleep    func(ctx context.Context, d time.Duration) bool
	wake     alertWaker
}

// NewAlertRelay returns an AlertRelay; Run starts it.
func NewAlertRelay(repo repository.AlertRelayRepository, producer AlertEventProducer, cfg AlertRelayConfig) *AlertRelay {
	if cfg.MaxInFlight <= 0 {
		cfg.MaxInFlight = alertRelayDefaultMaxInFlight
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = alertRelayDefaultPollInterval
	}
	if cfg.StaleAfter <= 0 {
		cfg.StaleAfter = alertRelayDefaultStaleAfter
	}
	if cfg.SweepInterval <= 0 {
		cfg.SweepInterval = alertRelayDefaultSweepInterval
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = alertRelayDefaultBatchSize
	}
	return &AlertRelay{
		repo:     repo,
		producer: producer,
		cfg:      cfg,
		now:      time.Now,
		sleep:    sleepContext,
		wake:     alertWaker{ch: make(chan struct{})},
	}
}

// Wake makes an idle relay look again at once: the alert_stored listener calls it when a NEW alert committed, and the
// processor after each decision, which frees room under MaxInFlight. Safe from any goroutine, any number of times.
func (r *AlertRelay) Wake() {
	r.wake.broadcast()
}

// Run publishes until ctx is done.
func (r *AlertRelay) Run(ctx context.Context) {
	failWait := alertRelayRetryMin
	nextSweep := r.now() // sweep at start: a restart may follow a crash between marking and publishing
	for ctx.Err() == nil {
		// Taken before looking, so a wake-up that arrives while the relay looks is not lost.
		woken := r.wake.wait()
		if !r.now().Before(nextSweep) {
			r.sweep(ctx)
			nextSweep = r.now().Add(r.cfg.SweepInterval)
		}
		published, err := r.publishBatch(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.ErrorContext(ctx, "alert relay: publishing alert.created failed, retrying", "in", failWait, "err", err)
			// Not idle: a wake-up must not turn a failing broker or database into a busy loop.
			if !r.sleep(ctx, failWait) {
				return
			}
			failWait = min(failWait*2, alertRelayRetryMax)
			continue
		}
		failWait = alertRelayRetryMin
		if published == r.cfg.BatchSize {
			continue // a full batch: more may be waiting
		}
		if !r.idle(ctx, woken) {
			return
		}
	}
}

// publishBatch takes a batch and publishes it in order. At the first failure it gives the unpublished rest back, so
// they are taken again in the same order, and returns the error.
func (r *AlertRelay) publishBatch(ctx context.Context) (int, error) {
	marked, err := r.repo.MarkAlertsToPublish(ctx, r.cfg.MaxInFlight, r.cfg.BatchSize)
	if err != nil {
		return 0, err
	}
	for i, alert := range marked {
		if err := r.publish(ctx, alert); err != nil {
			rest := make([]string, 0, len(marked)-i)
			for _, a := range marked[i:] {
				rest = append(rest, a.ID)
			}
			unmarkCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			if uerr := r.repo.UnmarkPublished(unmarkCtx, rest); uerr != nil {
				slog.ErrorContext(ctx, "alert relay: could not give unpublished alerts back; the stale sweep publishes them",
					"alerts", len(rest), "err", uerr)
			}
			return i, fmt.Errorf("publish alert.created for %s: %w", alert.ID, err)
		}
	}
	return len(marked), nil
}

func (r *AlertRelay) publish(ctx context.Context, alert domain.AlertToPublish) error {
	payload, err := json.Marshal(events.AlertCreatedPayload{
		AlertID:     alert.ID,
		Fingerprint: alert.Fingerprint,
		Tier:        string(alert.Tier),
		ReceivedOn:  alert.ReceivedOn.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return err
	}
	value, err := json.Marshal(events.Envelope{Type: events.TypeAlertCreated, EntityID: alert.ID, Payload: payload})
	if err != nil {
		return err
	}
	// The fingerprint keeps one fingerprint's alerts on one partition; an alert without one goes by its own id.
	key := alert.Fingerprint
	if key == "" {
		key = alert.ID
	}
	publishCtx, cancel := context.WithTimeout(ctx, alertPublishTimeout)
	defer cancel()
	return r.producer.Publish(publishCtx, []byte(key), value)
}

// sweep publishes again the alerts whose event was lost or whose processor died: published longer than StaleAfter
// ago, still undecided and not claimed.
func (r *AlertRelay) sweep(ctx context.Context) {
	n, err := r.repo.RepublishStale(ctx, r.cfg.StaleAfter)
	if err != nil {
		if ctx.Err() == nil {
			slog.ErrorContext(ctx, "alert relay: the stale sweep failed", "err", err)
		}
		return
	}
	if n > 0 {
		slog.WarnContext(ctx, "alert relay: publishing again alerts nobody acted on", "alerts", n, "after", r.cfg.StaleAfter)
	}
}

// idle waits for a wake-up or PollInterval, whichever comes first; false when ctx is done.
func (r *AlertRelay) idle(ctx context.Context, woken <-chan struct{}) bool {
	select {
	case <-woken:
		return ctx.Err() == nil
	default:
	}
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-woken:
			cancel()
		case <-waitCtx.Done():
		}
	}()
	r.sleep(waitCtx, r.cfg.PollInterval)
	return ctx.Err() == nil
}

// alertWaker wakes every waiting goroutine together: broadcast closes the channel they wait on and starts a new one.
type alertWaker struct {
	mu sync.Mutex
	ch chan struct{}
}

func (w *alertWaker) wait() <-chan struct{} {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ch
}

func (w *alertWaker) broadcast() {
	w.mu.Lock()
	defer w.mu.Unlock()
	close(w.ch)
	w.ch = make(chan struct{})
}
