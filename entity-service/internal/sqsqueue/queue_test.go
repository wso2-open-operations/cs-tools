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

package sqsqueue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// fakeClient records the SQS calls a Queue makes; receives returns each batch in turn, then empty polls.
type fakeClient struct {
	receives   [][]types.Message
	receiveErr error
	polls      int
	onCall     func() // runs inside receive and visibility calls, to model a slow response
	deleted    []string
	visibility map[string]int32
	sent       []*sqs.SendMessageInput
	sendErr    error
	// calls is every settlement call in order, so a test can check that the copy comes before the delete.
	calls []string
}

func (c *fakeClient) ReceiveMessage(ctx context.Context, in *sqs.ReceiveMessageInput, _ ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	c.polls++
	if c.onCall != nil {
		c.onCall()
	}
	if c.receiveErr != nil {
		return nil, c.receiveErr
	}
	if in.WaitTimeSeconds != 20 || in.VisibilityTimeout != 60 || in.MaxNumberOfMessages < 1 || in.MaxNumberOfMessages > 10 {
		return nil, errors.New("unexpected receive settings")
	}
	if len(c.receives) == 0 {
		return &sqs.ReceiveMessageOutput{}, ctx.Err()
	}
	batch := c.receives[0]
	c.receives = c.receives[1:]
	return &sqs.ReceiveMessageOutput{Messages: batch}, nil
}

func (c *fakeClient) DeleteMessage(_ context.Context, in *sqs.DeleteMessageInput, _ ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	c.deleted = append(c.deleted, aws.ToString(in.ReceiptHandle))
	c.calls = append(c.calls, "delete")
	return &sqs.DeleteMessageOutput{}, nil
}

func (c *fakeClient) ChangeMessageVisibility(_ context.Context, in *sqs.ChangeMessageVisibilityInput, _ ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
	if c.onCall != nil {
		c.onCall()
	}
	if c.visibility == nil {
		c.visibility = map[string]int32{}
	}
	c.visibility[aws.ToString(in.ReceiptHandle)] = in.VisibilityTimeout
	c.calls = append(c.calls, "visibility")
	return &sqs.ChangeMessageVisibilityOutput{}, nil
}

func (c *fakeClient) SendMessage(_ context.Context, in *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	c.calls = append(c.calls, "send")
	if c.sendErr != nil {
		return nil, c.sendErr
	}
	c.sent = append(c.sent, in)
	return &sqs.SendMessageOutput{}, nil
}

const priorityURL = "https://sqs.us-east-1.amazonaws.com/123456789012/priority-queue"

func sqsMessage(id string) types.Message {
	return types.Message{
		MessageId:     aws.String(id),
		ReceiptHandle: aws.String("receipt-" + id),
		Body:          aws.String(`{"alert":true}`),
		Attributes:    map[string]string{"SentTimestamp": "1794560400000"},
		MessageAttributes: map[string]types.MessageAttributeValue{
			"source":       {DataType: aws.String("String"), StringValue: aws.String(" azure ")},
			"received_at":  {DataType: aws.String("String"), StringValue: aws.String("2026-11-13T10:00:00Z")},
			"content_type": {DataType: aws.String("String"), StringValue: aws.String("application/json")},
		},
	}
}

func receiveOne(t *testing.T, q *Queue) *domain.AlertQueueMessage {
	t.Helper()
	messages, err := q.Receive(context.Background(), 5)
	if err != nil || len(messages) != 1 {
		t.Fatalf("Receive = %v, %v; want one message", messages, err)
	}
	return messages[0]
}

func TestQueue_ReceiveKeepsPollingUntilAMessageArrives(t *testing.T) {
	c := &fakeClient{receives: [][]types.Message{nil, nil, {sqsMessage("m-1")}}}
	q := newQueue(c, priorityURL, "")
	now := time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	q.now = func() time.Time { return now }

	m := receiveOne(t, q)
	if c.polls != 3 {
		t.Fatalf("polled %d times, want 3", c.polls)
	}
	if m.ID != "m-1" || string(m.Body) != `{"alert":true}` || m.Properties["source"] != "azure" || m.Properties["received_at"] != "2026-11-13T10:00:00Z" {
		t.Fatalf("message = %+v, want the id, body and trimmed attributes", m)
	}
	if m.ContentType != "application/json" {
		t.Fatalf("content type = %q, want the content_type attribute", m.ContentType)
	}
	if m.EnqueuedOn == nil || !m.EnqueuedOn.Equal(time.UnixMilli(1794560400000)) {
		t.Fatalf("enqueued on = %v, want the SentTimestamp", m.EnqueuedOn)
	}
	if m.LockedUntil == nil || !m.LockedUntil.Equal(now.Add(time.Minute)) {
		t.Fatalf("locked until = %v, want 60 seconds after receipt", m.LockedUntil)
	}
	if q.Name() != "sqs/priority-queue" {
		t.Fatalf("name = %q", q.Name())
	}
}

func TestQueue_LockExpiryCountsFromTheRequestNotTheResponse(t *testing.T) {
	c := &fakeClient{receives: [][]types.Message{{sqsMessage("m-1")}}}
	q := newQueue(c, priorityURL, "")
	start := time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	now := start
	q.now = func() time.Time { return now }
	c.onCall = func() { now = now.Add(20 * time.Second) }

	m := receiveOne(t, q)
	if !m.LockedUntil.Equal(start.Add(time.Minute)) {
		t.Fatalf("locked until = %v after a slow receive, want 60 seconds after the request at %v", m.LockedUntil, start)
	}
	renewAt := now
	if err := q.RenewLock(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	if !m.LockedUntil.Equal(renewAt.Add(time.Minute)) {
		t.Fatalf("locked until = %v after a slow renewal, want 60 seconds after the request at %v", m.LockedUntil, renewAt)
	}
}

func TestQueue_ReceiveStopsWhenContextIsDone(t *testing.T) {
	c := &fakeClient{}
	q := newQueue(c, priorityURL, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := q.Receive(ctx, 1); err == nil {
		t.Fatal("Receive returned no error after the context was cancelled")
	}
}

func TestQueue_Settlement(t *testing.T) {
	c := &fakeClient{receives: [][]types.Message{{sqsMessage("m-1"), sqsMessage("m-2"), sqsMessage("m-3")}}}
	q := newQueue(c, priorityURL, "")
	messages, _ := q.Receive(context.Background(), 3)
	ctx := context.Background()

	if err := q.Complete(ctx, messages[0]); err != nil || len(c.deleted) != 1 || c.deleted[0] != "receipt-m-1" {
		t.Fatalf("Complete deleted %v (%v), want receipt-m-1", c.deleted, err)
	}
	if err := q.Abandon(ctx, messages[1]); err != nil || !hasVisibility(c, "receipt-m-2", 0) {
		t.Fatalf("Abandon set visibility %v (%v), want 0", c.visibility, err)
	}
	before := *messages[2].LockedUntil
	q.now = func() time.Time { return before.Add(30 * time.Second) }
	if err := q.RenewLock(ctx, messages[2]); err != nil || c.visibility["receipt-m-3"] != 60 || !messages[2].LockedUntil.After(before) {
		t.Fatalf("RenewLock set visibility %v and locked until %v (%v), want 60 seconds more", c.visibility, messages[2].LockedUntil, err)
	}
}

func TestQueue_DeadLetterMovesTheMessageWithItsReason(t *testing.T) {
	c := &fakeClient{receives: [][]types.Message{{sqsMessage("m-1")}}}
	q := newQueue(c, priorityURL, "https://sqs.us-east-1.amazonaws.com/123456789012/alerts-dead-letter")
	m := receiveOne(t, q)

	if err := q.DeadLetter(context.Background(), m, "UnknownSource", "no transform"); err != nil {
		t.Fatalf("DeadLetter: %v", err)
	}
	if len(c.calls) != 2 || c.calls[0] != "send" || c.calls[1] != "delete" {
		t.Fatalf("calls = %v, want the message copied to the dead-letter queue and then deleted", c.calls)
	}
	sent := c.sent[0]
	if aws.ToString(sent.QueueUrl) != "https://sqs.us-east-1.amazonaws.com/123456789012/alerts-dead-letter" || aws.ToString(sent.MessageBody) != `{"alert":true}` {
		t.Fatalf("sent to %s body %s", aws.ToString(sent.QueueUrl), aws.ToString(sent.MessageBody))
	}
	for key, want := range map[string]string{"dead_letter_reason": "UnknownSource", "dead_letter_description": "no transform", "source_queue": "priority-queue", "source_message_id": "m-1", "source": " azure "} {
		if got := aws.ToString(sent.MessageAttributes[key].StringValue); got != want {
			t.Errorf("attribute %s = %q, want %q", key, got, want)
		}
	}
}

func TestQueue_DeadLetterWithoutAQueueLeavesItToTheRedrivePolicy(t *testing.T) {
	c := &fakeClient{receives: [][]types.Message{{sqsMessage("m-1")}}}
	q := newQueue(c, priorityURL, "")
	m := receiveOne(t, q)

	if err := q.DeadLetter(context.Background(), m, "UnknownSource", ""); err != nil {
		t.Fatalf("DeadLetter: %v", err)
	}
	if len(c.sent) != 0 || len(c.deleted) != 0 || !hasVisibility(c, "receipt-m-1", 0) {
		t.Fatalf("sent %d, deleted %d, visibility %v; want the message released, not moved", len(c.sent), len(c.deleted), c.visibility)
	}
}

func TestQueue_DeadLetterKeepsTheOriginalWhenTheCopyFails(t *testing.T) {
	c := &fakeClient{receives: [][]types.Message{{sqsMessage("m-1")}}, sendErr: errors.New("message too long")}
	q := newQueue(c, priorityURL, "https://sqs.us-east-1.amazonaws.com/123456789012/alerts-dead-letter")
	m := receiveOne(t, q)

	if err := q.DeadLetter(context.Background(), m, "UnknownSource", "no transform"); err == nil {
		t.Fatal("DeadLetter returned no error after the copy failed")
	}
	if len(c.deleted) != 0 || !hasVisibility(c, "receipt-m-1", 0) {
		t.Fatalf("deleted %v, visibility %v; want the original kept and released to the redrive policy", c.deleted, c.visibility)
	}
}

// hasVisibility reports whether a visibility change was actually sent for the receipt handle, with that timeout.
func hasVisibility(c *fakeClient, handle string, want int32) bool {
	got, ok := c.visibility[handle]
	return ok && got == want
}

func TestQueue_RefusesAMessageFromAnotherQueue(t *testing.T) {
	q := newQueue(&fakeClient{}, priorityURL, "")
	if err := q.Complete(context.Background(), &domain.AlertQueueMessage{ID: "x", Handle: "not an sqs message"}); !errors.Is(err, errForeignMessage) {
		t.Fatalf("Complete = %v, want errForeignMessage", err)
	}
}

func TestNewClient_RejectsHalfAKeyPair(t *testing.T) {
	if _, err := NewClient(context.Background(), Config{Region: "us-east-1", AccessKeyID: "id"}); err == nil {
		t.Fatal("NewClient accepted an access key id without its secret")
	}
	if _, err := NewClient(context.Background(), Config{}); err == nil {
		t.Fatal("NewClient accepted no region")
	}
}
