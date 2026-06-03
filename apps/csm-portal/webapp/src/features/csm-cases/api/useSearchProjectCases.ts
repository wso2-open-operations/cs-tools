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
import type {
  BeCaseSearchResponse,
  BeProjectCaseSearchPayload,
} from "@api/backend/types";

/**
 * Project-scoped case search backed by `POST /projects/{id}/cases/search`.
 * The case ID being scoped to a project comes from the URL path, so the
 * payload's `projectIds` is intentionally ignored by the BE.
 *
 * Returns the raw backend response. Callers map to UI shapes as needed.
 *
 * Used by:
 *   - Project detail page → cases tab
 *   - Cross-project fan-out aggregation in `useGetCsmCases` (LIVE mode)
 */
export function useSearchProjectCases(
  projectId: string | undefined,
  payload: BeProjectCaseSearchPayload = {},
  options: { enabled?: boolean } = {},
): UseQueryResult<BeCaseSearchResponse, Error> {
  const api = useBackendApi();

  return useQuery<BeCaseSearchResponse, Error>({
    queryKey: [
      ApiQueryKeys.BACKEND_PROJECT_CASES_SEARCH,
      projectId ?? "",
      payload,
    ],
    queryFn: async () => {
      if (!projectId) {
        return { cases: [], total: 0, limit: 0, offset: 0, hasMore: false };
      }
      if (isMockMode()) {
        // In mock mode, callers should use `useGetCsmCases` which has a richer
        // cross-project seed dataset. This hook is the BE-direct path; returning
        // an empty page keeps it inert when mocks are on.
        return { cases: [], total: 0, limit: 0, offset: 0, hasMore: false };
      }
      return api.post<BeProjectCaseSearchPayload, BeCaseSearchResponse>(
        `/projects/${encodeURIComponent(projectId)}/cases/search`,
        payload,
      );
    },
    enabled: options.enabled ?? !!projectId,
    staleTime: 15_000,
  });
}
