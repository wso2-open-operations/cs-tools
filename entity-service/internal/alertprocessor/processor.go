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

// Package alertprocessor turns stored alerts into incidents. Its subpackages split the work: event (the alert.created record), strategy (pure decisions), severity (label mapping), sink (incidents), relay (unpublished alerts to event hub) and subscriber (event hub to a handler, wrapped in middleware). This package holds the Processor and the ports it depends on.
package alertprocessor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/strategy"
)

// Processor decides each alert.created event with its strategy chain and, when Apply is set, carries the decision out. All its state is the alert table: an alert linked to an incident is done, and a fingerprint's group is read from its linked alerts.
type Processor struct {
	Store     Store
	Sink      Sink
	MuteRules MuteRules
	Chain     strategy.Chain
	// Lookback is how far back an alert looks for an earlier alert of its fingerprint whose incident was created but never linked.
	Lookback time.Duration
	// Apply writes incidents and notes; off, decisions are only logged (ALERTING_VERSION hybrid).
	Apply bool
}

// step is a decision to carry out: what to do with the alert, and the incident a fold adds it to.
type step struct {
	outcome    strategy.Outcome
	incidentID string
}

// Handle decides, writes to the sink, then links the alert. Each step is safe to repeat: Open finds the incident an alert already created and Fold skips a note already added, so a retry neither opens a second incident nor adds a second note.
func (p *Processor) Handle(ctx context.Context, e event.Event) error {
	todo, err := p.decide(ctx, e)
	if err != nil || todo == nil {
		return err
	}
	switch {
	case todo.outcome == strategy.Open:
		if todo.incidentID, err = p.Sink.Open(ctx, e); err != nil {
			return fmt.Errorf("open incident for %s: %w", e.AlertID, err)
		}
	case todo.outcome.Folds():
		if err := p.Sink.Fold(ctx, todo.incidentID, todo.outcome, e); err != nil {
			return fmt.Errorf("fold %s into incident %s: %w", e.AlertID, todo.incidentID, err)
		}
	}
	return p.Store.InGroup(ctx, e.Fingerprint(), func(tx GroupTx) error {
		return tx.LinkAlert(ctx, e.AlertID, todo.incidentID)
	})
}

// decide returns the step to carry out, or nil when there is none: the alert is already linked, the outcome writes nothing, or Apply is off.
func (p *Processor) decide(ctx context.Context, e event.Event) (*step, error) {
	var todo *step
	err := p.Store.InGroup(ctx, e.Fingerprint(), func(tx GroupTx) error {
		linked, err := tx.AlertLinked(ctx, e.AlertID)
		if err != nil || linked {
			return err
		}
		if p.Apply {
			if err := p.linkUnrecorded(ctx, tx, e); err != nil {
				return err
			}
		}
		in, err := p.input(ctx, tx, e)
		if err != nil {
			return err
		}
		d := p.Chain.Decide(in)
		slog.InfoContext(ctx, "alert processor: decided", "alertId", e.AlertID, "fingerprint", e.Fingerprint(),
			"outcome", d.Outcome, "strategy", d.Strategy, "reason", d.Reason, "apply", p.Apply)
		switch {
		case !p.Apply:
		case d.Outcome == strategy.Open:
			todo = &step{outcome: strategy.Open}
		case d.Outcome.Folds():
			todo = &step{outcome: d.Outcome, incidentID: in.Group.IncidentID}
		}
		return nil
	})
	return todo, err
}

// linkUnrecorded links earlier alerts of the fingerprint that created an incident but failed before linking it, so this alert folds into that incident instead of opening a second one.
func (p *Processor) linkUnrecorded(ctx context.Context, tx GroupTx, e event.Event) error {
	earlier, err := tx.Unlinked(ctx, e.ReceivedOn.Add(-p.Lookback), e.ReceivedOn, e.AlertID)
	if err != nil {
		return err
	}
	for _, alertID := range earlier {
		incidentID, err := p.Sink.Find(ctx, alertID)
		if err != nil {
			return err
		}
		if incidentID == "" {
			continue
		}
		if err := tx.LinkAlert(ctx, alertID, incidentID); err != nil {
			return err
		}
	}
	return nil
}

// input gathers what the strategies look at: the group, whether its incident is open, and the active mute rules.
func (p *Processor) input(ctx context.Context, tx GroupTx, e event.Event) (strategy.Input, error) {
	in := strategy.Input{Event: e}
	group, err := tx.Group(ctx)
	if err != nil {
		return strategy.Input{}, err
	}
	if group != nil {
		in.Group = group
		if in.IncidentOpen, err = p.Sink.IsOpen(ctx, group.IncidentID); err != nil {
			return strategy.Input{}, err
		}
	}
	// Muting fails open: without the rules, an alert still pages rather than being silenced by mistake.
	rules, err := p.MuteRules(ctx, e.ReceivedOn)
	if err != nil {
		slog.WarnContext(ctx, "alert processor: mute rules unavailable, muting nothing", "alertId", e.AlertID, "err", err)
	}
	in.MuteRules = rules
	return in, nil
}
