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

package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// Voicer is the subset of TwilioClient MultiChannelEscalator depends on.
type Voicer interface {
	Escalate(ctx context.Context, message string) error
}

// Messenger is the subset of GoogleChatClient MultiChannelEscalator depends
// on.
type Messenger interface {
	SendMessage(ctx context.Context, text string) error
}

// Mailer is the subset of EmailClient MultiChannelEscalator depends on.
type Mailer interface {
	SendEscalation(ctx context.Context, subject, htmlBody string) error
}

// MultiChannelEscalator fans a single escalation out to every configured
// channel — Twilio voice call, Google Chat message, email — independently.
// This is deliberate redundancy, not a first-success-wins race: the whole
// point of escalation is that CSM is unreachable, so the more independent
// ways SRE has a chance to notice, the better. A channel that isn't
// configured (its underlying client's zero-value posture — see
// TwilioClient/GoogleChatClient/EmailClient's own doc comments, all of
// which "never fail to construct," only to actually send) simply reports an
// error here like any other channel failure; it is not treated specially.
//
// Escalate reports success (nil) if *any* channel got through — internal/worker
// only needs to know whether it should keep pressing MarkEscalated's retry
// path, not force every channel to succeed. A channel that fails
// independently of the others is logged, not swallowed silently: see the
// per-channel warnings this emits regardless of the overall outcome.
type MultiChannelEscalator struct {
	twilio       Voicer
	chat         Messenger
	email        Mailer
	emailSubject string
}

// NewMultiChannelEscalator constructs a MultiChannelEscalator. Any of
// twilio/chat/email may be nil — a nil channel is skipped entirely (not
// attempted, not counted as a failure), for a deployment that has only
// configured some of the three. emailSubject is fixed rather than derived
// per call: every escalation email from this service carries the same
// subject line, with the actual per-alert detail in the body (the same
// message text every channel receives) and — for anyone scanning an inbox
// — a consistent, filterable subject.
func NewMultiChannelEscalator(twilio Voicer, chat Messenger, email Mailer, emailSubject string) *MultiChannelEscalator {
	return &MultiChannelEscalator{twilio: twilio, chat: chat, email: email, emailSubject: emailSubject}
}

// Escalate sends message to every configured channel. See the type's own
// doc comment for the fan-out/at-least-one-success contract.
func (m *MultiChannelEscalator) Escalate(ctx context.Context, message string) error {
	var errs []error
	attempted := 0
	succeeded := 0

	if m.twilio != nil {
		attempted++
		if err := m.twilio.Escalate(ctx, message); err != nil {
			slog.ErrorContext(ctx, "notifications: twilio escalation channel failed", "err", err)
			errs = append(errs, fmt.Errorf("twilio: %w", err))
		} else {
			succeeded++
		}
	}
	if m.chat != nil {
		attempted++
		if err := m.chat.SendMessage(ctx, message); err != nil {
			slog.ErrorContext(ctx, "notifications: google chat escalation channel failed", "err", err)
			errs = append(errs, fmt.Errorf("google chat: %w", err))
		} else {
			succeeded++
		}
	}
	if m.email != nil {
		attempted++
		if err := m.email.SendEscalation(ctx, m.emailSubject, message); err != nil {
			slog.ErrorContext(ctx, "notifications: email escalation channel failed", "err", err)
			errs = append(errs, fmt.Errorf("email: %w", err))
		} else {
			succeeded++
		}
	}

	if attempted == 0 {
		return fmt.Errorf("notifications: no escalation channel is configured")
	}
	if succeeded == 0 {
		return fmt.Errorf("notifications: every configured escalation channel failed: %w", errors.Join(errs...))
	}
	if len(errs) > 0 {
		// Partial failure is not this method's failure to report — at least
		// one channel got through, which is what internal/worker's own
		// MarkEscalated/retry-budget logic cares about — but it's worth
		// surfacing loudly here too, in case an operator is only watching
		// this service's own logs rather than the channels themselves.
		slog.WarnContext(ctx, "notifications: escalation partially failed, but at least one channel succeeded", "succeeded", succeeded, "attempted", attempted, "failures", errors.Join(errs...))
	}
	return nil
}
