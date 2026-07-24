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
import { severityFromPriority, uiStateFromBe } from "@api/backend/mappers";
import type { BeCaseSearchPayload, BeCaseSearchResponse } from "@api/backend/types";
import type { CaseState, Severity } from "@features/csm-dashboard/types/abtDashboard";

const CHILD_CASES_LIMIT = 20;

export interface ChildCaseRow {
  id: string;
  caseNumber?: string;
  subject: string;
  severity: Severity;
  state: CaseState;
  assigneeName?: string;
}

export interface ChildCasesResult {
  cases: ChildCaseRow[];
  total: number;
}

/**
 * Child cases of a case — i.e. cases whose `parentId` (the hierarchical
 * major-case/child-case relationship) points at this one. Calls
 * `POST /cases/search` filtered by `parentId`, reusing the existing
 * cross-project search endpoint rather than a dedicated one. Disabled until a
 * parent case id is provided.
 */
export function useSearchChildCases(
  parentCaseId: string | undefined,
): UseQueryResult<ChildCasesResult, Error> {
  const api = useBackendApi();

  return useQuery<ChildCasesResult, Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_CHILDREN, parentCaseId ?? ""],
    queryFn: async (): Promise<ChildCasesResult> => {
      const res = await api.post<BeCaseSearchPayload, BeCaseSearchResponse>(
        "/cases/search",
        {
          pagination: { offset: 0, limit: CHILD_CASES_LIMIT },
          filters: { parentId: parentCaseId },
        },
      );
      return {
        cases: (res.cases ?? []).map((c) => ({
          id: c.id,
          caseNumber: c.number,
          subject: c.subject ?? "(no subject)",
          severity: severityFromPriority(c.severity),
          state: uiStateFromBe(c.state),
          assigneeName: c.assignedEngineer?.name ?? undefined,
        })),
        total: res.total ?? 0,
      };
    },
    enabled: !!parentCaseId,
    staleTime: 30_000,
  });
}
