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

import { keepPreviousData, useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeServiceOffering,
  BeServiceOfferingSearchPayload,
  BeServiceOfferingSearchResponse,
} from "@api/backend/types";

/** A single page of matches is plenty for a type-ahead picker. */
const SERVICE_OFFERING_SEARCH_LIMIT = 20;

/**
 * Type-ahead service-offering search (`POST /service-offerings/search`) for
 * the "Service offering" picker on the change-request create form. Narrows
 * to offerings under `serviceId` when one is given (matching how the SN form
 * itself scopes this field once a Service is picked). Disabled until the
 * caller has typed something.
 *
 * `(query, enabled, serviceId)` — in that order, not `(query, serviceId,
 * enabled)` — so this matches AsyncEntitySelect's generic `useSearch` shape
 * and can be passed directly as a stable reference (`useSearch={useSearchServiceOfferings}`)
 * instead of wrapped in an inline arrow function, which would call a hook
 * from inside a closure and break the rules of hooks.
 */
export function useSearchServiceOfferings(
  query: string,
  enabled: boolean,
  serviceId?: string,
): UseQueryResult<BeServiceOffering[], Error> {
  const api = useBackendApi();
  const q = query.trim();

  return useQuery<BeServiceOffering[], Error>({
    queryKey: [ApiQueryKeys.SERVICE_OFFERINGS_SEARCH, q, serviceId ?? ""],
    queryFn: async (): Promise<BeServiceOffering[]> => {
      const res = await api.post<
        BeServiceOfferingSearchPayload,
        BeServiceOfferingSearchResponse
      >("/service-offerings/search", {
        filters: {
          searchQuery: q,
          ...(serviceId && { serviceIds: [serviceId] }),
        },
        pagination: { offset: 0, limit: SERVICE_OFFERING_SEARCH_LIMIT },
      });
      return res.serviceOfferings ?? [];
    },
    enabled: enabled && q.length > 0,
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });
}
