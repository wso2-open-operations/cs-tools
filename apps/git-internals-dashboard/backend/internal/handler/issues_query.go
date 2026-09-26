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

package handler

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/appconfig"
)

var (
	repoParamRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	// qParamRe caps at 9 digits: issues.github_number is a Postgres integer
	// (max 2,147,483,647, 10 digits), so a 10-digit value could exceed
	// int32 range even though it would just never match anyway — 9 digits
	// keeps every accepted value unambiguously in range.
	qParamRe = regexp.MustCompile(`^\d{1,9}$`)
)

// noPriorityValue is the `priority` sentinel meaning "issue has no
// priority" (i.priority IS NULL). Matches the webapp's NO_PRIORITY_VALUE
// (src/lib/filters.ts) one-for-one.
const noPriorityValue = "__none__"

// issuesQuery is GET /issues's validated query params. Repos, Priorities,
// Statuses, AbtTeams and SlaStates hold every value from a possibly-repeated
// query key (e.g. `?status=WOC&status=Pending+Patch+Queue`); values within
// one are OR-ed together by buildIssuesWhere.
type issuesQuery struct {
	Repos      []string
	Priorities []string
	State      string
	SlaStates  []string
	Statuses   []string
	AbtTeams   []string
	Q          string
	Limit      int
	Offset     int
	Bucket     string
	Sort       issueSortField
	Order      issueSortOrder
}

// multiValues reads name's repeated values from v, dropping empty strings
// and de-duplicating while preserving first-seen order, so an OR-list never
// carries a redundant SQL array element.
func multiValues(v url.Values, name string) []string {
	raw := v[name]
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(raw))
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// parseMultiParam reads name's values via multiValues, rejects more than
// maxValues of them, and validates each with validate. A non-empty return
// from validate short-circuits with that message.
func parseMultiParam(v url.Values, name string, maxValues int, validate func(string) string) ([]string, string) {
	values := multiValues(v, name)
	if len(values) > maxValues {
		return nil, fmt.Sprintf("%s accepts at most %d values", name, maxValues)
	}
	for _, val := range values {
		if errMsg := validate(val); errMsg != "" {
			return nil, errMsg
		}
	}
	return values, ""
}

// optionalText reads name from v, validating it against maxLen. Returns
// ("", "") when the param is absent, (value, "") when present and valid, or
// ("", errMsg) on the first length violation. Used by the single-valued
// metrics endpoints; /issues uses parseMultiParam instead.
func optionalText(v url.Values, name string, maxLen int) (value string, errMsg string) {
	raw := v.Get(name)
	if raw == "" {
		return "", ""
	}
	if len(raw) > maxLen {
		return "", fmt.Sprintf("%s must be at most %d characters", name, maxLen)
	}
	return raw, ""
}

func maxLengthValidator(name string, maxLen int) func(string) string {
	return func(s string) string {
		if len(s) > maxLen {
			return fmt.Sprintf("%s must be at most %d characters", name, maxLen)
		}
		return ""
	}
}

// parseIssuesQuery validates v against lim and returns a non-empty error
// message on the first violation found. Unknown query parameter names are
// intentionally ignored rather than rejected.
func parseIssuesQuery(v url.Values, lim appconfig.API) (issuesQuery, string) {
	q := issuesQuery{Limit: lim.IssuesDefaultLimit, Sort: defaultIssueSort, Order: defaultIssueSortOrder}

	repos, errMsg := parseMultiParam(v, "repo", lim.FilterParamMaxValues, func(s string) string {
		if !repoParamRe.MatchString(s) {
			return "repo must be owner/name"
		}
		return ""
	})
	if errMsg != "" {
		return q, errMsg
	}
	q.Repos = repos

	priorities, errMsg := parseMultiParam(v, "priority", lim.FilterParamMaxValues, maxLengthValidator("priority", lim.PriorityParamMaxLength))
	if errMsg != "" {
		return q, errMsg
	}
	q.Priorities = priorities

	if state := v.Get("state"); state != "" {
		if state != "OPEN" && state != "CLOSED" {
			return q, "state must be OPEN or CLOSED"
		}
		q.State = state
	}

	slaStates, errMsg := parseMultiParam(v, "slaState", lim.FilterParamMaxValues, func(s string) string {
		switch s {
		case "NO_SLA", "OK", "AT_RISK", "VIOLATED", "TERMINAL":
			return ""
		default:
			return "slaState must be one of NO_SLA, OK, AT_RISK, VIOLATED, TERMINAL"
		}
	})
	if errMsg != "" {
		return q, errMsg
	}
	q.SlaStates = slaStates

	statuses, errMsg := parseMultiParam(v, "status", lim.FilterParamMaxValues, maxLengthValidator("status", lim.StatusParamMaxLength))
	if errMsg != "" {
		return q, errMsg
	}
	q.Statuses = statuses

	abtTeams, errMsg := parseMultiParam(v, "abtTeam", lim.FilterParamMaxValues, maxLengthValidator("abtTeam", lim.AbtTeamParamMaxLength))
	if errMsg != "" {
		return q, errMsg
	}
	q.AbtTeams = abtTeams

	if qq := v.Get("q"); qq != "" {
		if !qParamRe.MatchString(qq) {
			return q, "q must be an issue number"
		}
		q.Q = qq
	}
	if limitStr := v.Get("limit"); limitStr != "" {
		n, err := strconv.Atoi(limitStr)
		if err != nil || n < 1 || n > lim.IssuesMaxLimit {
			return q, fmt.Sprintf("limit must be an integer between 1 and %d", lim.IssuesMaxLimit)
		}
		q.Limit = n
	}
	if offsetStr := v.Get("offset"); offsetStr != "" {
		n, err := strconv.Atoi(offsetStr)
		if err != nil || n < 0 {
			return q, "offset must be a non-negative integer"
		}
		q.Offset = n
	}
	if bucket := v.Get("bucket"); bucket != "" {
		switch bucket {
		case "all", "violated", "at_risk", "on_track", "cs", "product_side", "tracked", "untracked", "attention":
			q.Bucket = bucket
		default:
			return q, "bucket must be one of all, violated, at_risk, on_track, cs, product_side, tracked, untracked, attention"
		}
	}
	if sort := v.Get("sort"); sort != "" {
		field := issueSortField(sort)
		if _, ok := issueSortColumns[field]; !ok {
			return q, "sort must be one of " + strings.Join(validIssueSortValues(), ", ")
		}
		q.Sort = field
	}
	if order := v.Get("order"); order != "" {
		switch issueSortOrder(order) {
		case orderAsc, orderDesc:
			q.Order = issueSortOrder(order)
		default:
			return q, "order must be one of asc, desc"
		}
	}
	return q, ""
}

// sqlArgs accumulates parameterized query args and hands back "$N"
// placeholders in the order they're added. Only ever populated during the
// final materialization pass in buildIssuesWhere, so every allocated
// placeholder is guaranteed to appear in the rendered SQL — an argument
// added and then never referenced leaves Postgres unable to infer that
// parameter's type at prepare time.
type sqlArgs struct{ values []any }

// add appends v and returns its "$N" placeholder.
func (a *sqlArgs) add(v any) string {
	a.values = append(a.values, v)
	return fmt.Sprintf("$%d", len(a.values))
}

// slaFilter and statusFilter describe the *decided* bucket-derived filter
// for that column — a plain data value, not SQL text — so materializing SQL
// only once, at the very end, never allocates a placeholder that ends up
// unused.
type slaFilter struct {
	mode  string // "notTerminal" | "eq" | "none"
	value string
}

type statusFilter struct {
	mode   string // "" | "in" | "notIn"
	values []string
}

// priorityListCondition renders the explicit `priority` filter: real values
// OR-ed via ANY, with i.priority IS NULL added when noPriorityValue is
// present (or standing alone if it's the only value). Returns "" when
// priorities is empty.
func priorityListCondition(args *sqlArgs, priorities []string) string {
	if len(priorities) == 0 {
		return ""
	}
	var real []string
	hasNone := false
	for _, p := range priorities {
		if p == noPriorityValue {
			hasNone = true
			continue
		}
		real = append(real, p)
	}
	switch {
	case len(real) == 0:
		return "i.priority IS NULL"
	case hasNone:
		return "(i.priority = ANY(" + args.add(real) + ") OR i.priority IS NULL)"
	default:
		return "i.priority = ANY(" + args.add(real) + ")"
	}
}

// buildIssuesWhere translates q into a SQL WHERE clause body (without the
// "WHERE" keyword) plus its parameter args. `bucket` defines a scope —
// today's SLA/status/priority/attention conditions for violated, at_risk,
// on_track, cs, product_side, tracked, untracked and attention — and every
// explicit filter list (repo, priority, abtTeam, status, slaState) is AND-ed
// on top of it: a bucket never overrides an explicit filter. The one
// exception is slaState: when the bucket left the SLA rule at the
// base "exclude TERMINAL" default, an explicit slaState list replaces that
// default instead of AND-ing with it, since a caller who names SLA states
// explicitly is stating the whole SLA scope themselves (this also covers
// `slaState=NO_SLA` matching an issue with no issue_sla row at all, via
// COALESCE). csStatuses and productSideStatuses must be sortOrder-ascending
// status names categorized CS_SIDE and PRODUCT_SIDE respectively.
func buildIssuesWhere(csStatuses, productSideStatuses []string, q issuesQuery) (string, []any) {
	// Base scope: open, non-terminal issues from enabled repos.
	state := "OPEN"
	sla := slaFilter{mode: "notTerminal"}
	priorityBucket := "" // "" | "notNull" | "isNull"
	var statusBucket statusFilter
	attention := false

	switch q.Bucket {
	case "violated":
		sla = slaFilter{mode: "eq", value: "VIOLATED"}
	case "at_risk":
		sla = slaFilter{mode: "eq", value: "AT_RISK"}
	case "on_track":
		sla = slaFilter{mode: "eq", value: "OK"}
		statusBucket = statusFilter{mode: "notIn", values: csStatuses}
	case "cs":
		// The sla filter is cleared so NO_SLA issues currently on the CS
		// side are still included.
		statusBucket = statusFilter{mode: "in", values: csStatuses}
		sla = slaFilter{mode: "none"}
	case "product_side":
		// Mirrors overview.go's hero.productSide count: base open/non-terminal
		// scope (sla stays "notTerminal"), narrowed to statuses currently
		// categorized PRODUCT_SIDE.
		statusBucket = statusFilter{mode: "in", values: productSideStatuses}
	case "tracked":
		priorityBucket = "notNull"
	case "untracked":
		priorityBucket = "isNull"
		sla = slaFilter{mode: "none"}
	case "attention":
		sla = slaFilter{mode: "none"}
		attention = true
	default: // "all" or unset — keep base scope.
		if q.State != "" {
			state = q.State
		}
	}

	if len(q.SlaStates) > 0 && sla.mode == "notTerminal" {
		sla = slaFilter{mode: "none"}
	}

	args := &sqlArgs{}
	conditions := []string{"r.enabled = true", "i.state = " + args.add(state)}

	switch sla.mode {
	case "notTerminal":
		conditions = append(conditions, "s.sla_state IS DISTINCT FROM "+args.add("TERMINAL"))
	case "eq":
		conditions = append(conditions, "s.sla_state = "+args.add(sla.value))
	}
	if len(q.SlaStates) > 0 {
		conditions = append(conditions, "COALESCE(s.sla_state, 'NO_SLA') = ANY("+args.add(q.SlaStates)+")")
	}

	switch priorityBucket {
	case "notNull":
		conditions = append(conditions, "i.priority IS NOT NULL")
	case "isNull":
		conditions = append(conditions, "i.priority IS NULL")
	}
	if cond := priorityListCondition(args, q.Priorities); cond != "" {
		conditions = append(conditions, cond)
	}

	switch statusBucket.mode {
	case "in":
		conditions = append(conditions, "i.current_status = ANY("+args.add(statusBucket.values)+")")
	case "notIn":
		conditions = append(conditions, "i.current_status <> ALL("+args.add(statusBucket.values)+")")
	}
	if len(q.Statuses) > 0 {
		conditions = append(conditions, "i.current_status = ANY("+args.add(q.Statuses)+")")
	}

	if attention {
		conditions = append(conditions, fmt.Sprintf(
			"(s.sla_state = ANY(%s) OR i.current_status = ANY(%s))",
			args.add([]string{"VIOLATED", "AT_RISK"}), args.add(csStatuses),
		))
	}

	if len(q.Repos) > 0 {
		conditions = append(conditions, "(r.owner || '/' || r.name) = ANY("+args.add(q.Repos)+")")
	}
	if q.Q != "" {
		n, _ := strconv.Atoi(q.Q) // format guaranteed by parseIssuesQuery
		conditions = append(conditions, "i.github_number = "+args.add(n))
	}
	if len(q.AbtTeams) > 0 {
		conditions = append(conditions, "i.abt_team = ANY("+args.add(q.AbtTeams)+")")
	}

	return strings.Join(conditions, " AND "), args.values
}
