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
	"encoding/json"
	"log"
	"log/slog"
	"sync"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// startSalesforceWriteResultConsumers runs the two consumers of
// sales-entity-service's answers to the deferred Salesforce membership writes
// (service.DeferredSalesforceWriteCompleter) and returns the function that
// stops them.
//
// The Salesforce retry policy lives in sales-entity-service, which owns the
// Salesforce connection. What a RESULT's handling can wait on is narrower:
// Sales Entity's REST API (to read the records back) and this database. The
// main topic's consumer retries a few seconds then dead-letters, so a stuck
// result cannot hold later results on the partition; the dead-letter topic's
// consumer retries with exponential backoff, and on its final attempt the
// request is closed as FAILED naming the Salesforce ids it could not stamp
// (the Salesforce echo through Service Bus stamps them anyway when it
// arrives), so nothing sits PENDING with nothing behind it.
func startSalesforceWriteResultConsumers(cfg *config.Config, completer service.DeferredSalesforceWriteCompleter) func() {
	mainCfg := eventbus.Config{
		Broker:           cfg.EventHubBroker,
		ConnectionString: cfg.EventHubConnectionString,
		Topic:            cfg.SalesforceWriteResultsTopic,
	}
	dlqCfg := mainCfg
	dlqCfg.Topic = cfg.SalesforceWriteResultsDLQTopic

	mainPolicy := eventbus.RetryPolicy{Attempts: cfg.SalesforceWriteResultsRetryAttempts, Delay: cfg.SalesforceWriteResultsRetryDelay}
	dlqPolicy := eventbus.RetryPolicy{Attempts: cfg.SalesforceWriteResultsDLQRetryAttempts, Delay: cfg.SalesforceWriteResultsDLQRetryDelay, MaxDelay: cfg.SalesforceWriteResultsDLQRetryMaxDelay}

	dlqProducer := eventbus.NewProducer(dlqCfg)
	mainConsumer := eventbus.NewConsumer(mainCfg, cfg.SalesforceWriteResultsConsumerGroup, mainPolicy)
	dlqConsumer := eventbus.NewConsumer(dlqCfg, cfg.SalesforceWriteResultsDLQConsumerGroup, dlqPolicy)
	handle := salesforceWriteResultHandle(completer)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		mainConsumer.Run(ctx, handle, func(ctx context.Context, record eventbus.Record, handleErr error) error {
			slog.WarnContext(ctx, "salesforce write results: main-topic attempts exhausted; dead-lettering",
				"topic", record.Topic, "offset", record.Offset, "dlqTopic", dlqCfg.Topic, "err", handleErr)
			return dlqProducer.Publish(ctx, record.Key, record.Value)
		})
	}()
	go func() {
		defer wg.Done()
		dlqConsumer.Run(ctx, handle, nil)
	}()
	log.Printf("salesforce deferred writes enabled: publishing commands to %s; consuming results from %s (group %s, %d attempts, %s in-place cover) and %s (group %s, %d attempts, %s in-place cover)",
		cfg.SalesforceWriteEventHubTopic,
		mainCfg.Topic, cfg.SalesforceWriteResultsConsumerGroup, mainConsumer.Policy().Attempts, mainConsumer.Policy().TotalWait(),
		dlqCfg.Topic, cfg.SalesforceWriteResultsDLQConsumerGroup, dlqConsumer.Policy().Attempts, dlqConsumer.Policy().TotalWait())

	return func() {
		cancel()
		mainConsumer.Close()
		dlqConsumer.Close()
		wg.Wait()
		dlqProducer.Close()
	}
}

// salesforceWriteResultHandle adapts a result record to the service call. A
// record this consumer cannot act on (another type, a malformed body, no
// request id) is logged and acknowledged: there is nothing to retry and
// nothing to dead-letter.
func salesforceWriteResultHandle(completer service.DeferredSalesforceWriteCompleter) eventbus.Handle {
	return func(ctx context.Context, record eventbus.Record) error {
		var env events.Envelope
		if err := json.Unmarshal(record.Value, &env); err != nil {
			slog.ErrorContext(ctx, "salesforce write results: malformed envelope; skipping", "topic", record.Topic, "offset", record.Offset, "err", err)
			return nil
		}
		switch env.Type {
		case events.TypeSalesforceMembershipWriteCompleted:
			var p events.SalesforceMembershipWriteCompletedPayload
			if err := json.Unmarshal(env.Payload, &p); err != nil {
				slog.ErrorContext(ctx, "salesforce write results: malformed completed payload; skipping", "offset", record.Offset, "err", err)
				return nil
			}
			if p.RequestID == "" {
				p.RequestID = env.EntityID
			}
			if p.RequestID == "" {
				slog.ErrorContext(ctx, "salesforce write results: completed result carries no request id; skipping", "offset", record.Offset)
				return nil
			}
			return completer.CompleteDeferredSalesforceWrite(ctx, p, record.NoMoreRetries)
		case events.TypeSalesforceMembershipWriteFailed:
			var p events.SalesforceMembershipWriteFailedPayload
			if err := json.Unmarshal(env.Payload, &p); err != nil {
				slog.ErrorContext(ctx, "salesforce write results: malformed failed payload; skipping", "offset", record.Offset, "err", err)
				return nil
			}
			if p.RequestID == "" {
				p.RequestID = env.EntityID
			}
			if p.RequestID == "" {
				slog.ErrorContext(ctx, "salesforce write results: failed result carries no request id; skipping", "offset", record.Offset)
				return nil
			}
			return completer.FailDeferredSalesforceWrite(ctx, p)
		default:
			slog.WarnContext(ctx, "salesforce write results: unexpected event type on the results topic; skipping", "type", env.Type, "offset", record.Offset)
			return nil
		}
	}
}
