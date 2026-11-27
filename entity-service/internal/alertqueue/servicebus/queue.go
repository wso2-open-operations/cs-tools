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

// Package servicebus receives SRE alerts from Azure Service Bus queues in peek-lock mode, for the alert consumer.
package servicebus

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertqueue"
)

// receiveWait is how long one Receive waits for a first message before it returns none.
const receiveWait = 20 * time.Second

// lockRenewInterval renews a held message's lock well inside Service Bus's default 60-second lock duration.
const lockRenewInterval = 20 * time.Second

// Client is one Service Bus namespace.
type Client struct {
	client *azservicebus.Client
}

// NewClient connects to the namespace a shared access policy's connection string names. The connection opens lazily.
func NewClient(connectionString string) (*Client, error) {
	c, err := azservicebus.NewClientFromConnectionString(connectionString, nil)
	if err != nil {
		// The SDK's error can quote the connection string, which holds the key; never pass it on.
		return nil, errors.New("service bus: the connection string is not valid")
	}
	return &Client{client: c}, nil
}

// Queue returns a peek-lock receiver for the named queue.
func (c *Client) Queue(name string) (*Queue, error) {
	r, err := c.client.NewReceiverForQueue(name, &azservicebus.ReceiverOptions{ReceiveMode: azservicebus.ReceiveModePeekLock})
	if err != nil {
		return nil, fmt.Errorf("service bus: receiver for %s: %w", name, err)
	}
	return &Queue{name: name, receiver: r}, nil
}

// Close closes the namespace connection; close the queues first.
func (c *Client) Close(ctx context.Context) error {
	return c.client.Close(ctx)
}

// Queue is one Service Bus queue.
type Queue struct {
	name     string
	receiver *azservicebus.Receiver
}

var _ alertqueue.Queue = (*Queue)(nil)

// Name is the queue's name.
func (q *Queue) Name() string { return q.name }

// Receive waits up to receiveWait for messages and returns up to limit of them.
func (q *Queue) Receive(ctx context.Context, limit int) ([]alertqueue.Message, error) {
	rctx, cancel := context.WithTimeout(ctx, receiveWait)
	defer cancel()
	msgs, err := q.receiver.ReceiveMessages(rctx, limit, nil)
	if err != nil {
		if ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
			return nil, nil // nothing arrived in time
		}
		return nil, fmt.Errorf("service bus: receive from %s: %w", q.name, err)
	}
	out := make([]alertqueue.Message, len(msgs))
	for i, m := range msgs {
		out[i] = message{m}
	}
	return out, nil
}

// Complete removes the message.
func (q *Queue) Complete(ctx context.Context, m alertqueue.Message) error {
	return q.receiver.CompleteMessage(ctx, raw(m), nil)
}

// Abandon releases the lock so the message is delivered again; Service Bus counts the delivery and dead-letters it
// after the queue's maximum delivery count.
func (q *Queue) Abandon(ctx context.Context, m alertqueue.Message) error {
	return q.receiver.AbandonMessage(ctx, raw(m), nil)
}

// DeadLetter moves the message to the queue's dead-letter subqueue with the reason.
func (q *Queue) DeadLetter(ctx context.Context, m alertqueue.Message, reason, description string) error {
	return q.receiver.DeadLetterMessage(ctx, raw(m), &azservicebus.DeadLetterOptions{Reason: &reason, ErrorDescription: &description})
}

// RenewLock extends the message's lock by the queue's lock duration.
func (q *Queue) RenewLock(ctx context.Context, m alertqueue.Message) error {
	return q.receiver.RenewMessageLock(ctx, raw(m), nil)
}

// LockRenewInterval is how often a held message's lock is renewed.
func (q *Queue) LockRenewInterval() time.Duration { return lockRenewInterval }

// Close closes the receiver.
func (q *Queue) Close(ctx context.Context) error { return q.receiver.Close(ctx) }

type message struct {
	m *azservicebus.ReceivedMessage
}

func raw(m alertqueue.Message) *azservicebus.ReceivedMessage {
	return m.(message).m
}

func (m message) ID() string   { return m.m.MessageID }
func (m message) Body() []byte { return m.m.Body }

func (m message) ContentType() string {
	if m.m.ContentType == nil {
		return ""
	}
	return *m.m.ContentType
}

func (m message) Source() string {
	v, ok := m.m.ApplicationProperties[alertqueue.SourceProperty]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func (m message) EnqueuedOn() time.Time {
	if m.m.EnqueuedTime == nil {
		return time.Time{}
	}
	return *m.m.EnqueuedTime
}
