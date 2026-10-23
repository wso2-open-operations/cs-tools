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

// startSalesforceWriteConsumers runs the two consumers of the deferred
// Salesforce membership write (service.DeferredSalesforceWriter) and returns
// the function that stops them.
//
// TWO TIERS, TWO POLICIES. The main topic's consumer retries a record a few
// times a couple of seconds apart (SALESFORCE_WRITE_RETRY_*), then
// republishes it to the dead-letter topic and commits: a Salesforce outage
// must not hold the main partition for long, since every later invitation
// made during the outage lands behind it. The dead-letter topic's consumer
// retries the same record for roughly an hour with exponential backoff
// (SALESFORCE_WRITE_DLQ_RETRY_*), because waiting is that topic's whole job.
// A record that outlives both tiers is dropped by Event Hub, and the
// handler's final call (Record.NoMoreRetries) has already marked the
// salesforce_write_request FAILED so the portal shows it. This is what
// "short outages are covered by the event path" means in practice: the
// cover is the sum of the two tiers' waits, logged at startup.
func startSalesforceWriteConsumers(cfg *config.Config, writer service.DeferredSalesforceWriter) func() {
	mainCfg := eventbus.Config{
		Broker:           cfg.EventHubBroker,
		ConnectionString: cfg.EventHubConnectionString,
		Topic:            cfg.SalesforceWriteEventHubTopic,
	}
	dlqCfg := mainCfg
	dlqCfg.Topic = cfg.SalesforceWriteEventHubDLQTopic

	mainPolicy := eventbus.RetryPolicy{Attempts: cfg.SalesforceWriteRetryAttempts, Delay: cfg.SalesforceWriteRetryDelay}
	dlqPolicy := eventbus.RetryPolicy{Attempts: cfg.SalesforceWriteDLQRetryAttempts, Delay: cfg.SalesforceWriteDLQRetryDelay, MaxDelay: cfg.SalesforceWriteDLQRetryMaxDelay}

	dlqProducer := eventbus.NewProducer(dlqCfg)
	mainConsumer := eventbus.NewConsumer(mainCfg, cfg.SalesforceWriteConsumerGroup, mainPolicy)
	dlqConsumer := eventbus.NewConsumer(dlqCfg, cfg.SalesforceWriteDLQConsumerGroup, dlqPolicy)
	handle := salesforceWriteHandle(writer)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		mainConsumer.Run(ctx, handle, func(ctx context.Context, record eventbus.Record, handleErr error) error {
			slog.WarnContext(ctx, "salesforce deferred write: main-topic attempts exhausted; dead-lettering",
				"topic", record.Topic, "offset", record.Offset, "dlqTopic", dlqCfg.Topic, "err", handleErr)
			return dlqProducer.Publish(ctx, record.Key, record.Value)
		})
	}()
	go func() {
		defer wg.Done()
		dlqConsumer.Run(ctx, handle, nil)
	}()
	log.Printf("salesforce deferred writes enabled: consuming %s (group %s, %d attempts, %s in-place cover) and %s (group %s, %d attempts, %s in-place cover)",
		mainCfg.Topic, cfg.SalesforceWriteConsumerGroup, mainConsumer.Policy().Attempts, mainConsumer.Policy().TotalWait(),
		dlqCfg.Topic, cfg.SalesforceWriteDLQConsumerGroup, dlqConsumer.Policy().Attempts, dlqConsumer.Policy().TotalWait())

	return func() {
		cancel()
		mainConsumer.Close()
		dlqConsumer.Close()
		wg.Wait()
		dlqProducer.Close()
	}
}

// salesforceWriteHandle adapts the eventbus record to the service call. A
// record this service did not publish (another type, a malformed body, no
// request id) is logged and acknowledged: there is nothing to retry and
// nothing to dead-letter.
func salesforceWriteHandle(writer service.DeferredSalesforceWriter) eventbus.Handle {
	return func(ctx context.Context, record eventbus.Record) error {
		var env events.Envelope
		if err := json.Unmarshal(record.Value, &env); err != nil {
			slog.ErrorContext(ctx, "salesforce deferred write: malformed envelope; skipping", "topic", record.Topic, "offset", record.Offset, "err", err)
			return nil
		}
		if env.Type != events.TypeSalesforceMembershipWriteRequested {
			slog.WarnContext(ctx, "salesforce deferred write: unexpected event type on the write topic; skipping", "type", env.Type, "offset", record.Offset)
			return nil
		}
		var payload events.SalesforceMembershipWriteRequestedPayload
		if len(env.Payload) > 0 {
			if err := json.Unmarshal(env.Payload, &payload); err != nil {
				slog.ErrorContext(ctx, "salesforce deferred write: malformed payload; skipping", "offset", record.Offset, "err", err)
				return nil
			}
		}
		requestID := payload.RequestID
		if requestID == "" {
			requestID = env.EntityID
		}
		if requestID == "" {
			slog.ErrorContext(ctx, "salesforce deferred write: event carries no request id; skipping", "offset", record.Offset)
			return nil
		}
		return writer.ProcessDeferredSalesforceWrite(ctx, requestID, record.NoMoreRetries)
	}
}
