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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// goingToExceedFloorMinutes is the threshold that decides the report's second
// table: a group with less than ten hours of entitlement left is "going to
// exceed".
//
// *** THIS IS AN ABSOLUTE FLOOR, NOT A PERCENTAGE. *** It is unrelated to the
// 75/90/100 percent thresholds the per-project threshold notifier uses
// (QueryHourStateFor), and the two must not be unified. ServiceNow applies
// both rules in the same function to the same data and they disagree
// constantly — a 100h entitlement with 95h consumed is 95% used yet still has
// 300 minutes left, so it lands here too; a 1h entitlement with nothing
// consumed is 0% used yet has only 60 minutes, so it also lands here.
//
// The consequence worth knowing: an opportunity with NO entitlement at all
// has zero remaining, and zero is less than 600, so it is reported as going
// to exceed. That is ServiceNow's behaviour and it is reproduced rather than
// corrected, because "no query hours purchased" and "query hours nearly
// exhausted" genuinely are the same thing from the report's point of view —
// neither can absorb another billable hour.
const goingToExceedFloorMinutes = 600

// WeeklyReport assembles the weekly query-hour consumption report.
//
// This is the read half of ServiceNow's `[WSO2][Query Hours] Weekly Report`.
// The write half is deliberately NOT ported: that flow also stamps
// u_sf_opportunity.u_query_hour_state on every ungrouped opportunity it
// renders, and caches the rendered HTML onto customer_account.u_query_details.
// Reproducing those writes would put two systems on the same columns, which
// the double-fire rule forbids, and nothing has yet established what still
// reads the opportunity-level state. This function only reads.
//
// An empty database, or one without the mirrored Salesforce tables, yields a
// report with zero accounts rather than an error — the caller emails an empty
// report, which is a true statement about the estate.
func (s *queryHourService) WeeklyReport(ctx context.Context) (domain.QueryHoursWeeklyReport, error) {
	// Internal callers only, the same gate the sweep uses and for a stronger
	// reason. This is the whole estate in one response: every account's
	// entitlement and consumption, plus the account manager and technical
	// owner behind each one. There is no per-project scope to filter it down
	// to — a customer-scoped caller has no correct subset of this report, so
	// the answer is refusal rather than a filtered view.
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.QueryHoursWeeklyReport{}, err
	}

	rows, err := s.repo.WeeklyReportRows(ctx)
	if err != nil {
		return domain.QueryHoursWeeklyReport{}, err
	}

	report := domain.QueryHoursWeeklyReport{
		// The stamp is the UTC date the report was built. ServiceNow's stamp
		// is also a UTC date, which is why its Monday-morning mail carries
		// Sunday's — its trigger fires 00:00 Asia/Colombo, still the previous
		// day in UTC. Here the schedule is chosen to preserve that instant
		// (see the query_hours_weekly_report sub-cron), so the stamp agrees
		// with the original for the same reason rather than by coincidence.
		GeneratedOn:   time.Now().UTC().Format("2006-01-02"),
		Exceeded:      []domain.QueryHoursReportAccount{},
		GoingToExceed: []domain.QueryHoursReportAccount{},
	}

	for _, accountRows := range groupRowsByAccount(rows) {
		account := buildAccount(accountRows)
		report.UnmatchedLineCount += unmatchedLines(accountRows)
		if account.Exceeded {
			report.Exceeded = append(report.Exceeded, account)
		}
		if account.GoingToExceed {
			report.GoingToExceed = append(report.GoingToExceed, account)
		}
	}

	report.ExceededCount = len(report.Exceeded)
	report.GoingToExceedCount = len(report.GoingToExceed)
	return report, nil
}

// groupRowsByAccount splits the flat row set into per-account slices,
// preserving the query's account ordering.
func groupRowsByAccount(rows []domain.QueryHoursReportRow) [][]domain.QueryHoursReportRow {
	var out [][]domain.QueryHoursReportRow
	var current []domain.QueryHoursReportRow
	for i, row := range rows {
		if i > 0 && row.AccountID != rows[i-1].AccountID {
			out = append(out, current)
			current = nil
		}
		current = append(current, row)
	}
	if len(current) > 0 {
		out = append(out, current)
	}
	return out
}

// unmatchedLines totals the account's unmatched product lines, counting each
// opportunity once — the count travels on every row sharing an opportunity.
func unmatchedLines(rows []domain.QueryHoursReportRow) int {
	seen := map[string]bool{}
	total := 0
	for _, row := range rows {
		if seen[row.OpportunityID] {
			continue
		}
		seen[row.OpportunityID] = true
		total += row.UnmatchedLineCount
	}
	return total
}

// buildAccount turns one account's rows into its report section.
func buildAccount(rows []domain.QueryHoursReportRow) domain.QueryHoursReportAccount {
	account := domain.QueryHoursReportAccount{
		AccountID:           rows[0].AccountID,
		Name:                rows[0].AccountName,
		SFID:                rows[0].AccountSFID,
		AccountManagerEmail: rows[0].AccountManagerEmail,
		TechnicalOwnerEmail: rows[0].TechnicalOwnerEmail,
	}

	for _, component := range componentsOf(rows) {
		group := buildGroup(component)
		account.Groups = append(account.Groups, group)
		account.RowCount += group.RowCount
		account.Exceeded = account.Exceeded || group.Exceeded
		account.GoingToExceed = account.GoingToExceed || group.GoingToExceed
	}
	return account
}

// componentsOf partitions one account's rows into connected components of the
// opportunity-to-project funding graph.
//
// Two opportunities belong to the same component when they fund a project in
// common, transitively. This is union-find over a graph small enough that the
// structure costs nothing: an account has a handful of live opportunities.
//
// Components are returned in first-appearance order so the report's row order
// stays stable between runs — the SQL orders by account, opportunity name and
// project name, and this preserves that rather than imposing a map's ordering.
func componentsOf(rows []domain.QueryHoursReportRow) [][]domain.QueryHoursReportRow {
	parent := map[string]string{}

	var find func(string) string
	find = func(x string) string {
		if parent[x] == "" {
			parent[x] = x
		}
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}

	// Opportunity and project ids are both UUIDs from different tables, so
	// they are namespaced before sharing one disjoint-set forest.
	for _, row := range rows {
		union("o:"+row.OpportunityID, "p:"+row.ProjectID)
	}

	order := []string{}
	byRoot := map[string][]domain.QueryHoursReportRow{}
	for _, row := range rows {
		root := find("o:" + row.OpportunityID)
		if _, seen := byRoot[root]; !seen {
			order = append(order, root)
		}
		byRoot[root] = append(byRoot[root], row)
	}

	out := make([][]domain.QueryHoursReportRow, 0, len(order))
	for _, root := range order {
		out = append(out, byRoot[root])
	}
	return out
}

// buildGroup collapses one connected component into a report group.
//
// Entitlement counts each opportunity once; consumption counts each project
// once. Both matter: the entitlement figure repeats across every row sharing
// an opportunity, and a project funded by two of the component's
// opportunities would otherwise have its consumption counted twice.
//
// The thresholds are applied AFTER the component totals are known. ServiceNow
// computes its flags per opportunity BEFORE merging and never revisits them,
// which is how an account can show a hundred hours remaining and still sit in
// the going-to-exceed table — a pre-merge opportunity with no entitlement had
// zero remaining, and the stale flag survived the merge. Applying the rule to
// the merged totals is the fix.
func buildGroup(rows []domain.QueryHoursReportRow) domain.QueryHoursReportGroup {
	var group domain.QueryHoursReportGroup

	countedOpportunity := map[string]bool{}
	countedProject := map[string]bool{}
	opportunityIndex := map[string]int{}

	for _, row := range rows {
		idx, known := opportunityIndex[row.OpportunityID]
		if !known {
			idx = len(group.Opportunities)
			opportunityIndex[row.OpportunityID] = idx
			group.Opportunities = append(group.Opportunities, domain.QueryHoursReportOpportunity{
				OpportunityID:      row.OpportunityID,
				Name:               row.OpportunityName,
				SFID:               row.OpportunitySFID,
				EntitlementMinutes: row.EntitlementMinutes,
			})
		}
		if !countedOpportunity[row.OpportunityID] {
			countedOpportunity[row.OpportunityID] = true
			group.EntitlementMinutes += row.EntitlementMinutes
		}

		duplicate := countedProject[row.ProjectID]
		if !duplicate {
			countedProject[row.ProjectID] = true
			group.ConsumedMinutes += row.ConsumedMinutes
		}

		group.Opportunities[idx].Projects = append(group.Opportunities[idx].Projects,
			domain.QueryHoursReportProject{
				ProjectID:       row.ProjectID,
				Name:            row.ProjectName,
				Key:             row.ProjectKey,
				SFID:            row.ProjectSFID,
				ConsumedMinutes: row.ConsumedMinutes,
				Duplicate:       duplicate,
			})
		group.RowCount++
	}

	group.RemainingMinutes = group.EntitlementMinutes - group.ConsumedMinutes
	switch {
	case group.RemainingMinutes < 0:
		group.Exceeded = true
	case group.RemainingMinutes < goingToExceedFloorMinutes:
		group.GoingToExceed = true
	}
	return group
}
