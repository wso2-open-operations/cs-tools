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
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	alertRelayInterval = time.Second
	alertRelayBatch    = 100
)

// alertProcessorRetry retries a failing event briefly; after that the event is dropped and the alert stays unlinked for a replay.
var alertProcessorRetry = eventbus.RetryPolicy{Attempts: 5, Delay: time.Second, MaxDelay: 30 * time.Second}

// startAlertProcessor runs the relay that publishes stored alerts as alert.created and the processor that folds them into incidents, and returns the function that stops both.
func startAlertProcessor(cfg *config.Config, db *pgxpool.Pool, incidents alertprocessor.Incidents) func() {
	if !cfg.AlertProcessorEnabled || !cfg.AlertConsumerEnabled() || db == nil {
		return func() {}
	}
	bus := eventbus.Config{Broker: cfg.EventHubBroker, ConnectionString: cfg.EventHubConnectionString, Topic: cfg.AlertEventHubTopic}
	scoped := repository.NewScoped(db)
	muteRules := repository.NewAlertMuteRuleRepository(scoped)

	producer := eventbus.NewProducer(bus)
	relay := &alertprocessor.Relay{Outbox: repository.NewAlertOutbox(scoped), Publisher: producer, Interval: alertRelayInterval, Batch: alertRelayBatch}

	apply := cfg.AlertingVersion == config.AlertingVersionV2
	processor := &alertprocessor.Processor{
		Store: repository.NewAlertGroups(scoped),
		Sink: alertprocessor.IncidentSink{
			Incidents:        incidents,
			Services:         repository.NewITServiceRepository(db),
			CallerID:         cfg.AlertProcessorCallerID,
			DefaultServiceID: cfg.IncidentDefaultServiceID,
		},
		MuteRules: func(ctx context.Context, now time.Time) ([]domain.AlertMuteRule, error) {
			return muteRules.Active(ctx, now)
		},
		Chain: alertprocessor.CoreChain(cfg.AlertDedupWindow),
		Apply: apply,
	}
	consumer := eventbus.NewConsumer(bus, cfg.AlertProcessorConsumerGroup, alertProcessorRetry)

	// The system identity, because alerts and incidents created from them are internal and there is no caller.
	ctx, cancel := context.WithCancel(repository.WithSystemIdentity(context.Background()))
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		relay.Run(ctx)
	}()
	go func() {
		defer wg.Done()
		consumer.Run(ctx, func(ctx context.Context, record eventbus.Record) error {
			event, ok, err := alertprocessor.Decode(record.Value)
			if err != nil {
				// A record that can never be decoded is not retried.
				slog.ErrorContext(ctx, "alert processor: undecodable record skipped", "offset", record.Offset, "err", err)
				return nil
			}
			if !ok {
				return nil
			}
			return processor.Handle(ctx, event)
		}, nil)
	}()
	slog.Info("alert processor running", "topic", bus.Topic, "group", cfg.AlertProcessorConsumerGroup,
		"dedupWindow", cfg.AlertDedupWindow, "createsIncidents", apply)

	return func() {
		cancel()
		consumer.Close()
		wg.Wait()
		producer.Close()
	}
}
