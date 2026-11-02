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


import { useMutation, useQueryClient, type QueryClient, type UseMutationResult } from "@tanstack/react-query";
import { useBackendApi, type BackendApi } from "@api/backend/client";
import { ApiQueryKeys } from "@constants/apiConstants";
import { catalogWriteRoute, type CatalogWrite } from "@features/csm-products/api/catalogWriteRoute";
import type { CatalogChangePayload } from "@features/csm-products/types/productCatalog";

export type { CatalogWrite };

function send(api: BackendApi, write: CatalogWrite): Promise<unknown> {
  const route = catalogWriteRoute(write);
  if (route.method === "PATCH") return api.patch<CatalogChangePayload, unknown>(route.path, route.body ?? {});
  return route.body ? api.post<CatalogChangePayload, unknown>(route.path, route.body) : api.postEmpty<unknown>(route.path);
}

/**
 * After a catalogue write: the whole catalogue and the
 * product and version lists the rest of the portal reads (deployment and case
 * forms) are refetched.
 */
export function invalidateCatalog(qc: QueryClient): void {
  void qc.invalidateQueries({ queryKey: [ApiQueryKeys.PRODUCT_CATALOG] });
  void qc.invalidateQueries({ queryKey: [ApiQueryKeys.PRODUCTS] });
  void qc.invalidateQueries({ queryKey: [ApiQueryKeys.PRODUCT_VERSIONS_SEARCH] });
}

/**
 * Creates, edits or retires a product, version or release line. The backend
 * validates the write and applies it at once; a refusal carries its reason as
 * the error message. A version edit on a published listing applies to all its
 * profiles, and a retire retires the whole listing.
 */
export function useCatalogWrite(): UseMutationResult<unknown, Error, CatalogWrite> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<unknown, Error, CatalogWrite>({
    mutationFn: (write) => send(api, write),
    onSuccess: () => invalidateCatalog(qc),
  });
}
