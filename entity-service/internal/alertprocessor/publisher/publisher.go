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

// Package publisher sends alert.created for every stored alert. The consumer calls it after storing a message and before acknowledging it, so an alert is never acknowledged unpublished.
package publisher

import (
	"context"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// MaxAttempts bounds how often an event processing gave up on is published again, so one poison alert cannot cycle forever.
const MaxAttempts = 10

// Producer sends one record to the event hub; *eventbus.Producer is one.
type Producer interface {
	Publish(ctx context.Context, key, value []byte) error
}

// Publisher publishes alert.created records, keyed by fingerprint so one fingerprint's alerts stay on one partition, in order.
type Publisher struct {
	Producer Producer
}

// Publish sends one event per stored alert row, in order, skipping error rows; it stops at the first failure so the caller leaves the message to be redelivered.
func (p Publisher) Publish(ctx context.Context, rows []domain.NewAlert) error {
	for _, row := range rows {
		e, ok, err := event.FromRow(row)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if err := p.send(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

// Republish sends an event processing gave up on once more, with its attempt raised; false means it already reached MaxAttempts and was not sent.
func (p Publisher) Republish(ctx context.Context, e event.Event) (bool, error) {
	if e.Attempt+1 >= MaxAttempts {
		return false, nil
	}
	e.Attempt++
	return true, p.send(ctx, e)
}

func (p Publisher) send(ctx context.Context, e event.Event) error {
	key, value, err := event.Encode(e)
	if err != nil {
		return err
	}
	if err := p.Producer.Publish(ctx, key, value); err != nil {
		return fmt.Errorf("publish alert %s: %w", e.AlertID, err)
	}
	return nil
}
