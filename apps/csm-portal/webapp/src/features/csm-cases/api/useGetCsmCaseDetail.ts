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
import { useAuthApiClient } from "@hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import { getMockCsmCaseDetailById } from "@features/csm-cases/api/mocks/casesMocks";
import type { CsmCaseDetail } from "@features/csm-cases/types/csmCases";

const MOCK_LATENCY_MS = 150;

/**
 * Look up a single CSM case by id (or case number).
 *
 * Returns `null` (not an error) when the id is unknown — the page renders
 * a not-found state for that case.
 */
export function useGetCsmCaseDetail(
  caseId: string | undefined,
): UseQueryResult<CsmCaseDetail | null, Error> {
  const logger = useLogger();
  const authFetch = useAuthApiClient();

  return useQuery<CsmCaseDetail | null, Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_DETAIL, caseId ?? ""],
    queryFn: async (): Promise<CsmCaseDetail | null> => {
      if (!caseId) return null;

      if (window.config?.CSM_PORTAL_USE_MOCKS) {
        logger.debug(`[useGetCsmCaseDetail] Returning mock case ${caseId}`);
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockCsmCaseDetailById(caseId) ?? null;
      }

      const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const url = `${baseUrl}/csm/cases/${encodeURIComponent(caseId)}`;
      const response = await authFetch(url, { method: "GET" });
      if (response.status === 404) return null;
      if (!response.ok) {
        throw new Error(
          `Error fetching CSM case ${caseId}: ${response.statusText}`,
        );
      }
      return (await response.json()) as CsmCaseDetail;
    },
    enabled: !!caseId,
    staleTime: 30_000,
  });
}
