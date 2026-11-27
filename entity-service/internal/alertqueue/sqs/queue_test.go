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

package sqs

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type fakeAPI struct {
	receive    *awssqs.ReceiveMessageInput
	messages   []types.Message
	deleted    []string
	visibility map[string]int32
	sent       []*awssqs.SendMessageInput
}

func (f *fakeAPI) ReceiveMessage(_ context.Context, in *awssqs.ReceiveMessageInput, _ ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error) {
	f.receive = in
	return &awssqs.ReceiveMessageOutput{Messages: f.messages}, nil
}

func (f *fakeAPI) DeleteMessage(_ context.Context, in *awssqs.DeleteMessageInput, _ ...func(*awssqs.Options)) (*awssqs.DeleteMessageOutput, error) {
	f.deleted = append(f.deleted, aws.ToString(in.ReceiptHandle))
	return &awssqs.DeleteMessageOutput{}, nil
}

func (f *fakeAPI) ChangeMessageVisibility(_ context.Context, in *awssqs.ChangeMessageVisibilityInput, _ ...func(*awssqs.Options)) (*awssqs.ChangeMessageVisibilityOutput, error) {
	f.visibility[aws.ToString(in.ReceiptHandle)] = in.VisibilityTimeout
	return &awssqs.ChangeMessageVisibilityOutput{}, nil
}

func (f *fakeAPI) SendMessage(_ context.Context, in *awssqs.SendMessageInput, _ ...func(*awssqs.Options)) (*awssqs.SendMessageOutput, error) {
	f.sent = append(f.sent, in)
	return &awssqs.SendMessageOutput{}, nil
}

func TestRegionOf(t *testing.T) {
	for u, want := range map[string]string{
		"https://sqs.us-east-1.amazonaws.com/000000000000/q":   "us-east-1",
		"https://eu-west-1.queue.amazonaws.com/000000000000/q": "eu-west-1",
		"http://localstack:4566/000000000000/q":                "",
		"::not a url":                                          "",
	} {
		if got := RegionOf(u); got != want {
			t.Errorf("RegionOf(%q) = %q, want %q", u, got, want)
		}
	}
}

func TestQueue_ReceiveReadsSourceAndContentType(t *testing.T) {
	api := &fakeAPI{visibility: map[string]int32{}, messages: []types.Message{{
		MessageId: aws.String("m1"), ReceiptHandle: aws.String("r1"), Body: aws.String("{}"),
		MessageAttributes: map[string]types.MessageAttributeValue{
			"source":       {DataType: aws.String("String"), StringValue: aws.String("prometheus")},
			"content-type": {DataType: aws.String("String"), StringValue: aws.String("application/json")},
		},
		Attributes: map[string]string{"SentTimestamp": "1795593600123"},
	}}}
	q := NewQueue(api, "https://sqs.us-east-1.amazonaws.com/000000000000/alert-priority", "")
	msgs, err := q.Receive(context.Background(), 25)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("Receive = %v, %v", msgs, err)
	}
	if api.receive.MaxNumberOfMessages != 10 || api.receive.WaitTimeSeconds != waitSeconds || api.receive.VisibilityTimeout != visibilitySeconds {
		t.Errorf("receive input = %+v, want 10 messages at most, a long poll and the visibility timeout", api.receive)
	}
	m := msgs[0]
	if q.Name() != "alert-priority" || m.ID() != "m1" || string(m.Body()) != "{}" || m.Source() != "prometheus" ||
		m.ContentType() != "application/json" || m.EnqueuedOn().UnixMilli() != 1795593600123 {
		t.Errorf("message = %s %q %q %q %s, want the SQS fields", m.ID(), m.Body(), m.Source(), m.ContentType(), m.EnqueuedOn())
	}
}

func TestQueue_SettlesThroughVisibilityAndDelete(t *testing.T) {
	api := &fakeAPI{visibility: map[string]int32{}, messages: []types.Message{{MessageId: aws.String("m1"), ReceiptHandle: aws.String("r1"), Body: aws.String("x"),
		MessageAttributes: map[string]types.MessageAttributeValue{"source": {DataType: aws.String("String"), StringValue: aws.String("aws")}}}}}
	q := NewQueue(api, "https://sqs.us-east-1.amazonaws.com/000000000000/q", "https://sqs.us-east-1.amazonaws.com/000000000000/q-dlq")
	msgs, _ := q.Receive(context.Background(), 1)
	m := msgs[0]
	ctx := context.Background()

	if err := q.RenewLock(ctx, m); err != nil || api.visibility["r1"] != visibilitySeconds {
		t.Fatalf("RenewLock: %v, visibility %d", err, api.visibility["r1"])
	}
	if err := q.Abandon(ctx, m); err != nil || api.visibility["r1"] != 0 {
		t.Fatalf("Abandon: %v, visibility %d; want 0 (visible again at once)", err, api.visibility["r1"])
	}
	if err := q.DeadLetter(ctx, m, "StoreRefused", "check constraint"); err != nil {
		t.Fatal(err)
	}
	if len(api.sent) != 1 || aws.ToString(api.sent[0].QueueUrl) != "https://sqs.us-east-1.amazonaws.com/000000000000/q-dlq" ||
		aws.ToString(api.sent[0].MessageBody) != "x" || aws.ToString(api.sent[0].MessageAttributes[deadLetterReasonAttribute].StringValue) != "StoreRefused" ||
		aws.ToString(api.sent[0].MessageAttributes["source"].StringValue) != "aws" {
		t.Fatalf("dead letter sent = %+v, want the body, its source and the reason on the dead-letter queue", api.sent)
	}
	if len(api.deleted) != 1 || api.deleted[0] != "r1" {
		t.Fatalf("deleted = %v, want the original removed after the dead-letter send", api.deleted)
	}
}

func TestQueue_DeadLetterWithoutADeadLetterQueueGivesTheMessageBack(t *testing.T) {
	api := &fakeAPI{visibility: map[string]int32{"r1": 60}, messages: []types.Message{{MessageId: aws.String("m1"), ReceiptHandle: aws.String("r1")}}}
	q := NewQueue(api, "https://sqs.us-east-1.amazonaws.com/000000000000/q", "")
	msgs, _ := q.Receive(context.Background(), 1)
	if err := q.DeadLetter(context.Background(), msgs[0], "StoreRefused", ""); err != nil {
		t.Fatal(err)
	}
	if len(api.sent) != 0 || len(api.deleted) != 0 || api.visibility["r1"] != 0 {
		t.Fatalf("sent %d, deleted %v, visibility %d; want it given back for the redrive policy", len(api.sent), api.deleted, api.visibility["r1"])
	}
}
