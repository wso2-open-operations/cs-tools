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
	"slices"
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

// issuesQuery is GET /issues's validated query params.
type issuesQuery struct {
	Repo     string
	Priority string
	State    string
	SlaState string
	Status   string
	Q        string
	Limit    int
	Bucket   string
	Order    string
}

// parseIssuesQuery validates v against lim and returns a non-empty error
// message on the first violation found. Unknown query parameter names are
// intentionally ignored rather than rejected.
func parseIssuesQuery(v url.Values, lim appconfig.API) (issuesQuery, string) {
	q := issuesQuery{Limit: lim.IssuesDefaultLimit, Order: "updated_desc"}

	if repo := v.Get("repo"); repo != "" {
		if !repoParamRe.MatchString(repo) {
			return q, "repo must be owner/name"
		}
		q.Repo = repo
	}
	if priority := v.Get("priority"); priority != "" {
		if len(priority) > lim.PriorityParamMaxLength {
			return q, fmt.Sprintf("priority must be at most %d characters", lim.PriorityParamMaxLength)
		}
		q.Priority = priority
	}
	if state := v.Get("state"); state != "" {
		if state != "OPEN" && state != "CLOSED" {
			return q, "state must be OPEN or CLOSED"
		}
		q.State = state
	}
	if slaState := v.Get("slaState"); slaState != "" {
		switch slaState {
		case "NO_SLA", "OK", "AT_RISK", "VIOLATED", "TERMINAL":
			q.SlaState = slaState
		default:
			return q, "slaState must be one of NO_SLA, OK, AT_RISK, VIOLATED, TERMINAL"
		}
	}
	if status := v.Get("status"); status != "" {
		if len(status) > lim.StatusParamMaxLength {
			return q, fmt.Sprintf("status must be at most %d characters", lim.StatusParamMaxLength)
		}
		q.Status = status
	}
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
	if bucket := v.Get("bucket"); bucket != "" {
		switch bucket {
		case "all", "violated", "at_risk", "on_track", "cs", "product_side", "tracked", "untracked", "attention":
			q.Bucket = bucket
		default:
			return q, "bucket must be one of all, violated, at_risk, on_track, cs, product_side, tracked, untracked, attention"
		}
	}
	if order := v.Get("order"); order != "" {
		switch order {
		case "budget_desc", "updated_desc":
			q.Order = order
		default:
			return q, "order must be budget_desc or updated_desc"
		}
	}
	return q, ""
}

// sqlArgs accumulates parameterized query args and hands back "$N"
// placeholders in the order they're added. Only ever populated during the
// final materialization pass in buildIssuesWhere, so every allocated
// placeholder is guaranteed to appear in the rendered SQL — an argument
// added and then never referenced (e.g. because a later decision replaced
// it) leaves Postgres unable to infer that parameter's type at prepare time.
type sqlArgs struct{ values []any }

// add appends v and returns its "$N" placeholder.
func (a *sqlArgs) add(v any) string {
	a.values = append(a.values, v)
	return fmt.Sprintf("$%d", len(a.values))
}

// slaFilter, priorityFilter, and statusFilter describe the *decided* filter
// for that column — a plain data value, not SQL text — so a bucket
// overriding an earlier decision (e.g. a param-driven priority filter)
// simply replaces the Go value instead of leaving an orphaned SQL argument
// behind. SQL is materialized only once, at the very end, from whichever
// values survive every override.
type slaFilter struct {
	mode  string // "notTerminal" | "eq" | "none"
	value string
}

type priorityFilter struct {
	mode  string // "" | "eq" | "notNull" | "isNull"
	value string
}

type statusFilter struct {
	mode   string // "" | "eq" | "in" | "notIn"
	value  string
	values []string
}

// buildIssuesWhere translates q into a SQL WHERE clause body (without the
// "WHERE" keyword) plus its parameter args. Each bucket overrides the base
// scope field-by-field (e.g. "cs" clears the sla filter so NO_SLA issues on
// the CS side are included; "attention" = VIOLATED ∪ AT_RISK ∪ current CS
// statuses). csStatuses and productSideStatuses must be sortOrder-ascending
// status names categorized CS_SIDE and PRODUCT_SIDE respectively.
func buildIssuesWhere(csStatuses, productSideStatuses []string, q issuesQuery) (string, []any) {
	// Base scope: open, non-terminal issues from enabled repos.
	state := "OPEN"
	sla := slaFilter{mode: "notTerminal"}
	var priority priorityFilter
	var status statusFilter
	attention := false

	if q.Priority != "" {
		priority = priorityFilter{mode: "eq", value: q.Priority}
	}
	if q.Status != "" {
		status = statusFilter{mode: "eq", value: q.Status}
	}

	switch q.Bucket {
	case "violated":
		sla = slaFilter{mode: "eq", value: "VIOLATED"}
	case "at_risk":
		sla = slaFilter{mode: "eq", value: "AT_RISK"}
	case "on_track":
		sla = slaFilter{mode: "eq", value: "OK"}
		status = statusFilter{mode: "notIn", values: csStatuses}
	case "cs":
		// Narrow to a single CS status when one is requested, otherwise show
		// all CS statuses. The sla filter is cleared so NO_SLA issues
		// currently on the CS side are still included.
		if q.Status != "" && slices.Contains(csStatuses, q.Status) {
			status = statusFilter{mode: "eq", value: q.Status}
		} else {
			status = statusFilter{mode: "in", values: csStatuses}
		}
		sla = slaFilter{mode: "none"}
	case "product_side":
		// Mirrors overview.go's hero.productSide count: base open/non-terminal
		// scope (sla stays "notTerminal"), narrowed to statuses currently
		// categorized PRODUCT_SIDE. As with "cs" above, narrow to a single
		// status when one is requested, otherwise show all PRODUCT_SIDE
		// statuses.
		if q.Status != "" && slices.Contains(productSideStatuses, q.Status) {
			status = statusFilter{mode: "eq", value: q.Status}
		} else {
			status = statusFilter{mode: "in", values: productSideStatuses}
		}
	case "tracked":
		priority = priorityFilter{mode: "notNull"}
	case "untracked":
		priority = priorityFilter{mode: "isNull"}
		sla = slaFilter{mode: "none"}
	case "attention":
		sla = slaFilter{mode: "none"}
		attention = true
	default: // "all" or unset — keep base scope; honor explicit params.
		if q.SlaState != "" {
			sla = slaFilter{mode: "eq", value: q.SlaState}
		}
		if q.State != "" {
			state = q.State
		}
	}

	args := &sqlArgs{}
	conditions := []string{"r.enabled = true", "i.state = " + args.add(state)}

	switch sla.mode {
	case "notTerminal":
		conditions = append(conditions, "s.sla_state IS DISTINCT FROM "+args.add("TERMINAL"))
	case "eq":
		conditions = append(conditions, "s.sla_state = "+args.add(sla.value))
	}

	switch priority.mode {
	case "eq":
		conditions = append(conditions, "i.priority = "+args.add(priority.value))
	case "notNull":
		conditions = append(conditions, "i.priority IS NOT NULL")
	case "isNull":
		conditions = append(conditions, "i.priority IS NULL")
	}

	switch status.mode {
	case "eq":
		conditions = append(conditions, "i.current_status = "+args.add(status.value))
	case "in":
		conditions = append(conditions, "i.current_status = ANY("+args.add(status.values)+")")
	case "notIn":
		conditions = append(conditions, "i.current_status <> ALL("+args.add(status.values)+")")
	}

	if attention {
		conditions = append(conditions, fmt.Sprintf(
			"(s.sla_state = ANY(%s) OR i.current_status = ANY(%s))",
			args.add([]string{"VIOLATED", "AT_RISK"}), args.add(csStatuses),
		))
	}

	if q.Repo != "" {
		owner, name, _ := strings.Cut(q.Repo, "/") // format guaranteed by parseIssuesQuery
		conditions = append(conditions, fmt.Sprintf("r.owner = %s AND r.name = %s", args.add(owner), args.add(name)))
	}
	if q.Q != "" {
		n, _ := strconv.Atoi(q.Q) // format guaranteed by parseIssuesQuery
		conditions = append(conditions, "i.github_number = "+args.add(n))
	}

	return strings.Join(conditions, " AND "), args.values
}
