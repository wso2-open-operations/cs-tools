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

package server

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/incident"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/publisher"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/strategy"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/subscriber"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// The adapters below must satisfy the processor's ports.
var (
	_ alertprocessor.Store          = (*repository.AlertGroups)(nil)
	_ alertprocessor.IncidentWriter = incident.Writer{}
	_ incident.Notes                = (*repository.IncidentNotes)(nil)
	_ publisher.Producer            = (*eventbus.Producer)(nil)
	_ subscriber.Source             = (*eventbus.Consumer)(nil)
)

// alertProcessorRetry retries a failing event briefly; after that the event is published again (at most publisher.MaxAttempts times).
var alertProcessorRetry = eventbus.RetryPolicy{Attempts: 5, Delay: time.Second, MaxDelay: 30 * time.Second}

// startAlertProcessor runs the subscriber that hands alert.created events to the processor, and returns the function that stops it. The consumer publishes the events (cmd/api/main.go).
func startAlertProcessor(cfg *config.Config, db *pgxpool.Pool, incidents incident.Service) func() {
	if !cfg.AlertProcessorEnabled || !cfg.AlertConsumerEnabled() || db == nil {
		return func() {}
	}
	bus := eventbus.Config{Broker: cfg.EventHubBroker, ConnectionString: cfg.EventHubConnectionString, Topic: cfg.AlertEventHubTopic}
	scoped := repository.NewScoped(db)
	apply := cfg.AlertingVersion == config.AlertingVersionV2

	producer := eventbus.NewProducer(bus)
	republisher := publisher.Publisher{Producer: producer}

	processor := &alertprocessor.Processor{
		Store: repository.NewAlertGroups(scoped),
		Incidents: incident.Writer{
			Service:          incidents,
			Services:         repository.NewITServiceRepository(db),
			Notes:            repository.NewIncidentNotes(scoped),
			CallerID:         cfg.AlertProcessorCallerID,
			DefaultServiceID: cfg.IncidentDefaultServiceID,
		},
		// RIA and other ways of grouping join with Chain.Before or Chain.After.
		Chain:    strategy.Default(cfg.AlertDedupWindow),
		Lookback: cfg.AlertDedupWindow,
		Apply:    apply,
	}
	consumer := eventbus.NewConsumer(subscriberBus(bus, apply), consumerGroup(cfg.AlertProcessorConsumerGroup, apply), alertProcessorRetry)
	listener := subscriber.New(consumer, processor.Handle, subscriber.Log(), subscriber.Recover())
	failures := service.NewEventPublishFailureService(repository.NewEventPublishFailureRepository(db))
	listener.OnExhausted = func(ctx context.Context, e event.Event, cause error) error {
		sent, err := republisher.Republish(ctx, e)
		if err == nil && sent {
			return nil
		}
		// Not republished: record the event before its offset is committed, so it can be replayed from event_publish_failures.
		reason := fmt.Sprintf("gave up after %d attempts: %v", publisher.MaxAttempts, cause)
		if err != nil {
			reason = fmt.Sprintf("could not publish again: %v (processing failed: %v)", err, cause)
		}
		return recordAlertFailure(ctx, failures, e, reason)
	}

	// The system identity, because alerts and incidents created from them are internal and there is no caller.
	ctx, cancel := context.WithCancel(repository.WithSystemIdentity(context.Background()))
	done := make(chan struct{})
	go func() {
		defer close(done)
		listener.Run(ctx)
	}()
	slog.Info("alert processor running", "topic", bus.Topic, "group", cfg.AlertProcessorConsumerGroup,
		"dedupWindow", cfg.AlertDedupWindow, "createsIncidents", apply)

	return func() {
		cancel()
		consumer.Close()
		<-done
		producer.Close()
	}
}

// recordAlertFailure keeps an event the processor could not hand back to the event hub in event_publish_failures, the durable list operators replay from.
func recordAlertFailure(ctx context.Context, failures service.EventPublishFailureService, e event.Event, reason string) error {
	_, value, err := event.Encode(e)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if _, err := failures.CreateEventPublishFailure(ctx, domain.CreateEventPublishFailureRequest{
		EventType: string(event.Type), EntityID: e.AlertID, Payload: value, Error: reason,
	}); err != nil {
		slog.ErrorContext(ctx, "alert processor: could not record an alert it gave up on; it has no incident", "alertId", e.AlertID, "reason", reason, "err", err)
		return err
	}
	slog.ErrorContext(ctx, "alert processor: gave up on an alert; recorded in event_publish_failures for replay", "alertId", e.AlertID, "reason", reason)
	return nil
}

// consumerGroup keeps hybrid and v2 replicas in separate groups: a hybrid replica only logs, so if it shared v2's group it would commit offsets for alerts v2 never processed. During a rolling switch each mode reads every event.
func consumerGroup(group string, apply bool) string {
	if apply {
		return group
	}
	return group + "-hybrid"
}

// subscriberBus starts the v2 group at the newest event the first time it runs, so the switch to v2 does not replay the alerts v1 already handled during hybrid.
func subscriberBus(bus eventbus.Config, apply bool) eventbus.Config {
	bus.StartAtLatest = apply
	return bus
}
