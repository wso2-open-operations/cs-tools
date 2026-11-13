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
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// AlertReceiver is the part of a Service Bus queue receiver the alert consumer uses; *azservicebus.Receiver satisfies it.
type AlertReceiver interface {
	ReceiveMessages(ctx context.Context, maxMessages int, options *azservicebus.ReceiveMessagesOptions) ([]*azservicebus.ReceivedMessage, error)
	CompleteMessage(ctx context.Context, message *azservicebus.ReceivedMessage, options *azservicebus.CompleteMessageOptions) error
	AbandonMessage(ctx context.Context, message *azservicebus.ReceivedMessage, options *azservicebus.AbandonMessageOptions) error
	DeadLetterMessage(ctx context.Context, message *azservicebus.ReceivedMessage, options *azservicebus.DeadLetterOptions) error
}

// AlertTransforms finds the transform for a gateway route; *alertsource.Registry satisfies it.
type AlertTransforms interface {
	Lookup(source string) (alertsource.Transform, bool)
}

// AlertConsumerConfig tunes the alert consumer.
type AlertConsumerConfig struct {
	// Concurrency is how many messages are processed at once across both queues.
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
)

// maxAlertsPerMessage bounds one message's alerts, far above any real batch but low enough for one insert.
const maxAlertsPerMessage = 1000

// maxDeadLetterDescription keeps a dead-letter description within what Service Bus accepts.
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

// processTimeout bounds one message's processing, kept well inside the queue's 60 second lock.
const processTimeout = 45 * time.Second

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

type alertLane struct {
	tier     domain.AlertTier
	queue    string
	receiver AlertReceiver
	messages chan *azservicebus.ReceivedMessage
}

// AlertConsumer streams alerts from the priority and standard Service Bus queues into the alert table.
type AlertConsumer struct {
	repo       repository.AlertRepository
	transforms AlertTransforms
	lanes      map[domain.AlertTier]*alertLane
	scheduler  *priorityScheduler[*azservicebus.ReceivedMessage]
	workers    int
	now        func() time.Time
	sleep      func(ctx context.Context, d time.Duration) bool
}

// NewAlertConsumer wires one receiver per queue; each queue buffers at most one message per worker, so locks stay fresh.
func NewAlertConsumer(
	repo repository.AlertRepository,
	transforms AlertTransforms,
	high AlertReceiver, highQueue string,
	standard AlertReceiver, standardQueue string,
	cfg AlertConsumerConfig,
) *AlertConsumer {
	workers := max(cfg.Concurrency, 1)
	lanes := map[domain.AlertTier]*alertLane{
		domain.AlertTierHigh:    {tier: domain.AlertTierHigh, queue: highQueue, receiver: high, messages: make(chan *azservicebus.ReceivedMessage, workers)},
		domain.AlertTierDefault: {tier: domain.AlertTierDefault, queue: standardQueue, receiver: standard, messages: make(chan *azservicebus.ReceivedMessage, workers)},
	}
	return &AlertConsumer{
		repo:       repo,
		transforms: transforms,
		lanes:      lanes,
		scheduler:  newPriorityScheduler(lanes[domain.AlertTierHigh].messages, lanes[domain.AlertTierDefault].messages, cfg.PriorityWeight),
		workers:    workers,
		now:        time.Now,
		sleep:      sleepContext,
	}
}

// Run receives and processes alerts until ctx is done, then finishes the alerts in flight and hands buffered ones back.
func (c *AlertConsumer) Run(ctx context.Context) {
	var receiving, working sync.WaitGroup
	for _, lane := range c.lanes {
		receiving.Go(func() { c.receive(ctx, lane) })
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

// receive keeps one link open on a queue and asks for as many messages as the lane has room for.
func (c *AlertConsumer) receive(ctx context.Context, lane *alertLane) {
	backoff := receiveBackoffMin
	for ctx.Err() == nil {
		room := max(cap(lane.messages)-len(lane.messages), 1)
		messages, err := lane.receiver.ReceiveMessages(ctx, room, nil)
		if err != nil {
			if ctx.Err() != nil {
				c.abandonAll(lane, messages)
				return
			}
			slog.ErrorContext(ctx, "alert consumer: receive failed, retrying", "queue", lane.queue, "retryIn", backoff, "err", err)
			if !c.sleep(ctx, backoff) {
				return
			}
			backoff = min(2*backoff, receiveBackoffMax)
			continue
		}
		backoff = receiveBackoffMin
		for i, message := range messages {
			select {
			case lane.messages <- message:
			case <-ctx.Done():
				c.abandonAll(lane, messages[i:])
				return
			}
		}
	}
}

// work takes the next message by priority and settles it, until ctx is done.
func (c *AlertConsumer) work(ctx context.Context) {
	for {
		message, tier, ok := c.scheduler.next(ctx)
		if !ok {
			return
		}
		c.handle(ctx, c.lanes[tier], message)
	}
}

// handle processes and settles one message; it runs on a context shutdown does not cancel, so an alert in flight is finished.
func (c *AlertConsumer) handle(ctx context.Context, lane *alertLane, message *azservicebus.ReceivedMessage) {
	processCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), processTimeout)
	defer cancel()
	decision := c.process(processCtx, lane.tier, message)
	c.settle(processCtx, lane, message, decision)
}

// process stores one message's alerts and decides how to settle it; bad payloads are dead-lettered, store failures retried.
func (c *AlertConsumer) process(ctx context.Context, tier domain.AlertTier, message *azservicebus.ReceivedMessage) alertDecision {
	source := strings.ToLower(messageStringProperty(message.ApplicationProperties, alertPropertySource))
	if source == "" {
		return alertDeadLetterDecision(alertDeadLetterMissingSource, "the message has no source property")
	}
	transform, ok := c.transforms.Lookup(source)
	if !ok {
		return alertDeadLetterDecision(alertDeadLetterUnknownSource, fmt.Sprintf("no transform for source %q", source))
	}
	alerts, err := transform(message.Body)
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
		slog.ErrorContext(ctx, "alert consumer: could not reserve alert ids, the message will be redelivered", "messageId", message.MessageID, "err", err)
		return alertDecision{settlement: alertAbandon, alerts: len(alerts)}
	}

	rows, err := c.buildRows(source, tier, message, alerts, ids)
	if err != nil {
		return alertDeadLetterDecision(alertDeadLetterTransformFailed, err.Error())
	}
	var stored int64
	if err := c.withStoreRetry(ctx, func() (err error) {
		stored, err = c.repo.InsertAlerts(ctx, rows)
		return err
	}); err != nil {
		slog.ErrorContext(ctx, "alert consumer: could not store alerts, the message will be redelivered", "messageId", message.MessageID, "err", err)
		return alertDecision{settlement: alertAbandon, alerts: len(alerts)}
	}
	return alertDecision{settlement: alertComplete, stored: stored, alerts: len(alerts)}
}

// buildRows gives each alert its id and fingerprint the way alert-ingestion does: an alert with no identifier takes its own id.
func (c *AlertConsumer) buildRows(source string, tier domain.AlertTier, message *azservicebus.ReceivedMessage, alerts []model.Alert, ids []string) ([]domain.NewAlert, error) {
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
		rows[i] = domain.NewAlert{
			ID:           ids[i],
			Source:       source,
			Alert:        body,
			Fingerprint:  model.Fingerprint(alert),
			Tier:         tier,
			MessageID:    message.MessageID,
			MessageIndex: i,
			ReceivedOn:   receivedOn,
		}
	}
	return rows, nil
}

// receivedOn is when the gateway accepted the request, falling back to when Service Bus stored the message.
func (c *AlertConsumer) receivedOn(message *azservicebus.ReceivedMessage) time.Time {
	if raw := messageStringProperty(message.ApplicationProperties, alertPropertyReceivedAt); raw != "" {
		if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			return t.UTC()
		}
	}
	if message.EnqueuedTime != nil {
		return message.EnqueuedTime.UTC()
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

// settle completes, dead-letters or abandons a message; a failed settlement only means Service Bus redelivers it.
func (c *AlertConsumer) settle(ctx context.Context, lane *alertLane, message *azservicebus.ReceivedMessage, decision alertDecision) {
	settleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), settleTimeout)
	defer cancel()
	var err error
	switch decision.settlement {
	case alertComplete:
		err = lane.receiver.CompleteMessage(settleCtx, message, nil)
		if err == nil && decision.stored < int64(decision.alerts) {
			slog.InfoContext(ctx, "alert consumer: redelivered message was already stored", "messageId", message.MessageID, "queue", lane.queue, "alerts", decision.alerts, "new", decision.stored)
		}
	case alertDeadLetter:
		slog.WarnContext(ctx, "alert consumer: dead-lettering a message", "messageId", message.MessageID, "queue", lane.queue, "reason", decision.reason)
		reason, description := decision.reason, truncateRunes(decision.description, maxDeadLetterDescription)
		err = lane.receiver.DeadLetterMessage(settleCtx, message, &azservicebus.DeadLetterOptions{Reason: &reason, ErrorDescription: &description})
	case alertAbandon:
		err = lane.receiver.AbandonMessage(settleCtx, message, nil)
	}
	if err != nil {
		slog.WarnContext(ctx, "alert consumer: settlement failed, Service Bus will redeliver the message", "messageId", message.MessageID, "queue", lane.queue, "err", err)
	}
}

// abandonAll hands messages back so they are redelivered at once rather than after their lock expires.
func (c *AlertConsumer) abandonAll(lane *alertLane, messages []*azservicebus.ReceivedMessage) {
	for _, message := range messages {
		c.settle(context.Background(), lane, message, alertDecision{settlement: alertAbandon})
	}
}

// abandonBuffered hands back messages that were received but never picked up by a worker.
func (c *AlertConsumer) abandonBuffered(lane *alertLane) {
	for {
		select {
		case message := <-lane.messages:
			c.abandonAll(lane, []*azservicebus.ReceivedMessage{message})
		default:
			return
		}
	}
}

func alertDeadLetterDecision(reason, description string) alertDecision {
	return alertDecision{settlement: alertDeadLetter, reason: reason, description: description}
}

// messageStringProperty reads an application property as text; the gateway sends every property as a string.
func messageStringProperty(properties map[string]any, key string) string {
	value, ok := properties[key]
	if !ok || value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(value))
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
