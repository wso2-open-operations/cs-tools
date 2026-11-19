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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/slacondition"
)

// slaSubject is one work item the SLA engine evaluates policies for: a case or
// an incident today. It carries everything the generic core in this file needs
// that differs between kinds -- the facts, which policy family applies, how a
// policy's condition text is translated, when the "terminal" safety net fires,
// and which log messages and attribute key to use -- so reevaluate, register
// and revise exist once instead of once per kind.
type slaSubject interface {
	// Kind names the subject ("case", "incident"); the policy resolver uses it
	// in its log messages.
	Kind() string
	// ID is the work item id the repository calls are keyed on.
	ID() string
	// Facts is what policy conditions are evaluated against.
	Facts() slacondition.Facts
	// CreatedOn is the instant a retroactive policy's clock is backdated to.
	CreatedOn() time.Time
	// TerminalNow reports whether the safety net applies: every clock still
	// open on the subject is completed whatever its own stop_condition says.
	TerminalNow() bool
	// ConditionText translates a policy's raw condition text into the neutral
	// form internal/slacondition parses.
	ConditionText(candidate repository.SLAPolicyRef, raw string) string
	// IsPolicy is the policy-family filter: whether a candidate whose
	// conditions are given describes this kind of subject at all.
	IsPolicy(conditions ...string) bool

	logs() *slaSubjectLogs
}

// slaSubjectLogs holds a subject kind's log wording. The engine's messages
// differ per kind (and are asserted on by operators), so they are data here
// rather than being derived from Kind().
type slaSubjectLogs struct {
	idKey string // "caseId" / "incidentId"

	noPolicy          string
	registerFailed    string
	alreadyRegistered string

	reviseSkipped   string
	reviseRetry     string
	reviseFailed    string
	reviseCancelled string

	activeLookupFailed   string
	reevalRegisterFailed string

	// effectsPrefix prefixes the per-policy action failures
	// ("<prefix>: complete clock failed", ...).
	effectsPrefix string
}

var caseSLALogs = &slaSubjectLogs{
	idKey:                "caseId",
	noPolicy:             "sla engine: not registering clocks, no applicable policy matched",
	registerFailed:       "sla engine: register clock failed",
	alreadyRegistered:    "sla engine: clock already registered, skipped",
	reviseSkipped:        "sla engine: revise clocks skipped, a policy lookup failed",
	reviseRetry:          "sla engine: revise clocks failed, retrying once",
	reviseFailed:         "sla engine: revise clocks for severity change failed after retry",
	reviseCancelled:      "sla engine: cancelled active clocks for severity change",
	activeLookupFailed:   "sla engine: reevaluate case clocks: active clock policies lookup failed",
	reevalRegisterFailed: "sla engine: reevaluate case clocks: register newly-applicable clock failed",
	effectsPrefix:        "sla engine: apply case state effects",
}

var incidentSLALogs = &slaSubjectLogs{
	idKey:                "incidentId",
	noPolicy:             "sla engine: not registering incident clocks, no applicable policy matched",
	registerFailed:       "sla engine: register incident clock failed",
	alreadyRegistered:    "sla engine: incident clock already registered, skipped",
	reviseSkipped:        "sla engine: revise incident clocks skipped, a policy lookup failed",
	reviseRetry:          "sla engine: revise incident clocks failed, retrying once",
	reviseFailed:         "sla engine: revise incident clocks for priority change failed after retry",
	reviseCancelled:      "sla engine: cancelled active incident clocks for priority change",
	activeLookupFailed:   "sla engine: apply incident state effects: active clock policies lookup failed",
	reevalRegisterFailed: "sla engine: apply incident state effects: register newly-applicable clock failed",
	effectsPrefix:        "sla engine: apply incident state effects",
}

// caseSLASubject is a case with its facts already loaded. terminal is decided
// by the caller: true only when a state override of CLOSED was passed in (a
// field-driven re-evaluation of an already-closed case must not fire the
// safety net).
type caseSLASubject struct {
	id       string
	facts    repository.CaseFacts
	terminal bool
}

func (c caseSLASubject) Kind() string               { return "case" }
func (c caseSLASubject) ID() string                 { return c.id }
func (c caseSLASubject) Facts() slacondition.Facts  { return caseConditionFacts{c: c.facts} }
func (c caseSLASubject) CreatedOn() time.Time       { return c.facts.CreatedOn }
func (c caseSLASubject) TerminalNow() bool          { return c.terminal }
func (c caseSLASubject) logs() *slaSubjectLogs      { return caseSLALogs }
func (c caseSLASubject) IsPolicy(cs ...string) bool { return isCasePolicy(cs...) }
func (c caseSLASubject) ConditionText(p repository.SLAPolicyRef, raw string) string {
	return slaPolicyConditionText(p, raw)
}

// incidentSLASubject is an incident with its facts already loaded. Its
// terminal state is read from the loaded facts (already normalised by
// loadIncidentFacts), not only from an override.
type incidentSLASubject struct {
	id    string
	facts repository.IncidentFacts
}

func (i incidentSLASubject) Kind() string               { return "incident" }
func (i incidentSLASubject) ID() string                 { return i.id }
func (i incidentSLASubject) Facts() slacondition.Facts  { return incidentConditionFacts{i: i.facts} }
func (i incidentSLASubject) CreatedOn() time.Time       { return i.facts.CreatedOn }
func (i incidentSLASubject) TerminalNow() bool          { return i.facts.State == "CLOSED" }
func (i incidentSLASubject) logs() *slaSubjectLogs      { return incidentSLALogs }
func (i incidentSLASubject) IsPolicy(cs ...string) bool { return isIncidentPolicy(cs...) }
func (i incidentSLASubject) ConditionText(p repository.SLAPolicyRef, raw string) string {
	return slaIncidentPolicyConditionText(p, raw)
}

// slaPolicyShouldCancelFor decides whether policy's open clock should be
// cancelled right now, per policy.WhenToCancel, evaluating condition text
// translated by text against f.
func slaPolicyShouldCancelFor(policy repository.SLAPolicyRef, f slacondition.Facts, text func(repository.SLAPolicyRef, string) string) bool {
	switch policy.WhenToCancel {
	case "NEVER":
		return false
	case "START_CONDITIONS_ARE_NOT_MET":
		// No StartCondition at all is never treated as "always cancel" -- an
		// empty condition evaluates false, which would otherwise cancel every
		// such clock on its very next evaluation.
		if policy.StartCondition == "" {
			return false
		}
		return !slacondition.Parse(text(policy, policy.StartCondition)).Evaluate(f)
	default: // "CANCEL_CONDITIONS_ARE_MET", or unset/unrecognized
		return policy.CancelCondition != "" && slacondition.Parse(text(policy, policy.CancelCondition)).Evaluate(f)
	}
}

// slaPolicyShouldResumeFor is slaPolicyShouldCancelFor's resume counterpart,
// per policy.WhenToResume. Evaluated whether or not the clock is actually
// paused: SetPausedByPolicy is an idempotent no-op on a running clock.
func slaPolicyShouldResumeFor(policy repository.SLAPolicyRef, f slacondition.Facts, text func(repository.SLAPolicyRef, string) string) bool {
	switch policy.WhenToResume {
	case "PAUSE_CONDITIONS_ARE_NOT_MET":
		if policy.PauseCondition == "" {
			return false
		}
		return !slacondition.Parse(text(policy, policy.PauseCondition)).Evaluate(f)
	default: // "RESUME_CONDITIONS_ARE_MET", or unset/unrecognized
		return policy.ResumeCondition != "" && slacondition.Parse(text(policy, policy.ResumeCondition)).Evaluate(f)
	}
}

// resolveApplicablePolicies resolves, for every clock target (slaTargetOrder:
// RESPONSE, WORKAROUND, RESOLUTION), every sla_policy whose start_condition
// currently matches the subject's facts -- shared by register, revise and
// reevaluate so all derive "what should this subject's clocks look like" the
// same way.
//
// The second return, lookupFailed, distinguishes a target that is missing
// because no policy matches from one missing because the candidate lookup
// failed (a database blip). Registration at creation safely ignores it:
// nothing existing is at risk and the target is retried the next time the
// subject is touched. Revise must check it: proceeding with an incomplete list
// would cancel existing clocks and commit no replacement for the target that
// failed to resolve.
//
// A policy whose stop_condition ALSO already holds against the same facts is
// never returned: upstream attaches a definition only when start holds AND
// stop does not. Without this, a subject already satisfying a policy's
// completion condition would get a fresh clock on every re-resolution that the
// very next state-effects pass completes at ~0% elapsed -- a junk ACHIEVED row.
func (s *slaEngineService) resolveApplicablePolicies(ctx context.Context, subj slaSubject) (policies []repository.SLAPolicyRef, lookupFailed bool) {
	f := subj.Facts()
	policies = make([]repository.SLAPolicyRef, 0, len(slaTargetOrder))
	for _, clockType := range slaTargetOrder {
		target := slaClockTypeTarget[clockType]
		matches, err := s.resolver.resolveSubject(ctx, target, subj)
		if err != nil {
			// resolve already logged why -- this target's policy couldn't be
			// determined right now, not that none applies.
			lookupFailed = true
			continue
		}
		// matches may hold more than one policy for one target (a Hosting
		// case's RESOLUTION target legitimately matches two); every match is
		// appended, RegisterClock's per-policy uniqueness guard makes that safe.
		for _, m := range matches {
			if m.StopCondition != "" && slacondition.Parse(subj.ConditionText(m, m.StopCondition)).Evaluate(f) {
				continue
			}
			policies = append(policies, m)
		}
	}
	return policies, lookupFailed
}

// registerClocks registers a clock for every policy applicable to subj.
// lookupFailed is deliberately ignored, see resolveApplicablePolicies.
func (s *slaEngineService) registerClocks(ctx context.Context, subj slaSubject) {
	lg := subj.logs()
	policies, _ := s.resolveApplicablePolicies(ctx, subj)
	if len(policies) == 0 {
		slog.InfoContext(ctx, lg.noPolicy, lg.idKey, subj.ID())
		return
	}
	for _, policy := range policies {
		startOn := registerCaseSLAClockStartOn(policy, subj.CreatedOn())
		registered, err := s.repo.RegisterClock(ctx, subj.ID(), policy, startOn)
		if err != nil {
			slog.ErrorContext(ctx, lg.registerFailed, lg.idKey, subj.ID(), "clockType", policy.Target, "policy", policy.Name, "err", err)
			continue
		}
		if !registered {
			slog.InfoContext(ctx, lg.alreadyRegistered, lg.idKey, subj.ID(), "clockType", policy.Target, "policy", policy.Name)
		}
	}
}

// reviseClocks replaces subj's open clocks with the ones its current facts
// call for (a severity/priority change).
func (s *slaEngineService) reviseClocks(ctx context.Context, subj slaSubject) {
	lg := subj.logs()
	policies, lookupFailed := s.resolveApplicablePolicies(ctx, subj)
	if lookupFailed {
		// Do NOT proceed with an incomplete policy list: leaving the existing
		// clocks untouched (stale, but intact) is safer than cancelling them
		// and committing no replacement for the target whose lookup failed.
		slog.ErrorContext(ctx, lg.reviseSkipped, lg.idKey, subj.ID())
		return
	}
	// Bounded retry (2 attempts, short pause): ReviseClocks is one atomic
	// transaction, so a second attempt never duplicates rows.
	var (
		cancelled int
		err       error
	)
	for attempt := 1; attempt <= 2; attempt++ {
		cancelled, err = s.repo.ReviseClocks(ctx, subj.ID(), policies)
		if err == nil || attempt == 2 {
			break
		}
		slog.WarnContext(ctx, lg.reviseRetry, lg.idKey, subj.ID(), "err", err)
		time.Sleep(200 * time.Millisecond)
	}
	if err != nil {
		// Atomic: a failure here means NOTHING changed.
		slog.ErrorContext(ctx, lg.reviseFailed, lg.idKey, subj.ID(), "err", err)
		return
	}
	if cancelled > 0 {
		slog.InfoContext(ctx, lg.reviseCancelled, lg.idKey, subj.ID(), "count", cancelled)
	}
}

// reevaluateClocks is the generic core of the Apply*Effects methods:
// re-evaluates every currently open clock against its own bound policy's
// stop/cancel/pause/resume conditions, then re-resolves and registers any
// newly applicable policy.
func (s *slaEngineService) reevaluateClocks(ctx context.Context, subj slaSubject) {
	lg := subj.logs()
	f := subj.Facts()

	for _, clockType := range slaTargetOrder {
		target := slaClockTypeTarget[clockType]
		policies, err := s.repo.ActiveClockPolicies(ctx, subj.ID(), target)
		if err != nil {
			slog.ErrorContext(ctx, lg.activeLookupFailed, lg.idKey, subj.ID(), "clockType", target, "err", err)
			continue
		}
		for _, policy := range policies {
			s.applyEffectsForPolicy(ctx, subj, target, policy)
		}
	}

	// A policy whose cancel condition is ALSO true for these facts is skipped:
	// otherwise it would be cancelled above and immediately re-registered,
	// resetting on every update and never reaching a real stop.
	policies, _ := s.resolveApplicablePolicies(ctx, subj)
	for _, policy := range policies {
		if slaPolicyShouldCancelFor(policy, f, subj.ConditionText) {
			continue
		}
		startOn := registerCaseSLAClockStartOn(policy, subj.CreatedOn())
		if _, err := s.repo.RegisterClock(ctx, subj.ID(), policy, startOn); err != nil {
			slog.ErrorContext(ctx, lg.reevalRegisterFailed, lg.idKey, subj.ID(), "clockType", policy.Target, "policy", policy.Name, "err", err)
		}
	}
}

// applyEffectsForPolicy evaluates one open clock's own bound policy
// conditions: stop -> complete, cancel -> cancel, pause -> pause, resume ->
// resume (first match wins), then the terminal safety net: a clock not already
// settled is completed when subj.TerminalNow(), whatever its stop_condition
// says. CompleteClock/CancelClock are idempotent against a clock that is not
// open, so `settled` only avoids a pointless second query.
func (s *slaEngineService) applyEffectsForPolicy(ctx context.Context, subj slaSubject, target string, policy repository.SLAPolicyRef) {
	lg := subj.logs()
	id := subj.ID()
	f := subj.Facts()
	text := subj.ConditionText
	settled := false
	switch {
	case policy.StopCondition != "" && slacondition.Parse(text(policy, policy.StopCondition)).Evaluate(f):
		if _, err := s.repo.CompleteClockByPolicy(ctx, id, target, policy.ID); err != nil {
			slog.ErrorContext(ctx, lg.effectsPrefix+": complete clock failed", lg.idKey, id, "clockType", target, "policy", policy.Name, "err", err)
		}
		settled = true
	case slaPolicyShouldCancelFor(policy, f, text):
		if _, err := s.repo.CancelClockByPolicy(ctx, id, target, policy.ID); err != nil {
			slog.ErrorContext(ctx, lg.effectsPrefix+": cancel clock failed", lg.idKey, id, "clockType", target, "policy", policy.Name, "err", err)
		}
		settled = true
	case policy.PauseCondition != "" && slacondition.Parse(text(policy, policy.PauseCondition)).Evaluate(f):
		if _, err := s.repo.SetPausedByPolicy(ctx, id, target, policy.ID, true); err != nil {
			slog.ErrorContext(ctx, lg.effectsPrefix+": pause clock failed", lg.idKey, id, "clockType", target, "policy", policy.Name, "err", err)
		}
	case slaPolicyShouldResumeFor(policy, f, text):
		if _, err := s.repo.SetPausedByPolicy(ctx, id, target, policy.ID, false); err != nil {
			slog.ErrorContext(ctx, lg.effectsPrefix+": resume clock failed", lg.idKey, id, "clockType", target, "policy", policy.Name, "err", err)
		}
	default:
		// Neither pause nor resume applies: the clock is left exactly as it is.
	}

	if !settled && subj.TerminalNow() {
		if _, err := s.repo.CompleteClockByPolicy(ctx, id, target, policy.ID); err != nil {
			slog.ErrorContext(ctx, lg.effectsPrefix+": safety-net complete clock on CLOSED failed", lg.idKey, id, "clockType", target, "policy", policy.Name, "err", err)
		}
	}
}

// compile-time checks
var (
	_ slaSubject = caseSLASubject{}
	_ slaSubject = incidentSLASubject{}
)
