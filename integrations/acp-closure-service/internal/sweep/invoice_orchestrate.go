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

package sweep

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/closure"
	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/suspensionstate"
)

// buildInvoiceCascade evaluates the invoice-based closure reason — the
// Phase 2 sibling of buildSubscriptionCascade, entirely independent (its
// own suspensionProcessState track, its own InvoiceDueDateClosureState
// dimension) — and, if it fires, returns a cascadeDecision ready to be
// ordered against the subscription cascade (see processProject).
//
// Returns (nil, nil) when there's no eligible due invoice (resolveDueInvoice
// returns nil — a legitimate, common state, not an error), when isPartner
// disables the cascade (see below), or when the decision simply doesn't
// fire yet — mirroring buildSubscriptionCascade's own no-op shape.
//
// isPartner does NOT gate this cascade on its own, and deliberately isn't
// checked before fetching invoice/account data. Legacy's
// calculateEventTypeFromDate (ACPMainProcess.js) checks hasPrimaryPartner
// FIRST, unconditionally — a project whose account has both isPartner=true
// and hasPrimaryPartner=true still fires, via the grace-period path — and
// only disables the cascade via isPartner once hasPrimaryPartner is known
// to be false. legacy also fetches due invoices unconditionally regardless
// of isPartner (ACPInvoiceUtils.fetchDueInvoicesByProject takes no
// isPartner parameter at all). A prior version of this function gated on
// isPartner alone before hasPrimaryPartner was ever fetched, which silently
// disabled that isPartner=true/hasPrimaryPartner=true combination entirely
// — a real behavioral gap versus legacy, not a documented simplification.
//
// renewed reports that the invoice track's saved state should go back to
// "open" (see resetToOpen): nothing is left to chase on this track, either no
// due invoice (paid), an account the track doesn't apply to, or a suspend
// date more than 90 days away.
func buildInvoiceCascade(ctx context.Context, reader entityReader, updater projectUpdater, ntf notifier, proj project, now time.Time) (cascade *cascadeDecision, renewed bool, err error) {
	dueInvoices, err := resolveDueInvoices(ctx, reader, proj)
	if err != nil {
		return nil, false, fmt.Errorf("resolve due invoice for project %s: %w", proj.ID, err)
	}

	lastWindow, err := suspensionstate.LastNoticeWindowForInvoices(proj.SuspensionProcessState)
	if err != nil {
		return nil, false, fmt.Errorf("parse suspensionProcessState for project %s: %w", proj.ID, err)
	}
	canReset := lastWindow != nil && dimensionOpen(proj.InvoiceDueDateClosureState)

	if len(dueInvoices) == 0 {
		return nil, canReset, nil
	}
	// The earliest due invoice drives the timing, as in legacy; the internal
	// notice lists all of them (resolvedForNotice.Listed).
	invoice := dueInvoices[0]

	hasPrimaryPartner, err := resolveHasPrimaryPartner(ctx, reader, proj.accountID())
	if err != nil {
		return nil, false, fmt.Errorf("resolve hasPrimaryPartner for project %s: %w", proj.ID, err)
	}

	isPartner := proj.Account != nil && proj.Account.IsPartner != nil && *proj.Account.IsPartner
	if isPartner && !hasPrimaryPartner {
		return nil, canReset, nil
	}

	decision := closure.DecideInvoice(now, invoice.InvoiceDate, invoice.DueDate, invoice.EULAVersionDecimal, hasPrimaryPartner, lastWindow)
	if !decision.Fires {
		return nil, canReset && newTerm(decision.DaysRemaining), nil
	}

	resolvedForNotice := dueInvoice{
		ID:          invoice.Number,
		Opportunity: invoice.Opportunity,
		SfID:        invoice.SfID,
		Listed:      listedInvoices(dueInvoices),
		DueDate:     invoice.DueDate,
		SuspendDate: closure.InvoiceSuspendDate(invoice.InvoiceDate, invoice.DueDate, invoice.EULAVersionDecimal, hasPrimaryPartner),
	}

	return &cascadeDecision{
		decision: decision,
		act: func(ctx context.Context, alreadyClosed bool, history *json.RawMessage) error {
			return actInvoice(ctx, reader, updater, ntf, proj, decision, resolvedForNotice, alreadyClosed, history)
		},
	}, false, nil
}

// actInvoice carries out the invoice-based closure cascade's actions for a
// project decision already confirmed to fire — mirrors actSubscription
// exactly, writing based_on_due_invoices/invoiceDueDateClosureState instead
// of the subscription equivalents. alreadyClosed reflects every
// higher-priority cascade's own decision.ShouldSuspend so far this run —
// see processProject.
func actInvoice(ctx context.Context, reader entityReader, updater projectUpdater, ntf notifier, proj project, decision closure.Decision, invoice dueInvoice, alreadyClosed bool, history *json.RawMessage) error {
	if decision.ShouldNotify {
		delivered := false
		var err error
		if !alreadyClosed {
			delivered, err = notifyForWindow(ctx, reader, ntf, proj, decision.Window, invoice.sfIDs(),
				func(w closure.NoticeWindow, p project, accountOwnerName string) string {
					return internalInvoiceNoticeBody(w, p, accountOwnerName, invoice)
				},
				customerInvoiceNoticeSubject,
				func(w closure.NoticeWindow, p project) string {
					return customerInvoiceNoticeBody(w, p, invoice)
				},
			)
			if err != nil {
				return fmt.Errorf("sweep: notify invoice for project %s: %w", proj.ID, err)
			}
		}
		if err := recordInvoiceNoticeSent(ctx, updater, proj.ID, history, decision.Window, delivered); err != nil {
			return fmt.Errorf("sweep: record invoice notice for project %s: %w", proj.ID, err)
		}
	}

	if decision.ShouldSuspend {
		if err := suspendInvoice(ctx, updater, proj); err != nil {
			return fmt.Errorf("sweep: suspend invoice for project %s: %w", proj.ID, err)
		}
	}

	return nil
}

// resolveHasPrimaryPartner fetches the account's hasPrimaryPartner flag —
// confirmed via the real API to only be present on GetAccount's full
// response, not the shortened account summary embedded in a project
// response (unlike isPartner, which is present there). "" accountID (no
// linked account) and an absent field both resolve to false, matching
// usesGracePeriod's own "not a primary partner" default.
func resolveHasPrimaryPartner(ctx context.Context, reader entityReader, accountID string) (bool, error) {
	if accountID == "" {
		return false, nil
	}
	raw, err := reader.GetAccount(ctx, accountID)
	if err != nil {
		return false, fmt.Errorf("get account: %w", err)
	}
	var acc accountDTO
	if err := json.Unmarshal(raw, &acc); err != nil {
		return false, fmt.Errorf("parse account: %w", err)
	}
	return acc.HasPrimaryPartner != nil && *acc.HasPrimaryPartner, nil
}

// recordInvoiceNoticeSent mirrors recordNoticeSent exactly, writing
// based_on_due_invoices instead of based_on_subscription_end_date — its
// own independent idempotency track — and building on the same in-memory
// history.
func recordInvoiceNoticeSent(ctx context.Context, updater projectUpdater, projectID string, history *json.RawMessage, window closure.NoticeWindow, delivered bool) error {
	action := "IGNORED"
	if delivered {
		action = "SUCCESSFUL"
	}
	newState, err := suspensionstate.WithDueInvoicesState(*history, window, map[string]string{
		"actionSendEmailNotification": action,
	})
	if err != nil {
		return fmt.Errorf("build suspensionProcessState: %w", err)
	}

	body, err := json.Marshal(map[string]json.RawMessage{"suspensionProcessState": newState})
	if err != nil {
		return fmt.Errorf("marshal update request: %w", err)
	}

	if _, err := updater.UpdateProject(ctx, projectID, body); err != nil {
		return err
	}
	*history = newState
	return nil
}

// suspendInvoice mirrors suspend exactly, writing/reading
// InvoiceDueDateClosureState instead of EndDateClosureState — its own
// per-dimension idempotency guard, entirely separate from the subscription
// cascade's.
func suspendInvoice(ctx context.Context, updater projectUpdater, proj project) error {
	if proj.InvoiceDueDateClosureState != nil && *proj.InvoiceDueDateClosureState != "Open" {
		return nil
	}

	body, err := json.Marshal(map[string]string{"invoiceDueDateClosureState": "Suspended"})
	if err != nil {
		return fmt.Errorf("marshal update request: %w", err)
	}

	if _, err := updater.UpdateProject(ctx, proj.ID, body); err != nil {
		return err
	}
	return suspendInSalesforce(ctx, updater, proj)
}

// listedInvoices turns the resolved due invoices into the lines the internal
// notice lists, keeping their earliest-due-first order.
func listedInvoices(invoices []resolvedInvoice) []invoiceLine {
	lines := make([]invoiceLine, len(invoices))
	for i, inv := range invoices {
		lines[i] = invoiceLine{Number: inv.Number, Opportunity: inv.Opportunity, DueDate: inv.DueDate, SfID: inv.SfID}
	}
	return lines
}
