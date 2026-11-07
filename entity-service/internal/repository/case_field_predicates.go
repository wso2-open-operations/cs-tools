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
	AccountIDs       []string
	DeploymentIDs    []string
	AssignedUserIDs  []string
	States           []domain.CaseState
	Severities       []domain.CaseSeverity
	IssueTypes       []domain.CaseIssueType
	EngagementTypes  []domain.EngagementType
	SalesRegions     []string
	WorkStates       []domain.CaseWorkState
	EscalationLevels []string
	Tags             []string
	ExcludeTags      []string

	// SeverityColumn is the severity column expression the Severities filter
	// matches (see severityColumnFor): "c.severity" when empty. It includes a
	// service request's severity only for a caller allowed to see it.
	SeverityColumn string

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

// caseLikeStateLookupTables lists, for each of validCaseType's five
// case-like values, the per-type SQL that selects the ids of that type's
// rows matching a bound ANY() state list -- announcement's branch applies
// the same CLOSE->CLOSED normalization caseLikeStateColumn itself applies
// (see that const's own doc comment), so the two can never disagree on what
// "closed" means for an announcement row.
//
// This exists to avoid evaluating caseLikeStateColumn's five-way COALESCE
// as a post-join Filter, which real production-volume testing confirmed
// costs the most database time of any query this service runs, by far: the
// COALESCE is a runtime expression over five LEFT-joined tables, not a
// column, so it can never be served by any index, and every row
// caseSearchJoins admits has to be joined to all five extension tables
// before it can even be evaluated. Every dashboard widget's case search is,
// in practice, exactly a {field:"type"} filter alongside a {field:"state"}
// one -- so looking the matching ids up directly in just the type(s) actually requested lets the planner use
// that type's own existing state index (e.g. idx_case_state) and skip the
// other four extension-table joins entirely for the common case, instead of
// joining everything and filtering after.
//
// Known, accepted divergence from the COALESCE it replaces: a work_item row
// whose own `type` column disagrees with which extension table actually
// holds its data (a pre-existing, rare sync/data-quality issue -- confirmed
// live) is found by the old COALESCE regardless of its declared type,
// because that approach blindly joins and checks all five tables for every
// row. This lookup trusts wi.type and only checks that type's own table, so
// it diverges from the COALESCE both ways for such a row: negate=false
// (the `in` filter) misses it when the caller narrows Types to something
// other than the table the row's data actually lives in, and negate=true
// (the `notIn` filter) wrongly keeps it for the mirror-image reason -- its
// declared type's own table has no row to find, so this lookup can never
// see the state that should have excluded it. Verified end-to-end against a
// wide range of real dashboard filter combinations: this was the only
// source of divergence found, and only for rows already affected by that
// pre-existing issue. Deliberately not fixed by also checking the other
// four tables regardless of Types -- that would reproduce the exact cost
// this function exists to
// avoid, to compensate for a sync-side bug that belongs in the data, not in
// every case search query from here on.
//
// The four enum-typed branches compare the state column itself, not
// state::TEXT: casting the column hides it from its index (idx_case_state),
// so `state::TEXT = ANY(...)` was a sequential scan of "case" (measured on
// production-size data: 10.5 ms seq scan against 1.6 ms index-only scan).
// The bound list stays text[] and is turned into the table's own enum by
// keeping only the labels that enum has (caseLikeEnumStateList), so a label
// the enum does not know matches nothing, as the text comparison did, rather
// than failing the whole query the way a direct ::case_state_enum[] cast
// would. Announcement keeps the text comparison: its CLOSE->CLOSED
// normalization needs it, and it has no state index to lose.
var caseLikeStateLookupTables = map[string]string{
	"case":                     `SELECT id FROM "case" WHERE state = ANY(` + caseLikeEnumStateList("case_state_enum") + `)`,
	"engagement":               `SELECT id FROM engagement WHERE state = ANY(` + caseLikeEnumStateList("engagement_state_enum") + `)`,
	"service_request":          `SELECT id FROM service_request WHERE state = ANY(` + caseLikeEnumStateList("service_request_state_enum") + `)`,
	"security_report_analysis": `SELECT id FROM security_report_analysis WHERE state = ANY(` + caseLikeEnumStateList("security_report_analysis_state_enum") + `)`,
	"announcement":             `SELECT id FROM announcement WHERE (CASE WHEN state::TEXT = 'CLOSE' THEN 'CLOSED' ELSE state::TEXT END) = ANY(%[1]s)`,
}

// caseLikeEnumStateList returns the SQL for an array of enumType holding the
// labels of the bound text[] (the %[1]s placeholder) that enumType has, in
// enum order. Unknown labels are dropped instead of raising "invalid input
// value for enum".
func caseLikeEnumStateList(enumType string) string {
	return `ARRAY(SELECT s FROM unnest(enum_range(NULL::` + enumType + `)) s WHERE s::TEXT = ANY(%[1]s))`
}

// caseLikeStateLookupAllTypes is every type caseLikeStateLookupTables covers,
// in a fixed order -- the scope used whenever the caller names no type
// filter of its own, so the lookup still covers exactly what
// caseLikeStateColumn's COALESCE always implicitly covers (any of the five).
var caseLikeStateLookupAllTypes = []string{"case", "engagement", "service_request", "security_report_analysis", "announcement"}

// caseLikeStateLookupClause renders a "wi.id = ANY(ARRAY(...))" (or, when
// negate, a "NOT EXISTS (... WHERE excluded.id = wi.id)") predicate equivalent to filtering
// caseLikeStateColumn against the bound parameter at placeholder (e.g.
// "$3::text[]"), as a UNION ALL of per-type id lookups scoped to types --
// every case-like type when types is empty (the DefaultTypes case, or an
// anyOf branch that names no type of its own, same as the COALESCE it
// replaces). A type not in caseLikeStateLookupTables (reachable only from an
// anyOf branch naming a non-case-like type, e.g. "incident") contributes no
// branch: such a row can never have a case-like state at all, exactly how
// the COALESCE already treats it (every one of the five joins is NULL for
// it). If no requested type is case-like, the predicate degrades to the same
// answer the COALESCE already gives in that situation -- FALSE for "in" (can
// never match), TRUE for "not in" (always satisfies an exclusion it has no
// state to violate) -- rather than emitting invalid empty SQL.
//
// Wrapped in ARRAY(...) rather than used as a bare "wi.id [NOT ]IN (...)"
// subquery: confirmed against real production-volume data that the ARRAY
// form evaluates the UNION ALL exactly once (an upfront, independent
// computation) and then probes work_item's own primary key per element,
// while a bare IN/NOT IN subquery here is prone to being planned as a join
// against the subquery's result set instead -- which, combined with this
// schema's row-level-security predicates layered onto every table, measured
// meaningfully slower than the ARRAY form for the same result.
//
// The negated form (the ExcludeStates / notIn filter) is deliberately NOT the
// ARRAY form. "wi.id <> ALL(ARRAY(...))" cannot be served by an index or a
// hash: PostgreSQL compares every candidate row with every element of the
// array in turn, so its cost is rows x excluded ids. For "state notIn closed"
// the array holds every closed case-like id -- ~115,000 on production-volume
// data -- and the search took tens of seconds (past the 10 s request timeout,
// so the case list came back empty). NOT EXISTS over the same UNION ALL is
// planned as a hash anti-join and checks each row once: measured on
// production-size data at 27-70 ms against 5.5-32.5 s, with identical
// results. ids are always non-NULL (each branch selects a primary key), so
// the anti-join is an exact negation of "= ANY" here, with no
// three-valued-logic subtlety to account for.
func caseLikeStateLookupClause(types []string, placeholder string, negate bool) string {
	scope := types
	if len(scope) == 0 {
		scope = caseLikeStateLookupAllTypes
	}
	branches := make([]string, 0, len(scope))
	for _, t := range scope {
		tmpl, ok := caseLikeStateLookupTables[t]
		if !ok {
			continue
		}
		branches = append(branches, fmt.Sprintf(tmpl, placeholder))
	}
	if len(branches) == 0 {
		if negate {
			return "TRUE"
		}
		return "FALSE"
	}
	union := strings.Join(branches, " UNION ALL ")
	if negate {
		return fmt.Sprintf("NOT EXISTS (SELECT 1 FROM (%s) excluded WHERE excluded.id = wi.id)", union)
	}
	return fmt.Sprintf("wi.id = ANY(ARRAY(%s))", union)
}

// caseFieldPredicates returns the SQL conditions (no leading AND) and bound
// arguments for f, numbering placeholders from argIdx, and the next free index.
//
// Column notes: state is matched on caseLikeStateColumn (it exists on all five
// case-like extension tables); severity (and, for an internal caller, a service request's), issue type,
// work state and escalation level live on "case" (migration 0023) and engagement type only on
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
	if len(f.AccountIDs) > 0 {
		add("wi.account_id = ANY($%d::uuid[])", f.AccountIDs)
	}
	if len(f.DeploymentIDs) > 0 {
		add("wi.deployment_id = ANY($%d::uuid[])", f.DeploymentIDs)
	}
	if len(f.States) > 0 {
		// See caseLikeStateLookupClause's own doc comment for why this is a
		// targeted id lookup rather than caseLikeStateColumn's COALESCE.
		placeholder := fmt.Sprintf("$%d::text[]", argIdx)
		preds = append(preds, caseLikeStateLookupClause(f.Types, placeholder, false))
		args = append(args, upper(len(f.States), func(i int) string { return string(f.States[i]) }))
		argIdx++
	}
	if len(f.Severities) > 0 {
		sev := make([]string, len(f.Severities))
		for i, s := range f.Severities {
			sev[i] = caseSeverityToEnum[s]
		}
		col := f.SeverityColumn
		if col == "" {
			col = "c.severity"
		}
		add(col+" = ANY($%d::case_severity_enum[])", sev)
	}
	if len(f.IssueTypes) > 0 {
		add("c.issue_type = ANY($%d::case_issue_type_enum[])", upper(len(f.IssueTypes), func(i int) string { return string(f.IssueTypes[i]) }))
	}
	if len(f.EngagementTypes) > 0 {
		// engagement.type (migration 0024) is a column on the separate
		// engagement subtype table, not on "case".
		add("eng.type = ANY($%d::engagement_type_enum[])", upper(len(f.EngagementTypes), func(i int) string { return string(f.EngagementTypes[i]) }))
	}
	if len(f.SalesRegions) > 0 {
		// The parent account's sales region, on the "a" join caseSearchJoins
		// already carries for every search (no extra join). Case-insensitive on
		// trimmed values; the bound list is already trimmed by the filter parser.
		add("LOWER(BTRIM(a.sales_region)) = ANY($%d::text[])", lowerAll(f.SalesRegions))
	}
	if len(f.WorkStates) > 0 {
		// Every case-like type but announcement carries a work state (migration 0184).
		add(caseLikeWorkStateColumn+" = ANY($%d::text[])", upper(len(f.WorkStates), func(i int) string { return string(f.WorkStates[i]) }))
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
		IssueTypes: g.IssueTypes, EngagementTypes: g.EngagementTypes, SalesRegions: g.SalesRegions, WorkStates: g.WorkStates,
		EscalationLevels: g.EscalationLevels, Tags: g.Tags, ExcludeTags: g.ExcludeTags,
	}
}
