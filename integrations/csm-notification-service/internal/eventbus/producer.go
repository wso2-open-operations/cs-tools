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
	"time"

	kafka "github.com/segmentio/kafka-go"
)

// publishBatchSize/publishBatchTimeout make every Publish flush at once;
// see NewProducer.
const (
	publishBatchSize    = 1
	publishBatchTimeout = 10 * time.Millisecond
)

// Producer publishes records to a single topic.
type Producer struct {
	writer *kafka.Writer
}

// Message is one record to publish: Key picks the partition (see Publish),
// Value is the record body, and Headers are optional record headers (e.g.
// HeaderNotBefore) carried alongside the body without touching it.
type Message struct {
	Key     []byte
	Value   []byte
	Headers map[string]string
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
			// Every record for the same key (entity ID) must land in the same
			// partition and be written in the order Publish was called, so
			// e.g. case.created is never processed after case.comment_added
			// for the same case just because of network timing. Hash
			// deterministically maps a key to a partition (falls back to
			// round-robin only for a nil key, which never happens here — see
			// cmd/server/main.go's dead-letter OnExhausted func, this
			// service's only caller of Publish now that the producer side of
			// case.* events lives in the backends).
			Balancer: &kafka.Hash{},
			// Wait for the full ISR to acknowledge before Publish returns,
			// matching the previous Kafka client's synchronous-produce
			// behavior.
			RequiredAcks: kafka.RequireAll,
			// Flush each Publish on its own. kafka-go's synchronous Writer
			// otherwise holds a message until BatchSize (default 100)
			// accumulate or BatchTimeout (default 1 s) passes, so a lone
			// Publish -- every call this service makes -- waited about a
			// second: once per dead-letter publish while the partition
			// was blocked, and once per crossed SLA tier inside the SLA
			// engine's sequential tick. Async is deliberately not used:
			// every caller acts on the result (a failed dead-letter
			// publish falls back to parking, a failed tier publish
			// releases the tier claim for a retry).
			BatchSize:    publishBatchSize,
			BatchTimeout: publishBatchTimeout,
			Transport: &kafka.Transport{
				TLS:  &tls.Config{MinVersion: tls.VersionTLS12},
				SASL: cfg.saslMechanism(),
			},
			Logger:      kafka.LoggerFunc(logDebug),
			ErrorLogger: kafka.LoggerFunc(logError),
			// Compression is deliberately left unset (no codec). Azure Event
			// Hub's Kafka-compatible endpoint rejected compressed batches
			// with "UNSUPPORTED_FOR_MESSAGE_FORMAT" under the Kafka client
			// this service used before kafka-go — confirmed against the real
			// namespace, not a guess. At ~5,000 events/day, compression isn't
			// a throughput concern anyway.
		},
	}
}

// Publish sends value as a single record, keyed by key, and waits for the
// broker's acknowledgment before returning. key determines the partition —
// pass the same key (e.g. an entity ID) for every event that must stay
// ordered relative to each other.
func (p *Producer) Publish(ctx context.Context, key, value []byte) error {
	return p.PublishMessage(ctx, Message{Key: key, Value: value})
}

// PublishMessage is Publish with optional record headers.
func (p *Producer) PublishMessage(ctx context.Context, msg Message) error {
	km := kafka.Message{Key: msg.Key, Value: msg.Value}
	for k, v := range msg.Headers {
		km.Headers = append(km.Headers, kafka.Header{Key: k, Value: []byte(v)})
	}
	if err := p.writer.WriteMessages(ctx, km); err != nil {
		return fmt.Errorf("eventbus: publish: %w", err)
	}
	return nil
}

// Close releases the underlying connection. Safe to call once during
// shutdown.
func (p *Producer) Close() {
	_ = p.writer.Close()
}
