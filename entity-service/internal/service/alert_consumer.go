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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// AlertQueue is one alert queue on one cloud; servicebus.Queue and sqsqueue.Queue satisfy it.
type AlertQueue interface {
	// Name identifies the queue in logs.
	Name() string
	// Receive waits until at least one message arrives or ctx is done, and returns at most maxMessages.
	Receive(ctx context.Context, maxMessages int) ([]*domain.AlertQueueMessage, error)
	// Complete removes a message for good.
	Complete(ctx context.Context, message *domain.AlertQueueMessage) error
	// Abandon makes a message available again at once.
	Abandon(ctx context.Context, message *domain.AlertQueueMessage) error
	// DeadLetter moves a message that can never be stored aside, with the reason.
	DeadLetter(ctx context.Context, message *domain.AlertQueueMessage, reason, description string) error
	// RenewLock extends how long the message stays hidden from other receivers, and updates LockedUntil.
	RenewLock(ctx context.Context, message *domain.AlertQueueMessage) error
}

// AlertStore is the storage the alert consumer writes to; repository.AlertRepository satisfies it.
type AlertStore interface {
	ReserveAlertIDs(ctx context.Context, n int) ([]string, error)
	InsertAlerts(ctx context.Context, alerts []domain.NewAlert) (int64, error)
}

// AlertTransforms finds the transform for a gateway route; *alertsource.Registry satisfies it.
type AlertTransforms interface {
	Lookup(source string) (alertsource.Transform, bool)
}

// AlertConsumerConfig tunes the alert consumer.
type AlertConsumerConfig struct {
	// Concurrency is how many messages are processed at once across every queue.
	Concurrency int
	// PriorityWeight is how many priority alerts may be processed in a row while a standard alert waits.
	PriorityWeight int
}

// Message properties the alerting gateway sets on every message.
const (
	alertPropertySource     = "source"
	alertPropertyReceivedAt = "received_at"
)

// Dead-letter reasons, shown on the dead-letter card so a person can tell why an alert was not stored.
const (
	alertDeadLetterMissingSource   = "MissingSource"
	alertDeadLetterUnknownSource   = "UnknownSource"
	alertDeadLetterTransformFailed = "TransformFailed"
	alertDeadLetterTooManyAlerts   = "TooManyAlerts"
	alertDeadLetterUnstorable      = "UnstorableAlert"
)

// errUnstorableAlert marks an alert PostgreSQL refuses on every attempt, so it is dead-lettered instead of retried.
var errUnstorableAlert = errors.New("the alert holds a NUL character, which PostgreSQL jsonb cannot store")

// maxAlertsPerMessage bounds one message's alerts, far above any real batch but low enough for one insert.
const maxAlertsPerMessage = 1000

// maxDeadLetterDescription keeps a dead-letter description within what every cloud accepts.
const maxDeadLetterDescription = 1024

// Store attempts happen while the message lock is held, so a short database blip does not cost a delivery.
var alertStoreRetryDelays = []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second}

// Receive errors back off up to receiveBackoffMax, so a broken connection is retried without spinning.
const (
	receiveBackoffMin = time.Second
	receiveBackoffMax = 30 * time.Second
)

// settleTimeout bounds one settlement call, which also runs after shutdown has begun.
const settleTimeout = 30 * time.Second

// processTimeout bounds one message's processing; the lock is renewed meanwhile, so it may run past the queue's 60 second lock.
const processTimeout = 45 * time.Second

// defaultLockRenewMargin is how long before a message lock expires it is renewed.
const defaultLockRenewMargin = 15 * time.Second

// settleLockMargin is the lock a message must have left before settling, so a settlement that takes its full timeout still holds the lock.
const settleLockMargin = settleTimeout + 5*time.Second

// defaultLockRenewCallTimeout bounds one renewal call, shorter than defaultLockRenewMargin so a hung call ends while the lock still holds.
const defaultLockRenewCallTimeout = 10 * time.Second

// minLockRenewWait keeps renewal from spinning when a lock is already close to expiry.
const minLockRenewWait = time.Second

type alertSettlement int

const (
	alertComplete alertSettlement = iota
	alertDeadLetter
	alertAbandon
)

// alertDecision is what to do with a message once it has been processed.
type alertDecision struct {
	settlement  alertSettlement
	reason      string
	description string
	stored      int64
	alerts      int
}

// queuedMessage is a received message with the queue it must be settled on.
type queuedMessage struct {
	message *domain.AlertQueueMessage
	queue   AlertQueue
	// release frees the receive slot the message holds on its queue.
	release func()
	// stopRenewing stops the lock renewal that has run since the message was received.
	stopRenewing func()
}

// takeOver stops the receipt-time renewal and frees the receive slot, so the caller alone handles the message from here.
func (q queuedMessage) takeOver() {
	if q.stopRenewing != nil {
		q.stopRenewing()
	}
	if q.release != nil {
		q.release()
	}
}

// alertLane is one tier: every queue of that tier, on every cloud, feeds its buffer.
type alertLane struct {
	tier   domain.AlertTier
	queues []AlertQueue
	// share is how many messages each queue may hold received but not yet taken by a worker, so one queue cannot fill the tier.
	share    int
	messages chan queuedMessage
}

func newAlertLane(tier domain.AlertTier, queues []AlertQueue, workers int) *alertLane {
	share := max(workers/max(len(queues), 1), 1)
	return &alertLane{tier: tier, queues: queues, share: share, messages: make(chan queuedMessage, share*max(len(queues), 1))}
}

// AlertConsumer streams alerts from the priority and standard queues of every configured cloud into the alert table.
type AlertConsumer struct {
	repo            AlertStore
	transforms      AlertTransforms
	lanes           map[domain.AlertTier]*alertLane
	scheduler       *scheduler[queuedMessage]
	workers         int
	now             func() time.Time
	sleep           func(ctx context.Context, d time.Duration) bool
	lockRenewMargin time.Duration
	// lockRenewCallTimeout bounds each renewal call made while a message waits or is processed.
	lockRenewCallTimeout time.Duration
}

// NewAlertConsumer wires every priority queue into the priority tier and every standard queue into the standard tier; each queue holds at most its share of one message per worker.
func NewAlertConsumer(
	repo AlertStore,
	transforms AlertTransforms,
	priorityQueues, standardQueues []AlertQueue,
	cfg AlertConsumerConfig,
) *AlertConsumer {
	workers := max(cfg.Concurrency, 1)
	lanes := map[domain.AlertTier]*alertLane{
		domain.AlertTierHigh:    newAlertLane(domain.AlertTierHigh, priorityQueues, workers),
		domain.AlertTierDefault: newAlertLane(domain.AlertTierDefault, standardQueues, workers),
	}
	return &AlertConsumer{
		repo:                 repo,
		transforms:           transforms,
		lanes:                lanes,
		scheduler:            newScheduler(lanes[domain.AlertTierHigh].messages, lanes[domain.AlertTierDefault].messages, cfg.PriorityWeight),
		workers:              workers,
		now:                  time.Now,
		sleep:                sleepContext,
		lockRenewMargin:      defaultLockRenewMargin,
		lockRenewCallTimeout: defaultLockRenewCallTimeout,
	}
}

// Run receives and processes alerts until ctx is done, then finishes the alerts in flight and hands buffered ones back.
func (c *AlertConsumer) Run(ctx context.Context) {
	var receiving, working sync.WaitGroup
	for _, lane := range c.lanes {
		for _, queue := range lane.queues {
			receiving.Go(func() { c.receive(ctx, lane, queue) })
		}
	}
	for range c.workers {
		working.Go(func() { c.work(ctx) })
	}
	receiving.Wait()
	working.Wait()
	for _, lane := range c.lanes {
		c.abandonBuffered(lane)
	}
}

// receive keeps waiting on one queue and asks for as many messages as the queue has free slots; a message holds its slot until a worker takes it.
func (c *AlertConsumer) receive(ctx context.Context, lane *alertLane, queue AlertQueue) {
	slots := make(chan struct{}, lane.share)
	release := func() { <-slots }
	backoff := receiveBackoffMin
	for ctx.Err() == nil {
		reserved := reserveSlots(ctx, slots)
		if reserved == 0 {
			return
		}
		messages, err := queue.Receive(ctx, reserved)
		for range reserved - len(messages) {
			release()
		}
		if err != nil {
			for range messages {
				release()
			}
			if ctx.Err() != nil {
				c.abandonAll(queue, messages)
				return
			}
			slog.ErrorContext(ctx, "alert consumer: receive failed, retrying", "queue", queue.Name(), "retryIn", backoff, "err", err)
			if !c.sleep(ctx, backoff) {
				return
			}
			backoff = min(2*backoff, receiveBackoffMax)
			continue
		}
		backoff = receiveBackoffMin
		// Renewal starts at receipt, so a message keeps its lock while it waits for a worker.
		queued := make([]queuedMessage, len(messages))
		for i, message := range messages {
			queued[i] = queuedMessage{message: message, queue: queue, release: release, stopRenewing: c.keepLock(context.WithoutCancel(ctx), queue, message)}
		}
		for i := range queued {
			select {
			case lane.messages <- queued[i]:
			case <-ctx.Done():
				for _, rest := range queued[i:] {
					rest.takeOver()
					c.abandonAll(queue, []*domain.AlertQueueMessage{rest.message})
				}
				return
			}
		}
	}
}

// reserveSlots waits for one free slot, then takes every other free one, and returns how many it holds; 0 means ctx is done.
func reserveSlots(ctx context.Context, slots chan struct{}) int {
	select {
	case slots <- struct{}{}:
	case <-ctx.Done():
		return 0
	}
	n := 1
	for n < cap(slots) {
		select {
		case slots <- struct{}{}:
			n++
		default:
			return n
		}
	}
	return n
}

// work takes the next message by priority and settles it, until ctx is done.
func (c *AlertConsumer) work(ctx context.Context) {
	for {
		queued, tier, ok := c.scheduler.next(ctx)
		if !ok {
			return
		}
		queued.takeOver()
		c.handle(ctx, tier, queued.queue, queued.message)
	}
}

// handle processes and settles one message; it runs on a context shutdown does not cancel, so an alert in flight is finished.
func (c *AlertConsumer) handle(ctx context.Context, tier domain.AlertTier, queue AlertQueue, message *domain.AlertQueueMessage) {
	processCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), processTimeout)
	defer cancel()
	if err := c.renewLockIfDue(processCtx, queue, message, c.lockRenewMargin); err != nil {
		slog.WarnContext(ctx, "alert consumer: lock lost while the message waited for a worker, the queue will redeliver it", "messageId", message.ID, "queue", queue.Name(), "err", err)
		return
	}
	stopRenewing := c.keepLock(processCtx, queue, message)
	decision := c.process(processCtx, tier, queue.Name(), message)
	stopRenewing()
	// Renewal stops before settling, so the two never touch the message at once; the last renewal leaves room for a settlement that takes its full timeout.
	renewCtx, cancelRenew := context.WithTimeout(context.WithoutCancel(ctx), settleTimeout)
	defer cancelRenew()
	if err := c.renewLockIfDue(renewCtx, queue, message, settleLockMargin); err != nil {
		slog.WarnContext(ctx, "alert consumer: lock lost before settlement, the queue will redeliver the message", "messageId", message.ID, "queue", queue.Name(), "err", err)
		return
	}
	c.settle(processCtx, queue, message, decision)
}

// renewLockIfDue renews the message lock when it expires within margin.
func (c *AlertConsumer) renewLockIfDue(ctx context.Context, queue AlertQueue, message *domain.AlertQueueMessage, margin time.Duration) error {
	if message.LockedUntil == nil || c.now().Add(margin).Before(*message.LockedUntil) {
		return nil
	}
	return queue.RenewLock(ctx, message)
}

// keepLock renews the message lock while it is processed and returns the function that stops renewing.
func (c *AlertConsumer) keepLock(ctx context.Context, queue AlertQueue, message *domain.AlertQueueMessage) func() {
	done := make(chan struct{})
	var renewing sync.WaitGroup
	renewing.Go(func() {
		for {
			wait := c.lockRenewMargin
			if message.LockedUntil != nil {
				wait = max(message.LockedUntil.Sub(c.now())-c.lockRenewMargin, minLockRenewWait)
			}
			timer := time.NewTimer(wait)
			select {
			case <-done:
				timer.Stop()
				return
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			callCtx, cancelCall := context.WithTimeout(ctx, c.lockRenewCallTimeout)
			err := queue.RenewLock(callCtx, message)
			cancelCall()
			if err != nil {
				slog.WarnContext(ctx, "alert consumer: lock renewal failed", "messageId", message.ID, "queue", queue.Name(), "err", err)
				return
			}
		}
	})
	return func() {
		close(done)
		renewing.Wait()
	}
}

// process stores one message's alerts and decides how to settle it; bad payloads are dead-lettered, store failures retried.
func (c *AlertConsumer) process(ctx context.Context, tier domain.AlertTier, queueName string, message *domain.AlertQueueMessage) alertDecision {
	source := strings.ToLower(strings.TrimSpace(message.Properties[alertPropertySource]))
	if source == "" {
		return alertDeadLetterDecision(alertDeadLetterMissingSource, "the message has no source property")
	}
	transform, ok := c.transforms.Lookup(source)
	if !ok {
		return alertDeadLetterDecision(alertDeadLetterUnknownSource, fmt.Sprintf("no transform for source %q", source))
	}
	alerts, err := safeTransform(transform, message.Body)
	if err != nil {
		return alertDeadLetterDecision(alertDeadLetterTransformFailed, err.Error())
	}
	if len(alerts) > maxAlertsPerMessage {
		return alertDeadLetterDecision(alertDeadLetterTooManyAlerts, fmt.Sprintf("%d alerts in one message, at most %d are accepted", len(alerts), maxAlertsPerMessage))
	}
	if len(alerts) == 0 {
		return alertDecision{settlement: alertComplete}
	}

	var ids []string
	if err := c.withStoreRetry(ctx, func() (err error) {
		ids, err = c.repo.ReserveAlertIDs(ctx, len(alerts))
		return err
	}); err != nil {
		slog.ErrorContext(ctx, "alert consumer: could not reserve alert ids, the message will be redelivered", "messageId", message.ID, "err", err)
		return alertDecision{settlement: alertAbandon, alerts: len(alerts)}
	}

	rows, err := c.buildRows(source, tier, queueName, message, alerts, ids)
	if errors.Is(err, errUnstorableAlert) {
		return alertDeadLetterDecision(alertDeadLetterUnstorable, err.Error())
	}
	if err != nil {
		return alertDeadLetterDecision(alertDeadLetterTransformFailed, err.Error())
	}
	var stored int64
	if err := c.withStoreRetry(ctx, func() (err error) {
		stored, err = c.repo.InsertAlerts(ctx, rows)
		return err
	}); err != nil {
		slog.ErrorContext(ctx, "alert consumer: could not store alerts, the message will be redelivered", "messageId", message.ID, "err", err)
		return alertDecision{settlement: alertAbandon, alerts: len(alerts)}
	}
	return alertDecision{settlement: alertComplete, stored: stored, alerts: len(alerts)}
}

// safeTransform turns a panic in a transform into an error, so one malformed payload is dead-lettered instead of stopping the service.
func safeTransform(transform alertsource.Transform, body []byte) (alerts []model.Alert, err error) {
	defer func() {
		if r := recover(); r != nil {
			alerts, err = nil, fmt.Errorf("transform panicked: %v", r)
		}
	}()
	return transform(body)
}

// buildRows gives each alert its id and fingerprint the way alert-ingestion does: an alert with no identifier takes its own id.
func (c *AlertConsumer) buildRows(source string, tier domain.AlertTier, queueName string, message *domain.AlertQueueMessage, alerts []model.Alert, ids []string) ([]domain.NewAlert, error) {
	messageKey := alertMessageKey(queueName, message.ID)
	receivedOn := c.receivedOn(message)
	rows := make([]domain.NewAlert, len(alerts))
	for i, alert := range alerts {
		if alert.UniqueIdentifier == "" {
			alert.UniqueIdentifier = ids[i]
		}
		body, err := json.Marshal(alert)
		if err != nil {
			return nil, fmt.Errorf("encode alert %d: %w", i, err)
		}
		if hasEscapedNUL(body) {
			return nil, fmt.Errorf("alert %d: %w", i, errUnstorableAlert)
		}
		rows[i] = domain.NewAlert{
			ID:           ids[i],
			Source:       source,
			Alert:        body,
			Fingerprint:  model.Fingerprint(alert),
			Tier:         tier,
			MessageID:    messageKey,
			MessageIndex: i,
			ReceivedOn:   receivedOn,
		}
	}
	return rows, nil
}

// alertMessageKey is the stored message id: the queue's name and the message's own id, so a redelivery from one queue matches its earlier rows while the same id from another queue never collides with them.
func alertMessageKey(queueName, messageID string) string {
	return queueName + "/" + messageID
}

// hasEscapedNUL reports whether encoded JSON holds a \u0000 escape, skipping escaped backslashes so a literal "\\u0000" does not count.
func hasEscapedNUL(body []byte) bool {
	for i := 0; i < len(body); i++ {
		if body[i] != '\\' {
			continue
		}
		if bytes.HasPrefix(body[i+1:], []byte("u0000")) {
			return true
		}
		i++
	}
	return false
}

// receivedOn is when the gateway accepted the request, falling back to when the queue stored the message.
func (c *AlertConsumer) receivedOn(message *domain.AlertQueueMessage) time.Time {
	if raw := strings.TrimSpace(message.Properties[alertPropertyReceivedAt]); raw != "" {
		if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			return t.UTC()
		}
	}
	if message.EnqueuedOn != nil {
		return message.EnqueuedOn.UTC()
	}
	return c.now().UTC()
}

// withStoreRetry runs fn, retrying a failed attempt after each delay in alertStoreRetryDelays.
func (c *AlertConsumer) withStoreRetry(ctx context.Context, fn func() error) error {
	err := fn()
	for _, delay := range alertStoreRetryDelays {
		if err == nil || !c.sleep(ctx, delay) {
			return err
		}
		err = fn()
	}
	return err
}

// settle completes, dead-letters or abandons a message; a failed settlement only means the queue redelivers it.
func (c *AlertConsumer) settle(ctx context.Context, queue AlertQueue, message *domain.AlertQueueMessage, decision alertDecision) {
	settleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), settleTimeout)
	defer cancel()
	var err error
	switch decision.settlement {
	case alertComplete:
		err = queue.Complete(settleCtx, message)
		if err == nil && decision.stored < int64(decision.alerts) {
			slog.InfoContext(ctx, "alert consumer: redelivered message was already stored", "messageId", message.ID, "queue", queue.Name(), "alerts", decision.alerts, "new", decision.stored)
		}
	case alertDeadLetter:
		slog.WarnContext(ctx, "alert consumer: dead-lettering a message", "messageId", message.ID, "queue", queue.Name(), "reason", decision.reason)
		err = queue.DeadLetter(settleCtx, message, decision.reason, truncateRunes(decision.description, maxDeadLetterDescription))
	case alertAbandon:
		err = queue.Abandon(settleCtx, message)
	}
	if err != nil {
		slog.WarnContext(ctx, "alert consumer: settlement failed, the queue will redeliver the message", "messageId", message.ID, "queue", queue.Name(), "err", err)
	}
}

// abandonAll hands messages back so they are redelivered at once rather than after their lock expires.
func (c *AlertConsumer) abandonAll(queue AlertQueue, messages []*domain.AlertQueueMessage) {
	for _, message := range messages {
		c.settle(context.Background(), queue, message, alertDecision{settlement: alertAbandon})
	}
}

// abandonBuffered hands back messages that were received but never picked up by a worker.
func (c *AlertConsumer) abandonBuffered(lane *alertLane) {
	for {
		select {
		case queued := <-lane.messages:
			queued.takeOver()
			c.abandonAll(queued.queue, []*domain.AlertQueueMessage{queued.message})
		default:
			return
		}
	}
}

func alertDeadLetterDecision(reason, description string) alertDecision {
	return alertDecision{settlement: alertDeadLetter, reason: reason, description: description}
}

// sleepContext waits for d and reports whether ctx was still live afterwards.
func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
