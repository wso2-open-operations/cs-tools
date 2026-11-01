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
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/slacondition"
)

// SLA clock type names -- kept as the same three string values the
// now-deleted sla_clocks design used (internal/service/sla_policy.go
// before commit 116d43522), and matching sla_policy_target_enum's real
// labels (RESPONSE/WORKAROUND/RESOLUTION) once upper-cased.
const (
	slaClockTypeResponse   = "response"
	slaClockTypeWorkaround = "workaround"
	slaClockTypeResolution = "resolution"
)

// slaClockTypeTarget maps this package's own clock-type strings to
// sla_policy.target's real enum labels.
var slaClockTypeTarget = map[string]string{
	slaClockTypeResponse:   "RESPONSE",
	slaClockTypeWorkaround: "WORKAROUND",
	slaClockTypeResolution: "RESOLUTION",
}

// slaTargetClockType is slaClockTypeTarget's inverse -- iterated by
// sla_engine_service.go to walk every clock type a case might have, in a
// fixed, deterministic order (RESPONSE, then WORKAROUND, then RESOLUTION).
var slaTargetOrder = []string{slaClockTypeResponse, slaClockTypeWorkaround, slaClockTypeResolution}

// slaSeverityLabel maps domain.CaseSeverity to case_severity_enum's own
// 'S0'..'S4' labels (migration 0023) -- the same correspondence
// internal/repository/case_repo.go's own (unexported) caseSeverityToEnum
// uses, duplicated here since that map isn't exported across packages.
// Postgres itself already stores (and sla_policy's own decoded conditions
// already spell) these same 'S0'..'S4' labels verbatim (see
// repository.CaseFacts' own doc comment), so this map is only needed at
// this one seam: translating a caller's in-flight domain.CaseSeverity value
// (RegisterCaseClocks/ReviseCaseClocks are hamded one directly, since a
// fresh database read can't always be trusted to already reflect it -- see
// those methods' own doc comments) into the same vocabulary.
var slaSeverityLabel = map[domain.CaseSeverity]string{
	domain.CaseSeverityCatastrophic: "S0",
	domain.CaseSeverityCritical:     "S1",
	domain.CaseSeverityHigh:         "S2",
	domain.CaseSeverityMedium:       "S3",
	domain.CaseSeverityLow:          "S4",
}

// slaStateLabel maps domain.CaseState to case_state_enum's own labels, which
// sla_policy's own decoded conditions already spell verbatim (migration
// 0250) -- e.g. domain.CaseStateWorkInProgress ("work_in_progress") becomes
// "WORK_IN_PROGRESS". A plain strings.ToUpper would do the same thing here
// since every domain.CaseState value already is its enum label lower-cased,
// but spelling it out as a real map (rather than relying on that
// convention silently holding forever) is one less thing a future new
// state value could get wrong unnoticed.
var slaStateLabel = map[domain.CaseState]string{
	domain.CaseStateOpen:             "OPEN",
	domain.CaseStateWorkInProgress:   "WORK_IN_PROGRESS",
	domain.CaseStateWaitingOnWSO2:    "WAITING_ON_WSO2",
	domain.CaseStateAwaitingInfo:     "AWAITING_INFO",
	domain.CaseStateReopened:         "REOPENED",
	domain.CaseStateSolutionProposed: "SOLUTION_PROPOSED",
	domain.CaseStateClosed:           "CLOSED",
}

// slaPolicySkipFieldMarkers names condition fields that only ever appear on
// a non-case (incident/account-escalation) sla_policy row -- confirmed
// against every row in the real table, not assumed; see migration 0250's
// own doc comment for the same list. A row carrying any of these is never a
// case policy, full stop, regardless of anything else in its conditions.
var slaPolicySkipFieldMarkers = []string{
	"business_service",
	"category=",
	"category!=",
	"escalation_severity",
	"source_table",
	"active_escalation",
}

// slaPolicyCaseFieldMarkers names condition fields that exist ONLY in this
// engine's own CaseFacts vocabulary (see repository.CaseFacts' own doc
// comment) -- never in an incident/account-escalation/onboarding-task
// policy's conditions. A row that carries one of these, and none of
// slaPolicySkipFieldMarkers, is a case policy even without an explicit
// case_type token (the "[Cloud] P<n>" product-gated policies are exactly
// this shape: product/state/priority/account, no case_type at all -- product
// and account alone are enough).
//
// Deliberately NOT "priority" or "state": both were here once, and both are
// generic field names that real non-case rows also use with no skip marker
// present -- an onboarding-task row ("stage=1^state=4") and a bare-priority
// "IR" incident row both lack every slaPolicySkipFieldMarkers token, so
// either one used to be misclassified as a case policy purely by containing
// the substring "priority" or "state", and could then spuriously match via
// slacondition's not-equal-on-absent-field semantics and win resolution for
// a real case. isCasePolicy is a strict inclusion check now: a policy is a
// case policy only when its conditions name a field this list knows is
// case-exclusive, never from "priority"/"state" alone.
var slaPolicyCaseFieldMarkers = []string{
	"case_type",
	"account",
	"product",
	"project_type",
	"cs_engineer_response_comment",
	"u_workaround_provided",
	"fix_eta_shared",
	"opened_at",
	"sys_created_on",
}

// isCasePolicy decides whether an sla_policy row's conditions describe a
// case (vs. an incident/account-escalation/onboarding-task policy this
// engine never builds evaluation support for): it must carry at least one
// slaPolicyCaseFieldMarkers token and none of slaPolicySkipFieldMarkers'. A
// row with neither a case-only field nor a skip marker is ambiguous and
// skipped -- never guessed into "probably a case policy" just because it
// happens to mention "priority" or "state", which plenty of non-case rows
// also do.
func isCasePolicy(conditions ...string) bool {
	all := strings.Join(conditions, "^")
	for _, marker := range slaPolicySkipFieldMarkers {
		if strings.Contains(all, marker) {
			return false
		}
	}
	for _, marker := range slaPolicyCaseFieldMarkers {
		if strings.Contains(all, marker) {
			return true
		}
	}
	return false
}

// caseConditionFacts adapts a repository.CaseFacts into the
// slacondition.Facts interface -- the one bridge between "what a case's
// fields currently are" and "what sla_policy's own decoded condition
// vocabulary calls those fields" (migrations 0250-0258).
type caseConditionFacts struct {
	c repository.CaseFacts
}

func (f caseConditionFacts) Field(name string) (string, bool) {
	switch name {
	case "state":
		return f.c.State, f.c.State != ""
	case "priority":
		return f.c.Severity, f.c.Severity != ""
	case "case_type":
		return f.c.CaseType, f.c.CaseType != ""
	case "project_type":
		return f.c.ProjectType, f.c.ProjectType != ""
	case "product":
		return f.c.Product, f.c.Product != ""
	case "cs_engineer_response_comment":
		return strconv.FormatBool(f.c.CSEngineerResponseComment), true
	default:
		return "", false
	}
}

func (f caseConditionFacts) Present(name string) bool {
	switch name {
	case "account":
		return f.c.HasAccount
	case "u_workaround_provided":
		return f.c.WorkaroundProvided
	case "fix_eta_shared":
		return f.c.FixEtaShared
	case "case_type":
		return f.c.CaseType != ""
	default:
		v, ok := f.Field(name)
		return ok && v != ""
	}
}

func (f caseConditionFacts) Date(name string) (time.Time, bool) {
	switch name {
	case "opened_at", "sys_created_on":
		return f.c.CreatedOn, !f.c.CreatedOn.IsZero()
	default:
		return time.Time{}, false
	}
}

// slaPolicyResolver resolves the one real, source='SERVICENOW' sla_policy
// row that currently applies to a case for a given clock target (RESPONSE/
// WORKAROUND/RESOLUTION), by evaluating every active case-policy candidate's
// start_condition (internal/slacondition) against the case's current
// CaseFacts and picking the one that matches.
//
// This replaces an earlier design that looked up a deterministic,
// CSM-seeded row by a constructed name ("S0 - RESPONSE (CSM)",
// severity-keyed only) -- that whole mechanism is gone along with the 16
// source='CSM' sla_policy rows it depended on (migration 0259). The real,
// previous-system-synced policies this resolver now evaluates are keyed on much
// more than severity: case type, product, project type, account presence,
// creation date and whether a CS engineer has already replied all
// genuinely gate which policy applies, and several real policies have no
// severity condition at all (ServiceRequest SLA, the Query/SecReport
// Analysis family). A purely severity-keyed lookup could never have
// resolved those correctly.
type slaPolicyResolver struct {
	repo repository.SLAEngineRepository
}

func newSLAPolicyResolver(repo repository.SLAEngineRepository) *slaPolicyResolver {
	return &slaPolicyResolver{repo: repo}
}

// slaPolicyConditionText resolves candidate's condition column text into the
// neutral form internal/slacondition parses: translated through
// translateSLACondition when the policy's own Source is the external sync
// job's ("SERVICENOW", every row that job writes today), or returned
// unchanged for any other source -- an admin-authored CSM-native policy
// (see SLAPolicyRef.Source's own doc comment) is assumed to already be
// written in this engine's own neutral vocabulary and must never be run
// through a vendor-specific translator.
func slaPolicyConditionText(candidate repository.SLAPolicyRef, raw string) string {
	if candidate.Source == "SERVICENOW" {
		return translateSLACondition(raw)
	}
	return raw
}

// resolve returns every sla_policy row whose start_condition currently
// matches facts, for the given sla_policy_target_enum label (RESPONSE/
// WORKAROUND/RESOLUTION) -- not just one. More than one candidate matching
// the same facts is a real possibility in the live data: a Hosting case's
// RESOLUTION target legitimately matches both an open-source-track policy
// (e.g. "P1 - Resolution (Open Source)", case_type IN (case, hosting)) and a
// managed-services-track one (e.g. "P1 - Resolution (Managed Services)",
// case_type = hosting) at the same time, and the real system runs both
// clocks side by side on such a case. An earlier version of this method
// returned only one match (the lowest sla_policy.id), silently dropping
// whichever policy didn't win -- callers now register/evaluate every policy
// this returns, each as its own independent clock (see
// SLAEngineService.RegisterCaseClocks/ApplyCaseStateEffects).
//
// An empty, non-nil result means no case policy's start_condition matches --
// logged at INFO, the same "no fallback duration" posture the old design's
// absent map entry had. A non-nil error means the candidate list itself
// couldn't be read (a database blip), not that no policy applies -- see
// resolveApplicablePolicies' own doc comment in sla_engine_service.go for
// why callers must tell the two apart.
func (r *slaPolicyResolver) resolve(ctx context.Context, target string, facts repository.CaseFacts) ([]repository.SLAPolicyRef, error) {
	candidates, err := r.repo.ListCandidatePolicies(ctx, target)
	if err != nil {
		slog.ErrorContext(ctx, "sla policy resolver: list candidate policies failed", "target", target, "err", err)
		return nil, err
	}

	f := caseConditionFacts{c: facts}
	var matches []repository.SLAPolicyRef
	var skippedNonCase []string
	for _, candidate := range candidates {
		if !isCasePolicy(candidate.StartCondition, candidate.StopCondition, candidate.PauseCondition, candidate.ResumeCondition, candidate.CancelCondition) {
			skippedNonCase = append(skippedNonCase, candidate.Name)
			continue
		}
		if candidate.Schedule != "" && candidate.Schedule != "24 x 7" && candidate.Schedule != slaScheduleWSO2BusinessDaysLabel {
			slog.WarnContext(ctx, "sla policy resolver: unsupported schedule, evaluating with flat wall-clock time anyway",
				"policy", candidate.Name, "schedule", candidate.Schedule)
		}
		startCondition := slaPolicyConditionText(candidate, candidate.StartCondition)
		parsed := slacondition.Parse(startCondition)
		// A clause that fails closed with no visible trace is exactly how a
		// translation gap (an un-mapped raw value, a field the translator
		// doesn't yet know) goes unnoticed -- see slacondition's own doc
		// comment on UnknownClauses.
		if unknown := parsed.UnknownClauses(); len(unknown) > 0 {
			slog.WarnContext(ctx, "sla policy resolver: start condition has unknown/untranslated clauses, they evaluate false",
				"policy", candidate.Name, "target", target, "unknownClauses", unknown)
		}
		if parsed.Evaluate(f) {
			matches = append(matches, candidate)
		}
	}
	if len(skippedNonCase) > 0 {
		slog.DebugContext(ctx, "sla policy resolver: skipped non-case policy candidates", "target", target, "skipped", skippedNonCase)
	}

	switch len(matches) {
	case 0:
		slog.InfoContext(ctx, "sla policy resolver: no case policy matched", "target", target)
	case 1:
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.Name
		}
		slog.InfoContext(ctx, "sla policy resolver: more than one policy matched the same case, registering/evaluating each independently",
			"target", target, "matches", names)
	}
	return matches, nil
}

// slaScheduleWSO2BusinessDaysLabel mirrors
// repository.slaScheduleWSO2BusinessDays (unexported in that package) --
// kept as a separate literal here rather than exporting that constant
// purely for one string comparison in a warning log.
const slaScheduleWSO2BusinessDaysLabel = "WSO2 Business days"
