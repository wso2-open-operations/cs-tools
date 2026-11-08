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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package repository

import (
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Shared, work-item-agnostic SQL predicates for the two SLA search filters
// (slaBreached, taskSLABusinessElapsedPercent). The sla table links to any
// work item through work_item_id, so a work-item type registers by passing
// its own work-item id expression (e.g. "wi.id"); nothing here is
// case-specific. Both read sla_live, whose live_* columns are current at
// query time.

// slaBreachedPredicate returns an " AND [NOT] EXISTS (...)" fragment
// restricting to work items with (breached) or without (!breached) a
// currently-breached SLA record.
//
// Restricted to bs.is_active IS TRUE AND bs.stage = 'IN_PROGRESS' -- a real,
// reported defect: without this, a long-finished (COMPLETED/ACHIEVED/
// CANCELLED) clock whose frozen has_breached happened to be true kept
// matching forever, and a PAUSED clock's own live_has_breached (frozen
// correctly at pause time, same reasoning as slaPercentPredicate's own
// comment below) was being live-recomputed here for no reason. bs.stage is
// the row's RAW, pre-live-correction stage -- not bs.live_stage, which
// migration 0279's own view change can bump from IN_PROGRESS to BREACHED at
// read time; restricting to the raw IN_PROGRESS value (matching
// ServiceNow's own "violated" search, which has no stage condition of its
// own beyond active=true but is checking its own always-IN_PROGRESS live
// recompute) is what actually asks live_has_breached's live correction the
// question it exists to answer: has an otherwise-still-open clock's real
// elapsed time now exceeded its duration.
func slaBreachedPredicate(workItemID string, breached bool) string {
	not := ""
	if !breached {
		not = "NOT "
	}
	return fmt.Sprintf(" AND %sEXISTS (SELECT 1 FROM sla_live bs WHERE bs.work_item_id = %s AND bs.is_active IS TRUE AND bs.stage = 'IN_PROGRESS' AND bs.live_has_breached IS TRUE)", not, workItemID)
}

// slaPercentPredicate returns an " AND EXISTS (...)" fragment (and appends its
// bind values to args) matching a work item with at least one SLA record whose
// live business-elapsed percentage is within the bound(s). Both bounds apply
// to the SAME row (a range like 75..100 means one SLA in that range, not one
// row >= 75 and another <= 100). The percentage is uncapped (long-overdue
// SLAs climb far past 100) and 0 is a real bound, hence the nil checks.
// Returns the fragment, the extended args, and the next free placeholder
// index; an empty fragment when f carries no bound.
//
// Restricted to tsla.is_active IS TRUE -- a real, reported defect (this
// predicate used to count "any SLA row, whatever its stage"): a COMPLETED/
// ACHIEVED/CANCELLED clock's frozen percentage, written once and never
// touched again, kept matching a bound it happened to land in forever,
// which is how a saved "SLA at risk" view accumulated long-finished cases
// that had nothing left to be at risk of. is_active stays TRUE for a
// PAUSED row (only the three terminal stages ever set it FALSE -- see
// sla_engine_repo.go's own slaEngineActiveStageFilter), so a paused clock
// still matches, same as ServiceNow's own equivalent filter (it has no
// stage condition beyond active=true) -- its own frozen percentage is
// already correct, written at the moment it was paused, so there is
// nothing live to recompute for it either way.
func slaPercentPredicate(workItemID string, f *domain.TaskSLAFilter, args []any, argIdx int) (string, []any, int) {
	if f == nil || (f.MinBusinessElapsedPercent == nil && f.MaxBusinessElapsedPercent == nil) {
		return "", args, argIdx
	}
	slaWhere := "tsla.work_item_id = " + workItemID + " AND tsla.is_active IS TRUE"
	if f.MinBusinessElapsedPercent != nil {
		slaWhere += fmt.Sprintf(" AND tsla.live_elapsed_percentage >= $%d::numeric", argIdx)
		args = append(args, *f.MinBusinessElapsedPercent)
		argIdx++
	}
	if f.MaxBusinessElapsedPercent != nil {
		slaWhere += fmt.Sprintf(" AND tsla.live_elapsed_percentage <= $%d::numeric", argIdx)
		args = append(args, *f.MaxBusinessElapsedPercent)
		argIdx++
	}
	return " AND EXISTS (SELECT 1 FROM sla_live tsla WHERE " + slaWhere + ")", args, argIdx
}
