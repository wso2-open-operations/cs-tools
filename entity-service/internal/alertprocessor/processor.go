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

// step is a decision to carry out: what to do with the alert and the group it acts on.
type step struct {
	outcome strategy.Outcome
	group   strategy.Group
}

// Handle processes one event in three steps: decide (and record a pending open), write to the sink, record the result. Each step is safe to repeat, so a retry neither opens a second incident nor adds a second note.
func (p *Processor) Handle(ctx context.Context, e event.Event) error {
	todo, err := p.decide(ctx, e)
	if err != nil || todo == nil {
		return err
	}
	switch {
	case todo.outcome == strategy.Open:
		if todo.group.IncidentID, err = p.Sink.Open(ctx, e); err != nil {
			return fmt.Errorf("open incident for %s: %w", e.AlertID, err)
		}
		todo.group.OpeningAlertID = ""
	case todo.outcome.Folds():
		if err := p.Sink.Fold(ctx, todo.group.IncidentID, todo.outcome, e); err != nil {
			return fmt.Errorf("fold %s into incident %s: %w", e.AlertID, todo.group.IncidentID, err)
		}
	}
	return p.record(ctx, e, todo)
}

// decide returns the step to carry out, or nil when there is none: the alert is already linked, the outcome writes nothing, or Apply is off.
func (p *Processor) decide(ctx context.Context, e event.Event) (*step, error) {
	var todo *step
	err := p.Store.InGroup(ctx, e.Fingerprint(), func(tx GroupTx) error {
		linked, err := tx.AlertLinked(ctx, e.AlertID)
		if err != nil || linked {
			return err
		}
		group, err := tx.Group(ctx)
		if err != nil {
			return err
		}
		if group, err = p.finishOpen(ctx, tx, group, e.AlertID); err != nil {
			return err
		}
		if group != nil && group.OpeningAlertID == e.AlertID {
			// A retry of this alert's own open: carry on with it rather than deciding again.
			todo = &step{outcome: strategy.Open, group: *group}
			return nil
		}
		in, err := p.input(ctx, group, e)
		if err != nil {
			return err
		}
		d := p.Chain.Decide(in)
		slog.InfoContext(ctx, "alert processor: decided", "alertId", e.AlertID, "fingerprint", e.Fingerprint(),
			"outcome", d.Outcome, "strategy", d.Strategy, "reason", d.Reason, "apply", p.Apply)
		switch {
		case !p.Apply:
			return nil
		case d.Outcome == strategy.Open:
			// Recorded before the incident is created, so a later alert finds this open instead of starting another.
			pending := strategy.Group{Fingerprint: e.Fingerprint(), OpeningAlertID: e.AlertID, FirstSeenOn: e.ReceivedOn, LastSeenOn: e.ReceivedOn, AlertCount: 1}
			todo = &step{outcome: strategy.Open, group: pending}
			return tx.SaveGroup(ctx, pending)
		case d.Outcome.Folds():
			todo = &step{outcome: d.Outcome, group: *in.Group}
		}
		return nil
	})
	return todo, err
}

// finishOpen completes another alert's open that never recorded its incident: the incident it created is linked, or, when it created none, the group is dropped so this alert decides afresh.
func (p *Processor) finishOpen(ctx context.Context, tx GroupTx, group *strategy.Group, alertID string) (*strategy.Group, error) {
	if group == nil || group.IncidentID != "" || group.OpeningAlertID == "" || group.OpeningAlertID == alertID {
		return group, nil
	}
	incidentID, err := p.Sink.Find(ctx, group.OpeningAlertID)
	if err != nil || incidentID == "" {
		return nil, err
	}
	opener := group.OpeningAlertID
	group.IncidentID, group.OpeningAlertID = incidentID, ""
	if err := tx.SaveGroup(ctx, *group); err != nil {
		return nil, err
	}
	return group, tx.LinkAlert(ctx, opener, incidentID)
}

// input gathers what the strategies look at: the group, whether its incident is open, and the active mute rules.
func (p *Processor) input(ctx context.Context, group *strategy.Group, e event.Event) (strategy.Input, error) {
	in := strategy.Input{Event: e}
	if group != nil && group.IncidentID != "" {
		in.Group = group
		var err error
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

// record saves the group and links the alert to its incident, which marks the event done.
func (p *Processor) record(ctx context.Context, e event.Event, todo *step) error {
	return p.Store.InGroup(ctx, e.Fingerprint(), func(tx GroupTx) error {
		group := todo.group
		if todo.outcome.Folds() {
			current, err := tx.Group(ctx)
			if err != nil {
				return err
			}
			if current != nil && current.IncidentID == group.IncidentID {
				group = *current
			}
			group.AlertCount++
			if e.ReceivedOn.After(group.LastSeenOn) {
				group.LastSeenOn = e.ReceivedOn
			}
		}
		if err := tx.SaveGroup(ctx, group); err != nil {
			return err
		}
		return tx.LinkAlert(ctx, e.AlertID, group.IncidentID)
	})
}
