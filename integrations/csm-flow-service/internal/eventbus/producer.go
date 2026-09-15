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

// Producer publishes records to a single topic. The flow engine uses it for
// two things: dead-lettering a record whose flows exhausted their retries,
// and (later) the timer sweeper publishing timer.fired back under the same
// partition key.
type Producer struct {
	writer *kafka.Writer
}

// NewProducer constructs a Producer. Connecting is lazy — the underlying
// writer dials brokers on first use — so a wrong Broker/ConnectionString only
// surfaces as an error from the first Publish call.
func NewProducer(cfg Config) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:  kafka.TCP(cfg.Broker),
			Topic: cfg.Topic,
			// Every record for the same key (entity ID) must land in the same
			// partition and be written in publish order, so all work for one
			// entity stays serialized. Hash maps a key deterministically to a
			// partition.
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll,
			Transport: &kafka.Transport{
				TLS:  &tls.Config{MinVersion: tls.VersionTLS12},
				SASL: cfg.saslMechanism(),
			},
			Logger:      kafka.LoggerFunc(logDebug),
			ErrorLogger: kafka.LoggerFunc(logError),
			// Compression is deliberately left unset. Azure Event Hub's
			// Kafka-compatible endpoint rejects compressed batches with
			// "UNSUPPORTED_FOR_MESSAGE_FORMAT" — confirmed against the real
			// namespace, not a guess.
		},
	}
}

// Publish sends value as a single record, keyed by key, and waits for the
// broker's acknowledgment before returning. Pass the same key (an entity ID)
// for every event that must stay ordered relative to each other.
func (p *Producer) Publish(ctx context.Context, key, value []byte) error {
	if err := p.writer.WriteMessages(ctx, kafka.Message{Key: key, Value: value}); err != nil {
		return fmt.Errorf("eventbus: publish: %w", err)
	}
	return nil
}

// Close releases the underlying connection. Safe to call once during shutdown.
func (p *Producer) Close() {
	_ = p.writer.Close()
}
