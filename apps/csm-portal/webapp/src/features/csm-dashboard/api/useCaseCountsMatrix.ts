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
import type {
  CaseState,
  Severity,
} from "@features/csm-dashboard/types/abtDashboard";

export const MATRIX_SEVERITIES: Severity[] = ["S0", "S1", "S2", "S3", "S4"];
// Closed cases are deliberately excluded: the dashboard matrix tracks active
// work, so the totals reflect open cases only.
export const MATRIX_STATES: CaseState[] = [
  "open",
  "work_in_progress",
  "waiting_on_wso2",
  "awaiting_info",
  "solution_proposed",
  "reopened",
];

export interface CaseCountsMatrix {
  /** counts[severity][state] = number of cases. */
  counts: Record<Severity, Record<CaseState, number>>;
  severityTotals: Record<Severity, number>;
  stateTotals: Record<CaseState, number>;
  total: number;
}

function emptyMatrix(): CaseCountsMatrix {
  const counts = {} as Record<Severity, Record<CaseState, number>>;
  const severityTotals = {} as Record<Severity, number>;
  const stateTotals = {} as Record<CaseState, number>;
  for (const s of MATRIX_SEVERITIES) {
    severityTotals[s] = 0;
    counts[s] = {} as Record<CaseState, number>;
    for (const st of MATRIX_STATES) counts[s][st] = 0;
  }
  for (const st of MATRIX_STATES) stateTotals[st] = 0;
  return { counts, severityTotals, stateTotals, total: 0 };
}

/**
 * Case counts broken down by severity × state, for the dashboard matrix.
 *
 * The backend has no aggregation endpoint, so this fans out one count-only
 * `POST /cases/search` per (severity, state) cell: each request filters by the
 * cell's priority + state and asks for `limit: 1`, then reads the `total`
 * attribute off the response (the case rows themselves are discarded). Counts
 * are therefore *exact* — no sampling, no truncation. Row/column/grand totals
 * are summed from the cells. MOCK mode tallies the seeded cases client-side.
 *
 * Trade-off: this is `MATRIX_SEVERITIES.length * MATRIX_STATES.length` requests
 * (fired in parallel) per refresh. They are cheap indexed counts; the proper
 * long-term fix is a single faceted aggregation endpoint on the backend.
 *
 * Keyed under its own root (not `CSM_CASES`) so case create/patch mutations,
 * which invalidate the `CSM_CASES` prefix, do not re-trigger this fan-out.
 * `staleTime` plus refetch-on-mount keeps it fresh enough on the dashboard.
 */
export function useCaseCountsMatrix(): UseQueryResult<CaseCountsMatrix, Error> {
  const api = useBackendApi();

  return useQuery<CaseCountsMatrix, Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_COUNTS],
    queryFn: async (): Promise<CaseCountsMatrix> => {
      const matrix = emptyMatrix();

      if (isMockMode()) {
        for (const c of getMockCsmCases("all_customers")) {
          if (matrix.counts[c.severity]?.[c.state] === undefined) continue;
          matrix.counts[c.severity][c.state] += 1;
          matrix.severityTotals[c.severity] += 1;
          matrix.stateTotals[c.state] += 1;
          matrix.total += 1;
        }
        return matrix;
      }

      const cells = MATRIX_SEVERITIES.flatMap((severity) =>
        MATRIX_STATES.map((state) => ({ severity, state })),
      );
      const counted = await Promise.all(
        cells.map(({ severity, state }) =>
          api
            .post<BeCaseSearchPayload, BeCaseSearchResponse>("/cases/search", {
              pagination: { offset: 0, limit: 1 },
              priorityKeys: [priorityFromSeverity(severity)],
              stateKeys: [beStateFromUi(state)],
            })
            .then((res) => ({ severity, state, count: res.total ?? 0 })),
        ),
      );

      for (const { severity, state, count } of counted) {
        matrix.counts[severity][state] = count;
        matrix.severityTotals[severity] += count;
        matrix.stateTotals[state] += count;
        matrix.total += count;
      }
      return matrix;
    },
    staleTime: 60_000,
  });
}
