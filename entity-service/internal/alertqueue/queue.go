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

// Package alertqueue is the alert consumer's cloud-neutral view of a queue. Each provider type (servicebus, sqs) puts
// its SDK's receive, lock and settle calls behind it, so the consumer is written once.
package alertqueue

import (
	"context"
	"time"
)

// Message is one message received from an alert queue. It stays locked (hidden from other receivers) until it is
// settled, as long as its lock is renewed.
type Message interface {
	// ID is the queue's message id; a redelivery of the message keeps it.
	ID() string
	Body() []byte
	ContentType() string
	// Source is the route the gateway named in the message's "source" property, e.g. prometheus or azure.
	Source() string
	// EnqueuedOn is when the queue accepted the message; zero when the queue does not say.
	EnqueuedOn() time.Time
}

// Queue is one cloud queue of one tier.
type Queue interface {
	// Name identifies the queue in logs and in the stored message id.
	Name() string
	// Receive waits for messages, at most about 20 seconds, and returns up to max of them (possibly none).
	Receive(ctx context.Context, max int) ([]Message, error)
	// Complete removes a settled message from the queue.
	Complete(ctx context.Context, m Message) error
	// Abandon gives a message back so it is delivered again soon; the queue counts the delivery.
	Abandon(ctx context.Context, m Message) error
	// DeadLetter moves a message that can never be stored aside, with the reason.
	DeadLetter(ctx context.Context, m Message, reason, description string) error
	// RenewLock keeps a held message hidden for another lock period.
	RenewLock(ctx context.Context, m Message) error
	// LockRenewInterval is how often a held message's lock must be renewed.
	LockRenewInterval() time.Duration
	// Close releases the queue's connection.
	Close(ctx context.Context) error
}

// SourceProperty is the message property (Service Bus application property, SQS message attribute) the gateways set
// to the alert's source route.
const SourceProperty = "source"
