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
  BeCatalogItemVariable,
  BeGetCatalogItemVariablesResponse,
} from "@api/backend/types";

/**
 * Variables (form fields) for a catalog item, via
 * `GET /catalogs/{catalogId}/items/{catalogItemId}/variables`. Returned sorted
 * by the backend's `order` so the form renders fields in the catalog's intended
 * sequence. Disabled until both ids are present.
 */
export function useCatalogItemVariables(
  catalogId: string | undefined,
  catalogItemId: string | undefined,
): UseQueryResult<BeCatalogItemVariable[], Error> {
  const api = useBackendApi();

  return useQuery<BeCatalogItemVariable[], Error>({
    queryKey: [
      ApiQueryKeys.CATALOG_ITEM_VARIABLES,
      catalogId ?? "",
      catalogItemId ?? "",
    ],
    queryFn: async (): Promise<BeCatalogItemVariable[]> => {
      const res = await api.get<BeGetCatalogItemVariablesResponse>(
        `/catalogs/${encodeURIComponent(catalogId as string)}/items/${encodeURIComponent(
          catalogItemId as string,
        )}/variables`,
      );
      const variables = res?.variables ?? [];
      // Stable sort by display order; variables without an order sink to the end.
      return [...variables].sort(
        (a, b) => (a.order ?? Number.MAX_SAFE_INTEGER) - (b.order ?? Number.MAX_SAFE_INTEGER),
      );
    },
    enabled: !!catalogId && !!catalogItemId,
    staleTime: 60_000,
  });
}
