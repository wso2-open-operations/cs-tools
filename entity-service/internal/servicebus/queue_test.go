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
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// fakeReceiver records the Service Bus calls a Queue makes.
type fakeReceiver struct {
	messages     []*azservicebus.ReceivedMessage
	completed    int
	abandoned    int
	deadLettered string
	renewed      int
}

func (r *fakeReceiver) ReceiveMessages(context.Context, int, *azservicebus.ReceiveMessagesOptions) ([]*azservicebus.ReceivedMessage, error) {
	return r.messages, nil
}

func (r *fakeReceiver) CompleteMessage(context.Context, *azservicebus.ReceivedMessage, *azservicebus.CompleteMessageOptions) error {
	r.completed++
	return nil
}

func (r *fakeReceiver) AbandonMessage(context.Context, *azservicebus.ReceivedMessage, *azservicebus.AbandonMessageOptions) error {
	r.abandoned++
	return nil
}

func (r *fakeReceiver) DeadLetterMessage(_ context.Context, _ *azservicebus.ReceivedMessage, o *azservicebus.DeadLetterOptions) error {
	r.deadLettered = *o.Reason
	return nil
}

func (r *fakeReceiver) RenewMessageLock(_ context.Context, m *azservicebus.ReceivedMessage, _ *azservicebus.RenewMessageLockOptions) error {
	r.renewed++
	later := m.LockedUntil.Add(time.Minute)
	m.LockedUntil = &later
	return nil
}

func (r *fakeReceiver) Close(context.Context) error { return nil }

func TestQueue_ConvertsAndSettlesMessages(t *testing.T) {
	enqueued := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	locked := enqueued.Add(time.Minute)
	r := &fakeReceiver{messages: []*azservicebus.ReceivedMessage{{
		MessageID:             "m-1",
		Body:                  []byte("{}"),
		ApplicationProperties: map[string]any{"source": " azure ", "count": 3, "empty": nil},
		EnqueuedTime:          &enqueued,
		LockedUntil:           &locked,
	}}}
	q := &Queue{name: "priority-queue", receiver: r}
	ctx := context.Background()

	messages, err := q.Receive(ctx, 1)
	if err != nil || len(messages) != 1 {
		t.Fatalf("Receive = %v, %v", messages, err)
	}
	m := messages[0]
	if m.ID != "m-1" || m.Properties["source"] != "azure" || m.Properties["count"] != "3" || !m.EnqueuedOn.Equal(enqueued) || !m.LockedUntil.Equal(locked) {
		t.Fatalf("message = %+v, want the id, text properties and times", m)
	}
	if _, present := m.Properties["empty"]; present {
		t.Fatal("a nil property was kept")
	}
	if err := q.RenewLock(ctx, m); err != nil || !m.LockedUntil.Equal(locked.Add(time.Minute)) {
		t.Fatalf("RenewLock left locked until %v (%v), want a minute later", m.LockedUntil, err)
	}
	_ = q.Complete(ctx, m)
	_ = q.Abandon(ctx, m)
	_ = q.DeadLetter(ctx, m, "UnknownSource", "no transform")
	if r.completed != 1 || r.abandoned != 1 || r.deadLettered != "UnknownSource" {
		t.Fatalf("completed %d, abandoned %d, dead-lettered %q", r.completed, r.abandoned, r.deadLettered)
	}
	if q.Name() != "servicebus/priority-queue" {
		t.Fatalf("name = %q", q.Name())
	}
}

func TestQueue_RefusesAMessageFromAnotherQueue(t *testing.T) {
	q := &Queue{name: "priority-queue", receiver: &fakeReceiver{}}
	if err := q.Complete(context.Background(), &domain.AlertQueueMessage{ID: "x"}); !errors.Is(err, errForeignMessage) {
		t.Fatalf("Complete = %v, want errForeignMessage", err)
	}
}
