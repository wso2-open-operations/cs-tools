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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/relay"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/sink"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/strategy"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/subscriber"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The adapters below must satisfy the processor's ports.
var (
	_ alertprocessor.Store = (*repository.AlertGroups)(nil)
	_ alertprocessor.Sink  = sink.Incident{}
	_ relay.Outbox         = (*repository.AlertOutbox)(nil)
	_ relay.Publisher      = (*eventbus.Producer)(nil)
	_ subscriber.Source    = (*eventbus.Consumer)(nil)
)

const (
	alertRelayInterval = time.Second
	alertRelayBatch    = 100
)

// alertProcessorRetry retries a failing event briefly; after that the event is dropped and the alert stays without an incident for a replay.
var alertProcessorRetry = eventbus.RetryPolicy{Attempts: 5, Delay: time.Second, MaxDelay: 30 * time.Second}

// startAlertProcessor runs the relay that publishes stored alerts as alert.created and the subscriber that hands them to the processor, and returns the function that stops both.
func startAlertProcessor(cfg *config.Config, db *pgxpool.Pool, incidents sink.Incidents) func() {
	if !cfg.AlertProcessorEnabled || !cfg.AlertConsumerEnabled() || db == nil {
		return func() {}
	}
	bus := eventbus.Config{Broker: cfg.EventHubBroker, ConnectionString: cfg.EventHubConnectionString, Topic: cfg.AlertEventHubTopic}
	scoped := repository.NewScoped(db)
	apply := cfg.AlertingVersion == config.AlertingVersionV2

	producer := eventbus.NewProducer(bus)
	publisher := &relay.Relay{Outbox: repository.NewAlertOutbox(scoped), Publisher: producer, Interval: alertRelayInterval, Batch: alertRelayBatch}

	muteRules := repository.NewAlertMuteRuleRepository(scoped)
	processor := &alertprocessor.Processor{
		Store: repository.NewAlertGroups(scoped),
		Sink: sink.Incident{
			Incidents:        incidents,
			Services:         repository.NewITServiceRepository(db),
			CallerID:         cfg.AlertProcessorCallerID,
			DefaultServiceID: cfg.IncidentDefaultServiceID,
		},
		MuteRules: func(ctx context.Context, now time.Time) ([]domain.AlertMuteRule, error) {
			return muteRules.Active(ctx, now)
		},
		// RIA and other ways of grouping join with Chain.Before or Chain.After.
		Chain: strategy.Core(cfg.AlertDedupWindow),
		Apply: apply,
	}
	consumer := eventbus.NewConsumer(bus, cfg.AlertProcessorConsumerGroup, alertProcessorRetry)
	listener := subscriber.New(consumer, processor.Handle, subscriber.Log(), subscriber.Recover())

	// The system identity, because alerts and incidents created from them are internal and there is no caller.
	ctx, cancel := context.WithCancel(repository.WithSystemIdentity(context.Background()))
	var wg sync.WaitGroup
	for _, run := range []func(context.Context){publisher.Run, listener.Run} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			run(ctx)
		}()
	}
	slog.Info("alert processor running", "topic", bus.Topic, "group", cfg.AlertProcessorConsumerGroup,
		"dedupWindow", cfg.AlertDedupWindow, "createsIncidents", apply)

	return func() {
		cancel()
		consumer.Close()
		wg.Wait()
		producer.Close()
	}
}
