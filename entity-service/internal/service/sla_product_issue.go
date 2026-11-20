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
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/slacalendar"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/slacondition"
)

// The SLA engine for product issues (work items of type PRODUCT_ISSUE). The
// generic core in sla_subject.go does the policy decisions; this file supplies
// the subject (facts), and the four engine methods. Nothing here publishes an
// event: product-issue SLA rows must never reach csm-notification-service.

// Condition fields of a product issue's facts, the names an sla_policy
// condition (migration 0307) refers to.
const (
	piFieldPriority    = "priority"
	piFieldState       = "state"
	piFieldWaitsOn     = "waits_on"
	piFieldGitHubState = "github_state"
	piFieldCategory    = "category"
)

// Product issue statuses and waits_on values used when facts are derived (the
// vocabulary of work_item_status_event, migration 0304).
const (
	piStateOpen      = "OPEN"
	piStateClosed    = "CLOSED"
	piStateReopened  = "REOPENED"
	piStateResolved  = "RESOLVED"
	piStateWOC       = "WAITING_ON_CS"
	piWaitsCS        = "CS"
	piWaitsNobody    = "NOBODY"
	piWaitsProduct   = "PRODUCT_TEAM"
	piFamily         = "PRODUCT_ISSUE"
	piStageCancelled = "CANCELLED"
	piStageAchieved  = "ACHIEVED"
	piStagePaused    = "PAUSED"
	piStageRunning   = "IN_PROGRESS"
)

var errProductIssueSLAUnsupported = errors.New("sla engine: repository does not support product issue clocks")

// WorkItemSLAEngine is the product-issue half of the SLA engine. Every method
// is best-effort like the rest of the engine: failures are logged and never
// reach the caller as a panic, and the first failure is also returned so
// callers and tests can see it. None publishes an event.
type WorkItemSLAEngine interface {
	// RegisterWorkItemClocks registers a clock for every policy that applies to
	// the issue, back-dated to its creation, then rebuilds it from the issue's
	// status history.
	RegisterWorkItemClocks(ctx context.Context, workItemID string) error
	// ApplyWorkItemStatusEffects applies a stored status event. newest says
	// whether it is the newest stored event: then it is applied inline at the
	// event's own time (pause, resume or complete; a reopen registers a new
	// clock). An older event is never replayed inline: the clocks are rebuilt
	// from history instead.
	ApplyWorkItemStatusEffects(ctx context.Context, workItemID string, at time.Time, newest bool) error
	// ReviseWorkItemClocks handles a priority change: clocks whose policy no
	// longer applies are cancelled at `at`, the applicable policy gets a clock
	// back-dated to the issue's creation, and the clocks are rebuilt.
	ReviseWorkItemClocks(ctx context.Context, workItemID string, at time.Time) error
	// RecalculateWorkItemClocks rebuilds the newest clock of every policy of the
	// issue from the full status timeline, deterministically.
	RecalculateWorkItemClocks(ctx context.Context, workItemID string) error
}

var _ WorkItemSLAEngine = (*slaEngineService)(nil)

// productIssueFacts is what a product-issue policy condition is evaluated on.
type productIssueFacts struct {
	priority    string // "" when the issue has none (the field is then absent)
	category    string
	state       string // effective normalised status, see productIssueState
	waitsOn     string
	githubState string
}

func (f productIssueFacts) Field(name string) (string, bool) {
	var v string
	switch name {
	case piFieldPriority:
		v = f.priority
	case piFieldState:
		v = f.state
	case piFieldWaitsOn:
		v = f.waitsOn
	case piFieldGitHubState:
		v = f.githubState
	case piFieldCategory:
		v = f.category
	default:
		return "", false
	}
	return v, v != ""
}

func (f productIssueFacts) Present(name string) bool {
	v, ok := f.Field(name)
	return ok && v != ""
}

func (f productIssueFacts) Date(string) (time.Time, bool) { return time.Time{}, false }

// productIssueState is the issue's effective status: CLOSED when GitHub says
// closed; else REOPENED when the board still says RESOLVED/CLOSED but a GitHub
// reopen event is newer than the latest board event (the card was not moved
// back, P-45: a reopen restarts the clock); else the board status; else OPEN
// (not on a board). boardAt/reopenAt are the times of the latest board event and
// of the latest GitHub reopen event (nil: none).
func productIssueState(githubState string, boardStatus string, boardAt, reopenAt *time.Time) string {
	switch {
	case githubState == piStateClosed:
		return piStateClosed
	case (boardStatus == piStateResolved || boardStatus == piStateClosed) && reopenAt != nil && (boardAt == nil || reopenAt.After(*boardAt)):
		return piStateReopened
	case boardStatus != "":
		return boardStatus
	default:
		return piStateOpen
	}
}

// productIssueWaitsOn is who the issue waits on in state: "CS" in
// WAITING_ON_CS always, NOBODY once resolved or closed, else what the status
// map gave the board move that put it there (boardWaitsOn), else the product team.
func productIssueWaitsOn(state, boardWaitsOn string) string {
	switch {
	case state == piStateWOC:
		return piWaitsCS
	case state == piStateClosed || state == piStateResolved:
		return piWaitsNobody
	case boardWaitsOn != "":
		return boardWaitsOn
	default:
		return piWaitsProduct
	}
}

func nonNilString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// productIssueFactsFrom builds the current facts from the stored rows.
func productIssueFactsFrom(f repository.ProductIssueSLAFacts) productIssueFacts {
	board := nonNilString(f.BoardStatus)
	state := productIssueState(f.GitHubState, board, f.LatestBoardEventOn, f.LatestReopenEventOn)
	waits := ""
	if state == board {
		waits = f.BoardWaitsOn
	}
	return productIssueFacts{
		priority: nonNilString(f.Priority), category: nonNilString(f.Category),
		state: state, waitsOn: productIssueWaitsOn(state, waits), githubState: githubStateOrOpen(f.GitHubState),
	}
}

func githubStateOrOpen(s string) string {
	if s == "" {
		return piStateOpen
	}
	return s
}

// identityFacts is the same issue with its status neutralised: it answers "does
// this policy belong to this issue at all", whatever state the issue is in.
func (f productIssueFacts) identityFacts() productIssueFacts {
	f.state, f.waitsOn, f.githubState = piStateOpen, piWaitsProduct, piStateOpen
	return f
}

var productIssueSLALogs = &slaSubjectLogs{
	idKey:                "workItemId",
	noPolicy:             "sla engine: not registering product issue clocks, no applicable policy matched",
	registerFailed:       "sla engine: register product issue clock failed",
	alreadyRegistered:    "sla engine: product issue clock already registered, skipped",
	reviseSkipped:        "sla engine: revise product issue clocks skipped, a policy lookup failed",
	reviseRetry:          "sla engine: revise product issue clocks failed, retrying once",
	reviseFailed:         "sla engine: revise product issue clocks for priority change failed",
	reviseCancelled:      "sla engine: cancelled product issue clocks for priority change",
	activeLookupFailed:   "sla engine: apply product issue status effects: active clock policies lookup failed",
	reevalRegisterFailed: "sla engine: apply product issue status effects: register newly-applicable clock failed",
	effectsPrefix:        "sla engine: apply product issue status effects",
}

// productIssueSLASubject is a product issue with its facts loaded.
type productIssueSLASubject struct {
	id        string
	facts     productIssueFacts
	createdOn time.Time
	// closedOn/updatedOn are the link's github_closed_on/github_updated_on: the
	// best known instants for a state the status timeline does not record.
	closedOn, updatedOn *time.Time
}

func productIssueSubjectFrom(id string, f repository.ProductIssueSLAFacts) productIssueSLASubject {
	return productIssueSLASubject{id: id, facts: productIssueFactsFrom(f), createdOn: f.CreatedOn, closedOn: f.GitHubClosedOn, updatedOn: f.GitHubUpdatedOn}
}

func (p productIssueSLASubject) Kind() string              { return "product_issue" }
func (p productIssueSLASubject) ID() string                { return p.id }
func (p productIssueSLASubject) Facts() slacondition.Facts { return p.facts }
func (p productIssueSLASubject) CreatedOn() time.Time      { return p.createdOn }

// TerminalNow is false: there is no CLOSED safety net, the policy's own stop
// condition completes the clock in a terminal state.
func (p productIssueSLASubject) TerminalNow() bool { return false }

// IsPolicy is unused: a product issue resolves by Family, never by markers.
func (p productIssueSLASubject) IsPolicy(...string) bool { return true }
func (p productIssueSLASubject) Family() string          { return piFamily }
func (p productIssueSLASubject) logs() *slaSubjectLogs   { return productIssueSLALogs }

// ConditionText: policies of this family have source CSM, their text is the
// neutral grammar as written.
func (p productIssueSLASubject) ConditionText(_ repository.SLAPolicyRef, raw string) string {
	return raw
}

// productIssueTimedSLASubject is the subject of an inline status event: the
// core applies it at the event's own time.
type productIssueTimedSLASubject struct {
	productIssueSLASubject
	at time.Time
}

func (p productIssueTimedSLASubject) EventTime() time.Time { return p.at }

var (
	_ slaSubject           = productIssueSLASubject{}
	_ slaSubject           = productIssueTimedSLASubject{}
	_ slaEventTimedSubject = productIssueTimedSLASubject{}
)

// loadProductIssueSubject reads the issue's current facts.
func (s *slaEngineService) loadProductIssueSubject(ctx context.Context, workItemID string) (productIssueSLASubject, error) {
	if s.pi == nil {
		return productIssueSLASubject{}, errProductIssueSLAUnsupported
	}
	f, err := s.pi.GetProductIssueSLAFacts(ctx, workItemID)
	if err != nil {
		slog.ErrorContext(ctx, "sla engine: load product issue facts failed", "workItemId", workItemID, "err", err)
		return productIssueSLASubject{}, err
	}
	return productIssueSubjectFrom(workItemID, f), nil
}

// RegisterWorkItemClocks implements WorkItemSLAEngine.
func (s *slaEngineService) RegisterWorkItemClocks(ctx context.Context, workItemID string) error {
	return s.RecalculateWorkItemClocks(ctx, workItemID)
}

// RecalculateWorkItemClocks implements WorkItemSLAEngine. It registers a clock
// for every policy that applies now and has no open clock, then rebuilds the
// newest non-cancelled clock of every policy that belongs to the issue (by its
// identity, whatever its state, so an ACHIEVED clock of a resolved issue is
// rebuilt too). A relevant policy with no sla row at all gets one even when the
// issue is already resolved or closed (an imported issue), which then ends
// ACHIEVED where the terminal run began.
func (s *slaEngineService) RecalculateWorkItemClocks(ctx context.Context, workItemID string) error {
	return s.syncClocksLocked(ctx, workItemID, true)
}

// syncClocksLocked resolves the policies (every pool read happens here, before
// the transaction), then registers and rebuilds under the per-issue lock.
// createMissing adds the "relevant policy with no row at all" registration
// (Recalculate only).
func (s *slaEngineService) syncClocksLocked(ctx context.Context, workItemID string, createMissing bool) error {
	if s.pi == nil {
		return errProductIssueSLAUnsupported
	}
	subj, err := s.loadProductIssueSubject(ctx, workItemID)
	if err != nil {
		return err
	}
	var errs []error
	applicable, lookupFailed := s.resolveApplicablePolicies(ctx, subj)
	if lookupFailed {
		errs = append(errs, errors.New("sla engine: product issue policy lookup failed"))
	}
	relevant, err := s.relevantPolicies(ctx, subj)
	if err != nil {
		return errors.Join(append(errs, err)...)
	}
	errs = append(errs, s.runLocked(ctx, workItemID, applicable, relevant, true, createMissing))
	return errors.Join(errs...)
}

// runLocked is the transactional half: under the per-issue lock it reads the
// facts, registers the applicable policies (when register) and rebuilds the
// newest clock of every relevant one. The policy lists are resolved by the
// caller: the callback uses only tx, never the pool.
func (s *slaEngineService) runLocked(ctx context.Context, workItemID string, applicable, relevant []repository.SLAPolicyRef, register, createMissing bool) error {
	err := s.pi.WithProductIssueLock(ctx, workItemID, func(ctx context.Context, tx repository.ProductIssueSLATx) error {
		subj, err := s.loadProductIssueSubjectTx(ctx, workItemID, tx)
		if err != nil {
			return err
		}
		var errs []error
		if register {
			for _, p := range applicable {
				if _, err := s.registerClock(ctx, tx.RegisterClock, subj, p); err != nil {
					errs = append(errs, err)
				}
			}
		}
		errs = append(errs, s.rebuildRelevantClocks(ctx, tx, subj, relevant, createMissing)...)
		return errors.Join(errs...)
	})
	if err != nil {
		slog.ErrorContext(ctx, "sla engine: product issue clock sync failed", "workItemId", workItemID, "err", err)
	}
	return err
}

// loadProductIssueSubjectTx is loadProductIssueSubject inside a locked transaction.
func (s *slaEngineService) loadProductIssueSubjectTx(ctx context.Context, workItemID string, tx repository.ProductIssueSLATx) (productIssueSLASubject, error) {
	f, err := tx.Facts(ctx)
	if err != nil {
		return productIssueSLASubject{}, err
	}
	return productIssueSubjectFrom(workItemID, f), nil
}

// registerClock registers policy back-dated to the issue's creation through reg
// unless it already has an open clock (RegisterWorkItemClock's own guard). It
// reports whether a row was inserted.
func (s *slaEngineService) registerClock(ctx context.Context, reg func(context.Context, repository.SLAPolicyRef, time.Time) (bool, error), subj productIssueSLASubject, p repository.SLAPolicyRef) (bool, error) {
	registered, err := reg(ctx, p, subj.createdOn)
	if err != nil {
		slog.ErrorContext(ctx, productIssueSLALogs.registerFailed, "workItemId", subj.id, "clockType", p.Target, "policy", p.Name, "err", err)
		return false, err
	}
	if !registered {
		slog.DebugContext(ctx, productIssueSLALogs.alreadyRegistered, "workItemId", subj.id, "clockType", p.Target, "policy", p.Name)
	}
	return registered, nil
}

// relevantPolicies lists the policies that belong to the issue whatever its
// status (start condition evaluated on its identity facts).
func (s *slaEngineService) relevantPolicies(ctx context.Context, subj productIssueSLASubject) ([]repository.SLAPolicyRef, error) {
	identity := subj
	identity.facts = subj.facts.identityFacts()
	var out []repository.SLAPolicyRef
	for _, clockType := range slaTargetOrder {
		matches, err := s.resolver.resolveSubject(ctx, slaClockTypeTarget[clockType], identity)
		if err != nil {
			return nil, err
		}
		out = append(out, matches...)
	}
	return out, nil
}

// rebuildRelevantClocks rebuilds the newest non-cancelled clock of every
// given relevant policy from the full timeline (tx only, no pool reads).
func (s *slaEngineService) rebuildRelevantClocks(ctx context.Context, tx repository.ProductIssueSLATx, subj productIssueSLASubject, policies []repository.SLAPolicyRef, createMissing bool) []error {
	var errs []error
	for _, p := range policies {
		if createMissing {
			if _, ok, err := tx.LatestClockRow(ctx, p.ID); err != nil {
				errs = append(errs, err)
				continue
			} else if !ok {
				if _, err := s.registerClock(ctx, tx.RegisterClock, subj, p); err != nil {
					errs = append(errs, err)
					continue
				}
			}
		}
		if err := s.rebuildLatestClock(ctx, tx, subj, p); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// rebuildLatestClock restates policy's newest sla row (unless none or
// CANCELLED) from the issue's status timeline.
func (s *slaEngineService) rebuildLatestClock(ctx context.Context, tx repository.ProductIssueSLATx, subj productIssueSLASubject, p repository.SLAPolicyRef) error {
	row, ok, err := tx.LatestClockRow(ctx, p.ID)
	if err != nil {
		slog.ErrorContext(ctx, "sla engine: rebuild product issue clock: latest row lookup failed", "workItemId", subj.id, "policy", p.Name, "err", err)
		return err
	}
	if !ok || row.Stage == piStageCancelled {
		return nil
	}
	events, err := tx.Timeline(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "sla engine: rebuild product issue clock: timeline failed", "workItemId", subj.id, "policy", p.Name, "err", err)
		return err
	}
	now := s.clock()
	segs := piWithCurrentState(productIssueSegments(subj.facts, events), subj.facts, events, subj.createdOn, now, subj.closedOn, subj.updatedOn)
	st := rebuildProductIssueClock(p, subj.createdOn, now, segs)
	if err := tx.RewriteClock(ctx, row.ID, st); err != nil {
		slog.ErrorContext(ctx, "sla engine: rebuild product issue clock: write failed", "workItemId", subj.id, "policy", p.Name, "err", err)
		return err
	}
	return nil
}

// ApplyWorkItemStatusEffects implements WorkItemSLAEngine.
func (s *slaEngineService) ApplyWorkItemStatusEffects(ctx context.Context, workItemID string, at time.Time, newest bool) error {
	if !newest {
		return s.RecalculateWorkItemClocks(ctx, workItemID)
	}
	base, err := s.loadProductIssueSubject(ctx, workItemID)
	if err != nil {
		return err
	}
	subj := productIssueTimedSLASubject{productIssueSLASubject: base, at: at.UTC()}
	errs := []error{s.applyEffectsToOpenClocks(ctx, subj)}

	// A policy that now applies and has no open clock is a reopen (or the first
	// event of an issue never registered): register it back-dated and rebuild it
	// so the history before now counts.
	applicable, lookupFailed := s.resolveApplicablePolicies(ctx, base)
	if lookupFailed {
		errs = append(errs, errors.New("sla engine: product issue policy lookup failed"))
	}
	reg := func(ctx context.Context, p repository.SLAPolicyRef, startOn time.Time) (bool, error) {
		return s.repo.RegisterWorkItemClock(ctx, workItemID, p, startOn)
	}
	for _, p := range applicable {
		if _, err := s.registerClock(ctx, reg, base, p); err != nil {
			errs = append(errs, err)
		}
	}
	// The inline updates above are the event-time writes. A rebuild running at the
	// same time may have read the timeline before this event committed and then
	// overwritten them, so always end with a locked rebuild: it reads committed
	// state and produces the same figures. It also repairs a registration whose
	// own rebuild failed.
	relevant, err := s.relevantPolicies(ctx, base)
	if err != nil {
		return errors.Join(append(errs, err)...)
	}
	errs = append(errs, s.runLocked(ctx, workItemID, nil, relevant, false, false))
	return errors.Join(errs...)
}

// ReviseWorkItemClocks implements WorkItemSLAEngine.
func (s *slaEngineService) ReviseWorkItemClocks(ctx context.Context, workItemID string, at time.Time) error {
	subj, err := s.loadProductIssueSubject(ctx, workItemID)
	if err != nil {
		return err
	}
	lg := productIssueSLALogs
	applicable, lookupFailed := s.resolveApplicablePolicies(ctx, subj)
	if lookupFailed {
		// An incomplete list would cancel clocks with no replacement committed.
		slog.ErrorContext(ctx, lg.reviseSkipped, lg.idKey, workItemID)
		return errors.New("sla engine: product issue policy lookup failed")
	}
	keep := make(map[string]bool, len(applicable))
	for _, p := range applicable {
		keep[p.ID] = true
	}
	var errs []error
	cancelled := 0
	for _, clockType := range slaTargetOrder {
		target := slaClockTypeTarget[clockType]
		open, err := s.repo.ActiveClockPolicies(ctx, workItemID, target)
		if err != nil {
			slog.ErrorContext(ctx, lg.activeLookupFailed, lg.idKey, workItemID, "clockType", target, "err", err)
			errs = append(errs, err)
			continue
		}
		for _, p := range open {
			if p.AppliesTo != piFamily || keep[p.ID] {
				continue
			}
			done, err := s.repo.CancelClockByPolicyAt(ctx, workItemID, target, p.ID, at.UTC())
			if err != nil {
				slog.ErrorContext(ctx, lg.reviseFailed, lg.idKey, workItemID, "policy", p.Name, "err", err)
				errs = append(errs, err)
				continue
			}
			if done {
				cancelled++
			}
		}
	}
	if cancelled > 0 {
		slog.InfoContext(ctx, lg.reviseCancelled, lg.idKey, workItemID, "count", cancelled)
	}
	relevant, err := s.relevantPolicies(ctx, subj)
	if err != nil {
		return errors.Join(append(errs, err)...)
	}
	errs = append(errs, s.runLocked(ctx, workItemID, applicable, relevant, true, false))
	return errors.Join(errs...)
}

// piSegment is a stretch of the issue's life with one set of facts, from `from`
// to the next segment's `from` (open-ended for the last).
type piSegment struct {
	from  time.Time // the zero time for the first segment: from the beginning
	facts productIssueFacts
}

// productIssueSegments replays the timeline: the latest board status (events
// with a raw status) and open/closed (GitHub close and reopen events, which
// have none) are tracked separately; the effective state is productIssueState
// of the two. Priority and category are the issue's current ones throughout.
func productIssueSegments(current productIssueFacts, events []repository.ProductIssueStatusEvent) []piSegment {
	closed := false
	board, boardWaits := "", ""
	var boardAt, reopenAt *time.Time
	mk := func() productIssueFacts {
		gh := piStateOpen
		if closed {
			gh = piStateClosed
		}
		state := productIssueState(gh, board, boardAt, reopenAt)
		waits := ""
		if state == board {
			waits = boardWaits
		}
		return productIssueFacts{
			priority: current.priority, category: current.category,
			state: state, waitsOn: productIssueWaitsOn(state, waits), githubState: gh,
		}
	}
	segs := []piSegment{{facts: mk()}}
	for _, ev := range events {
		switch {
		case ev.IsBoardMove:
			at := ev.OccurredAt.UTC()
			board, boardWaits, boardAt = ev.ToStatus, ev.WaitsOn, &at
		case ev.ToStatus == piStateClosed:
			closed = true
		case ev.ToStatus == piStateReopened:
			at := ev.OccurredAt.UTC()
			closed, reopenAt = false, &at
		default:
			continue
		}
		segs = append(segs, piSegment{from: ev.OccurredAt.UTC(), facts: mk()})
	}
	return segs
}

// piWithCurrentState appends a synthetic final segment when the issue's current
// state (from the link) differs from where the timeline ends: Phase 1's backfill
// and reconcile write board history but no CLOSED/REOPENED events, so a closed
// issue whose last board event is In Progress would otherwise run forever. The
// segment starts at the best known instant: github_closed_on for CLOSED, else the
// later of the last event and creation, or github_updated_on when later than that
// and not in the future. It is never earlier than the last event or creation.
func piWithCurrentState(segs []piSegment, cur productIssueFacts, events []repository.ProductIssueStatusEvent, createdOn, now time.Time, closedOn, updatedOn *time.Time) []piSegment {
	last := segs[len(segs)-1].facts
	if last.state == cur.state && last.githubState == cur.githubState {
		return segs
	}
	from := createdOn.UTC()
	if n := len(events); n > 0 && events[n-1].OccurredAt.After(from) {
		from = events[n-1].OccurredAt.UTC()
	}
	if cur.state == piStateClosed {
		if closedOn != nil && closedOn.UTC().After(from) {
			from = closedOn.UTC()
		}
	} else if updatedOn != nil && updatedOn.UTC().After(from) && !updatedOn.UTC().After(now) {
		from = updatedOn.UTC()
	}
	return append(segs, piSegment{from: from, facts: cur})
}

type piSegClass int

const (
	piAccrues piSegClass = iota
	piPausedSeg
	piTerminalSeg
)

func piLaterOf(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func piEarlierOf(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// rebuildProductIssueClock restates a clock from the issue's segments. The
// clock starts at createdOn. A segment does not accrue when the policy's pause
// or stop condition is true for its facts (so waiting on CS, resolved and
// closed time never accrue). Currently terminal: ACHIEVED, ended where the
// trailing run of terminal segments starts. Currently paused: PAUSED, paused_on
// where the trailing pause run starts. Else IN_PROGRESS. pause_duration is the
// wall-clock non-accruing time before that point, business_pause_duration the
// same intervals on the policy's calendar, and the snapshot columns are the
// clock's figures as of end_on, paused_on or now.
func rebuildProductIssueClock(policy repository.SLAPolicyRef, createdOn, now time.Time, segs []piSegment) repository.ProductIssueClockState {
	cal, _ := slacalendar.For(policy.Schedule)
	var stop, pause slacondition.Condition
	if policy.StopCondition != "" {
		stop = slacondition.Parse(policy.StopCondition)
	}
	if policy.PauseCondition != "" {
		pause = slacondition.Parse(policy.PauseCondition)
	}
	class := func(i int) piSegClass {
		f := segs[i].facts
		switch {
		case stop != nil && stop.Evaluate(f):
			return piTerminalSeg
		case pause != nil && pause.Evaluate(f):
			return piPausedSeg
		default:
			return piAccrues
		}
	}

	start := createdOn.UTC()
	n := len(segs)
	st := repository.ProductIssueClockState{StartOn: start}
	var snapAt, windowEnd time.Time
	switch class(n - 1) {
	case piTerminalSeg:
		i := n - 1
		for i > 0 && class(i-1) == piTerminalSeg {
			i--
		}
		end := piLaterOf(segs[i].from, start)
		st.Stage, st.EndOn = piStageAchieved, &end
		snapAt, windowEnd = end, end
	case piPausedSeg:
		i := n - 1
		for i > 0 && class(i-1) == piPausedSeg {
			i--
		}
		pausedOn := piLaterOf(segs[i].from, start)
		st.Stage, st.PausedOn = piStagePaused, &pausedOn
		snapAt, windowEnd = pausedOn, pausedOn
	default:
		st.Stage = piStageRunning
		snapAt, windowEnd = now.UTC(), now.UTC()
	}

	for i := range segs {
		if class(i) == piAccrues {
			continue
		}
		lo := piLaterOf(segs[i].from, start)
		hi := windowEnd
		if i < n-1 {
			hi = piEarlierOf(segs[i+1].from, windowEnd)
		}
		if hi.After(lo) {
			st.PauseDuration += hi.Sub(lo)
			st.BusinessPauseDuration += cal.Elapsed(lo, hi)
		}
	}

	elapsed := cal.Elapsed(start, snapAt) - st.BusinessPauseDuration
	if elapsed < 0 {
		elapsed = 0
	}
	st.BusinessDuration = elapsed
	st.RemainingDuration = policy.Duration - elapsed
	if st.RemainingDuration < 0 {
		st.RemainingDuration = 0
	}
	if policy.Duration > 0 {
		st.ElapsedPercentage = elapsed.Seconds() / policy.Duration.Seconds() * 100
	}
	st.HasBreached = elapsed >= policy.Duration
	return st
}
