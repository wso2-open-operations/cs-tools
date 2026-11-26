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

package alertprocessor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Store keeps each fingerprint's group and each alert's incident.
type Store interface {
	// InGroup runs fn in one transaction that holds the fingerprint's lock, so one fingerprint is decided by one worker at a time.
	InGroup(ctx context.Context, fingerprint string, fn func(GroupTx) error) error
}

// GroupTx is the transaction InGroup hands to its function.
type GroupTx interface {
	// Group is the fingerprint's group, or nil when it has none.
	Group(ctx context.Context) (*Group, error)
	SaveGroup(ctx context.Context, g Group) error
	// AlertLinked reports whether the alert already belongs to an incident, which makes a redelivered event a no-op.
	AlertLinked(ctx context.Context, alertID string) (bool, error)
	LinkAlert(ctx context.Context, alertID, incidentID string) error
}

// Sink is where incidents live.
type Sink interface {
	// Open creates the alert's incident and returns its id; it returns the same incident when called again for the same alert.
	Open(ctx context.Context, e Event) (string, error)
	// IsOpen reports whether the incident is still open.
	IsOpen(ctx context.Context, incidentID string) (bool, error)
	// Note adds an HTML work note to the incident.
	Note(ctx context.Context, incidentID, html string) error
}

// MuteRules returns the mute rules active at now.
type MuteRules func(ctx context.Context, now time.Time) ([]domain.AlertMuteRule, error)

// Processor decides each alert.created event with its strategy chain and, when Apply is set, carries the decision out.
type Processor struct {
	Store     Store
	Sink      Sink
	MuteRules MuteRules
	Chain     []Strategy
	// Apply writes incidents and notes; off, decisions are only logged (ALERTING_VERSION hybrid).
	Apply bool
}

// Handle processes one event; an error leaves the event to be retried, and a retry never opens a second incident.
func (p *Processor) Handle(ctx context.Context, e Event) error {
	fingerprint := e.Fingerprint()
	return p.Store.InGroup(ctx, fingerprint, func(tx GroupTx) error {
		linked, err := tx.AlertLinked(ctx, e.AlertID)
		if err != nil || linked {
			return err
		}
		in, err := p.input(ctx, tx, e)
		if err != nil {
			return err
		}
		d := Decide(p.Chain, in)
		slog.InfoContext(ctx, "alert processor: decided", "alertId", e.AlertID, "fingerprint", fingerprint,
			"outcome", d.Outcome, "strategy", d.Strategy, "reason", d.Reason, "apply", p.Apply)
		if !p.Apply {
			return nil
		}
		return p.carryOut(ctx, tx, in, d)
	})
}

func (p *Processor) input(ctx context.Context, tx GroupTx, e Event) (Input, error) {
	in := Input{Event: e}
	group, err := tx.Group(ctx)
	if err != nil {
		return Input{}, err
	}
	if group != nil && group.IncidentID != "" {
		in.Group = group
		if in.IncidentOpen, err = p.Sink.IsOpen(ctx, group.IncidentID); err != nil {
			return Input{}, err
		}
	}
	// Muting fails open: without the rules, an alert still pages rather than being silenced by mistake.
	if in.MuteRules, err = p.MuteRules(ctx, e.ReceivedOn); err != nil {
		slog.WarnContext(ctx, "alert processor: mute rules unavailable, muting nothing", "alertId", e.AlertID, "err", err)
	}
	return in, nil
}

func (p *Processor) carryOut(ctx context.Context, tx GroupTx, in Input, d Decision) error {
	e := in.Event
	switch d.Outcome {
	case OutcomeOpen:
		incidentID, err := p.Sink.Open(ctx, e)
		if err != nil {
			return fmt.Errorf("open incident for %s: %w", e.AlertID, err)
		}
		group := Group{Fingerprint: e.Fingerprint(), IncidentID: incidentID, FirstSeenOn: e.ReceivedOn, LastSeenOn: e.ReceivedOn, AlertCount: 1}
		if err := tx.SaveGroup(ctx, group); err != nil {
			return err
		}
		return tx.LinkAlert(ctx, e.AlertID, incidentID)
	case OutcomeDuplicate, OutcomeResolve:
		group := *in.Group
		if err := p.Sink.Note(ctx, group.IncidentID, foldNote(d.Outcome, e)); err != nil {
			return fmt.Errorf("note %s on incident %s: %w", e.AlertID, group.IncidentID, err)
		}
		group.AlertCount++
		if e.ReceivedOn.After(group.LastSeenOn) {
			group.LastSeenOn = e.ReceivedOn
		}
		if err := tx.SaveGroup(ctx, group); err != nil {
			return err
		}
		return tx.LinkAlert(ctx, e.AlertID, group.IncidentID)
	default:
		return nil
	}
}
