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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
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
	consumer := eventbus.NewConsumer(bus, cfg.AlertProcessorConsumerGroup, alertProcessorRetry)
	listener := subscriber.New(consumer, processor.Handle, subscriber.Log(), subscriber.Recover())
	listener.OnExhausted = func(ctx context.Context, e event.Event, cause error) error {
		sent, err := republisher.Republish(ctx, e)
		if err == nil && !sent {
			slog.ErrorContext(ctx, "alert processor: gave up on an alert after its attempts; it has no incident", "alertId", e.AlertID, "attempts", publisher.MaxAttempts, "err", cause)
		}
		return err
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
