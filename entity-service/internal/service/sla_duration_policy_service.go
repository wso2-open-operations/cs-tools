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
	"sort"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/slacondition"
)

// slaDurationPolicySeverities lists the five case_severity_enum labels
// sla_policy's own decoded conditions use ("S0".."S4", migration 0250), each
// paired with the uppercase English word GET /sla-duration-policy's response
// has always used (matching domain.CaseSeverity's own values upper-cased,
// the same word a case.* event's own Priority field carries).
var slaDurationPolicySeverities = []struct {
	Label string
	Name  string
}{
	{"S0", "CATASTROPHIC"},
	{"S1", "CRITICAL"},
	{"S2", "HIGH"},
	{"S3", "MEDIUM"},
	{"S4", "LOW"},
}

type slaDurationPolicyService struct {
	repo   repository.SLAEngineRepository
	access AccessService
}

// NewSLADurationPolicyService constructs an SLADurationPolicyService backed
// by the given SLA engine repository -- the same one slaPolicyResolver reads
// (repository.SLAEngineRepository.ListCandidatePolicies), now that the old,
// static sla_duration_policy table (migration 0192, dropped by migration
// 0261) is redundant: sla_policy already holds the real, decoded per-policy
// durations. This endpoint is still a real, live consumer -- restored after
// an earlier pass mistakenly deleted it: csm-notification-service's
// internal/paging rung-escalation email displays a "response SLA remaining
// time" computed from exactly this data. access gates every call to
// internal callers -- same reasoning as NewSLAStatusService's own doc
// comment.
func NewSLADurationPolicyService(repo repository.SLAEngineRepository, access AccessService) SLADurationPolicyService {
	return &slaDurationPolicyService{repo: repo, access: access}
}

// ListSLADurationPolicy implements SLADurationPolicyService.
//
// Computed from the real sla_policy table instead of the old static one:
// for each (severity, clockType) pair, resolveDurationPolicy picks the one
// policy that would apply to a brand new, plain case_type=case case at that
// severity -- the exact same resolution slaPolicyResolver.resolve performs
// for a real case, just against a synthetic fact set instead of a real
// case's own. A severity/clockType with no matching policy at all is simply
// omitted (no fallback value), same "no fallback duration" posture the live
// engine itself has for an unmatched case.
func (s *slaDurationPolicyService) ListSLADurationPolicy(ctx context.Context) (domain.SLADurationPolicyResponse, error) {
	if err := RequireInternalCaller(ctx, s.access, "sla duration policy is only available to internal services"); err != nil {
		return domain.SLADurationPolicyResponse{}, err
	}

	var policies []domain.SLADurationPolicyItem
	for _, clockType := range slaTargetOrder {
		target := slaClockTypeTarget[clockType]
		for _, sev := range slaDurationPolicySeverities {
			policy, ok, err := s.resolveDurationPolicy(ctx, target, sev.Label)
			if err != nil {
				return domain.SLADurationPolicyResponse{}, err
			}
			if !ok {
				continue
			}
			policies = append(policies, domain.SLADurationPolicyItem{
				Severity:        sev.Name,
				ClockType:       clockType,
				DurationSeconds: int64(policy.Duration.Seconds()),
			})
		}
	}
	return domain.SLADurationPolicyResponse{Policies: policies}, nil
}

// resolveDurationPolicy picks the one canonical sla_policy duration for
// (target, severityLabel). Evaluates every active, source='SERVICENOW' case
// policy's start_condition (same isCasePolicy classification
// sla_policy_resolver.go uses) against a synthetic CaseFacts describing a
// plain, brand-new case: case_type=case, state=OPEN, this severity, no
// response yet, no account/project type/product/workaround/fix-ETA, created
// far enough in the past to satisfy any opened_at/sys_created_on condition.
//
// This synthetic fact set is what excludes every hosting/hosting_query/
// hosting_task-gated policy (the user confirmed none of those need an SLA
// here) and every product/project-type-gated policy with no effort spent on
// a separate exclusion list: their own start_condition requires a
// case_type/project_type/product this fact set never has, so they simply
// never match.
//
// Tie-break when more than one policy matches (a real possibility: e.g.
// "P1 - Resolution (Open Source)", gated on priority=S1, and
// "Query - Resolution (Open Source)", which has no priority condition at
// all and so matches every severity, can both match S1/RESOLUTION) --
// documented rule: the policy whose start_condition names this severity
// explicitly ("priority=S<n>") wins over one that applies to every severity
// generically -- more specific beats more general, which is also, in every
// case checked against the real table, the policy whose name/condition
// reads as the severity-specific "canonical" one. Still ambiguous after
// that (none currently are): lowest sla_policy.id wins deterministically,
// logged -- the same fallback slaPolicyResolver.resolve itself uses.
func (s *slaDurationPolicyService) resolveDurationPolicy(ctx context.Context, target, severityLabel string) (repository.SLAPolicyRef, bool, error) {
	candidates, err := s.repo.ListCandidatePolicies(ctx, target)
	if err != nil {
		return repository.SLAPolicyRef{}, false, err
	}

	facts := caseConditionFacts{c: repository.CaseFacts{
		State:    "OPEN",
		Severity: severityLabel,
		CaseType: "case",
		// ProjectType/CreatedOn are set to plausible, common values -- not
		// left blank -- so a real policy gated on either (e.g.
		// "P1 - Workaround (Open Source)"'s own project_type=Subscription
		// check, or "P1 - Resolution (Open Source)"'s own
		// sys_created_on>=2022-08-26 check) can still resolve. "Subscription"
		// is WSO2's own most common project type; the creation date is NOW,
		// not some arbitrary past date -- every real opened_at/sys_created_on
		// condition in the table today is a ">=" (at-or-after) check, so the
		// synthetic case has to be newer than the threshold, not older.
		ProjectType: "Subscription",
		CreatedOn:   time.Now().UTC(),
	}}

	var matches []repository.SLAPolicyRef
	for _, candidate := range candidates {
		if !isCasePolicy(candidate.StartCondition, candidate.StopCondition, candidate.PauseCondition, candidate.ResumeCondition, candidate.CancelCondition) {
			continue
		}
		// Route through the same slaPolicyConditionText the engine's own
		// resolver uses -- candidate.StartCondition is still raw,
		// vendor-encoded text (the external sync job never pre-decodes it),
		// so parsing it directly here used to never match on real data at
		// all (a confirmed, reported bug: GET /sla-duration-policy returned
		// nothing against the real table). One matching path, not two.
		translatedStart := slaPolicyConditionText(candidate, candidate.StartCondition)
		if slacondition.Parse(translatedStart).Evaluate(facts) {
			matches = append(matches, candidate)
		}
	}
	switch len(matches) {
	case 0:
		return repository.SLAPolicyRef{}, false, nil
	case 1:
		return matches[0], true, nil
	}

	// The specificity needle matches against the TRANSLATED vocabulary
	// ("priority=S1", slaCaseSeverityCodes' own decoded label) -- the raw
	// text never contains this spelling (it stores "priority=10"), so this
	// needs the same translation every match above already got.
	needle := "priority=" + severityLabel
	var specific []repository.SLAPolicyRef
	for _, m := range matches {
		if strings.Contains(slaPolicyConditionText(m, m.StartCondition), needle) {
			specific = append(specific, m)
		}
	}
	if len(specific) > 0 {
		matches = specific
	}
	if len(matches) == 1 {
		return matches[0], true, nil
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
	names := make([]string, len(matches))
	for i, m := range matches {
		names[i] = m.Name
	}
	slog.WarnContext(ctx, "sla duration policy: multiple policies matched, picking lowest id deterministically",
		"target", target, "severity", severityLabel, "matches", names)
	return matches[0], true, nil
}
