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
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertqueue"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// AlertQueueMessage and AlertQueue are the consumer's view of a cloud queue (internal/alertqueue).
type (
	AlertQueueMessage = alertqueue.Message
	AlertQueue        = alertqueue.Queue
)

// AlertTransforms looks up a source's transform; *alertsource.Registry satisfies it.
type AlertTransforms interface {
	Lookup(source string) (alertsource.Transform, bool)
}

// AlertConsumerConfig tunes the alert consumer.
type AlertConsumerConfig struct {
	// Concurrency is how many messages are stored at once.
	Concurrency int
}

// alertReceiveBatch is the most messages one Receive asks for; SQS returns at most 10.
const alertReceiveBatch = 10

// alertStoreTimeout bounds storing one message's rows.
const alertStoreTimeout = 30 * time.Second

// alertSettleTimeout bounds one complete, abandon or dead-letter call, also during shutdown.
const alertSettleTimeout = 15 * time.Second

// Receive errors are retried after alertReceiveRetryMin, doubling up to alertReceiveRetryMax.
const (
	alertReceiveRetryMin = time.Second
	alertReceiveRetryMax = 30 * time.Second
)

// maxAlertErrorText keeps a stored or dead-lettered error description short; the full error is in the log.
const maxAlertErrorText = 1000

// AlertConsumer receives SRE alerts from the priority and standard queues of every cloud, turns each message into
// canonical alerts with its source's transform, stores them in the alert table and only then completes the message.
// A message it cannot read is stored as an error row and completed, so it is kept, not lost or retried forever. A
// free worker always takes a waiting priority message before a standard one.
type AlertConsumer struct {
	store      repository.AlertRepository
	transforms AlertTransforms
	priority   []AlertQueue
	standard   []AlertQueue
	cfg        AlertConsumerConfig
	now        func() time.Time
	sleep      func(ctx context.Context, d time.Duration) bool
}

// NewAlertConsumer returns an AlertConsumer; Run starts it.
func NewAlertConsumer(store repository.AlertRepository, transforms AlertTransforms, priority, standard []AlertQueue, cfg AlertConsumerConfig) *AlertConsumer {
	cfg.Concurrency = max(cfg.Concurrency, 1)
	return &AlertConsumer{
		store:      store,
		transforms: transforms,
		priority:   priority,
		standard:   standard,
		cfg:        cfg,
		now:        time.Now,
		sleep:      sleepContext,
	}
}

// heldAlertMessage is a received message with the queue it came from and the renewal that keeps it locked.
type heldAlertMessage struct {
	queue       AlertQueue
	tier        domain.AlertTier
	msg         AlertQueueMessage
	receivedOn  time.Time
	stopRenewal context.CancelFunc
	renewalDone chan struct{}
}

// Run receives and stores alerts until ctx is done. Messages in flight are finished; messages received but not yet
// started are abandoned, so they are delivered again at once.
func (c *AlertConsumer) Run(ctx context.Context) {
	// Each tier's channel holds received messages until a worker is free. Its capacity, with the workers, bounds how
	// many messages are held (locked) at once.
	priorityCh := make(chan *heldAlertMessage, c.cfg.Concurrency)
	standardCh := make(chan *heldAlertMessage, c.cfg.Concurrency)
	// One token per message held anywhere: a receiver takes tokens before it receives, a worker gives one back when it
	// settles a message. So no more is received than can be stored before the locks need renewing many times.
	tokens := make(chan struct{}, 2*c.cfg.Concurrency)
	for range cap(tokens) {
		tokens <- struct{}{}
	}

	var receivers sync.WaitGroup
	for _, q := range c.priority {
		receivers.Go(func() { c.receive(ctx, q, domain.AlertTierHigh, priorityCh, tokens) })
	}
	for _, q := range c.standard {
		receivers.Go(func() { c.receive(ctx, q, domain.AlertTierDefault, standardCh, tokens) })
	}
	var workers sync.WaitGroup
	for range c.cfg.Concurrency {
		workers.Go(func() { c.work(ctx, priorityCh, standardCh, tokens) })
	}

	receivers.Wait()
	workers.Wait()
	// Nobody reads the channels any more: give back what was received but never started.
	close(priorityCh)
	close(standardCh)
	for _, ch := range []chan *heldAlertMessage{priorityCh, standardCh} {
		for h := range ch {
			c.settle(ctx, h, func(sctx context.Context) error { return h.queue.Abandon(sctx, h.msg) }, "abandon")
		}
	}
}

// receive long-polls one queue and hands every message to its tier's channel, holding its lock meanwhile.
func (c *AlertConsumer) receive(ctx context.Context, q AlertQueue, tier domain.AlertTier, out chan<- *heldAlertMessage, tokens chan struct{}) {
	wait := alertReceiveRetryMin
	for ctx.Err() == nil {
		// Take at least one token, then as many more as are free, up to a batch.
		select {
		case <-tokens:
		case <-ctx.Done():
			return
		}
		taken := 1
		for taken < alertReceiveBatch {
			select {
			case <-tokens:
				taken++
				continue
			default:
			}
			break
		}
		msgs, err := q.Receive(ctx, taken)
		for range taken - len(msgs) {
			tokens <- struct{}{}
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.ErrorContext(ctx, "alert consumer: receiving failed, retrying", "queue", q.Name(), "in", wait, "err", err)
			if !c.sleep(ctx, wait) {
				return
			}
			wait = min(wait*2, alertReceiveRetryMax)
			continue
		}
		wait = alertReceiveRetryMin
		for _, m := range msgs {
			h := c.hold(ctx, q, tier, m)
			select {
			case out <- h:
			case <-ctx.Done():
				// Shutting down before a worker took it: give it back now.
				c.settle(ctx, h, func(sctx context.Context) error { return q.Abandon(sctx, m) }, "abandon")
				tokens <- struct{}{}
			}
		}
	}
}

// hold starts renewing a received message's lock until it is settled.
func (c *AlertConsumer) hold(ctx context.Context, q AlertQueue, tier domain.AlertTier, m AlertQueueMessage) *heldAlertMessage {
	renewCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	h := &heldAlertMessage{queue: q, tier: tier, msg: m, receivedOn: c.now(), stopRenewal: stop, renewalDone: make(chan struct{})}
	go func() {
		defer close(h.renewalDone)
		interval := q.LockRenewInterval()
		if interval <= 0 {
			return
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-renewCtx.Done():
				return
			case <-ticker.C:
				rctx, cancel := context.WithTimeout(renewCtx, alertSettleTimeout)
				err := q.RenewLock(rctx, m)
				cancel()
				if err != nil && renewCtx.Err() == nil {
					slog.WarnContext(ctx, "alert consumer: renewing a message lock failed; it may be delivered again", "queue", q.Name(), "messageId", m.ID(), "err", err)
				}
			}
		}
	}()
	return h
}

// work stores messages, priority first, until ctx is done.
func (c *AlertConsumer) work(ctx context.Context, priority, standard <-chan *heldAlertMessage, tokens chan<- struct{}) {
	for ctx.Err() == nil {
		var h *heldAlertMessage
		select {
		case h = <-priority:
		default:
			select {
			case h = <-priority:
			case h = <-standard:
			case <-ctx.Done():
				return
			}
		}
		c.handle(ctx, h)
		tokens <- struct{}{}
	}
}

// handle stores one message's alerts and settles it. A message in flight is finished even during shutdown.
func (c *AlertConsumer) handle(ctx context.Context, h *heldAlertMessage) {
	rows := c.rowsFor(h)
	storeCtx, cancel := context.WithTimeout(repository.WithSystemIdentity(context.WithoutCancel(ctx)), alertStoreTimeout)
	defer cancel()
	err := c.store1(storeCtx, h, rows)
	if err == nil {
		c.settle(ctx, h, func(sctx context.Context) error { return h.queue.Complete(sctx, h.msg) }, "complete")
		return
	}
	if !isPermanentStoreError(err) {
		// The database is unreachable or busy: give the message back; the queue delivers it again.
		slog.ErrorContext(ctx, "alert consumer: storing a message failed, it will be delivered again", "queue", h.queue.Name(), "messageId", h.msg.ID(), "err", err)
		c.settle(ctx, h, func(sctx context.Context) error { return h.queue.Abandon(sctx, h.msg) }, "abandon")
		return
	}
	// The database refused the rows themselves. Keep the message as one error row with its raw body instead.
	slog.ErrorContext(ctx, "alert consumer: the database refused a message's alerts; storing it as an error row", "queue", h.queue.Name(), "messageId", h.msg.ID(), "err", err)
	errRow := c.errorRow(h, domain.AlertErrorStoreRefused, err.Error())
	if err2 := c.store1(storeCtx, h, []domain.NewAlert{errRow}); err2 == nil {
		c.settle(ctx, h, func(sctx context.Context) error { return h.queue.Complete(sctx, h.msg) }, "complete")
		return
	} else if !isPermanentStoreError(err2) {
		c.settle(ctx, h, func(sctx context.Context) error { return h.queue.Abandon(sctx, h.msg) }, "abandon")
		return
	}
	// Not even the error row can be stored: dead-letter the message, the last resort.
	slog.ErrorContext(ctx, "alert consumer: not even an error row could be stored; dead-lettering the message", "queue", h.queue.Name(), "messageId", h.msg.ID())
	c.settle(ctx, h, func(sctx context.Context) error {
		return h.queue.DeadLetter(sctx, h.msg, domain.AlertErrorStoreRefused, truncateRunes(err.Error(), maxAlertErrorText))
	}, "dead-letter")
}

// store1 reserves ids for rows and stores them. Ids are reserved per attempt, so a retried message gets new ones; the
// unique message key makes the second store a no-op.
func (c *AlertConsumer) store1(ctx context.Context, h *heldAlertMessage, rows []domain.NewAlert) error {
	ids, err := c.store.ReserveAlertIDs(ctx, len(rows))
	if err != nil {
		return err
	}
	if len(ids) != len(rows) {
		return fmt.Errorf("reserved %d alert ids for %d rows", len(ids), len(rows))
	}
	for i := range rows {
		rows[i].ID = ids[i]
	}
	n, err := c.store.InsertAlerts(ctx, rows)
	if err != nil {
		return err
	}
	if n == 0 {
		slog.InfoContext(ctx, "alert consumer: message already stored, completing it", "queue", h.queue.Name(), "messageId", h.msg.ID())
	}
	return nil
}

// rowsFor turns a message into rows: one per alert its source's transform returns, or one error row saying why it
// could not be read. The first row keeps the raw body.
func (c *AlertConsumer) rowsFor(h *heldAlertMessage) []domain.NewAlert {
	source := strings.ToLower(strings.TrimSpace(h.msg.Source()))
	if source == "" {
		return []domain.NewAlert{c.errorRow(h, domain.AlertErrorNoSource, "the message has no source property")}
	}
	transform, ok := c.transforms.Lookup(source)
	if !ok {
		row := c.errorRow(h, domain.AlertErrorUnknownSource, "no transform for source "+source)
		row.Source = source
		return []domain.NewAlert{row}
	}
	alerts, err := transform(h.msg.Body())
	if err != nil {
		row := c.errorRow(h, domain.AlertErrorTransformFailed, err.Error())
		row.Source = source
		return []domain.NewAlert{row}
	}
	if len(alerts) == 0 {
		row := c.errorRow(h, domain.AlertErrorNoAlerts, "the transform found no alert in the message")
		row.Source = source
		return []domain.NewAlert{row}
	}
	rows := make([]domain.NewAlert, 0, len(alerts))
	for i, a := range alerts {
		body, err := json.Marshal(a)
		if err != nil {
			row := c.errorRow(h, domain.AlertErrorTransformFailed, err.Error())
			row.Source = source
			return []domain.NewAlert{row}
		}
		rows = append(rows, domain.NewAlert{
			Source:       source,
			Alert:        body,
			Fingerprint:  model.Fingerprint(a),
			Tier:         h.tier,
			MessageID:    c.messageKey(h),
			MessageIndex: i,
			ReceivedOn:   h.receivedOn,
		})
	}
	rows[0].RawBody, rows[0].ContentType = c.rawBody(h), h.msg.ContentType()
	return rows
}

func (c *AlertConsumer) errorRow(h *heldAlertMessage, reason, text string) domain.NewAlert {
	return domain.NewAlert{
		Tier:        h.tier,
		MessageID:   c.messageKey(h),
		RawBody:     c.rawBody(h),
		ContentType: h.msg.ContentType(),
		ErrorReason: reason,
		Error:       truncateRunes(text, maxAlertErrorText),
		ReceivedOn:  h.receivedOn,
	}
}

// messageKey is the stored message id: the queue's name and its message id, so two queues never share a key.
func (c *AlertConsumer) messageKey(h *heldAlertMessage) string {
	return h.queue.Name() + "/" + h.msg.ID()
}

// rawBody is the message body; an empty body is stored as an empty value, so the first row always has one.
func (c *AlertConsumer) rawBody(h *heldAlertMessage) []byte {
	if b := h.msg.Body(); b != nil {
		return b
	}
	return []byte{}
}

// settle stops the lock renewal, then completes, abandons or dead-letters the message; a failure is logged, and the
// queue delivers the message again once its lock lapses.
func (c *AlertConsumer) settle(ctx context.Context, h *heldAlertMessage, do func(context.Context) error, what string) {
	h.stopRenewal()
	<-h.renewalDone
	sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), alertSettleTimeout)
	defer cancel()
	if err := do(sctx); err != nil {
		slog.ErrorContext(ctx, "alert consumer: settling a message failed; it will be delivered again", "action", what, "queue", h.queue.Name(), "messageId", h.msg.ID(), "err", err)
	}
}

// isPermanentStoreError reports a database refusal of the rows themselves (a data or integrity error), which a retry
// cannot fix, as opposed to the database being unreachable or busy.
func isPermanentStoreError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return strings.HasPrefix(pgErr.Code, "22") || strings.HasPrefix(pgErr.Code, "23")
	}
	return false
}

// sleepContext waits d; false when ctx ends first.
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
