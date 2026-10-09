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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
)

// recordFailureTimeout bounds the failure-recording call in Publish below —
// it runs on a context.WithoutCancel copy of the caller's ctx, so without its
// own bound it could hang indefinitely on a slow database, now that it's no
// longer tied to the caller's own deadline.
const recordFailureTimeout = 10 * time.Second

// publishTimeout bounds one Event Hub write in Publish. Like the failure
// record, the write runs on a context.WithoutCancel copy of the caller's ctx
// (see Publish), so it needs a bound of its own.
const publishTimeout = 15 * time.Second

// notifyTimeout bounds a whole post-commit notification: the reads that
// enrich an event (case view, recipients) plus its publish.
const notifyTimeout = 30 * time.Second

// detachedNotifyContext returns the context for notification work that runs
// after a write has committed.
//
// THE REQUEST'S OWN CONTEXT IS THE WRONG ONE FOR THIS. It is cancelled when
// the caller disconnects or REQUEST_TIMEOUT (60s) passes -- and by then the
// write it announces has already committed, so cancelling the notification
// loses it for good: the record changed and nobody was told. On staging that
// was dozens of case.comment_added / case.created / sr.comment_added events
// failing with "context canceled" or "context deadline exceeded".
//
// WithoutCancel keeps ctx's values (caller identity, correlation id), which
// the enrichment reads need, and drops only its cancellation; notifyTimeout
// stands in for the deadline it drops.
func detachedNotifyContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), notifyTimeout)
}

// kafkaProducer abstracts eventbus.Producer for testability.
type kafkaProducer interface {
	Publish(ctx context.Context, key, value []byte) error
}

// eventPublisherService implements EventPublisherService.
type eventPublisherService struct {
	kafka    kafkaProducer
	failures EventPublishFailureService
}

// NewEventPublisherService constructs an EventPublisherService. Unlike
// apps/csm-portal/backend's own internal/eventpublisher.Publisher — which
// has to record a failed publish via an HTTP call to this service's
// POST /event-publish-failures — failures is called in-process here, since
// this service is the one that already owns that table. failures may be
// nil when no Postgres pool is available; Publish then skips recording.
func NewEventPublisherService(kafka kafkaProducer, failures EventPublishFailureService) EventPublisherService {
	return &eventPublisherService{kafka: kafka, failures: failures}
}

// Publish implements EventPublisherService.
//
// KNOWN GAP: a publish error does not prove Event Hub rejected the record —
// the write can still land while only the acknowledgement is lost (a network
// blip after the broker appended it). Manually republishing from the
// recorded failure in that case would duplicate the event, since neither the
// envelope nor the failure record carries a stable event ID a consumer could
// dedupe on. Closing this needs an event ID threaded through the envelope,
// the failure record, and a durable dedupe check on the consumer side — a
// real design addition, not a quick fix, so it's flagged here rather than
// built speculatively.
func (s *eventPublisherService) Publish(ctx context.Context, eventType events.Type, entityID string, payload json.RawMessage) error {
	body, err := json.Marshal(events.Envelope{Type: eventType, EntityID: entityID, Payload: payload})
	if err != nil {
		return fmt.Errorf("eventpublisher: encode envelope: %w", err)
	}

	// Detached for the same reason as detachedNotifyContext: every caller
	// publishes after its write has committed, so the caller going away must
	// not take the event with it. Done here as well as at the call sites so no
	// event type can lose its publish to a cancelled request again.
	pubCtx, cancelPub := context.WithTimeout(context.WithoutCancel(ctx), publishTimeout)
	defer cancelPub()
	pubErr := s.kafka.Publish(pubCtx, []byte(entityID), body)
	if pubErr == nil {
		return nil
	}

	// failures is nil when no database is configured, which is legal when
	// DATA_SOURCE=servicenow (see config.Config.HasDatabase). There is nowhere
	// durable to record the failure in that case, so log loudly and return the
	// original publish error — the alternative, dereferencing a nil service,
	// would turn a recoverable publish failure into a panic.
	if s.failures == nil {
		slog.ErrorContext(ctx, "eventpublisher: publish failed and no database is configured to record it", "eventType", eventType, "entityId", entityID, "publishErr", pubErr)
		return fmt.Errorf("eventpublisher: publish %s for entity %s: %w", eventType, entityID, pubErr)
	}

	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordFailureTimeout)
	defer cancel()
	if _, recErr := s.failures.CreateEventPublishFailure(recordCtx, domain.CreateEventPublishFailureRequest{
		EventType: string(eventType),
		EntityID:  entityID,
		Payload:   payload,
		Error:     pubErr.Error(),
	}); recErr != nil {
		slog.ErrorContext(ctx, "eventpublisher: publish failed and recording the failure also failed", "eventType", eventType, "entityId", entityID, "publishErr", pubErr, "recordErr", recErr)
	}

	return fmt.Errorf("eventpublisher: publish %s for entity %s: %w", eventType, entityID, pubErr)
}

// Close implements EventPublisherService.
func (s *eventPublisherService) Close() {
	if closer, ok := s.kafka.(interface{ Close() }); ok {
		closer.Close()
	}
}
