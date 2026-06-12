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
import { severityFromPriority, uiStateFromBe } from "@api/backend/mappers";
import type {
  BeCaseSearchPayload,
  BeCaseSearchResponse,
} from "@api/backend/types";
import { getMockCsmCases } from "@features/csm-cases/api/mocks/casesMocks";
import type {
  CaseState,
  Severity,
} from "@features/csm-dashboard/types/abtDashboard";

const PAGE_LIMIT = 100; // backend caps pagination limit at 100
const MAX_PAGES = 5; // bound the fan-out (≤500 cases sampled for the matrix)

export const MATRIX_SEVERITIES: Severity[] = ["S0", "S1", "S2", "S3", "S4"];
export const MATRIX_STATES: CaseState[] = [
  "open",
  "work_in_progress",
  "waiting_on_wso2",
  "awaiting_info",
  "solution_proposed",
  "reopened",
  "closed",
];

export interface CaseCountsMatrix {
  /** counts[severity][state] = number of cases. */
  counts: Record<Severity, Record<CaseState, number>>;
  severityTotals: Record<Severity, number>;
  stateTotals: Record<CaseState, number>;
  total: number;
  /** True when more cases exist than were sampled (page cap hit). */
  truncated: boolean;
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
  return { counts, severityTotals, stateTotals, total: 0, truncated: false };
}

/**
 * Case counts broken down by severity × state, for the dashboard matrix.
 *
 * The backend has no aggregation endpoint, so this samples cases via
 * `POST /cases/search` (up to {@link MAX_PAGES} pages of {@link PAGE_LIMIT}) and
 * tallies client-side. `truncated` flags when the real total exceeds the
 * sample. MOCK mode tallies the seeded cases.
 *
 * Keyed under its own root (not `CSM_CASES`) so case create/patch mutations,
 * which invalidate the `CSM_CASES` prefix, do not re-trigger this up-to-500-row
 * fan-out. The matrix is a sampled approximation; `staleTime` plus
 * refetch-on-mount keeps it fresh enough on the dashboard.
 */
export function useCaseCountsMatrix(): UseQueryResult<CaseCountsMatrix, Error> {
  const api = useBackendApi();

  return useQuery<CaseCountsMatrix, Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_COUNTS],
    queryFn: async (): Promise<CaseCountsMatrix> => {
      const matrix = emptyMatrix();

      const tally = (severity: Severity, state: CaseState): void => {
        if (!matrix.counts[severity] || matrix.counts[severity][state] === undefined) {
          return;
        }
        matrix.counts[severity][state] += 1;
        matrix.severityTotals[severity] += 1;
        matrix.stateTotals[state] += 1;
        matrix.total += 1;
      };

      if (isMockMode()) {
        for (const c of getMockCsmCases("all_customers").cases) {
          tally(c.severity, c.state);
        }
        return matrix;
      }

      let offset = 0;
      for (let page = 0; page < MAX_PAGES; page += 1) {
        const res = await api.post<BeCaseSearchPayload, BeCaseSearchResponse>(
          "/cases/search",
          { pagination: { offset, limit: PAGE_LIMIT } },
        );
        for (const c of res.cases ?? []) {
          tally(severityFromPriority(c.priority), uiStateFromBe(c.state));
        }
        if (!res.hasMore || (res.cases ?? []).length === 0) break;
        offset += PAGE_LIMIT;
        if (page === MAX_PAGES - 1 && res.hasMore) matrix.truncated = true;
      }
      return matrix;
    },
    staleTime: 60_000,
  });
}
