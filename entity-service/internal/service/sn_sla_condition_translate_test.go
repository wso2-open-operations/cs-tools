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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/slacondition"
)

func TestTranslateSLACondition(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "strips the trailing EQ terminator",
			raw:  "state=1^priority=10^EQ",
			want: "state=OPEN^priority=S1",
		},
		{
			name: "translates Incident case_type sys_id into a severity membership predicate and strips u_ prefix",
			raw:  "u_case_type=8d4b87bd1b18f010cb6898aebd4bcb59^state=1^EQ",
			want: "priorityINS0,S1,S2,S3^state=OPEN",
		},
		{
			name: "translates Query case_type sys_id into a severity predicate",
			raw:  "u_case_type=0d5b8fbd1b18f010cb6898aebd4bcba5^state=1^EQ",
			want: "priority=S4^state=OPEN",
		},
		{
			name: "translates negated Incident/Query case_type clauses with the comparison flipped",
			raw:  "u_case_type!=8d4b87bd1b18f010cb6898aebd4bcb59^ORu_case_type!=0d5b8fbd1b18f010cb6898aebd4bcba5^EQ",
			want: "priorityNOT INS0,S1,S2,S3^ORpriority!=S4",
		},
		{
			name: "translates product sys_ids across an OR run",
			raw:  "product=691b4f861b746010cb6898aebd4bcbc0^ORproduct=720bf91d1bf92010cb6898aebd4bcb10^priority=10^EQ",
			want: "product=WSO2 Developer Platform^ORproduct=Asgardeo^priority=S1",
		},
		{
			name: "translates project.u_project_type dot-walk field and sys_id",
			raw:  "project.u_project_type=cb4a87bd1b18f010cb6898aebd4bcb4f^ORproject.u_project_type=87a8676c1b347410cb6898aebd4bcb76^EQ",
			want: "project_type=Subscription^ORproject_type=Managed Cloud Subscription",
		},
		{
			name: "renames u_wso2_response_comment",
			raw:  "state=1^u_wso2_response_comment=false^EQ",
			want: "state=OPEN^cs_engineer_response_comment=false",
		},
		{
			name: "renames u_fix_eta_shared and keeps ISNOTEMPTY suffix",
			raw:  "state=3^ORu_fix_eta_sharedISNOTEMPTY^EQ",
			want: "state=CLOSED^ORfix_eta_sharedISNOTEMPTY",
		},
		{
			name: "leaves the Admin Task case_type sys_id unmapped (no longer created, confirmed with the team) and translates the Incident clause alongside it",
			raw:  "case_type!=3f5b47bd1b18f010cb6898aebd4bcbc2^case_type!=8d4b87bd1b18f010cb6898aebd4bcb59^EQ",
			want: "case_type!=3f5b47bd1b18f010cb6898aebd4bcbc2^priorityNOT INS0,S1,S2,S3",
		},
		{
			name: "falls through Hosting/Hosting Query/Hosting Task sys_ids unchanged -- permanently inert, by product decision",
			raw:  "case_type=bfa1473c1bbcb410cb6898aebd4bcb52^ORcase_type=80810ff81bbcb410cb6898aebd4bcb3c^ORcase_type=f46103f81bbcb410cb6898aebd4bcb27^EQ",
			want: "case_type=bfa1473c1bbcb410cb6898aebd4bcb52^ORcase_type=80810ff81bbcb410cb6898aebd4bcb3c^ORcase_type=f46103f81bbcb410cb6898aebd4bcb27",
		},
		{
			name: "keeps a malformed, value-less trailing clause as a false disjunct rather than dropping it",
			raw:  "priority!=S4^ORstate=",
			want: "priority!=S4^ORstate=",
		},
		{
			name: "strips the javascript date-literal wrapper",
			raw:  "sys_created_on>=javascript:gs.dateGenerate('2022-08-26','00:00:00')^EQ",
			want: "sys_created_on>=2022-08-26",
		},
		{
			name: "translates IN lists member by member",
			raw:  "stateINWORK_IN_PROGRESS,AWAITING_INFO^EQ",
			want: "stateINWORK_IN_PROGRESS,AWAITING_INFO",
		},
		{
			name: "translates numeric IN list members",
			raw:  "stateIN10,18,1009^EQ",
			want: "stateINWORK_IN_PROGRESS,AWAITING_INFO,1009",
		},
		{
			name: "preserves NQ disjunction boundaries",
			raw:  "state=1^priority=10^NQstate=1^u_case_type=ab36479047ccf510a0a29cd3846d43ee^EQ",
			want: "state=OPEN^priority=S1^NQstate=OPEN^case_type=security_report_analysis",
		},
		{
			name: "is idempotent on already-neutral text",
			raw:  "state=OPEN^case_type=case^priority=S1",
			want: "state=OPEN^case_type=case^priority=S1",
		},
		{
			name: "passes through an unmapped value unchanged",
			raw:  "state=OPEN^product=unknown-sys-id",
			want: "state=OPEN^product=unknown-sys-id",
		},
		{
			name: "empty input stays empty",
			raw:  "",
			want: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := translateSLACondition(c.raw)
			if got != c.want {
				t.Errorf("translateSLACondition(%q) = %q, want %q", c.raw, got, c.want)
			}
		})
	}
}

// TestTranslateSLACondition_RealRowsParseWithNoUnknownClauses runs every raw
// shape actually observed in the real sla_policy table (migrations
// 0250-0260's own backup/UPDATE statements are the source for these) through
// translateSLACondition then slacondition.Parse, and asserts no unknown
// clause survives translation -- the regression test for the exact blocker a
// PR review caught: an untranslated "^EQ" terminator made every one of these
// evaluate to false.
func TestTranslateSLACondition_RealRowsParseWithNoUnknownClauses(t *testing.T) {
	raws := []string{
		"u_case_type=8d4b87bd1b18f010cb6898aebd4bcb59^state=1^priority=10^u_wso2_response_comment=false^EQ",
		"product=691b4f861b746010cb6898aebd4bcbc0^ORproduct=720bf91d1bf92010cb6898aebd4bcb10^ORproduct=6436091d1b8ae210a002c9d3604bcb45^ORproduct=7bf5811d1b8ae210a002c9d3604bcbc8^state=1^priority=10^EQ",
		"project.u_project_type=cb4a87bd1b18f010cb6898aebd4bcb4f^ORproject.u_project_type=87a8676c1b347410cb6898aebd4bcb76^u_case_type=8d4b87bd1b18f010cb6898aebd4bcb59^priority=10^EQ",
		"stateINAWAITING_INFO,SOLUTION_PROPOSED^ORu_workaround_providedISNOTEMPTY^EQ",
		"u_workaround_providedISEMPTY^stateINWORK_IN_PROGRESS,WAITING_ON_WSO2,REOPENED^EQ",
		"priority=S1^u_case_type=8d4b87bd1b18f010cb6898aebd4bcb59^ORu_case_type=bfa1473c1bbcb410cb6898aebd4bcb52^sys_created_on>=javascript:gs.dateGenerate('2022-08-26','00:00:00')^EQ",
		"u_case_type!=3f5b47bd1b18f010cb6898aebd4bcbc2^u_case_type!=8d4b87bd1b18f010cb6898aebd4bcb59^state=1^u_case_typeISNOTEMPTY^EQ",
		"state=1^priority=10^u_case_type=8d4b87bd1b18f010cb6898aebd4bcb59^u_wso2_response_comment=false^NQstate=1^u_case_type=ab36479047ccf510a0a29cd3846d43ee^u_wso2_response_comment=false^EQ",
		"product!=691b4f861b746010cb6898aebd4bcbc0^product=720bf91d1bf92010cb6898aebd4bcb10^product=6436091d1b8ae210a002c9d3604bcb45^product=7bf5811d1b8ae210a002c9d3604bcbc8^ORpriority!=13^ORstate=",
	}
	for _, raw := range raws {
		translated := translateSLACondition(raw)
		cond := slacondition.Parse(translated)
		if unknown := cond.UnknownClauses(); len(unknown) > 0 {
			t.Errorf("translateSLACondition(%q) = %q, left unknown clauses: %v", raw, translated, unknown)
		}
	}
}

// facts is a small helper building a caseConditionFacts for the semantic
// tests below -- severity-only, since that's all these tests need to vary.
func sysIDTranslateTestFacts(caseType, severity string) caseConditionFacts {
	return caseConditionFacts{c: repository.CaseFacts{
		State:    "OPEN",
		Severity: severity,
		CaseType: caseType,
	}}
}

// TestTranslateSLACondition_IncidentQueryCaseTypeSemantics proves the fix
// for a PR review's High-severity finding (Incident and Query both used to
// translate to the same "case_type=case" value, so every Incident case
// matched the Query policies too) at the level that actually matters:
// Parse+Evaluate outcomes against fabricated facts, not just the translated
// text's surface spelling.
func TestTranslateSLACondition_IncidentQueryCaseTypeSemantics(t *testing.T) {
	const incidentSysID = "8d4b87bd1b18f010cb6898aebd4bcb59"
	const querySysID = "0d5b8fbd1b18f010cb6898aebd4bcba5"

	incidentS2Policy := slacondition.Parse(translateSLACondition("u_case_type=" + incidentSysID + "^priority=11^EQ")) // priority=11 -> S2
	queryPolicy := slacondition.Parse(translateSLACondition(
		"u_case_type!=3f5b47bd1b18f010cb6898aebd4bcbc2^u_case_type!=" + incidentSysID +
			"^u_case_type!=bfa1473c1bbcb410cb6898aebd4bcb52^state=1^u_case_type!=3b8b43311b58f010cb6898aebd4bcb8f" +
			"^u_case_typeISNOTEMPTY^u_case_type!=8f8fc2c41b0bd550d64e64a2604bcb38^u_case_type!=ab36479047ccf510a0a29cd3846d43ee" +
			"^u_case_type!=5aeff1201b74c210264c997a234bcb54^EQ")) // the real "none of the other types" catch-all, migration 0250's own 85fe07fa row

	s2Facts := sysIDTranslateTestFacts("case", "S2")
	s4Facts := sysIDTranslateTestFacts("case", "S4")

	if !incidentS2Policy.Evaluate(s2Facts) {
		t.Error("an S2 case must match the Incident/S2 policy")
	}
	if incidentS2Policy.Evaluate(s4Facts) {
		t.Error("an S4 case must NOT match the Incident/S2 policy")
	}
	if queryPolicy.Evaluate(s2Facts) {
		t.Error("an S2 (Incident) case must NOT match the Query catch-all policy -- this is the exact bug the fix closes")
	}
	if !queryPolicy.Evaluate(s4Facts) {
		t.Error("an S4 case must match the Query catch-all policy")
	}

	// Sanity check against the real, decoded condition for policy
	// 726dc150-1b64-0550-6a67-caa1604bcbad (migration 0250): Incident OR
	// Hosting at S2. With Hosting discarded, this must reduce to exactly
	// "S2 matches, nothing else does" -- not "always true" and not "never
	// matches".
	startCond := slacondition.Parse(translateSLACondition(
		"u_case_type=" + incidentSysID + "^ORu_case_type=bfa1473c1bbcb410cb6898aebd4bcb52^priority=11^EQ")) // priority=11 -> S2
	if !startCond.Evaluate(s2Facts) {
		t.Error("726dc150's start_condition (Incident-or-Hosting at S2) must match an S2 case")
	}
	if startCond.Evaluate(s4Facts) {
		t.Error("726dc150's start_condition must not match an S4 case")
	}
	if startCond.Evaluate(sysIDTranslateTestFacts("case", "S1")) {
		t.Error("726dc150's start_condition must not match an S1 case")
	}

	cancelCond := slacondition.Parse(translateSLACondition(
		"u_case_type!=" + incidentSysID + "^ORu_case_type!=bfa1473c1bbcb410cb6898aebd4bcb52^priority!=11^EQ")) // priority!=11 -> priority!=S2
	if cancelCond.Evaluate(s2Facts) {
		t.Error("726dc150's cancel_condition must NOT fire while the case is still S2")
	}
	if !cancelCond.Evaluate(s4Facts) {
		t.Error("726dc150's cancel_condition must fire once the case is no longer S2")
	}

	// Regression: a work item with NO severity at all (an engagement,
	// service_request, security_report_analysis or announcement -- every
	// case-like type other than "case" always has an empty Severity) must
	// never match a positive Incident condition. internal/slacondition's
	// OpNotEqual treats a missing field as "not equal to anything" (true),
	// so the earlier "priority!=S4" form wrongly matched here; the
	// membership form (OpIn, false when absent) does not. This is the real,
	// reachable path: ReviseCaseClocks runs on a type transfer away from
	// "case", and a synced Incident-only start_condition must not register a
	// clock against the transferred-to item.
	noSeverityFacts := sysIDTranslateTestFacts("engagement", "")
	if incidentS2Policy.Evaluate(noSeverityFacts) {
		t.Error("an Incident condition must not match a work item with no severity at all")
	}
	if startCond.Evaluate(noSeverityFacts) {
		t.Error("726dc150's Incident-or-Hosting start_condition must not match a work item with no severity at all")
	}
	// The mirror-image negated form must still be true for a severity-less
	// item (it genuinely isn't an Incident, so "case_type != Incident"
	// should hold) -- OpNotIn's "absent means not excluded" gives this for
	// free, same as OpNotEqual did by luck before.
	if !cancelCond.Evaluate(noSeverityFacts) {
		t.Error("726dc150's cancel_condition (case_type != Incident/Hosting OR priority != S2) must still fire for a work item with no severity at all")
	}
}

// TestTranslateSLACondition_HostingDiscarded proves a Hosting-only policy
// (nothing else distinguishing it) never matches any real case, at any
// severity -- the deliberate, product-decided "discard Hosting" outcome,
// not an oversight.
func TestTranslateSLACondition_HostingDiscarded(t *testing.T) {
	hostingOnly := slacondition.Parse(translateSLACondition("u_case_type=bfa1473c1bbcb410cb6898aebd4bcb52^priority=11^EQ")) // Hosting, S2
	for _, sev := range []string{"S0", "S1", "S2", "S3", "S4"} {
		if hostingOnly.Evaluate(sysIDTranslateTestFacts("case", sev)) {
			t.Errorf("a Hosting-only policy must never match any case (severity %s) -- Hosting is discarded by product decision", sev)
		}
	}
}

// TestTranslateSLACondition_AdminTaskStaysUnmapped proves the Admin Task
// sys_id (slaAdminTaskCaseTypeSysID) is deliberately NOT in slaCaseTypeSysIDs
// -- confirmed with the team that Admin Task cases are no longer created, so
// there's nothing to map it to -- and that a positive clause against it
// still never matches a real case (same fail-closed outcome as before,
// unchanged).
func TestTranslateSLACondition_AdminTaskStaysUnmapped(t *testing.T) {
	if _, mapped := slaCaseTypeSysIDs[slaAdminTaskCaseTypeSysID]; mapped {
		t.Fatalf("slaAdminTaskCaseTypeSysID must stay unmapped in slaCaseTypeSysIDs, got an entry: %q", slaCaseTypeSysIDs[slaAdminTaskCaseTypeSysID])
	}

	positive := slacondition.Parse(translateSLACondition("u_case_type=" + slaAdminTaskCaseTypeSysID + "^EQ"))
	for _, ct := range []string{"case", "engagement", "service_request", "security_report_analysis", "announcement"} {
		if positive.Evaluate(sysIDTranslateTestFacts(ct, "S1")) {
			t.Errorf("a positive Admin Task clause must never match a real CaseType=%q", ct)
		}
	}
}
