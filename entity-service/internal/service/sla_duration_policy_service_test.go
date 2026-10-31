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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// TestSLADurationPolicy_MatchesRawUntranslatedConditions is the regression
// test for a High-severity PR review finding: resolveDurationPolicy used to
// call slacondition.Parse directly on candidate.StartCondition, which is
// still raw, vendor-encoded text (sys_ids, "u_" field names, numeric
// state/priority codes) -- the external sync job never pre-decodes it. That
// made GET /sla-duration-policy return nothing against real data. This test
// seeds a policy shaped exactly like a real row (raw u_case_type sys_id,
// numeric priority code) and confirms it resolves once routed through
// slaPolicyConditionText, the same translation the engine's own resolver
// already applies.
func TestSLADurationPolicy_MatchesRawUntranslatedConditions(t *testing.T) {
	const incidentSysID = "8d4b87bd1b18f010cb6898aebd4bcb59" // Incident -- see sn_sla_condition_translate.go
	repo := &fakeCandidatePolicyRepo{byTarget: map[string][]repository.SLAPolicyRef{
		"RESPONSE": {
			{
				ID:              "p2-response",
				Name:            "P2 - Response (Open Source)",
				Target:          "RESPONSE",
				Duration:        4 * time.Hour,
				Source:          "SERVICENOW",
				StartCondition:  "u_case_type=" + incidentSysID + "^priority=11^state=1^u_wso2_response_comment=false^EQ", // priority=11 -> S2
				CancelCondition: "u_case_type!=" + incidentSysID + "^ORpriority!=11^EQ",
			},
		},
	}}
	svc := NewSLADurationPolicyService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})

	resp, err := svc.ListSLADurationPolicy(context.Background())
	if err != nil {
		t.Fatalf("ListSLADurationPolicy: %v", err)
	}

	var found bool
	for _, p := range resp.Policies {
		if p.Severity == "HIGH" && p.ClockType == "response" { // S2 -> "HIGH", slaDurationPolicySeverities
			found = true
			if p.DurationSeconds != int64((4 * time.Hour).Seconds()) {
				t.Errorf("DurationSeconds = %d, want %d", p.DurationSeconds, int64((4*time.Hour).Seconds()))
			}
		}
	}
	if !found {
		t.Fatalf("expected a HIGH/response entry (the P2 policy, raw-text matched via translation), got %+v", resp.Policies)
	}
}

// TestSLADurationPolicy_SpecificityNeedleMatchesTranslatedText confirms the
// "more specific beats more general" tie-break (resolveDurationPolicy's own
// needle, "priority="+severityLabel) matches against the TRANSLATED text
// ("priority=S2"), not the raw stored form ("priority=11") -- the second
// half of the same High-severity finding.
func TestSLADurationPolicy_SpecificityNeedleMatchesTranslatedText(t *testing.T) {
	const incidentSysID = "8d4b87bd1b18f010cb6898aebd4bcb59"
	repo := &fakeCandidatePolicyRepo{byTarget: map[string][]repository.SLAPolicyRef{
		"RESOLUTION": {
			{
				ID:             "p2-resolution-specific",
				Name:           "P2 - Resolution (Open Source)",
				Target:         "RESOLUTION",
				Duration:       8 * time.Hour,
				Source:         "SERVICENOW",
				StartCondition: "u_case_type=" + incidentSysID + "^priority=11^EQ", // explicit S2
			},
			{
				ID:             "incident-resolution-generic",
				Name:           "Incident - Resolution (any severity)",
				Target:         "RESOLUTION",
				Duration:       240 * time.Hour,
				Source:         "SERVICENOW",
				// No priority condition at all -- translates to just
				// "priority!=S4" (Incident with no severity restriction),
				// which also matches S2, same as the real "Query -
				// Resolution"/generic-fallback shape this tie-break exists
				// for.
				StartCondition: "u_case_type=" + incidentSysID + "^EQ",
			},
		},
	}}
	svc := NewSLADurationPolicyService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})

	resp, err := svc.ListSLADurationPolicy(context.Background())
	if err != nil {
		t.Fatalf("ListSLADurationPolicy: %v", err)
	}

	for _, p := range resp.Policies {
		if p.Severity == "HIGH" && p.ClockType == "resolution" {
			if p.DurationSeconds != int64((8 * time.Hour).Seconds()) {
				t.Errorf("HIGH/resolution duration = %d, want the specific P2 policy's 8h (%d) -- specificity needle must match the translated text",
					p.DurationSeconds, int64((8*time.Hour).Seconds()))
			}
			return
		}
	}
	t.Fatalf("expected a HIGH/resolution entry, got %+v", resp.Policies)
}
