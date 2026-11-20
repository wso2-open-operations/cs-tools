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

// Package sweep orchestrates one full ACP evaluation pass: fetch open
// projects, decide what's due, resolve recipients, notify, and write back
// state. Dry-run is not a flag branched on inside this package — it is
// entirely a property of which projectUpdater/notifier implementation the
// caller injects (see types.go).
package sweep

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/closure"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/recipients"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/suspensionstate"
)

// cascadeDecision pairs a closure reason's confirmed-firing closure.Decision
// (its DaysRemaining — negative once overdue, mirroring legacy's
// days_left — decides the run's processing order; its ShouldSuspend decides
// whether this cascade closes the project) with the deferred action that
// carries it out. processProject collects one of these per closure reason
// that fires this run, then sorts and executes them — see processProject's
// own doc comment for why this two-phase "decide everything, then act in
// urgency order" shape matters, not just single-reason correctness.
type cascadeDecision struct {
	decision closure.Decision
	// act carries out the cascade. history is the run's current
	// suspensionProcessState: the cascade's own record write builds on it
	// and updates it, so a later cascade in the same run never writes back a
	// stale copy of an earlier one's section (see processProject).
	act func(ctx context.Context, alreadyClosed bool, history *json.RawMessage) error
}

// processProject evaluates every closure reason this team handles —
// subscription end-date and invoice-based, each entirely independent (its
// own suspensionProcessState track, its own closure-state dimension) — and
// acts on whichever ones are due, in the same priority order legacy uses:
// ACPMainProcess.js's calculateProjectSuspension computes a Decision for
// every reason first, sorts them by days_left ascending (most overdue
// first), and only then does actionHandler act on them in that order, one
// at a time. This matters beyond ordering: legacy's checkForOpenProject
// gates every notify action on the project's closure status, so once the
// first (most urgent) reason's suspend fires within a run, every later
// reason sees the project already closed and skips its own notification.
//
// alreadyClosed tracks that in memory as cascades execute, seeded from
// proj.ClosureState (closed from a *previous* run) and updated to true the
// instant a cascade's own decision.ShouldSuspend is true (closed *by this
// run*, whether the suspend PATCH that just fired or an idempotent repeat
// of one already applied). This is deliberately NOT a live re-fetch of the
// project between cascades — an earlier version of this function tried
// exactly that (GetProject right before the second cascade acts) and it
// failed in a real live test: the project's rolled-up closureState is not
// read-after-write consistent here (the same stale-read behavior this
// backend has shown throughout this project, per prior handoffs — a GET
// moments after a PATCH cannot be trusted to reflect it), so the re-fetch
// kept observing the pre-suspend value and both cascades notified anyway.
// Tracking the fact in memory — something this process already knows with
// certainty, since decision.ShouldSuspend is computed locally, not read
// back from the API — sidesteps that entirely.
//
// suspensionProcessState is tracked the same way, for the same reason. Each
// cascade's record write replaces the whole object, so each must build on
// the previous cascade's write, not on the snapshot fetched at the start of
// the run. Building both from the snapshot let the second write reset the
// first cascade's section: a real staging run recorded an invoice "suspend"
// and then the subscription cascade's IGNORED record wrote the invoice
// section back to "open", after which the legacy system reopened the project and
// the next run suspended and emailed it again. history is that in-memory
// copy, updated after every successful record write.
//
// A failure *acting* on one cascade (in the execution loop below) returns
// immediately without attempting the next — matching how a failure partway
// through one cascade's own notify/suspend sequence already behaves, rather
// than silently swallowing it to try the remaining cascade anyway.
//
// Building the invoice cascade is treated differently (CodeRabbit, PR
// #1933): a failure there — a transient SearchProjectOpportunityLinks/
// SearchInvoices/GetOpportunity error, a malformed date or EULA-version
// field on one legacy-synced row — is logged and does not stop the
// subscription cascade, which was already built successfully and doesn't
// depend on invoice data at all, from being executed. The invoice error is
// still returned once the loop finishes, so Run still counts this project
// as failed — it's deferred, not swallowed. Building the subscription
// cascade doesn't get this same treatment: its only possible failure is a
// corrupt suspensionProcessState (pure parsing, no I/O), which is a
// genuinely unsafe state to decide *anything* from — including whether the
// invoice cascade's own section of that same JSON blob can be trusted — so
// that one still aborts immediately.
//
// acted reports whether a cascade of this project sends or records a notice
// or attempts a suspension (ACP_DAILY_LIMIT counts these, see Run). It is
// decided from the cascades before they run, so a project whose action then
// fails still counts.
func processProject(ctx context.Context, reader entityReader, updater projectUpdater, ntf notifier, now time.Time, proj project) (acted bool, err error) {
	var cascades []cascadeDecision

	subCascade, err := buildSubscriptionCascade(reader, updater, ntf, proj, now)
	if err != nil {
		return false, err
	}
	if subCascade != nil {
		cascades = append(cascades, *subCascade)
	}

	subRenewed, err := subscriptionRenewed(proj, now)
	if err != nil {
		return false, err
	}

	invoiceCascade, invoiceRenewed, invoiceErr := buildInvoiceCascade(ctx, reader, updater, ntf, proj, now)
	if invoiceErr != nil {
		slog.ErrorContext(ctx, "invoice cascade evaluation failed; still running the subscription cascade",
			"projectID", proj.ID, "err", invoiceErr)
	} else if invoiceCascade != nil {
		cascades = append(cascades, *invoiceCascade)
	}

	// Most-overdue-first, matching legacy's sorted_by_days_left ascending
	// sort. Stable so an exact days_left tie keeps subscription ahead of
	// invoice — the order they were appended above, mirroring legacy's own
	// pre-sort array order ([based_on_subscription_end_date,
	// based_on_due_invoices, based_on_compliance]).
	sort.SliceStable(cascades, func(i, j int) bool {
		return cascades[i].decision.DaysRemaining < cascades[j].decision.DaysRemaining
	})

	for _, c := range cascades {
		if c.decision.ShouldNotify || c.decision.ShouldSuspend {
			acted = true
		}
	}

	alreadyClosed := proj.ClosureState != nil && *proj.ClosureState != "Open"
	history := proj.SuspensionProcessState
	for _, c := range cascades {
		if err := c.act(ctx, alreadyClosed, &history); err != nil {
			return acted, err
		}
		if c.decision.ShouldSuspend {
			alreadyClosed = true
		}
	}

	if subRenewed {
		if err := resetToOpen(ctx, updater, proj.ID, &history, "end date", suspensionstate.WithSubscriptionEndDateOpen); err != nil {
			return acted, err
		}
	}
	if invoiceRenewed {
		if err := resetToOpen(ctx, updater, proj.ID, &history, "invoice", suspensionstate.WithDueInvoicesOpen); err != nil {
			return acted, err
		}
	}

	return acted, invoiceErr
}

// movedOut reports whether a track's date has moved out past the notice window
// recorded for it: more days remain than the recorded window's day count (90,
// 60, 30, 15 or 7; the terminal day-0 window counts 0). A recorded window is
// always one the date had reached (days <= window), and days only fall as time
// passes, so more days than the window can only mean the date moved later: a
// renewal, an extended due date, or a hand-reopened project whose date is in
// the future again. Legacy recomputed the event from the date on every run, so
// any such move changed the event and the next term's reminders started over;
// this is that test. It is false at exactly the window's day count and below,
// so an untouched project is never rewritten.
func movedOut(daysRemaining int, recorded closure.NoticeWindow) bool {
	return daysRemaining > int(recorded)
}

// dimensionOpen reports whether a per-dimension closure state is Open; unset
// counts as Open, as in suspend and suspendInvoice.
func dimensionOpen(state *string) bool {
	return state == nil || *state == "Open"
}

// subscriptionRenewed reports whether the end-date track's saved state should
// go back to "open" (see resetToOpen): a notice is recorded, the dimension is
// Open, and the end date has moved out past the recorded window (movedOut).
func subscriptionRenewed(proj project, now time.Time) (bool, error) {
	if proj.EndDate == nil || !dimensionOpen(proj.EndDateClosureState) {
		return false, nil
	}
	lastWindow, err := suspensionstate.LastNoticeWindow(proj.SuspensionProcessState)
	if err != nil {
		return false, fmt.Errorf("sweep: parse suspensionProcessState for project %s: %w", proj.ID, err)
	}
	return lastWindow != nil && movedOut(closure.Decide(now, *proj.EndDate, nil).DaysRemaining, *lastWindow), nil
}

// resetToOpen saves one track's suspensionProcessState section as "open",
// building on history like the notice records do. Legacy recomputed the event
// from the date on every run and saved it, so once a customer renewed (or
// paid) the track went back to "open" and the next term's reminders started
// over. Without this, a renewed project keeps its old "30_days_notice" and its
// next 60, 30, 15 and 7-day reminders are skipped. Only a track whose own closure
// state is still Open is reset, even if the other track suspended the project,
// as in legacy. No closure state changes: reopening a suspended project is a
// separate, manual step.
func resetToOpen(ctx context.Context, updater projectUpdater, projectID string, history *json.RawMessage, track string, reset func(json.RawMessage) (json.RawMessage, error)) error {
	newState, err := reset(*history)
	if err != nil {
		return fmt.Errorf("sweep: build %s suspensionProcessState for project %s: %w", track, projectID, err)
	}

	body, err := json.Marshal(map[string]json.RawMessage{"suspensionProcessState": newState})
	if err != nil {
		return fmt.Errorf("sweep: marshal update request: %w", err)
	}

	if _, err := updater.UpdateProject(ctx, projectID, body); err != nil {
		return fmt.Errorf("sweep: reset %s state for project %s: %w", track, projectID, err)
	}
	*history = newState
	slog.InfoContext(ctx, "state reset to open", "projectID", projectID, "track", track)
	return nil
}

// buildSubscriptionCascade evaluates the subscription end-date closure
// reason and, if it fires, returns a cascadeDecision ready to be ordered
// against any other firing reason. Returns (nil, nil) when there's no end
// date at all, when the track's own closure state is not Open (see
// actSubscription), the decision simply doesn't fire yet, or the track already
// holds a completed suspension (see below).
//
// An Open track whose saved event is a completed suspension is left alone when
// the decision is to suspend: the legacy process does not repeat an event it
// already completed, so a project a person set back to Open stays Open until
// the date moves the event (a renewal resets it, see subscriptionRenewed). Like
// a non-Open track it is left out of the cascades, so it does not mark the
// project closed for the other track.
//
// A track that is not Open is left out of the cascades, not just made
// inert in act: processProject sorts the cascades and marks the project
// closed once one that should suspend has run, and a cascade that suspends
// nothing must not suppress the other track's notice.
func buildSubscriptionCascade(reader entityReader, updater projectUpdater, ntf notifier, proj project, now time.Time) (*cascadeDecision, error) {
	if proj.EndDate == nil || !dimensionOpen(proj.EndDateClosureState) {
		return nil, nil
	}

	lastWindow, err := suspensionstate.LastNoticeWindow(proj.SuspensionProcessState)
	if err != nil {
		return nil, fmt.Errorf("sweep: parse suspensionProcessState for project %s: %w", proj.ID, err)
	}

	decision := closure.Decide(now, *proj.EndDate, lastWindow)
	if !decision.Fires {
		return nil, nil
	}

	if decision.ShouldSuspend {
		completed, err := suspensionstate.SuspensionCompleted(proj.SuspensionProcessState)
		if err != nil {
			return nil, fmt.Errorf("sweep: parse suspensionProcessState for project %s: %w", proj.ID, err)
		}
		if completed {
			return nil, nil
		}
	}

	return &cascadeDecision{
		decision: decision,
		act: func(ctx context.Context, alreadyClosed bool, history *json.RawMessage) error {
			return actSubscription(ctx, reader, updater, ntf, proj, decision, alreadyClosed, history, now)
		},
	}, nil
}

// actSubscription carries out the subscription end-date cascade's actions
// for a project decision already confirmed to fire — notify (gated on
// alreadyClosed, which reflects every higher-priority cascade's own
// decision.ShouldSuspend so far this run — see processProject) then
// suspend. Notify happens before suspend is ever attempted, and an error
// from notify returns immediately — this ordering, not a separate flag, is
// what guarantees suspend never proceeds after a failed notify (the day-0
// "email first, stop on failure" contract).
//
// A track whose own closure state is not Open (a person reopened the
// project, say, leaving "Pending Suspended" or "Notified" behind) does
// nothing: suspend would skip it, so a notice would tell the customer the
// project is suspended when nothing is.
func actSubscription(ctx context.Context, reader entityReader, updater projectUpdater, ntf notifier, proj project, decision closure.Decision, alreadyClosed bool, history *json.RawMessage, now time.Time) error {
	if !dimensionOpen(proj.EndDateClosureState) {
		return nil
	}
	if decision.ShouldNotify {
		delivered := false
		var err error
		if !alreadyClosed {
			if err := probeRecordWrite(ctx, updater, proj.ID, *history); err != nil {
				return err
			}
			delivered, err = notifyForWindow(ctx, reader, ntf, proj, decision.Window, nil, internalNoticeBody, customerNoticeSubject, customerNoticeBody)
			if err != nil {
				return fmt.Errorf("sweep: notify project %s: %w", proj.ID, err)
			}
		}
		if err := recordNoticeSent(ctx, updater, proj.ID, history, decision.Window, delivered); err != nil {
			return fmt.Errorf("sweep: record notice for project %s: %w", proj.ID, err)
		}
	}

	if decision.ShouldSuspend {
		if err := suspend(ctx, updater, proj, history, now); err != nil {
			return fmt.Errorf("sweep: suspend project %s: %w", proj.ID, err)
		}
	}

	return nil
}

// needsCustomerAudience reports whether window's confirmed audience matrix
// includes the customer, not just the Account Manager. 90/60/30 are
// internal-only; 15/7/0 are both.
func needsCustomerAudience(window closure.NoticeWindow) bool {
	switch window {
	case closure.NoticeWindow15, closure.NoticeWindow7, closure.NoticeWindow0:
		return true
	default:
		return false
	}
}

// projectOfAccount builds the "{ProjectName} of {AccountName}" clause
// shared by both internal subject variants, omitting the " of {AccountName}"
// half entirely when there's no linked account — a project with
// proj.Account == nil previously produced a dangling "...of " with a
// trailing space and nothing after it in a real outbound email subject (PR
// #1440 review).
func projectOfAccount(projectName, accountName string) string {
	if accountName == "" {
		return projectName
	}
	return fmt.Sprintf("%s of %s", projectName, accountName)
}

// internalNoticeSubject builds the always-sent internal (Account Owner/
// Renewal Manager/Technical Owner) notice's title line. Every internal copy
// is [ACP]-prefixed regardless of window — that prefix marks "this is the
// internal-audience copy," not "this is a 90/60/30 window" (confirmed
// against real examples from the product owner spanning 90 through 0; an earlier
// version of this function had the rule backwards). Day-0 gets distinct
// "suspension notice" wording instead of "N Days Reminder" — there's no
// "days remaining" left to report once a project is actually suspended.
func internalNoticeSubject(window closure.NoticeWindow, projectName, accountName string) string {
	target := projectOfAccount(projectName, accountName)
	if window.IsTerminal() {
		return fmt.Sprintf("[ACP] Project Suspension Notice of %s", target)
	}
	return fmt.Sprintf("[ACP] %d Days Reminder of Project for %s", int(window), target)
}

// customerNoticeSubject builds the customer-facing notice's title line
// (15/7/0 only) — never [ACP]-prefixed, never names the account, and uses
// future tense ("Upcoming") before day-0 versus past tense once actually
// suspended.
func customerNoticeSubject(window closure.NoticeWindow, projectName string) string {
	if window.IsTerminal() {
		return fmt.Sprintf("Project Suspension Notice - %s", projectName)
	}
	return fmt.Sprintf("Upcoming Project Suspension Notice - %s", projectName)
}

// internalReminderBodyTemplate is the day-count (90/60/30/15/7) internal
// notice body, confirmed verbatim from real examples the product owner shared. The
// greeting always names the Account Manager (%[1]s), regardless of which of
// the three internal recipients is actually reading their copy — not
// personalized per recipient. Dates are formatted 2006-01-02.
const internalReminderBodyTemplate = `Dear %[1]s

The following project has a non renewed contract. Please find the details below.

Project Name: %[2]s

Project Key: %[3]s

Account Owner: %[1]s

Start Date: %[4]s

End Date: %[5]s

Since projects needs contract renewal, kindly take the remedial actions to avoid any disruptions of subscription support. We appreciate your understanding and your prompt attention to this matter.

Best Regards,
%[6]s Team`

// internalSuspensionBodyTemplate is the day-0 internal notice body — past
// tense ("has been suspended"), asking for reinstatement rather than
// renewal.
const internalSuspensionBodyTemplate = `Dear %[1]s

The following project has been suspended due to non renewed contract. Kindly request that you take the appropriate action to reinitiate the suspended support account.

Project Name: %[2]s

Project Key: %[3]s

Account Owner: %[1]s

Start Date: %[4]s

End Date: %[5]s

Since the project is suspended, kindly take the remedial actions to reinstate the subscription support. We appreciate your prompt attention to this matter.

Best Regards,
%[6]s Team`

func internalNoticeBody(window closure.NoticeWindow, proj project, accountOwnerName string) string {
	template := internalReminderBodyTemplate
	if window.IsTerminal() {
		template = internalSuspensionBodyTemplate
	}
	return fmt.Sprintf(template, accountOwnerName, proj.Name, proj.ProjectKey, formatDate(proj.StartDate), formatDate(proj.EndDate), notify.CompanyName())
}

// customerUpcomingSuspensionBodyTemplate is the 15/7-day customer-facing
// body — future tense, no greeting (customer-facing notices never open with
// "Dear X", confirmed explicitly). Dates are formatted 01/02/2006 (US
// style), distinct from the internal bodies' 2006-01-02 — matches the product owner's
// real examples exactly.
const customerUpcomingSuspensionBodyTemplate = `We regret to inform you that the project %[1]s will be suspended on %[2]s due to non-renewal of the contracts upon the end of the previous subscription period.

Please ensure to take necessary actions on or before %[2]s to avoid service disruption. If you have any questions or need assistance, please contact your Account Manager or the %[3]s Customer Success Team.

Best Regards,
%[3]s Team`

// customerAlreadySuspendedBodyTemplate is the day-0 customer-facing body —
// past tense, once the project has actually been suspended.
const customerAlreadySuspendedBodyTemplate = `We trust this message finds you well. This project %[1]s was suspended %[2]s due to non-renewal of the contracts upon the end of the previous subscription period.

To avoid future suspensions, please ensure that all necessary actions are completed in a timely manner. If you have any questions or need assistance, please contact your Account Manager or the %[3]s Customer Success Team.

Best Regards,
%[3]s Team`

func customerNoticeBody(window closure.NoticeWindow, proj project) string {
	template := customerUpcomingSuspensionBodyTemplate
	if window.IsTerminal() {
		template = customerAlreadySuspendedBodyTemplate
	}
	return fmt.Sprintf(template, proj.Name, formatDateUS(proj.EndDate), notify.CompanyName())
}

// noBusinessContactBodyTemplate is the fixed body for the urgent
// no-business-contact notice, confirmed verbatim from a real existing
// notice the product owner shared. Dates are formatted 2006-01-02; Account Owner is
// the resolved Account Manager's *name*, not email.
const noBusinessContactBodyTemplate = `Internal - Customer Project without Business Contacts

Urgent reminder regarding the project %[1]s.

Please note that no Business Contacts was found for the project %[1]s. Immediate action is required to address this issue, as without a business contact, the customers will not receive essential notifications regarding their project suspension status. Additionally, this will lead to failures in further automated actions related to project suspension.

Please refer this document to Update Business Contact

Project Name: %[1]s
Project Key: %[2]s
Account Owner: %[3]s
Start Date: %[4]s
End Date: %[5]s`

func noBusinessContactBody(proj project, accountOwnerName string) string {
	return fmt.Sprintf(noBusinessContactBodyTemplate,
		proj.Name, proj.ProjectKey, accountOwnerName, formatDate(proj.StartDate), formatDate(proj.EndDate))
}

func formatDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// formatDateUS formats a date 01/02/2006 (US style), for the customer-facing
// prose bodies specifically — distinct from formatDate's 2006-01-02, used
// everywhere else (internal bodies' "Start Date:"/"End Date:" fields).
func formatDateUS(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("01/02/2006")
}

func timeValue(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func accountName(proj project) string {
	if proj.Account == nil {
		return ""
	}
	return proj.Account.Name
}

// notifyForWindow sends the always-fired internal notice (Account Owner/
// Renewal Manager/Technical Owner) for every firing window, and, only for
// the customer-audience windows (15/7/0), a second, separate notice with
// entirely different subject/body: either the customer notice (Customer
// populated, Account Owner/Renewal Manager/Technical Owner copied alongside
// as context) when the three-tier contact fallback resolves, or the
// no-business-contact urgent notice (to all three internal recipients) when
// it doesn't. Internal and customer notices are always two separate Send
// calls, never merged into one — their content genuinely differs, not just
// their recipient list.
//
// For a customer-audience window, contact resolution (fetchContacts +
// ResolveCustomerContacts) happens BEFORE the internal notice sends, not
// after — deliberately. A transient fetchContacts failure must leave zero
// notices sent, not an internal notice sent with no corresponding
// suspensionProcessState record: the caller (processProject) skips
// recordNoticeSent on any error here, so a partial send in that order would
// make the window look "not yet notified" on the next sweep and resend the
// same internal notice for real (raised in review). This
// does mean ntf.Send(internalNotice) has two call sites below rather than
// one — that's the tradeoff for keeping "skip contact-fetch entirely for
// internal-only windows" (needsCustomerAudience) and "never send before
// contacts resolve" both true at once; both sites wrap the error identically.
// notifyForWindow returns whether every notice it sent for this window was
// actually delivered — false if any one of them (internal, customer,
// nudge) wasn't, so the caller never records a window as fully handled
// when part of it silently wasn't (e.g. a customer notice filtered out by
// EmailNotifier's WSO2-only staging safeguard, even though the internal
// notice sent fine).
// internalBodyBuilder/customerSubjectBuilder/customerBodyBuilder let
// notifyForWindow serve both the subscription end-date cascade and the
// invoice cascade — the send/recipient-resolution logic below is identical
// for both, only the notice content differs. internalNoticeBody/
// customerNoticeSubject/customerNoticeBody (subscription) and
// internalInvoiceNoticeBody/customerInvoiceNoticeSubject/
// customerInvoiceNoticeBody (invoice, the last two adapted via a closure
// to capture the resolved invoice) satisfy these.
type internalBodyBuilder func(window closure.NoticeWindow, proj project, accountOwnerName string) string
type customerSubjectBuilder func(window closure.NoticeWindow, projectName string) string
type customerBodyBuilder func(window closure.NoticeWindow, proj project) string

func notifyForWindow(
	ctx context.Context, reader entityReader, ntf notifier, proj project, window closure.NoticeWindow,
	invoiceSfIDs []string,
	buildInternalBody internalBodyBuilder,
	buildCustomerSubject customerSubjectBuilder,
	buildCustomerBody customerBodyBuilder,
) (bool, error) {
	contacts, err := resolveAccountContacts(ctx, reader, proj.accountID())
	if err != nil {
		return false, fmt.Errorf("resolve account contacts: %w", err)
	}

	internalRecipients := notify.Recipients{
		AccountOwner:   contacts.AccountOwner,
		RenewalManager: contacts.RenewalManager,
		TechnicalOwner: contacts.TechnicalOwner,
	}

	internalNotice := baseNotice(proj, window)
	internalNotice.Subject = internalNoticeSubject(window, proj.Name, accountName(proj))
	internalNotice.Body = buildInternalBody(window, proj, contacts.AccountOwner.Name)
	internalNotice.Recipients = internalRecipients
	// Only the internal notice links to the invoices' Salesforce records;
	// customer and nudge notices never carry them.
	internalNotice.InvoiceSfIDs = invoiceSfIDs

	if !needsCustomerAudience(window) {
		delivered, err := ntf.Send(ctx, internalNotice)
		if err != nil {
			return false, fmt.Errorf("send internal notice: %w", err)
		}
		return delivered, nil
	}

	projectContacts, accountContactsList, err := fetchContacts(ctx, reader, proj)
	if err != nil {
		return false, err
	}
	resolution := recipients.ResolveCustomerContacts(projectContacts, accountContactsList)

	internalDelivered, err := ntf.Send(ctx, internalNotice)
	if err != nil {
		return false, fmt.Errorf("send internal notice: %w", err)
	}

	if !resolution.NeedsAMNudge {
		customerNotice := baseNotice(proj, window)
		customerNotice.Subject = buildCustomerSubject(window, proj.Name)
		customerNotice.Body = buildCustomerBody(window, proj)
		customerNotice.Recipients = internalRecipients
		customerNotice.Recipients.Customers = resolution.CustomerContacts
		customerNotice.ResolvedVia = resolution.ResolvedVia
		customerDelivered, err := ntf.Send(ctx, customerNotice)
		if err != nil {
			return false, err
		}
		return internalDelivered && customerDelivered, nil
	}

	nudgeNotice := baseNotice(proj, window)
	nudgeNotice.Subject = fmt.Sprintf("[Urgent] [ACP] No Business Contacts Specified for Project %s", proj.Name)
	nudgeNotice.Body = noBusinessContactBody(proj, contacts.AccountOwner.Name)
	nudgeNotice.Recipients = internalRecipients
	nudgeNotice.ResolvedVia = resolution.ResolvedVia
	nudgeDelivered, err := ntf.Send(ctx, nudgeNotice)
	if err != nil {
		return false, err
	}
	return internalDelivered && nudgeDelivered, nil
}

// baseNotice builds the project-identity fields shared by every Notice sent
// for a project/window — Subject, Body, Recipients, and ResolvedVia are left
// at their zero value for the caller to fill in per notice type.
func baseNotice(proj project, window closure.NoticeWindow) notify.Notice {
	return notify.Notice{
		ProjectID:   proj.ID,
		ProjectName: proj.Name,
		ProjectKey:  proj.ProjectKey,
		ProjectSfID: stringValue(proj.SfID),
		StartDate:   timeValue(proj.StartDate),
		EndDate:     timeValue(proj.EndDate),
		Window:      window,
	}
}

// accountContacts is the three account-level people a day-count reminder's
// Recipients draws from.
type accountContacts struct {
	AccountOwner   recipients.Contact
	RenewalManager recipients.Contact
	TechnicalOwner recipients.Contact
}

// resolveAccountContacts fetches the account and extracts its Account
// Manager (Account Owner), Renewal Account Manager, and Technical Owner as
// Contacts. Each Contact's Email may legitimately be "" — no person
// assigned to that role, or one assigned with no email on file — mirroring
// recipients.AccountManagerEmail's existing treatment of that as a
// non-error state, now applied to all three roles. Returns the zero
// accountContacts (not an error) if the project has no linked account.
func resolveAccountContacts(ctx context.Context, reader entityReader, accountID string) (accountContacts, error) {
	if accountID == "" {
		return accountContacts{}, nil
	}

	raw, err := reader.GetAccount(ctx, accountID)
	if err != nil {
		return accountContacts{}, fmt.Errorf("get account: %w", err)
	}

	var acc accountDTO
	if err := json.Unmarshal(raw, &acc); err != nil {
		return accountContacts{}, fmt.Errorf("parse account: %w", err)
	}

	return accountContacts{
		AccountOwner:   contactFromPersonRef(acc.AccountManager),
		RenewalManager: contactFromPersonRef(acc.RenewalAccountManager),
		TechnicalOwner: contactFromPersonRef(acc.TechnicalOwner),
	}, nil
}

// contactFromPersonRef converts a possibly-nil personRefDTO into a
// recipients.Contact, reusing recipients.AccountManagerEmail's nil/no-email
// handling (role-agnostic despite the name) rather than duplicating it.
func contactFromPersonRef(p *personRefDTO) recipients.Contact {
	if p == nil {
		return recipients.Contact{}
	}
	ref := recipients.PersonRef{ID: p.ID, Name: p.Name, Email: p.Email}
	return recipients.Contact{Name: p.Name, Email: recipients.AccountManagerEmail(&ref)}
}

// fetchContacts reads every page of the project's contacts and, unless the
// project already has business contacts (account contacts only feed the
// Primary Contact fallback), every page of the account's contacts. Both
// searches return one page at a time (20 rows
// unless a limit is sent, 50 at most, confirmed against staging) and report
// total but no hasMore, so each is paged until a page comes back empty or
// offset reaches total. Without this, a contact past the first page would
// never get the customer notice.
func fetchContacts(ctx context.Context, reader entityReader, proj project) ([]recipients.ProjectContact, []recipients.AccountContact, error) {
	var projectContacts []recipients.ProjectContact
	err := pageContacts(func(body []byte) (int, int, error) {
		raw, err := reader.SearchProjectContacts(ctx, proj.ID, body)
		if err != nil {
			return 0, 0, fmt.Errorf("search project contacts: %w", err)
		}
		var page projectContactSearchResponse
		if err := json.Unmarshal(raw, &page); err != nil {
			return 0, 0, fmt.Errorf("parse project contacts: %w", err)
		}
		for _, c := range page.Contacts {
			projectContacts = append(projectContacts, recipients.ProjectContact{Name: c.Name, Email: c.Email, Roles: c.Roles})
		}
		return len(page.Contacts), page.Total, nil
	})
	if err != nil {
		return nil, nil, err
	}

	// Account contacts only feed the Primary Contact fallback, so skip them
	// when the project already has business contacts (CodeRabbit, PR #2134).
	if proj.accountID() == "" || recipients.HasBusinessContacts(projectContacts) {
		return projectContacts, nil, nil
	}

	var accountContacts []recipients.AccountContact
	err = pageContacts(func(body []byte) (int, int, error) {
		raw, err := reader.SearchAccountContacts(ctx, proj.accountID(), body)
		if err != nil {
			return 0, 0, fmt.Errorf("search account contacts: %w", err)
		}
		var page accountContactSearchResponse
		if err := json.Unmarshal(raw, &page); err != nil {
			return 0, 0, fmt.Errorf("parse account contacts: %w", err)
		}
		for _, c := range page.Contacts {
			accountContacts = append(accountContacts, recipients.AccountContact{Name: c.Name, Email: c.Email, IsPrimary: c.IsPrimary})
		}
		return len(page.Contacts), page.Total, nil
	})
	if err != nil {
		return nil, nil, err
	}
	return projectContacts, accountContacts, nil
}

// pageContacts calls fetch once per page, passing the request body for that
// page. fetch returns how many rows the page held and the reported total.
func pageContacts(fetch func(body []byte) (rows, total int, err error)) error {
	offset := 0
	for {
		body, err := json.Marshal(searchContactsRequest{Pagination: pagination{Limit: pageSize, Offset: offset}})
		if err != nil {
			return fmt.Errorf("build contacts search request: %w", err)
		}
		rows, total, err := fetch(body)
		if err != nil {
			return err
		}
		offset += rows
		if rows == 0 || offset >= total {
			return nil
		}
	}
}

// probeRecordWrite writes the project's suspensionProcessState back as it is
// (history; {} when the project has none), before a notice is sent. Notices go
// out before they are recorded, so a project whose record CSM refuses (a 400,
// 404, 409 or 422 about this project) would get the same notice on every run
// until someone fixed it. Refused here, nothing has been sent: the project is
// a dataError (skipped with a WARN, the run not failed) and stays that way,
// sending nothing, until its record is fixed. A refusal every project would
// get (writesFailingError) stops the run as any CSM write does. The probe
// changes nothing but updated_on/updated_by.
func probeRecordWrite(ctx context.Context, updater projectUpdater, projectID string, history json.RawMessage) error {
	state := history
	if len(bytes.TrimSpace(state)) == 0 {
		state = json.RawMessage(`{}`)
	}
	if err := updater.CheckRecordWritable(ctx, projectID, state); err != nil {
		var refused *writesFailingError
		if errors.As(err, &refused) {
			return err
		}
		return &dataError{err: fmt.Errorf("CSM refuses writes to this project's record, so no notice is sent until it is fixed: %w", err)}
	}
	return nil
}

// recordNoticeSent writes the new window into suspensionProcessState's
// based_on_subscription_end_date key, preserving every other key untouched.
// actionSendEmailNotification records "SUCCESSFUL" only when delivered is
// true (the notifier in use actually sends real notices); otherwise it
// records "IGNORED" — the notice was logged, not sent, and the state must
// not claim a delivery that never happened. It builds on, and after a
// successful write updates, history: the run's current suspensionProcessState
// (see processProject), never the start-of-run snapshot.
func recordNoticeSent(ctx context.Context, updater projectUpdater, projectID string, history *json.RawMessage, window closure.NoticeWindow, delivered bool) error {
	action := "IGNORED"
	if delivered {
		action = "SUCCESSFUL"
	}
	newState, err := suspensionstate.WithSubscriptionEndDateState(*history, window, map[string]string{
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

// suspend writes endDateClosureState=Closed, unless this dimension has
// already moved past its initial "Open" state (checked against the
// already-fetched endDateClosureState — no extra round-trip), mirroring
// legacy's checkForOpenProject guard.
//
// The suspension's record (based_on_subscription_end_date "suspend" with
// suspended_on, see suspensionstate.WithSubscriptionEndDateSuspended) goes in
// the same update as the closure state, after Salesforce took
// State__c=Suspended, and nowhere else: the day-0 notice is recorded as
// "suspend_notice" (recordNoticeSent). So a suspension that stops half way
// (Salesforce or the CSM write failing) leaves an Open project with no
// "suspend" record, which entity-service's reopen would otherwise read as a
// project a person set back to Open and finish, with a "Back to Open" email
// for a project that was never suspended. The next run sees "suspend_notice"
// as the day-0 notice already sent and only retries the suspension. A
// dimension already past Open was not suspended by this run, so nothing is
// recorded as ACP's suspension for it.
//
// "Closed" is the end-date value the legacy system mapped to an overall
// "Suspended" (an end-date "Suspended" did not count there), and the only end-date suspension value Salesforce's
// Closure_State_End_Date_Based__c picklist has. CSM's own roll-up accepts both,
// so the overall closureState is "Suspended" either way. Invoice suspension
// keeps writing "Suspended" (suspendInvoice), as legacy did.
//
// Guarding on "not Open" rather than one exact value matters: projects this
// component suspended before it wrote "Closed" hold "Suspended" (and real
// legacy-suspended projects hold "Closed"), and both must count as already handled.
//
// Deliberately logs nothing, matching DryRunProjectUpdater's own silence —
// per explicit user direction, the log should show notice/email content
// only, nothing else, full stop. Known, accepted tradeoff (raised in PR
// #1440 review): the retry scenario where a prior run
// already recorded the notice (ShouldNotify=false) but suspend itself
// failed or was interrupted produces zero dry-run output for that project,
// since notifyForWindow is never called on that path either. Confirmed
// acceptable — do not add logging back here without re-confirming that
// decision has changed.
func suspend(ctx context.Context, updater projectUpdater, proj project, history *json.RawMessage, now time.Time) error {
	if !dimensionOpen(proj.EndDateClosureState) {
		return nil
	}
	newState, err := suspensionstate.WithSubscriptionEndDateSuspended(*history, suspendedOn(now))
	if err != nil {
		return fmt.Errorf("build suspensionProcessState: %w", err)
	}
	return writeSuspension(ctx, updater, proj, history, newState, "endDateClosureState", "Closed")
}

// writeSuspension sets Salesforce State__c to Suspended, then writes the
// closure state field and the suspension's record (newState) to CSM in one
// update, and moves history on. Shared by suspend and suspendInvoice.
func writeSuspension(ctx context.Context, updater projectUpdater, proj project, history *json.RawMessage, newState json.RawMessage, field, value string) error {
	body, err := json.Marshal(map[string]any{field: value, "suspensionProcessState": newState})
	if err != nil {
		return fmt.Errorf("marshal update request: %w", err)
	}
	if err := suspendInSalesforce(ctx, updater, proj); err != nil {
		return err
	}
	if _, err := updater.UpdateProject(ctx, proj.ID, body); err != nil {
		return fmt.Errorf("Salesforce State__c set to Suspended, but suspending the CSM project failed (the next run tries again): %w", err)
	}
	*history = newState
	return nil
}

// suspensionDateZone is the operations time zone (Colombo, UTC+05:30) the
// suspended_on date is given in, as the run's weekday is.
var suspensionDateZone = time.FixedZone("UTC+05:30", 5*60*60+30*60)

// suspendedOn is the suspended_on date (YYYY-MM-DD) of a suspension made at
// now. entity-service's reopen sends its "Back to Open" email for a project a
// person set back to Open only when this is recent.
func suspendedOn(now time.Time) string {
	return now.In(suspensionDateZone).Format("2006-01-02")
}

// suspendInSalesforce sets the project's Salesforce State__c to Suspended,
// before suspend or suspendInvoice writes the CSM suspension — ACP writes
// both itself; nothing syncs CSM to Salesforce. Salesforce goes first:
//
//   - A failure here fails the project with nothing written in CSM, so the
//     project is still Open there and the next run tries both writes again.
//     (Written second, a failure left CSM suspended, which the next run never
//     lists again, so State__c had to be set by hand.)
//   - entity-service's reopen check reopens only what CSM shows ACP
//     suspended, and the "suspend" record is written only in the CSM update
//     that suspends (see suspend). Run between the two writes, the reopen
//     finds an Open project with no "suspend" record: nothing to reopen and
//     nothing to finish, and the project ends suspended everywhere (if it
//     was in fact renewed or paid, the reopen's daily check reopens it).
//     Written CSM first, a reopen in between could set the project Open, and
//     this write then set Salesforce back to Suspended after the "Back to
//     Open" email went out.
//
// ACP never reopens, so it never writes Open.
func suspendInSalesforce(ctx context.Context, updater projectUpdater, proj project) error {
	if err := updater.SuspendInSalesforce(ctx, stringValue(proj.SfID)); err != nil {
		return fmt.Errorf("setting Salesforce State__c to Suspended failed, CSM left as it was (the next run tries again): %w", err)
	}
	return nil
}
