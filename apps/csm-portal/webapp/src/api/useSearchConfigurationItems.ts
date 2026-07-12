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
  BeConfigurationItem,
  BeConfigurationItemSearchPayload,
  BeConfigurationItemSearchResponse,
} from "@api/backend/types";

/** A single page of matches is plenty for a type-ahead picker. */
const CONFIGURATION_ITEM_SEARCH_LIMIT = 20;

/**
 * Type-ahead configuration-item search (`POST /configuration-items/search`)
 * for the "Configuration item" picker on the change-request create form.
 * Disabled until the caller has typed something — the CMDB is far too large
 * to load up front.
 */
export function useSearchConfigurationItems(
  query: string,
  enabled: boolean,
): UseQueryResult<BeConfigurationItem[], Error> {
  const api = useBackendApi();
  const q = query.trim();

  return useQuery<BeConfigurationItem[], Error>({
    queryKey: [ApiQueryKeys.CONFIGURATION_ITEMS_SEARCH, q],
    queryFn: async (): Promise<BeConfigurationItem[]> => {
      const res = await api.post<
        BeConfigurationItemSearchPayload,
        BeConfigurationItemSearchResponse
      >("/configuration-items/search", {
        filters: { searchQuery: q },
        pagination: { offset: 0, limit: CONFIGURATION_ITEM_SEARCH_LIMIT },
      });
      return res.configurationItems ?? [];
    },
    enabled: enabled && q.length > 0,
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });
}
