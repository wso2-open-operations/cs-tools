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

import { queryOptions } from "@tanstack/react-query";
import type { CaseSearchFiltersDto, CaseSeverity, CaseState } from "@src/types";
import { ALL_SEVERITIES } from "@components/support/config";
import { ASSIGNED_TO_ME_STATES, COMPOSITION_STATES } from "@components/home/config";
import { getAllCases, type CaseSearchResult } from "./cases";

// Mirrors the webapp's CaseComposition (useCaseComposition.ts): a 1-D breakdown by severity and a
// 1-D breakdown by state, each over active cases only (closed excluded). The closed count is
// returned separately so a large closed backlog can't skew the active totals.
export interface CaseComposition {
  bySeverity: Record<CaseSeverity, number>;
  byState: Record<CaseState, number>;
  severityTotal: number;
  stateTotal: number;
  closedTotal: number;
}

const EMPTY_RESULT: CaseSearchResult = { items: [], total: 0, limit: 0, offset: 0, hasMore: false };

// The backend has no count/aggregation endpoint, so — like the webapp — this fans out
// count-only searches (`limit: 1`, read `.total`): one per severity (scoped to active states) and
// one per active state (scoped to every severity), plus one for the closed total. 11 requests,
// fired in parallel.
async function fetchComposition(): Promise<CaseComposition> {
  const countOf = (filters: CaseSearchFiltersDto): Promise<number> =>
    getAllCases({ filters: { types: ["case"], ...filters }, pagination: { limit: 1 } }).then((r) => r.total);

  const [severityCounts, stateCounts, closedTotal] = await Promise.all([
    Promise.all(
      ALL_SEVERITIES.map((severity) =>
        countOf({ severities: [severity], states: COMPOSITION_STATES }).then((n) => [severity, n] as const),
      ),
    ),
    Promise.all(
      COMPOSITION_STATES.map((state) =>
        countOf({ states: [state], severities: ALL_SEVERITIES }).then((n) => [state, n] as const),
      ),
    ),
    countOf({ states: ["closed"], severities: ALL_SEVERITIES }),
  ]);

  const bySeverity = Object.fromEntries(severityCounts) as Record<CaseSeverity, number>;
  const byState = Object.fromEntries(stateCounts) as Record<CaseState, number>;
  const severityTotal = severityCounts.reduce((sum, [, n]) => sum + n, 0);
  const stateTotal = stateCounts.reduce((sum, [, n]) => sum + n, 0);

  return { bySeverity, byState, severityTotal, stateTotal, closedTotal };
}

export const dashboard = {
  // Same active-vs-closed split the webapp's dashboard donuts show; staleTime keeps a page
  // revisit from re-firing all 11 requests immediately.
  composition: () =>
    queryOptions({
      queryKey: ["dashboard", "composition"],
      queryFn: fetchComposition,
      staleTime: 60_000,
    }),

  // The signed-in user's own non-closed cases, newest-updated first — mirrors the webapp's
  // MyAssignedCases widget, capped to a short preview (5) rather than paginated.
  assignedToMe: (userId: string | null) =>
    queryOptions({
      queryKey: ["dashboard", "assigned-to-me", userId],
      queryFn: () =>
        userId
          ? getAllCases({
              filters: { types: ["case"], assignedUserIds: [userId], states: ASSIGNED_TO_ME_STATES },
              sortBy: { field: "updatedOn", order: "desc" },
              pagination: { limit: 5 },
            })
          : Promise.resolve(EMPTY_RESULT),
      enabled: !!userId,
    }),
};
