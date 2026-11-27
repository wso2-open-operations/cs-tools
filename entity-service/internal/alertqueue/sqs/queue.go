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

// Package sqs receives SRE alerts from AWS SQS standard queues, for the alert consumer.
package sqs

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertqueue"
)

// Long poll and visibility: a Receive waits up to waitSeconds; a received message is hidden visibilitySeconds and its
// visibility is extended every lockRenewInterval while it is held.
const (
	waitSeconds       = 20
	visibilitySeconds = 60
	lockRenewInterval = 20 * time.Second
)

// Dead-letter message attributes: why the consumer gave up on a message.
const (
	deadLetterReasonAttribute      = "dead-letter-reason"
	deadLetterDescriptionAttribute = "dead-letter-description"
)

// Config signs in to SQS.
type Config struct {
	// Region is the AWS region of every queue.
	Region string
	// AccessKeyID and SecretAccessKey are an IAM user's keys; both empty uses the default AWS credential chain.
	AccessKeyID     string
	SecretAccessKey string
}

// NewClient returns an SQS client. AWS_ENDPOINT_URL, when set, points it at a local stand-in such as LocalStack.
func NewClient(ctx context.Context, cfg Config) (*awssqs.Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(cfg.Region)}
	if cfg.AccessKeyID != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("sqs: load aws config: %w", err)
	}
	return awssqs.NewFromConfig(awsCfg), nil
}

// RegionOf reads the region from an SQS queue URL (https://sqs.<region>.amazonaws.com/... or
// https://<region>.queue.amazonaws.com/...); empty when the URL names none.
func RegionOf(queueURL string) string {
	u, err := url.Parse(queueURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	parts := strings.Split(host, ".")
	switch {
	case len(parts) >= 3 && parts[0] == "sqs":
		return parts[1]
	case len(parts) >= 3 && parts[1] == "queue":
		return parts[0]
	}
	return ""
}

// API is the part of the SQS client a Queue uses; *sqs.Client satisfies it.
type API interface {
	ReceiveMessage(ctx context.Context, in *awssqs.ReceiveMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, in *awssqs.DeleteMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.DeleteMessageOutput, error)
	ChangeMessageVisibility(ctx context.Context, in *awssqs.ChangeMessageVisibilityInput, optFns ...func(*awssqs.Options)) (*awssqs.ChangeMessageVisibilityOutput, error)
	SendMessage(ctx context.Context, in *awssqs.SendMessageInput, optFns ...func(*awssqs.Options)) (*awssqs.SendMessageOutput, error)
}

// Queue is one SQS standard queue.
type Queue struct {
	name          string
	url           string
	deadLetterURL string
	api           API
}

var _ alertqueue.Queue = (*Queue)(nil)

// NewQueue returns the queue at queueURL. deadLetterURL, when set, is where DeadLetter moves a message; empty leaves it
// to the queue's redrive policy.
func NewQueue(api API, queueURL, deadLetterURL string) *Queue {
	name := path.Base(strings.TrimRight(queueURL, "/"))
	return &Queue{name: name, url: queueURL, deadLetterURL: deadLetterURL, api: api}
}

// Name is the queue's name, the last part of its URL.
func (q *Queue) Name() string { return q.name }

// Receive long-polls up to waitSeconds and returns up to max messages (SQS gives at most 10).
func (q *Queue) Receive(ctx context.Context, limit int) ([]alertqueue.Message, error) {
	out, err := q.api.ReceiveMessage(ctx, &awssqs.ReceiveMessageInput{
		QueueUrl:                    aws.String(q.url),
		MaxNumberOfMessages:         int32(min(max(limit, 1), 10)), // #nosec G115 -- between 1 and 10
		WaitTimeSeconds:             waitSeconds,
		VisibilityTimeout:           visibilitySeconds,
		MessageAttributeNames:       []string{"All"},
		MessageSystemAttributeNames: []types.MessageSystemAttributeName{types.MessageSystemAttributeNameSentTimestamp},
	})
	if err != nil {
		return nil, fmt.Errorf("sqs: receive from %s: %w", q.name, err)
	}
	msgs := make([]alertqueue.Message, 0, len(out.Messages))
	for _, m := range out.Messages {
		msgs = append(msgs, message{m})
	}
	return msgs, nil
}

// Complete deletes the message.
func (q *Queue) Complete(ctx context.Context, m alertqueue.Message) error {
	_, err := q.api.DeleteMessage(ctx, &awssqs.DeleteMessageInput{QueueUrl: aws.String(q.url), ReceiptHandle: raw(m).ReceiptHandle})
	return err
}

// Abandon makes the message visible again at once; SQS counts the receive and the redrive policy moves it after the
// maximum receive count.
func (q *Queue) Abandon(ctx context.Context, m alertqueue.Message) error {
	return q.visibility(ctx, m, 0)
}

// RenewLock keeps the message hidden for another visibility timeout.
func (q *Queue) RenewLock(ctx context.Context, m alertqueue.Message) error {
	return q.visibility(ctx, m, visibilitySeconds)
}

func (q *Queue) visibility(ctx context.Context, m alertqueue.Message, seconds int32) error {
	_, err := q.api.ChangeMessageVisibility(ctx, &awssqs.ChangeMessageVisibilityInput{
		QueueUrl: aws.String(q.url), ReceiptHandle: raw(m).ReceiptHandle, VisibilityTimeout: seconds,
	})
	return err
}

// DeadLetter sends the message, with its attributes and the reason, to the dead-letter queue, then deletes it. With
// no dead-letter queue configured it gives the message back, so the queue's redrive policy moves it.
func (q *Queue) DeadLetter(ctx context.Context, m alertqueue.Message, reason, description string) error {
	if q.deadLetterURL == "" {
		return q.Abandon(ctx, m)
	}
	r := raw(m)
	attrs := map[string]types.MessageAttributeValue{}
	for k, v := range r.MessageAttributes {
		attrs[k] = v
	}
	attrs[deadLetterReasonAttribute] = types.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(reason)}
	if description != "" {
		attrs[deadLetterDescriptionAttribute] = types.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(description)}
	}
	if _, err := q.api.SendMessage(ctx, &awssqs.SendMessageInput{QueueUrl: aws.String(q.deadLetterURL), MessageBody: r.Body, MessageAttributes: attrs}); err != nil {
		return fmt.Errorf("sqs: send to the dead-letter queue: %w", err)
	}
	return q.Complete(ctx, m)
}

// LockRenewInterval is how often a held message's visibility is extended.
func (q *Queue) LockRenewInterval() time.Duration { return lockRenewInterval }

// Close does nothing: the SQS client holds no connection of its own.
func (q *Queue) Close(context.Context) error { return nil }

type message struct {
	m types.Message
}

func raw(m alertqueue.Message) types.Message {
	return m.(message).m
}

func (m message) ID() string { return aws.ToString(m.m.MessageId) }

func (m message) Body() []byte {
	if m.m.Body == nil {
		return nil
	}
	return []byte(*m.m.Body)
}

func (m message) ContentType() string {
	for _, k := range []string{"content-type", "Content-Type", "contentType"} {
		if v, ok := m.m.MessageAttributes[k]; ok {
			return aws.ToString(v.StringValue)
		}
	}
	return ""
}

func (m message) Source() string {
	if v, ok := m.m.MessageAttributes[alertqueue.SourceProperty]; ok {
		return aws.ToString(v.StringValue)
	}
	return ""
}

func (m message) EnqueuedOn() time.Time {
	ms, err := strconv.ParseInt(m.m.Attributes[string(types.MessageSystemAttributeNameSentTimestamp)], 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}
