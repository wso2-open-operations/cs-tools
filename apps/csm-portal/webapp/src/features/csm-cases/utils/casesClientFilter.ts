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

import { ASSIGNEE_ME_TOKEN } from "@features/csm-cases/components/CasesFilterBar";
import type { CasesFilters } from "@features/csm-cases/components/CasesFilterBar";
import type { CsmCaseRow } from "@features/csm-cases/types/csmCases";

export const SLA_AT_RISK_THRESHOLD_MINUTES = 60;

/**
 * Apply the full filter set to a case list client-side. Used for MOCK mode,
 * where the hook holds the whole seeded dataset and must filter + paginate it
 * itself. In LIVE mode the backend applies the supported filters
 * (search / severity / state / project) server-side, so this is not used.
 */
export function applyCasesFilters(
  cases: CsmCaseRow[],
  f: CasesFilters,
): CsmCaseRow[] {
  const q = f.search.trim().toLowerCase();
  return cases.filter((c) => {
    if (f.severities.length && !f.severities.includes(c.severity)) return false;
    if (f.states.length && !f.states.includes(c.state)) return false;
    // SLA status only applies to active cases; a closed row (often
    // minutesToBreach = 0) must not match "breached" or "at_risk".
    if (f.sla === "breached") {
      if (c.state === "closed" || c.minutesToBreach >= 0) return false;
    }
    if (
      f.sla === "at_risk" &&
      !(
        c.state !== "closed" &&
        c.minutesToBreach >= 0 &&
        c.minutesToBreach <= SLA_AT_RISK_THRESHOLD_MINUTES
      )
    )
      return false;
    if (f.assignees.length) {
      const match = f.assignees.some((a) =>
        a === ASSIGNEE_ME_TOKEN ? c.assigneeIsMe : a === c.assignee,
      );
      if (!match) return false;
    }
    if (f.projects.length && !f.projects.includes(c.projectId)) return false;
    if (f.products.length && !f.products.includes(c.product)) return false;
    if (q) {
      const hay =
        `${c.caseNumber ?? ""} ${c.wso2CaseId ?? ""} ${c.subject} ${c.customer} ${c.projectName} ${c.assignee} ${c.product}`.toLowerCase();
      if (!hay.includes(q)) return false;
    }
    return true;
  });
}

/** Sort by SLA urgency: closed last, then soonest-to-breach first. */
export function sortBySlaUrgency(a: CsmCaseRow, b: CsmCaseRow): number {
  const aClosed = a.state === "closed" ? 1 : 0;
  const bClosed = b.state === "closed" ? 1 : 0;
  if (aClosed !== bClosed) return aClosed - bClosed;
  return a.minutesToBreach - b.minutesToBreach;
}
