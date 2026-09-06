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

package dto

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestCaseStatusRef_EmitsDisplayLabel guards the regression where the case header
// showed "work_in_progress" instead of "Work In Progress".
//
// This is not merely cosmetic: the frontend gates the whole Calls tab on
// CALL_SCHEDULABLE_CASE_STATUSES.includes(statusLabel), and that list holds the
// display values. A raw enum misses the match, isCallSchedulingAllowed goes
// false, and hideCallsTab hides the tab entirely — so the label *is* the feature
// flag. See CaseDetailsContent.tsx.
func TestCaseStatusRef_EmitsDisplayLabel(t *testing.T) {
	for in, want := range map[string]string{
		"work_in_progress":  "Work In Progress",
		"closed":            "Closed",
		"open":              "Open",
		"awaiting_info":     "Awaiting Info",
		"waiting_on_wso2":   "Waiting On WSO2",
		"reopened":          "Reopened",
		"solution_proposed": "Solution Proposed",
	} {
		got := caseStatusRef(in)
		if got == nil {
			t.Errorf("caseStatusRef(%q) = nil, want a ref", in)
			continue
		}
		if got.Label != want {
			t.Errorf("caseStatusRef(%q).Label = %q, want %q", in, got.Label, want)
		}
		if got.ID == "" {
			t.Errorf("caseStatusRef(%q).ID is empty; the numeric id must still resolve", in)
		}
	}
}

// TestCaseStatusRef_CallSchedulableLabelsMatchFrontend pins the four statuses the
// frontend treats as call-schedulable. If one of these labels drifts, the Calls
// tab silently vanishes for cases in that state rather than failing loudly.
func TestCaseStatusRef_CallSchedulableLabelsMatchFrontend(t *testing.T) {
	// Verbatim from CALL_SCHEDULABLE_CASE_STATUSES in
	// features/support/constants/supportConstants.ts.
	schedulable := map[string]string{
		"work_in_progress":  "Work In Progress",
		"awaiting_info":     "Awaiting Info",
		"waiting_on_wso2":   "Waiting On WSO2",
		"solution_proposed": "Solution Proposed",
		"reopened":          "Reopened",
	}
	for enum, want := range schedulable {
		if got := displayLabelOr(caseStateDisplayLabels, enum); got != want {
			t.Errorf("status %q maps to %q, want %q — the Calls tab is gated on this exact string", enum, got, want)
		}
	}
}

// TestCaseSeverityRef_EmitsLabelTheFrontendMaps checks the severity label is the
// SN-style string SEVERITY_LABEL_TO_DISPLAY is keyed on — "Low (P4)" becomes
// "S4(Query)" there. The bare enum missed that lookup, so the header read "low".
func TestCaseSeverityRef_EmitsLabelTheFrontendMaps(t *testing.T) {
	for in, want := range map[string]string{
		"low":          "Low (P4)",
		"medium":       "Medium (P3)",
		"high":         "High (P2)",
		"critical":     "Critical (P1)",
		"catastrophic": "Catastrophic (P0)",
	} {
		v := in
		got := caseSeverityRef(&v)
		if got == nil {
			t.Errorf("caseSeverityRef(%q) = nil, want a ref", in)
			continue
		}
		if got.Label != want {
			t.Errorf("caseSeverityRef(%q).Label = %q, want %q", in, got.Label, want)
		}
		if got.ID == "" {
			t.Errorf("caseSeverityRef(%q).ID is empty; the numeric id must still resolve", in)
		}
	}
}

// TestDisplayLabels_UnknownValuePassesThrough keeps an unrecognised value
// rendering as-is rather than blanking the field — the same tolerance the *Ref
// helpers already apply to labels they cannot classify.
func TestDisplayLabels_UnknownValuePassesThrough(t *testing.T) {
	if got := displayLabelOr(caseStateDisplayLabels, "some_future_state"); got != "some_future_state" {
		t.Errorf("got %q, want the value passed through unchanged", got)
	}
	if got := displayLabelOr(caseSeverityDisplayLabels, ""); got != "" {
		t.Errorf("got %q, want empty preserved", got)
	}
}

// TestCaseAttachmentsResponse_EmitsTotalRecords is the key-name assertion that
// would have caught this class of bug at build time.
//
// useGetCaseAttachments destructures `totalRecords` with **no** array-length
// fallback (unlike the calls and escalations panels), so emitting `total` left
// attachmentCount undefined and the tab read "Attachments (0)" even though
// entity-service returned 200 with a populated list.
func TestCaseAttachmentsResponse_EmitsTotalRecords(t *testing.T) {
	b, err := json.Marshal(CaseAttachmentsResponse{TotalRecords: 2, Limit: 10})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, `"totalRecords":2`) {
		t.Errorf("payload %s missing \"totalRecords\":2 — the frontend reads this key with no fallback", got)
	}
	if strings.Contains(got, `"total":`) {
		t.Errorf("payload %s still emits the legacy \"total\" key", got)
	}
}
