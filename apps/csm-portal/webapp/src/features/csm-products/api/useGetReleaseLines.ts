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
import { useBackendApi } from "@api/backend/client";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { CatalogReleaseLine } from "@features/csm-products/types/productCatalog";
import { fromWireReleaseLine } from "@features/csm-products/utils/catalogWire";

/** Release lines, retired ones only when asked for. */
export function useGetReleaseLines(includeRetired = false): UseQueryResult<CatalogReleaseLine[], Error> {
  const api = useBackendApi();
  return useQuery<CatalogReleaseLine[], Error>({
    queryKey: [ApiQueryKeys.PRODUCT_CATALOG, ApiQueryKeys.PRODUCT_CATALOG_RELEASE_LINES, includeRetired],
    queryFn: async () =>
      (
        (await api.get<{ releaseLines: CatalogReleaseLine[] }>(
          `/release-lines${includeRetired ? "?includeRetired=true" : ""}`,
        ))?.releaseLines ?? []
      ).map(fromWireReleaseLine),
    staleTime: 60_000,
  });
}
