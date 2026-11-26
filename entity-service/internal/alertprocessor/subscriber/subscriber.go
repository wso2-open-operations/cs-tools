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

// Package subscriber reads alert.created records from the event hub and hands each event to a Handler wrapped in Middleware, so cross-cutting behaviour (recovery, logging, metrics later) stays out of the processor.
package subscriber

import (
	"context"
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/eventbus"
)

// Handler handles one alert.created event; an error retries it.
type Handler func(ctx context.Context, e event.Event) error

// Source delivers records to a handle function until ctx ends; *eventbus.Consumer is one.
type Source interface {
	Run(ctx context.Context, handle eventbus.Handle, onExhausted eventbus.OnExhausted)
}

// Exhausted is called with an event every retry failed on, before its offset is committed; it should keep the event somewhere durable, such as publishing it again.
type Exhausted func(ctx context.Context, e event.Event, cause error) error

// Subscriber connects a Source to a Handler.
type Subscriber struct {
	Source  Source
	Handler Handler
	// OnExhausted, when set, receives every event the retries gave up on; without it such an event is dropped.
	OnExhausted Exhausted
}

// New returns a Subscriber whose handler is wrapped in mw, the first listed outermost.
func New(source Source, h Handler, mw ...Middleware) *Subscriber {
	return &Subscriber{Source: source, Handler: Wrap(h, mw...)}
}

// Run delivers records until ctx ends.
func (s *Subscriber) Run(ctx context.Context) {
	var onExhausted eventbus.OnExhausted
	if s.OnExhausted != nil {
		onExhausted = s.exhausted
	}
	s.Source.Run(ctx, s.handle, onExhausted)
}

// exhausted hands a decodable record the retries gave up on to OnExhausted.
func (s *Subscriber) exhausted(ctx context.Context, record eventbus.Record, cause error) error {
	e, ok, err := event.Decode(record.Value)
	if err != nil || !ok {
		return err
	}
	return s.OnExhausted(ctx, e, cause)
}

// handle decodes a record; other event types and records that can never decode are skipped rather than retried.
func (s *Subscriber) handle(ctx context.Context, record eventbus.Record) error {
	e, ok, err := event.Decode(record.Value)
	if err != nil {
		slog.ErrorContext(ctx, "alert subscriber: undecodable record skipped", "topic", record.Topic, "offset", record.Offset, "err", err)
		return nil
	}
	if !ok {
		return nil
	}
	return s.Handler(ctx, e)
}
