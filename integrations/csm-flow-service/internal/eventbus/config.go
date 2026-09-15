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

// Package eventbus wraps github.com/segmentio/kafka-go to talk to Azure
// Event Hub's Kafka-compatible endpoint. It provides just two things: a
// Producer (publish a record, wait for the broker's ack) and a Consumer (join
// a consumer group, poll records, commit offsets after they're handled).
//
// This is a VERBATIM COPY of csm-notification-service's internal/eventbus —
// the two services are separate Go modules and neither imports the other by
// design, so the package is copied rather than shared (see docs/architecture.md
// §18). It carries hard-won Event Hub constraints (compression unset,
// idempotent producing off, "$ConnectionString" SASL user, explicit
// FirstOffset) and must be kept in sync with that copy by hand. Do not edit
// csm-notification-service's copy from here.
package eventbus

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	kafka "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
)

// Config holds the connection settings shared by Producer and Consumer.
type Config struct {
	// Broker is the Kafka-compatible bootstrap address, e.g.
	// "<namespace>.servicebus.windows.net:9093".
	Broker string
	// ConnectionString is the Event Hub namespace's Shared Access Policy
	// connection string — the SASL/PLAIN password; Event Hub's Kafka surface
	// always expects the literal username "$ConnectionString".
	ConnectionString string
	// Topic is the Event Hub name (Kafka topic) to produce to / consume from.
	Topic string
}

// saslMechanism builds the SASL/PLAIN credential Event Hub's Kafka endpoint
// requires: username is always the literal string "$ConnectionString", and
// the password is the connection string itself.
func (c Config) saslMechanism() sasl.Mechanism {
	return plain.Mechanism{
		Username: "$ConnectionString",
		Password: c.ConnectionString,
	}
}

// partitionCountProbeTimeout bounds PartitionCount's entire dial+read so an
// unreachable broker cannot block startup indefinitely.
const partitionCountProbeTimeout = 10 * time.Second

// PartitionCount returns cfg.Topic's current partition count — used at
// startup to sanity-check a configured consumer count against reality: a
// Kafka consumer group only ever hands out as many partitions as exist, so a
// consumer count higher than this leaves some consumers permanently idle.
func PartitionCount(ctx context.Context, cfg Config) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, partitionCountProbeTimeout)
	defer cancel()
	deadline, _ := ctx.Deadline()

	dialer := &kafka.Dialer{
		TLS:           &tls.Config{MinVersion: tls.VersionTLS12},
		SASLMechanism: cfg.saslMechanism(),
	}
	conn, err := dialer.DialContext(ctx, "tcp", cfg.Broker)
	if err != nil {
		return 0, fmt.Errorf("eventbus: dial %s: %w", cfg.Broker, err)
	}
	defer func() {
		_ = conn.Close()
	}()

	if err := conn.SetDeadline(deadline); err != nil {
		return 0, fmt.Errorf("eventbus: set deadline for topic %s: %w", cfg.Topic, err)
	}

	partitions, err := conn.ReadPartitions(cfg.Topic)
	if err != nil {
		return 0, fmt.Errorf("eventbus: read partitions for topic %s: %w", cfg.Topic, err)
	}
	return len(partitions), nil
}
