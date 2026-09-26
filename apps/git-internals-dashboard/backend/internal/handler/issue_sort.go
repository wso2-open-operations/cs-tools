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
	"slices"
)

// issueSortField is GET /issues's `sort` query value. Matches the webapp's
// IssueSortField type (src/api/issueSort.ts) one-for-one.
type issueSortField string

// issueSortOrder is GET /issues's `order` query value.
type issueSortOrder string

const (
	orderAsc  issueSortOrder = "asc"
	orderDesc issueSortOrder = "desc"
)

// defaultIssueSort and defaultIssueSortOrder apply when `sort`/`order` are
// absent: highest SLA consumption first.
const (
	defaultIssueSort      issueSortField = "sla_consumption"
	defaultIssueSortOrder issueSortOrder = orderDesc
)

// issueSortColumns is the whitelist of accepted `sort` values, mapped to the
// column each one orders by. This is the only place a new sortable field
// needs to be wired in on the backend — pair it with a new enum value in
// openapi.yaml and the webapp's IssueSortField type. The direction keyword
// itself never comes from raw input: ListIssues appends it from the
// validated issueSortOrder enum.
var issueSortColumns = map[issueSortField]string{
	defaultIssueSort: "s.pct_consumed", // uses the issue_sla.pct_consumed index
	"created":        "i.github_created_at",
	"updated":        "i.github_updated_at",
}

// validIssueSortValues returns every accepted `sort` value, sorted for a
// deterministic validation-error message.
func validIssueSortValues() []string {
	values := make([]string, 0, len(issueSortColumns))
	for k := range issueSortColumns {
		values = append(values, string(k))
	}
	slices.Sort(values)
	return values
}

// issueOrderBy renders field/order into an ORDER BY clause body. NULLS LAST
// applies in both directions so rows without a value for the sorted column
// (e.g. an issue with no issue_sla row, sorted by SLA consumption) never
// crowd the top of an ascending sort. The i.id ASC tie-breaker is
// deterministic: without it, rows sharing an equal or NULL sort value could
// repeat or vanish across LIMIT/OFFSET pages. field and order are only ever
// values already validated against issueSortColumns and {asc, desc} by
// parseIssuesQuery, never raw request input.
func issueOrderBy(field issueSortField, order issueSortOrder) string {
	direction := "DESC"
	if order == orderAsc {
		direction = "ASC"
	}
	return fmt.Sprintf("%s %s NULLS LAST, i.id ASC", issueSortColumns[field], direction)
}
