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

// Package sqsqueue receives alerts from AWS SQS standard queues by long polling, signing in with access keys or the default AWS credential chain.
package sqsqueue

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Config holds what is needed to reach SQS; without keys the default AWS credential chain is used.
type Config struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
}

// Long polling and visibility settings, matching the 60 second lock the Service Bus queues use.
const (
	// waitTime is the longest one receive request stays open waiting for a message, the SQS maximum.
	waitTime = 20 * time.Second
	// visibilityTimeout is how long a received message stays hidden from other receivers unless renewed.
	visibilityTimeout = 60 * time.Second
	// maxReceiveBatch is the most messages SQS returns to one receive request.
	maxReceiveBatch = 10
	// maxMessageAttributes is the most message attributes SQS accepts on one message.
	maxMessageAttributes = 10
)

// Message attributes added to a dead-lettered message, so the reason travels with it.
const (
	attributeDeadLetterReason      = "dead_letter_reason"
	attributeDeadLetterDescription = "dead_letter_description"
	attributeSourceQueue           = "source_queue"
	// attributeSourceMessageID is the original SQS message id, which stays the same across redeliveries, so copies of one message can be matched.
	attributeSourceMessageID = "source_message_id"
)

// errForeignMessage means a message was settled on a queue that did not receive it.
var errForeignMessage = errors.New("message was not received from an SQS queue")

// NewClient returns an SQS client for one region.
func NewClient(ctx context.Context, cfg Config) (*sqs.Client, error) {
	if cfg.Region == "" {
		return nil, errors.New("sqs region is required")
	}
	if (cfg.AccessKeyID == "") != (cfg.SecretAccessKey == "") {
		return nil, errors.New("sqs access key id and secret access key must be set together")
	}
	options := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(cfg.Region)}
	if cfg.AccessKeyID != "" {
		options = append(options, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("sqs config: %w", err)
	}
	return sqs.NewFromConfig(awsCfg), nil
}

// client is the part of *sqs.Client a Queue uses.
type client interface {
	ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	ChangeMessageVisibility(ctx context.Context, params *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error)
	SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

// Queue is one SQS standard queue the alert consumer receives from.
type Queue struct {
	client        client
	url           string
	name          string
	deadLetterURL string
	now           func() time.Time
}

// NewQueue returns a Queue; deadLetterURL may be empty, in which case the queue's own redrive policy handles dead letters.
func NewQueue(c *sqs.Client, queueURL, deadLetterURL string) *Queue {
	return newQueue(c, queueURL, deadLetterURL)
}

func newQueue(c client, queueURL, deadLetterURL string) *Queue {
	name := queueURL
	if i := strings.LastIndex(strings.TrimSuffix(queueURL, "/"), "/"); i >= 0 {
		name = strings.TrimSuffix(queueURL, "/")[i+1:]
	}
	return &Queue{client: c, url: queueURL, name: name, deadLetterURL: deadLetterURL, now: time.Now}
}

// Name identifies the queue in logs.
func (q *Queue) Name() string {
	return "sqs/" + q.name
}

// Receive long polls until at least one message arrives or ctx is done; SQS returns at most 10 at a time.
func (q *Queue) Receive(ctx context.Context, maxMessages int) ([]*domain.AlertQueueMessage, error) {
	for {
		// SQS hides a message before its response arrives, so the expiry counts from the request, not the reply.
		requestedAt := q.now()
		out, err := q.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:                    aws.String(q.url),
			MaxNumberOfMessages:         int32(min(max(maxMessages, 1), maxReceiveBatch)), // #nosec G115 -- clamped to 1..10
			WaitTimeSeconds:             int32(waitTime / time.Second),
			VisibilityTimeout:           int32(visibilityTimeout / time.Second),
			MessageAttributeNames:       []string{"All"},
			MessageSystemAttributeNames: []types.MessageSystemAttributeName{types.MessageSystemAttributeNameSentTimestamp},
		})
		if err != nil {
			return nil, err
		}
		if len(out.Messages) > 0 {
			messages := make([]*domain.AlertQueueMessage, len(out.Messages))
			for i := range out.Messages {
				messages[i] = toAlertQueueMessage(&out.Messages[i], requestedAt)
			}
			return messages, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
}

// Complete deletes the message.
func (q *Queue) Complete(ctx context.Context, message *domain.AlertQueueMessage) error {
	m, err := handleOf(message)
	if err != nil {
		return err
	}
	_, err = q.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{QueueUrl: aws.String(q.url), ReceiptHandle: m.ReceiptHandle})
	return err
}

// Abandon makes the message visible again at once.
func (q *Queue) Abandon(ctx context.Context, message *domain.AlertQueueMessage) error {
	return q.setVisibility(ctx, message, 0)
}

// RenewLock keeps the message hidden for another visibility timeout and records the new expiry.
func (q *Queue) RenewLock(ctx context.Context, message *domain.AlertQueueMessage) error {
	requestedAt := q.now()
	if err := q.setVisibility(ctx, message, visibilityTimeout); err != nil {
		return err
	}
	lockedUntil := requestedAt.Add(visibilityTimeout)
	message.LockedUntil = &lockedUntil
	return nil
}

// DeadLetter copies the message with its reason to the dead-letter queue and only then deletes it; without one, or when the copy fails, it releases the message to the redrive policy.
func (q *Queue) DeadLetter(ctx context.Context, message *domain.AlertQueueMessage, reason, description string) error {
	m, err := handleOf(message)
	if err != nil {
		return err
	}
	if q.deadLetterURL == "" {
		return q.Abandon(ctx, message)
	}
	attributes := map[string]types.MessageAttributeValue{
		attributeDeadLetterReason: stringAttribute(reason),
		attributeSourceQueue:      stringAttribute(q.name),
		attributeSourceMessageID:  stringAttribute(aws.ToString(m.MessageId)),
	}
	if description != "" {
		attributes[attributeDeadLetterDescription] = stringAttribute(description)
	}
	for key, value := range m.MessageAttributes {
		if len(attributes) >= maxMessageAttributes {
			break
		}
		if _, taken := attributes[key]; !taken {
			attributes[key] = value
		}
	}
	if _, err := q.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:          aws.String(q.deadLetterURL),
		MessageBody:       m.Body,
		MessageAttributes: attributes,
	}); err != nil {
		// A copy that never fits, such as a body near the size limit plus the reason attributes, must not loop, so the redrive policy takes the original.
		return errors.Join(fmt.Errorf("send to dead-letter queue: %w", err), q.Abandon(ctx, message))
	}
	return q.Complete(ctx, message)
}

func (q *Queue) setVisibility(ctx context.Context, message *domain.AlertQueueMessage, d time.Duration) error {
	m, err := handleOf(message)
	if err != nil {
		return err
	}
	_, err = q.client.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(q.url),
		ReceiptHandle:     m.ReceiptHandle,
		VisibilityTimeout: int32(d / time.Second),
	})
	return err
}

func toAlertQueueMessage(m *types.Message, requestedAt time.Time) *domain.AlertQueueMessage {
	properties := make(map[string]string, len(m.MessageAttributes))
	for key, value := range m.MessageAttributes {
		if value.StringValue != nil {
			properties[key] = strings.TrimSpace(*value.StringValue)
		}
	}
	message := &domain.AlertQueueMessage{
		ID:         aws.ToString(m.MessageId),
		Body:       []byte(aws.ToString(m.Body)),
		Properties: properties,
		Handle:     m,
	}
	if sent, err := strconv.ParseInt(m.Attributes[string(types.MessageSystemAttributeNameSentTimestamp)], 10, 64); err == nil {
		enqueuedOn := time.UnixMilli(sent).UTC()
		message.EnqueuedOn = &enqueuedOn
	}
	lockedUntil := requestedAt.Add(visibilityTimeout)
	message.LockedUntil = &lockedUntil
	return message
}

func stringAttribute(value string) types.MessageAttributeValue {
	return types.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(value)}
}

func handleOf(message *domain.AlertQueueMessage) (*types.Message, error) {
	m, ok := message.Handle.(*types.Message)
	if !ok || m == nil {
		return nil, errForeignMessage
	}
	return m, nil
}
