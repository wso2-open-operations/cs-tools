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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// alertTopicRetryPolicy is the alert.created consumers' in-place retry. The processor records a failed decision itself
// and the alert goes back to the outbox, so a handler error here means only that the alert could not even be claimed;
// a short retry, then the record is dropped and the relay's stale sweep publishes the alert again. Long retries would
// hold every later alert on the partition.
var alertTopicRetryPolicy = eventbus.RetryPolicy{Attempts: 3, Delay: time.Second}

// alertProducerBatchTimeout lets the relay's lone records go at once instead of after kafka-go's one-second default.
const alertProducerBatchTimeout = 5 * time.Millisecond

// startAlertPipeline runs the alert relay (the alert table as the outbox, publishing alert.created), the alert_stored
// listener that wakes it, and ALERT_PROCESSOR_CONCURRENCY alert.created consumers in one group, each deciding the
// alerts of its partitions. It returns the function that stops them and waits for the alerts in flight.
func startAlertPipeline(cfg *config.Config, db *pgxpool.Pool, incidents service.AlertIncidentCreator) func() {
	topicCfg := eventbus.Config{
		Broker:           cfg.EventHubBroker,
		ConnectionString: cfg.EventHubConnectionString,
		Topic:            cfg.AlertEventHubTopic,
		BatchTimeout:     alertProducerBatchTimeout,
	}
	producer := eventbus.NewProducer(topicCfg)
	relay := service.NewAlertRelay(repository.NewAlertRelayRepository(repository.NewScoped(db)), producer, service.AlertRelayConfig{
		MaxInFlight:  cfg.AlertRelayMaxInFlight,
		PollInterval: cfg.AlertRelayPollInterval,
	})
	processor := service.NewAlertProcessor(
		repository.NewAlertProcessorRepository(repository.NewScoped(db)),
		incidents,
		repository.NewAlertMuteRuleRepository(repository.NewScoped(db)),
		service.AlertProcessorConfig{
			Live:             cfg.AlertProcessorLive(),
			CallerID:         cfg.AlertProcessorCallerID,
			DefaultServiceID: cfg.IncidentDefaultServiceID,
			MaxIncidentAge:   cfg.AlertProcessorMaxIncidentAge,
			Owner:            alertProcessorOwner(),
			// A decision frees room under ALERT_RELAY_MAX_IN_FLIGHT, so the relay looks again at once.
			Decided: relay.Wake,
		},
	)
	consumers := make([]*eventbus.Consumer, cfg.AlertProcessorConcurrency)
	for i := range consumers {
		consumers[i] = eventbus.NewConsumer(topicCfg, cfg.AlertProcessorConsumerGroup, alertTopicRetryPolicy)
	}

	// alert is internal only: the relay and the processor read and write it as the system.
	ctx, cancel := context.WithCancel(repository.WithSystemIdentity(context.Background()))
	var wg sync.WaitGroup
	wg.Go(func() { relay.Run(ctx) })
	// The consumer notifies alert_stored when it commits a NEW alert; the listener wakes the relay at once.
	// ALERT_RELAY_POLL_INTERVAL is the backstop for a missed notification and for retries falling due.
	wg.Go(func() { repository.ListenAlertStored(ctx, db, relay.Wake) })
	for _, consumer := range consumers {
		wg.Go(func() {
			consumer.Run(ctx, func(ctx context.Context, record eventbus.Record) error {
				return processor.HandleEvent(ctx, record.Value)
			}, nil)
		})
	}
	slog.Info("alert relay and processor running", "live", cfg.AlertProcessorLive(), "alertingVersion", cfg.AlertingVersion,
		"topic", topicCfg.Topic, "consumerGroup", cfg.AlertProcessorConsumerGroup, "consumers", len(consumers),
		"maxInFlight", cfg.AlertRelayMaxInFlight, "backstopPoll", cfg.AlertRelayPollInterval, "maxIncidentAge", cfg.AlertProcessorMaxIncidentAge)

	return func() {
		cancel()
		for _, consumer := range consumers {
			consumer.Close()
		}
		wg.Wait()
		producer.Close()
	}
}
