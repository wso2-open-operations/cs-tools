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
import type {
  ProductIssueListResponse,
  ProductIssueSummary,
} from "@features/csm-cases/types/productWorkItems";

/**
 * Product issues (GitHub issues raised from the case) linked to a case, via
 * `GET /cases/{id}/product-work-items`. The route only exists when the
 * backend's `GIT_INTERNALS_ENABLED` is on; with it off the BFF answers 404,
 * which the client turns into `null`. `null` therefore means "feature off —
 * render nothing", distinct from `[]` (feature on, nothing linked).
 */
export function useCaseProductWorkItems(
  caseId: string | undefined,
): UseQueryResult<ProductIssueSummary[] | null, Error> {
  const api = useBackendApi();

  return useQuery<ProductIssueSummary[] | null, Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_PRODUCT_WORK_ITEMS, caseId ?? ""],
    queryFn: async (): Promise<ProductIssueSummary[] | null> => {
      const res = await api.get<ProductIssueListResponse>(
        `/cases/${encodeURIComponent(caseId as string)}/product-work-items`,
      );
      if (res === null) return null;
      return res.items ?? [];
    },
    enabled: !!caseId,
    staleTime: 30_000,
    // A 404 is a resolved answer ("feature off"), not an error; real errors
    // keep the default retry behaviour.
  });
}
