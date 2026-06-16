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
import { isMockMode, useBackendApi } from "@api/backend/client";
import { beStateFromUi, priorityFromSeverity } from "@api/backend/mappers";
import type {
  BeCaseSearchPayload,
  BeCaseSearchResponse,
} from "@api/backend/types";
import { getMockCsmCases } from "@features/csm-cases/api/mocks/casesMocks";
import { MATRIX_SEVERITIES } from "@features/csm-dashboard/api/useCaseCountsMatrix";
import type {
  CaseState,
  Severity,
} from "@features/csm-dashboard/types/abtDashboard";

/**
 * Every case state, including `closed` — the composition pies show the whole
 * population, not just the active states the severity/state matrix tracks.
 */
export const COMPOSITION_STATES: CaseState[] = [
  "open",
  "work_in_progress",
  "waiting_on_wso2",
  "awaiting_info",
  "solution_proposed",
  "reopened",
  "closed",
];

export interface CaseComposition {
  /** Case count per severity, across all states. */
  bySeverity: Record<Severity, number>;
  /** Case count per state, across all severities. */
  byState: Record<CaseState, number>;
  severityTotal: number;
  stateTotal: number;
}

/**
 * Case composition for the dashboard pies: a 1-D breakdown by severity and a
 * 1-D breakdown by state, each over *all* cases (closed included).
 *
 * The backend has no aggregation endpoint, so — like the severity×state matrix
 * — this fans out count-only searches (`limit: 1`, read `total`): one per
 * severity (filtered by priority, any state) and one per state (filtered by
 * state, any severity). MOCK mode tallies the seeded cases.
 *
 * Severity and state each partition the same population, so both totals equal
 * the overall case count (give or take cases the backend leaves unprioritised).
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

      if (isMockMode()) {
        for (const c of getMockCsmCases("all_customers")) {
          if (bySeverity[c.severity] !== undefined) bySeverity[c.severity] += 1;
          if (byState[c.state] !== undefined) byState[c.state] += 1;
        }
      } else {
        const countTotal = (payload: BeCaseSearchPayload): Promise<number> =>
          api
            .post<BeCaseSearchPayload, BeCaseSearchResponse>(
              "/cases/search",
              payload,
            )
            .then((r) => r.total ?? 0);

        // All severity + state counts fire in one parallel wave.
        const [severityCounts, stateCounts] = await Promise.all([
          Promise.all(
            MATRIX_SEVERITIES.map((sev) =>
              countTotal({
                pagination: { offset: 0, limit: 1 },
                priorityKeys: [priorityFromSeverity(sev)],
              }).then((n) => ({ key: sev, n })),
            ),
          ),
          Promise.all(
            COMPOSITION_STATES.map((st) =>
              countTotal({
                pagination: { offset: 0, limit: 1 },
                stateKeys: [beStateFromUi(st)],
              }).then((n) => ({ key: st, n })),
            ),
          ),
        ]);
        severityCounts.forEach(({ key, n }) => (bySeverity[key] = n));
        stateCounts.forEach(({ key, n }) => (byState[key] = n));
      }

      const severityTotal = MATRIX_SEVERITIES.reduce(
        (a, s) => a + bySeverity[s],
        0,
      );
      const stateTotal = COMPOSITION_STATES.reduce(
        (a, s) => a + byState[s],
        0,
      );
      return { bySeverity, byState, severityTotal, stateTotal };
    },
    staleTime: 60_000,
  });
}
