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

package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// This file owns the conversation about a time a CUSTOMER proposes for a change
// that is waiting in Customer Approval, and WSO2's answer to it: the previous system's
// own mechanism, which the synced schema already carries.
//
//	change_request.customer_updated_on                    the customer's proposed plan START
//	                                                      (migration 0043)
//	change_request.customer_updated_date_confirmation     WSO2's answer, AGREE | DISAGREE
//	                                                      (the confirmation of the proposed date)
//
// together with two triggers that already exist on change_request:
// change_request_reset_confirmation (0052: a moved proposal clears the standing
// answer) and change_request_plan_date_comment (0053: a proposal made in Customer
// Approval writes one customer-visible COMMENT on the change's parent record, as the
// previous system's flow for it did; nothing here reads that comment). No column,
// table, type, marker row or migration is added: a migrated change request that
// carries the pair is a conversation like any other.
//
//   - A customer's proposal (proposeCustomerTime) writes the proposed START to
//     customer_updated_on and nothing else. The change STAYS in Customer Approval,
//     start_on / end_on stay what WSO2 planned, no stage, approver row or state is
//     touched: until WSO2 answers there is nothing to apply.
//   - Accept proposed time (acceptCustomerProposal) writes AGREE, applies the
//     proposal to the planned window (the planned LENGTH is kept: a proposal is a
//     start) and moves the change to Scheduled, in one UPDATE. Nothing goes through
//     CAB again (the change itself has not changed) and the customer is not asked
//     again: the proposal is their own consent.
//   - Propose a different time (a staff {state: "authorize", ...} while a proposal
//     waits) writes DISAGREE with WSO2's window and asks the customer again, as a
//     Re-schedule does; keeping the window as it is declines the proposal and leaves
//     the customer's live request alone. A Re-schedule with no proposal waiting is the
//     same re-ask. The state does not move in any of them: "authorize" is the wire
//     name of the Time Change loop, not a destination.
//
// WHEN A PROPOSAL "WAITS" is one predicate, evaluated in SQL under the row lock by
// every act that depends on it and by the detail read (pendingProposalSQL): an
// ALLOWLIST, so that nothing the data could mean otherwise ever reads as a proposal
// to answer.
//
// WHO PROPOSED is what makes a waiting date a customer's PROPOSAL: the predicate only says a
// date is stored and unanswered. The proposer is the LAST WRITER of the change
// (work_item.updated_by) and nothing else: no comment, audit, outbox or other row is read to
// name anybody, because those rows log what the system did and are no reference for a
// business rule such as whether an Accept is allowed; and nothing is added to record it. A
// customer's proposal stamps the proposing contact as the last writer, so the time is
// recorded as theirs for as long as nobody else writes to the change. The writer counts only
// when they are a registered contact of the change's project (resolveProposer).
//
// A date that someone at WSO2 wrote in the previous system, one left over from an earlier
// cycle, and a genuine proposal that another write (a staff note, an edit, the hold) has
// overwritten as last writer since are alike "not recorded". Then no staff answer is about
// the date: Accept is refused (no staff action stands in for the customer's consent), a
// Re-schedule is a plain Re-schedule that writes no answer against it, a staff request with no
// window is refused, and the read model says so
// (domain.ChangeRequestCustomerProposal.ProposerRecorded) instead of naming anybody. What
// remains for a genuine proposal that has been overwritten is "Propose a different time",
// which the customer then approves: one more approval, by design. A WSO2-written date is
// migrated history once the previous system is retired, not a daily event.

// Values of change_request_confirmation_enum, written as plain literals (never bound
// parameters or casts) so the SQL reads the same on a database whose enum columns have
// the sync tool's own shape.
const (
	crConfirmationAgree    = "AGREE"
	crConfirmationDisagree = "DISAGREE"
)

// otherApprovalAskedSQL is the part of the pending predicate that asks "is an
// approval that is NOT the customer's still being asked": an EXISTS over the change's
// approver rows that are still REQUESTED, on a stage that is not a customer stage. A
// customer stage is one this service wrote (label "Customer Approval") or one whose
// group is the change's customer group (customer_group_id, the previous system's own record
// of who the customer is: the stages it asked of the customer carry no label).
// Every other REQUESTED row blocks -- a Peer / CAB / Review stage, and equally a
// stage in a group this service has no name for, because the allowlist cannot prove it
// is the customer's. COALESCE keeps the three-valued logic honest: a NULL label or a NULL
// group is "not a customer stage", never "unknown, so not blocking".
//
// cr is the alias of change_request in the enclosing query.
const otherApprovalAskedSQL = `EXISTS (
	SELECT 1
	FROM approval_stage_approver asa
	JOIN approval_stage st ON st.id = asa.stage_id
	WHERE asa.work_item_id = cr.id
	  AND asa.state = 'REQUESTED'
	  AND NOT (COALESCE(st.checkpoint_label = 'Customer Approval', false)
	        OR COALESCE(st.assignment_group_id = cr.customer_group_id, false)))`

// pendingProposalSQL is THE predicate "a customer's proposed time is waiting for
// WSO2's answer" (cr = change_request). Every reader and every act uses this text:
//
//   - the change is in Customer Approval (a change has one state: one waiting on a
//     live CAB stage is in Authorize and can never match; a closed, scheduled or
//     cancelled one cannot either; a NULL state is no state, and the whole predicate is
//     COALESCEd so that "unknown" reads false, never NULL);
//   - customer_updated_on is set, finite, and DIFFERENT from the planned start (a
//     proposal equal to the plan is applied already: nothing to answer);
//   - WSO2 has not answered (the confirmation is NULL: an AGREE or DISAGREE belongs to
//     history, and the 0052 trigger clears it whenever the proposal moves);
//   - and no approval but the customer's own is still being asked (otherApprovalAskedSQL).
//
// It says nothing about WHO wrote customer_updated_on: the previous system lets WSO2
// users write it too, and a date left over from an old cycle looks the same. That is what
// resolveProposer is for, and every act that answers a waiting time asks it.
const pendingProposalSQL = `COALESCE(cr.state = 'CUSTOMER_APPROVAL'
	AND cr.customer_updated_on IS NOT NULL
	AND isfinite(cr.customer_updated_on)
	AND cr.customer_updated_date_confirmation IS NULL
	AND cr.customer_updated_on IS DISTINCT FROM cr.start_on
	AND NOT ` + otherApprovalAskedSQL + `, false)`

// customerProposalFactsSQL reads, in one statement, what the conversation needs from
// change_request: the state, the finite planned window, the proposed start, the
// answer, the hold, the predicate and the blocking half of it on its own.
const customerProposalFactsSQL = `
	SELECT cr.state::text,
	       CASE WHEN isfinite(cr.start_on) THEN cr.start_on END,
	       CASE WHEN isfinite(cr.end_on) THEN cr.end_on END,
	       CASE WHEN isfinite(cr.customer_updated_on) THEN cr.customer_updated_on END,
	       cr.customer_updated_date_confirmation::text,
	       COALESCE(cr.is_on_hold, false),
	       ` + pendingProposalSQL + `,
	       ` + otherApprovalAskedSQL + `
	FROM change_request cr
	WHERE cr.id = $1`

// customerProposalFacts is customerProposalFactsSQL's row.
type customerProposalFacts struct {
	state string
	// start / end are the planned window (nil when not set or not finite);
	// proposed is customer_updated_on (nil when not set or not finite).
	start, end, proposed *time.Time
	confirmation         string
	onHold               bool
	// pending is pendingProposalSQL; otherAsked is its blocking half.
	pending, otherAsked bool
}

// readCustomerProposalFacts evaluates the predicate for the change request id. Every
// act that depends on it calls this under the change_request row lock (taken by
// lockChangeRequestGateSnapshot), so a read and the write that follows cannot drift;
// the detail read calls it without one. A missing row is a 404.
func readCustomerProposalFacts(ctx context.Context, q crQuerier, id string) (customerProposalFacts, error) {
	var f customerProposalFacts
	var state, confirmation *string
	err := q.QueryRow(ctx, customerProposalFactsSQL, id).Scan(
		&state, &f.start, &f.end, &f.proposed, &confirmation, &f.onHold, &f.pending, &f.otherAsked)
	if errors.Is(err, pgx.ErrNoRows) {
		return customerProposalFacts{}, &apierror.NotFoundError{Msg: "change request not found"}
	}
	if err != nil {
		return customerProposalFacts{}, fmt.Errorf("read the customer's proposed time: %w", err)
	}
	f.state = strings.ToUpper(stringOrEmpty(state))
	f.confirmation = strings.ToUpper(stringOrEmpty(confirmation))
	// Microseconds are all a timestamptz holds; compare instants at that precision.
	for _, t := range []*time.Time{f.start, f.end, f.proposed} {
		if t != nil {
			*t = t.UTC().Truncate(time.Microsecond)
		}
	}
	return f, nil
}

// plannedLength is the length of the planned window, ok=false when there is none to
// keep (no window, or an end that is not after the start).
func (f customerProposalFacts) plannedLength() (time.Duration, bool) {
	if f.start == nil || f.end == nil || !f.end.After(*f.start) {
		return 0, false
	}
	return f.end.Sub(*f.start), true
}

// proposedEnd is the proposal applied to the planned window: the proposed start plus
// the planned length. nil when there is no proposal or no length to keep.
func (f customerProposalFacts) proposedEnd() *time.Time {
	length, ok := f.plannedLength()
	if f.proposed == nil || !ok {
		return nil
	}
	e := f.proposed.Add(length)
	return &e
}

// fmtInstant is how an instant reads in a message and in the proposal read model:
// RFC 3339 in UTC, with fractional seconds only when there are some, so a value that
// came from the API goes back to it unchanged (expectedCustomerUpdatedOn).
func fmtInstant(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func fmtInstantPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := fmtInstant(*t)
	return &s
}

// fmtPlannedLength reads a planned length for a message: "2 hours", "1 day 30 minutes".
func fmtPlannedLength(d time.Duration) string {
	if d <= 0 {
		return "0 seconds"
	}
	var parts []string
	add := func(n int64, unit string) {
		if n == 0 {
			return
		}
		if n != 1 {
			unit += "s"
		}
		parts = append(parts, fmt.Sprintf("%d %s", n, unit))
	}
	secs := int64(d / time.Second)
	add(secs/86400, "day")
	add(secs%86400/3600, "hour")
	add(secs%3600/60, "minute")
	add(secs%60, "second")
	if len(parts) == 0 {
		return d.String()
	}
	return strings.Join(parts, " ")
}

// ---------------------------------------------------------------------------
// The refusals (the API contract: openapi.yaml and the webapps read these)
// ---------------------------------------------------------------------------

const (
	msgProposalNeedsStart          = "a proposed implementation time needs a new start: send plannedStartOn"
	msgProposalNoWindow            = "this change request has no planned window to move, so a new time cannot be proposed for it"
	msgProposalIsPlannedStart      = "plannedStartOn is the planned start already: propose a different start"
	msgProposalAlreadyProposed     = "that time is already proposed and is waiting for WSO2's response"
	msgProposalWSO2AskedOther      = "WSO2 asked for a different time than that one: propose another start"
	msgProposalOtherApprovalAsked  = "this change request is also waiting for an approval that is not the customer's, so a new time cannot be proposed for it right now"
	msgProposalOnHold              = "this change request is on hold, so a new implementation time cannot be proposed now"
	msgAcceptValueMustBeAgree      = `confirmCustomerUpdatedDate must be "agree": to decline a proposal, propose a different time (state "authorize" with the new planned window)`
	msgAcceptNeedsExpectedProposal = "expectedCustomerUpdatedOn is required with confirmCustomerUpdatedDate: it names the proposed time you are accepting"
	msgAcceptNeedsExpectedWindow   = "expectedPlannedStartOn and expectedPlannedEndOn are required with confirmCustomerUpdatedDate: they name the planned time the proposal replaces"
	msgAcceptAlone                 = "confirmCustomerUpdatedDate cannot be combined with other fields; only expectedCustomerUpdatedOn, expectedPlannedStartOn and expectedPlannedEndOn go with it"
	msgAcceptNothingWaiting        = "no new time proposed by the customer is waiting for a response on this change request"
	msgAcceptOnHold                = "change request is on hold; take it off hold (onHold: false) before changing its state"
	msgAcceptNoLength              = `the planned window has no length, so the customer's proposed start cannot be applied to it: use "Propose a different time"`
	msgCounterIsTheProposal        = `the time you are proposing is the one the customer proposed: use "Accept proposed time" instead`
	msgProposalNoLongerWaiting     = "the customer's proposed time is no longer waiting for a response; read the change request again"
	// msgAcceptProposerNotRecorded refuses an Accept (and is the reason the read model gives for
	// canAccept: false) when no registered contact of the project is recorded as having proposed
	// the stored time. No staff action stands in for the customer's own answer: a time nobody
	// proposed is not one a customer agreed to.
	msgAcceptProposerNotRecorded = `nobody is recorded as having proposed this time (it may have been written by someone at WSO2 or left over from an earlier cycle), so it cannot be accepted: use "Propose a different time" to ask the customer to approve a time`
	// msgNoRecordedProposalToDecline is the 400 for a staff {state: "authorize"} with no window
	// while the stored time is one nobody is recorded as having proposed: there is no customer
	// proposal to decline, and a time is stored that no answer of ours would be about.
	msgNoRecordedProposalToDecline = "no customer is recorded as having proposed the time stored on this change request, so there is no proposal to decline: send the new planned window to re-schedule it"
	// msgProposalStoredNotProposed is the customer's refusal for proposing exactly the time that
	// is stored already but that nobody is recorded as having proposed (it is not theirs, and
	// writing it again could not record them as its proposer).
	msgProposalStoredNotProposed = "that time is already stored on this change request, although nobody is recorded as having proposed it: propose a different start"
	// readAgainSuffix ends the stale-window refusal of a staff answer (the customer's
	// own answers end it "before giving your answer").
	readAgainSuffix = "read it again before responding"
)

func msgProposalChangedStart(now time.Time) string {
	return fmt.Sprintf("the customer's proposed time changed after you opened this change request (it is now %s); read it again before responding", fmtInstant(now))
}

// msgStoredTimeChanged is the 409 for a staff Re-schedule that names the stored time it was
// shown (expectedCustomerUpdatedOn) when it is no longer the stored one, in a case where nobody is
// recorded as having proposed the time: the page it was sent from is out of date.
func msgStoredTimeChanged(now time.Time) string {
	return fmt.Sprintf("the time stored on this change request changed after you opened it (it is now %s); read it again before responding", fmtInstant(now))
}

func msgCounterProposalMissed(proposed time.Time) string {
	return fmt.Sprintf("the customer proposed a new time (%s) after you opened this change request; read it again to accept it or propose a different time", fmtInstant(proposed))
}

func msgAcceptNotInCustomerApproval(state string) string {
	return fmt.Sprintf("a proposed time can only be accepted while the change request is in Customer Approval, but it is in %s",
		changeRequestStateDisplayName(stateForMessage(state)))
}

func msgAcceptTimePassed(proposed time.Time) string {
	return fmt.Sprintf(`the time the customer proposed (%s) has already passed, so it cannot be accepted: use "Propose a different time" to ask the customer to approve another time`, fmtInstant(proposed))
}

// msgAcceptTooFarAhead is the refusal of an Accept that would write a window beyond the range
// every planned window is held to (plannedYearMax). A customer's own proposal never gets there
// (proposeCustomerTime refuses it), but customer_updated_on is a column the previous system writes too.
func msgAcceptTooFarAhead(proposed time.Time, end time.Time) string {
	return fmt.Sprintf(`the time the customer proposed (%s) is too far ahead to be accepted: the window would end after the year %d (%s), so use "Propose a different time" to ask the customer to approve another time`,
		fmtInstant(proposed), plannedYearMax, fmtInstant(end))
}

func msgProposalKeepsLength(length time.Duration, end time.Time) string {
	return fmt.Sprintf("a proposed time moves the start and keeps the planned length of %s: plannedEndOn must be %s, or be left out",
		fmtPlannedLength(length), fmtInstant(end))
}

// ---------------------------------------------------------------------------
// The read model
// ---------------------------------------------------------------------------

// customerProposalAnswer names the state of the conversation.
func customerProposalAnswer(f customerProposalFacts) string {
	switch {
	case f.proposed == nil:
		return ""
	case f.pending:
		return "pending"
	case f.confirmation == crConfirmationAgree:
		return "agreed"
	case f.confirmation == crConfirmationDisagree:
		return "disagreed"
	}
	return "unanswered"
}

// acceptBlock says whether "Accept proposed time" would be refused right now for a
// reason the data shows (the same refusals acceptCustomerProposal gives, in the same
// words and the same order), "" when it would not. Meaningful only while the proposal
// is pending. who is the proposer (the last writer, when they are a registered contact of the
// project): no registered contact recorded as having proposed the time is the first refusal,
// because nothing else is worth fixing about a time no customer proposed.
func acceptBlock(f customerProposalFacts, who proposer, now time.Time) string {
	switch {
	case !who.known:
		return msgAcceptProposerNotRecorded
	case f.onHold:
		return msgAcceptOnHold
	case f.proposed != nil && !f.proposed.After(now):
		return msgAcceptTimePassed(*f.proposed)
	}
	if _, ok := f.plannedLength(); !ok {
		return msgAcceptNoLength
	}
	if end := f.proposedEnd(); end != nil && f.proposed != nil && end.UTC().Year() > plannedYearMax {
		return msgAcceptTooFarAhead(*f.proposed, *end)
	}
	return ""
}

// proposer is who is recorded as having proposed the time that waits for WSO2: a
// registered contact of the change's project, and nobody else. A proposer is never
// guessed (known is false whenever the data cannot say), and never a WSO2 user.
type proposer struct {
	known       bool
	name, email string
	// proposedOn is when the last writer wrote the change, which is when the time was proposed
	// while that writer is the proposer. Set by readProposer only; zero from the acts, which have
	// stamped themselves on the row by the time they look.
	proposedOn time.Time
}

// proposerFromWriter is the whole rule: the last writer of the change is the proposer when
// they are a REGISTERED PORTAL_USER contact of the change's project (callerIsRegisteredPortalContact,
// the test of the customer's own answer), and nobody otherwise. A blank writer, a WSO2 user, a
// contact who has left, a contact of another project and the sync's own stamp all name nobody.
// Only the writer's email is looked at: nothing else is read to name a proposer (no comment,
// audit, outbox or other row), and nothing is written to record one.
func proposerFromWriter(ctx context.Context, q crQuerier, project *string, writer string) (proposer, error) {
	email := strings.TrimSpace(writer)
	if email == "" {
		return proposer{}, nil
	}
	ok, err := callerIsRegisteredPortalContact(ctx, q, project, email)
	if err != nil || !ok {
		return proposer{}, err
	}
	return proposer{known: true, email: email}, nil
}

// resolveProposer says who is recorded as having proposed the time stored in
// change_request.customer_updated_on, for an ACT that answers it (Accept, Propose a different
// time, a customer's own proposal), from the last writer the act found before its own write.
//
// The acts stamp the caller on work_item.updated_by as their first write (lockWorkItemKeepingWriter,
// lockChangeRequestForPatch), so after it the column only ever names the caller: the act must
// pass the writer it kept (lockedWorkItem.priorWriter, changeRequestGateSnapshot.priorWriter).
// A writer that was never read is a bug of the caller, not "nobody", and is refused loudly.
func resolveProposer(ctx context.Context, q crQuerier, project *string, prior lastWriter) (proposer, error) {
	if !prior.read {
		return proposer{}, errors.New("resolve the proposer: the last writer of the change request was not read before this request wrote to it")
	}
	return proposerFromWriter(ctx, q, project, prior.email)
}

// readProposer is the proposer for a reader (the detail read): the last writer as the
// change request shows it now, with the writer's name and when they wrote.
func readProposer(ctx context.Context, q crQuerier, id string) (proposer, error) {
	var project, writer *string
	var updatedOn time.Time
	err := q.QueryRow(ctx,
		`SELECT project_id::text, updated_by, updated_on FROM work_item WHERE id = $1`, id).Scan(&project, &writer, &updatedOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return proposer{}, nil
	}
	if err != nil {
		return proposer{}, fmt.Errorf("read who last wrote the change request: %w", err)
	}
	p, err := proposerFromWriter(ctx, q, project, stringOrEmpty(writer))
	if err != nil || !p.known {
		return p, err
	}
	p.proposedOn = updatedOn.UTC()
	var name *string
	if err := q.QueryRow(ctx, `
		SELECT COALESCE(NULLIF(TRIM(u.name), ''), NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''))
		FROM "user" u WHERE LOWER(u.email) = LOWER($1)
		ORDER BY u.created_on ASC LIMIT 1`, p.email).Scan(&name); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return p, fmt.Errorf("read the proposer's name: %w", err)
	}
	p.name = stringOrEmpty(name)
	return p, nil
}

// fillCustomerProposal sets domain.ChangeRequest.CustomerProposal for the caller
// reading cr: absent when the change has no proposed time (customer_updated_on
// unset), otherwise the conversation as the caller may see it -- a staff reader gets
// the proposer's name and whether Accept would work, a customer gets only whether the
// proposal is theirs. Best effort like markCustomerCanAnswer: a failure leaves the field
// unset (and is logged), the detail read is not worth failing for it.
//
// A stored time that nobody is recorded as having proposed is not a customer's proposal
// waiting for an answer: staff read it as pending with proposerRecorded false and Accept
// blocked (the date is there for them to deal with), and a customer reads it as
// "unanswered", history, so no page tells them WSO2 is deciding on a time they did not
// propose. That includes a customer whose genuine proposal another write has since replaced
// as last writer: nothing records it any more, and Propose a different time is the way on.
//
// The predicate reads approver rows of every stage, so it runs under the system
// identity: the caller was already shown this change request by the visibility-gated
// read, and what comes out is a handful of derived facts, never a row.
func (r *changeRequestRepo) fillCustomerProposal(ctx context.Context, cr *domain.ChangeRequest) {
	if cr.CustomerUpdatedOn == nil {
		return
	}
	sys := WithSystemIdentity(ctx)
	f, err := readCustomerProposalFacts(sys, r.db, cr.ID)
	if err != nil {
		slog.WarnContext(ctx, "get change request: customerProposal left unset", "changeRequestId", cr.ID, "error", err)
		return
	}
	if f.proposed == nil {
		return
	}
	now := time.Now()
	out := &domain.ChangeRequestCustomerProposal{StartOn: fmtInstant(*f.proposed), Answer: customerProposalAnswer(f)}
	if !f.pending {
		cr.CustomerProposal = out
		return
	}
	scope, external := CallerIdentityFromContext(ctx)
	external = external && IsExternalCaller(ctx)
	who, err := readProposer(sys, r.db, cr.ID)
	if err != nil {
		slog.WarnContext(ctx, "get change request: the proposer left unset", "changeRequestId", cr.ID, "error", err)
		if external {
			// Who proposed it is not known, so no customer is told a time of theirs is waiting.
			out.Answer = "unanswered"
		} else {
			out.EndOn = fmtInstantPtr(f.proposedEnd())
		}
		cr.CustomerProposal = out
		return
	}
	recorded := who.known
	if external {
		if !recorded {
			// Not theirs, not a colleague's, not anybody's: nothing waits for WSO2 on their account.
			out.Answer = "unanswered"
			cr.CustomerProposal = out
			return
		}
		viewer := strings.EqualFold(who.email, strings.TrimSpace(scope.ViewerEmail))
		out.EndOn = fmtInstantPtr(f.proposedEnd())
		out.ProposerRecorded = &recorded
		out.ProposedByViewer = &viewer
		cr.CustomerProposal = out
		return
	}
	out.EndOn = fmtInstantPtr(f.proposedEnd())
	out.ProposerRecorded = &recorded
	if recorded {
		out.ProposedByEmail = &who.email
		if who.name != "" {
			out.ProposedByName = &who.name
		}
		on := fmtInstant(who.proposedOn)
		out.ProposedOn = &on
	}
	reason := acceptBlock(f, who, now)
	can := reason == ""
	out.CanAccept = &can
	if !can {
		out.AcceptBlockedReason = &reason
	}
	cr.CustomerProposal = out
}

// ---------------------------------------------------------------------------
// The customer proposes
// ---------------------------------------------------------------------------

// proposeCustomerTime is PATCH {plannedStartOn, plannedEndOn?} from an external
// caller: a registered contact of the change's project who has been asked for the
// customer's approval proposes a new START for the implementation. Only
// customer_updated_on is written (and the answer cleared): the change stays in
// Customer Approval, the planned window is untouched, no stage, approver row or state
// moves -- the customer's request stays live, so they (and their colleagues) can still
// approve the CURRENT plan, and the proposer keeps seeing the change request. WSO2
// answers from the change request (acceptCustomerProposal, or a staff "propose a
// different time").
//
// A proposal is a start: the planned LENGTH is kept (customer_updated_on holds one
// instant, the previous system's own model). The customer's dialog sends the start plus a
// derived end for the servers that still take a whole window; an end that is sent
// must be exactly the derived one.
//
// In order, the first failing check wins and nothing is written until all pass:
//
//  1. the window is parsed (nothing but a date-time in range reaches SQL: 'infinity',
//     'now', a zone name are a 400 whoever sends them) and must be still to come;
//  2. the change request is visible to the caller (404; the caller's work_item UPDATE
//     is the proof) and the caller is a registered PORTAL_USER contact of its own
//     project (403), who is not its creator (403);
//  3. it is in Customer Approval (409), has a live customer stage (409: a legacy change
//     request is given its stage here, as the customer's answer would) on which the
//     caller holds a REQUESTED row (403);
//  4. a start was sent (400), the change is not on hold (409), no other approval is
//     being asked (409), there is a planned window to move (409), an end that was sent
//     is the derived one (400), the start is not the planned one (400), nor one already
//     proposed or already refused by WSO2 (400).
//
// Returns the change request's id.
func proposeCustomerTime(ctx context.Context, tx pgx.Tx, id string, req domain.PatchChangeRequestRequest, actorEmail string) (string, error) {
	start, err := normalizePlannedTimestamp("plannedStartOn", req.PlannedStartOn)
	if err != nil {
		return "", err
	}
	end, err := normalizePlannedTimestamp("plannedEndOn", req.PlannedEndOn)
	if err != nil {
		return "", err
	}

	locked, err := lockWorkItemKeepingWriter(ctx, tx, id, actorEmail)
	if err != nil {
		return "", err
	}
	projectID := locked.projectID
	if err := requireRegisteredContact(ctx, tx, projectID, actorEmail); err != nil {
		return "", err
	}
	gates, err := lockChangeRequestGateSnapshot(ctx, tx, id)
	if err != nil {
		return "", err
	}
	if gates.state != crStateCustomerApproval {
		return "", &apierror.ConflictError{Code: apierror.CodeChangeRequestNotProposable, Msg: fmt.Sprintf(
			"a new implementation time can only be proposed while the change request is in Customer Approval, but it is in %s",
			changeRequestStateDisplayName(stateForMessage(gates.state)))}
	}

	userID, err := customerApproverUserID(ctx, tx, id, actorEmail)
	if err != nil {
		return "", err
	}
	creatorIDs, err := changeRequestCreatorsForApprover(ctx, tx, id, userID, actorEmail)
	if err != nil {
		return "", fmt.Errorf("propose implementation time: %w", err)
	}
	if err := approverDecisionBlock(ctx, tx, userID, creatorIDs, stageKindCustomerApproval); err != nil {
		return "", err
	}

	// A legacy change request waiting in Customer Approval with nobody asked is given
	// its live stage first (see ensureCustomerStageForLegacy), so the proposal has
	// somebody to be proposed to.
	if err := ensureCustomerStageForLegacy(ctx, tx, id, actorEmail); err != nil {
		return "", err
	}
	live, err := liveCustomerStageForState(ctx, tx, id, gates.state)
	if err != nil {
		return "", fmt.Errorf("propose implementation time: %w", err)
	}
	if live == nil {
		return "", &apierror.ConflictError{Code: apierror.CodeChangeRequestNotProposable, Msg: "no customer approval is pending on this change request: it has not been requested from the project's registered contacts, so there is nobody for a new implementation time to be proposed to here"}
	}
	asked, err := customerHasRequestedRow(ctx, tx, live.stageID, userID)
	if err != nil {
		return "", fmt.Errorf("propose implementation time: %w", err)
	}
	if !asked {
		return "", &apierror.ForbiddenError{Code: apierror.CodeChangeRequestNotAsked, Msg: "only members of the customer group (the registered contacts of this change request's project) who have been asked for the customer's approval of this change request can propose a new implementation time for it"}
	}

	if err := requireFutureWindow(time.Now(), start, end); err != nil {
		return "", err
	}
	if start == nil {
		return "", &apierror.ValidationError{Msg: msgProposalNeedsStart}
	}

	// What the proposal is written for, and who may know of it, is the data's to say:
	// the caller has proven their access to this change request in this transaction (the
	// work_item write above), so the rest -- the approval rows of every stage, and the
	// trigger that comments on the parent record, as the previous system's flow did -- runs as the
	// system, exactly as provisionCustomerStage does after the same proof.
	if err := setCallerIdentity(ctx, tx, SearchScope{Unrestricted: true}); err != nil {
		return "", fmt.Errorf("propose implementation time: escalate identity: %w", err)
	}
	f, err := readCustomerProposalFacts(ctx, tx, id)
	if err != nil {
		return "", err
	}
	if f.onHold {
		return "", &apierror.ConflictError{Code: apierror.CodeChangeRequestOnHold, Msg: msgProposalOnHold}
	}
	if f.otherAsked {
		return "", &apierror.ConflictError{Code: apierror.CodeChangeRequestProposalNotNow, Msg: msgProposalOtherApprovalAsked}
	}
	length, ok := f.plannedLength()
	if !ok {
		return "", &apierror.ConflictError{Code: apierror.CodeChangeRequestNoPlannedWindow, Msg: msgProposalNoWindow}
	}
	startAt, err := time.Parse(time.RFC3339Nano, *start)
	if err != nil {
		return "", plannedTimestampError("plannedStartOn")
	}
	startAt = startAt.UTC().Truncate(time.Microsecond)
	if end != nil {
		endAt, err := time.Parse(time.RFC3339Nano, *end)
		if err != nil {
			return "", plannedTimestampError("plannedEndOn")
		}
		if want := startAt.Add(length); !endAt.UTC().Truncate(time.Microsecond).Equal(want) {
			return "", &apierror.ValidationError{Msg: msgProposalKeepsLength(length, want)}
		}
	}
	if startAt.Equal(*f.start) {
		return "", &apierror.ValidationError{Msg: msgProposalIsPlannedStart}
	}
	// The end this proposal would give the window (start + the planned length) is held to the
	// same range every written window is, so a pathological stored length cannot make a later
	// Accept write a year no date-time of ours has.
	if y := startAt.Add(length).UTC().Year(); y > plannedYearMax {
		return "", &apierror.ValidationError{Msg: fmt.Sprintf(
			"plannedStartOn is too far ahead: with the planned length of %s the proposed window would end after the year %d", fmtPlannedLength(length), plannedYearMax)}
	}
	if f.proposed != nil && startAt.Equal(*f.proposed) {
		if f.confirmation == crConfirmationDisagree {
			return "", &apierror.ValidationError{Msg: msgProposalWSO2AskedOther}
		}
		if f.pending {
			// "Waiting for WSO2's response" is true only of a proposal somebody made. A time that
			// nobody is recorded as having proposed is stored, not waiting, and writing it again
			// here could not make this caller its proposer.
			who, err := resolveProposer(ctx, tx, locked.projectID, locked.priorWriter)
			if err != nil {
				return "", err
			}
			if !who.known {
				return "", &apierror.ValidationError{Msg: msgProposalStoredNotProposed}
			}
		}
		return "", &apierror.ValidationError{Msg: msgProposalAlreadyProposed}
	}

	// The answer is cleared explicitly (the 0052 trigger would too when only the date
	// moved): a proposal is a new question, whatever was answered before.
	ct, err := tx.Exec(ctx,
		`UPDATE change_request SET customer_updated_on = $2::text::timestamptz, customer_updated_date_confirmation = NULL
		 WHERE id = $1 AND state = 'CUSTOMER_APPROVAL'`, id, fmtInstant(startAt))
	if err != nil {
		return "", fmt.Errorf("propose implementation time: write the proposed start: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return "", &apierror.ConflictError{Code: apierror.CodeChangeRequestNotProposable, Msg: msgAcceptNotInCustomerApproval(gates.state)}
	}
	return id, nil
}

// ---------------------------------------------------------------------------
// WSO2 accepts
// ---------------------------------------------------------------------------

// validateAcceptRequest judges the SHAPE of an Accept (PATCH {confirmCustomerUpdatedDate:
// "agree", expectedCustomerUpdatedOn, expectedPlannedStartOn, expectedPlannedEndOn}) before
// anything is read or written, and returns the three expectations as instants. The request
// carries these four fields and nothing else (the whitelist is "clear them and require the
// rest to be empty", as classifyExternalPatch does, so a field added to the request later
// cannot ride along with an acceptance until somebody decides it may).
func validateAcceptRequest(req domain.PatchChangeRequestRequest) (proposed, expStart, expEnd *time.Time, err error) {
	if req.ConfirmCustomerUpdatedDate == nil || strings.ToLower(strings.TrimSpace(*req.ConfirmCustomerUpdatedDate)) != "agree" {
		return nil, nil, nil, &apierror.ValidationError{Msg: msgAcceptValueMustBeAgree}
	}
	rest := req
	rest.ConfirmCustomerUpdatedDate, rest.ExpectedCustomerUpdatedOn = nil, nil
	rest.ExpectedPlannedStartOn, rest.ExpectedPlannedEndOn = nil, nil
	if !reflect.DeepEqual(rest, domain.PatchChangeRequestRequest{}) {
		return nil, nil, nil, &apierror.ValidationError{Msg: msgAcceptAlone}
	}
	if req.ExpectedCustomerUpdatedOn == nil {
		return nil, nil, nil, &apierror.ValidationError{Msg: msgAcceptNeedsExpectedProposal}
	}
	if req.ExpectedPlannedStartOn == nil || req.ExpectedPlannedEndOn == nil {
		return nil, nil, nil, &apierror.ValidationError{Msg: msgAcceptNeedsExpectedWindow}
	}
	if proposed, err = parseExpectedTimestamp("expectedCustomerUpdatedOn", req.ExpectedCustomerUpdatedOn); err != nil {
		return nil, nil, nil, err
	}
	if expStart, err = parseExpectedTimestamp("expectedPlannedStartOn", req.ExpectedPlannedStartOn); err != nil {
		return nil, nil, nil, err
	}
	if expEnd, err = parseExpectedTimestamp("expectedPlannedEndOn", req.ExpectedPlannedEndOn); err != nil {
		return nil, nil, nil, err
	}
	return proposed, expStart, expEnd, nil
}

// acceptCustomerProposal is "Accept proposed time": a staff PATCH
// {confirmCustomerUpdatedDate: "agree", ...} while the customer's proposal waits. It
// writes, in ONE UPDATE, WSO2's AGREE, the proposed start as the planned start (the
// planned length kept: the end is start + length), and the state SCHEDULED -- the change
// has not changed, so it does not go through CAB again, and the customer is not asked
// again: the proposal is their own consent. The still-requested customer rows are closed as
// every state change closes the rows of a stage the change has left (reconcileStaleApprovers).
//
// Not written, on purpose: is_customer_approval_required (no staff action records the
// customer's approval; the proposal is the customer's own consent, kept in
// customer_updated_on), customer_approval_required, any stage or approver row but the
// cancelling.
//
// It is another door out of Customer Approval whose only precondition is the customer's own
// recorded proposal; {state: "scheduled"} stays refused for every staff caller, proposal or
// not (customerOutcomeRefusal).
//
// In order, the first failing check wins and nothing is written until all pass: the shape
// (validateAcceptRequest: 400 -- and an external caller never gets here, the field is outside
// their four-field whitelist: 403); the change is visible to the caller (the caller's
// work_item UPDATE, the first statement); it is in Customer Approval (409); a proposal
// waits (409); the proposal is the one the caller saw (409); the planned window is the one
// the caller saw (409); a registered contact of the project is recorded as the proposer (409,
// errorCode change_request_proposer_not_recorded: resolveProposer, with the last writer read
// BEFORE this request's own write replaced it); the change is not on hold (400, the
// state-change gate's own text); the proposed start has not passed (409); the planned window
// has a length to keep (409); the window the proposal gives is within the range every planned
// window is held to (409: the column is written by the previous system too).
func acceptCustomerProposal(ctx context.Context, tx pgx.Tx, id string, req domain.PatchChangeRequestRequest, actorEmail string) (string, error) {
	expProposed, expStart, expEnd, err := validateAcceptRequest(req)
	if err != nil {
		return "", err
	}
	// work_item first, change_request second: the order every other PATCH takes. The lock stamps
	// the caller on work_item.updated_by, so the last writer it found (the proposer, when a
	// customer's proposal is what waits) is kept for the check below.
	locked, err := lockWorkItemKeepingWriter(ctx, tx, id, actorEmail)
	if err != nil {
		return "", err
	}
	gates, err := lockChangeRequestGateSnapshot(ctx, tx, id)
	if err != nil {
		return "", err
	}
	if gates.state != crStateCustomerApproval {
		return "", &apierror.ConflictError{Msg: msgAcceptNotInCustomerApproval(gates.state)}
	}
	f, err := readCustomerProposalFacts(ctx, tx, id)
	if err != nil {
		return "", err
	}
	if !f.pending {
		return "", &apierror.ConflictError{Msg: msgAcceptNothingWaiting}
	}
	if !expProposed.Equal(*f.proposed) {
		return "", &apierror.ConflictError{Msg: msgProposalChangedStart(*f.proposed)}
	}
	if err := expectedScheduleConflict(f.start, f.end, expStart, expEnd, readAgainSuffix); err != nil {
		return "", err
	}
	// No staff action stands in for the customer's answer, so a time is accepted only when a
	// registered contact of the project is recorded as having proposed it: the proposal is their
	// own consent. A date nobody is recorded as having written (a WSO2 user's, an earlier cycle's,
	// or a proposal that a later write has replaced as last writer) is never accepted for them.
	who, err := resolveProposer(ctx, tx, locked.projectID, locked.priorWriter)
	if err != nil {
		return "", err
	}
	if !who.known {
		return "", &apierror.ConflictError{Code: apierror.CodeChangeRequestProposerNotRecorded, Msg: msgAcceptProposerNotRecorded}
	}
	if f.onHold {
		return "", &apierror.ValidationError{Msg: msgAcceptOnHold}
	}
	if !f.proposed.After(time.Now()) {
		return "", &apierror.ConflictError{Msg: msgAcceptTimePassed(*f.proposed)}
	}
	newEnd := f.proposedEnd()
	if newEnd == nil {
		return "", &apierror.ConflictError{Msg: msgAcceptNoLength}
	}
	if newEnd.UTC().Year() > plannedYearMax {
		return "", &apierror.ConflictError{Msg: msgAcceptTooFarAhead(*f.proposed, *newEnd)}
	}

	ct, err := tx.Exec(ctx, `
		UPDATE change_request
		SET start_on = $2::text::timestamptz, end_on = $3::text::timestamptz,
		    customer_updated_date_confirmation = 'AGREE', state = 'SCHEDULED'
		WHERE id = $1 AND state = 'CUSTOMER_APPROVAL'`, id, fmtInstant(*f.proposed), fmtInstant(*newEnd))
	if err != nil {
		return "", fmt.Errorf("accept the proposed time: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return "", &apierror.ConflictError{Msg: msgAcceptNotInCustomerApproval(gates.state)}
	}
	if err := reconcileStaleApprovers(ctx, tx, id, actorEmail); err != nil {
		return "", err
	}
	return id, nil
}

// ---------------------------------------------------------------------------
// WSO2 proposes a different time / re-schedules
// ---------------------------------------------------------------------------

// timeResponse is what a staff {state: "authorize"} does to a change in Customer
// Approval, decided by planStaffTimeResponse before anything is written.
type timeResponse struct {
	// disagree: a customer's proposal waited, and this is WSO2's answer to it
	// (DISAGREE is written).
	disagree bool
	// asksAgain: the planned window changes, so the customers' standing request is
	// replaced by a fresh one (a Re-schedule, or a counter-proposal). False for a
	// decline that keeps the window: the customers keep their live request.
	asksAgain bool
}

// declineOnly: WSO2 answered a proposal with Disagree and kept the plan -- the answer is the
// only write (no state, no stage, no approver row: the customers' request stands, and nobody
// has to be found to ask).
func (t timeResponse) declineOnly() bool { return t.disagree && !t.asksAgain }

// planStaffTimeResponse judges a staff {state: "authorize"} against the change as it is
// now, under the row lock, and says what it will do. A proposal that waits -- a stored,
// unanswered time that a registered contact is recorded as having proposed -- is answered
// (the request must name it, expectedCustomerUpdatedOn, so a client that never saw a
// proposal can never answer one); with none waiting it is the plain Re-schedule, and so is
// a request that finds a stored time nobody is recorded as having proposed (it is not
// answered, and a request with no window is refused). Neither goes through CAB: the change
// has not changed, only its time. Refusals are in the doc of each branch; the first failing
// one wins and nothing has been written.
func planStaffTimeResponse(ctx context.Context, tx pgx.Tx, id string, req domain.PatchChangeRequestRequest, snap changeRequestGateSnapshot) (timeResponse, error) {
	expProposed, err := parseExpectedTimestamp("expectedCustomerUpdatedOn", req.ExpectedCustomerUpdatedOn)
	if err != nil {
		return timeResponse{}, err
	}
	expStart, err := parseExpectedTimestamp("expectedPlannedStartOn", req.ExpectedPlannedStartOn)
	if err != nil {
		return timeResponse{}, err
	}
	expEnd, err := parseExpectedTimestamp("expectedPlannedEndOn", req.ExpectedPlannedEndOn)
	if err != nil {
		return timeResponse{}, err
	}
	f, err := readCustomerProposalFacts(ctx, tx, id)
	if err != nil {
		return timeResponse{}, err
	}

	// A time waits for an answer of ours only when a customer is recorded as having proposed
	// it. One that nobody is recorded as having proposed (the date is stored, and the
	// predicate holds, but it was written by someone at WSO2 or is left over from an earlier
	// cycle) is no proposal: it is never answered, so a plain Re-schedule stays a plain
	// Re-schedule (no answer is written against a time nobody proposed) and a request with no
	// window is refused instead of declining it. The last writer is the one this PATCH's lock
	// found (snap.priorWriter): its own write has stamped the caller since.
	waits := false
	if f.pending {
		who, err := resolveProposer(ctx, tx, snap.projectID, snap.priorWriter)
		if err != nil {
			return timeResponse{}, err
		}
		waits = who.known
	}

	if !waits {
		// A plain Re-schedule: the planned time really changes (the diagram's "Time Change =
		// Yes") and somebody can be asked about it.
		if f.pending {
			// A time is stored that nobody is recorded as having proposed. A page that showed it
			// may name it; the name must be the stored time.
			if expProposed != nil && !expProposed.Equal(*f.proposed) {
				return timeResponse{}, &apierror.ConflictError{Msg: msgStoredTimeChanged(*f.proposed)}
			}
		} else if expProposed != nil {
			return timeResponse{}, &apierror.ConflictError{Msg: msgProposalNoLongerWaiting}
		}
		if err := expectedScheduleConflict(f.start, f.end, expStart, expEnd, readAgainSuffix); err != nil {
			return timeResponse{}, err
		}
		if f.pending && req.PlannedStartOn == nil && req.PlannedEndOn == nil {
			return timeResponse{}, &apierror.ValidationError{Msg: msgNoRecordedProposalToDecline}
		}
		if err := checkRescheduleWindow(ctx, tx, id, req.PlannedStartOn, req.PlannedEndOn); err != nil {
			return timeResponse{}, err
		}
		if err := requireSomebodyToAskForWindow(ctx, tx, id, snap, req.RequestedByID); err != nil {
			return timeResponse{}, err
		}
		return timeResponse{asksAgain: true}, nil
	}

	// A proposal waits: answer it, with the version the caller saw.
	if expProposed == nil || !expProposed.Equal(*f.proposed) {
		return timeResponse{}, &apierror.ConflictError{Msg: msgCounterProposalMissed(*f.proposed)}
	}
	if err := expectedScheduleConflict(f.start, f.end, expStart, expEnd, readAgainSuffix); err != nil {
		return timeResponse{}, err
	}
	if req.PlannedStartOn == nil && req.PlannedEndOn == nil {
		// Declined, the plan unchanged: the previous system's Disagree. The customers keep their
		// live request and nobody has to be found to ask.
		return timeResponse{disagree: true}, nil
	}
	win, err := judgeStaffWindow(f, req.PlannedStartOn, req.PlannedEndOn)
	if err != nil {
		return timeResponse{}, err
	}
	if err := win.refuseUnusable(); err != nil {
		return timeResponse{}, err
	}
	if win.sameAsProposal {
		return timeResponse{}, &apierror.ValidationError{Msg: msgCounterIsTheProposal}
	}
	if !win.changed {
		// Re-stating the planned window is "keep our time": the same decline.
		return timeResponse{disagree: true}, nil
	}
	if err := requireSomebodyToAskForWindow(ctx, tx, id, snap, req.RequestedByID); err != nil {
		return timeResponse{}, err
	}
	return timeResponse{disagree: true, asksAgain: true}, nil
}

// requireSomebodyToAskForWindow is the refusal a staff Re-schedule or counter-proposal
// gets when the customers it would ask cannot be asked: the very test Request Approval
// applies (requireSomebodyToAsk, customerGroupCanBeAsked), for the customer approval.
// A change in Customer Approval always needs the customer's approval, whatever an older
// row's box says.
func requireSomebodyToAskForWindow(ctx context.Context, q crQuerier, id string, snap changeRequestGateSnapshot, requestedBy **string) error {
	return requireSomebodyToAskWith(ctx, q, id, snap.projectID, true, false, requestedBy)
}

// windowFacts is a staff window judged against the stored one (judgeStaffWindow).
type windowFacts struct {
	// changed: the request moves the start or the end (a value equal to the stored
	// instant is not a move).
	changed bool
	// inverted / empty: the window that would result starts after, or exactly when, it ends.
	inverted, empty bool
	// sameAsProposal: the window that would result is the customer's proposal applied
	// to the plan (their start, the planned length) -- or, with no length to keep, their start.
	sameAsProposal bool
}

// judgeStaffWindow evaluates what a staff window (the normalised RFC 3339 values of
// normalizePatchPlannedWindow, nil when not sent) would do to the stored window and to
// the customer's proposal. Instants are compared in Go, at the microseconds a timestamptz
// holds, so nothing about the database session's time zone can matter.
func judgeStaffWindow(f customerProposalFacts, start, end *string) (windowFacts, error) {
	parse := func(field string, v *string) (*time.Time, error) {
		if v == nil {
			return nil, nil
		}
		t, err := time.Parse(time.RFC3339Nano, *v)
		if err != nil {
			return nil, plannedTimestampError(field)
		}
		t = t.UTC().Truncate(time.Microsecond)
		return &t, nil
	}
	gotStart, err := parse("plannedStartOn", start)
	if err != nil {
		return windowFacts{}, err
	}
	gotEnd, err := parse("plannedEndOn", end)
	if err != nil {
		return windowFacts{}, err
	}
	differs := func(got, stored *time.Time) bool {
		return got != nil && (stored == nil || !got.Equal(*stored))
	}
	var w windowFacts
	w.changed = differs(gotStart, f.start) || differs(gotEnd, f.end)
	effStart, effEnd := f.start, f.end
	if gotStart != nil {
		effStart = gotStart
	}
	if gotEnd != nil {
		effEnd = gotEnd
	}
	if effStart != nil && effEnd != nil {
		w.inverted = effStart.After(*effEnd)
		w.empty = effStart.Equal(*effEnd)
	}
	if effStart != nil && f.proposed != nil && effStart.Equal(*f.proposed) {
		if pe := f.proposedEnd(); pe == nil || (effEnd != nil && effEnd.Equal(*pe)) {
			w.sameAsProposal = true
		}
	}
	return w, nil
}

// refuseUnusable is the 400 of a window that cannot be a window.
func (w windowFacts) refuseUnusable() error {
	if w.inverted {
		return &apierror.ValidationError{Msg: "the planned start must not be after the planned end"}
	}
	if w.empty {
		return &apierror.ValidationError{Msg: "the planned start must not be the same as the planned end: the window must have a duration"}
	}
	return nil
}

// expectedScheduleConflict is the precondition of an answer that names the window it was
// given for: each bound named must equal the stored one, else 409. read says how the
// message ends ("read it again before giving your answer" for a customer's answer,
// readAgainSuffix for a staff one). nil bounds in the expectation are not checked.
func expectedScheduleConflict(start, end, expectedStart, expectedEnd *time.Time, read string) error {
	if expectedStart == nil && expectedEnd == nil {
		return nil
	}
	same := func(want, got *time.Time) bool {
		return want == nil || (got != nil && got.UTC().Truncate(time.Microsecond).Equal(*want))
	}
	if same(expectedStart, start) && same(expectedEnd, end) {
		return nil
	}
	show := func(t *time.Time) string {
		if t == nil {
			return "not set"
		}
		return fmtInstant(*t)
	}
	now := "no planned time is set"
	if start != nil || end != nil {
		now = fmt.Sprintf("%s to %s", show(start), show(end))
	}
	return &apierror.ConflictError{Code: apierror.CodeChangeRequestScheduleChanged, Msg: fmt.Sprintf(
		"the planned implementation time of this change request changed after you opened it (it is now %s); %s", now, read)}
}
