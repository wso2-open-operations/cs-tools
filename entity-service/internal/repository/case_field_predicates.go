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
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// caseFieldSet is the set of case-search fields that can appear both at the top
// level of a search and inside an anyOf branch. One builder (caseFieldPredicates)
// turns it into SQL, so the two places cannot drift apart -- an earlier state
// filter fixed in one and not the other is what this exists to prevent.
type caseFieldSet struct {
	Types            []string
	ProjectIDs       []string
	DeploymentIDs    []string
	AssignedUserIDs  []string
	States           []domain.CaseState
	Severities       []domain.CaseSeverity
	IssueTypes       []domain.CaseIssueType
	EngagementTypes  []domain.EngagementType
	WorkStates       []domain.CaseWorkState
	EscalationLevels []string
	Tags             []string
	ExcludeTags      []string

	// DefaultTypes limits an empty Types to the five case-like work-item types.
	// True for the top-level search, so it never returns change requests or
	// incidents; false inside an anyOf branch, where "no type" means no
	// constraint from that branch.
	DefaultTypes bool
}

// escalationEnumLabels maps escalationLevel filter ids ("0".."5") to
// case_escalation_level_enum labels ("EL0".."EL5"). Anything else is a
// validation error: an unknown id would otherwise reach the database as an
// invalid enum value and surface as a 500.
func escalationEnumLabels(ids []string) ([]string, error) {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if len(id) != 1 || id[0] < '0' || id[0] > '5' {
			return nil, &apierror.ValidationError{Msg: "escalationLevel contains invalid value: " + id + " (expected 0-5)"}
		}
		out = append(out, "EL"+id)
	}
	return out, nil
}

// caseFieldPredicates returns the SQL conditions (no leading AND) and bound
// arguments for f, numbering placeholders from argIdx, and the next free index.
//
// Column notes: state is matched on caseLikeStateColumn (it exists on all five
// case-like extension tables); severity, issue type, work state and escalation
// level live only on "case" (migration 000018) and engagement type only on
// engagement, so those narrow the result to those rows. escalationLevel matches
// "case".current_escalation_level, the value the case detail shows. Labels are
// UPPER_SNAKE_CASE in the enums and lowercase in the domain, hence ToUpper;
// severity is the exception (S0..S4, see caseSeverityToEnum).
func caseFieldPredicates(f caseFieldSet, argIdx int) ([]string, []any, int, error) {
	var preds []string
	var args []any
	add := func(clause string, val any) {
		preds = append(preds, fmt.Sprintf(clause, argIdx))
		args = append(args, val)
		argIdx++
	}
	upper := func(n int, at func(int) string) []string {
		out := make([]string, n)
		for i := 0; i < n; i++ {
			out[i] = strings.ToUpper(at(i))
		}
		return out
	}

	switch {
	case len(f.Types) > 0:
		// Types holds validCaseType's lowercase values ("case", "service_request",
		// ...); work_item_type_enum's labels match 1:1 once uppercased.
		add("wi.type = ANY($%d::work_item_type_enum[])", upper(len(f.Types), func(i int) string { return f.Types[i] }))
	case f.DefaultTypes:
		preds = append(preds, "wi.type = ANY("+caseLikeWorkItemTypes+")")
	}
	if len(f.ProjectIDs) > 0 {
		add("wi.project_id = ANY($%d::uuid[])", f.ProjectIDs)
	}
	if len(f.DeploymentIDs) > 0 {
		add("wi.deployment_id = ANY($%d::uuid[])", f.DeploymentIDs)
	}
	if len(f.States) > 0 {
		add(caseLikeStateColumn+" = ANY($%d::text[])", upper(len(f.States), func(i int) string { return string(f.States[i]) }))
	}
	if len(f.Severities) > 0 {
		sev := make([]string, len(f.Severities))
		for i, s := range f.Severities {
			sev[i] = caseSeverityToEnum[s]
		}
		add("c.severity = ANY($%d::case_severity_enum[])", sev)
	}
	if len(f.IssueTypes) > 0 {
		add("c.issue_type = ANY($%d::case_issue_type_enum[])", upper(len(f.IssueTypes), func(i int) string { return string(f.IssueTypes[i]) }))
	}
	if len(f.EngagementTypes) > 0 {
		// engagement.type (migration 000019) is a column on the separate
		// engagement subtype table, not on "case".
		add("eng.type = ANY($%d::engagement_type_enum[])", upper(len(f.EngagementTypes), func(i int) string { return string(f.EngagementTypes[i]) }))
	}
	if len(f.WorkStates) > 0 {
		add("c.work_state = ANY($%d::case_work_state_enum[])", upper(len(f.WorkStates), func(i int) string { return string(f.WorkStates[i]) }))
	}
	if len(f.AssignedUserIDs) > 0 {
		add("wi.assigned_to_id = ANY($%d::uuid[])", f.AssignedUserIDs)
	}
	if len(f.EscalationLevels) > 0 {
		labels, err := escalationEnumLabels(f.EscalationLevels)
		if err != nil {
			return nil, nil, argIdx, err
		}
		add("c.current_escalation_level = ANY($%d::text[]::case_escalation_level_enum[])", labels)
	}
	// tag: a case carries a tag when a work_item_tag row links it to a tag of that
	// name, compared case-insensitively as AddCaseTag looks tags up. in = carries
	// ANY of the names; notIn = carries NONE (an untagged case satisfies it).
	if len(f.Tags) > 0 {
		add("EXISTS (SELECT 1 FROM work_item_tag wit JOIN tag t ON t.id = wit.tag_id WHERE wit.work_item_id = wi.id AND LOWER(t.name) = ANY($%d::text[]))", lowerAll(f.Tags))
	}
	if len(f.ExcludeTags) > 0 {
		add("NOT EXISTS (SELECT 1 FROM work_item_tag wit JOIN tag t ON t.id = wit.tag_id WHERE wit.work_item_id = wi.id AND LOWER(t.name) = ANY($%d::text[]))", lowerAll(f.ExcludeTags))
	}
	return preds, args, argIdx, nil
}

// caseFieldSetFromGroup converts one anyOf branch into a caseFieldSet. A branch
// never defaults its types: an empty Types adds no constraint of its own.
func caseFieldSetFromGroup(g domain.CaseFilterGroup) caseFieldSet {
	return caseFieldSet{
		Types: g.Types, ProjectIDs: g.ProjectIDs, DeploymentIDs: g.DeploymentIDs,
		AssignedUserIDs: g.AssignedUserIDs, States: g.States, Severities: g.Severities,
		IssueTypes: g.IssueTypes, EngagementTypes: g.EngagementTypes, WorkStates: g.WorkStates,
		EscalationLevels: g.EscalationLevels, Tags: g.Tags, ExcludeTags: g.ExcludeTags,
	}
}
