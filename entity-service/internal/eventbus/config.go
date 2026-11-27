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

// Package eventbus wraps github.com/segmentio/kafka-go to publish domain
// events to Azure Event Hub's Kafka-compatible endpoint — the producer side
// of the pipeline csm-notification-service consumes from. Only a Producer
// exists here — no Consumer, since this service is not in the business of
// reacting to these events itself. Package choice and config shape
// deliberately mirror apps/csm-portal/backend's own internal/eventbus
// (that backend's copy of this pipeline predates this one) so the two stay
// easy to compare.
package eventbus

import (
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
)

// Config holds the connection settings for Producer.
type Config struct {
	// Broker is the Kafka-compatible bootstrap address, e.g.
	// "<namespace>.servicebus.windows.net:9093" — the standard Kafka port
	// Event Hub's Standard tier and above expose alongside its native AMQP
	// endpoint.
	Broker string
	// ConnectionString is the Event Hub namespace's Shared Access Policy
	// connection string (Namespace > Shared access policies > a policy's
	// Primary Connection String). This is the SASL/PLAIN password; Event
	// Hub's Kafka surface always expects the literal username
	// "$ConnectionString" — see saslMechanism. Must be namespace-scoped (no
	// EntityPath suffix), not scoped to a single Event Hub, if this service
	// is ever pointed at more than one topic.
	ConnectionString string
	// Topic is the Event Hub name (Kafka topic) to produce to.
	Topic string
}

// saslMechanism builds the SASL/PLAIN credential Event Hub's Kafka endpoint
// requires: username is always the literal string "$ConnectionString" (not a
// real username — this tells Event Hub the password is a connection string,
// not an Azure AD token), and the password is the connection string itself.
func (c Config) saslMechanism() sasl.Mechanism {
	return plain.Mechanism{
		Username: "$ConnectionString",
		Password: c.ConnectionString,
	}
}
