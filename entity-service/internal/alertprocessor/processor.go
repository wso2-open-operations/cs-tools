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

// Package alertprocessor turns stored alerts into incidents. Its subpackages split the work: event (the alert.created record), strategy (pure decisions), severity (label mapping), sink (incidents), relay (outbox to event hub) and subscriber (event hub to a handler, wrapped in middleware). This package holds the Processor and the ports it depends on.
package alertprocessor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/strategy"
)

// Processor decides each alert.created event with its strategy chain and, when Apply is set, carries the decision out.
type Processor struct {
	Store     Store
	Sink      Sink
	MuteRules MuteRules
	Chain     strategy.Chain
	// Apply writes incidents and notes; off, decisions are only logged (ALERTING_VERSION hybrid).
	Apply bool
}

// Handle processes one event; an error leaves the event to be retried, and a retry never opens a second incident.
func (p *Processor) Handle(ctx context.Context, e event.Event) error {
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
		d := p.Chain.Decide(in)
		slog.InfoContext(ctx, "alert processor: decided", "alertId", e.AlertID, "fingerprint", fingerprint,
			"outcome", d.Outcome, "strategy", d.Strategy, "reason", d.Reason, "apply", p.Apply)
		if !p.Apply {
			return nil
		}
		return p.carryOut(ctx, tx, in, d.Outcome)
	})
}

// input gathers what the strategies look at: the group, whether its incident is open, and the active mute rules.
func (p *Processor) input(ctx context.Context, tx GroupTx, e event.Event) (strategy.Input, error) {
	in := strategy.Input{Event: e}
	group, err := tx.Group(ctx)
	if err != nil {
		return strategy.Input{}, err
	}
	if group != nil && group.IncidentID != "" {
		in.Group = group
		if in.IncidentOpen, err = p.Sink.IsOpen(ctx, group.IncidentID); err != nil {
			return strategy.Input{}, err
		}
	}
	// Muting fails open: without the rules, an alert still pages rather than being silenced by mistake.
	if in.MuteRules, err = p.MuteRules(ctx, e.ReceivedOn); err != nil {
		slog.WarnContext(ctx, "alert processor: mute rules unavailable, muting nothing", "alertId", e.AlertID, "err", err)
	}
	return in, nil
}

func (p *Processor) carryOut(ctx context.Context, tx GroupTx, in strategy.Input, outcome strategy.Outcome) error {
	switch {
	case outcome == strategy.Open:
		return p.open(ctx, tx, in.Event)
	case outcome.Folds():
		return p.fold(ctx, tx, *in.Group, outcome, in.Event)
	default:
		return nil
	}
}

func (p *Processor) open(ctx context.Context, tx GroupTx, e event.Event) error {
	incidentID, err := p.Sink.Open(ctx, e)
	if err != nil {
		return fmt.Errorf("open incident for %s: %w", e.AlertID, err)
	}
	group := strategy.Group{Fingerprint: e.Fingerprint(), IncidentID: incidentID, FirstSeenOn: e.ReceivedOn, LastSeenOn: e.ReceivedOn, AlertCount: 1}
	return p.save(ctx, tx, group, e.AlertID)
}

func (p *Processor) fold(ctx context.Context, tx GroupTx, group strategy.Group, outcome strategy.Outcome, e event.Event) error {
	if err := p.Sink.Fold(ctx, group.IncidentID, outcome, e); err != nil {
		return fmt.Errorf("fold %s into incident %s: %w", e.AlertID, group.IncidentID, err)
	}
	group.AlertCount++
	if e.ReceivedOn.After(group.LastSeenOn) {
		group.LastSeenOn = e.ReceivedOn
	}
	return p.save(ctx, tx, group, e.AlertID)
}

func (p *Processor) save(ctx context.Context, tx GroupTx, group strategy.Group, alertID string) error {
	if err := tx.SaveGroup(ctx, group); err != nil {
		return err
	}
	return tx.LinkAlert(ctx, alertID, group.IncidentID)
}
