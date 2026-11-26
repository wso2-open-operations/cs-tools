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

package eventbus

import (
	"context"
	"crypto/tls"
	"fmt"

	kafka "github.com/segmentio/kafka-go"
)

// Producer publishes records to a single topic.
type Producer struct {
	writer *kafka.Writer
}

// NewProducer constructs a Producer. Connecting is lazy — the underlying
// writer dials brokers on first use, not here — so a wrong
// Broker/ConnectionString only surfaces as an error from the first Publish
// call, not from this constructor.
func NewProducer(cfg Config) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:  kafka.TCP(cfg.Broker),
			Topic: cfg.Topic,
			// Every record for the same key (entity ID) must land in the
			// same partition and be written in the order Publish was
			// called, matching csm-notification-service's consumer-side
			// ordering guarantee (see its eventbus.Producer).
			Balancer: &kafka.Hash{},
			// Wait for the full ISR to acknowledge before Publish returns.
			RequiredAcks: kafka.RequireAll,
			Transport: &kafka.Transport{
				TLS:  &tls.Config{MinVersion: tls.VersionTLS12},
				SASL: cfg.saslMechanism(),
			},
			Logger:       kafka.LoggerFunc(logDebug),
			ErrorLogger:  kafka.LoggerFunc(logError),
			BatchTimeout: cfg.BatchTimeout,
			// Compression is deliberately left unset (no codec) — Azure
			// Event Hub's Kafka-compatible endpoint rejected compressed
			// batches under the client csm-notification-service used
			// before kafka-go (confirmed against the real namespace; see
			// that service's CLAUDE.md).
		},
	}
}

// Publish sends value as a single record, keyed by key, and waits for the
// broker's acknowledgment before returning. key determines the partition —
// pass the same key (e.g. an entity ID) for every event that must stay
// ordered relative to each other.
func (p *Producer) Publish(ctx context.Context, key, value []byte) error {
	if err := p.writer.WriteMessages(ctx, kafka.Message{Key: key, Value: value}); err != nil {
		return fmt.Errorf("eventbus: publish: %w", err)
	}
	return nil
}

// Close releases the underlying connection. Safe to call once during
// shutdown.
func (p *Producer) Close() {
	_ = p.writer.Close()
}
