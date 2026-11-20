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
import { isAwaitingGithub } from "@features/csm-cases/utils/productWorkItems";
import type { ProductIssueDetail } from "@features/csm-cases/types/productWorkItems";

export const PENDING_POLL_MS = 5000;

/** One product issue with conversation and status history, via
 * `GET /product-work-items/{id}`. `null` when the id is unknown (404). */
export function useProductWorkItem(
  id: string | undefined,
): UseQueryResult<ProductIssueDetail | null, Error> {
  const api = useBackendApi();

  return useQuery<ProductIssueDetail | null, Error>({
    queryKey: [ApiQueryKeys.CSM_PRODUCT_WORK_ITEM, id ?? ""],
    queryFn: (): Promise<ProductIssueDetail | null> =>
      api.get<ProductIssueDetail>(`/product-work-items/${encodeURIComponent(id as string)}`),
    enabled: !!id,
    // The conversation changes on GitHub; always refetch when the dialog opens.
    staleTime: 0,
    refetchOnMount: "always",
    // While GitHub has not confirmed a queued write, poll until it does.
    refetchInterval: (query) =>
      query.state.data?.commands?.some((c) => c.status === "PENDING" || isAwaitingGithub(c, Date.now()))
        ? PENDING_POLL_MS
        : false,
  });
}
