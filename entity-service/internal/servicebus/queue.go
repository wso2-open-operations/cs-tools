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

package servicebus

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// receiver is the part of *azservicebus.Receiver a Queue uses.
type receiver interface {
	ReceiveMessages(ctx context.Context, maxMessages int, options *azservicebus.ReceiveMessagesOptions) ([]*azservicebus.ReceivedMessage, error)
	CompleteMessage(ctx context.Context, message *azservicebus.ReceivedMessage, options *azservicebus.CompleteMessageOptions) error
	AbandonMessage(ctx context.Context, message *azservicebus.ReceivedMessage, options *azservicebus.AbandonMessageOptions) error
	DeadLetterMessage(ctx context.Context, message *azservicebus.ReceivedMessage, options *azservicebus.DeadLetterOptions) error
	RenewMessageLock(ctx context.Context, message *azservicebus.ReceivedMessage, options *azservicebus.RenewMessageLockOptions) error
	Close(ctx context.Context) error
}

// errForeignMessage means a message was settled on a queue that did not receive it.
var errForeignMessage = errors.New("message was not received from a Service Bus queue")

// Queue is one Service Bus queue the alert consumer receives from, holding one AMQP link open.
type Queue struct {
	name     string
	receiver receiver
}

// NewQueue opens a peek-lock receiver on a queue.
func NewQueue(client *azservicebus.Client, name string) (*Queue, error) {
	r, err := client.NewReceiverForQueue(name, nil)
	if err != nil {
		return nil, fmt.Errorf("service bus queue %s: %w", name, err)
	}
	return &Queue{name: name, receiver: r}, nil
}

// Name identifies the queue in logs.
func (q *Queue) Name() string {
	return "servicebus/" + q.name
}

// Receive waits on the open link until at least one message arrives or ctx is done.
func (q *Queue) Receive(ctx context.Context, maxMessages int) ([]*domain.AlertQueueMessage, error) {
	received, err := q.receiver.ReceiveMessages(ctx, maxMessages, nil)
	messages := make([]*domain.AlertQueueMessage, len(received))
	for i, m := range received {
		messages[i] = toAlertQueueMessage(m)
	}
	return messages, err
}

// Complete removes the message from the queue.
func (q *Queue) Complete(ctx context.Context, message *domain.AlertQueueMessage) error {
	m, err := handleOf(message)
	if err != nil {
		return err
	}
	return q.receiver.CompleteMessage(ctx, m, nil)
}

// Abandon releases the lock so the message is delivered again at once.
func (q *Queue) Abandon(ctx context.Context, message *domain.AlertQueueMessage) error {
	m, err := handleOf(message)
	if err != nil {
		return err
	}
	return q.receiver.AbandonMessage(ctx, m, nil)
}

// DeadLetter moves the message to the queue's own dead-letter subqueue with the reason.
func (q *Queue) DeadLetter(ctx context.Context, message *domain.AlertQueueMessage, reason, description string) error {
	m, err := handleOf(message)
	if err != nil {
		return err
	}
	return q.receiver.DeadLetterMessage(ctx, m, &azservicebus.DeadLetterOptions{Reason: &reason, ErrorDescription: &description})
}

// RenewLock extends the message lock and records its new expiry.
func (q *Queue) RenewLock(ctx context.Context, message *domain.AlertQueueMessage) error {
	m, err := handleOf(message)
	if err != nil {
		return err
	}
	if err := q.receiver.RenewMessageLock(ctx, m, nil); err != nil {
		return err
	}
	message.LockedUntil = copyTime(m.LockedUntil)
	return nil
}

// Close closes the queue's link.
func (q *Queue) Close(ctx context.Context) error {
	return q.receiver.Close(ctx)
}

func toAlertQueueMessage(m *azservicebus.ReceivedMessage) *domain.AlertQueueMessage {
	properties := make(map[string]string, len(m.ApplicationProperties))
	for key, value := range m.ApplicationProperties {
		if value == nil {
			continue
		}
		if s, ok := value.(string); ok {
			properties[key] = strings.TrimSpace(s)
		} else {
			properties[key] = strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return &domain.AlertQueueMessage{
		ID:          m.MessageID,
		Body:        m.Body,
		Properties:  properties,
		EnqueuedOn:  copyTime(m.EnqueuedTime),
		LockedUntil: copyTime(m.LockedUntil),
		Handle:      m,
	}
}

func handleOf(message *domain.AlertQueueMessage) (*azservicebus.ReceivedMessage, error) {
	m, ok := message.Handle.(*azservicebus.ReceivedMessage)
	if !ok || m == nil {
		return nil, errForeignMessage
	}
	return m, nil
}

func copyTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	c := *t
	return &c
}
