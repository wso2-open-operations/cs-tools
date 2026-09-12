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
import type { BeCaseEscalationSearchResponse } from "@api/backend/types";
import type { CaseEscalationHistory } from "@features/csm-cases/types/csmCases";

const EMPTY_HISTORY: CaseEscalationHistory = {
  escalations: [],
  currentNotifiedUsers: [],
};

/**
 * A case's full escalation history, newest first, plus who's authorized to
 * de-escalate its current level. Calls `GET /cases/{id}/escalations`
 * (ServiceNow data source only — the backend returns an empty history rather
 * than an error for a non-ServiceNow case).
 */
export function useGetCsmCaseEscalations(
  caseId: string | undefined,
): UseQueryResult<CaseEscalationHistory, Error> {
  const api = useBackendApi();

  return useQuery<CaseEscalationHistory, Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_ESCALATIONS, caseId ?? ""],
    queryFn: async (): Promise<CaseEscalationHistory> => {
      if (!caseId) return EMPTY_HISTORY;

      const response = await api.get<BeCaseEscalationSearchResponse>(
        `/cases/${encodeURIComponent(caseId)}/escalations`,
      );
      return {
        escalations: (response?.escalations ?? []).map((e) => ({
          id: e.id,
          currentLevel: e.currentLevel.id,
          previousLevel: e.previousLevel.id,
          createdBy: e.createdBy,
          createdOn: e.createdOn,
          reason: e.reason,
        })),
        currentNotifiedUsers: (response?.currentNotifiedUsers ?? []).map((u) => ({
          id: u.id,
          name: u.name,
          email: u.email,
        })),
      };
    },
    enabled: !!caseId,
    staleTime: 30_000,
  });
}
