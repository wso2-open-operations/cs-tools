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
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertqueue/servicebus"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertqueue/sqs"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource"
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

// startAlertPipeline runs the SRE alert pipeline: the queue consumer (Service Bus and SQS to the alert table), the
// alert relay (the alert table as the outbox, publishing alert.created), the alert_stored listener that wakes it, and
// ALERT_PROCESSOR_CONCURRENCY alert.created consumers in one group, each deciding the alerts of its partitions. It
// returns the function that stops them in that order and waits for the alerts in flight. A configuration that cannot
// start (a source config that does not parse, a queue that cannot be opened) refuses to start the service.
func startAlertPipeline(cfg *config.Config, db *pgxpool.Pool, incidents service.AlertIncidentCreator) func() {
	transforms, err := alertsource.New()
	if err != nil {
		panic(fmt.Sprintf("alert pipeline: loading the alert source configs: %v", err))
	}
	priority, standard, closeQueues, err := openAlertQueues(cfg)
	if err != nil {
		panic(fmt.Sprintf("alert pipeline: %v", err))
	}
	alertConsumer := service.NewAlertConsumer(repository.NewAlertRepository(repository.NewScoped(db)), transforms, priority, standard,
		service.AlertConsumerConfig{Concurrency: cfg.AlertConsumerConcurrency})

	topicCfg := eventbus.Config{
		Broker:           cfg.EventHubBroker,
		ConnectionString: cfg.EventHubConnectionString,
		Topic:            cfg.AlertEventHubTopic,
		BatchTimeout:     eventbus.PromptBatchTimeout,
	}
	producer := eventbus.NewProducer(topicCfg)
	relay := service.NewAlertRelay(repository.NewAlertRelayRepository(repository.NewScoped(db)), producer, service.AlertRelayConfig{
		MaxInFlight:  cfg.AlertRelayMaxInFlight,
		PollInterval: cfg.AlertRelayPollInterval,
	})
	processor := service.NewAlertProcessor(
		repository.NewAlertProcessorRepository(repository.NewScoped(db)),
		incidents,
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

	// alert is internal only: the consumer, the relay and the processor read and write it as the system.
	consumerCtx, stopConsumer := context.WithCancel(repository.WithSystemIdentity(context.Background()))
	var consumerWG sync.WaitGroup
	consumerWG.Go(func() { alertConsumer.Run(consumerCtx) })
	slog.Info("alert consumer running", "priorityQueues", len(priority), "standardQueues", len(standard), "workers", cfg.AlertConsumerConcurrency)

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
		// The consumer first: a message in flight is stored and completed, the rest are given back to the queues.
		stopConsumer()
		consumerWG.Wait()
		closeQueues()
		cancel()
		for _, consumer := range consumers {
			consumer.Close()
		}
		wg.Wait()
		producer.Close()
	}
}

// openAlertQueues opens every configured Service Bus and SQS queue, by tier, and returns the function that closes
// them. Nothing connects yet: Service Bus opens its connection on the first receive, and SQS has none.
func openAlertQueues(cfg *config.Config) (priority, standard []service.AlertQueue, closeAll func(), err error) {
	var closers []func(context.Context) error
	closeAll = func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for i := len(closers) - 1; i >= 0; i-- {
			if err := closers[i](ctx); err != nil {
				slog.Warn("alert pipeline: closing a queue failed", "err", err)
			}
		}
	}
	if cfg.HasAlertServiceBus() {
		client, err := servicebus.NewClient(cfg.AlertServiceBusConnectionString)
		if err != nil {
			return nil, nil, closeAll, err
		}
		closers = append(closers, client.Close)
		for _, tier := range []struct {
			names []string
			out   *[]service.AlertQueue
		}{{cfg.AlertServiceBusPriorityQueues, &priority}, {cfg.AlertServiceBusStandardQueues, &standard}} {
			for _, name := range tier.names {
				q, err := client.Queue(name)
				if err != nil {
					closeAll()
					return nil, nil, func() {}, err
				}
				closers = append(closers, q.Close)
				*tier.out = append(*tier.out, q)
			}
		}
	}
	if cfg.HasAlertSQS() {
		client, err := sqs.NewClient(context.Background(), sqs.Config{
			Region:          cfg.AlertSQSResolvedRegion(),
			AccessKeyID:     cfg.AlertSQSAccessKeyID,
			SecretAccessKey: cfg.AlertSQSSecretAccessKey,
		})
		if err != nil {
			closeAll()
			return nil, nil, func() {}, err
		}
		for _, u := range cfg.AlertSQSPriorityQueueURLs {
			priority = append(priority, sqs.NewQueue(client, u, cfg.AlertSQSDeadLetterQueueURL))
		}
		for _, u := range cfg.AlertSQSStandardQueueURLs {
			standard = append(standard, sqs.NewQueue(client, u, cfg.AlertSQSDeadLetterQueueURL))
		}
	}
	return priority, standard, closeAll, nil
}

// alertProcessorOwner names this replica on the alerts it claims, so a stuck claim can be traced to its pod.
func alertProcessorOwner() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "entity-service"
	}
	return fmt.Sprintf("%s/%d", host, os.Getpid())
}
