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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// Apply implements SalesforceResyncService. It re-ingests the requested
// components from Salesforce, parents before children, through the ingest's
// own per-record functions on a context that bypasses the duplicate-event
// guard, deletes nothing, publishes no invitation event and starts no side
// refresh (withResyncApply). A component is read from Salesforce completely
// before any of its rows is written, so a call-budget cap skips a component
// whole rather than leaving it half written.
func (s *salesforceResyncService) Apply(ctx context.Context, req domain.SalesforceResyncApplyRequest) (domain.SalesforceResyncApplyResponse, error) {
	if err := s.requireAdmin(ctx); err != nil {
		return domain.SalesforceResyncApplyResponse{}, err
	}
	defs, err := resyncComponentsOf(req.Scope)
	if err != nil {
		return domain.SalesforceResyncApplyResponse{}, err
	}
	valid := map[string]bool{}
	for _, d := range defs {
		valid[d.key] = true
	}
	selected := map[string]bool{}
	for _, k := range req.Components {
		k = strings.TrimSpace(k)
		if !valid[k] {
			return domain.SalesforceResyncApplyResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("component %q is not part of the %s scope", k, req.Scope)}
		}
		selected[k] = true
	}
	if len(selected) == 0 {
		return domain.SalesforceResyncApplyResponse{}, &apierror.ValidationError{Msg: "components must name at least one component"}
	}
	run, err := s.newRun(ctx, req.Scope, req.ID)
	if err != nil {
		return domain.SalesforceResyncApplyResponse{}, err
	}
	release, err := s.lock(req.Scope, req.ID)
	if err != nil {
		return domain.SalesforceResyncApplyResponse{}, err
	}
	defer release()

	applyCtx, stale := withResyncApply(ctx)
	out := domain.SalesforceResyncApplyResponse{Components: []domain.SalesforceResyncApplyComponent{}}
	for _, def := range defs {
		if !selected[def.key] {
			continue
		}
		if reason := s.disabledReason(def.key); reason != "" {
			out.Components = append(out.Components, skippedApplyComponent(def.key, reason))
			continue
		}
		res, err := run.apply(applyCtx, stale, def)
		if err != nil {
			out.Components = append(out.Components, skippedApplyComponent(def.key, run.skipReason(ctx, err)))
			continue
		}
		out.Components = append(out.Components, res)
	}
	return out, nil
}

func skippedApplyComponent(key, reason string) domain.SalesforceResyncApplyComponent {
	return domain.SalesforceResyncApplyComponent{
		Key: key, Status: domain.SalesforceResyncApplySkipped, SkippedReason: &reason,
		Errors: []domain.SalesforceResyncApplyError{}, SkippedStaleSfIDs: []string{},
	}
}

// applyTally accumulates one component's outcome.
type applyTally struct {
	comp  domain.SalesforceResyncApplyComponent
	stale *resyncStale
	// seen is how many stale ids were already folded into comp.
	seen int
}

func newApplyTally(key string, stale *resyncStale) *applyTally {
	return &applyTally{stale: stale, comp: domain.SalesforceResyncApplyComponent{
		Key: key, Errors: []domain.SalesforceResyncApplyError{}, SkippedStaleSfIDs: []string{},
	}}
}

// record counts one record's write; a failure keeps its (safe) message.
func (t *applyTally) record(ctx context.Context, sfID string, err error) {
	if err == nil {
		// A record the guard left alone (it changed in Salesforce since the
		// snapshot was read) is reported as such, not as written.
		if t.stale != nil && len(t.stale.ids) > t.seen {
			for _, id := range t.stale.ids[t.seen:] {
				t.comp.SkippedStale++
				if len(t.comp.SkippedStaleSfIDs) < resyncMaxApplyErrors {
					t.comp.SkippedStaleSfIDs = append(t.comp.SkippedStaleSfIDs, id)
				}
			}
			t.seen = len(t.stale.ids)
			return
		}
		t.comp.Succeeded++
		return
	}
	t.comp.Failed++
	slog.WarnContext(ctx, "salesforce resync: record could not be written", "component", t.comp.Key, "sfId", sfID, "err", err)
	if len(t.comp.Errors) < resyncMaxApplyErrors {
		t.comp.Errors = append(t.comp.Errors, domain.SalesforceResyncApplyError{SfID: sfID, Message: resyncSafeMessage(err)})
	}
}

// each runs write for every item until the request's context ends; the
// remaining items are then counted as failed (not written).
func applyEach[T any](ctx context.Context, t *applyTally, items []T, idOf func(T) string, write func(T) error) {
	for i, it := range items {
		if err := ctx.Err(); err != nil {
			for _, rest := range items[i:] {
				t.record(ctx, idOf(rest), err)
			}
			return
		}
		t.record(ctx, idOf(it), write(it))
	}
}

func (t *applyTally) finish() domain.SalesforceResyncApplyComponent {
	t.comp.Status = domain.SalesforceResyncApplyApplied
	if t.comp.Failed > 0 {
		t.comp.Status = domain.SalesforceResyncApplyFailed
	}
	return t.comp
}

func (r *resyncRun) apply(ctx context.Context, stale *resyncStale, def resyncComponentDef) (domain.SalesforceResyncApplyComponent, error) {
	ev := r.s.ev
	t := newApplyTally(def.key, stale)
	switch def.key {
	case domain.SalesforceResyncComponentAccount:
		cust, err := r.sfAccount(ctx)
		if err != nil {
			return domain.SalesforceResyncApplyComponent{}, err
		}
		t.record(ctx, r.target.SfID, ev.upsertAccountRecord(ctx, cust, r.target.SfID, domain.SalesforceEventUpdated, true))

	case domain.SalesforceResyncComponentPartners:
		if err := r.budget.spend(ctx); err != nil {
			return domain.SalesforceResyncApplyComponent{}, err
		}
		res, err := ev.RefreshPartners(ctx, r.target.SfID)
		if err == nil && res.Disabled {
			return skippedApplyComponent(def.key, "partner ingest is disabled"), nil
		}
		t.record(ctx, r.target.SfID, err)

	case domain.SalesforceResyncComponentProjects, domain.SalesforceResyncComponentProject:
		ps, err := r.sfProjects(ctx)
		if err != nil {
			return domain.SalesforceResyncApplyComponent{}, err
		}
		applyEach(ctx, t, ps, func(p salesentity.Project) string { return strings.TrimSpace(p.ID) }, func(p salesentity.Project) error {
			return ev.upsertProjectRecord(ctx, p, strings.TrimSpace(p.ID), domain.SalesforceEventUpdated, true)
		})

	case domain.SalesforceResyncComponentOpportunities:
		opps, err := r.sfOpportunities(ctx)
		if err != nil {
			return domain.SalesforceResyncApplyComponent{}, err
		}
		applyEach(ctx, t, opps, func(o salesentity.Opportunity) string { return strings.TrimSpace(o.ID) }, func(o salesentity.Opportunity) error {
			return ev.ingestOpportunityRecord(ctx, o, strings.TrimSpace(o.ID), domain.SalesforceEventUpdated, true)
		})

	case domain.SalesforceResyncComponentLinkedOpportunities:
		links, err := r.sfLinks(ctx)
		if err != nil {
			return domain.SalesforceResyncApplyComponent{}, err
		}
		applyEach(ctx, t, links, func(l salesentity.LinkedOpportunity) string { return strings.TrimSpace(l.ID) }, func(l salesentity.LinkedOpportunity) error {
			return ev.ingestLinkedOpportunityRecord(ctx, l, strings.TrimSpace(l.ID), domain.SalesforceEventUpdated)
		})

	case domain.SalesforceResyncComponentInvoices:
		invs, err := r.sfInvoices(ctx)
		if err != nil {
			return domain.SalesforceResyncApplyComponent{}, err
		}
		applyEach(ctx, t, invs, func(i salesentity.Invoice) string { return strings.TrimSpace(derefString(i.ID)) }, func(i salesentity.Invoice) error {
			return ev.ingestInvoiceRecord(ctx, i, strings.TrimSpace(derefString(i.ID)), domain.SalesforceEventUpdated)
		})

	case domain.SalesforceResyncComponentContacts:
		cs, err := r.sfContacts(ctx)
		if err != nil {
			return domain.SalesforceResyncApplyComponent{}, err
		}
		// Only the contact half: the memberships are a component of their own.
		applyEach(ctx, t, cs, func(c salesentity.Contact) string { return strings.TrimSpace(derefString(c.ID)) }, func(c salesentity.Contact) error {
			return ev.writeContact(ctx, strings.TrimSpace(derefString(c.ID)), domain.SalesforceEventUpdated, c)
		})

	case domain.SalesforceResyncComponentMemberships:
		data, err := r.sfMemberships(ctx)
		if err != nil {
			return domain.SalesforceResyncApplyComponent{}, err
		}
		applyEach(ctx, t, data.items, func(p salesentity.ProjectContact) string { return strings.TrimSpace(p.ID) }, func(p salesentity.ProjectContact) error {
			contact, ok := membershipContact(data, p)
			if !ok {
				return &apierror.ServiceUnavailableError{Msg: "the membership's contact was not returned by Salesforce"}
			}
			return ev.ingestMembershipRecord(ctx, strings.TrimSpace(p.ID), domain.SalesforceEventUpdated, p, &contact)
		})
	}
	return t.finish(), nil
}
