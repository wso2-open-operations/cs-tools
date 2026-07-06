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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import { beStateFromUi, priorityFromSeverity } from "@api/backend/mappers";
import type {
  BeCaseSearchPayload,
  BeCaseSearchResponse,
} from "@api/backend/types";
import {
  MATRIX_SEVERITIES,
  MATRIX_STATES,
} from "@features/csm-dashboard/api/useCaseCountsMatrix";
import type {
  CaseState,
  Severity,
} from "@features/csm-dashboard/types/abtDashboard";

/**
 * The states the composition pies break down — the SAME active set the
 * severity×state matrix tracks, so the pie totals reconcile with the table
 * above them. `closed` is intentionally excluded from both pies and surfaced
 * separately as {@link CaseComposition.closedTotal}, so a large backlog of
 * closed cases can't make the active counts disagree with the matrix.
 */
export const COMPOSITION_STATES: CaseState[] = MATRIX_STATES;

export interface CaseComposition {
  /** Active-case count per severity (excludes closed). */
  bySeverity: Record<Severity, number>;
  /** Active-case count per state (excludes closed). */
  byState: Record<CaseState, number>;
  severityTotal: number;
  stateTotal: number;
  /** Closed cases — excluded from the pies, shown as a separate figure. */
  closedTotal: number;
}

/**
 * Case composition for the dashboard pies: a 1-D breakdown by severity and a
 * 1-D breakdown by state, each over the *active* cases only (closed excluded,
 * matching the severity×state matrix). The closed count is returned separately.
 *
 * The backend has no aggregation endpoint, so — like the severity×state matrix
 * — this fans out count-only searches (`limit: 1`, read `total`): one per
 * severity (filtered by priority AND the active states) and one per active
 * state (filtered by state), plus one for the closed total.
 *
 * Severity and state each partition the same active population, so both totals
 * are equal and reconcile with the matrix's grand total.
 */
export function useCaseComposition(): UseQueryResult<CaseComposition, Error> {
  const api = useBackendApi();

  return useQuery<CaseComposition, Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_COUNTS, "composition"],
    queryFn: async (): Promise<CaseComposition> => {
      const bySeverity = {} as Record<Severity, number>;
      const byState = {} as Record<CaseState, number>;
      MATRIX_SEVERITIES.forEach((s) => (bySeverity[s] = 0));
      COMPOSITION_STATES.forEach((s) => (byState[s] = 0));
      let closedTotal = 0;

      const countTotal = (payload: BeCaseSearchPayload): Promise<number> =>
        api
          .post<BeCaseSearchPayload, BeCaseSearchResponse>(
            "/cases/search",
            payload,
          )
          .then((r) => r.total ?? 0);

      // The active states, expressed once for the severity counts so each is
      // scoped to active cases (priority AND active-state), not the whole
      // population.
      const activeStateKeys = COMPOSITION_STATES.map(beStateFromUi);
      // The state/closed counts don't filter by severity. The backend,
      // however, counts only catastrophic cases when `severityKeys` is absent
      // (an empty priority filter is not treated as "all"), which made the
      // state pie show the S0 row only and disagree with the matrix totals.
      // Pass every priority explicitly so these counts span all severities.
      // BE follow-up: treat an absent/empty severityKeys as "all priorities".
      const allPriorityKeys = MATRIX_SEVERITIES.map(priorityFromSeverity);

      // All severity + state counts (plus the closed total) fire in one wave.
      const [severityCounts, stateCounts, closedCount] = await Promise.all([
        Promise.all(
          MATRIX_SEVERITIES.map((sev) =>
            countTotal({
              pagination: { offset: 0, limit: 1 },
              filters: {
                severities: [priorityFromSeverity(sev)],
                states: activeStateKeys,
              },
            }).then((n) => ({ key: sev, n })),
          ),
        ),
        Promise.all(
          COMPOSITION_STATES.map((st) =>
            countTotal({
              pagination: { offset: 0, limit: 1 },
              filters: {
                states: [beStateFromUi(st)],
                severities: allPriorityKeys,
              },
            }).then((n) => ({ key: st, n })),
          ),
        ),
        countTotal({
          pagination: { offset: 0, limit: 1 },
          filters: {
            states: [beStateFromUi("closed")],
            severities: allPriorityKeys,
          },
        }),
      ]);
      severityCounts.forEach(({ key, n }) => (bySeverity[key] = n));
      stateCounts.forEach(({ key, n }) => (byState[key] = n));
      closedTotal = closedCount;

      const severityTotal = MATRIX_SEVERITIES.reduce(
        (a, s) => a + bySeverity[s],
        0,
      );
      const stateTotal = COMPOSITION_STATES.reduce(
        (a, s) => a + byState[s],
        0,
      );
      return { bySeverity, byState, severityTotal, stateTotal, closedTotal };
    },
    staleTime: 60_000,
  });
}
