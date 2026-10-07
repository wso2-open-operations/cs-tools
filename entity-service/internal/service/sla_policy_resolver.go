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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
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

// slaSeverityLabel maps domain.CaseSeverity to case_severity_enum's own
// 'S0'..'S4' labels (migration 0023) -- the same correspondence
// internal/repository/case_repo.go's own (unexported) caseSeverityToEnum
// uses, duplicated here since that map isn't exported across packages. Kept
// in sync with sla_duration_policy's own seed data (migration 0192), which
// is keyed on these same labels -- this is deliberately the one place this
// engine's policy lookup depends on severity at all.
var slaSeverityLabel = map[domain.CaseSeverity]string{
	domain.CaseSeverityCatastrophic: "S0",
	domain.CaseSeverityCritical:     "S1",
	domain.CaseSeverityHigh:         "S2",
	domain.CaseSeverityMedium:       "S3",
	domain.CaseSeverityLow:          "S4",
}

// slaApplicableClockTypes lists, per severity, which clock types this
// engine ever registers -- LOW/S4 gets "response" only, matching WSO2's
// published support policy's "best efforts" tier (no fixed Workaround/
// Resolution SLA), and mirroring sla_duration_policy's own seed data
// (migration 0192), which likewise has no low/workaround or low/resolution
// row at all.
var slaApplicableClockTypes = map[domain.CaseSeverity][]string{
	domain.CaseSeverityCatastrophic: {slaClockTypeResponse, slaClockTypeWorkaround, slaClockTypeResolution},
	domain.CaseSeverityCritical:     {slaClockTypeResponse, slaClockTypeWorkaround, slaClockTypeResolution},
	domain.CaseSeverityHigh:         {slaClockTypeResponse, slaClockTypeWorkaround, slaClockTypeResolution},
	domain.CaseSeverityMedium:       {slaClockTypeResponse, slaClockTypeWorkaround, slaClockTypeResolution},
	domain.CaseSeverityLow:          {slaClockTypeResponse},
}

// slaPolicyResolver resolves the sla_policy row (duration, id) backing a
// given severity/clock-type pair -- a single, deterministic lookup against
// the CSM-seeded rows migration 0203 adds (one per severity/clock-type pair
// sla_duration_policy already defines), keyed on severity directly.
//
// This replaces an earlier design that looked up the real, ServiceNow-synced
// sla_policy rows by a guessed exact name
// ("<P0-P3|Query> - <Response|Workaround|Resolution> (<Managed Services|Open
// Source>)"), with the "plan" half of that name itself guessed from a case's
// project subscription type -- confirmed, against real staging data, to
// silently find nothing for every LOW-severity case: the real synced rows
// for "Query" (LOW) are named things like "QuerySLA" and "Onboarding Case
// Customer Query Response", none of which match that assumed convention
// under either plan label or a loose pattern fallback. Rather than widen the
// guess further, this resolver stops guessing at all: it looks up a row this
// migration seeded itself, under a name it built the same way, so the
// lookup can never miss.
type slaPolicyResolver struct {
	repo repository.SLAEngineRepository
}

func newSLAPolicyResolver(repo repository.SLAEngineRepository) *slaPolicyResolver {
	return &slaPolicyResolver{repo: repo}
}

// resolve returns the sla_policy row for severity/clockType.
//
// ok=false with a nil error means no policy is seeded for this
// severity/clockType combination -- logged as a warning by the caller, the
// same "no fallback duration" posture the old design's hardcoded severity
// map had for an absent entry. A non-nil error means the lookup itself
// failed (e.g. a database blip), NOT that the policy is absent -- callers
// must treat these two cases differently: RegisterCaseClocks safely skips
// either one (nothing existing is at risk), but ReviseCaseClocks must NOT
// proceed to cancel a case's existing clocks on the strength of an
// incomplete policy list caused by a transient lookup failure (see
// resolveApplicablePolicies' own doc comment).
func (r *slaPolicyResolver) resolve(ctx context.Context, severity domain.CaseSeverity, clockType string) (repository.SLAPolicyRef, bool, error) {
	label, ok := slaSeverityLabel[severity]
	if !ok {
		slog.WarnContext(ctx, "sla engine: no severity label for policy lookup", "severity", severity)
		return repository.SLAPolicyRef{}, false, nil
	}
	target, ok := slaClockTypeTarget[clockType]
	if !ok {
		slog.WarnContext(ctx, "sla engine: unknown clock type", "clockType", clockType)
		return repository.SLAPolicyRef{}, false, nil
	}

	name := label + " - " + target + " (CSM)"
	ref, err := r.repo.FindPolicyByName(ctx, name, target)
	if err == nil {
		return ref, true, nil
	}
	var notFound *apierror.NotFoundError
	if errors.As(err, &notFound) {
		slog.WarnContext(ctx, "sla engine: no sla_policy found for severity/clockType -- migration 0203 may not be applied",
			"severity", severity, "clockType", clockType, "name", name)
		return repository.SLAPolicyRef{}, false, nil
	}
	slog.ErrorContext(ctx, "sla engine: policy lookup failed", "name", name, "err", err)
	return repository.SLAPolicyRef{}, false, err
}
