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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	autoclosureDefaultIntervalDays = 7
	autoclosureDefaultLimit        = 500
	autoclosureMaxLimit            = 5000
	autoclosureMaxIntervalDays     = 365

	// A ticket whose stage has failed this many times inside this many days is
	// left out of the list until the oldest failure ages out, so one ticket
	// that can never succeed (a close refused by validation) cannot raise the
	// same alert every run for ever. Its failures stay in the audit table.
	autoclosureMaxFailures       = 3
	autoclosureFailureWindowDays = 7
	autoclosureCleanupTimeout    = 15 * time.Second
)

// AutoclosureService runs the reminder-and-close workflow for tickets that sit
// in Awaiting Info or Solution Proposed with no activity. It is driven from
// outside (operations/csm-scheduled-tasks asks for the tickets due, then
// advances them one at a time) and decides nothing about scheduling itself.
type AutoclosureService interface {
	// FindCandidates lists the tickets due for a stage, oldest first.
	FindCandidates(ctx context.Context, req domain.AutoclosureCandidatesRequest) (domain.AutoclosureCandidatesResponse, error)
	// Advance performs a stage on one ticket if, and only if, it is still due.
	// It is safe to call twice: the second call finds the ticket already moved
	// on and reports it skipped.
	Advance(ctx context.Context, caseID string, req domain.AutoclosureAdvanceRequest) (domain.AutoclosureAdvanceResponse, error)
}

// autoclosureComments is the slice of CaseService the workflow needs. A
// narrow interface keeps this service testable without the whole case service.
type autoclosureComments interface {
	CreateCaseCommentAs(ctx context.Context, req domain.CreateCaseCommentRequest, actorEmail string) (domain.CreateCaseCommentResponse, error)
	GetCaseByID(ctx context.Context, id string) (domain.CaseView, error)
	UpdateCase(ctx context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error)
}

// autoclosureVerificationQueue is the slice of the Pending Verification
// repository the workflow needs to queue a closed case for the customer's own
// verification (see queueForVerification). repository.PendingVerificationRepository
// satisfies it.
type autoclosureVerificationQueue interface {
	GetWorkItemTypeAndProject(ctx context.Context, workItemID string) (workItemType string, projectID string, caseState *string, err error)
	GetProjectVerificationEnabled(ctx context.Context, projectID string) (bool, error)
	Create(ctx context.Context, req domain.CreatePendingVerificationRequest) (domain.PendingVerification, error)
}

type autoclosureService struct {
	repo  repository.AutoclosureRepository
	cases autoclosureComments
	// verification is optional: nil leaves a closed case out of the Pending
	// Verification queue, which is what a deployment without that feature wants.
	verification autoclosureVerificationQueue
	access       AccessService
	systemEmail  string
	now          func() time.Time
}

// NewAutoclosureService constructs the service. systemEmail is the system
// user the reminder and closure comments are written as and the close is
// attributed to. It is optional: empty (or blank) runs the workflow as the
// system itself, shown as domain.AutoclosureDefaultActor ("System user"), with
// no user row behind it, so closed_by_user_id stays empty.
func NewAutoclosureService(repo repository.AutoclosureRepository, cases CaseService, access AccessService, systemEmail string) AutoclosureService {
	systemEmail = strings.TrimSpace(systemEmail)
	if systemEmail == "" {
		systemEmail = domain.AutoclosureDefaultActor
	}
	return &autoclosureService{repo: repo, cases: cases, access: access, systemEmail: systemEmail, now: time.Now}
}

// unavailableAutoclosureService stands in where the workflow cannot run, so the
// routes answer an explicit 503 with the reason instead of a bare 404.
type unavailableAutoclosureService struct{ reason string }

// NewUnavailableAutoclosureService returns a service whose every call answers
// a 503 carrying reason.
func NewUnavailableAutoclosureService(reason string) AutoclosureService {
	return unavailableAutoclosureService{reason: reason}
}

func (u unavailableAutoclosureService) FindCandidates(context.Context, domain.AutoclosureCandidatesRequest) (domain.AutoclosureCandidatesResponse, error) {
	return domain.AutoclosureCandidatesResponse{}, &apierror.ServiceUnavailableError{Msg: u.reason}
}

func (u unavailableAutoclosureService) Advance(context.Context, string, domain.AutoclosureAdvanceRequest) (domain.AutoclosureAdvanceResponse, error) {
	return domain.AutoclosureAdvanceResponse{}, &apierror.ServiceUnavailableError{Msg: u.reason}
}

// WithPendingVerificationQueue makes the workflow queue every case it
// auto-closes for the customer's own verification (Pending Verification, reason
// AUTO_CLOSED), so a case closed for inactivity does not silently disappear from
// the customer's view. It is set in place, after construction, so existing
// callers and tests are unaffected. A svc that is not the real service is
// returned unchanged.
func WithPendingVerificationQueue(svc AutoclosureService, queue autoclosureVerificationQueue) AutoclosureService {
	if s, ok := svc.(*autoclosureService); ok {
		s.verification = queue
	}
	return svc
}

// autoclosureStageSpec is what a stage does to the workflow position.
type autoclosureStageSpec struct {
	// toStep is the step the ticket holds after the stage.
	toStep string
	// template is the comment kind posted, "" for none.
	template string
}

var autoclosureStages = map[domain.AutoclosureStage]autoclosureStageSpec{
	domain.AutoclosureStageReleaseHold:    {toStep: domain.AutoclosureStepDefault},
	domain.AutoclosureStageFirstReminder:  {toStep: domain.AutoclosureStepFirstComment, template: "first_reminder"},
	domain.AutoclosureStageSecondReminder: {toStep: domain.AutoclosureStepSecondComment, template: "second_reminder"},
	// The ticket keeps SECOND_COMMENT: it is closed, and a closed ticket is
	// never a candidate again whatever its step says.
	domain.AutoclosureStageClose: {toStep: domain.AutoclosureStepSecondComment, template: "closure"},
}

// requireInternal refuses anyone but an internal (WSO2 or machine) caller: the
// workflow comments on and closes tickets, and a customer must never be able
// to start it.
func (s *autoclosureService) requireInternal(ctx context.Context) error {
	return RequireInternalCaller(ctx, s.access, "this resource is only available to internal callers")
}

func (s *autoclosureService) validate(stage domain.AutoclosureStage, intervalDays, limit int, states []domain.CaseState) (days, lim int, stateLabels []string, err error) {
	if _, ok := autoclosureStages[stage]; !ok {
		return 0, 0, nil, &apierror.ValidationError{Msg: "stage must be one of release_hold, first_reminder, second_reminder, close"}
	}
	days = intervalDays
	if days == 0 {
		days = autoclosureDefaultIntervalDays
	}
	if days < 0 || days > autoclosureMaxIntervalDays {
		return 0, 0, nil, &apierror.ValidationError{Msg: fmt.Sprintf("intervalDays must be between 1 and %d", autoclosureMaxIntervalDays)}
	}
	lim = limit
	if lim == 0 {
		lim = autoclosureDefaultLimit
	}
	if lim < 0 || lim > autoclosureMaxLimit {
		return 0, 0, nil, &apierror.ValidationError{Msg: fmt.Sprintf("limit must be between 1 and %d", autoclosureMaxLimit)}
	}
	if len(states) == 0 {
		return days, lim, []string{"AWAITING_INFO", "SOLUTION_PROPOSED"}, nil
	}
	for _, st := range states {
		switch st {
		case domain.CaseStateAwaitingInfo, domain.CaseStateSolutionProposed:
			stateLabels = append(stateLabels, strings.ToUpper(string(st)))
		default:
			return 0, 0, nil, &apierror.ValidationError{Msg: "states may only contain awaiting_info and solution_proposed"}
		}
	}
	return days, lim, stateLabels, nil
}

// advancedToday is how many reminders and closures have been performed so far
// this UTC day, the number the daily limit is measured against. Dry-run rows are
// not in it: only ADVANCED events count.
func (s *autoclosureService) advancedToday(ctx context.Context) (int, error) {
	now := s.now().UTC()
	return s.repo.CountAdvancedSince(ctx, time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC))
}

// cleanupContext is a context for the writes that must still happen after the
// request was cancelled or timed out: putting a claimed step back and recording
// what happened. Without it a ticket claimed just before the caller gave up
// would stay marked as reminded with no reminder and no audit row.
func cleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), autoclosureCleanupTimeout)
}

// FindCandidates implements AutoclosureService.
func (s *autoclosureService) FindCandidates(ctx context.Context, req domain.AutoclosureCandidatesRequest) (domain.AutoclosureCandidatesResponse, error) {
	if err := s.requireInternal(ctx); err != nil {
		return domain.AutoclosureCandidatesResponse{}, err
	}
	days, limit, states, err := s.validate(req.Stage, req.IntervalDays, req.Limit, req.States)
	if err != nil {
		return domain.AutoclosureCandidatesResponse{}, err
	}
	if req.DailyLimit < 0 || req.DailyLimit > autoclosureMaxLimit*10 {
		return domain.AutoclosureCandidatesResponse{}, &apierror.ValidationError{Msg: "dailyLimit must not be negative or absurdly large"}
	}
	// With a daily limit and no explicit page size, one call may return up to
	// the whole day's budget instead of the 500 default, so a limit above 500
	// is not silently halved.
	if req.Limit == 0 && req.DailyLimit > 0 {
		limit = min(req.DailyLimit, autoclosureMaxLimit)
	}
	// Reminders and closures share one budget per UTC day, and a hold release
	// notifies nobody so it is exempt. What this day has already done counts
	// against it, which is what keeps a retried run inside the limit.
	if req.DailyLimit > 0 && req.Stage != domain.AutoclosureStageReleaseHold {
		done, err := s.advancedToday(ctx)
		if err != nil {
			return domain.AutoclosureCandidatesResponse{}, err
		}
		remaining := req.DailyLimit - done
		if remaining <= 0 {
			return domain.AutoclosureCandidatesResponse{Candidates: []domain.AutoclosureCandidate{}}, nil
		}
		if remaining < limit {
			limit = remaining
		}
	}
	now := s.now().UTC()
	rows, err := s.repo.FindCandidates(ctx, repository.AutoclosureCandidateQuery{
		Stage: req.Stage, Now: now, IntervalDays: days, Limit: limit,
		States: states, SystemEmail: s.systemEmail,
		MaxFailures: autoclosureMaxFailures, FailureWindowDays: autoclosureFailureWindowDays,
	})
	if err != nil {
		return domain.AutoclosureCandidatesResponse{}, err
	}
	// Make the skipping visible: a ticket left out for repeated failures is
	// otherwise silent.
	// The count goes back to the caller (the scheduled task reports it in its run
	// summary) as well as to the log. A failed count is logged, not swallowed.
	quarantined, qErr := s.repo.CountQuarantined(ctx, req.Stage, now, autoclosureFailureWindowDays, autoclosureMaxFailures)
	if qErr != nil {
		slog.WarnContext(ctx, "autoclosure: could not count the tickets left out after repeated failures", "stage", req.Stage, "error", qErr)
		quarantined = 0
	}
	if quarantined > 0 {
		slog.WarnContext(ctx, "autoclosure: tickets left out of the list after repeated failures; see case_autoclosure_event",
			"stage", req.Stage, "tickets", quarantined, "failures", autoclosureMaxFailures, "windowDays", autoclosureFailureWindowDays)
	}
	out := domain.AutoclosureCandidatesResponse{Candidates: make([]domain.AutoclosureCandidate, 0, len(rows)), Quarantined: quarantined}
	for _, r := range rows {
		out.Candidates = append(out.Candidates, r.AutoclosureCandidate)
	}
	return out, nil
}

// Advance implements AutoclosureService.
func (s *autoclosureService) Advance(ctx context.Context, caseID string, req domain.AutoclosureAdvanceRequest) (domain.AutoclosureAdvanceResponse, error) {
	if err := s.requireInternal(ctx); err != nil {
		return domain.AutoclosureAdvanceResponse{}, err
	}
	if err := validateUUIDs("id", []string{caseID}); err != nil {
		return domain.AutoclosureAdvanceResponse{}, err
	}
	days, _, states, err := s.validate(req.Stage, req.IntervalDays, 0, req.States)
	if err != nil {
		return domain.AutoclosureAdvanceResponse{}, err
	}
	if req.DailyLimit < 0 || req.DailyLimit > autoclosureMaxLimit*10 {
		return domain.AutoclosureAdvanceResponse{}, &apierror.ValidationError{Msg: "dailyLimit must not be negative or absurdly large"}
	}
	spec := autoclosureStages[req.Stage]
	now := s.now().UTC()
	resp := domain.AutoclosureAdvanceResponse{CaseID: caseID, Stage: req.Stage}
	skip := func(reason string) (domain.AutoclosureAdvanceResponse, error) {
		resp.Outcome = domain.AutoclosureOutcomeSkipped
		resp.Reason = reason
		return resp, nil
	}

	// Ask again, with the predicate that listed the ticket, whether it is still
	// due: it may since have been answered, put on hold, moved, closed or given
	// an open child. Nothing is trusted from whatever listed it.
	rows, err := s.repo.FindCandidates(ctx, repository.AutoclosureCandidateQuery{
		Stage: req.Stage, Now: now, IntervalDays: days, Limit: 1,
		States: states, OnlyID: caseID, SystemEmail: s.systemEmail,
		MaxFailures: autoclosureMaxFailures, FailureWindowDays: autoclosureFailureWindowDays,
	})
	if err != nil {
		return resp, err
	}
	if len(rows) == 0 {
		return skip("the ticket is not due for this stage any more")
	}
	c := rows[0]
	resp.FromStep = c.Step
	toStep := spec.toStep
	resp.ToStep = &toStep

	// The daily limit is checked here as well as when the list was made. Two
	// overlapping runs can read the same count when they list, and listing once
	// per stage lets a page overshoot; checking right before the claim stops
	// both. It is a read followed by a write, not one atomic step, so up to one
	// ticket per concurrent run can still slip past it. Hold releases notify
	// nobody and are exempt.
	if req.DailyLimit > 0 && req.Stage != domain.AutoclosureStageReleaseHold {
		done, cErr := s.advancedToday(ctx)
		if cErr != nil {
			return resp, cErr
		}
		if done >= req.DailyLimit {
			return skip("the daily limit has been reached")
		}
	}

	// A dry run stops here: the ticket is due and within the limit, which is
	// everything a real run would have decided. Nothing is claimed, posted,
	// closed or queued; the audit row says what would have happened and, being
	// DRY_RUN rather than ADVANCED, is not counted against the daily limit.
	if req.DryRun {
		s.record(ctx, caseID, req.Stage, c.Step, &toStep, "DRY_RUN", dryRunDetail(req.Stage, c, toStep))
		resp.Outcome = domain.AutoclosureOutcomeDryRun
		resp.Reason = dryRunDetail(req.Stage, c, toStep)
		return resp, nil
	}

	// Claim before acting. A second run reaching this point finds the step and
	// time already changed and loses, so a ticket is never reminded twice.
	claimed, err := s.repo.ClaimStep(ctx, c, spec.toStep, now)
	if err != nil {
		return resp, err
	}
	if !claimed {
		return skip("another run handled the ticket first")
	}

	var actErr error
	var detail string
	switch req.Stage {
	case domain.AutoclosureStageReleaseHold:
		// Nothing to write beyond the claim.
	case domain.AutoclosureStageClose:
		detail, actErr = s.closeCase(ctx, c, s.commentText(c.State, spec.template, req.Templates))
	default:
		_, actErr = s.cases.CreateCaseCommentAs(ctx, domain.CreateCaseCommentRequest{
			CaseID:  caseID,
			Type:    domain.CommentTypeComment,
			Content: s.commentText(c.State, spec.template, req.Templates),
		}, s.systemEmail)
	}
	// What follows must survive the request having been cancelled or timed out
	// (the caller gave up mid-action): it runs on a context that is detached from
	// the request's cancellation, with its own short timeout.
	cctx, cancel := cleanupContext(ctx)
	defer cancel()
	if actErr != nil {
		// Put the position back so the next run retries it, rather than leaving
		// a ticket marked as reminded that never was.
		if rErr := s.repo.RestoreStep(cctx, c, now); rErr != nil {
			slog.ErrorContext(ctx, "autoclosure: restore after failed action", "caseId", caseID, "stage", req.Stage, "error", rErr)
		}
		s.record(cctx, caseID, req.Stage, c.Step, &toStep, failureOutcome(actErr), actErr.Error())
		return resp, actErr
	}

	s.record(cctx, caseID, req.Stage, c.Step, &toStep, "ADVANCED", detail)
	resp.Outcome = domain.AutoclosureOutcomeAdvanced
	return resp, nil
}

// failureOutcome decides which audit outcome a failed action is recorded as.
// Only FAILED counts toward the failure cap that leaves a ticket out of the
// listing. The default is FAILED: an error nobody has classified (a constraint
// violation, a wrapped internal error from UpdateCase) repeats every run, and
// must end up quarantined rather than alerting daily for ever. Only errors that
// say nothing about the ticket are FAILED_TRANSIENT, which the cap ignores so a
// short outage cannot hide healthy tickets for a week: a cancelled request or
// deadline, an unavailable dependency, a network or dropped-connection error, and
// a database connection / resource / shutdown error.
func failureOutcome(err error) string {
	if isTransientFailure(err) {
		return "FAILED_TRANSIENT"
	}
	return "FAILED"
}

func isTransientFailure(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var unavailable *apierror.ServiceUnavailableError
	if errors.As(err, &unavailable) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && len(pgErr.Code) >= 2 {
		switch pgErr.Code[:2] {
		case "08", "53", "57": // connection exception, insufficient resources, operator intervention
			return true
		}
	}
	return pgconn.SafeToRetry(err)
}

// dryRunDetail says what a real run would have done to the ticket, for the
// audit row and the response of a dry run.
func dryRunDetail(stage domain.AutoclosureStage, c repository.AutoclosureCandidateRow, toStep string) string {
	from := "none"
	if c.Step != nil && *c.Step != "" {
		from = *c.Step
	}
	what := map[domain.AutoclosureStage]string{
		domain.AutoclosureStageReleaseHold:    "release the hold",
		domain.AutoclosureStageFirstReminder:  "send the first reminder",
		domain.AutoclosureStageSecondReminder: "send the second reminder",
		domain.AutoclosureStageClose:          "post the closure comment and close the ticket",
	}[stage]
	return fmt.Sprintf("dry run: would %s (%s, step %s -> %s, clock reference %s)",
		what, c.State, from, toStep, c.ReferenceTime.UTC().Format(time.RFC3339))
}

// closeCase closes the ticket as the system user and then writes the closure
// comment. The comment follows the close, not the other way round: a comment
// first and a refused close (an open child appeared, a concurrent edit) would
// leave a "this case is closed" comment on a case that is not.
//
// Awaiting Info tickets get the non-responsiveness resolution code (which
// keeps them out of the KB process); Solution Proposed tickets keep whatever
// resolution they were proposed with.
func (s *autoclosureService) closeCase(ctx context.Context, c repository.AutoclosureCandidateRow, comment string) (string, error) {
	view, err := s.cases.GetCaseByID(ctx, c.ID)
	if err != nil {
		return "", fmt.Errorf("read ticket before closing: %w", err)
	}
	closed := domain.CaseStateClosed
	update := domain.UpdateCaseRequest{ID: c.ID, State: &closed, CloseNotes: &comment}
	if view.Cause != nil {
		update.Cause = view.Cause
	} else {
		unknown := domain.CaseCauseUnknown
		update.Cause = &unknown
	}
	if c.State == domain.CaseStateAwaitingInfo {
		code := domain.CaseResolutionCodeAbruptlyClosedDueToNonResponsiveness
		update.ResolutionCode = &code
	} else if view.ResolutionCode != nil {
		update.ResolutionCode = view.ResolutionCode
	}

	// Closing is done as the system user, which is also what allows the one
	// move no staff table offers (Awaiting Info -> Closed).
	if _, err := s.cases.UpdateCase(withAutoclosureActor(ctx, s.systemEmail), update); err != nil {
		return "", fmt.Errorf("close ticket: %w", err)
	}
	if _, err := s.cases.CreateCaseCommentAs(ctx, domain.CreateCaseCommentRequest{
		CaseID: c.ID, Type: domain.CommentTypeComment, Content: comment,
	}, s.systemEmail); err != nil {
		// The ticket is closed; a missing closure comment is a lesser failure
		// than undoing the close. Logged, and the audit row still says ADVANCED.
		slog.ErrorContext(ctx, "autoclosure: ticket closed but its closure comment failed", "caseId", c.ID, "error", err)
	}
	return s.queueForVerification(ctx, c), nil
}

// autoclosureVerificationNote is the context a customer sees on the Pending
// Verification card of a case that was closed for them. Two reminders always
// precede the close, so the text can say so.
var autoclosureVerificationNote = map[domain.CaseState]string{
	domain.CaseStateAwaitingInfo:     "Automatically closed: no response to our request for information after two reminders.",
	domain.CaseStateSolutionProposed: "Automatically closed: no response to the proposed solution after two reminders.",
}

// queueForVerification adds a case the workflow has just closed to the
// customer's Pending Verification queue (reason AUTO_CLOSED, with the state it
// was closed from), so they can review and sign it off. It never fails the
// close: the case is already closed, and a missing queue entry is a lesser
// problem than undoing that. The returned text goes into the audit row.
//
// It follows the rules the Pending Verification feature itself enforces on
// this call (see pendingVerificationService.CreatePendingVerification), which
// it cannot reuse as is: that path needs an end-user token for "added by", and
// the workflow has none, so it writes through the repository as the system user.
//
//   - Cases only: a security report analysis is never queued.
//   - The project must have the feature turned on; otherwise this is a quiet no-op.
//   - One unverified entry per case: an existing one is success, not an error.
func (s *autoclosureService) queueForVerification(ctx context.Context, c repository.AutoclosureCandidateRow) string {
	if s.verification == nil || c.RowType != repository.AutoclosureRowCase {
		return ""
	}
	var previous domain.PendingVerificationPreviousStatus
	switch c.State {
	case domain.CaseStateAwaitingInfo:
		previous = domain.PendingVerificationPreviousStatusAwaitingInfo
	case domain.CaseStateSolutionProposed:
		previous = domain.PendingVerificationPreviousStatusSolutionProposed
	default:
		return ""
	}
	_, projectID, _, err := s.verification.GetWorkItemTypeAndProject(ctx, c.ID)
	if err != nil {
		slog.WarnContext(ctx, "autoclosure: could not look the closed case up for Pending Verification", "caseId", c.ID, "error", err)
		return "not queued for verification: lookup failed"
	}
	enabled, err := s.verification.GetProjectVerificationEnabled(ctx, projectID)
	if err != nil {
		slog.WarnContext(ctx, "autoclosure: could not read the project's Pending Verification setting", "caseId", c.ID, "error", err)
		return "not queued for verification: setting unreadable"
	}
	if !enabled {
		return "" // the project does not use Pending Verification
	}
	// The customer portal prints "Added by <this>" as it is stored, so store
	// what a person should read ("Support Team (auto-closure)"), not the system
	// user's email address. Falls back to the email if the name cannot be read.
	addedBy, nameErr := s.repo.DisplayNameOf(ctx, s.systemEmail)
	if nameErr != nil || strings.TrimSpace(addedBy) == "" {
		addedBy = s.systemEmail
	}
	note := autoclosureVerificationNote[c.State]
	var notePtr *string
	if note != "" {
		notePtr = &note
	}
	_, err = s.verification.Create(ctx, domain.CreatePendingVerificationRequest{
		WorkItemID:     c.ID,
		AddedReason:    domain.PendingVerificationAddedReasonAutoClosed,
		PreviousStatus: &previous,
		Note:           notePtr,
		AddedBy:        addedBy,
	})
	var conflict *apierror.ConflictError
	switch {
	case err == nil, errors.As(err, &conflict):
		return "queued for customer verification"
	default:
		slog.WarnContext(ctx, "autoclosure: closed case could not be queued for Pending Verification", "caseId", c.ID, "error", err)
		return "not queued for verification: " + err.Error()
	}
}

func (s *autoclosureService) record(ctx context.Context, caseID string, stage domain.AutoclosureStage, from, to *string, outcome, detail string) {
	if err := s.repo.RecordEvent(ctx, repository.AutoclosureEvent{
		WorkItemID: caseID, Stage: stage, FromStep: from, ToStep: to, Outcome: outcome, Detail: detail,
	}); err != nil {
		slog.ErrorContext(ctx, "autoclosure: audit row failed", "caseId", caseID, "stage", stage, "error", err)
	}
}

// commentText returns the comment for a state and stage, preferring a caller's
// override.
func (s *autoclosureService) commentText(state domain.CaseState, kind string, overrides map[string]string) string {
	key := string(state) + "." + kind
	if text, ok := overrides[key]; ok && strings.TrimSpace(text) != "" {
		return text
	}
	return autoclosureDefaultTemplates[key]
}

// autoclosureDefaultTemplates holds the comments sent to the customer, keyed
// "<state>.<kind>". They are defaults: a caller may send its own for any key
// (the scheduled task reads them from its configuration), so wording can
// change without a deploy of this service.
var autoclosureDefaultTemplates = map[string]string{
	"solution_proposed.first_reminder": "Hello,\n\n" +
		"This case will be automatically closed as a solution has been proposed and no further updates have been received. " +
		"Please let us know whether you were able to try out the proposed solution by either selecting Accept or Reject.\n\n" +
		"Thanks,\nSupport Team",
	"solution_proposed.second_reminder": "Hello,\n\n" +
		"Please let us know whether you need further support for this case. " +
		"Please note that if you are unable to respond, the case will be closed off after one week.\n\n" +
		"Thanks,\nSupport Team",
	"solution_proposed.closure": "Hello,\n\n" +
		"Please note that this case has been closed due to inactivity after the solution was suggested. " +
		"Thank you for contacting the Support Team.\n\n" +
		"Thanks,\nSupport Team",
	"awaiting_info.first_reminder": "Hello,\n\n" +
		"This case will be automatically closed as no further updates have been received. " +
		"Please share the requested details to continue the investigation of the issue.\n\n" +
		"Thanks,\nSupport Team",
	"awaiting_info.second_reminder": "Hello,\n\n" +
		"Gentle reminder for the requested details. If you are able to provide the information or wish to keep this case open, please reply to this message.\n\n" +
		"Please note that if you are unable to respond, the case will be closed off after one week.\n\n" +
		"Thanks,\nSupport Team",
	"awaiting_info.closure": "Hello,\n\n" +
		"Please note that this case is closed as we did not receive an update. " +
		"Thank you for contacting the Support Team.\n\n" +
		"Thanks,\nSupport Team",
}
