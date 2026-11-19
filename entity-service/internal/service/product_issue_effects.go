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

package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// productIssueEffectsTimeout bounds the follow-up work after a product issue's
// GitHub state has been written: each step on the synchronous paths (reconcile,
// backfill), all steps of one delivery together on the webhook path.
const productIssueEffectsTimeout = 30 * time.Second

// webhookFollowUpSlots bounds the follow-ups of deliveries running at once, and
// webhookFollowUpWait is how long a delivery waits for a slot before its
// follow-up is dropped. A dropped or failed follow-up is repaired by the daily
// reconciliation, which converges every active linked issue.
const (
	webhookFollowUpSlots = 8
	webhookFollowUpWait  = time.Second
)

// productIssueEffects is what runs after a product issue's GitHub state changed:
// the SLA clocks and the case tag rule. Both are optional (nil: nothing runs) and
// best effort: an error is logged and never fails the delivery, the reconcile unit
// or the backfill item, whose own write has already committed. The methods here
// are synchronous (reconcile and backfill have their own time budget); the
// webhook path batches them into a webhookFollowUp instead.
type productIssueEffects struct {
	sla  WorkItemSLAEngine
	tags ProductIssueCaseTagRule
}

func (e productIssueEffects) run(ctx context.Context, workItemID, what string, f func(ctx context.Context) error) {
	ctx2, cancel := context.WithTimeout(context.WithoutCancel(ctx), productIssueEffectsTimeout)
	defer cancel()
	if err := f(ctx2); err != nil {
		slog.ErrorContext(ctx, "product issue: follow-up failed; the write stands", "workItemId", workItemID, "step", what, "err", err)
	}
}

func (e productIssueEffects) recalculate(ctx context.Context, id string) {
	if e.sla == nil {
		return
	}
	e.run(ctx, id, "sla recalculate", func(c context.Context) error { return e.sla.RecalculateWorkItemClocks(c, id) })
}

func (e productIssueEffects) register(ctx context.Context, id string) {
	if e.sla == nil {
		return
	}
	e.run(ctx, id, "sla register", func(c context.Context) error { return e.sla.RegisterWorkItemClocks(c, id) })
}

func (e productIssueEffects) tagRule(ctx context.Context, id string) {
	if e.tags == nil {
		return
	}
	e.run(ctx, id, "case tag rule", func(c context.Context) error { return e.tags.ApplyCaseTagRuleForWorkItem(c, id) })
}

// converge brings the issue's clocks and its case's tag in line with what is
// stored, whether or not anything changed. The clocks are rebuilt from the status
// history first, so an open clock of an issue that has since been resolved is
// completed rather than cancelled; then clocks of a policy that no longer applies
// (a changed priority) are cancelled, the applicable policy gets a back-dated
// clock where it has none, and everything is rebuilt once more.
func (e productIssueEffects) converge(ctx context.Context, id string, now time.Time) {
	if e.sla != nil {
		e.recalculate(ctx, id)
		e.run(ctx, id, "sla revise", func(c context.Context) error { return e.sla.ReviseWorkItemClocks(c, id, now) })
	}
	e.tagRule(ctx, id)
}

// followUps hands the webhook path's follow-ups to goroutines.
type followUps struct {
	inline bool // run in the caller (tests)
	sem    chan struct{}
	wg     sync.WaitGroup
}

func newFollowUps() *followUps { return &followUps{sem: make(chan struct{}, webhookFollowUpSlots)} }

// submit runs f after the delivery's write has committed. It never blocks the
// delivery for long and never delays shutdown: f runs on its own goroutine over a
// context that outlives the request, and nothing waits for it.
func (f *followUps) submit(ctx context.Context, workItemID string, run func(ctx context.Context)) {
	if f.inline {
		run(context.WithoutCancel(ctx))
		return
	}
	bg := context.WithoutCancel(ctx) // the request's values (the caller's identity), not its cancellation
	t := time.NewTimer(webhookFollowUpWait)
	defer t.Stop()
	select {
	case f.sem <- struct{}{}:
	case <-t.C:
		slog.Warn("product issue: too many follow-ups running; this one is dropped and left to the daily reconciliation", "workItemId", workItemID)
		return
	}
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		defer func() { <-f.sem }()
		run(bg)
	}()
}

// webhookFollowUp collects the follow-up steps of one delivery. They run in the
// order they were added (SLA steps, then the tag rule once) under one overall
// timeout.
type webhookFollowUp struct {
	fx    productIssueEffects
	id    string
	steps []func(ctx context.Context) error
	names []string
	tag   bool
}

func newWebhookFollowUp(fx productIssueEffects, id string) *webhookFollowUp {
	return &webhookFollowUp{fx: fx, id: id}
}

func (w *webhookFollowUp) add(name string, f func(ctx context.Context) error) {
	w.names, w.steps = append(w.names, name), append(w.steps, f)
}

// status applies a stored status event to the clocks.
func (w *webhookFollowUp) status(at time.Time, newest bool) {
	if w.fx.sla != nil {
		w.add("sla status effects", func(c context.Context) error { return w.fx.sla.ApplyWorkItemStatusEffects(c, w.id, at, newest) })
	}
	w.tag = true
}

func (w *webhookFollowUp) revise(at time.Time) {
	if w.fx.sla != nil {
		w.add("sla revise", func(c context.Context) error { return w.fx.sla.ReviseWorkItemClocks(c, w.id, at) })
	}
	w.tag = true
}

func (w *webhookFollowUp) recalculate() {
	if w.fx.sla != nil {
		w.add("sla recalculate", func(c context.Context) error { return w.fx.sla.RecalculateWorkItemClocks(c, w.id) })
	}
	w.tag = true
}

func (w *webhookFollowUp) tagOnly() { w.tag = true }

// dispatch runs the collected steps asynchronously (nothing collected: nothing runs).
func (w *webhookFollowUp) dispatch(ctx context.Context, f *followUps) {
	if w.fx.tags != nil && w.tag {
		w.add("case tag rule", func(c context.Context) error { return w.fx.tags.ApplyCaseTagRuleForWorkItem(c, w.id) })
	}
	if len(w.steps) == 0 {
		return
	}
	f.submit(ctx, w.id, func(ctx context.Context) {
		ctx, cancel := context.WithTimeout(ctx, productIssueEffectsTimeout)
		defer cancel()
		for i, step := range w.steps {
			if err := step(ctx); err != nil {
				slog.Error("product issue: follow-up failed; the write stands", "workItemId", w.id, "step", w.names[i], "err", err)
			}
		}
	})
}
